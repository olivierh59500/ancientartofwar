package game

import "fmt"

type Tactic int

const (
	Attack Tactic = iota
	Advance
	BackOff
	Hold
)

type Battle struct {
	ID                      int
	Squads                  [2]int // indexed by Side, preserving ownership while fighting
	Created, Started        int
	Manual, Attacking, Done bool
	Terrain                 [2]Terrain
	Arena                   *legacyArena
	FrameClock              int
	AutoPhase               [2]int // positive countdown, -1/0 owner, -2 inactive
	AutoChance              [2]int
	AutoSide                [2]Side // original flashing side, separate from ownership
	AppliedCasualties       [2]int
	AppliedCaptured         [2]int
}

func (s *State) Battle(id int) *Battle {
	if id >= 0 && id < len(s.Battles) && s.Battles[id].ID == id {
		return &s.Battles[id]
	}
	for i := range s.Battles {
		if s.Battles[i].ID == id {
			return &s.Battles[i]
		}
	}
	return nil
}

func (s *State) ConditionAdvantage(a, b *Squad) int {
	v := a.Condition - b.Condition
	ta, tb := s.World.Tile(a.Position), s.World.Tile(b.Position)
	if ta == 5 || ta == 6 {
		v += 15
	}
	if tb == 5 || tb == 6 {
		v -= 15
	}
	if IsWaterTile(ta) || a.Fighters() == 0 {
		v = -30
	}
	if IsWaterTile(tb) || b.Fighters() == 0 {
		v = 30
	}
	return min(30, max(-30, v))
}
func (s *State) CaptureAdvantage(a, b *Squad) int {
	if a.Fighters() == 0 && b.Fighters() == 0 {
		return 0
	}
	if a.Fighters() == 0 {
		return -67
	}
	if b.Fighters() == 0 {
		return 67
	}
	return (s.ConditionAdvantage(a, b) >> 1) + (a.originalCount()-b.originalCount())*4
}

func (s *State) encounters() {
	seen := s.EncounterSeen
	defer func() {
		if s.EncounterCursor != 0 {
			return
		}
		for i := range s.Squads {
			q := &s.Squads[i]
			q.Sighted = seen[i] || q.BattleID != 0
			q.Visible = q.Side == Player || s.Rules.Visibility == 80 || q.Sighted
		}
	}()
	for i := s.EncounterCursor; i < MaxSquads; i++ {
		a := &s.Squads[i]
		if !a.Alive() || s.originalSide(a) != Player || a.BattleID != 0 {
			continue
		}
		for j := range s.Squads {
			b := &s.Squads[j]
			if !b.Alive() || s.originalSide(b) != Enemy || b.BattleID != 0 {
				continue
			}
			ta, tb := s.World.Tile(a.Position), s.World.Tile(b.Position)
			dx, dy := encounterDistance(a.Position, b.Position, ta, tb)
			if dx >= 3 || dy >= 5 {
				d := dx*2 + dy
				if a.Men[Spy] > 0 {
					d >>= 1
				}
				if d < s.Rules.Visibility && s.Rules.Visibility < 80 || s.Rules.Visibility >= 80 && d < 20 {
					seen[i], seen[j] = true, true
				}
				continue
			}
			if a.Fighters() == 0 && b.Fighters() == 0 {
				continue
			}
			if ta == 3 && tb != 3 && a.Men[Archer] == 0 || tb == 3 && ta != 3 && b.Men[Archer] == 0 {
				continue
			}
			adv := s.CaptureAdvantage(a, b)
			if adv > 50 && tb != 3 {
				s.capture(b, a)
				if s.Outcome != Running || s.SurrenderOffered {
					if s.Outcome == Running {
						s.EncounterCursor, s.EncounterSeen = i+1, seen
					}
					return
				}
				break
			}
			if adv < -50 && ta != 3 {
				s.capture(a, b)
				if s.Outcome != Running || s.SurrenderOffered {
					if s.Outcome == Running {
						s.EncounterCursor, s.EncounterSeen = i+1, seen
					}
					return
				}
				break
			}
			id := s.NextBattleID
			s.NextBattleID++
			chance := [2]int{s.previousCombatChance(i), s.previousCombatChance(j)}
			s.Battles = append(s.Battles, Battle{ID: id, Squads: [2]int{i, j}, Created: s.Tick, AutoPhase: [2]int{1, 1}, AutoChance: chance, AutoSide: [2]Side{Player, Enemy}, Terrain: [2]Terrain{s.World.TerrainAt(a.Position), s.World.TerrainAt(b.Position)}})
			a.BattleID = id + 1
			b.BattleID = id + 1
			seen[i], seen[j] = true, true
			s.log(fmt.Sprintf("Encounter: squads %02d / %02d — Z to take command", i+1, j+1), a.Position)
			break
		}
	}
	s.EncounterCursor, s.EncounterSeen = 0, [MaxSquads]bool{}
}

