package ui

import (
	"image"
	"strings"

	"ancientartofwar/internal/game"
	"github.com/hajimehoshi/ebiten/v2"
)

func (g *Game) largeButton(dst *ebiten.Image, x, y, w, h int, title string, fn func()) {
	r := image.Rect(x, y, x+w, y+h)
	background := panel
	if image.Pt(g.Pointer.X, g.Pointer.Y).In(r) {
		background = line
	}
	rect(dst, float32(x), float32(y), float32(w), float32(h), background)
	stroke(dst, float32(x), float32(y), float32(w), float32(h), line)
	scale := 1.2
	if h >= 88 {
		scale = 1.8
	}
	label(dst, title, x+12, y+(h-int(13*scale))/2, scale, paper)
	g.Buttons = append(g.Buttons, button{r, title, fn})
}

func (g *Game) drawTouchExtras(dst *ebiten.Image) {
	if !g.Touch {
		return
	}
	if g.CanvasWidth >= 1250 {
		g.drawTouchDock(dst)
		return
	}
	if g.Scene != title && g.Scene != tactical {
		g.largeButton(dst, 930, 14, 70, 44, "BACK", g.Back)
		if g.Scene != help {
			g.largeButton(dst, 850, 14, 70, 44, "HELP", func() { g.HelpReturn = g.Scene; g.Scene = help })
		}
	}
	switch g.Scene {
	case campaigns:
		g.largeButton(dst, 40, 548, 180, 64, "PREVIOUS", func() { g.choose(g.SelectedCampaign - 1) })
		g.largeButton(dst, 232, 548, 180, 64, "NEXT", func() { g.choose(g.SelectedCampaign + 1) })
		g.largeButton(dst, 424, 548, 220, 64, "CONTINUE", func() { g.Scene = briefing })
	case war:
		if g.State == nil || g.Selected < 0 {
			return
		}
		for i, s := range []string{"SLOW", "MARCH", "FAST"} {
			pace := 3 - i
			id := g.Selected
			g.largeButton(dst, 544+i*152, 420, 140, 52, s, func() { g.result(g.State.SetPace(id, pace)) })
		}
	case editor:
		g.largeButton(dst, 848, 132, 150, 40, "SIDE", func() { g.EditorSide = g.EditorSide.Other() })
		title := "PAINT"
		if g.TouchErase {
			title = "ERASE"
		}
		g.largeButton(dst, 848, 180, 150, 40, title, func() { g.TouchErase = !g.TouchErase })
	case formations:
		if g.EditFormations && g.Formation >= 6 && g.FormationMan >= 0 {
			for i, k := range []game.Kind{game.Archer, game.Knight, game.Barbarian} {
				kind := k
				title := []string{"ARCHER", "KNIGHT", "BARB."}[i]
				g.largeButton(dst, 690+i*100, 504, 92, 40, title, func() { g.formationSource()[g.Formation].Kind[g.FormationMan] = kind })
			}
		}
	}
}

func (g *Game) drawTouchTactical(dst *ebiten.Image) {
	b := g.State.Battle(g.BattleID)
	if b == nil {
		return
	}
	g.top(dst, "TACTICAL COMMAND", "Choose a group, then give an order")
	status := "PAUSE"
	if g.Paused {
		status = "RESUME"
	}
	g.largeButton(dst, 832, 14, 168, 44, status, func() { g.Paused = !g.Paused })
	g.drawBattleField(dst, b.ID, 32, 88, 960, 414)
	groups := []int{-1, int(game.Archer), int(game.Knight), int(game.Barbarian)}
	for i, title := range []string{"ALL SOLDIERS", "ARCHERS", "KNIGHTS", "BARBARIANS"} {
		kind := groups[i]
		name := title
		if g.TacticGroup == kind || g.TacticGroup < -1 && kind == -1 {
			name = "* " + name
		}
		g.largeButton(dst, 32+i*244, 518, 228, 64, name, func() { g.TacticGroup = kind })
	}
	for i, title := range []string{"ATTACK", "FORWARD", "BACK OFF", "STOP", "RETREAT"} {
		order := i
		if i == 0 && !b.Attacking {
			title = "BEGIN BATTLE"
		}
		g.largeButton(dst, 32+i*196, 598, 176, 68, title, func() {
			if order == 4 {
				g.result(g.State.Retreat(b.ID))
				return
			}
			kind := g.TacticGroup
			if kind < -1 {
				kind = -1
			}
			g.result(g.State.CommandBattle(b.ID, kind, game.Tactic(order)))
		})
	}
	g.footer(dst, "Orders affect the selected group. Retreat withdraws the entire squad.")
}

func (g *Game) drawTouchEntry(dst *ebiten.Image) {
	g.top(dst, "WRITE THE "+strings.ToUpper(g.EntryField), "Use the game keyboard; DONE saves your changes")
	lines := wrap(g.EntryBuffer+"_", 110)
	lineLimit := 9
	keyY, keyStep, keyHeight := 338, 54, 46
	wide := g.CanvasWidth >= 1250
	if wide {
		lineLimit, keyY, keyStep, keyHeight = 4, 240, 96, 88
	}
	if len(lines) > lineLimit {
		lines = lines[len(lines)-lineLimit:]
	}
	paragraph(dst, lines, 40, 104, 24, paper)
	rows := []string{"1234567890", "QWERTYUIOP", "ASDFGHJKL'", "ZXCVBNM.,-"}
	for row, text := range rows {
		for col, r := range text {
			ch := string(r)
			if !g.KeyboardShift {
				ch = strings.ToLower(ch)
			}
			value := ch
			g.largeButton(dst, 40+col*94, keyY+row*keyStep, 88, keyHeight, ch, func() {
				limit := 80
				if g.EntryField == "story" {
					limit = 800
				}
				if len(g.EntryBuffer) < limit {
					g.EntryBuffer += value
				}
			})
		}
	}
	if wide {
		g.footer(dst, "Tap letters to type. DONE saves; CANCEL keeps the previous text.")
		return
	}
	g.largeButton(dst, 40, 558, 172, 52, "SHIFT", func() { g.KeyboardShift = !g.KeyboardShift })
	g.largeButton(dst, 226, 558, 266, 52, "SPACE", func() { g.EntryBuffer += " " })
	g.largeButton(dst, 506, 558, 222, 52, "BACKSPACE", func() {
		if len(g.EntryBuffer) > 0 {
			g.EntryBuffer = g.EntryBuffer[:len(g.EntryBuffer)-1]
		}
	})
	g.largeButton(dst, 742, 558, 238, 52, "NEW LINE", func() {
		if g.EntryField == "story" {
			g.EntryBuffer += "\n"
		}
	})
	g.largeButton(dst, 40, 630, 330, 60, "DONE", g.finishTouchEntry)
	g.largeButton(dst, 390, 630, 240, 60, "CANCEL", func() { g.Scene = editor })
	g.footer(dst, "SHIFT changes letter case. DONE saves; CANCEL keeps the previous text.")
}

func (g *Game) finishTouchEntry() {
	if g.EntryField == "story" {
		g.Pending.Story = g.EntryBuffer
	} else if strings.TrimSpace(g.EntryBuffer) != "" {
		g.Pending.Title = g.EntryBuffer
	}
	g.Scene = editor
}
