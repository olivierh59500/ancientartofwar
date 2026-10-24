package game

import "fmt"

func New(c Campaign, leader Leader, formations [10]Formation, seed uint64) *State {
	if seed == 0 {
		seed = 1
	}
	s := &State{Version: StateVersion, CampaignID: c.ID, Title: c.Title, World: c.World, Squads: c.Squads, Flags: append([]Flag(nil), c.Flags...), Rules: c.Rules, Leader: leader, LeaderIndex: c.Opponent, Formations: formations, Options: DefaultOptions(), Random: seed, Tactical: c.Tactical}
	for i := range s.Squads {
		q := &s.Squads[i]
		q.Route = nil
		q.FineActive, q.FineOffset = false, 0
		q.AIActive, q.AIPathActive, q.AIPathOffset = false, false, 0
		q.BattleID = 0
		q.Visible = q.Side == Player
		q.Sighted = false
		q.AITarget = -1
		q.CountCached, q.CountCache = false, 0
		// Resident initialization replaces spies when everyone is always visible.
		if s.Rules.Visibility == 80 {
			q.Men[Barbarian] += q.Men[Spy]
			q.Men[Spy] = 0
		}
	}
	// Overlay 1, 314a / 3424: incomplete campaigns receive a randomly
	// positioned flag for each missing side (Sherwood Forest needs this).
	for side := Player; side <= Enemy; side++ {
		found := false
		for _, f := range s.Flags {
			found = found || f.Side == side
		}
		if !found && len(s.Flags) < MaxFlags {
			s.Flags = append(s.Flags, Flag{Position: Point{X: (s.rand128() >> 1) + 8, Y: s.rand128() + 31}, Side: side})
		}
	}
	s.UpdateInfluence()
	enemySquads := 0
	for _, q := range s.Squads {
		if q.Alive() && q.Side == Enemy {
			enemySquads++
		}
	}
	// Overlay 1 caps the leader's home-garrison demand for this campaign.
	s.Leader.Parameters[6] = min(s.Leader.Parameters[6], enemySquads>>1)
	s.PendingInitialization = true
	s.reassessArmies(-1, -1)
	if s.Outcome == Running && !s.SurrenderOffered {
		s.finishInitialization()
	} else if s.Outcome != Running {
		s.PendingInitialization = false
	}
	s.log("Campaign started: "+c.Title, Point{40, 100})
	return s
}

func (s *State) finishInitialization() {
	s.PendingInitialization = false
	s.initializeBrain()
	for i := 0; i < 5; i++ {
		s.think()
	}
	s.updateSightings(false)
}

// rand128 replaces the DOS generator's sampling of mutable process memory with
// a saved, deterministic generator. All original probability thresholds use 0..127.
func (s *State) rand128() int {
	x := s.Random
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	s.Random = x
	return int(x>>32) & 127
}

func (s *State) log(text string, p Point) {
	s.Events = append(s.Events, Event{Tick: s.Tick, Text: text, Position: p})
	if len(s.Events) > 40 {
		s.Events = append([]Event(nil), s.Events[len(s.Events)-40:]...)
	}
}

// Step advances one fixed 1/60-second tick. Presentation pauses and time scaling
// are outside the simulation, so saves and headless runs replay identically.
func (s *State) Step() {
	if s.SurrenderOffered && s.Options.AutoAcceptSurrender {
		s.AcceptSurrender()
	}
	if s.Outcome != Running || s.SurrenderOffered || s.Fault != "" {
		return
	}
	resume := s.WorldResume
	if resume == 0 {
		s.Tick++
		for i := range s.Battles {
			b := &s.Battles[i]
			if b.Manual && !b.Done {
				s.stepBattle(b)
				if b.Done {
					s.checkOutcome()
				}
				return
			}
		}
		s.WorldTick++
	}
	s.WorldResume = 0
	if resume <= 1 {
		s.stepAutomaticBattles()
		if s.Outcome != Running || s.SurrenderOffered {
			if s.Outcome == Running {
				s.WorldResume = 1
			}
			return
		}
	}
	if s.WorldTick%4 == 0 {
		if resume <= 2 {
			s.captureFlags()
			s.checkOutcome()
			if s.Outcome != Running {
				return
			}
		}
		if resume <= 3 {
			s.updateSightings(true)
			if s.Outcome != Running || s.SurrenderOffered {
				if s.Outcome == Running {
					s.WorldResume = 3
				}
				return
			}
		}
		if resume <= 4 {
			s.think()
		}
		s.updateMovement()
		if s.Outcome != Running || s.SurrenderOffered {
			if s.Outcome == Running {
				s.WorldResume = 5
			}
			return
		}
	}
	if s.WorldTick%64 == 0 {
		s.UpdateInfluence()
		s.updateSupplies()
		s.recruit()
	}
	s.checkOutcome()
}

