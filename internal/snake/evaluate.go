package snake

const infinity = 1_000_000

type analysis struct {
	placementScores            [4]int
	blocked, queued            []bool
	queue, own, other, nearest []int
}

func newAnalysis(n int) *analysis {
	return &analysis{
		blocked: make([]bool, n), queued: make([]bool, n), queue: make([]int, n),
		own: make([]int, n), other: make([]int, n), nearest: make([]int, n),
	}
}

type region struct {
	area, exits, food, safe int
}

// Each edge costs one health plus the actual hazard damage. A food square can
// be reached on the last health point and restores health before hazard damage.
func (a *analysis) walk(p position, start int, dist []int) region {
	for i := range dist {
		dist[i] = infinity
	}
	n := len(dist)
	dist[start] = 0
	a.queue[0] = start
	a.queued[start] = true
	head, tail, count := 0, 1%n, 1
	stats := region{food: infinity, safe: infinity}
	for count > 0 {
		at := a.queue[head]
		head = (head + 1) % n
		count--
		a.queued[at] = false
		cost := dist[at]
		if p.food[at] {
			stats.food = min(stats.food, cost)
		}
		if p.damage[at] == 0 {
			stats.safe = min(stats.safe, cost)
		}
		for _, to := range p.next[at] {
			if to < 0 || a.blocked[to] {
				continue
			}
			if at == start {
				stats.exits++
			}
			newCost := cost + 1
			if !p.food[to] {
				newCost += p.damage[to]
			}
			if newCost >= dist[to] {
				continue
			}
			dist[to] = newCost
			if !a.queued[to] {
				a.queue[tail] = to
				tail = (tail + 1) % n
				count++
				a.queued[to] = true
			}
		}
	}
	for _, d := range dist {
		if d < infinity {
			stats.area++
		}
	}
	return stats
}

