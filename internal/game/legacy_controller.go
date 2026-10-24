package game

// stepActors translates 53be. Projectile allocation and release are explicit
// boundaries so the controller can be compared independently with the original
// routine, then wired to the original projectile subsystem.
func (b *legacyArena) stepActors(random func() int, aim func(int) bool, release func(int)) error {
	b.Moved = [3]bool{}
	b.Draws = b.Draws[:0]
	if b.Intro != 0 {
		if b.Intro < 11 {
			b.Intro--
		} else if b.Interactive && (b.Leader == 0 || (b.Leader == 2 || b.Leader == 5) && random() < 2 || random() == 5 && b.Leader != 3) {
			b.Intro = 7
			b.Started = true
			if !b.Demo {
				b.Orders = [3]int{}
			}
			if b.Layout == 3 {
				b.Orders[2] = 1
			}
		}
	}
	for index := range b.Actors {
		a := &b.Actors[index]
		if a.Sprite == -1 {
			continue
		}
		b.locateTarget(index, random)
		word, err := b.Data.word(a.Program)
		if err != nil {
			return err
		}
		if word > 100 {
			a.Action = word
			if b.Intro == 0 {
				if a.State == 9 {
					offset := 0x1e52
					if a.Action == 102 {
						offset = 0x1d64
					}
					a.Program = legacyPrimaryBase + offset
				} else if b.Countdown == 0 || b.Countdown > 3 && b.Retreat != 0 {
					if b.Orders[a.Kind] == 3 && a.Side == -1 {
						b.Moved[a.Kind] = true
						table := 0x343a
						if a.X < 72 && (a.Y < 74 || b.Layout != 3) {
							table = 0x3410
						}
						a.Program = legacyProgram(table, a.Kind)
					} else if b.Countdown == 0 || (b.Retreat != 1 || a.Side != -1) && (b.Retreat != 2 || a.Side != 0) {
						if a.Kind == 2 {
							b.archerProgram(index, random, aim)
						} else if b.CurrentTarget < 0 {
							if b.Orders[a.Kind] == 1 || a.Side == 0 {
								b.alignProgram(index, random)
							} else if b.Orders[a.Kind] == 2 {
								b.Moved[a.Kind] = true
								table := 0x341c
								band := (a.Y - 5) >> 3
								if b.Layout == 3 && a.X < band+56 || b.Layout == 4 && a.X < 37-band {
									table = 0x343a
								}
								a.Program = legacyProgram(table, a.Kind)
							} else {
								table := 0x343a
								if a.Action == 102 {
									table = 0x3440
								}
								a.Program = legacyProgram(table, a.Kind)
							}
						} else if !b.avoidOrEngage(index, random) {
							if b.Actors[b.CurrentTarget].X < a.X {
								if a.Kind == 1 && random() < 30 {
									a.Program = legacyAuxBase + 0x75c
								} else {
									a.Program = legacyProgram(0x3464, a.Kind)
								}
							} else {
								if a.Kind == 1 && random() < 30 {
									a.Program = legacyAuxBase + 0x76a
								} else {
									a.Program = legacyProgram(0x345e, a.Kind)
								}
							}
						}
					} else {
						table := 0x341c
						if b.Retreat == 1 {
							table = 0x3416
						}
						a.Program = legacyProgram(table, a.Kind)
					}
				} else {
					if b.Retreat == 1 && a.Side == -1 || b.Retreat == 2 && a.Side != -1 {
						continue
					}
					a.Program = legacyPrimaryBase + 0x1d6c
				}
			} else if b.Intro < 7 {
				if b.Intro < 4 {
					table := 0x3440
					if a.Action == 101 {
						table = 0x343a
					}
					a.Program = legacyProgram(table, a.Kind)
				} else {
					table := 0x3404
					if a.Action == 101 {
						table = 0x33fe
					}
					a.Program = legacyProgram(table, a.Kind)
				}
			} else {
				table := 0x33f2
				if a.Action == 101 {
					table = 0x33f8
				}
				a.Program = legacyProgram(table, a.Kind)
			}
		}
		b.melee(index, random)
		if err := b.Data.stepPose(&a.legacyPose, b.Layout, b.Retreat); err != nil {
			return err
		}
		if a.Sprite != -1 {
			b.Draws = append(b.Draws, legacyDraw{Address: legacyFrames[a.Frame], Sprite: a.Sprite, Side: a.Side})
		}
		if a.Released && release != nil {
			release(index)
		}
	}
	for kind, moved := range b.Moved {
		if moved {
			b.Budgets[kind]--
			if b.Budgets[kind] < 0 {
				b.Orders[kind] = 0
			}
		}
	}
	b.sortActors()
	return nil
}

func (b *legacyArena) alignProgram(index int, random func() int) {
	a := &b.Actors[index]
	step := b.alignment(index, random)
	table := 0
	switch step {
	case 0:
		table = 0x3440
		if a.Action == 101 {
			table = 0x343a
		}
	case -161:
		table = 0x3428
	case -160:
		table = 0x344c
		if a.Action == 101 {
			table = 0x3446
		}
	case -159:
		table = 0x3422
	case -1:
		table = 0x341c
		if b.Retreat != 0 {
			table = 0x3470
		}
	case 1:
		table = 0x3416
		if b.Retreat != 0 {
			table = 0x346a
		}
	case 161:
		table = 0x342e
	case 160:
		table = 0x3458
		if a.Action == 101 {
			table = 0x3452
		}
	case 159:
		table = 0x3434
	}
	if table != 0 {
		a.Program = legacyProgram(table, a.Kind)
	}
}

func (b *legacyArena) archerProgram(index int, random func() int, aim func(int) bool) {
	a := &b.Actors[index]
	if b.Orders[2] == 1 || a.Side == 0 {
		if aim != nil && aim(index) {
			return
		}
		if b.Layout == 4 || a.Side != 0 {
			if a.Side == 0 {
				a.Program = legacyAuxBase + 0x38
				return
			}
			value := random()
			offset := 0x17a
			if a.X < 38 {
				offset = 0x150
			} else if value < 105 || b.Layout == 3 {
				offset = 0x13a
			} else if value < 117 {
				offset = 0x188
			}
			a.Program = legacyAuxBase + offset
		} else {
			value := random()
			offset := 0x7e
			if value < 5 || b.Retreat == 1 && b.Layout != 3 {
				offset = 0x40
				if b.Layout == 3 && a.X > 45 {
					offset = 0x4e
				}
			} else if value < 10 && a.X > 5 || a.X > 42 {
				offset = 0x4e
			} else if value < 121 {
				offset = 0x8c
				if value < 114 {
					offset = 0x38
				}
			}
			a.Program = legacyAuxBase + offset
		}
	} else if b.Orders[2] == 2 {
		b.Moved[2] = true
		band := (a.Y - 5) >> 3
		offset := 0x142
		if b.Layout == 3 && a.X < band+56 || b.Layout == 4 && a.X < 37-band {
			offset = 0x13a
		}
		a.Program = legacyAuxBase + offset
	} else {
		offset := 0x38
		if a.Action == 101 {
			offset = 0x13a
		}
		a.Program = legacyAuxBase + offset
	}
}