// 5bc9 recalculates every squad's delay and samples terrain losses before
// 6273 checks any movement clock. A long delay does not defer those rolls.
func (s *State) movementDelays(losses bool) (delays [MaxSquads]int) {
	if s.MovementCursor != 0 {
		delays = s.MovementDelay
	}
	for i := s.MovementCursor; i < MaxSquads; i++ {
		q := &s.Squads[i]
		if !q.Alive() {
			continue
		}
		pace := 0
		if q.Moving() {
			pace = q.Pace
		}
		delay := pace
		t := s.World.TerrainAt(q.Position)
		water, mountain, forest := s.Rules.WaterSpeed, s.Rules.MountainSpeed, s.Rules.ForestSpeed
		if s.originalSide(q) == Enemy {
			mountain, forest = s.Leader.Parameters[15], s.Leader.Parameters[16]
		}
		switch t {
		case Forest:
			delay *= forest
		case Water:
			delay *= water
		case Mountain:
			delay *= mountain
		}
		if q.Men[Knight] > 0 {
			delay += pace
		}
		delays[i] = delay
		danger := t == Mountain && mountain > 15 || t == Water && water > 19
		if losses && pace != 0 && (danger && s.rand128() < 2 || t == Water && water > 39 && s.rand128() < 4) && q.BattleID == 0 {
			s.casualty(q, q, true)
			if q.Fighters() == 0 {
				s.normalizeDeadSquads()
				s.reassessArmies(q.ID, q.ID)
				if !q.Alive() {
					delays[i] = 0
					s.log(fmt.Sprintf("%s squad %02d lost in %s", q.Side, q.ID+1, t), q.Position)
				}
				if s.SurrenderOffered && s.Outcome == Running {
					s.MovementCursor, s.MovementDelay = i+1, delays
					return delays
				}
			}
		}
	}
	s.MovementCursor, s.MovementDelay = 0, [MaxSquads]int{}
	return delays
}

func (s *State) updateMovement() [MaxSquads]int {
	delays := s.movementDelays(true)
	if s.SurrenderOffered || s.Outcome != Running {
		return delays
	}
	for i := range s.Squads {
		q := &s.Squads[i]
		if !q.Alive() || !q.Moving() {
			continue
		}
		q.MoveClock--
		if q.MoveClock > 0 {
			continue
		}
		q.MoveClock = delays[i]
		if q.FineActive {
			s.takeFineStep(q)
		} else if len(q.Route) != 0 {
			q.Position = q.Route[0]
			q.Route = q.Route[1:]
		}
	}
	return delays
}

// UpdateInfluence is the two-pass diffusion in overlay 5, 1000:8b0c.
// Negative values belong to the player; positive values belong to the enemy.
func (s *State) UpdateInfluence() {
	s.refreshOriginalCounts()
	var power [200]int
	for _, q := range s.Squads {
		if !q.Alive() {
			continue
		}
		cell := q.Position.Cell()
		v := q.originalCount() * 4
		if s.originalSide(&q) == Player {
			v = -v
		}
		tile := s.World.Tiles[cell]
		if IsWaterTile(tile) {
			v = 0
		} else if IsMountainTile(tile) {
			v >>= 2
		} else if tile == 3 || tile == 5 || tile == 6 {
			v <<= 1
		}
		if q.Condition < 10 {
			v >>= 1
		}
		power[cell] += v
	}
	for pass := 0; pass < 2; pass++ {
		var next [200]int
		for cell, tile := range s.World.Tiles {
			if IsWaterTile(tile) {
				continue
			}
			v := power[cell]
			for _, n := range neighbors(cell) {
				if IsMountainTile(tile) {
					v += power[n] >> 5
				} else {
					v += power[n] >> 3
				}
			}
			next[cell] = v
		}
		power = next
	}
	s.Influence = power
}

func neighbors(cell int) []int {
	var ns []int
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			x, y := cell%10+dx, cell/10+dy
			if (dx != 0 || dy != 0) && x >= 0 && x < 10 && y >= 0 && y < 20 {
				ns = append(ns, y*10+x)
			}
		}
	}
	return ns
}

