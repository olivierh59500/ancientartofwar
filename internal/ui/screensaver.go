package ui

import (
	"image/color"

	"ancientartofwar/internal/original"
	"github.com/hajimehoshi/ebiten/v2"
)

// Idle menus and paused games show a moving original knight on black. Both
// the sprite and its short hint move, leaving no fixed interface on the OLED.
func (g *Game) drawScreensaver(dst *ebiten.Image) {
	dst.Fill(color.Black)
	frame := (g.Frame / 8) % 4
	if g.ScreensaverSprites[frame] == nil {
		offset := [...]int{0, 0x74, 0x158, 0x74}[frame]
		im, err := original.Sprite(g.Library.Files["O6"], offset, true)
		if err != nil {
			return
		}
		g.ScreensaverSprites[frame] = ebiten.NewImageFromImage(im)
	}
	bounce := func(value, limit int) int {
		if limit < 1 {
			return 0
		}
		value %= limit * 2
		if value > limit {
			value = limit*2 - value
		}
		return value
	}
	x := 36 + bounce(g.Frame*3, g.CanvasWidth-350)
	y := 40 + bounce(g.Frame*2, height-230)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(4, 4)
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(g.ScreensaverSprites[frame], op)
	label(dst, "TOUCH TO CONTINUE", x, y+140, 1.4, paper)
}