func (s *State) previousCombatChance(id int) int {
	for i := len(s.Battles) - 1; i >= 0; i-- {
		b := &s.Battles[i]
		for side, squad := range b.Squads {
			if squad == id {
				return b.AutoChance[side]
			}
		}
	}
	return 0
}

// 4c8c measures forts and hills from the tile center. Hills retain their raw
// squad position when the opposing squad occupies a fort.
func encounterDistance(a, b Point, ta, tb byte) (int, int) {
	pa, pb := a, b
	pa.Y += 5
	pb.Y += 5
	if ta == 3 || (ta == 5 || ta == 6) && tb != 3 {
		pa = cellCenter(a.Cell())
		pa.Y += 5
	}
	if tb == 3 || (tb == 5 || tb == 6) && ta != 3 {
		pb = cellCenter(b.Cell())
		pb.Y += 5
	}
	dx, dy := abs(pa.X-pb.X), abs(pa.Y-pb.Y)
	if ta == 3 || tb == 3 || ta == 5 || ta == 6 || tb == 5 || tb == 6 {
		dx, dy = max(0, dx-3), max(0, dy-3)
	}
	return dx, dy
}

// The DOS side array flashes during automatic combat. Ownership remains in
// its combat marker, but other world routines read the flashing array.
func (s *State) originalSide(q *Squad) Side {
	if q.BattleID != 0 {
		if b := s.Battle(q.BattleID - 1); b != nil && !b.Manual && !b.Done {
			return b.AutoSide[q.Side]
		}
	}
	return q.Side
}

func (s *State) capture(loser, winner *Squad) {
	s.Stats.Captured[loser.Side] += loser.originalCount()
	s.log(fmt.Sprintf("%s squad %02d captured", loser.Side, loser.ID+1), loser.Position)
	s.originalCancelRoutes(loser)
	loser.Men = [4]int{}
	loser.Route = nil
	loser.BattleID = 0
	s.normalizeDeadSquads()
	s.reassessArmies(loser.ID, winner.ID)
}

// casualty translates 1000:5442. Both selection loops use the original 42/84
// cutoffs and reject unavailable kinds. The second chance roll is consumed even
// after a counter kill; this affects every subsequent random decision.
func (s *State) casualty(attacker, defender *Squad, terrainLoss bool) {
	if defender.Fighters() == 0 {
		defender.Men[Spy] = 0
		defender.CountCached, defender.CountCache = true, 0
		return
	}
	chosen := -1
	if s.rand128() < 64 {
		kinds := [3]Kind{Knight, Barbarian, Archer}
		allowed := [3]bool{
			attacker.Men[Archer] > 0 && defender.Men[Knight] > 0,
			attacker.Men[Knight] > 0 && defender.Men[Barbarian] > 0,
			attacker.Men[Barbarian] > 0 && defender.Men[Archer] > 0,
		}
		for chosen < 0 && (allowed[0] || allowed[1] || allowed[2]) {
			bucket := min(2, s.rand128()/42)
			if allowed[bucket] {
				chosen = int(kinds[bucket])
			}
		}
	}
	if s.rand128() < 32 && chosen < 0 {
		for chosen < 0 {
			kind := min(2, s.rand128()/42)
			if defender.Men[kind] > 0 {
				chosen = kind
			}
		}
	}
	if chosen >= 0 {
		defender.Men[chosen]--
		side := defender.Side
		if terrainLoss {
			side = s.originalSide(defender)
		} else if b := s.Battle(defender.BattleID - 1); defender.BattleID != 0 && b != nil && !b.Manual {
			// At the exact transition into combat, the higher-ID opponent
			// can still hold a positive delay marker. 5442 counts that loss
			// as enemy, even when the squad's ownership is player.
			side = Enemy
			if b.AutoPhase[defender.Side] == -1 {
				side = Player
			}
		}
		s.Stats.Casualties[side]++
	}
	defender.CountCached, defender.CountCache = true, defender.Fighters()
}

