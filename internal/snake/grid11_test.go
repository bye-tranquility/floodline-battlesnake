package snake

import "testing"

func TestGridNeighborsMatchBoard(t *testing.T) {
	var req Request
	req.Board.Width, req.Board.Height = 11, 11
	req.Board.Snakes = []WireSnake{{ID: "one", Body: []Point{{5, 5}}}}
	req.You = req.Board.Snakes[0]
	p, err := req.position()
	if err != nil {
		t.Fatal(err)
	}
	for c, adjacent := range p.next {
		var origin, want cells11
		origin.add(c)
		for _, to := range adjacent {
			if to >= 0 {
				want.add(to)
			}
		}
		if got := origin.neighbors(); got != want {
			t.Fatalf("neighbors of %d: got %v, want %v", c, got, want)
		}
	}
}

func TestPlainDuelTerritoryMatchesFloodFill(t *testing.T) {
	for _, corpus := range []string{"duels", "duels-approach", "duels-olej", "duels-olej-rematch", "wins"} {
		for _, r := range readReplay(t, corpus) {
			p, err := r.position()
			if err != nil {
				t.Fatal(err)
			}
			if !p.plainDuelGrid || len(p.snakes) != 2 {
				continue
			}
			original := newAnalysis(len(p.food))
			p.occupied(original.blocked)
			me, foe := p.snakes[0], p.snakes[1]
			us := original.walk(p, me.body[0], original.own)
			them := original.walk(p, foe.body[0], original.other)
			gap := len(me.body) - len(foe.body)
			land := 0
			for cell, ours := range original.own {
				theirs := original.other[cell]
				if min(ours, theirs) > 7 {
					continue
				}
				weight := 1
				if min(ours, theirs) <= 4 {
					weight = 3
				}
				switch {
				case ours < theirs:
					land += 2 * weight
				case theirs < ours:
					land -= 2 * weight
				case gap > 0:
					land += weight
				case gap < 0:
					land -= weight
				}
			}
			search := duelSearch{analysis: newAnalysis(len(p.food))}
			fastUs, fastThem, fastLand := search.duelRegions(p, gap)
			if fastUs != us || fastThem != them || fastLand != land {
				t.Fatalf("%s turn %d: regions %v/%v, territory %d, want %v/%v and %d", r.Game.ID, r.Turn, fastUs, fastThem, fastLand, us, them, land)
			}
		}
	}
}
