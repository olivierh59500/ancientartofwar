package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
)

func (g *Game) picture(name string) *ebiten.Image {
	if im := g.Pictures[name]; im != nil {
		return im
	}
	im, err := g.Library.Picture(name)
	if err != nil {
		return nil
	}
	g.Pictures[name] = ebiten.NewImageFromImage(im)
	return g.Pictures[name]
}

func (g *Game) drawBattleField(dst *ebiten.Image, id int, x, y, w, h float64) {
	palette, err := g.State.BattlePixels(id, g.BattlePixels[:])
	if err != nil {
		label(dst, err.Error(), int(x), int(y)+24, 1, red)
		return
	}
	for py := 0; py < 200; py++ {
		for px := 0; px < 320; px++ {
			packed := g.BattlePixels[(py&1)*8000+(py/2)*80+px/4]
			value := palette[(packed>>uint(6-(px&3)*2))&3]
			bright := (value >> 3) * 85
			r, green, blue := ((value>>2)&1)*170+bright, ((value>>1)&1)*170+bright, (value&1)*170+bright
			if value == 6 {
				green = 85
			}
			at := (py*320 + px) * 4
			g.BattleRGBA[at], g.BattleRGBA[at+1], g.BattleRGBA[at+2], g.BattleRGBA[at+3] = r, green, blue, 255
		}
	}
	if g.BattleTexture == nil {
		g.BattleTexture = ebiten.NewImage(320, 200)
	}
	g.BattleTexture.WritePixels(g.BattleRGBA[:])
	// The original CGA picture is displayed with the monitor's 4:3 aspect.
	fw, fh := w, w*3/4
	if fh > h {
		fw, fh = h*4/3, h
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(fw/320, fh/200)
	op.GeoM.Translate(x+(w-fw)/2, y+(h-fh)/2)
	dst.DrawImage(g.BattleTexture, op)
}