func cellCenter(cell int) Point       { return Point{min(78, cell%10*8+3), min(194, cell/10*10)} }
func cellDistance(a, b int) int       { return max(abs(a%10-b%10), abs(a/10-b/10)) }
func originalDistance(a, b Point) int { return max(abs(a.X-b.X), abs(a.Y-b.Y)) }

// SupplyDistance follows 1000:6ca1. A source needs friendly influence, and the
// player supply-line setting scales the distance. Enemy supply range is fixed.
func (s *State) SupplyDistance(q *Squad) int {
	d := 999
	for cell, tile := range s.World.Tiles {
		if !(tile == 3 && s.Rules.FortFood || tile == 4 && s.Rules.VillageFood) {
			continue
		}
		v := s.Influence[cell]
		side := s.originalSide(q)
		if side == Player && v >= -1 || side == Enemy && v <= 1 {
			continue
		}
		center := Point{cell%10*8 + 4, cell/10*10 + 5}
		d = min(d, originalDistance(q.Position, center))
	}
	if s.originalSide(q) == Player {
		return d / max(1, s.Rules.SupplyRange)
	}
	return d / 2
}

// updateSupplies ports overlay 5, 1000:5683, retaining the original 1..49 bars,
// ration threshold, side differences, settlement recovery and pace downgrade.
func (s *State) updateSupplies() {
	for i := range s.Squads {
		q := &s.Squads[i]
		if !q.Alive() {
			continue
		}
		d := s.SupplyDistance(q)
		if d > 20 {
			q.Food--
		}
		if d > 40 {
			q.Food--
		}
		if d < 10 {
			q.Food++
			if s.originalSide(q) == Enemy {
				q.Food++
			}
		}
		tile := s.World.Tile(q.Position)
		if tile == 4 && s.Rules.VillageFood || tile == 3 && s.Rules.FortFood {
			q.Food = 49
		}
		q.Food = min(49, max(1, q.Food))
		pace := 0
		if q.Moving() {
			pace = q.Pace
		}
		if s.originalSide(q) == Player {
			if pace == 1 {
				q.Condition -= s.Rules.Fitness * 2
			}
			if pace == 2 {
				q.Condition -= s.Rules.Fitness
			}
			if pace == 0 {
				q.Condition++
			}
			if q.Food < 10 {
				q.Condition -= 3
			}
		} else {
			if pace == 1 || pace == 2 {
				q.Condition--
			}
			if pace == 0 {
				q.Condition++
			}
			if q.Food < 10 {
				q.Condition -= 2
			}
		}
		if (tile == 3 || tile == 4) && q.Food > 10 {
			q.Condition++
		}
		q.Condition = min(49, max(1, q.Condition))
		if pace == 1 && q.Condition < 15 {
			q.Pace = 2
		}
		if pace == 2 && q.Condition < 5 {
			q.Pace = 3
		}
	}
}

// Overlay 5, 1000:58c7: forts train only occupied, non-full squads.
func (s *State) recruit() {
	defer s.refreshOriginalCounts()
	for cell, tile := range s.World.Tiles {
		if tile != 3 {
			continue
		}
		for i := range s.Squads {
			q := &s.Squads[i]
			if !q.Alive() || q.Position.Cell() != cell {
				continue
			}
			if q.originalCount() < MaxMen {
				if s.rand128() < 90 {
					if s.rand128() < s.Rules.Recruitment {
						r := s.rand128()
						k := Barbarian
						if r < 42 {
							k = Archer
						} else if r < 84 {
							k = Knight
						}
						q.Men[k]++
					}
					break
				}
			} else if s.originalSide(q) == Enemy && !q.Moving() {
				s.detachForAI(i)
			}
		}
	}
}

func (s *State) captureFlags() {
	retarget := false
	// 3869 scans squads first and takes at most one flag per squad. Its hit
	// test uses framebuffer addresses, including the asymmetric vertical span.
	for _, q := range s.Squads {
		if !q.Alive() || q.BattleID != 0 {
			continue
		}
		side := s.originalSide(&q)
		for i := range s.Flags {
			f := &s.Flags[i]
			d := (q.Position.Y-f.Position.Y)*80 + q.Position.X - f.Position.X
			near := d >= -1 && d <= 1 || d >= 79 && d <= 81 || d >= 159 && d <= 161 || d >= -81 && d <= -79
			if f.Side == side || !near {
				continue
			}
			if side == Enemy && (i == s.Brain.TargetFlag || s.Brain.TargetFlag == -1) {
				retarget = true
			}
			if side == Player && i == s.Brain.HomeFlag {
				s.Brain.HomeFlag = -1
			}
			f.Side = side
			s.Stats.Flags[side]++
			s.log(fmt.Sprintf("%s captured a flag", side), f.Position)
			break
		}
	}
	if s.Brain.Initialized {
		if retarget {
			s.originalChooseFlag()
		}
		s.updateOriginalHome()
	}
}

