package game

// StrategyBrain contains the original commander's shared objectives. Individual
// squads do not independently pick a fresh strategic goal on every update.
type StrategyBrain struct {
	Initialized            bool
	HomeFlag, HomeCell     int
	TargetFlag, TargetCell int
	Alert                  bool
	PathPool               [200]byte
}

// originalObjective translates overlay 5:79fc and 7b27. Path construction is a
// separate step so its original-code comparison does not depend on rendering or
// the route allocator. A successful decision marks the squad as commanded.
func (s *State) originalObjective(id int) bool {
	q := &s.Squads[id]
	p := &s.Leader.Parameters
	brain := &s.Brain
	cell := q.Position.Cell()
	values := s.strategicValues()
	if p[3] < 150 && (values[cell] != 0 || cell == brain.HomeCell) {
		count := s.stationarySquads(cell, Enemy)
		if s.World.Tiles[cell] == 3 {
			_, distance := s.originalNearestSquad(id, cell, 0)
			if q.originalCount() < 14 || count == 1 || distance < 4 && q.Food > 10 {
				return false
			}
		}
		if (q.Food >= p[8] || s.SupplyDistance(q) < 10) && (((p[5] < s.rand128() || s.rand128() < 100) && !brain.Alert) || (cell == brain.HomeCell && count <= p[6] || q.Condition < p[9])) {
			return false
		}
	}
	if cell == brain.TargetCell {
		return false
	}
	supply := s.originalSupplyObjective(q)
	if q.Food < p[8] && supply < 999 {
		q.AITarget = supply
	} else if s.originalDefenders(brain.HomeCell) < p[6] {
		q.Pace = 1
		q.AITarget = brain.HomeCell
	} else {
		if p[0] <= s.stationarySquads(brain.TargetCell, Enemy) && s.rand128() < 60 {
			brain.TargetCell = -1
			if values[cell] != 0 {
				brain.Alert = true
			}
		}
		if brain.TargetCell == -1 {
			if brain.TargetFlag == -1 {
				s.originalChooseFlag()
			}
			if brain.TargetFlag < 0 || brain.TargetFlag >= len(s.Flags) {
				return false
			}
			flagCell := s.Flags[brain.TargetFlag].Position.Cell()
			if p[3] <= s.rand128() {
				best := -860
				for candidate, value := range values {
					if value == 0 || candidate == cell {
						continue
					}
					influence := s.Influence[candidate]
					score := 0
					if influence < 0 && p[18] < influence && s.World.Tiles[candidate] != 3 {
						score = 100
					}
					if s.World.Tiles[candidate] == 3 && influence == 0 {
						score = 150
					}
					score += value
					if s.rand128() < p[10] {
						score -= s.rand128()
					}
					if best < score {
						best = score
						brain.TargetCell = candidate
					}
				}
			} else {
				brain.TargetCell = flagCell
			}
			if brain.TargetCell == -1 {
				brain.TargetCell = flagCell
			}
		} else if s.rand128() < 25 {
			brain.Alert = false
		}
		q.AITarget = brain.TargetCell
	}
	q.AIActive = true
	return true
}

func (s *State) stationarySquads(cell int, side Side) int {
	n := 0
	for _, q := range s.Squads {
		if q.Alive() && s.originalSide(&q) == side && !q.Moving() && q.Position.Cell() == cell {
			n++
		}
	}
	return n
}

func (s *State) originalDefenders(cell int) int {
	n := 0
	for _, q := range s.Squads {
		if q.Alive() && s.originalSide(&q) == Enemy && (q.AIActive && q.AITarget == cell || q.Position.Cell() == cell) {
			n++
		}
	}
	return n
}

// 815c: target flags are scored relative to the previous flag, with a random
// anchor only when there is no previous objective.
func (s *State) originalChooseFlag() {
	brain := &s.Brain
	anchor := 0
	if brain.TargetFlag == -1 {
		anchor = s.rand128() + 36
	} else if brain.TargetFlag < len(s.Flags) {
		anchor = s.Flags[brain.TargetFlag].Position.Cell()
	}
	brain.TargetFlag = -1
	best := -860
	for i, f := range s.Flags {
		if f.Side != Player {
			continue
		}
		cell := f.Position.Cell()
		score := min(0, s.Influence[cell]-s.Leader.Parameters[4]) - 8*cellDistance(cell, anchor)
		if s.rand128() < s.Leader.Parameters[10] {
			score -= s.rand128()
		}
		if best < score {
			best = score
			brain.TargetFlag = i
		}
	}
}

// 6dbf: hungry squads may select an uncontrolled source, but hostile influence
// penalizes it. This is distinct from the controlled-source supply distance.
func (s *State) originalSupplyObjective(q *Squad) int {
	best, chosen := 999, 999
	for cell, tile := range s.World.Tiles {
		if !(tile == 4 && s.Rules.VillageFood || tile == 3 && s.Rules.FortFood) {
			continue
		}
		center := Point{cell%10*8 + 4, cell/10*10 + 5}
		score := originalDistance(q.Position, center) - (s.Influence[cell] >> 1)
		if score < best {
			best = score
			chosen = cell
		}
	}
	return chosen
}

// 8248: all variants retain the original first-match ties and distance + 1.
func (s *State) originalNearestSquad(id, cell, filter int) (int, int) {
	chosen, distance := -1, 999
	q := &s.Squads[id]
	for i, foe := range s.Squads {
		if !foe.Alive() || foe.Side != Player || foe.BattleID != 0 {
			continue
		}
		candidate := foe.Position.Cell()
		if filter == 1 && (s.World.Tiles[candidate] == 3 || s.World.Tiles[candidate] == 4 || candidate == s.Brain.HomeCell) {
			continue
		}
		if filter == 2 && (s.CaptureAdvantage(q, &foe) < s.Leader.Parameters[12] || !s.clearApproach(cell, candidate)) {
			continue
		}
		d := cellDistance(cell, candidate)
		if d < distance {
			distance = d
			chosen = i
		}
	}
	return chosen, distance + 1
}
