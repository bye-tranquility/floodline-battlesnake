package snake

import "time"

const victory = 100_000

type duelSearch struct {
	*analysis
	escape        pocketEscape
	history       map[uint64]lineOrder
	deadline      time.Time
	limit, nodes  int
	tacticalDepth int
	reuseMoves    bool
}

type lineOrder struct {
	ours    direction
	replies [4]direction
}

func (s *duelSearch) deepen(p position, preferred direction) (direction, int) {
	completed := 0
	for depth := 1; depth <= 16; depth++ {
		move, score, ok := s.best(p, depth, 0, -infinity, infinity, preferred, 1)
		if !ok {
			break
		}
		preferred, completed = move, depth
		if abs(score) >= victory-depth {
			return preferred, completed
		}
	}
	return s.confirmTactics(p, completed, preferred), completed
}

type moveList struct {
	moves [4]direction
	count int
}

func (moves moveList) prefer(hint direction) moveList {
	for i, m := range moves.moves[:moves.count] {
		if m == hint {
			copy(moves.moves[1:i+1], moves.moves[:i])
			moves.moves[0] = m
			break
		}
	}
	return moves
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
	gap := len(me.body) - len(them.body)
	var our, their region
	var land int
	if p.plainDuelGrid {
		our, their, land = s.duelRegions(p, gap)
	} else {
		p.occupied(s.blocked)
		our = s.walk(p, me.body[0], s.own)
		their = s.walk(p, them.body[0], s.other)
		weight := 3
		if p.constrictor {
			weight = 5
		}
		land = weight * (territory(s.own, s.other, gap) - territory(s.other, s.own, -gap))
	}
	score := spaceScore(our, len(me.body)) - spaceScore(their, len(them.body))
	score += land + p.centerDistance(them.body[0]) - p.centerDistance(me.body[0])
	if p.constrictor {
		return score + 3*(our.area-their.area)
	}
	if !p.royale {
		score += s.escape.score(p, me, them, our, s.other) - s.escape.score(p, them, me, their, s.own)
	}
	score += min(3, max(-3, gap))*95 + gap*8
	if !p.royale {
		// A fourth length deficit still makes it easier for the opponent to
		// take the next food and seal a corridor. Keep valuing recovery.
		score += 60 * (min(9, max(-9, gap)) - min(3, max(-3, gap)))
	}
	score += nutrition(our, me.health, gap) - nutrition(their, them.health, -gap)
	score += max(0, 40-them.health)*2 - max(0, 40-me.health)*2
	score += min(30, their.safe)*3 - min(30, our.safe)*3
	// These endgames have enough training examples for both length ranges.
	if p.plainDuelGrid && len(me.body)+len(them.body) >= 24 && abs(gap) <= 13 {
		score += endgameAdjustment(p, score, land, our, their)
	}
	return score
}

// Both players choose from the same board. Apply each pair of moves together,
// maximize the worst reply, and propagate bounds between simultaneous turns.
func (s *duelSearch) best(p position, depth, ply, alpha, beta int, preferred direction, path uint64) (direction, int, bool) {
	var joint jointTurn
	if s.reuseMoves {
		joint.prepare(p, s.blocked)
	} else {
		p.occupied(s.blocked)
	}
	our := p.choices(p.snakes[0], s.blocked, preferred)
	their := p.choices(p.snakes[1], s.blocked, -1)
	// Earlier depths of this request suggest which legal moves to try first.
	// Every score is recomputed at the current depth.
	hints, seen := s.history[path]
	if seen && preferred < 0 {
		our = our.prefer(hints.ours)
	}
	if !seen {
		hints.replies = [4]direction{-1, -1, -1, -1}
	}
	best, bestMove := -infinity, our.moves[0]
	for _, m := range our.moves[:our.count] {
		worst := infinity
		replies := their
		if seen {
			replies = their.prefer(hints.replies[m])
		}
		for _, reply := range replies.moves[:replies.count] {
			if s.nodes >= s.limit || (s.nodes%8 == 0 && !time.Now().Before(s.deadline)) {
				return bestMove, 0, false
			}
			s.nodes++
			var next position
			var us, foe bool
			if s.reuseMoves {
				next, us, foe = joint.advance(m, reply)
			} else {
				next = p.advance([]direction{m, reply})
				us, foe = len(next.snakes[0].body) > 0, len(next.snakes[1].body) > 0
			}
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
			case depth == 1 && (ply+1 >= s.tacticalDepth || !s.forcedOrContested(next)):
				score = s.evaluate(next)
			default:
				var complete bool
				// Four bits identify one joint turn. The depth limit keeps the full
				// ordered line in one word, including the leading root marker.
				nextPath := path<<4 | uint64(m)<<2 | uint64(reply)
				_, score, complete = s.best(next, max(1, depth-1), ply+1, alpha, min(beta, worst), -1, nextPath)
				if !complete {
					return bestMove, 0, false
				}
			}
			if score < worst {
				worst, hints.replies[m] = score, reply
			}
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
	if s.history != nil {
		hints.ours = bestMove
		s.history[path] = hints
	}
	return bestMove, best, true
}
