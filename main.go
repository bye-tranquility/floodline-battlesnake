package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"floodline/internal/snake"
)

var version = "dev"

func value(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}

type service struct {
	opts           snake.Options
	searchSlot     chan struct{}
	captured       sync.Map
	capture, quiet bool
}

func (s *service) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"apiversion": "1", "author": "bye-tranquility", "color": "#157c80", "head": "default", "tail": "default", "version": version})
	})
	for _, path := range []string{"/start", "/end"} {
		mux.HandleFunc("POST "+path, func(w http.ResponseWriter, r *http.Request) {
			if !s.quiet {
				var body snake.Request
				if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&body); err == nil {
					log.Printf("%s game=%q map=%q rules=%q turn=%d", r.URL.Path, body.Game.ID, body.Game.Map, body.Game.Ruleset.Name, body.Turn)
				}
			}
			w.WriteHeader(http.StatusOK)
		})
	}
	mux.HandleFunc("POST /move", s.move)
	return mux
}

func (s *service) move(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	var req snake.Request
	if err != nil || json.Unmarshal(raw, &req) != nil {
		http.Error(w, "invalid game JSON", 400)
		return
	}
	opts := s.opts
	// Keep independent concurrent games responsive on a single core.
	if opts.Style == "search" && len(req.Board.Snakes) == 2 {
		select {
		case s.searchSlot <- struct{}{}:
			defer func() { <-s.searchSlot }()
		default:
			opts.Style = "baseline"
		}
	}
	result, err := snake.Decide(&req, started, opts)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"move": result.Move})
	if s.capture && req.Turn == 0 {
		key := req.Game.Map + "/" + req.Game.Ruleset.Name + "/" + strconv.Itoa(len(req.Board.Snakes))
		if _, wasSet := s.captured.LoadOrStore(key, true); !wasSet {
			log.Printf("sample %s %s", key, raw)
		}
	}
	if !s.quiet && (req.Turn == 0 || req.Turn%40 == 0) {
		log.Printf("move game=%q turn=%d depth=%d nodes=%d elapsed=%s", req.Game.ID, req.Turn, result.Depth, result.Nodes, time.Since(started))
	}
}

func main() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" {
					version = s.Value[:min(12, len(s.Value))]
				}
			}
		}
	}
	version = value("RAILWAY_GIT_COMMIT_SHA", version)
	budget, err := strconv.Atoi(value("SEARCH_MS", "75"))
	if err != nil || budget < 0 || budget > 120 {
		log.Fatal("SEARCH_MS must be 0..120")
	}
	nodes, err := strconv.Atoi(value("SEARCH_NODES", "1200"))
	if err != nil || nodes < 1 {
		log.Fatal("SEARCH_NODES must be positive")
	}
	style := value("STYLE", "search")
	switch style {
	case "search", "baseline", "food", "space", "press":
	default:
		log.Fatal("unknown STYLE")
	}
	s := service{opts: snake.Options{Style: style, Budget: time.Duration(budget) * time.Millisecond, NodeLimit: nodes}, searchSlot: make(chan struct{}, 1), capture: os.Getenv("CAPTURE_SAMPLES") == "1", quiet: os.Getenv("QUIET") == "1"}
	addr := os.Getenv("BIND_HOST") + ":" + value("PORT", "8080")
	log.Printf("Floodline %s listening on %s (%s)", version, addr, style)
	server := http.Server{Addr: addr, Handler: s.handler(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 2 * time.Second}
	log.Fatal(server.ListenAndServe())
}
