package game

import (
	"encoding/json"
	"fmt"
	"io"
)

func (s *State) Encode(w io.Writer) error {
	if err := s.Validate(); err != nil {
		return err
	}
	e := json.NewEncoder(w)
	return e.Encode(s)
}
func Decode(r io.Reader) (*State, error) {
	d := json.NewDecoder(io.LimitReader(r, 16<<20))
	d.DisallowUnknownFields()
	var s State
	if err := d.Decode(&s); err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("save: trailing data")
	}
	if s.Version == 1 {
		for _, b := range s.Battles {
			if b.Squads[Player] < 0 || b.Squads[Player] >= MaxSquads || b.Squads[Enemy] < 0 || b.Squads[Enemy] >= MaxSquads {
				return nil, fmt.Errorf("save: invalid legacy battle reference")
			}
		}
		s.upgradeSaveV1()
	} else if s.Version != StateVersion {
		return nil, fmt.Errorf("save: unsupported version %d", s.Version)
	}
	for i := range s.Battles {
		if a := s.Battles[i].Arena; a != nil {
			data, err := newLegacyAnimation(s.Tactical.Primary, s.Tactical.Auxiliary, a.Layout)
			if err != nil {
				return nil, fmt.Errorf("save: %w", err)
			}
			if a.Retreat != 0 {
				data.patchRetreat(a.Layout)
			}
			a.Data = data
		}
	}
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}
	return &s, nil
}

