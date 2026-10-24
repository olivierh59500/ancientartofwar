package ui

import (
	"encoding/binary"
	"math"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// The event tone is synthesized in Go; no DOS speaker, process or recording is
// needed. Music and the original machine-dependent speaker driver are separate
// from the native rules engine.
type soundPlayer struct {
	context *audio.Context
	players []*audio.Player
}

func (g *Game) beep() {
	if g.audio == nil {
		g.audio = &soundPlayer{context: audio.NewContext(44100)}
	}
	buf := make([]byte, 4410*4)
	for i := 0; i < 4410; i++ {
		v := int16(math.Sin(float64(i)*2*math.Pi*660/44100) * 1800 * (1 - float64(i)/4410))
		binary.LittleEndian.PutUint16(buf[i*4:], uint16(v))
		binary.LittleEndian.PutUint16(buf[i*4+2:], uint16(v))
	}
	alive := g.audio.players[:0]
	for _, p := range g.audio.players {
		if p.IsPlaying() {
			alive = append(alive, p)
		} else {
			p.Close()
		}
	}
	g.audio.players = alive
	p := g.audio.context.NewPlayerFromBytes(buf)
	g.audio.players = append(g.audio.players, p)
	p.Play()
}
