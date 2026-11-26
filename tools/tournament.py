"""Play paired games with Battlesnake CLI v1.2.3. Uses only Python's standard library."""
import argparse
import contextlib
import hashlib
import http.client
import json
import os
from pathlib import Path
import socket
import subprocess
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def request(con, method, path, data=None):
    con.request(method, path, data, {"Content-Type": "application/json"})
    response = con.getresponse()
    return response.status, response.read()


@contextlib.contextmanager
def player(binary, style):
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    env = dict(os.environ, PORT=str(port), BIND_HOST="127.0.0.1", STYLE=style, QUIET="1", GOMAXPROCS="1")
    with subprocess.Popen([binary], env=env, stderr=subprocess.DEVNULL) as proc:
        try:
            for _ in range(200):
                if proc.poll() is not None:
                    raise RuntimeError(f"{style} exited with {proc.returncode}")
                try:
                    with contextlib.closing(http.client.HTTPConnection("127.0.0.1", port, timeout=2)) as con:
                        if request(con, "GET", "/")[0] == 200:
                            break
                except OSError:
                    pass
                time.sleep(.025)
            else:
                raise RuntimeError(f"{style} did not start")
            yield port
        finally:
            proc.terminate()


class SeatRouter(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    disable_nagle_algorithm = True

    # The official CLI iterates a Go map to assign IDs to starts. Resolve each
    # player by its actual opening cell so the second game really swaps sides.
    def log_message(self, *args):
        pass

    def reply(self, status, body):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        self.reply(200, b'{"apiversion":"1"}')

    def do_POST(self):
        state = self.server
        raw = self.rfile.read(int(self.headers["Content-Length"]))
        try:
            req = json.loads(raw)
            if self.path == "/start":
                snakes = sorted(req["board"]["snakes"], key=lambda s: (s["head"]["x"], s["head"]["y"]))
                state.labels = {s["id"]: name for s, name in zip(snakes, state.seats)}
            label = state.labels[req["you"]["id"]]
            started = time.perf_counter()
            with state.locks[label]:
                status, body = request(state.connections[label], "POST", self.path, raw)
            elapsed = (time.perf_counter()-started)*1000
            if status != 200:
                raise RuntimeError(f"{label} returned {status}")
            if self.path == "/move":
                if json.loads(body)["move"] not in ("up", "down", "left", "right"):
                    raise ValueError("invalid direction")
                state.times[label].append(elapsed)
            self.reply(status, body)
        except Exception as exc:
            state.errors.append(str(exc))
            self.reply(502, b'{}')


def play(args, router, seed, side):
    out = Path(args.out)
    path = out / f"{seed}-{side}.jsonl"
    rules = "standard" if args.mode in ("duels", "standard") else args.mode
    board = "royale" if args.mode == "royale" else "standard"
    # All seats share one router. Sequential HTTP reads the same turn for every
    # snake while letting the CLI reuse a connection throughout long tournaments.
    cmd = [args.cli, "play", "--sequential", "-W", "11", "-H", "11", "-g", rules, "-m", board,
           "--seed", str(seed), "--timeout", "500", "--output", str(path)]
    if rules == "constrictor":
        cmd += ["--minimumFood", "0", "--foodSpawnChance", "0"]
    for i in range(len(router.seats)):
        cmd += ["--name", f"Seat{i}", "--url", f"http://127.0.0.1:{router.server_port}"]
    router.errors, router.labels = [], {}
    router.times = {k: [] for k in router.seats}
    run = subprocess.run(cmd, capture_output=True, text=True, timeout=240,
                         env=dict(os.environ, GODEBUG="randseednop=0"))
    if run.returncode or router.errors or "WARN " in run.stderr or "ERROR " in run.stderr:
        diagnostics = [line for line in run.stderr.splitlines()
                       if line.startswith(("WARN ", "ERROR ", "\t"))]
        detail = "\n".join(diagnostics) or run.stderr
        raise RuntimeError(f"seed {seed} side {side}: {router.errors} {detail[-1800:]}")
    with path.open() as f:
        rows = [json.loads(line) for line in f]
    frames, result = rows[1:-1], rows[-1]
    first = frames[0]["board"]
    starts = {router.labels[s["id"]]: [s["head"]["x"], s["head"]["y"]] for s in first["snakes"]}
    last_seen = {}
    for frame in frames:
        for s in frame["board"]["snakes"]:
            last_seen[router.labels[s["id"]]] = frame["turn"]
    def place(label):
        turn = last_seen[label]
        ahead = sum(t > turn for t in last_seen.values())
        tied = sum(t == turn for t in last_seen.values())
        return 1 + ahead + (tied-1)/2
    winner = router.labels.get(result["winnerId"], "")
    times = sorted(router.times["candidate"])
    return dict(seed=seed, side=side, starts=starts, food=sorted((c["x"], c["y"]) for c in first["food"]),
                winner=winner, draw=result["isDraw"], place=place("candidate"), reference_place=place("reference"),
                turns=frames[-1]["turn"],
                candidate_requests=len(times), candidate_max_ms=round(max(times), 3))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cli", default=".work/bin/battlesnake")
    parser.add_argument("--binary", default=".work/bin/floodline")
    parser.add_argument("--reference-binary")
    parser.add_argument("--candidate", default="search")
    parser.add_argument("--opponent", default="baseline")
    parser.add_argument("--mode", choices=("duels", "standard", "royale", "constrictor"), default="duels")
    parser.add_argument("--seeds", type=int, default=12)
    parser.add_argument("--first-seed", type=int, default=1001)
    parser.add_argument("--out", required=True)
    parser.add_argument("--hypothesis", default="", help="description to keep with the results")
    args = parser.parse_args()
    if args.seeds < 1:
        parser.error("--seeds must be positive")
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    log = out / "games.jsonl"
    if log.exists():
        parser.error(f"{log} already exists; select a new output directory")
    ids = {label: hashlib.sha256(Path(binary).read_bytes()).hexdigest() for label, binary in
           (("candidate", args.binary), ("reference", args.reference_binary or args.binary))}
    (out / "run.json").write_text(json.dumps(dict(hypothesis=args.hypothesis, sha256=ids,
        search_ms=os.environ.get("SEARCH_MS", "75"), search_nodes=os.environ.get("SEARCH_NODES", "1200"),
        engine="Battlesnake CLI v1.2.3", options=vars(args)), indent=2)+"\n")
    records = []
    with contextlib.ExitStack() as stack, ThreadingHTTPServer(("127.0.0.1", 0), SeatRouter) as router:
        styles = {"candidate": args.candidate, "reference": args.opponent}
        if args.mode != "duels":
            styles.update(forager="food", pressure="press")
        router.players = {k: stack.enter_context(player(
            args.reference_binary if k == "reference" and args.reference_binary else args.binary, v))
            for k, v in styles.items()}
        router.connections = {k: stack.enter_context(contextlib.closing(
            http.client.HTTPConnection("127.0.0.1", port, timeout=2))) for k, port in router.players.items()}
        router.locks = {k: threading.Lock() for k in styles}
        threading.Thread(target=router.serve_forever, daemon=True).start()
        try:
            with log.open("x") as f:
                for seed in range(args.first_seed, args.first_seed + args.seeds):
                    for side in range(2):
                        router.seats = list(styles)
                        if side:
                            router.seats[:2] = reversed(router.seats[:2])
                        record = play(args, router, seed, side)
                        if side:
                            other = records[-1]
                            if (other["starts"]["candidate"] != record["starts"]["reference"] or
                                other["starts"]["reference"] != record["starts"]["candidate"] or
                                other["food"] != record["food"]):
                                raise RuntimeError(f"opening mismatch on seed {seed}")
                        records.append(record)
                        print(json.dumps(record), file=f, flush=True)
        finally:
            router.shutdown()
    wins = sum(r["winner"] == "candidate" for r in records)
    draws = sum(r["draw"] for r in records)
    summary = dict(mode=args.mode, candidate=args.candidate, opponent=args.opponent,
                   first_seed=args.first_seed, pairs=args.seeds, wins=wins, draws=draws,
                   losses=len(records)-wins-draws, average_place=sum(r["place"] for r in records)/len(records),
                   reference_average_place=sum(r["reference_place"] for r in records)/len(records),
                   max_ms=max(r["candidate_max_ms"] for r in records))
    if args.mode == "duels":
        summary["points"] = wins + draws / 2
    (out / "summary.json").write_text(json.dumps(summary, indent=2)+"\n")
    print(json.dumps(summary))


if __name__ == "__main__":
    main()
