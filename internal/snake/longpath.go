package snake

type chamberSearch struct {
	board                          position
	analysis                       *analysis
	seen                           []int
	visit, nodesLeft, best, target int
}

// Every trail alternates between the two checkerboard colors. Count just the
// component the snake can enter after its earlier steps have closed cells.
func (s *chamberSearch) bound(start int) (limit, size int) {
	s.visit++
	s.seen[start] = s.visit
	queue := s.analysis.queue
	queue[0] = start
	width := s.board.width
	color := (start%width + start/width) & 1
	same := 0
	size = 1
	for i := 0; i < size; i++ {
		at := queue[i]
		if (at%width+at/width)&1 == color {
			same++
		}
		for _, to := range s.board.next[at] {
			if to < 0 || s.analysis.blocked[to] || s.seen[to] == s.visit {
				continue
			}
			s.seen[to] = s.visit
			queue[size] = to
			size++
		}
	}
	return min(2*same, 2*(size-same)+1), size
}

func (s *chamberSearch) follow(at, preceding int) {
	if s.nodesLeft == 0 || s.best == s.target {
		return
	}
	s.nodesLeft--
	bound, _ := s.bound(at)
	if preceding+bound <= s.best {
		return
	}
	s.best = max(s.best, preceding+1)
	s.analysis.blocked[at] = true
	// Visit the neighbor with the fewest onward exits first. This often
	// finds a long route early enough to prune the remaining branches.
	var cells, degree [4]int
	count := 0
	for _, to := range s.board.next[at] {
		if to < 0 || s.analysis.blocked[to] {
			continue
		}
		exits := 0
		for _, next := range s.board.next[to] {
			if next >= 0 && !s.analysis.blocked[next] {
				exits++
			}
		}
		i := count
		for i > 0 && degree[i-1] > exits {
			cells[i], degree[i] = cells[i-1], degree[i-1]
			i--
		}
		cells[i], degree[i] = to, exits
		count++
	}
	for _, to := range cells[:count] {
		s.follow(to, preceding+1)
		if s.best == s.target || s.nodesLeft == 0 {
			break
		}
	}
	s.analysis.blocked[at] = false
}

func (p position) separatedPath(a *analysis, fallback direction) direction {
	me := p.snakes[0]
	if len(me.body) < 8 || len(p.food) > 169 {
		return fallback
	}
	options := p.next[me.body[0]]
	for _, to := range options {
		if to >= 0 && !a.blocked[to] && a.nearest[to] < infinity {
			return fallback
		}
	}
	search := chamberSearch{board: p, analysis: a, seen: make([]int, len(p.food))}
	var lengths [4]int
	for move, to := range options {
		if to < 0 || a.blocked[to] {
			continue
		}
		bound, size := search.bound(to)
		if size > 48 {
			return fallback
		}
		search.nodesLeft, search.best, search.target = 1400, 0, bound
		search.follow(to, 0)
		lengths[move] = search.best
	}
	best := fallback
	for move, length := range lengths {
		if length > lengths[best] {
			best = direction(move)
		}
	}
	return best
}
