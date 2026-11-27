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

// These two layouts are the symmetric four-player openings on the 11x11
// arena board. The first direction in each seat was selected using the
// finishing places of independently simulated games.
type trailStart struct {
	pattern, seat uint8
	move          direction
}

const (
	edgePattern = iota
	cornerPattern
)

var trailStarts = map[int]trailStart{
	5*11 + 1: {edgePattern, 1, up}, 5*11 + 9: {edgePattern, 2, up},
	1*11 + 5: {edgePattern, 4, left}, 9*11 + 5: {edgePattern, 8, right},
	1*11 + 1: {cornerPattern, 1, right}, 1*11 + 9: {cornerPattern, 2, left},
	9*11 + 1: {cornerPattern, 4, right}, 9*11 + 9: {cornerPattern, 8, left},
}

func (p position) constrictorOpening() (direction, bool) {
	if p.width != 11 || p.height != 11 || len(p.snakes) != 4 {
		return up, false
	}
	ours, ok := trailStarts[p.snakes[0].body[0]]
	if !ok {
		return up, false
	}
	var occupied uint8
	for _, who := range p.snakes {
		head := who.body[0]
		start, found := trailStarts[head]
		if !found || start.pattern != ours.pattern || len(who.body) != 3 || who.body[1] != head || who.body[2] != head {
			return up, false
		}
		occupied |= start.seat
	}
	return ours.move, occupied == 15
}