func (p position) centerDistance(c int) int {
	return abs(2*(c%p.width)-p.width+1) + abs(2*(c/p.width)-p.height+1)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Conservative territory estimate. A contested cell is valuable when a
// head-to-head there is favorable, even if both snakes can arrive on time.
func territory(ours, theirs []int, advantage int) int {
	total := 0
	for i, d := range ours {
		switch {
		case d < theirs[i]:
			total += 2
		case d == theirs[i] && d < infinity && advantage > 0:
			total++
		}
	}
	return total
}

func spaceScore(r region, length int) int {
	penalty := max(0, length+2-r.area)
	return min(2*length+10, r.area)*3 + r.exits*12 - penalty*85
}

func nutrition(r region, hp, lengthGap int) int {
	if r.food == infinity {
		return -max(0, 35-hp) * 8
	}
	distance := min(30, r.food)
	urgency := max(0, 48-hp)/4 + max(0, 2-min(lengthGap, 2))*2
	return -distance*(2+urgency) - max(0, r.food-hp+2)*14
}

func (p position) steer(a *analysis, style string) direction {
	ahead := p.snakes[0]
	var routes *royaleRoutes
	if p.royale && style == "search" && len(p.snakes) > 2 {
		routes = a.prepareRoyale(p)
	} else {
		a.measureOpponents(p)
	}
	longest := 0
	for _, s := range p.snakes[1:] {
		longest = max(longest, len(s.body))
	}
	best, chosen := -infinity, up
	checkPassages := p.constrictor && style == "search" && len(p.snakes) > 2 && len(p.food) <= 169
	if checkPassages {
		for _, cell := range p.next[ahead.body[0]] {
			if cell >= 0 && !a.blocked[cell] && a.nearest[cell] < 6 {
				checkPassages = false
				break
			}
		}
	}
	var choke chokeCheck
	for move, c := range p.next[ahead.body[0]] {
		if c < 0 || a.blocked[c] || p.nextHealth(ahead, c) <= 0 {
			continue
		}
		r := a.walk(p, c, a.own)
		inaccessible := 0
		if checkPassages {
			bound := a.passageBound(p, c)
			inaccessible = min(48, r.area) - min(48, bound)
			r.area = bound
		}
		length := len(ahead.body)
		if p.food[c] {
			length++
		}
		gap := length - longest
		hp := p.nextHealth(ahead, c)
		score := spaceScore(r, length) - p.centerDistance(c) - 36*inaccessible
		claimed := territory(a.own, a.nearest, gap)
		if routes != nil {
			var penalty int
			claimed, penalty = routes.assess(p, a, c, hp, length, &r)
			score -= penalty
		}
		switch style {
		case "food":
			score = 3*r.exits - 12*min(r.food, 35)
			if p.food[c] {
				score += 150
			}
		case "space":
			score = 3*r.area + claimed + 5*r.exits
		case "press":
			score += claimed - min(a.nearest[c], 25)*12
		default:
			score += claimed * 2
			if !p.constrictor {
				score += nutrition(r, hp, gap)
				if p.food[c] {
					score += 32 + max(0, 2-min(2, gap))*20 + max(0, 60-ahead.health)
				}
			}
		}
		if p.constrictor {
			score += r.area + claimed
			if len(p.snakes) > 2 && (style == "search" || style == "trail") {
				remaining := 240
				score += 110 * a.trail(p, c, 1, 14, &remaining)
			}
		}
		if style == "search" && len(p.snakes) > 2 {
			if p.constrictor {
				score += choke.score(p, a, c, r.area)
			} else if routes != nil {
				const extra = 8
				steps := routes.escape(p, a, c, hp, extra)
				score -= 110 * (extra - steps)
				// A snake longer than the route cannot revisit a cell.
				// Exceeding the component's size requires a tail to open a way out.
				if steps == extra && length > extra && r.area < extra+1 {
					score += 85 * max(0, length+2-r.area)
				}
			} else {
				// Bodies can move away in these modes. Look only a few turns
				// ahead for an escape before penalizing a crowded pocket.
				const escapeMoves = 4
				remaining := 60
				score -= 90 * (escapeMoves - a.trail(p, c, 1, 1+escapeMoves, &remaining))
			}
		}
		if p.damage[c] > 0 {
			score -= p.damage[c] + min(r.safe, 30)*3
		}
		if p.constrictor && style == "search" && len(p.snakes) >= 3 && len(p.snakes) <= 4 && p.width == 11 && p.height == 11 {
			score = a.placementScore(p, c, r, score)
			a.placementScores[move] = score
		}
		for _, s := range p.snakes[1:] {
			if len(s.body) == 0 || p.nextHealth(s, c) <= 0 {
				continue
			}
			for _, to := range p.next[s.body[0]] {
				if to != c {
					continue
				}
				if len(s.body) >= len(ahead.body) {
					score -= 6000
				} else {
					score += 36
				}
				break
			}
		}
		if score > best {
			best, chosen = score, direction(move)
		}
	}
	if p.constrictor && style == "search" && len(p.snakes) > 2 {
		return p.separatedPath(a, chosen)
	}
	return chosen
}

// Length of a continuous path through distinct cells that our snake can claim
// before a rival. The depth and the node budget bound the work per move.
func (a *analysis) trail(p position, cell, arrival, horizon int, remaining *int) int {
	if arrival >= horizon || *remaining <= 0 {
		return 0
	}
	*remaining = *remaining - 1
	a.blocked[cell] = true
	defer func() { a.blocked[cell] = false }()
	best := 0
	for _, to := range p.next[cell] {
		if to < 0 || a.blocked[to] || a.nearest[to] <= arrival+1 {
			continue
		}
		best = max(best, 1+a.trail(p, to, arrival+1, horizon, remaining))
		if best >= horizon-arrival {
			break
		}
	}
	return best
}

func (a *analysis) measureOpponents(p position) {
	p.occupied(a.blocked)
	for i := range a.nearest {
		a.nearest[i] = infinity
	}
	for _, s := range p.snakes[1:] {
		if len(s.body) == 0 {
			continue
		}
		a.walk(p, s.body[0], a.other)
		for c, d := range a.other {
			a.nearest[c] = min(a.nearest[c], d)
		}
	}
}