func (s *State) stepBattle(b *Battle) {
	if !b.Done && b.Manual {
		s.stepTactics(b)
	}
}

// 5172 scans global squad IDs, not battle pairs. Each side caches its chance
// on entering combat, then attacks every original main-loop iteration.
func (s *State) stepAutomaticBattles() {
	delay := max(1, s.Options.FightDelay)
	for i := s.AutomaticCursor; i < MaxSquads; i++ {
		attacker := &s.Squads[i]
		if !attacker.Alive() || attacker.BattleID == 0 {
			continue
		}
		b := s.Battle(attacker.BattleID - 1)
		if b == nil || b.Done || b.Manual {
			continue
		}
		side := attacker.Side
		defender := &s.Squads[b.Squads[side.Other()]]
		phase := &b.AutoPhase[side]
		if *phase > 0 && *phase < delay {
			*phase++
		} else if *phase == delay {
			*phase = int(b.AutoSide[side]) - 1
			b.AutoChance[side] = s.ConditionAdvantage(attacker, defender)*2 + 60
			if s.World.Tile(attacker.Position) == 3 && attacker.Men[Archer] > 0 {
				b.AutoChance[side] <<= 1
			}
			if b.Started == 0 {
				b.Started = max(1, s.Tick)
				b.Attacking = true
				s.log(fmt.Sprintf("Fighting: %02d / %02d", b.Squads[Player]+1, b.Squads[Enemy]+1), attacker.Position)
			}
		} else {
			b.AutoSide[side] = b.AutoSide[side].Other()
		}
		if *phase == -1 || *phase == 0 {
			if s.rand128() < b.AutoChance[side] && s.rand128() < 25 {
				s.casualty(attacker, defender, false)
				if defender.Fighters() == 0 {
					defender.Men[Spy] = 0
					if b.AutoPhase[defender.Side] == 0 && s.Leader.Parameters[3] < s.rand128() && defender.Position.Cell() == s.Brain.TargetCell {
						s.Brain.TargetCell = -1
					}
					attacker.Condition >>= 1
					b.AutoSide = [2]Side{Player, Enemy}
					b.AutoPhase = [2]int{-2, -2}
					b.Done = true
					attacker.BattleID, defender.BattleID = 0, 0
					s.normalizeDeadSquads()
					s.reassessArmies(attacker.ID, defender.ID)
					s.log(fmt.Sprintf("%s won the battle", side), attacker.Position)
					if s.Outcome != Running || s.SurrenderOffered {
						if s.Outcome == Running {
							s.AutomaticCursor = i + 1
						}
						return
					}
				}
			}
		}
	}
	s.AutomaticCursor = 0
}

func (s *State) Zoom(squadID int) (*Battle, error) {
	if squadID < 0 || squadID >= 20 {
		return nil, fmt.Errorf("select a squad")
	}
	q := &s.Squads[squadID]
	if q.Side != Player || q.BattleID == 0 {
		return nil, fmt.Errorf("no encounter for this allied squad")
	}
	b := s.Battle(q.BattleID - 1)
	if b == nil || b.Done {
		return nil, fmt.Errorf("battle has ended")
	}
	if b.Started > 0 && !b.Manual {
		return nil, fmt.Errorf("fighting has already started; zoom is too late")
	}
	if !b.Manual {
		arena, err := s.newOriginalArena(b)
		if err != nil {
			return nil, err
		}
		if err := arena.stepActors(s.rand128, func(i int) bool { return arena.aimShot(i, s.rand128) }, arena.releaseShot); err != nil {
			return nil, err
		}
		arena.choosePalette(s.rand128)
		arena.Interactive = true
		b.Arena = arena
		b.Manual = true
		b.Created = s.Tick
		s.syncOriginalArena(b)
	}
	return b, nil
}

