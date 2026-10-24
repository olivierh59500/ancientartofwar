package game

// originalPathSlot translates the longest-free-run allocator at 1598. Route
// terminators are occupied bytes; they are not equivalent to free space.
func originalPathSlot(pool *[200]byte) (int, int) {
	best, length := 0, 0
	for start := 0; start < 200; {
		end := start
		for end < 200 && pool[end] == 0 {
			end++
		}
		if end-start > length {
			best, length = start, end-start
		}
		for end < 199 && pool[end] != 0 {
			end++
		}
		if end > 198 {
			break
		}
		start = end
	}
	return best, length
}

// originalStorePath keeps the DOS write boundary, truncation order and shared
// pool semantics. The native guard for a completely full pool prevents a DOS
// out-of-bounds write into the neighboring fine-route buffer.
func originalStorePath(pool *[200]byte, path []byte) (int, bool) {
	slot, free := originalPathSlot(pool)
	if free == 0 {
		return 0, false
	}
	index, written := slot, 0
	for _, direction := range path {
		if index > 197 {
			index = 198
			break
		}
		if pool[index] != 0 {
			index--
			break
		}
		pool[index] = direction
		written++
		index++
	}
	if index < 0 || index >= len(pool) {
		return slot, false
	}
	pool[index] = 99
	if written > 1 {
		cut := slot + (written >> 1)
		pool[cut] = 99
		for p := cut + 1; p <= index; p++ {
			pool[p] = 0
		}
	}
	return slot, true
}

func (s *State) originalPlanSquad(q *Squad) bool {
	path, found := legacySearchWithFrontier(q.Position.Cell(), q.AITarget, s.legacyPathCosts(), s.Influence, s.Leader.Parameters[18], func(index, cell int) {
		// The DOS frontier begins at 886f and its 101st word aliases 8937,
		// the influence map. Preserve these observable writes in Go arrays;
		// a dynamically growing queue alone silently changes later AI.
		if index >= 100 && index < 300 {
			s.Influence[index-100] = cell
		}
	})
	if !found {
		return false
	}
	slot, ok := originalStorePath(&s.Brain.PathPool, path)
	if !ok {
		return false
	}
	q.AIPathOffset = slot
	q.AIPathActive = true
	return true
}

func (s *State) originalCancelRoutes(q *Squad) {
	s.clearFine(q)
	if q.AIPathActive {
		for index := q.AIPathOffset; index >= 0 && index < 200; index++ {
			s.Brain.PathPool[index] = 0
			if index == 199 || s.Brain.PathPool[index+1] == 0 {
				break
			}
		}
	}
	q.AIPathActive = false
}

// originalFineRoute follows 14a3's raw unit origin versus cursor origin, the
// one-in-five skipped horizontal step and the caller's finite step allowance.
func originalFineRoute(from, cursor Point, limit int) []Point {
	x, y := from.X+1, from.Y
	phase := 0
	var route []Point
	for (x != cursor.X || y != cursor.Y) && len(route) < limit {
		phase++
		previousX, previousY := x, y
		if phase < 5 {
			x += sign(cursor.X - x)
		} else {
			phase = 0
		}
		y += sign(cursor.Y - y)
		if x == previousX && y == previousY {
			continue
		}
		p := Point{x - 1, y}
		if !p.Valid() {
			break
		}
		route = append(route, p)
	}
	return route
}

func (s *State) originalFineOrder(q *Squad, cursor Point, limit int) {
	s.originalInstallFine(q, cursor, limit)
}

// originalNearestFlag translates 834d, including its strict distance bound.
func (s *State) originalNearestFlag(q *Squad, side Side, limit int) (int, int) {
	selected := -1
	for i, f := range s.Flags {
		if f.Side != side || !s.clearApproach(q.Position.Cell(), f.Position.Cell()) {
			continue
		}
		distance := originalDistance(q.Position, f.Position)
		if distance != 0 && distance < limit {
			selected = i
			limit = distance
		}
	}
	return selected, limit
}

func (s *State) originalPrecisionFlag(q *Squad, flag int) {
	s.originalCancelRoutes(q)
	q.AIPathActive = true
	if q.AIPathOffset >= 0 && q.AIPathOffset < 200 {
		s.Brain.PathPool[q.AIPathOffset] = 99
	}
	q.Pace = s.Leader.Parameters[17]
	s.originalFineOrder(q, s.Flags[flag].Position, 35)
}

