"""Reproduce the Constrictor placement coefficients with Python 3 and NumPy."""

import argparse
import gzip
import hashlib
import itertools
import json
from pathlib import Path

import numpy as np


def comparisons(source):
    features, targets, weights, held_out = [], [], [], []
    with gzip.open(source, "rt") as records:
        for line in records:
            case = json.loads(line)
            seed = str(case["Seed"]).encode()
            fold = int.from_bytes(hashlib.sha256(seed).digest()[:2], "big") % 5
            pairs = list(itertools.combinations(case["Branches"] or [], 2))
            for first, second in pairs:
                features.append(np.subtract(first["Features"], second["Features"]))
                places = list(zip(first["Rank"], second["Rank"]))
                wins = sum(1.0 if a < b else 0.5 if a == b else 0.0 for a, b in places)
                targets.append(wins / len(places))
                weights.append(1.0 / len(pairs))
                held_out.append(fold == 0)
    return tuple(map(np.asarray, (features, targets, weights, held_out)))


def fit(features, targets, weights):
    weights = weights / weights.sum()
    scale = np.maximum(np.sqrt(weights @ np.square(features)), 0.01)
    inputs = features / scale
    coefficients = np.zeros(inputs.shape[1])
    regularization = 0.004
    for _ in range(35):
        logits = np.clip(inputs @ coefficients, -35, 35)
        probabilities = 1 / (1 + np.exp(-logits))
        gradient = inputs.T @ (weights * (probabilities - targets)) + regularization * coefficients
        curvature = weights * probabilities * (1 - probabilities)
        hessian = inputs.T @ (inputs * curvature[:, None]) + regularization * np.eye(len(coefficients))
        step = np.linalg.solve(hessian, gradient)
        coefficients -= step
        if np.max(np.abs(step)) < 1e-6:
            break
    return coefficients / scale


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("choices", type=Path, nargs="?", default=Path(__file__).with_name("choices.jsonl.gz"))
    features, targets, weights, holdout = comparisons(parser.parse_args().choices)
    train = ~holdout
    baseline = fit(features[train, :1], targets[train], weights[train])
    model = fit(features[train], targets[train], weights[train])
    print("Training comparisons", int(train.sum()))
    print("Held-out comparisons", int(holdout.sum()))
    for label, logits in (("baseline", features[holdout, :1] @ baseline),
                          ("model", features[holdout] @ model)):
        loss = np.logaddexp(0, logits) - targets[holdout] * logits
        print(label, "held-out log loss", round(np.average(loss, weights=weights[holdout]), 6))
    print("Coefficients in millionths of log odds", np.rint(1_000_000 * model).astype(int).tolist())
    print("Coefficients in hundredths of the baseline score",
          np.rint(100 * model / baseline[0]).astype(int).tolist())


if __name__ == "__main__":
    main()
