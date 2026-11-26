package snake

// A permanent trail can pass through an articulation point only once. Each
// block can contribute its cells, then at most one branch beyond its exits.
func (a *analysis) passageBound(p position, start int) int {
	n := len(p.food)
	order, low, extra := make([]int, n), make([]int, n), make([]int, n)
	stack := make([]int, 0, n)
	tick := 0
	var visit func(int, int)
	visit = func(at, prev int) {
		tick++
		order[at], low[at] = tick, tick
		stack = append(stack, at)
		for _, to := range p.next[at] {
			if to < 0 || a.blocked[to] || to == prev {
				continue
			}
			if order[to] != 0 {
				low[at] = min(low[at], order[to])
				continue
			}
			visit(to, at)
			low[at] = min(low[at], low[to])
			if low[to] < order[at] {
				continue
			}
			size, beyond := 0, 0
			for {
				i := len(stack) - 1
				vertex := stack[i]
				stack = stack[:i]
				size++
				beyond = max(beyond, extra[vertex])
				if vertex == to {
					break
				}
			}
			extra[at] = max(extra[at], size+beyond)
		}
	}
	visit(start, -1)
	return 1 + extra[start]
}
