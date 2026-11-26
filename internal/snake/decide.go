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
	return Decision{Move: names[fallback]}, nil
}
