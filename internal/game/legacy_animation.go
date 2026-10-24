package game

import (
	"encoding/binary"
	"fmt"
)

const legacyPrimaryBase = 0x43ab
const legacyAuxBase = 0x822f

// legacyAnimationData contains the original game-specific pose programs, not
// executable CPU instructions. Their records are movement/frame/state triples.
type legacyAnimationData struct{ Primary, Auxiliary []byte }
type legacyPose struct {
	Kind, Side, X, Y, State, Action, Program, Sprite, Frame int
	Released, Escaped                                       bool
}

func newLegacyAnimation(primary, auxiliary []byte, layout int) (*legacyAnimationData, error) {
	if len(primary) < 0x1f3e || len(auxiliary) < 0x7ec {
		return nil, fmt.Errorf("truncated original tactical data")
	}
	d := &legacyAnimationData{Primary: append([]byte(nil), primary...), Auxiliary: append([]byte(nil), auxiliary...)}
	common, right, left := 0, 2, -1
	if layout == 1 {
		common, right, left = 80, -78, 79
	} else if layout == 2 {
		common, right, left = -80, 82, -81
	}
	for _, offset := range []int{0x1ef0, 0x1d7c, 0x1e7e, 0x1cae} {
		d.setPrimary(offset, common)
	}
	d.setAux(0x4e, common)
	for _, offset := range []int{0x1e62, 0x1c92, 0x1f0c, 0x1d98, 0x1e70, 0x1ca0} {
		d.setPrimary(offset, right)
	}
	d.setAux(0x150, right)
	d.setAux(0x40, right)
	for _, offset := range []int{0x1efe, 0x1d8a} {
		d.setPrimary(offset, left)
	}
	d.setAux(0x142, left)
	if layout == 3 {
		d.setPrimary(0x1efe, -81)
		d.setPrimary(0x1d8a, -81)
		d.setAux(0x142, -81)
		d.setPrimary(0x1f1a, -79)
		d.setPrimary(0x1da6, -79)
		d.setAux(0x150, 81)
		d.setPrimary(0x1f0c, 81)
		d.setPrimary(0x1d98, 81)
	}
	if layout == 4 {
		d.setPrimary(0x1ca0, -79)
		d.setPrimary(0x1e70, -79)
		d.setAux(0x40, -79)
		d.setPrimary(0x1e7e, 81)
		d.setPrimary(0x1cae, 81)
		d.setAux(0x4e, 81)
	}
	for x := 0; x < 80; x++ {
		low, high := 28, 77
		switch layout {
		case 1:
			low = 45 - (x >> 1)
			if x >= 31 {
				high = 91 - (x >> 1)
			}
		case 2:
			low = (x >> 1) + 5
		case 3:
			low = 10
			if x >= 55 {
				low, high = 5, 70
			}
		case 4:
			low = 10
			if x < 25 {
				low, high = 5, 70
			}
		case 5:
			low, high = 26, 45
		case 6:
			low, high = 18, 48
		case 7:
			low, high = 26, 53
		case 8, 10:
			low = 30
		}
		d.Auxiliary[0x274+x] = byte(low)
		d.Auxiliary[0x2c4+x] = byte(high)
	}
	return d, nil
}

func (d *legacyAnimationData) setPrimary(offset, value int) {
	binary.LittleEndian.PutUint16(d.Primary[offset:], uint16(value))
}
func (d *legacyAnimationData) setAux(offset, value int) {
	binary.LittleEndian.PutUint16(d.Auxiliary[offset:], uint16(value))
}

func (d *legacyAnimationData) patchRetreat(layout int) {
	if layout == 3 {
		d.setPrimary(0x1d8a, 0)
		d.setPrimary(0x1efe, 0)
		d.setAux(0x142, 0)
	} else if layout == 4 {
		d.setPrimary(0x1ca0, 2)
		d.setPrimary(0x1e70, 2)
		d.setAux(0x40, 2)
	}
}
func (d *legacyAnimationData) word(address int) (int, error) {
	address &= 65535
	if offset := address - legacyPrimaryBase; offset >= 0 && offset+2 <= len(d.Primary) {
		return int(int16(binary.LittleEndian.Uint16(d.Primary[offset:]))), nil
	}
	if offset := address - legacyAuxBase; offset >= 0 && offset+2 <= len(d.Auxiliary) {
		return int(int16(binary.LittleEndian.Uint16(d.Auxiliary[offset:]))), nil
	}
	return 0, fmt.Errorf("tactical program address %04x outside original data", address)
}

