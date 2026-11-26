package snake

import "time"

type Options struct {
	Style     string
	Budget    time.Duration
	NodeLimit int
}
type Decision struct {
	Move         string
	Depth, Nodes int
}

func Decide(r *Request, started time.Time, opts Options) (Decision, error) {
	p, err := r.position()
	if err != nil {
		return Decision{}, err
	}
	a := newAnalysis(len(p.food))
	fallback := p.steer(a, opts.Style)
	result := Decision{Move: names[fallback]}
	if opts.Style != "search" || len(p.snakes) != 2 {
		return result, nil
	}
	timeout := r.Game.Timeout
	if timeout == 0 {
		timeout = 500
	}
	budget := min(opts.Budget, 120*time.Millisecond, time.Duration(timeout-150)*time.Millisecond)
	if budget <= 0 || !time.Now().Before(started.Add(budget)) {
		return result, nil
	}
	if !p.constrictor && !p.royale && r.Turn < 8 && p.snakes[0].health >= 90 &&
		len(p.snakes[0].body) == 3 && len(p.snakes[1].body) == 3 {
		if move, ok := a.openingFood(p); ok {
			return Decision{Move: names[move]}, nil
		}
	}
	search := duelSearch{analysis: a, deadline: started.Add(budget), limit: opts.NodeLimit}
	if opts.NodeLimit == 1200 && !p.royale && !p.constrictor && a.tightDuel(p) {
		search.limit = 4800
	}
	preferred := fallback
	for depth := 1; depth <= 16; depth++ {
		move, score, complete := search.best(p, depth, 0, -infinity, infinity, preferred)
		if !complete {
			break
		}
		preferred = move
		result.Move, result.Depth = names[move], depth
		if abs(score) >= victory-depth {
			break
		}
	}
	result.Nodes = search.nodes
	return result, nil
}
