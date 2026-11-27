"""Select first moves for the two Constrictor formations, then evaluate held-out fields."""

from collections import defaultdict
import gzip
import json
from pathlib import Path
from statistics import mean


DIRECTIONS = ("up", "right", "down", "left")


def main():
    with gzip.open(Path(__file__).with_name("outcomes.jsonl.gz"), "rt") as source:
        rows = [json.loads(line) for line in source]
    fields = defaultdict(set)
    starts = defaultdict(list)
    for row in rows:
        fields[row["Fold"]].add(tuple(sorted(row["Rivals"])))
        x, y = row["Spot"]
        pattern = "edge" if x == 5 or y == 5 else "corner"
        starts[(pattern, x, y)].append(row)
    assert fields["train"].isdisjoint(fields["held"])

    tested, prior = defaultdict(list), defaultdict(list)
    for (pattern, x, y), cases in sorted(starts.items()):
        train = [row["Places"] for row in cases if row["Fold"] == "train"]
        ranks = [mean(direction) for direction in zip(*train)]
        chosen = min(range(4), key=ranks.__getitem__)
        held = [row for row in cases if row["Fold"] == "held"]
        tested[pattern].extend(row["Places"][chosen] for row in held)
        prior[pattern].extend(row["Places"][DIRECTIONS.index(row["Prior"])] for row in held)
        print((x, y), DIRECTIONS[chosen], "training mean place", round(ranks[chosen], 3))
    for pattern in sorted(tested):
        print("Held-out", pattern, "starts", len(tested[pattern]), "proposed",
              round(mean(tested[pattern]), 3), "previous", round(mean(prior[pattern]), 3))


if __name__ == "__main__":
    main()
