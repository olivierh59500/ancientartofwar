package game

// The tactical engine keeps the original fixed 28 actor slots and integer
// coordinates. Rendering may scale these coordinates, but combat never uses
// normalized floating-point velocities or invented cooldowns.
type legacyActor struct {
	legacyPose
	Target, Shot int
}
type legacyProjectile struct{ X, Y, Sprite, Kind, Accuracy, Side int }
type legacyArena struct {
	Data                                      *legacyAnimationData `json:"-"`
	Actors                                    [28]legacyActor
	Projectiles                               [10]legacyProjectile
	Layout, Leader, Intro, Countdown, Retreat int
	Interactive, Demo, Started                bool
	Orders, Preferred, Budgets                [3]int
	Moved                                     [3]bool
	Skill                                     [2]int
	Men                                       [2][4]int
	Casualties, Captured                      [2]int
	ClearObjective                            bool
	Aggression, EnemyCell, Objective          int
	CurrentTarget                             int
	Positions                                 [2]Point
	Conditions, Formations                    [2]int
	ClearRoutes                               [2]bool
	ResetEnemyAI                              bool
	Done                                      bool
	Palette                                   [4]uint8
	Pending                                   legacyBattleCommand
	Draws                                     []legacyDraw
	Background                                []byte `json:"-"`
}

func originalSide(side int) Side {
	if side == -1 {
		return Player
	}
	return Enemy
}
func originalKind(kind int) Kind { return [...]Kind{Knight, Barbarian, Archer}[kind] }
func originalByte(value int) int { return int(int8(byte(value))) }

// 6f06 / 6fbf: targets persist until death, including the original special
// preference for a soldier who is already attacking this actor.
func (b *legacyArena) locateTarget(index int, random func() int) {
	a := &b.Actors[index]
	target := a.Target
	if target >= 0 && target < 28 && b.Actors[target].State == 9 {
		target = -1
	}
	if target >= 0 && target < 28 {
		b.CurrentTarget = target
		if abs(a.X-b.Actors[target].X) > 4 || abs(a.Y-b.Actors[target].Y) > 3 {
			b.CurrentTarget = -2
		}
		return
	}
	best := 500
	target = -1
	for i, other := range b.Actors {
		if other.Sprite == -1 || other.State == 9 || other.Side == a.Side {
			continue
		}
		dx, dy := abs(a.X-other.X), abs(a.Y-other.Y)
		if other.Target == index && best > 8 && (a.Side == 0 || b.Preferred[a.Kind] == other.Kind || dx+dy < 8) {
			best = 8
			target = i
		}
		if a.Kind == 1 && other.Kind == 2 || a.Kind == 0 && other.Kind == 1 {
			dx >>= 1
		}
		if dx+dy < best {
			best = dx + dy
			target = i
		}
	}
	if best == 500 && b.Countdown == 0 {
		b.Countdown = 7
		if b.Layout == 3 || b.Layout == 4 {
			b.Countdown = 9
		}
		loser := originalSide(a.Side).Other()
		b.Captured[loser] += b.Men[loser][Spy]
		b.Men[loser][Spy] = 0
		if a.Side == -1 {
			if b.Aggression < random() && b.Positions[Enemy].Cell() == b.Objective {
				b.ClearObjective = true
			}
		}
		for i := range b.Projectiles {
			b.Projectiles[i].X = 0
		}
	}
	a.Target = target
	b.CurrentTarget = target
	if target >= 0 && (abs(a.X-b.Actors[target].X) > 4 || abs(a.Y-b.Actors[target].Y) > 3) {
		b.CurrentTarget = -2
	}
}

// 7262: alignment consumes a random value even when fort terrain forces zero.
func (b *legacyArena) alignment(index int, random func() int) int {
	a := &b.Actors[index]
	target := a.Target
	b.CurrentTarget = target
	if target < 0 || target >= 28 {
		return 0
	}
	other := b.Actors[target]
	step := 0
	if a.Y < other.Y {
		step = 160
	} else if a.Y > other.Y {
		step = -160
	}
	if a.X-other.X < -4 {
		step++
	} else if a.X-other.X > 4 {
		step--
	}
	value := random()
	if value < 30 && b.Retreat == 0 || b.Layout == 3 || b.Layout == 4 {
		step = 0
	}
	return step
}

// 732f deliberately tests same-side body positions without excluding dead or
// inactive slots. Adding such a filter would change the original decisions.
func (b *legacyArena) avoidOrEngage(index int, random func() int) bool {
	a := &b.Actors[index]
	target := b.CurrentTarget
	if target < 0 || target >= 28 {
		return false
	}
	other := b.Actors[target]
	if abs(a.X-other.X) < 3 && abs(a.Y-other.Y) < 2 {
		table := 0x3410
		if a.Side == 0 {
			table = 0x340a
		}
		a.Program = legacyProgram(table, a.Kind)
		return true
	}
	for i, other := range b.Actors {
		if i == index || other.Side != a.Side || a.X-other.X <= -2 || a.X-other.X >= 2 || a.Y != other.Y {
			continue
		}
		table := 0x3452
		if random() < 65 {
			table = 0x3446
		}
		a.Program = legacyProgram(table, a.Kind)
		return true
	}
	return false
}

