package original

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// Picture decodes the alternating byte-run streams used by the full-screen
// DOS artwork (resident overlay 1, 1000:8754). No graphics emulator is involved.
func (l *Library) Picture(name string) (*image.RGBA, error) {
	b, ok := l.Files[name]
	if !ok {
		return nil, fmt.Errorf("missing picture %s", name)
	}
	var planes [2][]byte
	at, side := 0, 0
	for len(planes[0]) < 8000 || len(planes[1]) < 8000 {
		if len(planes[side]) == 8000 {
			side = 1 - side
			continue
		}
		if at+2 > len(b) {
			return nil, fmt.Errorf("%s: truncated picture stream", name)
		}
		n, v := int(b[at]), b[at+1]
		at += 2
		if len(planes[side])+n > 8000 {
			return nil, fmt.Errorf("%s: picture run exceeds plane", name)
		}
		for i := 0; i < n; i++ {
			planes[side] = append(planes[side], v)
		}
		side = 1 - side
	}
	im := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			b := planes[y%2][(y/2)*80+x/4]
			im.SetRGBA(x, y, CGA[b>>uint(6-(x%4)*2)&3])
		}
	}
	return im, nil
}

// Sprite decodes one variable-size CGA PUT record. The two scanlines of each
// row pair are consecutive in the file, as in M and O6.
func Sprite(b []byte, offset int, transparent bool) (*image.RGBA, error) {
	if offset < 0 || offset+4 > len(b) {
		return nil, fmt.Errorf("sprite header outside file")
	}
	w, h := int(binary.LittleEndian.Uint16(b[offset:])), int(binary.LittleEndian.Uint16(b[offset+2:]))
	if w < 1 || w > 80 || h < 1 || h > 200 || offset+4+w*h*4 > len(b) {
		return nil, fmt.Errorf("invalid sprite dimensions")
	}
	im := image.NewRGBA(image.Rect(0, 0, w*8, h*2))
	data := b[offset+4 : offset+4+w*h*4]
	for y := 0; y < h*2; y++ {
		for x := 0; x < w*8; x++ {
			v := data[y*w*2+x/4] >> uint(6-(x%4)*2) & 3
			c := CGA[v]
			if transparent && v == 0 {
				c = color.RGBA{}
			}
			im.SetRGBA(x, y, c)
		}
	}
	return im, nil
}
