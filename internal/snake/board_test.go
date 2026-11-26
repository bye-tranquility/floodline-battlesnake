package snake

import (
	"fmt"
	"slices"
	"testing"
	"time"

	rules "github.com/BattlesnakeOfficial/rules"
)

func setup(mode string, bodies [][]int, health []int, food, hazards []int) Request {
	var r Request
	r.Game.Ruleset.Name = mode
	r.Game.Map = mode
	r.Game.Timeout = 500
	r.Board.Width, r.Board.Height = 7, 7
	pts := func(cells []int) []Point {
		var result []Point
		for _, c := range cells {
			result = append(result, Point{c % 7, c / 7})
		}
		return result
	}
	r.Board.Food, r.Board.Hazards = pts(food), pts(hazards)
	for i, body := range bodies {
		r.Board.Snakes = append(r.Board.Snakes, WireSnake{ID: fmt.Sprint(i), Health: health[i], Body: pts(body)})
	}
	r.You = r.Board.Snakes[0]
	return r
}

func TestTurnMatchesOfficialEngine(t *testing.T) {
	cases := map[string]Request{
		"shared food and last health": setup("standard", [][]int{{23, 22, 21}, {25, 26, 27}}, []int{1, 1}, []int{24}, nil),
		"unequal heads":               setup("standard", [][]int{{23, 22, 21, 28}, {25, 26, 27}}, []int{80, 70}, nil, nil),
		"single tail, stacked tail":   setup("standard", [][]int{{8, 9, 16, 15}, {22, 23, 30, 30}}, []int{80, 70}, []int{29}, nil),
		"wall and starving body":      setup("standard", [][]int{{0, 7, 8}, {2, 1, 8}}, []int{90, 1}, nil, nil),
		"food and stacked hazards":    setup("royale", [][]int{{23, 22, 21}, {25, 26, 27}}, []int{15, 1}, []int{24}, []int{16, 16, 24, 32}),
		"opening constrictor":         setup("constrictor", [][]int{{8, 8, 8}, {40, 40, 40}}, []int{100, 100}, nil, nil),
		"constrictor trail":           setup("constrictor", [][]int{{23, 22, 21, 21}, {25, 26, 27, 27}}, []int{100, 100}, nil, nil),
		"three heads":                 setup("standard", [][]int{{23, 22, 21}, {25, 26, 27}, {31, 38, 45}}, []int{80, 70, 50}, []int{24}, nil),
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := r.position()
			if err != nil {
				t.Fatal(err)
			}
			board := rules.NewBoardState(7, 7)
			board.Turn = 5
			points := func(ps []Point) []rules.Point {
				var out []rules.Point
				for _, q := range ps {
					out = append(out, rules.Point{X: q.X, Y: q.Y})
				}
				return out
			}
			board.Food, board.Hazards = points(r.Board.Food), points(r.Board.Hazards)
			for _, s := range r.Board.Snakes {
				board.Snakes = append(board.Snakes, rules.Snake{ID: s.ID, Body: points(s.Body), Health: s.Health})
			}
			engine := rules.NewRulesetBuilder().WithParams(map[string]string{rules.ParamHazardDamagePerTurn: "14"}).NamedRuleset(r.Game.Ruleset.Name)
			for choices := 0; choices < 1<<(2*len(board.Snakes)); choices++ {
				moves := make([]direction, len(board.Snakes))
				var official []rules.SnakeMove
				for i, s := range board.Snakes {
					moves[i] = direction(choices >> (2 * i) & 3)
					official = append(official, rules.SnakeMove{ID: s.ID, Move: names[moves[i]]})
				}
				got := p.advance(moves)
				_, want, err := engine.Execute(board, official)
				if err != nil {
					t.Fatal(err)
				}
				for i, s := range want.Snakes {
					actual := got.snakes[i]
					if (len(actual.body) > 0) != (s.EliminatedCause == rules.NotEliminated) {
						t.Fatalf("moves %v snake %d life mismatch: %+v / %+v", moves, i, actual, s)
					}
					if len(actual.body) == 0 {
						continue
					}
					var body []int
					for _, q := range s.Body {
						body = append(body, q.Y*7+q.X)
					}
					if actual.health != s.Health || !slices.Equal(actual.body, body) {
						t.Fatalf("moves %v snake %d got %+v want %v hp=%d", moves, i, actual, body, s.Health)
					}
				}
				wantFood := make([]bool, len(p.food))
				for _, q := range want.Food {
					wantFood[q.Y*7+q.X] = true
				}
				if !slices.Equal(got.food, wantFood) {
					t.Fatalf("moves %v food mismatch", moves)
				}
			}
		})
	}
}

func TestSearchDeadlineAndLastHealth(t *testing.T) {
	r := setup("standard", [][]int{{23, 22, 21}, {39, 40, 41}}, []int{1, 100}, []int{24}, nil)
	opts := Options{Style: "search", Budget: 75 * time.Millisecond, NodeLimit: 1200}
	got, err := Decide(&r, time.Now(), opts)
	if err != nil || got.Move != "right" || got.Depth == 0 {
		t.Fatalf("%+v %v", got, err)
	}
	r.Game.Timeout = 100
	got, err = Decide(&r, time.Now(), opts)
	if err != nil || got.Move != "right" || got.Nodes != 0 {
		t.Fatalf("short timeout %+v %v", got, err)
	}
	r.Game.Timeout = 500
	got, _ = Decide(&r, time.Now().Add(-time.Second), opts)
	if got.Nodes != 0 || got.Move != "right" {
		t.Fatalf("expired request %+v", got)
	}
}
