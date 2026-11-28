package snake

// In an ordinary duel the segment following each new head depends only on
// that snake's direction. Opponent replies share the same successor body.
type duelStep struct {
	cell, health, length int
	meal                 bool
	body                 []int
}
type jointTurn struct {
	stable [2]cells11
	food   cells11
	p      position
	body   [2]cells11
	moves  [2][4]duelStep
}

func (b cells11) has(c int) bool {
	if c < 64 {
		return b.low&(uint64(1)<<c) != 0
	}
	return b.high&(uint64(1)<<(c-64)) != 0
}
func (t *jointTurn) prepare(p position, blocked []bool) {
	t.p = p
	clear(blocked)
	if p.cachedGrid {
		t.food = p.foodMask
	} else {
		for c, v := range p.food {
			if v {
				t.food.add(c)
			}
		}
	}
	for i, s := range p.snakes {
		for k, c := range s.body[:len(s.body)-1] {
			t.body[i].add(c)
			blocked[c] = true
			if k < len(s.body)-2 {
				t.stable[i].add(c)
			}
		}
		for m, c := range p.next[s.body[0]] {
			step := &t.moves[i][m]
			step.cell = c
			if c < 0 {
				continue
			}
			step.health = p.nextHealth(s, c)
			step.length = len(s.body)
			step.meal = p.food[c]
			if step.meal {
				step.length++
			}
		}
	}
}
func (t *jointTurn) alive(i int, own, rival *duelStep) bool {
	if own.cell < 0 || own.health <= 0 || t.body[i].has(own.cell) || own.meal && own.length == 2 {
		return false
	}
	if rival.cell < 0 || rival.health <= 0 {
		return true
	}
	if t.body[1-i].has(own.cell) {
		return false
	}
	if own.cell != rival.cell {
		return true
	}
	return own.length > rival.length && !(rival.meal && rival.length == 2)
}
func (t *jointTurn) serpent(i int, step *duelStep) serpent {
	if step.body == nil {
		old := t.p.snakes[i].body
		b := make([]int, step.length)
		b[0] = step.cell
		copy(b[1:], old[:len(old)-1])
		if step.meal {
			b[len(b)-1] = b[len(b)-2]
		}
		step.body = b
	}
	return serpent{body: step.body, health: step.health}
}
func (t *jointTurn) advance(ours, reply direction) (position, bool, bool) {
	a, b := &t.moves[0][ours], &t.moves[1][reply]
	alive, foe := t.alive(0, a, b), t.alive(1, b, a)
	if !alive || !foe {
		return position{}, alive, foe
	}
	oursMask, theirMask := t.stable[0], t.stable[1]
	if a.meal {
		oursMask = t.body[0]
	}
	if b.meal {
		theirMask = t.body[1]
	}
	if a.length > 1 {
		oursMask.add(a.cell)
	}
	if b.length > 1 {
		theirMask.add(b.cell)
	}
	n := position{
		field:       t.p.field,
		food:        t.p.food,
		cachedGrid:  true,
		foodMask:    t.food,
		blockedMask: oursMask.union(theirMask),
		snakes:      []serpent{t.serpent(0, a), t.serpent(1, b)},
	}
	if a.meal || b.meal {
		n.food = append([]bool(nil), n.food...)
		if a.meal {
			n.food[a.cell] = false
			var cell cells11
			cell.add(a.cell)
			n.foodMask = n.foodMask.without(cell)
		}
		if b.meal {
			n.food[b.cell] = false
			var cell cells11
			cell.add(b.cell)
			n.foodMask = n.foodMask.without(cell)
		}
	}
	return n, true, true
}
