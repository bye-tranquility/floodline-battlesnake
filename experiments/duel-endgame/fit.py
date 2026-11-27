"""Fit the 32 depth-two trees used in Floodline's ordinary duel endgames."""

from collections import Counter
import gzip
import hashlib
import json
from pathlib import Path
import runpy

import numpy as np


HERE = Path(__file__).resolve().parent
RATE = 0.16
PENALTY = 1.5


def read_examples(name):
    with gzip.open(HERE / name, "rt") as source:
        records = [json.loads(line) for line in source]
    counts = Counter(row["Seed"] for row in records)
    features = np.asarray([row["Features"] for row in records], dtype=np.int32)
    wins = np.asarray([row["Win"] for row in records])
    weights = np.asarray([1 / counts[row["Seed"]] for row in records])
    held = np.asarray([
        int.from_bytes(hashlib.sha256(str(row["Seed"]).encode()).digest()[:2], "big") % 5 == 0
        for row in records
    ])
    return features, wins, weights, held


def fit_tree(inputs, bins, thresholds, gradient, curvature, indices, depth):
    g, h = gradient[indices].sum(), curvature[indices].sum()
    leaf = float(np.clip(g / (h + PENALTY), -2, 2))
    if depth == 0 or len(indices) < 140:
        return leaf
    best = None
    best_gain = 0.12 + g**2 / (h + PENALTY)
    for feature, cuts in enumerate(thresholds):
        which = bins[feature][indices]
        def prefix(weights=None):
            return np.bincount(which, weights=weights, minlength=len(cuts) + 1).cumsum()[:-1]
        count = prefix()
        left_g, left_h = prefix(gradient[indices]), prefix(curvature[indices])
        gains = left_g**2 / (left_h + PENALTY) + (g - left_g)**2 / (h - left_h + PENALTY)
        valid = (count >= 70) & (len(indices) - count >= 70) & (left_h >= 2) & (h - left_h >= 2)
        gains[~valid] = -np.inf
        cut = int(np.argmax(gains))
        if gains[cut] > best_gain:
            best_gain = gains[cut]
            best = feature, int(cuts[cut])
    if best is None:
        return leaf
    feature, cut = best
    left = inputs[indices, feature] <= cut
    args = inputs, bins, thresholds, gradient, curvature
    return (feature, cut, fit_tree(*args, indices[left], depth - 1),
            fit_tree(*args, indices[~left], depth - 1))


def evaluate(tree, inputs):
    if isinstance(tree, float):
        return np.full(len(inputs), tree)
    feature, cut, left, right = tree
    return np.where(inputs[:, feature] <= cut, evaluate(left, inputs), evaluate(right, inputs))


def train(features, wins, weights):
    n = len(wins)
    inputs = np.vstack((features[:, 0], features[:, 1]))
    labels = np.r_[wins, 1 - wins]
    weights = np.tile(weights, 2)
    logistic_fit = runpy.run_path(str(HERE.parent / "constrictor-placement" / "fit.py"))["fit"]
    scale = logistic_fit(inputs[:, :1].astype(float), labels, weights)[0]
    logits = scale * inputs[:, 0]
    quantiles = np.arange(1, 20) / 20
    thresholds = [np.unique(np.quantile(column, quantiles, method="nearest")) for column in inputs.T]
    bins = [np.searchsorted(cuts, column) for cuts, column in zip(thresholds, inputs.T)]
    trees = []
    for _ in range(32):
        probability = 1 / (1 + np.exp(-np.clip(logits, -35, 35)))
        gradient = (labels - probability) * weights
        curvature = probability * (1 - probability) * weights
        tree = fit_tree(inputs, bins, thresholds, gradient, curvature, np.arange(len(labels)), 2)
        trees.append(tree)
        predictions = evaluate(tree, inputs)
        step = RATE * (predictions[:n] - predictions[n:]) / 2
        logits += np.r_[step, -step]
    return scale, trees


def report(label, scale, trees, features, wins, weights):
    baseline = features[:, 0, 0] * scale
    change = sum(evaluate(tree, features[:, 0]) - evaluate(tree, features[:, 1]) for tree in trees)
    change *= RATE / 2
    for name, logits in (("baseline", baseline), ("fitted", baseline + change),
                         ("quarter", baseline + change / 4)):
        loss = np.logaddexp(0, logits) - wins * logits
        accuracy = np.where((logits == 0) | (wins == 0.5), 0.5, (logits > 0) == (wins > 0.5))
        print(label, name, "log loss", round(np.average(loss, weights=weights), 6),
              "accuracy", round(np.average(accuracy, weights=weights), 6))


def coefficients(tree, scale):
    def as_split(node):
        return node if isinstance(node, tuple) else (0, 0, node, node)
    feature, cut, left, right = as_split(tree)
    left, right = as_split(left), as_split(right)
    leaves = [round(100 * RATE * weight / scale) for weight in (*left[2:], *right[2:])]
    return [feature, cut, *left[:2], *right[:2], *leaves]


def main():
    features, wins, weights, held = read_examples("examples.jsonl.gz")
    scale, trees = train(features[~held], wins[~held], weights[~held])
    report("held-out seeds", scale, trees, features[held], wins[held], weights[held])
    arena, outcomes, arena_weights, _ = read_examples("arena.jsonl.gz")
    report("rated games", scale, trees, arena, outcomes, arena_weights)
    for tree in trees:
        print("{" + ", ".join(map(str, coefficients(tree, scale))) + "},")


if __name__ == "__main__":
    main()