// 74ab is one adjacent sorting pass, not a complete sort. Every stored target
// index is rewritten after each swap, including references in inactive slots.
func (b *legacyArena) sortActors() {
	for i := 1; i < 28; i++ {
		left := i - 1
		if b.Actors[left].Sprite == -1 || b.Actors[i].Sprite == -1 || b.Actors[i].Y >= b.Actors[left].Y {
			continue
		}
		b.Actors[left], b.Actors[i] = b.Actors[i], b.Actors[left]
		for k := range b.Actors {
			if b.Actors[k].Target == left {
				b.Actors[k].Target = i
			} else if b.Actors[k].Target == i {
				b.Actors[k].Target = left
			}
		}
	}
}

// 6407: casualties select the original death program, including fort-wall
// falls. Existing in-flight arrows retain their independent projectile slot.
func (b *legacyArena) killActor(index int, random func() int) {
	a := &b.Actors[index]
	side := originalSide(a.Side)
	kind := originalKind(a.Kind)
	b.Casualties[side]++
	b.Men[side][kind] = (b.Men[side][kind] - 1) & 255
	if b.Layout == 3 && a.Side == -1 {
		offset := 0x1e24
		if a.Y < 42 {
			offset = 0x1e00
		} else if a.Y < 60 {
			offset = 0x1e0c
		}
		a.Program = legacyPrimaryBase + offset
	} else if b.Layout == 4 && a.Side == 0 {
		offset := 0x1d30
		if a.Y < 42 {
			offset = 0x1d12
		} else if a.Y < 60 {
			offset = 0x1d1e
		}
		a.Program = legacyPrimaryBase + offset
	} else if a.Action == 101 {
		offset := 0x1e38
		if a.X < 71 {
			offset = 0x1e46
		}
		a.Program = legacyPrimaryBase + offset
	} else {
		offset := 0x1d58
		if a.X < 10 {
			offset = 0x1d4a
		}
		a.Program = legacyPrimaryBase + offset
	}
	if b.Retreat == 0 {
		if originalByte(b.Men[side][Archer]) < 1 && (b.Layout == 3 || b.Layout == 4) {
			b.startRetreat(1+int(side), random)
		}
		if side == Enemy && b.Skill[Enemy] < 6 {
			value := random()
			if b.Skill[Enemy]+115 < value && b.Layout != 4 && b.Leader != 0 && b.Leader != 4 {
				b.startRetreat(2, random)
			}
		}
		if side == Player && b.Skill[Player] < 6 {
			value := random()
			if b.Skill[Player]+115 < value && b.Layout != 3 {
				b.startRetreat(3, random)
			}
		}
	}
	a.State = 9
	a.Shot = -1
}

// 4412 mutates map positions at the start of withdrawal, using a single random
// sample for both axes. Winner skill and morale changes are not symmetric.
func (b *legacyArena) startRetreat(side int, random func() int) {
	if b.Data != nil {
		b.Data.patchRetreat(b.Layout)
	}
	b.Intro = 0
	b.Countdown = 75
	b.Retreat = side
	value := random()
	dx, dy := (value&3)+1, value&7
	if b.Positions[Player].X-b.Positions[Enemy].X < 1 {
		dx = -dx
	}
	if b.Positions[Player].Y-b.Positions[Enemy].Y < 1 {
		dy = -dy
	}
	if b.Layout > 0 && b.Layout < 5 {
		dx *= 2
		dy *= 2
	}
	if side == 1 || side == 3 {
		b.Retreat = 1
		p := &b.Positions[Player]
		if p.X > 64 || p.X < 16 {
			dx = 0
		}
		if p.Y > 164 || p.Y < 21 {
			dy = 0
		}
		p.X = word16(p.X + dx)
		p.Y = word16(p.Y + dy)
		if b.Layout == 3 {
			b.Orders = [3]int{3, 3, 3}
			b.Budgets = [3]int{10, 10, 10}
			b.Formations[Player] = 1
		}
		b.ClearRoutes[Player] = true
		b.Conditions[Enemy] = min(98, b.Conditions[Enemy]*2+5)
		b.Skill[Enemy] = 85
	} else {
		p := &b.Positions[Enemy]
		if p.X > 64 || p.X < 16 {
			dx = 0
		}
		if p.Y > 164 || p.Y < 21 {
			dy = 0
		}
		p.X = word16(p.X - dx)
		p.Y = word16(p.Y - dy)
		b.ClearRoutes[Enemy] = true
		b.ResetEnemyAI = true
		b.Skill[Player] = 85
	}
}

func (b *legacyArena) melee(index int, random func() int) {
	a := &b.Actors[index]
	target := b.CurrentTarget
	if a.State != 2 || a.Kind == 2 || target < 0 || target >= 28 || b.Actors[target].State == 9 {
		return
	}
	other := b.Actors[target]
	canHit := other.Kind == 2 || b.Retreat != 0 || a.Kind == 0 && other.Kind == 1 && random() < 35 || random() < 90
	if canHit {
		value := random()
		if value < b.Skill[originalSide(a.Side)]+40 || b.Retreat != 0 {
			b.killActor(target, random)
			a.Target = -1
		}
	}
}