func (d *legacyAnimationData) bound(base, x int) int {
	offset := base + x
	if offset < 0 || offset >= len(d.Auxiliary) {
		return 0
	}
	return int(int8(d.Auxiliary[offset]))
}

// These are the pointer families initialized by overlay 6:2fe0 and selected by
// the boundary logic at 5eca. Kind order here is the DOS K/B/A order.
func legacyBoundaryProgram(table, kind int) int {
	p := func(offset int) int { return legacyPrimaryBase + offset }
	a := func(offset int) int { return legacyAuxBase + offset }
	programs := map[int][3]int{
		0x3416: {p(0x1ca0), p(0x1e70), a(0x40)},
		0x341c: {p(0x1d8a), p(0x1efe), a(0x142)},
		0x3452: {p(0x1dd0), a(0x0e), a(0x188)},
		0x342e: {p(0x1cca), p(0x1e9a), a(0x70)},
		0x3434: {p(0x1db4), p(0x1f28), a(0x16c)},
		0x3458: {p(0x1ce6), p(0x1eb6), a(0x8c)},
		0x3428: {p(0x1da6), p(0x1f1a), a(0x15e)},
		0x344c: {p(0x1cd8), p(0x1ea8), a(0x7e)},
		0x3446: {p(0x1dc2), a(0), a(0x17a)},
		0x3422: {p(0x1cbc), p(0x1e8c), a(0x62)},
	}
	return programs[table][kind]
}

// stepPose is the integer motion/animation routine at overlay 6:5eca. Boundary
// steering selects original programs; it does not invent continuous velocity.
func (d *legacyAnimationData) stepPose(a *legacyPose, layout, retreat int) error {
	if a.Kind < 0 || a.Kind > 2 {
		return fmt.Errorf("invalid original actor kind")
	}
	a.Released = false
	if a.State != 9 {
		if a.X < 73 || a.State == 3 {
			if a.X < 1 && a.State != 4 {
				if retreat != 0 {
					a.Sprite = -1
					a.X = word16(a.X - 10)
					a.Escaped = true
					return nil
				}
				a.Program = legacyBoundaryProgram(0x3416, a.Kind)
			} else if a.Y < d.bound(0x274, a.X) {
				table := 0x342e
				if a.Action == 101 {
					table = 0x3434
				}
				if layout == 1 && a.Action == 101 {
					table = 0x3452
				}
				if layout == 2 && a.Action != 101 {
					table = 0x3458
				}
				a.Program = legacyBoundaryProgram(table, a.Kind)
			} else if d.bound(0x2c4, a.X) < a.Y {
				table := 0x3422
				if a.Action == 101 {
					table = 0x3428
				}
				if layout == 1 && a.Action != 101 {
					table = 0x344c
				}
				if layout == 2 && a.Action == 101 {
					table = 0x3446
				}
				a.Program = legacyBoundaryProgram(table, a.Kind)
			}
		} else {
			if retreat != 0 {
				a.Sprite = -1
				a.X = word16(a.X + 10)
				a.Escaped = true
				return nil
			}
			a.Program = legacyBoundaryProgram(0x341c, a.Kind)
		}
	}
	delta, err := d.word(a.Program)
	if err != nil {
		return err
	}
	frame, err := d.word(a.Program + 2)
	if err != nil {
		return err
	}
	state, err := d.word(a.Program + 4)
	if err != nil {
		return err
	}
	a.Sprite = word16(a.Sprite + delta)
	if delta < -10 {
		a.Y = word16(a.Y - 2)
		a.X = word16(a.X + delta + 80)
		a.Sprite = word16(a.Sprite - 80)
	} else if delta < 11 {
		a.X = word16(a.X + delta)
	} else {
		a.Y = word16(a.Y + 2)
		a.X = word16(a.X + delta - 80)
		a.Sprite = word16(a.Sprite + 80)
	}
	a.Frame = frame
	a.State = state
	a.Program = (a.Program + 6) & 65535
	a.Released = a.State == 2 && a.Kind == 2
	return nil
}
