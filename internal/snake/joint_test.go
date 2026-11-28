package snake

import (
	"slices"
	"testing"
	"time"
)

func TestCachedDuelMovesMatchSimultaneousRules(t *testing.T) {
	for _, corpus := range []string{"duels-denom", "duels-elizabeth1", "duels-whirav-rematch", "duels-sosna-timeout"} {
		for _, request := range readReplay(t, corpus) {
			p, err := request.position()
			if err != nil {
				t.Fatal(err)
			}
			var joint jointTurn
			joint.prepare(p, make([]bool, len(p.food)))
			for ours := up; ours <= left; ours++ {
				for reply := up; reply <= left; reply++ {
					got, usAlive, rivalAlive := joint.advance(ours, reply)
					want := p.advance([]direction{ours, reply})
					if usAlive != (len(want.snakes[0].body) > 0) || rivalAlive != (len(want.snakes[1].body) > 0) {
						t.Fatalf("%s turn %d moves %d/%d: elimination differs", corpus, request.Turn, ours, reply)
					}
					if !usAlive || !rivalAlive {
						continue
					}
					if !slices.Equal(got.food, want.food) {
						t.Fatalf("%s turn %d moves %d/%d: food differs", corpus, request.Turn, ours, reply)
					}
					for i, snake := range got.snakes {
						if snake.health != want.snakes[i].health || !slices.Equal(snake.body, want.snakes[i].body) {
							t.Fatalf("%s turn %d moves %d/%d: snake %d differs", corpus, request.Turn, ours, reply, i)
						}
					}
					check := duelSearch{analysis: newAnalysis(len(p.food))}
					if check.evaluate(got) != check.evaluate(want) {
						t.Fatalf("%s turn %d moves %d/%d: evaluation differs", corpus, request.Turn, ours, reply)
					}
				}
			}
			ordinary := duelSearch{analysis: newAnalysis(len(p.food)), deadline: time.Now().Add(time.Second), limit: 30_000}
			cached := duelSearch{analysis: newAnalysis(len(p.food)), deadline: time.Now().Add(time.Second), limit: 30_000, reuseMoves: true}
			first, score, complete := ordinary.best(p, 4, 0, -infinity, infinity, -1, 1)
			second, otherScore, done := cached.best(p, 4, 0, -infinity, infinity, -1, 1)
			if !complete || !done || first != second || score != otherScore {
				t.Fatalf("%s turn %d: cached search chose (%s, %d), ordinary search chose (%s, %d)", corpus, request.Turn, names[second], otherScore, names[first], score)
			}
		}
	}
}
