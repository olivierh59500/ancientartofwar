package game

import (
	"encoding/binary"
	"fmt"
)

type legacyDraw struct{ Address, Sprite, Side int }

// decodeBattleBackground translates 8274's two word-run planes into a compact
// 16,000-byte framebuffer. The original gap holding O6 is not a display plane.
func decodeBattleBackground(data []byte) ([]byte, error) {
	pixels := make([]byte, 16000)
	at := 0
	for plane := 0; plane < 2; plane++ {
		if at+2 > len(data) {
			return nil, fmt.Errorf("truncated tactical background")
		}
		count := int(binary.LittleEndian.Uint16(data[at:]))
		at += 2
		out := plane * 8000
		for record := 0; record < count; record++ {
			if at+3 > len(data) {
				return nil, fmt.Errorf("truncated tactical background run")
			}
			n := int(data[at])
			if out+n*2 > (plane+1)*8000 {
				return nil, fmt.Errorf("tactical background exceeds plane")
			}
			for i := 0; i < n; i++ {
				pixels[out], pixels[out+1] = data[at+1], data[at+2]
				out += 2
			}
			at += 3
		}
		if out != (plane+1)*8000 {
			return nil, fmt.Errorf("incomplete tactical background plane")
		}
	}
	return pixels, nil
}

// drawOriginalSprite translates the packed word masks in resident 1c3d. The
// enemy's outline operation depends on existing pixels and adjacent bit pairs;
// replacing it with an RGBA tint changes the original artwork.
func (d *legacyAnimationData) drawOriginalSprite(pixels []byte, draw legacyDraw) error {
	var source []byte
	if at := draw.Address - legacyPrimaryBase; at >= 0 && at+4 <= len(d.Primary) {
		source = d.Primary[at:]
	} else if at := draw.Address - legacyAuxBase; at >= 0 && at+4 <= len(d.Auxiliary) {
		source = d.Auxiliary[at:]
	} else {
		return fmt.Errorf("tactical sprite address %04x outside original data", draw.Address)
	}
	width := int(binary.LittleEndian.Uint16(source))
	height := int(binary.LittleEndian.Uint16(source[2:]))
	if width < 1 || width > 40 || height < 1 || height > 100 || 4+width*height*4 > len(source) {
		return fmt.Errorf("invalid tactical sprite dimensions")
	}
	at := 4
	for row := 0; row < height; row++ {
		for plane := 0; plane < 2; plane++ {
			for col := 0; col < width; col++ {
				mask := binary.BigEndian.Uint16(source[at:])
				at += 2
				offset := draw.Sprite - 0x246b + row*80 + col*2
				if offset < 0 || offset+1 >= 8000 {
					continue
				}
				offset += plane * 8000
				old := binary.BigEndian.Uint16(pixels[offset:])
				value := mask | old & ^(mask>>2) & ^(mask<<2)
				if draw.Side == 0 {
					value = (old | mask>>2 | mask<<2) & ^mask
				}
				binary.BigEndian.PutUint16(pixels[offset:], value)
			}
		}
	}
	return nil
}

// BattlePixels supplies the actual original CGA pixels to either host. Drawing
// order is recorded while stepping actors, before the original adjacent sort.
func (s *State) BattlePixels(id int, destination []byte) ([4]uint8, error) {
	b := s.Battle(id)
	if b == nil || b.Arena == nil || len(destination) != 16000 {
		return [4]uint8{}, fmt.Errorf("no tactical framebuffer")
	}
	a := b.Arena
	if a.Background == nil {
		background, err := decodeBattleBackground(s.Tactical.Backgrounds[a.Layout])
		if err != nil {
			return a.Palette, err
		}
		a.Background = background
	}
	copy(destination, a.Background)
	for _, draw := range a.Draws {
		if err := a.Data.drawOriginalSprite(destination, draw); err != nil {
			return a.Palette, err
		}
	}
	return a.Palette, nil
}
