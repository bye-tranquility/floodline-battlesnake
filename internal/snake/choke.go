package snake

// In Constrictor every step of a rival's reply permanently closes a cell.
// Try the two-step approaches of the nearest rival to see what can be sealed.
type chokeCheck struct {
	seen  []int
	stamp int
}

func (h *chokeCheck) area(p position, a *analysis, start int) int {
	const limit = 40
	h.stamp++
	h.seen[start] = h.stamp
	a.queue[0] = start
	count := 1
	for i := 0; i < count && count < limit; i++ {
		for _, to := range p.next[a.queue[i]] {
			if to < 0 || a.blocked[to] || h.seen[to] == h.stamp {
				continue
			}
			h.seen[to] = h.stamp
			a.queue[count] = to
			count++
			if count == limit {
				break
			}
		}
	}
	return count
}

func (h *chokeCheck) score(p position, a *analysis, start, area int) int {
	if h.seen == nil {
		h.seen = make([]int, len(p.food))
	}
	nearest := infinity
	for _, foe := range p.snakes[1:] {
		for _, to := range p.next[foe.body[0]] {
			if to >= 0 && !a.blocked[to] {
				nearest = min(nearest, a.own[to])
			}
		}
	}
	worst := min(40, area)
	for _, foe := range p.snakes[1:] {
		for _, first := range p.next[foe.body[0]] {
			if first < 0 || first == start || a.blocked[first] || a.own[first] > nearest+2 {
				continue
			}
			a.blocked[first] = true
			worst = min(worst, h.area(p, a, start))
			for _, second := range p.next[first] {
				if second < 0 || second == start || a.blocked[second] {
					continue
				}
				a.blocked[second] = true
				worst = min(worst, h.area(p, a, start))
				a.blocked[second] = false
			}
			a.blocked[first] = false
		}
	}
	return 12*worst - 85*max(0, 14-worst)
}