func (s *State) CommandBattle(id int, kind int, order Tactic) error {
	b := s.Battle(id)
	if b == nil || !b.Manual || b.Done || b.Arena == nil {
		return fmt.Errorf("no tactical battle")
	}
	if kind < -1 || kind > int(Barbarian) || order < Attack || order > Hold {
		return fmt.Errorf("invalid battle order")
	}
	a := b.Arena
	if a.Countdown != 0 || a.Retreat != 0 {
		return fmt.Errorf("the battle is ending")
	}
	if a.Started && a.Layout == 3 && (kind < 0 || kind == int(Archer)) {
		return fmt.Errorf("fort archers fire automatically; select knights or barbarians")
	}
	rawKind := 5
	if kind >= 0 {
		rawKind = [...]int{2, 0, 1}[kind]
	}
	a.Pending = legacyBattleCommand{Valid: true, Kind: rawKind, Order: [...]int{1, 2, 3, 0}[order]}
	return nil
}

func (s *State) Retreat(id int) error {
	b := s.Battle(id)
	if b == nil || !b.Manual || b.Done || b.Arena == nil {
		return fmt.Errorf("no tactical battle")
	}
	if b.Arena.Retreat == 0 && b.Arena.Countdown == 0 {
		b.Arena.Pending = legacyBattleCommand{Valid: true, Withdraw: true}
	}
	return nil
}

func (s *State) stepTactics(b *Battle) {
	// The original loop uses machine-dependent busy waits. The Go host gives
	// it a stable ten frames per second while retaining the exact frame logic.
	b.FrameClock++
	if b.FrameClock < 6 {
		return
	}
	b.FrameClock = 0
	if err := b.Arena.stepFrame(s.rand128); err != nil {
		s.Fault = err.Error()
		s.log("Tactical error: "+s.Fault, s.Squads[b.Squads[Player]].Position)
		return
	}
	s.syncOriginalArena(b)
	if b.Arena.Done {
		s.finishOriginalBattle(b)
	}
}

func (s *State) syncOriginalArena(b *Battle) {
	a := b.Arena
	for side := Player; side <= Enemy; side++ {
		q := &s.Squads[b.Squads[side]]
		q.Men, q.Condition, q.Formation, q.Position = a.Men[side], a.Conditions[side], a.Formations[side], a.Positions[side]
		s.Stats.Casualties[side] += a.Casualties[side] - b.AppliedCasualties[side]
		s.Stats.Captured[side] += a.Captured[side] - b.AppliedCaptured[side]
		b.AppliedCasualties[side], b.AppliedCaptured[side] = a.Casualties[side], a.Captured[side]
		if a.ClearRoutes[side] {
			s.originalCancelRoutes(q)
		}
	}
	if a.ResetEnemyAI {
		s.Squads[b.Squads[Enemy]].AIActive = false
	}
	if a.ClearObjective {
		s.Brain.TargetCell = -1
	}
	b.Attacking = a.Started
}

func (s *State) finishOriginalBattle(b *Battle) {
	b.Done = true
	s.TacticalScratch = b.Arena.Actors
	for side := Player; side <= Enemy; side++ {
		q := &s.Squads[b.Squads[side]]
		q.BattleID, q.Cooldown = 0, 0
		q.Condition >>= 1
		if q.Fighters() == 0 {
			s.originalCancelRoutes(q)
		}
	}
	winner := Player
	if b.Arena.Retreat == 1 || s.Squads[b.Squads[Player]].Fighters() == 0 {
		winner = Enemy
	}
	s.log(fmt.Sprintf("%s won the battle", winner), s.Squads[b.Squads[winner]].Position)
	b.Arena = nil
	// Returning from overlay 6 re-enters overlay 5's normalization, influence
	// and army assessment without repeating the five new-campaign decisions.
	s.normalizeDeadSquads()
	s.UpdateInfluence()
	s.reassessArmies(b.Squads[Player], b.Squads[Enemy])
}
