package snake

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"slices"
	"testing"
	"time"
)

func readReplay(t *testing.T, corpus string) []Request {
	t.Helper()
	file, err := os.Open("../../testdata/replays/" + corpus + ".jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var records []Request
	decoder := json.NewDecoder(z)
	for {
		var req Request
		err := decoder.Decode(&req)
		if err == io.EOF {
			return records
		}
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, req)
	}
}

func replayMove(t *testing.T, req *Request) direction {
	t.Helper()
	d, err := Decide(req, time.Now(), Options{Style: "search", Budget: 75 * time.Millisecond, NodeLimit: 1200})
	if err != nil {
		t.Fatal(err)
	}
	return direction(slices.Index(names[:], d.Move))
}

func TestEscapeAtWhiravFork(t *testing.T) {
	for _, req := range readReplay(t, "duels") {
		if req.Game.ID == "35227f65-f759-4caa-8c87-17a1c746f6a6" && req.Turn == 231 {
			if got := replayMove(t, &req); got != down {
				t.Fatalf("turn 231: chose %s at the last eight-turn escape", names[got])
			}
			return
		}
	}
	t.Fatal("missing Whirav fork")
}

func TestApproachFoodWhenFiveSegmentsBehind(t *testing.T) {
	for _, req := range readReplay(t, "duels-olej") {
		if req.Turn != 54 {
			continue
		}
		p, err := req.position()
		if err != nil {
			t.Fatal(err)
		}
		if len(p.snakes[1].body)-len(p.snakes[0].body) != 5 || !p.food[4*p.width+7] {
			t.Fatal("expected a five-segment deficit and food at (7,4)")
		}
		got := replayMove(t, &req)
		if got != right && got != down {
			t.Fatalf("turn 54 against Olej1: %s takes the head further from food", names[got])
		}
		return
	}
	t.Fatal("missing Olej1 turn 54")
}

func TestWinningDuelAndStandardEndings(t *testing.T) {
	records := readReplay(t, "wins")
	if len(records) != 24 {
		t.Fatalf("expected both twelve-turn winning lines, got %d positions", len(records))
	}
	for i, req := range records {
		p, err := req.position()
		if err != nil {
			t.Fatal(err)
		}
		final := i+1 == len(records) || records[i+1].Game.ID != req.Game.ID
		reply := right // The Arena final frame of both games records a right move.
		if !final {
			after, err := records[i+1].position()
			if err != nil {
				t.Fatal(err)
			}
			adjacent := p.next[p.snakes[1].body[0]]
			idx := slices.Index(adjacent[:], after.snakes[1].body[0])
			if idx < 0 {
				t.Fatalf("invalid recorded reply on turn %d", req.Turn)
			}
			reply = direction(idx)
		}
		next := p.advance([]direction{replayMove(t, &req), reply})
		if len(next.snakes[0].body) == 0 || final && len(next.snakes[1].body) != 0 {
			t.Fatalf("lost the winning continuation of %s at turn %d", req.Game.ID, req.Turn)
		}
	}
}

func TestRoyaleTailOpensPitonEscape(t *testing.T) {
	records := readReplay(t, "royale-piton")
	if len(records) != 1 || records[0].Game.ID != "cccacfaa-e3e7-4201-bf26-e869fbb1f706" || records[0].Turn != 129 {
		t.Fatal("missing PITON turn 129")
	}
	if got := replayMove(t, &records[0]); got != up {
		t.Fatalf("chose %s instead of the exit opened by the moving tail", names[got])
	}
}

func TestSeparatedChamberAgainstHoveringHobbs(t *testing.T) {
	records := readReplay(t, "constrictor-hobbs")
	if len(records) != 1 || records[0].Game.ID != "5bc99e79-75c5-4162-9d48-a83373b739ef" || records[0].Turn != 24 {
		t.Fatal("missing Hovering Hobbs turn 24")
	}
	p, err := records[0].position()
	if err != nil {
		t.Fatal(err)
	}
	a := newAnalysis(len(p.food))
	a.measureOpponents(p)
	before := slices.Clone(a.blocked)
	route := func(move direction) int {
		to := p.next[p.snakes[0].body[0]][move]
		search := chamberSearch{board: p, analysis: a, seen: make([]int, len(p.food)), nodesLeft: 1400}
		search.target, _ = search.bound(to)
		search.follow(to, 0)
		t.Logf("%s found %d cells, upper bound %d", names[move], search.best, search.target)
		return search.best
	}
	if route(right) <= route(up) || p.separatedPath(a, up) != right || replayMove(t, &records[0]) != right {
		t.Fatal("failed to preserve the longer trail on the right")
	}
	if !slices.Equal(before, a.blocked) {
		t.Fatal("hypothetical trail altered the actual position")
	}
}

func TestPermanentPassagesWithDistantRivals(t *testing.T) {
	records := readReplay(t, "constrictor-passages")
	expected := []struct{ turn, area, downBound, leftBound int }{
		{14, 32, 21, 29}, // ActionHero
		{11, 70, 25, 31}, // Moses' Staff
	}
	if len(records) != len(expected) {
		t.Fatalf("expected two passage positions, got %d", len(records))
	}
	for i, req := range records {
		p, err := req.position()
		if err != nil {
			t.Fatal(err)
		}
		a := newAnalysis(len(p.food))
		p.occupied(a.blocked)
		want := expected[i]
		if req.Turn != want.turn {
			t.Fatalf("unexpected turn %d", req.Turn)
		}
		for _, choice := range []struct {
			move  direction
			bound int
		}{{down, want.downBound}, {left, want.leftBound}} {
			cell := p.next[p.snakes[0].body[0]][choice.move]
			area := a.walk(p, cell, a.own).area
			bound := a.passageBound(p, cell)
			if area != want.area || bound != choice.bound {
				t.Fatalf("turn %d moving %s: area %d, path bound %d", req.Turn, names[choice.move], area, bound)
			}
		}
		if got := replayMove(t, &req); got != left {
			t.Fatalf("turn %d: chose %s, expected the left passage", req.Turn, names[got])
		}
	}
}

func TestEscapeAtClassBFork(t *testing.T) {
	for _, req := range readReplay(t, "duels-actionhero-classb") {
		if req.Game.ID != "3a216dd4-d985-482e-aa96-dad1ab5cd062" || req.Turn != 232 {
			continue
		}
		if got := replayMove(t, &req); got != up {
			t.Fatalf("turn 232: chose %s; only up survives 12 turns against every reply", names[got])
		}
		return
	}
	t.Fatal("missing Class B turn 232")
}
