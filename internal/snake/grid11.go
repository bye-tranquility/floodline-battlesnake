package snake

import "math/bits"

// An 11-by-11 board fits in two machine words. Bit y*11+x represents (x,y).
type cells11 struct{ low, high uint64 }

func (b *cells11) add(c int) {
	if c < 64 {
		b.low |= 1 << c
	} else {
		b.high |= 1 << (c - 64)
	}
}

func (b cells11) without(other cells11) cells11 {
	return cells11{b.low &^ other.low, b.high &^ other.high}
}

func (b cells11) union(other cells11) cells11 {
	return cells11{b.low | other.low, b.high | other.high}
}

func (b cells11) both(other cells11) cells11 {
	return cells11{b.low & other.low, b.high & other.high}
}

func (b cells11) count() int { return bits.OnesCount64(b.low) + bits.OnesCount64(b.high) }

var left11, right11 = func() (cells11, cells11) {
	var left, right cells11
	for y := 0; y < 11; y++ {
		left.add(11 * y)
		right.add(11*y + 10)
	}
	return left, right
}()

func (b cells11) shiftUp(n uint) cells11 {
	const validHigh = 1<<(121-64) - 1
	return cells11{b.low << n, (b.high<<n | b.low>>(64-n)) & validHigh}
}

func (b cells11) shiftDown(n uint) cells11 {
	return cells11{b.low>>n | b.high<<(64-n), b.high >> n}
}

func (b cells11) neighbors() cells11 {
	west := b.without(left11).shiftDown(1)
	east := b.without(right11).shiftUp(1)
	return west.union(east).union(b.shiftDown(11)).union(b.shiftUp(11))
}

func (b cells11) empty() bool { return b.low == 0 && b.high == 0 }

func (b cells11) writeArrival(dst []int, turn int) {
	for mask := b.low; mask != 0; mask &= mask - 1 {
		dst[bits.TrailingZeros64(mask)] = turn
	}
	for mask := b.high; mask != 0; mask &= mask - 1 {
		dst[64+bits.TrailingZeros64(mask)] = turn
	}
}

func fillArrivals(blocked, start cells11, dst []int) {
	for i := range dst {
		dst[i] = infinity
	}
	front, seen := start, start
	for step := 0; !front.empty(); step++ {
		front.writeArrival(dst, step)
		front = front.neighbors().without(blocked.union(seen))
		seen = seen.union(front)
	}
}

// Both frontiers advance one turn at a time. The longer head wins a tie.
// Keep counting reachable space and food after the territory horizon.
func (s *duelSearch) duelRegions(p position, gap int) (region, region, int) {
	me, them := p.snakes[0], p.snakes[1]
	var blocked, food cells11
	for _, snake := range p.snakes {
		for _, c := range snake.body[:len(snake.body)-1] {
			blocked.add(c)
		}
	}
	for c, present := range p.food {
		if present {
			food.add(c)
		}
	}
	var ourStart, theirStart cells11
	ourStart.add(me.body[0])
	theirStart.add(them.body[0])
	us, foe := ourStart, theirStart
	ourSeen, theirSeen := us, foe
	our, their := region{area: 1, food: infinity}, region{area: 1, food: infinity}
	if !us.both(food).empty() {
		our.food = 0
	}
	if !foe.both(food).empty() {
		their.food = 0
	}
	land := 0
	for step := 1; !us.empty() || !foe.empty(); step++ {
		us = us.neighbors().without(blocked.union(ourSeen))
		foe = foe.neighbors().without(blocked.union(theirSeen))
		owned := us.without(theirSeen)
		lost := foe.without(ourSeen)
		if step <= 7 {
			weight := 1
			if step <= 4 {
				weight = 3
			}
			delta := 2 * (owned.count() - lost.count())
			switch {
			case gap > 0:
				delta += us.both(foe).count()
			case gap < 0:
				delta -= us.both(foe).count()
			}
			land += weight * delta
		}
		if step == 1 {
			our.exits, their.exits = us.count(), foe.count()
		}
		if our.food == infinity && !us.both(food).empty() {
			our.food = step
		}
		if their.food == infinity && !foe.both(food).empty() {
			their.food = step
		}
		ourSeen = ourSeen.union(us)
		theirSeen = theirSeen.union(foe)
	}
	our.area, their.area = ourSeen.count(), theirSeen.count()
	// Only the pocket check reads the opponent's complete arrival map.
	if our.needsPocketCheck(len(me.body)) {
		fillArrivals(blocked, theirStart, s.other)
	}
	if their.needsPocketCheck(len(them.body)) {
		fillArrivals(blocked, ourStart, s.own)
	}
	return our, their, land
}
