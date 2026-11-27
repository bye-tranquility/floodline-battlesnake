package snake

// Adjust the multi-snake Constrictor score using final places in paired
// continuations from the same board. The fitted coefficients are in hundredths.
func (a *analysis) placementScore(p position, start int, room region, baseline int) int {
	nearFour, nearEight, tiedSix := 0, 0, 0
	sameColor, otherColor := 0, 0
	color := (start%p.width + start/p.width) & 1
	for c, distance := range a.own {
		if distance == infinity {
			continue
		}
		arrival := distance + 1
		if arrival < a.nearest[c] {
			if arrival <= 4 {
				nearFour++
			}
			if arrival <= 8 {
				nearEight++
			}
			if (c%p.width+c/p.width)&1 == color {
				sameColor++
			} else {
				otherColor++
			}
		} else if arrival == a.nearest[c] && arrival <= 6 {
			tiedSix++
		}
	}

	// A continuous path alternates square colors. Count onward choices that
	// can still branch after an opponent's second move.
	parityBound := min(45, 2*sameColor, 2*otherColor+1)
	branching := 0
	for _, exit := range p.next[start] {
		if exit < 0 || a.blocked[exit] || a.nearest[exit] <= 2 {
			continue
		}
		onward := 0
		for _, c := range p.next[exit] {
			if c >= 0 && c != start && !a.blocked[c] && a.nearest[c] > 2 {
				onward++
			}
		}
		if onward >= 2 {
			branching++
		}
	}

	maturity := min(18, max(0, len(p.snakes[0].body)-7))
	centrality := -p.centerDistance(start)
	predicted := 88*baseline + 768*min(40, room.area) - 2383*room.exits
	predicted += -2092*nearFour - 3661*nearEight + 3737*tiedSix
	predicted += (7040-173*maturity)*parityBound - 3403*min(12, a.nearest[start])
	predicted += (2274-422*maturity)*centrality + 5763*branching
	adjustment := (predicted - 100*baseline) * 12 / 10000
	return baseline + max(-600, min(600, adjustment))
}
