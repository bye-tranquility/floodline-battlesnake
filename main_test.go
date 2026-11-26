package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
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
