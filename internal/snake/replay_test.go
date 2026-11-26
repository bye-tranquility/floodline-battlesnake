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
