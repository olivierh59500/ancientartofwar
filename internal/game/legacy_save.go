package game

import "fmt"

func (b *legacyArena) validate() error {
	if b.Layout < 0 || b.Layout > 13 || b.Leader < 0 || b.Leader > 7 || b.Intro < 0 || b.Intro > 20 || b.Countdown < 0 || b.Countdown > 75 || b.Retreat < 0 || b.Retreat > 2 || b.Data == nil {
		return fmt.Errorf("invalid tactical arena")
	}
	if b.Pending.Valid && (b.Pending.Kind < 0 || b.Pending.Kind > 2 && b.Pending.Kind != 5 || b.Pending.Order < 0 || b.Pending.Order > 3) {
		return fmt.Errorf("invalid tactical command")
	}
	for side := Player; side <= Enemy; side++ {
		count := 0
		for _, n := range b.Men[side] {
			if n < 0 || n > 14 {
				return fmt.Errorf("invalid tactical troop count")
			}
			count += n
		}
		if count-b.Men[side][Spy] > 14 || b.Skill[side] < 5 || b.Skill[side] > 85 || b.Conditions[side] < 0 || b.Conditions[side] > 98 || b.Formations[side] < 0 || b.Formations[side] > 9 || !b.Positions[side].Valid() {
			return fmt.Errorf("invalid tactical army")
		}
	}
	for kind, order := range b.Orders {
		if order < 0 || order > 3 || b.Preferred[kind] < 0 || b.Preferred[kind] > 2 && b.Preferred[kind] != 5 {
			return fmt.Errorf("invalid tactical group order")
		}
	}
	for _, a := range b.Actors {
		if a.Kind < 0 || a.Kind > 2 || a.Side < -1 || a.Side > 0 || a.State < 0 || a.State > 9 || a.Target < -1 || a.Target >= 28 || a.Shot < -1 || a.Shot >= 10 || a.Frame < 0 || a.Frame >= len(legacyFrames) {
			return fmt.Errorf("invalid tactical actor")
		}
		if a.Sprite != -1 {
			if a.X < -20 || a.X > 100 || a.Y < -32 || a.Y > 128 {
				return fmt.Errorf("tactical actor outside arena")
			}
			if _, err := b.Data.word(a.Program); err != nil {
				return err
			}
		}
	}
	for _, shot := range b.Projectiles {
		if shot.Kind < 0 || shot.Kind > 20 || shot.Side < -1 || shot.Side > 0 || shot.X != 9999 && (shot.X < -10 || shot.X > 100) || shot.Y < -20 || shot.Y > 110 || shot.Accuracy < -32768 || shot.Accuracy > 32767 {
			return fmt.Errorf("invalid tactical projectile")
		}
	}
	for _, c := range b.Palette {
		if c > 15 {
			return fmt.Errorf("invalid tactical palette")
		}
	}
	return nil
}
