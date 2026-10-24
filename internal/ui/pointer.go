package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type pointer struct {
	X, Y                           int
	Pressed, Down, Released, Right bool
	active                         bool
	id                             ebiten.TouchID
	ids                            []ebiten.TouchID
}

// updatePointer captures one finger until release, avoiding synthetic mouse
// events and accidental selection changes when a second finger touches down.
func (g *Game) updatePointer() {
	p := &g.Pointer
	p.Pressed, p.Released, p.Right = false, false, false
	p.ids = ebiten.AppendTouchIDs(p.ids[:0])
	if p.active {
		held := false
		for _, id := range p.ids {
			if id == p.id {
				held = true
				break
			}
		}
		if held {
			p.X, p.Y = ebiten.TouchPosition(p.id)
			p.Down = true
			return
		}
		p.active = false
		p.Down = false
		p.Released = true
		return
	}
	pressed := inpututil.AppendJustPressedTouchIDs(nil)
	if len(pressed) > 0 {
		p.id = pressed[0]
		p.active = true
		p.X, p.Y = ebiten.TouchPosition(p.id)
		p.Pressed = true
		p.Down = true
		return
	}
	p.X, p.Y = ebiten.CursorPosition()
	p.Pressed = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	p.Down = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	p.Released = inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
	p.Right = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
}

func (g *Game) Suspend() {
	g.Paused = true
	if g.State != nil {
		g.save()
	}
}
func (g *Game) Back() {
	g.LastInputFrame = g.Frame
	if g.Screensaver {
		g.Screensaver = false
		return
	}
	switch g.Scene {
	case war:
		g.Paused = true
		g.DrawingRoute = false
		g.Scene = title
	case tactical:
		g.Paused = !g.Paused
	case campaigns:
		g.Scene = title
	case briefing:
		g.Scene = campaigns
	case rules, leaders:
		if g.Return == editor {
			g.Scene = editor
			g.Return = title
		} else {
			g.Scene = briefing
		}
	case formations, options:
		g.Scene = g.Return
	case transfer:
		g.Scene = war
	case editor:
		g.choose(g.SelectedCampaign)
		g.Scene = campaigns
	case textEntry:
		g.Scene = editor
	case help:
		g.Scene = g.HelpReturn
	}
}

func (g *Game) toggleRoute() {
	if g.DrawingRoute {
		g.commitRoute()
		return
	}
	if g.State == nil || g.Selected < 0 {
		return
	}
	q := &g.State.Squads[g.Selected]
	if q.Side != 0 || !q.Alive() || q.BattleID != 0 {
		g.toast("Select an allied squad outside a battle")
		return
	}
	g.DrawingRoute = true
	g.Waypoints = nil
	g.Cursor = q.Position
}
