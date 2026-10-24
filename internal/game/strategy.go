package game

// The enemy uses the original O2 parameter rows. The path search is expressed
// with Go arrays instead of the original shared direction and bytecode buffers.

func (s *State) think() {
	s.initializeBrain()
	for n := 0; n < 20; n++ {
		s.AICursor = (s.AICursor + 1) % 20
		q := &s.Squads[s.AICursor]
		if !q.Alive() || s.originalSide(q) != Enemy || q.Moving() {
			continue
		}
		// 790e stops on the first idle enemy even when that squad is engaged.
		// The battle check happens after selection, rather than in the scan.
		if q.BattleID == 0 {
			s.originalEnemyOrders(q)
		}
		return
	}
}

func (s *State) strategicValues() (values [200]int) {
	fort := 30
	if s.Rules.FortFood {
		fort = 80
	}
	fort += s.Rules.Recruitment
	village := 5
	if s.Rules.VillageFood {
		village = 50
		if !s.Rules.FortFood {
			village = 90
		}
	}
	bridge := 25
	for _, t := range s.World.Tiles {
		if t == 3 {
			fort -= 20
		}
		if t == 4 {
			village -= 10
		}
		if t == 9 || t == 10 {
			bridge -= 5
		}
	}
	fort = max(10, fort)
	village = max(5, village)
	bridge = max(5, bridge)
	for cell, t := range s.World.Tiles {
		switch t {
		case 3:
			values[cell] = fort
		case 4:
			values[cell] = village
		case 5, 6:
			values[cell] = 2
		case 9, 10:
			values[cell] = bridge
		}
	}
	for _, f := range s.Flags {
		if f.Side == Player {
			values[f.Position.Cell()] += 130
		}
	}
	return
}

// 8425: direct pursuit does not cross rivers or mountains that the selected
// leader cannot negotiate quickly. The strategic path search can go around.
func (s *State) clearApproach(from, to int) bool {
	for from != to {
		x, y := from%10, from/10
		x += sign(to%10 - x)
		y += sign(to/10 - y)
		from = y*10 + x
		t := s.World.Tiles[from]
		if IsWaterTile(t) || IsMountainTile(t) && s.Leader.Parameters[15] >= 8 {
			return false
		}
	}
	return true
}

// FindRoute searches the original 10x20 map graph with terrain and influence
// costs, then emits the fine-grained movement steps used by the simulation.
func (s *State) FindRoute(from, to Point, side Side) ([]Point, error) {
	if side == Enemy {
		return s.originalEnemyRoute(from, to)
	}
	start, goal := from.Cell(), to.Cell()
	var cost [200]int
	var previous [200]int
	var visited [200]bool
	for i := range cost {
		cost[i] = 1 << 30
		previous[i] = -1
	}
	cost[start] = 0
	for n := 0; n < 200; n++ {
		c := -1
		for i := range cost {
			if !visited[i] && (c < 0 || cost[i] < cost[c]) {
				c = i
			}
		}
		if c < 0 || cost[c] == 1<<30 {
			break
		}
		if c == goal {
			break
		}
		visited[c] = true
		for _, next := range neighbors(c) {
			t := s.World.Tiles[next]
			v := 1
			if t == 3 {
				v = 3
			} else if IsWaterTile(t) {
				v = s.Rules.WaterSpeed
				if v > 10 {
					v *= 2
				}
			} else if IsMountainTile(t) {
				v = s.Rules.MountainSpeed
				if side == Enemy {
					v = s.Leader.Parameters[15]
				}
				if v > 14 {
					v *= 2
				}
			} else if IsForestTile(t) {
				v = s.Rules.ForestSpeed
				if side == Enemy {
					v = s.Leader.Parameters[16]
				}
			}
			if side == Enemy {
				danger := s.Leader.Parameters[18] - s.Influence[next]
				if danger > 0 {
					v += (danger >> 1) + 3
				}
			}
			if cost[c]+v < cost[next] {
				cost[next] = cost[c] + v
				previous[next] = c
			}
		}
	}
	var cells []int
	for c := goal; c != start; c = previous[c] {
		if c < 0 {
			break
		}
		cells = append(cells, c)
	}
	var points []Point
	for i := len(cells) - 1; i >= 0; i-- {
		if i == 0 {
			points = append(points, to)
		} else {
			points = append(points, cellCenter(cells[i]))
		}
	}
	if len(points) == 0 {
		points = append(points, to)
	}
	return TraceRoute(from, points)
}

// AutoCommand is used by the attract demo and deterministic headless smoke runs.
// Human control uses Move, which follows the path drawn on the map.
func (s *State) AutoCommand(side Side) {
	for i := range s.Squads {
		q := &s.Squads[i]
		if !q.Alive() || q.Side != side || q.BattleID != 0 || q.Moving() {
			continue
		}
		target := -1
		d := 99999
		for j, f := range s.Flags {
			if f.Side != side {
				v := originalDistance(q.Position, f.Position)
				if v < d {
					d = v
					target = j
				}
			}
		}
		if target >= 0 {
			r, err := s.FindRoute(q.Position, s.Flags[target].Position, side)
			if err == nil {
				capacity := s.routeCapacity(q.ID)
				if capacity == 0 {
					continue
				}
				if len(r) > capacity {
					r = r[:capacity]
				}
				s.installDrawnRoute(q, r)
			}
		}
	}
}