func (s *State) updateSightings(report bool) {
	var before [MaxSquads]bool
	for i, q := range s.Squads {
		before[i] = q.Visible
	}
	s.encounters()
	for i := range s.Squads {
		q := &s.Squads[i]
		if q.Visible && !before[i] && q.Alive() && q.Side == Enemy && report && s.Rules.Visibility < 80 {
			s.log("Enemy sighted", q.Position)
		}
	}
}

func (s *State) Totals() (men [2]int) {
	for _, q := range s.Squads {
		men[q.Side] += q.Count()
	}
	return
}

func (s *State) checkOutcome() {
	if s.Outcome != Running {
		return
	}
	men := s.Totals()
	if men[Player] == 0 {
		s.finish(Enemy, "All allied squads lost")
		return
	}
	if men[Enemy] == 0 {
		s.finish(Player, "All enemy squads defeated")
		return
	}
	var flags [2]int
	for _, f := range s.Flags {
		flags[f.Side]++
	}
	if len(s.Flags) > 0 {
		if flags[Player] == 0 {
			s.finish(Enemy, "All allied flags captured")
			return
		}
		if flags[Enemy] == 0 {
			s.finish(Player, "All enemy flags captured")
			return
		}
	}
}

func (s *State) normalizeDeadSquads() {
	s.refreshOriginalCounts()
	for i := range s.Squads {
		if q := &s.Squads[i]; !q.Alive() {
			s.originalCancelRoutes(q)
		}
	}
}

func (s *State) refreshOriginalCounts() {
	for i := range s.Squads {
		s.Squads[i].CountCached, s.Squads[i].CountCache = false, 0
	}
}

// 346b reassesses the commander only after an army is captured or a battle or
// terrain loss ends. Its garrison reduction is one step, rather than a clamp.
func (s *State) reassessArmies(first, second int) {
	var armies, men [2]int
	for i := range s.Squads {
		q := &s.Squads[i]
		if !q.Alive() {
			s.originalCancelRoutes(q)
			continue
		}
		armies[q.Side]++
		men[q.Side] += q.Count()
		if (i == first || i == second) && q.FineActive {
			q.Stopped = false
		}
	}
	p := &s.Leader.Parameters
	if armies[Enemy]>>1 < p[6] {
		p[6]--
	}
	if armies[Player] < 3 || armies[Player] < armies[Enemy]>>2 || armies[Enemy] < 3 {
		p[3], p[6], p[12] = 200, 0, -100
	}
	if armies[Player] == 0 {
		s.finish(Enemy, "All allied squads lost")
	} else if armies[Enemy] == 0 {
		s.finish(Player, "All enemy squads defeated")
	} else if men[Enemy] > 0 && men[Player] > 20 && p[7] <= men[Player]/men[Enemy] {
		p[7] = 500
		s.SurrenderOffered = true
		s.log("The enemy offers to surrender", Point{40, 100})
		if s.Options.AutoAcceptSurrender {
			s.AcceptSurrender()
		}
	}
}
func (s *State) finish(winner Side, reason string) {
	s.Outcome = PlayerWon
	if winner == Enemy {
		s.Outcome = EnemyWon
	}
	s.Reason = reason
	s.log(reason, Point{40, 100})
}
func (s *State) Surrender() {
	if s.Outcome == Running {
		s.finish(Enemy, "You surrendered")
	}
}
func (s *State) AcceptSurrender() {
	if s.SurrenderOffered && s.Outcome == Running {
		s.SurrenderOffered = false
		s.WorldResume = 0
		s.PendingInitialization = false
		s.finish(Player, "Enemy surrender accepted")
	}
}

func (s *State) DeclineSurrender() {
	if !s.SurrenderOffered || s.Outcome != Running {
		return
	}
	s.SurrenderOffered = false
	s.Leader.Parameters[7] = 500
	s.Leader.Parameters[3] = 200
	s.Leader.Parameters[6] = 0
	s.Leader.Parameters[12] = -100
	s.log("Enemy surrender declined; the war continues", Point{40, 100})
	if s.PendingInitialization {
		s.finishInitialization()
	}
}
