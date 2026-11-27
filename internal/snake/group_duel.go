package snake

import "time"

type groupDuel struct {
	*analysis
	deadline time.Time
	nodes    int
}

func (g *groupDuel) active(p position) bool {
	me := p.snakes[0]
	h := me.body[0]
	if min(p.distanceBetween(h, p.snakes[1].body[0]), p.distanceBetween(h, p.snakes[2].body[0])) <= 4 {
		return true
	}
	p.occupied(g.blocked)
	for _, c := range p.next[h] {
		if c >= 0 && !g.blocked[c] && g.walk(p, c, g.own).area <= len(me.body)+4 {
			return true
		}
	}
	return false
}

// Project the distant opponent a turn at a time. Keep its moving body on the
// board even when the two nearer snakes fight over a different part of it.
func (g *groupDuel) distant(p position, who int) direction {
	enemy := p.snakes[who]
	if len(enemy.body) == 0 {
		return up
	}
	head := enemy.body[0]
	best, m := -infinity, up
	for d, c := range p.next[head] {
		if c < 0 || g.blocked[c] || p.nextHealth(enemy, c) <= 0 {
			continue
		}
		exits := 0
		for _, v := range p.next[c] {
			if v >= 0 && !g.blocked[v] {
				exits++
			}
		}
		value := 15*exits - p.centerDistance(c)
		if len(enemy.body) > 1 && p.next[enemy.body[1]][d] == head {
			value += 7
		}
		if p.food[c] {
			value += 70
		}
		for i, s := range p.snakes {
			if i == who || len(s.body) < len(enemy.body) || len(s.body) == 0 {
				continue
			}
			if p.distanceBetween(s.body[0], c) == 1 {
				value -= 170
			}
		}
		if value > best {
			best, m = value, direction(d)
		}
	}
	return m
}

// Count the near territory each other head can actually reach on the next
// six turns. The whole connected component and the nearest meal still
// matter after that horizon.
func (g *groupDuel) utility(p position) int {
	me := p.snakes[0]
	n := len(me.body)
	if n == 0 {
		if len(p.snakes[1].body) == 0 || len(p.snakes[2].body) == 0 {
			return -victory / 2
		}
		return -victory
	}
	var blocked, food cells11
	var f, seen [3]cells11
	longest := 0
	for i, s := range p.snakes {
		if len(s.body) == 0 {
			continue
		}
		f[i].add(s.body[0])
		seen[i] = f[i]
		for _, c := range s.body[:len(s.body)-1] {
			blocked.add(c)
		}
		if i > 0 {
			longest = max(longest, len(s.body))
		}
	}
	if longest == 0 {
		return victory
	}
	for c, v := range p.food {
		if v {
			food.add(c)
		}
	}
	area, exits, meal, land := 1, 0, infinity, 0
	for step := 1; !f[0].empty(); step++ {
		for i := range f {
			f[i] = f[i].neighbors().without(blocked.union(seen[i]))
		}
		if step == 1 {
			exits = f[0].count()
		}
		if step <= 6 {
			lost := seen[1].union(seen[2])
			for i := 1; i < 3; i++ {
				if len(p.snakes[i].body) >= n {
					lost = lost.union(f[i])
				}
			}
			land += f[0].without(lost).count() * (7 - step)
		}
		area += f[0].count()
		if meal == infinity && !f[0].both(food).empty() {
			meal = step
		}
		for i := range seen {
			seen[i] = seen[i].union(f[i])
		}
	}
	room := region{area: area, exits: exits, food: meal}
	score := spaceScore(room, n) + land + nutrition(room, me.health, n-longest) + min(6, max(-6, n-longest))*28 - p.centerDistance(me.body[0])
	if len(p.snakes[1].body) == 0 || len(p.snakes[2].body) == 0 {
		score += 800
	}
	return score
}

func (g *groupDuel) search(p position, depth, alpha, beta int, wanted direction, root *[4]int) (direction, int, bool) {
	if len(p.snakes[0].body) == 0 || len(p.snakes[1].body)+len(p.snakes[2].body) == 0 || depth == 0 {
		return up, g.utility(p), true
	}
	if g.nodes >= 1800 || (g.nodes%8 == 0 && !time.Now().Before(g.deadline)) {
		return wanted, 0, false
	}
	h := p.snakes[0].body[0]
	near, far := 1, 2
	if len(p.snakes[1].body) == 0 || len(p.snakes[2].body) > 0 && p.distanceBetween(h, p.snakes[2].body[0]) < p.distanceBetween(h, p.snakes[1].body[0]) {
		near, far = 2, 1
	}
	p.occupied(g.blocked)
	our := p.choices(p.snakes[0], g.blocked, wanted)
	rival := p.choices(p.snakes[near], g.blocked, -1)
	moves := [3]direction{}
	moves[far] = g.distant(p, far)
	best, move := -infinity, our.moves[0]
	for _, m := range our.moves[:our.count] {
		worst := infinity
		moves[0] = m
		for _, r := range rival.moves[:rival.count] {
			moves[near] = r
			g.nodes++
			child := p.advance(moves[:])
			_, v, ok := g.search(child, depth-1, alpha, min(worst, beta), -1, nil)
			if !ok {
				return move, 0, false
			}
			worst = min(worst, v)
			if worst <= alpha {
				break
			}
		}
		if root != nil {
			root[m] = worst
		}
		if worst > best {
			best, move = worst, m
		}
		if root == nil {
			alpha = max(alpha, best)
		}
		if alpha >= beta {
			break
		}
	}
	return move, best, true
}
func (g *groupDuel) choose(p position, prior direction) direction {
	chosen := prior
	for d := 1; d <= 5; d++ {
		res := [4]int{-infinity, -infinity, -infinity, -infinity}
		move, best, ok := g.search(p, d, -infinity, infinity, chosen, &res)
		if !ok {
			break
		}
		chosen = prior
		if best >= res[prior]+65 {
			chosen = move
		}
	}
	return chosen
}
