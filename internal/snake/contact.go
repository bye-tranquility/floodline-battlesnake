package snake

import "time"

// Revisit the last completed depth in confined positions or near the enemy
// head. Return the ordinary move if the extra search runs out of nodes or time.
func (s *duelSearch) confirmTactics(p position, depth int, preferred direction) direction {
	if !p.plainDuelGrid || depth == 0 || len(p.snakes[0].body) < 6 || !time.Now().Before(s.deadline) {
		return preferred
	}
	p.occupied(s.blocked)
	us, rival := p.snakes[0].body[0], p.snakes[1].body[0]
	distance := abs(us%p.width-rival%p.width) + abs(us/p.width-rival/p.width)
	if distance > 6 && !s.tightDuel(p) {
		return preferred
	}
	s.tacticalDepth = depth + 1
	s.limit += s.nodes
	move, _, complete := s.best(p, depth, 0, -infinity, infinity, preferred, 1)
	if complete {
		return move
	}
	return preferred
}

// At the horizon, extend one simultaneous turn if either player is forced
// or if both players can move their heads to the same cell.
func (s *duelSearch) forcedOrContested(p position) bool {
	me, foe := p.snakes[0], p.snakes[1]
	if len(me.body) < 6 && len(foe.body) < 6 {
		return false
	}
	p.occupied(s.blocked)
	var choices [2][4]int
	var sizes [2]int
	for i, who := range p.snakes {
		for _, c := range p.next[who.body[0]] {
			if c < 0 || s.blocked[c] || p.nextHealth(who, c) <= 0 {
				continue
			}
			choices[i][sizes[i]] = c
			sizes[i]++
		}
		if sizes[i] <= 1 {
			return true
		}
	}
	for _, a := range choices[0][:sizes[0]] {
		for _, b := range choices[1][:sizes[1]] {
			if a == b {
				return true
			}
		}
	}
	return false
}
