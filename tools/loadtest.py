"""Measure full /move HTTP round trips using serial and overlapping games."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import http.client
import gzip
import json
import math
from pathlib import Path
import time
from urllib.parse import urlsplit


def measure(endpoint, payload):
    client = http.client.HTTPSConnection if endpoint.scheme == "https" else http.client.HTTPConnection
    conn = client(endpoint.hostname, endpoint.port, timeout=2)
    start = time.perf_counter_ns()
    try:
        conn.request("POST", endpoint.path.rstrip("/")+"/move", payload, {"Content-Type": "application/json"})
        resp = conn.getresponse()
        data = resp.read()
        elapsed = (time.perf_counter_ns()-start)/1e6
        if resp.status != 200 or json.loads(data).get("move") not in ("up", "right", "down", "left"):
            raise RuntimeError(f"invalid move response: {resp.status} {data[:150]}")
        return elapsed
    finally:
        conn.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", default="http://127.0.0.1:8080")
    parser.add_argument("--fixtures", default="testdata/positions.jsonl.gz")
    parser.add_argument("--requests", type=int, default=1000, help="total, split between serial and concurrent")
    parser.add_argument("--out")
    args = parser.parse_args()
    opener = gzip.open if args.fixtures.endswith(".gz") else open
    with opener(args.fixtures, "rt") as f:
        samples = [json.loads(line) for line in f if line.strip()]
    url = urlsplit(args.url)
    if not samples or args.requests < 2 or url.scheme not in ("http", "https"):
        parser.error("provide HTTP URL, move fixtures and at least two requests")
    result = {}
    for workers, count in ((1, args.requests//2), (4, args.requests-args.requests//2)):
        payloads = []
        for i in range(count):
            sample = samples[i % len(samples)].copy()
            sample["game"] = dict(sample["game"], id=f"load-{workers}-{i}")
            payloads.append(json.dumps(sample, separators=(",", ":")).encode())
        with ThreadPoolExecutor(max_workers=workers) as pool:
            times = sorted(pool.map(lambda p: measure(url, p), payloads))
        result[str(workers)] = {"requests": len(times), "concurrency": workers,
                                "p50_ms": round(times[math.ceil(count*.5)-1], 3),
                                "p99_ms": round(times[math.ceil(count*.99)-1], 3),
                                "max_ms": round(times[-1], 3),
                                "over_500_ms": sum(t >= 500 for t in times)}
    report = json.dumps(result, indent=2)
    print(report)
    if args.out:
        Path(args.out).write_text(report+"\n")
    if any(v["over_500_ms"] or v["p99_ms"] >= 200 for v in result.values()):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
