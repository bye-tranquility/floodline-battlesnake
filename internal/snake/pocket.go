package snake

// pocketEscape follows a moving tail in cramped positions. A static flood
// fill loses exits that open on a later turn and counts exits a rival can seal.
type pocketEscape struct {
	release []int
	rival   []int
	eaten   []bool
}

type contestTimes struct {
	arrival, lengths    []int
	singleLength, until int
}

func (t contestTimes) unsafe(c, step, ourLength int) bool {
	if step > t.until || t.arrival[c] > step {
		return false
	}
	rivalLength := t.singleLength
	if t.lengths != nil {
		rivalLength = t.lengths[c]
	}
	return t.arrival[c] < step || rivalLength >= ourLength
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

func (r region) needsPocketCheck(length int) bool {
	return length >= 6 && (r.area <= 2*length+3 || r.exits < 2)
}

func (e *pocketEscape) score(p position, us, foe serpent, room region, threat []int) int {
	const horizon = 10
	length := len(us.body)
	if !room.needsPocketCheck(length) {
		return 0
	}
	e.setup(len(p.food), us, []serpent{foe})
	nodes := 96
	contest := contestTimes{arrival: threat, singleLength: len(foe.body), until: horizon}
	steps := e.path(p, us.body[0], length, us.health, 0, 0, horizon, contest, &nodes)
	// The static size penalty can be misleading when the next tail segment
	// opens in time. Restore it only after finding a continuous escape route.
	return -(horizon-steps)*100 + min(steps, horizon)*max(0, length+2-room.area)*7
}

func (e *pocketEscape) setup(cells int, us serpent, enemies []serpent) {
	if len(e.release) != cells {
		e.release = make([]int, cells)
		e.rival = make([]int, cells)
		e.eaten = make([]bool, cells)
	} else {
		clear(e.release)
		clear(e.rival)
	}
	for i, c := range us.body {
		e.release[c] = max(e.release[c], len(us.body)-i)
	}
	for _, foe := range enemies {
		for i, c := range foe.body {
			e.rival[c] = max(e.rival[c], len(foe.body)-i)
		}
	}
}

func (e *pocketEscape) path(p position, cell, length, hp, step, growth, horizon int, contest contestTimes, nodes *int) int {
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
		if health <= 0 || contest.unsafe(to, nextStep, length+growth) {
			continue
		}
		old := e.release[to]
		e.release[to] = nextStep + length
		if food {
			e.eaten[to] = true
		}
		best = max(best, e.path(p, to, length, health, nextStep, grow, horizon, contest, nodes))
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
