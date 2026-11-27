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

func TestInterceptDenoMBeforeEnteringEdgeCorridor(t *testing.T) {
	for _, req := range readReplay(t, "duels-denom") {
		if req.Turn != 238 {
			continue
		}
		if req.You.Body[0] != (Point{8, 2}) || len(req.You.Body) != 22 {
			t.Fatal("expected DenoM's approach to the east wall")
		}
		if got := replayMove(t, &req); got != up {
			t.Fatalf("turn 238: chose %s; up intercepts the recorded opponent four moves later", names[got])
		}
		return
	}
	t.Fatal("missing DenoM turn 238")
}

func TestWhiravRematchAtNorthWall(t *testing.T) {
	for _, req := range readReplay(t, "duels-whirav-rematch") {
		if req.Turn == 257 {
			if req.You.Body[0] != (Point{4, 9}) {
				t.Fatal("unexpected Whirav position")
			}
			if got := replayMove(t, &req); got != left {
				t.Fatalf("turn 257 chose %s, left wins on turn 275 against the recorded replies", names[got])
			}
			return
		}
	}
	t.Fatal("missing Whirav rematch turn 257")
}

func TestConstrictorStartingFormations(t *testing.T) {
	want := map[Point]direction{
		{1, 5}: up, {9, 5}: up, {5, 1}: left, {5, 9}: right,
		{1, 1}: right, {1, 9}: right, {9, 1}: left, {9, 9}: left,
	}
	records := append(readReplay(t, "constrictor-edge-openings"), readReplay(t, "constrictor-corner-openings")...)
	for _, req := range records {
		start := req.You.Body[0]
		move, ok := want[start]
		if !ok || replayMove(t, &req) != move {
			t.Fatalf("unexpected opening at %v", start)
		}
		delete(want, start)
		p, err := req.position()
		if err != nil {
			t.Fatal(err)
		}
		p.snakes[1].body = []int{0, 0, 0}
		if _, matched := p.constrictorOpening(); matched {
			t.Fatal("opening matched a nonstandard formation")
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing starting positions %v", want)
	}
}

func TestElizabeth1Corridor(t *testing.T) {
	checked := 0
	for _, req := range readReplay(t, "duels-elizabeth1") {
		if req.Turn != 307 && req.Turn != 311 {
			continue
		}
		if len(req.You.Body) != 39 {
			t.Fatal("expected the long snake approaching its own corridor")
		}
		if got := replayMove(t, &req); got != left {
			t.Fatalf("turn %d: chose %s, left wins against the recorded replies", req.Turn, names[got])
		}
		checked++
	}
	if checked != 2 {
		t.Fatalf("found %d of the two turning points", checked)
	}
}

func TestMoveOrderingPreservesCompletedDuelScores(t *testing.T) {
	for _, req := range readReplay(t, "duels-denom") {
		p, err := req.position()
		if err != nil {
			t.Fatal(err)
		}
		search := func() duelSearch {
			return duelSearch{analysis: newAnalysis(len(p.food)), deadline: time.Now().Add(time.Second), limit: 100000}
		}
		normal, ordered := search(), search()
		ordered.history = make(map[uint64]lineOrder)
		ordered.best(p, 2, 0, -infinity, infinity, -1, 1)
		_, want, complete := normal.best(p, 3, 0, -infinity, infinity, -1, 1)
		_, got, done := ordered.best(p, 3, 0, -infinity, infinity, -1, 1)
		if !complete || !done || got != want {
			t.Fatalf("turn %d: ordered score %d, original %d, completed %v/%v", req.Turn, got, want, done, complete)
		}
	}
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

func TestConstrictorKopertekhovFork(t *testing.T) {
	for _, req := range readReplay(t, "constrictor-kopertekhov") {
		if req.Turn != 15 {
			continue
		}
		p, err := req.position()
		if err != nil || !p.forecastFork() || req.You.Body[0] != (Point{0, 3}) {
			t.Fatalf("unexpected Constrictor fork, error %v", err)
		}
		a := newAnalysis(len(p.food))
		prior := p.steer(a, "search")
		if prior != up {
			t.Fatalf("ordinary placement chose %s", names[prior])
		}
		if got := p.refinePlacement(a, prior, time.Now()); got != prior {
			t.Fatal("expired forecast replaced the completed decision")
		}
		if got := p.refinePlacement(a, prior, time.Now().Add(time.Second)); got != down {
			t.Fatalf("chose %s, down survives the recorded opponent responses", names[got])
		}
		return
	}
	t.Fatal("missing Constrictor turn 15")
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

func TestConstrictorForkfangRematch(t *testing.T) {
	records := readReplay(t, "constrictor-forkfang-rematch")
	if len(records) != 12 {
		t.Fatalf("received %d recorded positions", len(records))
	}
	for _, want := range []struct {
		turn int
		move direction
	}{{5, right}, {11, left}} {
		req := records[want.turn-2]
		if req.Game.ID != "3773cfda-13ee-4a3f-b95c-2fccc723e7b0" || req.Turn != want.turn {
			t.Fatalf("missing Forkfang turn %d", want.turn)
		}
		if move := replayMove(t, &req); move != want.move {
			t.Errorf("turn %d: got %s, want %s", want.turn, names[move], names[want.move])
		}
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

func TestTakeFoodAfterMissingMealsInDuels(t *testing.T) {
	for _, tc := range []struct {
		corpus, game string
		turn, health int
		food         Point
		move         direction
	}{
		{"duels-olej-rematch", "db3542c3-588b-4b7e-98b5-952d415c0d0d", 97, 5, Point{1, 3}, left},
		{"duels-actionhero-classb", "94110afd-c621-48b8-8c4e-2e85769ec2a1", 85, 17, Point{3, 1}, down},
	} {
		t.Run(tc.corpus, func(t *testing.T) {
			for _, req := range readReplay(t, tc.corpus) {
				if req.Game.ID != tc.game || req.Turn != tc.turn {
					continue
				}
				if req.You.Health != tc.health || !slices.Contains(req.Board.Food, tc.food) {
					t.Fatal("missing the recorded food or health")
				}
				if got := replayMove(t, &req); got != tc.move {
					t.Fatalf("turn %d: chose %s, expected %s towards food", tc.turn, names[got], names[tc.move])
				}
				return
			}
			t.Fatalf("missing turn %d", tc.turn)
		})
	}
}