func (s *State) Validate() error {
	if s.Version != StateVersion || s.Random == 0 || s.Tick < 0 || s.WorldTick < 0 || s.WorldTick > s.Tick || s.Outcome < Running || s.Outcome > EnemyWon || s.LeaderIndex < 0 || s.LeaderIndex > 7 || s.RouteCursor < 0 || s.RouteCursor >= 1000 {
		return fmt.Errorf("invalid state header")
	}
	if s.WorldResume != 0 && s.WorldResume != 1 && s.WorldResume != 3 && s.WorldResume != 5 || s.AutomaticCursor < 0 || s.AutomaticCursor > MaxSquads || s.EncounterCursor < 0 || s.EncounterCursor > MaxSquads || s.MovementCursor < 0 || s.MovementCursor > MaxSquads {
		return fmt.Errorf("invalid world continuation")
	}
	if s.AICursor < -1 || s.AICursor >= MaxSquads || s.ReservedSquad < -1 || s.ReservedSquad >= MaxSquads {
		return fmt.Errorf("invalid squad cursor")
	}
	if err := s.Rules.Validate(); err != nil {
		return err
	}
	if s.Options.FightDelay < 0 || s.Options.FightDelay > 36000 {
		return fmt.Errorf("invalid fight delay")
	}
	if len(s.Flags) > MaxFlags || len(s.Events) > 40 || len(s.Battles) > 20000 {
		return fmt.Errorf("state exceeds object limits")
	}
	for _, t := range s.World.Tiles {
		if t < 1 || t > 72 {
			return fmt.Errorf("invalid map tile")
		}
	}
	for _, code := range s.RoutePool {
		if code != 0 && code != 1 && code != 255 && code != 79 && code != 80 && code != 81 && code != 175 && code != 176 && code != 177 && code != 99 {
			return fmt.Errorf("invalid shared route byte")
		}
	}
	routeSteps := 0
	for i, q := range s.Squads {
		routeSteps += len(q.Route)
		if q.ID != i || q.Side < Player || q.Side > Enemy || q.Fighters() > 14 || len(q.Route) > MaxRouteSteps || q.AITarget < -1 || q.AITarget > 199 {
			return fmt.Errorf("invalid squad %d", i)
		}
		if q.CountCache < 0 || q.CountCache > 28 || q.CountCached && q.CountCache > q.Count() {
			return fmt.Errorf("invalid cached troop count")
		}
		if q.FineActive && (q.FineOffset < 0 || q.FineOffset >= 1000) || q.AIPathActive && (q.AIPathOffset < 0 || q.AIPathOffset >= 200) {
			return fmt.Errorf("invalid shared route pointer")
		}
		for _, n := range q.Men {
			if n < 0 || n > 14 {
				return fmt.Errorf("invalid troop count")
			}
		}
		maxCondition := 49
		if q.BattleID != 0 {
			if b := s.Battle(q.BattleID - 1); b != nil && b.Manual {
				maxCondition = 98
			}
		}
		if q.Alive() && (!q.Position.Valid() || q.Pace < 1 || q.Pace > 3 || q.Formation < 0 || q.Formation > 9 || q.Condition < 0 || q.Condition > maxCondition || q.Food < 1 || q.Food > 49) {
			return fmt.Errorf("invalid active squad %d", i)
		}
		for _, p := range q.Route {
			if !p.Valid() {
				return fmt.Errorf("invalid route")
			}
		}
		if q.BattleID != 0 {
			b := s.Battle(q.BattleID - 1)
			if b == nil || b.Done || b.Squads[q.Side] != i {
				return fmt.Errorf("invalid battle reference")
			}
		}
	}
	if routeSteps > MaxRouteSteps {
		return fmt.Errorf("shared route budget exceeded")
	}
	if s.Brain.Initialized {
		if s.Brain.HomeCell < 0 || s.Brain.HomeCell >= 200 || s.Brain.TargetCell < -1 || s.Brain.TargetCell >= 200 || s.Brain.HomeFlag < -1 || s.Brain.HomeFlag >= len(s.Flags) || s.Brain.TargetFlag < -1 || s.Brain.TargetFlag >= len(s.Flags) {
			return fmt.Errorf("invalid commander objectives")
		}
		for _, d := range s.Brain.PathPool {
			if d > 8 && d != 99 {
				return fmt.Errorf("invalid AI route direction")
			}
		}
	}
	for _, f := range s.Flags {
		if !f.Position.Valid() || f.Side < Player || f.Side > Enemy {
			return fmt.Errorf("invalid flag")
		}
	}
	seen := make(map[int]bool)
	for _, b := range s.Battles {
		if b.ID < 0 || b.ID >= s.NextBattleID || seen[b.ID] || b.Squads[0] < 0 || b.Squads[0] >= 20 || b.Squads[1] < 0 || b.Squads[1] >= 20 || b.FrameClock < 0 || b.FrameClock > 5 {
			return fmt.Errorf("invalid battle")
		}
		seen[b.ID] = true
		for side := Player; side <= Enemy; side++ {
			if b.AutoPhase[side] < -2 || b.AutoPhase[side] > 36000 || b.AutoChance[side] < 0 || b.AutoChance[side] > 240 || b.AutoSide[side] < Player || b.AutoSide[side] > Enemy {
				return fmt.Errorf("invalid automatic combat state")
			}
		}
		if b.Manual && !b.Done {
			if b.Arena == nil {
				return fmt.Errorf("tactical arena missing")
			}
			if err := b.Arena.validate(); err != nil {
				return err
			}
		}
	}
	for _, formation := range s.Formations {
		for i, k := range formation.Kind {
			if k < Archer || k > Barbarian || formation.X[i] < 0 || formation.X[i] > 79 || formation.Y[i] < 0 || formation.Y[i] > 79 {
				return fmt.Errorf("invalid formation")
			}
		}
	}
	return nil
}

// Version 1 had no separate world clock or cached automatic-combat markers.
// Preserve its current timing phase and reconstruct those missing markers at
// load time. Version 2 records them exactly, including interrupted world ticks.
func (s *State) upgradeSaveV1() {
	s.Version = StateVersion
	if s.WorldTick == 0 {
		s.WorldTick = s.Tick
	}
	for i := range s.Squads {
		q := &s.Squads[i]
		q.Cooldown = 0
		q.Sighted = q.Side == Enemy && q.Visible || q.BattleID != 0
	}
	for i := range s.Battles {
		b := &s.Battles[i]
		if b.Manual || b.Done || b.AutoPhase != [2]int{} {
			continue
		}
		b.AutoSide = [2]Side{Player, Enemy}
		if b.Started == 0 {
			phase := min(max(1, s.Options.FightDelay), max(1, s.Tick-b.Created+1))
			b.AutoPhase = [2]int{phase, phase}
			continue
		}
		b.AutoPhase = [2]int{-1, 0}
		for side := Player; side <= Enemy; side++ {
			a, d := &s.Squads[b.Squads[side]], &s.Squads[b.Squads[side.Other()]]
			b.AutoChance[side] = s.ConditionAdvantage(a, d)*2 + 60
			if s.World.Tile(a.Position) == 3 && a.Men[Archer] > 0 {
				b.AutoChance[side] <<= 1
			}
		}
	}
}