// originalAdvance translates 7d57: opportunistic flags, weak-squad pursuit,
// one coarse direction, replanning, joining, then final positioning at a flag.
func (s *State) originalAdvance(q *Squad) {
	flag, _ := s.originalNearestFlag(q, Player, 30)
	if flag >= 0 {
		f := s.Flags[flag]
		defenders := s.stationarySquads(f.Position.Cell(), Player)
		if defenders <= 2 && s.rand128() < s.Leader.Parameters[13] && (s.World.Tiles[f.Position.Cell()] != 3 || q.Men[Archer] > 0 || defenders == 0) {
			s.originalPrecisionFlag(q, flag)
			return
		}
	}
	cell := q.Position.Cell()
	foe, distance := s.originalNearestSquad(q.ID, cell, 2)
	if distance <= s.Leader.Parameters[11] && foe >= 0 && (q.Men[Archer] > 0 || s.World.Tiles[s.Squads[foe].Position.Cell()] != 3) {
		s.originalCancelRoutes(q)
		q.AIPathActive = true
		if q.AIPathOffset >= 0 && q.AIPathOffset < 200 {
			s.Brain.PathPool[q.AIPathOffset] = 99
		}
		q.Pace = 1
		s.originalFineOrder(q, s.Squads[foe].Position, distance<<2)
		return
	}
	for attempts := 0; attempts < 202; attempts++ {
		if q.AIPathOffset < 0 || q.AIPathOffset >= 200 {
			break
		}
		direction := s.Brain.PathPool[q.AIPathOffset]
		s.Brain.PathPool[q.AIPathOffset] = 0
		if direction != 0 && direction != 99 {
			q.AIPathOffset++
			if direction > 8 {
				break
			}
			delta := legacyDirections[direction]
			dx, dy := 0, 0
			if delta < -1 {
				dy = -10
				delta += 10
			} else if delta > 1 {
				dy = 10
				delta -= 10
			}
			dx = delta * 8
			cursor := Point{cell%10*8 + 4 + dx, cell/10*10 + dy}
			s.originalFineOrder(q, cursor, 30)
			return
		}
		if cell == q.AITarget || !s.originalPlanSquad(q) {
			break
		}
	}
	q.Route = nil
	q.AIPathActive = false
	q.AIActive = false
	if cell == s.Brain.TargetCell {
		s.Brain.TargetCell = -1
	}
	s.originalJoin(q)
	s.originalJoin(q)
	flag, distance = s.originalNearestFlag(q, Enemy, 4)
	if flag >= 0 && cell == q.AITarget && distance >= 2 {
		s.originalPrecisionFlag(q, flag)
	}
}

// 1625 / 5ad5: the first nearby friendly squad wins the search, even when it is
// already full. Soldiers transfer in B/K/A/S order and inherit the recipient.
func (s *State) originalJoin(q *Squad) {
	if q.BattleID != 0 || q.originalCount() > 13 {
		return
	}
	for i := range s.Squads {
		other := &s.Squads[i]
		if !other.Alive() || other.Side != q.Side || other.ID == q.ID || other.BattleID != 0 || abs(other.Position.X-q.Position.X) >= 3 || abs(other.Position.Y-q.Position.Y) >= 7 {
			continue
		}
		for _, kind := range []Kind{Barbarian, Knight, Archer, Spy} {
			for other.originalCount() < 14 && q.Men[kind] > 0 {
				q.Men[kind]--
				other.Men[kind]++
				s.refreshOriginalCounts()
			}
		}
		if !q.Alive() {
			s.originalCancelRoutes(q)
		}
		return
	}
}

func (s *State) initializeBrain() {
	if s.Brain.Initialized {
		return
	}
	// The original fresh data segment starts the previous-flag anchor at zero.
	s.Brain = StrategyBrain{Initialized: true, HomeFlag: -1, HomeCell: 0, TargetFlag: 0, TargetCell: -1}
	s.originalChooseFlag()
	s.updateOriginalHome()
}

func (s *State) updateOriginalHome() {
	brain := &s.Brain
	if brain.HomeFlag >= len(s.Flags) || brain.HomeFlag >= 0 && s.Flags[brain.HomeFlag].Side != Enemy {
		brain.HomeFlag = -1
	}
	for i, f := range s.Flags {
		if f.Side != Enemy {
			continue
		}
		cell := f.Position.Cell()
		if brain.HomeFlag < 0 || s.World.Tiles[brain.HomeCell] != 3 && s.World.Tiles[cell] == 3 {
			brain.HomeFlag = i
			brain.HomeCell = cell
		}
	}
}

func (s *State) originalEnemyOrders(q *Squad) {
	if q.AIPathActive {
		s.originalAdvance(q)
		return
	}
	if !q.AIActive && s.originalObjective(q.ID) {
		if !s.originalPlanSquad(q) {
			q.AIActive = false
		}
	}
}
