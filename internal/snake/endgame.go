package snake

// Feature order is shared with experiments/duel-endgame/fit.py.
type endgameFeatures [23]int

func (p position) distanceBetween(first, second int) int {
	return abs(first%p.width-second%p.width) + abs(first/p.width-second/p.width)
}

func (p position) distanceFromWall(cell int) int {
	x, y := cell%p.width, cell/p.width
	return min(x, y, p.width-1-x, p.height-1-y)
}

func (p position) endgameFeatures(us, opponent serpent, our, their region, score, land int) endgameFeatures {
	length, otherLength := len(us.body), len(opponent.body)
	head, otherHead := us.body[0], opponent.body[0]
	return endgameFeatures{
		score, length, otherLength, length - otherLength, us.health, opponent.health,
		min(30, our.food), min(30, their.food), our.exits, their.exits,
		our.area, their.area, 100 * our.area / length, 100 * their.area / otherLength,
		land, p.distanceFromWall(head), p.distanceFromWall(otherHead), p.distanceBetween(head, otherHead),
		p.distanceBetween(head, us.body[length-1]), p.distanceBetween(otherHead, opponent.body[otherLength-1]),
		length + otherLength, min(50, us.health-min(101, our.food)), min(50, opponent.health-min(101, their.food)),
	}
}

func endgameAdjustment(p position, score, land int, our, their region) int {
	us, opponent := p.snakes[0], p.snakes[1]
	first := p.endgameFeatures(us, opponent, our, their, score, land)
	second := p.endgameFeatures(opponent, us, their, our, -score, -land)
	// Leaves are in hundredths of a search point. Take the antisymmetric
	// mean of both viewpoints and apply one quarter of the fitted correction.
	return (endgameValue(first) - endgameValue(second)) / 800
}

func endgameValue(features endgameFeatures) int {
	total := 0
	for _, tree := range endgameTrees {
		branch, leaf := 2, 6
		if features[tree[0]] > tree[1] {
			branch, leaf = 4, 8
		}
		if features[tree[branch]] > tree[branch+1] {
			leaf++
		}
		total += tree[leaf]
	}
	return total
}
