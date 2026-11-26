package snake

type routeLabel struct{ cell, turn, hp int }

type royaleRoutes struct {
	arrival, bestHP, retreat, rivals []int
	frontier                         []routeLabel
	tails                            pocketEscape
}

// A healthy snake crosses a hazard in one turn, even when it costs 15 HP.
// A later arrival matters if it carries enough health to reach new cells.
func (r *royaleRoutes) travel(p position, blocked []bool, start, hp, turn int) {
	for i := range r.arrival {
		r.arrival[i], r.bestHP[i] = infinity, 0
	}
	r.arrival[start], r.bestHP[start] = turn, hp
	r.frontier = append(r.frontier[:0], routeLabel{start, turn, hp})
	for i := 0; i < len(r.frontier); i++ {
		at := r.frontier[i]
		for _, to := range p.next[at.cell] {
			if to < 0 || blocked[to] {
				continue
			}
			nextHP := at.hp - 1 - p.damage[to]
			if p.food[to] && to != start {
				nextHP = 100
			}
			if nextHP <= r.bestHP[to] {
				continue
			}
			r.bestHP[to] = nextHP
			r.arrival[to] = min(r.arrival[to], at.turn+1)
			r.frontier = append(r.frontier, routeLabel{to, at.turn + 1, nextHP})
		}
	}
}

func (a *analysis) prepareRoyale(p position) *royaleRoutes {
	n := len(p.food)
	r := &royaleRoutes{arrival: make([]int, n), bestHP: make([]int, n), retreat: make([]int, n), rivals: make([]int, n)}
	r.tails.setup(n, p.snakes[0], p.snakes[1:])
	p.occupied(a.blocked)
	// Reverse shortest paths: leaving a hazard costs the price of the square
	// entered. No second meal is assumed between the food and the safe zone.
	queue := make([]int, 0, n)
	for c := range p.damage {
		a.nearest[c], r.retreat[c] = infinity, infinity
		if !a.blocked[c] && p.damage[c] == 0 {
			r.retreat[c] = 0
			queue = append(queue, c)
		}
	}
	for i := 0; i < len(queue); i++ {
		to := queue[i]
		cost := r.retreat[to] + 1 + p.damage[to]
		for _, from := range p.next[to] {
			if from >= 0 && !a.blocked[from] && cost < r.retreat[from] {
				r.retreat[from] = cost
				queue = append(queue, from)
			}
		}
	}
	for _, enemy := range p.snakes[1:] {
		r.travel(p, a.blocked, enemy.body[0], enemy.health, 0)
		for c, turn := range r.arrival {
			if turn < a.nearest[c] {
				a.nearest[c], r.rivals[c] = turn, len(enemy.body)
			} else if turn == a.nearest[c] {
				r.rivals[c] = max(r.rivals[c], len(enemy.body))
			}
		}
	}
	return r
}

func (r *royaleRoutes) assess(p position, a *analysis, c, hp, length int, room *region) (int, int) {
	r.travel(p, a.blocked, c, hp, 1)
	land, safe, meal, available := 0, infinity, infinity, infinity
	for to, turn := range r.arrival {
		if turn == infinity {
			continue
		}
		if p.food[to] && to != c {
			available = min(available, a.own[to])
		}
		if turn > a.nearest[to] || turn == a.nearest[to] && length <= r.rivals[to] {
			continue
		}
		if turn < a.nearest[to] {
			land += 2
		} else {
			land++
		}
		if p.damage[to] == 0 {
			safe = min(safe, a.own[to])
		}
		if p.food[to] && to != c {
			retreat := r.retreat[to]
			if retreat >= 100 {
				continue
			}
			meal = min(meal, a.own[to]+retreat/3)
		}
	}
	// A contested meal is still a chance to live. At low health the costs
	// of an impossible meal and a contested but reachable meal differ sharply.
	room.food = min(meal, available+5)
	penalty := 0
	if hp < 65 && available > hp {
		penalty = (65 - hp) * 35
	} else if hp < 65 && meal > hp {
		penalty = (65 - hp) * 3
	}
	if p.damage[c] > 0 {
		penalty += min(75, safe)*2 + max(0, min(100, r.retreat[c])-hp+1)*10
		if p.food[c] {
			penalty += min(100, r.retreat[c])
		}
	}
	return land, penalty
}

// Body cells become free in tail order. Food delays later releases. Use
// near-term enemy arrivals to test whether our first way out is contestable.
func (r *royaleRoutes) escape(p position, a *analysis, first, hp, extra int) int {
	length := len(p.snakes[0].body)
	old := r.tails.release[first]
	r.tails.release[first] = 1 + length
	growth := 0
	if p.food[first] {
		r.tails.eaten[first] = true
		growth++
	}
	nodes := 160
	contest := contestTimes{arrival: a.nearest, lengths: r.rivals, until: 4}
	steps := r.tails.path(p, first, length, hp, 1, growth, extra+1, contest, &nodes) - 1
	r.tails.release[first], r.tails.eaten[first] = old, false
	return steps
}
