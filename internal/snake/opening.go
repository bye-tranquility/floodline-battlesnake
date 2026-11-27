package snake

// A short route to uncontested food gives a length advantage before both
// heads reach the center. Check every cell of the route and a way out.
func (a *analysis) openingFood(p position) (direction, bool) {
	best, choice := -infinity, up
	head := p.snakes[0].body[0]
	for m, first := range p.next[head] {
		if first < 0 || a.blocked[first] || a.nearest[first] <= 1 {
			continue
		}
		visit := func(food, before, turns int) {
			if !p.food[food] || a.nearest[food] <= turns {
				return
			}
			exits := 0
			for _, to := range p.next[food] {
				if to >= 0 && to != before && !a.blocked[to] && a.nearest[to] > turns+1 {
					exits++
				}
			}
			if exits == 0 {
				return
			}
			score := -100*turns + 5*exits - p.centerDistance(first)
			if score > best {
				best, choice = score, direction(m)
			}
		}
		visit(first, head, 1)
		for _, food := range p.next[first] {
			if food >= 0 && !a.blocked[food] {
				visit(food, first, 2)
			}
		}
	}
	return choice, best != -infinity
}

// On the four cardinal starts, follow the edge for a turn before choosing
// the first inward corridor. Each assigned move was selected on training
// lineups that were disjoint from the ones used to validate the opening.
func (p position) edgeOpening() (direction, bool) {
	if p.width != 11 || p.height != 11 || len(p.snakes) != 4 {
		return up, false
	}
	var seen [4]bool
	var first direction
	for i, s := range p.snakes {
		head := s.body[0]
		if len(s.body) != 3 || s.body[1] != head || s.body[2] != head {
			return up, false
		}
		var side, move direction
		switch head {
		case 5*11 + 1:
			side, move = left, up
		case 5*11 + 9:
			side, move = right, up
		case 1*11 + 5:
			side, move = down, left
		case 9*11 + 5:
			side, move = up, right
		default:
			return up, false
		}
		if seen[side] {
			return up, false
		}
		seen[side] = true
		if i == 0 {
			first = move
		}
	}
	return first, true
}
