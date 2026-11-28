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
	if opts.Style == "search" && p.plainDuelGrid && len(p.snakes) == 2 {
		p.occupied(a.blocked)
		options := p.choices(p.snakes[0], a.blocked, -1)
		if options.count == 1 {
			return Decision{Move: names[options.moves[0]]}, nil
		}
	}
	fallback := p.steer(a, opts.Style)
	result := Decision{Move: names[fallback]}
	if opts.Style != "search" {
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
	if r.Turn == 0 && p.constrictor && len(r.Board.Hazards) == 0 && len(r.Board.Food) == 0 {
		if move, ok := p.constrictorOpening(); ok {
			return Decision{Move: names[move]}, nil
		}
	}
	if len(p.snakes) != 2 {
		if p.plainDuelGrid && len(p.snakes) == 3 && len(p.snakes[0].body) >= 6 {
			search := groupDuel{analysis: a, deadline: started.Add(min(budget, 35*time.Millisecond))}
			if search.active(p) {
				result.Move = names[search.choose(p, fallback)]
			}
		}
		if p.forecastFork() {
			end := started.Add(min(budget, 45*time.Millisecond))
			result.Move = names[p.refinePlacement(a, fallback, end)]
		}
		return result, nil
	}
	if !p.constrictor && !p.royale && r.Turn < 8 && p.snakes[0].health >= 90 &&
		len(p.snakes[0].body) == 3 && len(p.snakes[1].body) == 3 {
		if move, ok := a.openingFood(p); ok {
			return Decision{Move: names[move]}, nil
		}
	}
	search := duelSearch{analysis: a, deadline: started.Add(budget), limit: opts.NodeLimit}
	if p.plainDuelGrid {
		search.history = make(map[uint64]lineOrder, 128)
	}
	if opts.NodeLimit == 1200 && !p.royale && !p.constrictor {
		if a.tightDuel(p) {
			search.limit = 4800
		}
		// Short snakes far behind have to convert food opportunities quickly.
		// The broader territorial search can spend that window circling.
		if p.plainDuelGrid && (len(p.snakes[0].body) >= 7 || len(p.snakes[0].body)+2 >= len(p.snakes[1].body)) {
			search.limit *= 4
		}
	}
	ourLength, rivalLength := len(p.snakes[0].body), len(p.snakes[1].body)
	// Spend the saved search work while both snakes can still change their
	// route through open space. Keep the ordinary search in dense endgames.
	if opts.NodeLimit == 1200 && p.plainDuelGrid && ourLength+rivalLength >= 26 &&
		max(ourLength, rivalLength) <= 26 {
		search.reuseMoves = true
		search.limit += search.limit / 2
	}
	move, depth := search.deepen(p, fallback)
	return Decision{Move: names[move], Depth: depth, Nodes: search.nodes}, nil
}
