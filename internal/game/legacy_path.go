package game

import "fmt"

// The original AI uses these direction numbers and expansion order. Its search
// stops when the target is discovered and stores only the first half of a path.
// This is intentionally different from replacing it with an optimal A* path.
var legacyDirections = [9]int{0, 1, 11, 10, 9, -1, -11, -10, -9}
var legacyDirectionOrder = [8]int{0, 1, -1, 2, -2, 3, -3, 4}

func word16(v int) int { return int(int16(v)) }

// legacyPlan is a native translation of overlay 5:85ce, excluding the shared
// route-pool allocator. Original-code fixtures cover its expansion, ties,
// signed costs, early termination, reconstruction and half-path truncation.
func legacyPlan(start, goal int, costs, influence [200]int, risk int) ([]byte, bool) {
	path, ok := legacySearch(start, goal, costs, influence, risk)
	if len(path) > 1 {
		path = path[:len(path)>>1]
	}
	return path, ok
}

func legacySearch(start, goal int, costs, influence [200]int, risk int) ([]byte, bool) {
	return legacySearchWithFrontier(start, goal, costs, influence, risk, nil)
}

func legacySearchWithFrontier(start, goal int, costs, influence [200]int, risk int, write func(int, int)) ([]byte, bool) {
	if start < 0 || start >= 200 || goal < 0 || goal >= 200 {
		return nil, false
	}
	for i := range costs {
		penalty := word16(risk - influence[i])
		if penalty > 0 {
			costs[i] = word16(costs[i] + (penalty >> 1) + 3)
		}
	}
	var direction, distance [200]int
	direction[start] = 9
	frontier := []int{start}
	found := false
	for iterations := 0; len(frontier) > 0 && !found; iterations++ {
		if iterations > 2000000 {
			return nil, false
		}
		// The DOS loop compares all queue entries except the last one.
		// Preserve that boundary and its first-entry tie behavior.
		best, minimum := 0, 999
		for i := 0; i < len(frontier)-1; i++ {
			if distance[frontier[i]] < minimum {
				best = i
				minimum = distance[frontier[i]]
			}
		}
		cell := frontier[best]
		for i := best; i < len(frontier)-1; i++ {
			frontier[i] = frontier[i+1]
			if write != nil {
				write(i, frontier[i])
			}
		}
		frontier = frontier[:len(frontier)-1]
		column, targetColumn := cell%10, goal%10
		initial := 0
		if goal < cell {
			if targetColumn == column {
				initial = 7
			} else if targetColumn < column {
				if column-10 < targetColumn {
					initial = 5
				} else {
					initial = 6
				}
			} else {
				initial = 8
			}
		} else if targetColumn == column {
			initial = 3
		} else if column < targetColumn {
			if targetColumn < column+10 {
				initial = 1
			} else {
				initial = 2
			}
		} else {
			initial = 4
		}
		for _, offset := range legacyDirectionOrder {
			d := initial + offset
			if d > 8 {
				d -= 8
			}
			if d < 1 {
				d += 8
			}
			if cell < 10 && d >= 6 || cell >= 190 && !(d > 4 || d < 2) || column == 0 && !(d < 4 || d > 6) || column == 9 && (d == 8 || d == 1 || d == 2) {
				continue
			}
			next := cell + legacyDirections[d]
			candidate := word16(distance[cell] + costs[next])
			if direction[next] == 0 || direction[next] < 9 && candidate < distance[next] {
				direction[next] = d
				distance[next] = candidate
				frontier = append(frontier, next)
				if write != nil {
					write(len(frontier)-1, next)
				}
			}
			if next == goal {
				found = true
				break
			}
		}
	}
	if !found {
		return nil, false
	}
	var reverse []byte
	for cell := goal; cell != start; {
		if len(reverse) >= 99 {
			return nil, false
		}
		d := direction[cell]
		if d < 1 || d > 8 {
			return nil, false
		}
		reverse = append(reverse, byte(d))
		cell -= legacyDirections[d]
	}
	path := make([]byte, len(reverse))
	for i, d := range reverse {
		path[len(path)-1-i] = d
	}
	return path, true
}

func (s *State) legacyPathCosts() (costs [200]int) {
	water, mountain := s.Rules.WaterSpeed, s.Leader.Parameters[15]
	if water > 10 {
		water *= 2
	}
	if mountain > 14 {
		mountain *= 2
	}
	for i, tile := range s.World.Tiles {
		switch {
		case IsWaterTile(tile):
			costs[i] = water
		case IsMountainTile(tile):
			costs[i] = mountain
		case IsForestTile(tile):
			costs[i] = s.Leader.Parameters[16]
		case tile == 3:
			costs[i] = 3
		default:
			costs[i] = 1
		}
	}
	return
}

func (s *State) originalEnemyRoute(from, to Point) ([]Point, error) {
	directions, ok := legacyPlan(from.Cell(), to.Cell(), s.legacyPathCosts(), s.Influence, s.Leader.Parameters[18])
	if !ok {
		return nil, fmt.Errorf("original AI search could not reach the destination")
	}
	cell := from.Cell()
	points := make([]Point, 0, len(directions))
	for _, d := range directions {
		cell += legacyDirections[d]
		points = append(points, cellCenter(cell))
	}
	if len(points) == 0 {
		points = append(points, to)
	} else if cell == to.Cell() {
		points[len(points)-1] = to
	}
	return TraceRoute(from, points)
}
