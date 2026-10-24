package game

// aimShot translates 77b7. Arrows reserve a slot before the bow animation
// releases it, and scan opponents in actor-slot order rather than by distance.
func (b *legacyArena) aimShot(index int, random func() int) bool {
	a := &b.Actors[index]
	slot := 0
	for slot < len(b.Projectiles) && b.Projectiles[slot].X != 0 {
		slot++
	}
	side := originalSide(a.Side)
	threshold := func(n int) int {
		value, _ := b.Data.word(legacyAuxBase + 2000 + n*2)
		return value
	}
	if slot == len(b.Projectiles) || threshold(originalByte(b.Men[side][Archer])-1) < random() && b.Retreat == 0 || b.Countdown > 0 && b.Countdown < 10 || a.Shot != -1 {
		return false
	}
	remaining := originalByte(b.Men[side.Other()][Archer]+b.Men[side.Other()][Knight]+b.Men[side.Other()][Barbarian]) - 1
	for _, other := range b.Actors {
		if other.Side == a.Side || other.Sprite == -1 || other.State == 9 || b.Layout == 3 && other.X >= 66 || b.Layout == 4 && other.X <= 14 {
			continue
		}
		kind, program := originalTrajectory(a.X, a.Y, other.X, other.Y)
		// The original selects the bow program even when this target's chance
		// subsequently fails. Failed attempts can therefore change the pose.
		if program != 0 {
			a.Program = legacyAuxBase + program
		}
		if kind != 0 && (random() < threshold(remaining) || b.Retreat != 0 || kind == 6 || kind == 16) {
			shot := &b.Projectiles[slot]
			shot.Kind, shot.X, shot.Y, shot.Sprite, shot.Side = kind, 9999, a.Y, a.Sprite+320, a.Side
			if kind == 11 {
				shot.Y += 3
				shot.Sprite += 320
			}
			a.Shot = slot
			return true
		}
		remaining--
	}
	return false
}

// originalTrajectory preserves the twenty discrete O7 trajectories, including
// the original asymmetric vertical tolerance and its unshootable steep angles.
func originalTrajectory(x, y, targetX, targetY int) (kind, program int) {
	if targetX-3 < x && x < targetX+3 {
		if targetY < y {
			return 1, 0x114
		}
		return 11, 0xee
	}
	if targetY-3 < y && y < targetY+4 {
		if targetX < x {
			return 16, 0x196
		}
		return 6, 0x9a
	}
	// These subtraction loops also see a retreating target at X=-1 for one
	// frame before it escapes. abs(x-targetX) is not equivalent in that case.
	ex, tx, ey, ty := x, targetX, y, targetY
	for ex != 0 && tx != 0 {
		ex, tx = word16(ex-1), word16(tx-1)
	}
	for ey != 0 && ty != 0 {
		ey, ty = word16(ey-1), word16(ty-1)
	}
	dx, dy := tx, ty
	if ex != 0 {
		dx = ex
	}
	if ey != 0 {
		dy = ey
	}
	wide := dy < dx
	ratio := dy / dx
	if wide {
		ratio = dx / dy
	}
	left, up := ex != 0, ey != 0
	if ratio == 1 && !wide {
		if tx < ex {
			if up {
				return 19, 0x1b6
			}
			return 13, 0x1d6
		}
		if up {
			return 3, 0xba
		}
		return 9, 0xd4
	}
	if ratio == 2 && !wide {
		if left {
			if up {
				return 20, 0x1b6
			}
			return 12, 0x1d6
		}
		if up {
			return 2, 0xba
		}
		return 10, 0xd4
	}
	if ratio == 2 || ratio == 1 && wide {
		if left {
			if up {
				return 18, 0x1b6
			}
			return 14, 0x1d6
		}
		if up {
			return 4, 0xba
		}
		return 8, 0xd4
	}
	if ratio < 3 || ratio > 9 || !wide {
		return 0, 0
	}
	if left {
		if up {
			return 17, 0x196
		}
		return 15, 0x196
	}
	if up {
		return 5, 0x9a
	}
	return 7, 0x9a
}

// 7e00: the animation's release pose activates the previously reserved arrow.
func (b *legacyArena) releaseShot(index int) {
	a := &b.Actors[index]
	if a.Shot != -1 {
		shot := &b.Projectiles[a.Shot]
		shot.X = a.X
		shot.Accuracy = b.Skill[originalSide(a.Side)]*2 + 40
		a.Shot = -1
	}
}

// 80db: flight uses signed byte deltas and a signed word framebuffer offset.
// The fort-wall cutoffs precede the general arena bounds.
func (b *legacyArena) advanceProjectiles() {
	for i := range b.Projectiles {
		shot := &b.Projectiles[i]
		if shot.X == 0 || shot.X == 9999 {
			continue
		}
		shot.Accuracy = word16(shot.Accuracy - 4)
		shot.X = word16(shot.X + b.Data.bound(0x220, shot.Kind))
		shot.Y = word16(shot.Y + b.Data.bound(0x235, shot.Kind))
		delta, _ := b.Data.word(legacyAuxBase + 0x24a + shot.Kind*2)
		shot.Sprite = word16(shot.Sprite + delta)
		if b.Layout == 3 && (shot.Y-5)>>3+60 < shot.X || b.Layout == 4 && shot.X < 20-((shot.Y-5)>>3) {
			shot.X = 0
		} else if shot.X < 0 || shot.X >= 77 || shot.Y < b.Data.bound(0x274, shot.X) || shot.Y >= 89 || shot.Kind >= 8 && shot.Kind <= 14 && shot.Y >= 83 {
			shot.X = 0
		} else {
			b.Draws = append(b.Draws, legacyDraw{Address: legacyArrowFrames[shot.Kind], Sprite: shot.Sprite, Side: shot.Side})
		}
	}
}

// 7eb1: friendly fire is asymmetric. The successive random tests are kept in
// executable order, including tests performed before rejecting an allied hit.
func (b *legacyArena) collideProjectiles(random func() int) {
	for i := range b.Projectiles {
		shot := &b.Projectiles[i]
		if shot.X == 0 || shot.X == 9999 {
			continue
		}
		width, height := b.Data.bound(0x1f6, shot.Kind), b.Data.bound(0x20b, shot.Kind)
		for index, a := range b.Actors {
			if a.State == 9 || a.Sprite == -1 || shot.Y <= a.Y-3 || shot.Y >= a.Y+height || shot.X < a.X || shot.X >= a.X+width {
				continue
			}
			if random() < shot.Accuracy && (random() < 85 || a.Kind == 2) && (shot.Side != 0 || a.Side != 0) && (shot.Side != -1 || a.Side != -1 || random() > 79 && b.Retreat == 0) && (a.Kind != 0 || random() < 89) && (a.Kind != 1 || random() < 51) {
				shot.X = 0
				b.killActor(index, random)
				break
			}
		}
	}
}
