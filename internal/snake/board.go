package snake

import (
	"errors"
	"strings"
)

type Point struct{ X, Y int }

type WireSnake struct {
	ID, Name string
	Health   int
	Body     []Point
}

type Request struct {
	Game struct {
		ID, Map, Source string
		Timeout         int
		Ruleset         struct {
			Name     string
			Settings struct{ HazardDamagePerTurn *int }
		}
	}
	Turn  int
	Board struct {
		Width, Height int
		Food, Hazards []Point
		Snakes        []WireSnake
	}
	You WireSnake
}

type direction int

const (
	up direction = iota
	right
	down
	left
)

var names = [...]string{"up", "right", "down", "left"}

type field struct {
	width, height int
	next          [][4]int
	damage        []int
	constrictor   bool
}

type serpent struct {
	body   []int
	health int
}

// The controlled snake is always at index zero. Empty bodies mark eliminations.
type position struct {
	*field
	snakes []serpent
	food   []bool
}

func (r *Request) position() (position, error) {
	b := &r.Board
	if b.Width < 1 || b.Height < 1 || b.Width > 64 || b.Height > 64 || len(b.Snakes) == 0 || r.You.ID == "" {
		return position{}, errors.New("invalid board dimensions or snake identity")
	}
	n := b.Width * b.Height
	f := &field{width: b.Width, height: b.Height, next: make([][4]int, n), damage: make([]int, n)}
	f.constrictor = strings.Contains(strings.ToLower(r.Game.Map+" "+r.Game.Ruleset.Name), "constrictor")
	for c := range f.next {
		x, y := c%b.Width, c/b.Width
		adj := [4]int{-1, -1, -1, -1}
		if y+1 < b.Height {
			adj[up] = c + b.Width
		}
		if x+1 < b.Width {
			adj[right] = c + 1
		}
		if y > 0 {
			adj[down] = c - b.Width
		}
		if x > 0 {
			adj[left] = c - 1
		}
		f.next[c] = adj
	}
	cell := func(q Point) (int, bool) {
		return q.Y*b.Width + q.X, q.X >= 0 && q.Y >= 0 && q.X < b.Width && q.Y < b.Height
	}
	p := position{field: f, food: make([]bool, n)}
	for _, q := range b.Food {
		if c, ok := cell(q); ok {
			p.food[c] = true
		}
	}
	damage := 14
	if v := r.Game.Ruleset.Settings.HazardDamagePerTurn; v != nil {
		damage = max(0, *v)
	}
	for _, q := range b.Hazards {
		if c, ok := cell(q); ok {
			f.damage[c] += damage
		}
	}
	me := -1
	for i, s := range b.Snakes {
		if len(s.Body) == 0 {
			return position{}, errors.New("empty snake")
		}
		body := make([]int, len(s.Body))
		for j, q := range s.Body {
			c, ok := cell(q)
			if !ok {
				return position{}, errors.New("snake outside board")
			}
			body[j] = c
		}
		p.snakes = append(p.snakes, serpent{body, s.Health})
		if s.ID == r.You.ID {
			me = i
		}
	}
	if me < 0 {
		return position{}, errors.New("you.id absent from board")
	}
	p.snakes[0], p.snakes[me] = p.snakes[me], p.snakes[0]
	return p, nil
}

// occupied marks exactly the old segments that remain after the next movement.
// Removing one segment also handles stacked tails and all legal Constrictor trails.
func (p position) occupied(dst []bool) {
	clear(dst)
	for _, s := range p.snakes {
		for i := 0; i+1 < len(s.body); i++ {
			dst[s.body[i]] = true
		}
	}
}

func (p position) nextHealth(s serpent, c int) int {
	if p.food[c] {
		return 100
	}
	return s.health - 1 - p.damage[c]
}

// advance resolves known moves simultaneously. Random future food and new
// hazard rows are left to the match engine and arrive in the next request.
func (p position) advance(moves []direction) position {
	next := position{field: p.field, food: p.food, snakes: make([]serpent, len(p.snakes))}
	ate := false
	for i, s := range p.snakes {
		if len(s.body) == 0 {
			continue
		}
		h := p.next[s.body[0]][moves[i]]
		if h < 0 {
			continue
		}
		hp := p.nextHealth(s, h)
		if hp <= 0 {
			continue
		}
		growing := p.food[h]
		length := len(s.body)
		if growing {
			length++
		}
		body := make([]int, length, length+1)
		body[0] = h
		copy(body[1:], s.body[:len(s.body)-1])
		if growing {
			body[length-1] = body[length-2]
			if !ate {
				next.food = append([]bool(nil), p.food...)
				ate = true
			}
			next.food[h] = false
		}
		next.snakes[i] = serpent{body, hp}
	}
	dead := make([]bool, len(next.snakes))
	for i, s := range next.snakes {
		if len(s.body) == 0 {
			continue
		}
		for j, other := range next.snakes {
			if len(other.body) == 0 {
				continue
			}
			if i != j && s.body[0] == other.body[0] && len(s.body) <= len(other.body) {
				dead[i] = true
			}
			for _, c := range other.body[1:] {
				if c == s.body[0] {
					dead[i] = true
					break
				}
			}
		}
	}
	for i, s := range next.snakes {
		if dead[i] {
			next.snakes[i] = serpent{}
			continue
		}
		if p.constrictor && len(s.body) > 0 {
			last := len(s.body) - 1
			if last == 0 || s.body[last] != s.body[last-1] {
				next.snakes[i].body = append(s.body, s.body[last])
			}
			next.snakes[i].health = 100
		}
	}
	return next
}
