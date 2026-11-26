package snake

import "time"

const victory = 100_000

type duelSearch struct {
	*analysis
	escape       pocketEscape
	deadline     time.Time
	limit, nodes int
}

type moveList struct {
	moves [4]direction
	count int
}

func (p position) choices(s serpent, blocked []bool, preferred direction) moveList {
	var out moveList
	var ranks [4]int
	for m, c := range p.next[s.body[0]] {
		if c < 0 || blocked[c] || p.nextHealth(s, c) <= 0 {
			continue
		}
		rank := -p.centerDistance(c) - p.damage[c]
		if p.food[c] {
			rank += 30
		}
		if direction(m) == preferred {
			rank += 100
		}
		i := out.count
		for i > 0 && rank > ranks[i-1] {
			ranks[i] = ranks[i-1]
			out.moves[i] = out.moves[i-1]
			i--
		}
		out.moves[i], ranks[i] = direction(m), rank
		out.count++
	}
	// A trapped opponent still makes a move, and its body can kill us on that turn.
	if out.count == 0 {
		return moveList{[4]direction{up, right, down, left}, 4}
	}
	return out
}

func (s *duelSearch) evaluate(p position) int {
	me, them := p.snakes[0], p.snakes[1]
	p.occupied(s.blocked)
	our := s.walk(p, me.body[0], s.own)
	their := s.walk(p, them.body[0], s.other)
	gap := len(me.body) - len(them.body)
	land := territory(s.own, s.other, gap) - territory(s.other, s.own, -gap)
	score := spaceScore(our, len(me.body)) - spaceScore(their, len(them.body))
	score += land*3 + p.centerDistance(them.body[0]) - p.centerDistance(me.body[0])
	if p.constrictor {
		return score + land*2 + 3*(our.area-their.area)
	}
	if !p.royale {
		score += s.escape.score(p, me, them, our, s.other) - s.escape.score(p, them, me, their, s.own)
	}
	score += min(3, max(-3, gap))*95 + gap*8
	score += nutrition(our, me.health, gap) - nutrition(their, them.health, -gap)
	score += max(0, 40-them.health)*2 - max(0, 40-me.health)*2
	score += min(30, their.safe)*3 - min(30, our.safe)*3
	return score
}

// Both players choose from the same board. Apply each pair of moves together,
// maximize the worst reply, and propagate bounds between simultaneous turns.
func (s *duelSearch) best(p position, depth, ply, alpha, beta int, preferred direction) (direction, int, bool) {
	p.occupied(s.blocked)
	our := p.choices(p.snakes[0], s.blocked, preferred)
	their := p.choices(p.snakes[1], s.blocked, -1)
	best, bestMove := -infinity, our.moves[0]
	for _, m := range our.moves[:our.count] {
		worst := infinity
		for _, reply := range their.moves[:their.count] {
			if s.nodes >= s.limit || (s.nodes%8 == 0 && !time.Now().Before(s.deadline)) {
				return bestMove, 0, false
			}
			s.nodes++
			next := p.advance([]direction{m, reply})
			us, foe := len(next.snakes[0].body) > 0, len(next.snakes[1].body) > 0
			score := 0
			switch {
			case !us && !foe:
				if !p.constrictor && !p.royale {
					// When there is a playable escape, keep the opportunity to
					// win. A mutual elimination is still better than certain loss.
					score = -1800
				}
			case !us:
				score = -victory + ply + 1
			case !foe:
				score = victory - ply - 1
			case depth == 1:
				score = s.evaluate(next)
			default:
				var complete bool
				_, score, complete = s.best(next, depth-1, ply+1, alpha, min(beta, worst), -1)
				if !complete {
					return bestMove, 0, false
				}
			}
			worst = min(worst, score)
			if worst <= alpha {
				break
			}
		}
		if worst > best {
			best, bestMove = worst, m
		}
		alpha = max(alpha, best)
		if alpha >= beta {
			break
		}
	}
	return bestMove, best, true
}
