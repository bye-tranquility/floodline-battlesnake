package snake

import "time"

func (p position) forecastFork() bool {
	return p.constrictor && p.width == 11 && p.height == 11 && len(p.snakes) >= 3 && len(p.snakes) <= 4 && len(p.snakes[0].body) >= 5
}

// Use the usual move evaluation and the two-player search inside a rollout.
func continuationMove(p position, actor int, style string, end time.Time) direction {
	view := p
	view.snakes = append([]serpent(nil), p.snakes...)
	view.snakes[0], view.snakes[actor] = view.snakes[actor], view.snakes[0]
	a := newAnalysis(len(p.food))
	prior := view.steer(a, style)
	if (style != "search" && style != "trail") || len(view.snakes) != 2 {
		return prior
	}
	search := duelSearch{analysis: a, limit: 240, deadline: end}
	move, _ := search.deepen(view, prior)
	return move
}

// Return twice the final place so that simultaneous eliminations retain half places.
// A Constrictor snake cannot survive longer than the number of cells on the board.
func projectedPlace(p position, first direction, rivalStyle string, end time.Time) (int, bool) {
	// After filtering deaths our snake remains at index zero whenever alive.
	for turn := 0; turn <= len(p.food); turn++ {
		var moves [4]direction
		for i := range p.snakes {
			if !time.Now().Before(end) {
				return 0, false
			}
			switch {
			case turn == 0 && i == 0:
				moves[i] = first
			case i == 0:
				moves[i] = continuationMove(p, i, "search", end)
			default:
				moves[i] = continuationMove(p, i, rivalStyle, end)
			}
		}
		n := len(p.snakes)
		next := p.advance(moves[:n])
		ours := len(next.snakes[0].body) > 0
		survivors := next.snakes[:0]
		for _, who := range next.snakes {
			if len(who.body) > 0 {
				survivors = append(survivors, who)
			}
		}
		if !ours {
			return n + len(survivors) + 1, true
		}
		if len(survivors) == 1 {
			return 2, true
		}
		next.snakes = survivors
		p = next
	}
	return 0, false
}

// The coefficients come from paired Constrictor continuations with mixed
// opponents. Each forecast produces twice the finishing place, including ties.
func forecastScore(p position, move direction, prior int, end time.Time) (int, bool) {
	var places [3]int
	for i, style := range [...]string{"search", "press", "trail"} {
		place, complete := projectedPlace(p, move, style, end)
		if !complete {
			return 0, false
		}
		places[i] = place
	}
	worst := max(places[0], places[1], places[2])
	best := min(places[0], places[1], places[2])
	stage := min(15, max(0, len(p.snakes[0].body)-8))
	score := 248*prior - 277586*places[0] - 194199*places[1] - 109498*places[2]
	score -= 354406*worst + 50630*best + 2663*stage*(worst-best)
	return score, true
}

// Compare two safe exits under identical opponent policies. If either
// forecast cannot finish within budget, keep the ordinary placement decision.
func (p position) refinePlacement(a *analysis, incumbent direction, end time.Time) direction {
	p.occupied(a.blocked)
	other, choices := incumbent, 0
	valid := false
	for move, cell := range p.next[p.snakes[0].body[0]] {
		if cell < 0 || a.blocked[cell] || a.nearest[cell] <= 1 {
			continue
		}
		choices++
		if direction(move) == incumbent {
			valid = true
		} else {
			other = direction(move)
		}
	}
	if choices != 2 || !valid {
		return incumbent
	}
	// The training tournament evaluated moves in their board order.
	// Share the deadline across both complete comparisons.
	moves := [2]direction{min(incumbent, other), max(incumbent, other)}
	var scores [4]int
	for _, move := range moves {
		score, complete := forecastScore(p, move, a.placementScores[move], end)
		if !complete {
			return incumbent
		}
		scores[move] = score
	}
	const confidence = 847298 // one million times log(0.70 / 0.30)
	if time.Now().Before(end) && scores[other]-scores[incumbent] >= confidence {
		return other
	}
	return incumbent
}
