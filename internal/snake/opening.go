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
