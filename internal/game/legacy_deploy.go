package game

func (s *State) newOriginalArena(b *Battle) (*legacyArena, error) {
	player, enemy := &s.Squads[b.Squads[Player]], &s.Squads[b.Squads[Enemy]]
	a := &legacyArena{Actors: s.TacticalScratch, Leader: s.LeaderIndex, Men: [2][4]int{player.Men, enemy.Men}, Positions: [2]Point{player.Position, enemy.Position}, Conditions: [2]int{player.Condition, enemy.Condition}, Formations: [2]int{player.Formation, enemy.Formation}, Objective: s.Brain.TargetCell, Aggression: s.Leader.Parameters[3], EnemyCell: enemy.Position.Cell()}
	if a.Formations[Player] == 0 {
		a.Formations[Player] = 1
	}
	a.Formations[Enemy] = originalEnemyFormation(s.LeaderIndex, a.Men, s.rand128)
	a.Layout = originalBattleLayout(s.World.Tile(player.Position), s.World.Tile(enemy.Position), s.rand128)
	if a.Layout == 3 {
		a.Formations[Player] = 0
	} else if a.Layout == 4 {
		a.Formations[Enemy] = 0
	}
	advantage := min(40, max(-40, s.ConditionAdvantage(player, enemy)*2))
	a.Skill = [2]int{45 + advantage, 45 - advantage}
	data, err := newLegacyAnimation(s.Tactical.Primary, s.Tactical.Auxiliary, a.Layout)
	if err != nil {
		return nil, err
	}
	a.Data = data
	a.deployActors(s.Formations, s.rand128)
	return a, nil
}

// TacticalResources contains the unchanged O6/O7 domain data. No executable
// code is interpreted by the native engine.
type TacticalResources struct {
	Primary, Auxiliary []byte
	Backgrounds        [14][]byte
}

func originalEnemyFormation(leader int, men [2][4]int, random func() int) int {
	value, formation := random(), 2
	switch leader {
	case 1, 4:
		if value < 90 {
			formation = 3
		}
	case 2:
		formation = 4
		if value < 64 {
			formation = 5
		}
	case 0:
		if value < 90 {
			formation = 4
		}
	case 3:
		formation = 1
		if value < 64 {
			formation = 5
		}
	default:
		if men[Enemy][Archer] == 0 || men[Enemy][Knight]+men[Enemy][Barbarian] < 5 {
			formation = 3
			if men[Enemy][Archer] == 0 && men[Player][Archer] != 0 {
				formation = 4
			}
		}
	}
	if men[Player][Archer] == 0 && men[Enemy][Archer] != 0 && random() < 100 {
		formation = 1
	}
	return formation
}

// 482a chooses fourteen distinct layouts from the map tile IDs. Forest and
// mountain scenery deliberately depend on the player's tile, as in the DOS
// branch order; they are not synthesized from a combined terrain category.
func originalBattleLayout(player, enemy byte, random func() int) int {
	random()
	hill := func(tile byte) bool { return tile == 5 || tile == 6 }
	bridge := func(tile byte) bool { return tile == 9 || tile == 10 }
	switch {
	case player == 3 && enemy == 3:
		return 0
	case enemy == 3:
		return 4
	case player == 3:
		return 3
	case player == 4 || enemy == 4:
		return 8
	case hill(player) && !hill(enemy):
		return 1
	case hill(enemy) && !hill(player):
		return 2
	case IsWaterTile(player) || IsWaterTile(enemy):
		return 6
	case player >= 58 && player <= 60 || player == 11 || player == 12 || player == 64 || player == 70 || player >= 16 && player <= 18:
		return 9
	case IsForestTile(player):
		return 7
	case IsMountainTile(player):
		return 10
	case bridge(player) || bridge(enemy):
		return 5
	case hill(player) && hill(enemy):
		return 11
	default:
		if random() < 75 {
			return 12
		}
		return 13
	}
}

// deployActors translates resident 0a21 and the placement loop in 2fe0. The
// shared actor slots are retained between battles: inactive coordinates matter
// to the original body-avoidance checks.
func (b *legacyArena) deployActors(formations [10]Formation, random func() int) {
	for i := range b.Actors {
		b.Actors[i].Sprite = -1
	}
	next := 0
	for side := Player; side <= Enemy; side++ {
		formation, remaining := formations[b.Formations[side]], b.Men[side]
		for i, preferred := range formation.Kind {
			a := &b.Actors[next]
			a.Kind = [...]int{2, 0, 1}[preferred]
			a.Side = 0
			if side == Player {
				a.Side = -1
			}
			kind := preferred
			if remaining[kind] == 0 {
				kind = Spy
				for _, candidate := range [...]Kind{Knight, Barbarian, Archer} {
					if remaining[candidate] != 0 {
						kind = candidate
						break
					}
				}
				if kind == Spy {
					break
				}
				a.Kind = [...]int{2, 0, 1}[kind]
			}
			remaining[kind]--
			a.X, a.Y, a.Sprite = formation.X[i], formation.Y[i], 0
			if kind == Archer {
				a.X -= 2
			}
			next++
		}
	}
	for i := range b.Actors {
		a := &b.Actors[i]
		a.Target, a.Shot, a.State = -1, -1, 0
		if a.Sprite == -1 {
			continue
		}
		a.Program, a.Action = legacyProgram(0x33f2, a.Kind), 102
		if a.Side == -1 {
			a.Program, a.Action = legacyProgram(0x33f8, a.Kind), 101
		}
		switch {
		case a.Side == -1 && b.Layout == 1 || a.Side == 0 && b.Layout == 2:
			a.Y -= (a.Y - 10) / 3
		case a.Side == -1 && b.Layout == 2 || a.Side == 0 && b.Layout == 1:
			a.Y += (80 - a.Y) >> 2
		case b.Layout == 5 || b.Layout == 6:
			a.Y -= 10
		}
		if a.Side == 0 {
			a.X = 76 - a.X
			if (b.Layout == 3 || b.Layout == 4) && a.Kind != 2 {
				a.X = 2
				if b.Layout == 4 {
					a.Y += 10
				}
			}
		}
		if low := b.Data.bound(0x274, a.X); a.Y < low {
			a.Y = low + random()/50
			a.X += random()/30 - 2
		}
		if high := b.Data.bound(0x2c4, a.X); a.Y > high {
			a.Y = high - random()/50
			a.X += random()/30 - 2
		}
		a.X = min(72, max(1, a.X))
		a.Sprite = 0x246b + a.Y*80 + a.X
	}
	b.Intro, b.Countdown, b.Retreat = 20, 0, 0
	for i := range b.Projectiles {
		b.sortActors()
		b.Projectiles[i].X = 0
	}
	b.Orders, b.Preferred, b.CurrentTarget = [3]int{}, [3]int{5, 5, 5}, -1
	if b.Demo {
		b.Orders = [3]int{1, 1, 1}
	}
}
