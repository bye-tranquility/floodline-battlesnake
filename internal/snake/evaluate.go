package snake

const infinity = 1_000_000

type analysis struct {
	blocked, queued            []bool
	queue, own, other, nearest []int
}

func newAnalysis(n int) *analysis {
	return &analysis{make([]bool, n), make([]bool, n), make([]int, n), make([]int, n), make([]int, n), make([]int, n)}
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
	a.ownDistancesOfOpponents(p)
	longest := 0
	for _, s := range p.snakes[1:] {
		longest = max(longest, len(s.body))
	}
	best, chosen := -infinity, up
	for move, c := range p.next[ahead.body[0]] {
		if c < 0 || a.blocked[c] || p.nextHealth(ahead, c) <= 0 {
			continue
		}
		r := a.walk(p, c, a.own)
		length := len(ahead.body)
		if p.food[c] {
			length++
		}
		gap := length - longest
		hp := p.nextHealth(ahead, c)
		score := spaceScore(r, length) - p.centerDistance(c)
		claimed := territory(a.own, a.nearest, gap)
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
		}
		if p.damage[c] > 0 {
			score -= p.damage[c] + min(r.safe, 30)*3
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
	return chosen
}

func (a *analysis) ownDistancesOfOpponents(p position) {
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
