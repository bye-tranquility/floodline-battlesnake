package snake

// pocketEscape follows a moving tail in cramped positions. A static flood
// fill loses exits that open on a later turn and counts exits a rival can seal.
type pocketEscape struct {
	release []int
	rival   []int
	eaten   []bool
}

func (a *analysis) tightDuel(p position) bool {
	for _, s := range p.snakes {
		if len(s.body) < 6 {
			continue
		}
		r := a.walk(p, s.body[0], a.other)
		if r.area <= 2*len(s.body)+3 || r.exits <= 2 {
			return true
		}
	}
	return false
}

func (e *pocketEscape) score(p position, us, foe serpent, room region, threat []int) int {
	const horizon = 10
	length := len(us.body)
	if length < 6 || room.area > 2*length+3 && room.exits >= 2 {
		return 0
	}
	if len(e.release) != len(p.food) {
		e.release = make([]int, len(p.food))
		e.rival = make([]int, len(p.food))
		e.eaten = make([]bool, len(p.food))
	} else {
		clear(e.release)
		clear(e.rival)
	}
	for i, c := range us.body {
		e.release[c] = max(e.release[c], length-i)
	}
	for i, c := range foe.body {
		e.rival[c] = max(e.rival[c], len(foe.body)-i)
	}
	nodes := 96
	steps := e.path(p, us.body[0], length, len(foe.body), us.health, 0, 0, horizon, threat, &nodes)
	// The static size penalty can be misleading when the next tail segment
	// opens in time. Restore it only after finding a continuous escape route.
	return -(horizon-steps)*100 + min(steps, horizon)*max(0, length+2-room.area)*7
}

func (e *pocketEscape) path(p position, cell, length, enemyLength, hp, step, growth, horizon int, threat []int, nodes *int) int {
	if step == horizon || *nodes == 0 {
		return step
	}
	*nodes = *nodes - 1
	best := step
	nextStep := step + 1
	for _, to := range p.next[cell] {
		if to < 0 || e.release[to] > nextStep-growth || e.rival[to] > nextStep {
			continue
		}
		food := p.food[to] && !e.eaten[to]
		health, grow := hp-1-p.damage[to], growth
		if food {
			health, grow = 100, growth+1
		}
		if health <= 0 || threat[to] < nextStep || threat[to] == nextStep && length+growth <= enemyLength {
			continue
		}
		old := e.release[to]
		e.release[to] = nextStep + length
		if food {
			e.eaten[to] = true
		}
		best = max(best, e.path(p, to, length, enemyLength, health, nextStep, grow, horizon, threat, nodes))
		e.release[to] = old
		if food {
			e.eaten[to] = false
		}
		if best == horizon {
			break
		}
	}
	return best
}
