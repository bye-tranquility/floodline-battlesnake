package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"floodline/internal/snake"
)

func TestWebhooks(t *testing.T) {
	s := service{quiet: true}
	h := s.handler()
	for _, tt := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/", "", 200}, {"POST", "/start", "{}", 200}, {"POST", "/end", "{}", 200},
		{"GET", "/other", "", 404}, {"GET", "/move", "", 405}, {"POST", "/move", "{}", 400},
		{"POST", "/move", `{"game":{"timeout":500},"turn":0,"board":{"width":3,"height":3,"snakes":[{"id":"s","health":90,"body":[{"x":0,"y":0},{"x":0,"y":1},{"x":0,"y":2}]}]},"you":{"id":"s"}}`, 200},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
		if w.Code != tt.status {
			t.Fatalf("%s %s: status %d", tt.method, tt.path, w.Code)
		}
		if tt.path == "/move" && tt.status == 200 && !strings.Contains(w.Body.String(), `"move":"right"`) {
			t.Fatal(w.Body.String())
		}
		if tt.path == "/" {
			var info map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil || info["apiversion"] != "1" || info["version"] == "" {
				t.Fatal(w.Body.String())
			}
		}
	}
}

func TestArenaOpenings(t *testing.T) {
	s := service{quiet: true, searchSlot: make(chan struct{}, 1),
		opts: snake.Options{Style: "search", Budget: 75 * time.Millisecond, NodeLimit: 1200}}
	h := s.handler()
	delta := map[string]snake.Point{"up": {Y: 1}, "right": {X: 1}, "down": {Y: -1}, "left": {X: -1}}
	for _, mode := range []string{"duels", "standard", "royale", "constrictor"} {
		t.Run(mode, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("testdata", "arena", mode+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var game snake.Request
			if err := json.Unmarshal(body, &game); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("POST", "/move", bytes.NewReader(body)))
			var reply struct{ Move string }
			if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil || w.Code != 200 {
				t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
			}
			step, ok := delta[reply.Move]
			head := game.You.Body[0]
			x, y := head.X+step.X, head.Y+step.Y
			if !ok || x < 0 || x >= game.Board.Width || y < 0 || y >= game.Board.Height {
				t.Fatalf("invalid opening move %q from %v", reply.Move, head)
			}
			for _, player := range game.Board.Snakes {
				for _, segment := range player.Body[:len(player.Body)-1] {
					if segment.X == x && segment.Y == y {
						t.Fatalf("opening move %q hits a body", reply.Move)
					}
				}
			}
		})
	}
}
