package game

type legacyBattleCommand struct {
	Valid, Withdraw bool
	Kind, Order     int
}

func (b *legacyArena) start() {
	b.Intro, b.Started = 7, true
	b.Orders = [3]int{}
	if b.Layout == 3 {
		b.Orders[2] = 1
	}
}

// Commands enter at the same point as 67a0 in the original frame: after actor
// and projectile updates. The initial A starts the engagement; subsequent
// orders act on the selected group. Fort defenders' archers fire on their own.
func (b *legacyArena) applyCommand() {
	c := b.Pending
	b.Pending = legacyBattleCommand{}
	if !c.Valid {
		return
	}
	if !b.Started {
		if c.Order == 1 {
			b.start()
		}
		return
	}
	if c.Order == 1 && b.Layout == 3 {
		return
	}
	for kind := range b.Orders {
		if c.Kind != 5 && c.Kind != kind {
			continue
		}
		b.Orders[kind] = c.Order
		if c.Order == 2 || c.Order == 3 {
			b.Budgets[kind] = 15
		}
	}
}

func (b *legacyArena) stepFrame(random func() int) error {
	if err := b.stepActors(random, func(i int) bool { return b.aimShot(i, random) }, b.releaseShot); err != nil {
		return err
	}
	b.advanceProjectiles()
	b.collideProjectiles(random)
	b.finishFrame(random)
	return nil
}

// The rest of 2fe0 advances the end-of-battle countdown, shortens withdrawal
// after the last soldier escapes, and handles the two fort attack exceptions.
func (b *legacyArena) finishFrame(random func() int) {
	if b.Countdown == 0 {
		withdraw := b.Pending.Valid && b.Pending.Withdraw
		if withdraw {
			b.Pending = legacyBattleCommand{}
		} else {
			b.applyCommand()
		}
		if withdraw && b.Retreat == 0 {
			b.startRetreat(1, random)
		}
		value := random()
		if value < 3 && b.Intro == 0 && b.Retreat == 0 {
			if b.Layout == 3 && originalByte(b.Men[Enemy][Archer]) < value && (b.Men[Enemy][Archer] == 0 || random() <= 19) {
				b.startRetreat(2, random)
			}
			if b.Layout == 4 && originalByte(b.Men[Player][Archer]) < value && (b.Men[Enemy][Archer] == 0 || random() <= 19) {
				b.startRetreat(1, random)
			}
		}
		return
	}
	b.Countdown--
	if b.Countdown < 1 {
		b.Done = true
	}
	if b.Countdown == 1 {
		for i := range b.Projectiles {
			b.Projectiles[i].X = 0
		}
	}
	remaining := 1
	if b.Retreat == 1 || b.Retreat == 2 {
		remaining = 0
		for _, a := range b.Actors {
			if a.Sprite != -1 && a.State != 9 && originalSide(a.Side) == Side(b.Retreat-1) {
				remaining++
			}
		}
	}
	if remaining == 0 && b.Countdown > 8 {
		if b.Layout == 3 || b.Layout == 4 {
			if b.Countdown > 15 {
				b.Countdown = 16
			}
		} else {
			b.Countdown = 8
		}
	}
}

// 4d61 chooses one of two EGA palettes and consumes exactly one random sample.
func (b *legacyArena) choosePalette(random func() int) {
	palettes := [14][2][4]uint8{
		{{2, 3, 12, 15}, {2, 6, 10, 15}},
		{{2, 12, 11, 15}, {2, 4, 11, 15}},
		{{0, 2, 14, 15}, {0, 3, 4, 15}},
		{{0, 2, 12, 15}, {0, 3, 4, 15}},
		{{0, 3, 4, 15}, {0, 10, 12, 15}},
		{{0, 6, 12, 15}, {0, 10, 6, 15}},
		{{0, 14, 1, 15}, {0, 10, 1, 15}},
		{{0, 10, 12, 14}, {0, 10, 8, 14}},
		{{0, 2, 4, 15}, {0, 6, 12, 15}},
		{{0, 10, 12, 14}, {0, 10, 8, 14}},
		{{0, 3, 5, 14}, {0, 2, 8, 14}},
		{{0, 2, 1, 15}, {0, 2, 4, 15}},
		{{0, 10, 14, 15}, {0, 14, 12, 15}},
		{{0, 2, 7, 14}, {2, 11, 7, 15}},
	}
	variant := 0
	if random() >= 64 {
		variant = 1
	}
	b.Palette = palettes[b.Layout][variant]
}
