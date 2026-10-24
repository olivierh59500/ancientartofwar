package ui

import (
	"fmt"

	"ancientartofwar/internal/game"
	"github.com/hajimehoshi/ebiten/v2"
)

// Wide phones keep the original board readable and use the remaining screen
// for large controls. All actions still enter through the shared Go UI.
func (g *Game) drawTouchDock(dst *ebiten.Image) {
	left, available := width+24, g.CanvasWidth-width-48
	rect(dst, width+8, 24, 1, height-48, line)
	label(dst, "ORDERS", left, 26, 1.8, gold)
	type action struct {
		title string
		call  func()
	}
	var actions []action
	add := func(title string, call func()) { actions = append(actions, action{title, call}) }
	back := func() { add("BACK", g.Back) }
	openHelp := func() { add("FIELD MANUAL", func() { g.HelpReturn = g.Scene; g.Scene = help }) }
	pause := func() {
		title := "PAUSE"
		if g.Paused {
			title = "RESUME"
		}
		add(title, func() { g.Paused = !g.Paused })
	}
	switch g.Scene {
	case title:
		add("GO TO WAR", func() { g.choose(g.SelectedCampaign); g.Scene = campaigns })
		add("LOAD GAME", g.load)
		if g.State != nil {
			add("RESUME GAME", func() { g.Scene = war; g.Paused = true; g.refreshMap(&g.State.World) })
		}
		add("FORMATIONS", func() { g.openFormation(true) })
		add("GAME EDITOR", func() { g.choose(0); g.beginEditor(false) })
		add("OPTIONS", func() { g.Return = title; g.Scene = options })
		openHelp()
	case campaigns:
		add("PREVIOUS", func() { g.choose(g.SelectedCampaign - 1) })
		add("NEXT", func() { g.choose(g.SelectedCampaign + 1) })
		add("CONTINUE", func() { g.Scene = briefing })
		add("EDIT A COPY", func() { g.beginEditor(false) })
		add("NEW CAMPAIGN", func() { g.beginEditor(true) })
		back()
	case briefing:
		add("STORY UP", func() { g.StoryScroll = max(0, g.StoryScroll-5) })
		add("STORY DOWN", func() { g.StoryScroll += 5 })
		add("CAMPAIGN RULES", func() { g.Scene = rules })
		back()
		openHelp()
	case rules:
		add("PREVIOUS RULE", func() { g.RuleIndex = (g.RuleIndex + 8) % 9 })
		add("NEXT RULE", func() { g.RuleIndex = (g.RuleIndex + 1) % 9 })
		add("CHANGE VALUE", func() { g.cycleRule(g.RuleIndex) })
		add("CONTINUE", func() {
			if g.Return == editor {
				g.Scene, g.Return = editor, title
			} else {
				g.Scene = leaders
			}
		})
		back()
		openHelp()
	case leaders:
		add("PREVIOUS", func() { g.SelectedLeader = (g.SelectedLeader + 7) % 8 })
		add("NEXT", func() { g.SelectedLeader = (g.SelectedLeader + 1) % 8 })
		add("CONTINUE", func() {
			if g.Return == editor {
				g.Pending.Opponent, g.Scene, g.Return = g.SelectedLeader, editor, title
			} else {
				g.begin()
			}
		})
		back()
		openHelp()
	case war:
		if g.State != nil && g.State.Outcome != game.Running {
			add("CAMPAIGNS", func() { g.choose(g.SelectedCampaign); g.Scene = campaigns })
			add("SAVE GAME", g.save)
			add("MENU", g.Back)
			openHelp()
			break
		}
		if g.State != nil && g.State.SurrenderOffered {
			add("ACCEPT SURRENDER", g.State.AcceptSurrender)
			add("CONTINUE THE WAR", g.State.DeclineSurrender)
			add("SAVE GAME", g.save)
			add("MENU", g.Back)
			openHelp()
			break
		}
		pause()
		move := "MOVE"
		if g.DrawingRoute {
			move = "GO"
		}
		add(move, g.toggleRoute)
		add("ZOOM BATTLE", g.zoom)
		add("FORMATION", func() { g.openFormation(false) })
		for _, item := range []struct {
			title string
			pace  int
		}{{"SLOW", 3}, {"MARCH", 2}, {"FAST", 1}, {"STOP", 0}} {
			pace := item.pace
			add(item.title, func() { g.result(g.State.SetPace(g.Selected, pace)) })
		}
		add("DETACH MEN", func() { g.openTransfer(false) })
		add("JOIN MEN", func() { g.openTransfer(true) })
		add("SAVE GAME", g.save)
		add("MENU", g.Back)
	case tactical:
		for i, title := range []string{"ALL SOLDIERS", "ARCHERS", "KNIGHTS", "BARBARIANS"} {
			kind := []int{-1, int(game.Archer), int(game.Knight), int(game.Barbarian)}[i]
			if g.TacticGroup == kind || g.TacticGroup < -1 && kind == -1 {
				title = "* " + title
			}
			add(title, func() { g.TacticGroup = kind })
		}
		for i, title := range []string{"ATTACK", "FORWARD", "BACK OFF", "STOP"} {
			order := game.Tactic(i)
			if i == 0 {
				if b := g.State.Battle(g.BattleID); b != nil && !b.Attacking {
					title = "BEGIN BATTLE"
				}
			}
			add(title, func() { g.result(g.State.CommandBattle(g.BattleID, max(-1, g.TacticGroup), order)) })
		}
		add("RETREAT", func() { g.result(g.State.Retreat(g.BattleID)) })
		pause()
		add("SAVE GAME", g.save)
		openHelp()
	case formations:
		add("PREVIOUS", func() { g.selectFormation(g.Formation - 1) })
		add("NEXT", func() { g.selectFormation(g.Formation + 1) })
		for _, kind := range []game.Kind{game.Archer, game.Knight, game.Barbarian} {
			add(kind.String(), func() {
				if g.EditFormations && g.Formation >= 6 && g.FormationMan >= 0 {
					g.formationSource()[g.Formation].Kind[g.FormationMan] = kind
				}
			})
		}
		if g.EditFormations {
			add("SAVE FORMATIONS", g.saveFormations)
		}
		back()
	case transfer:
		for kind := game.Archer; kind <= game.Spy; kind++ {
			add(fmt.Sprintf("%s -", kind), func() { g.changeTransfer(kind, -1) })
			add(fmt.Sprintf("%s +", kind), func() { g.changeTransfer(kind, 1) })
		}
		add("CONFIRM", g.finishTransfer)
		back()
	case editor:
		for i, title := range []string{"TERRAIN", "SQUADS", "FLAGS"} {
			tool := i
			add(title, func() { g.EditorTool = tool })
		}
		add("CHANGE SIDE", func() { g.EditorSide = g.EditorSide.Other() })
		add("PREVIOUS TILE", func() { g.EditorTile = (g.EditorTile+70)%72 + 1 })
		add("NEXT TILE", func() { g.EditorTile = g.EditorTile%72 + 1 })
		add("PAINT / ERASE", func() { g.TouchErase = !g.TouchErase })
		add("TITLE", func() { g.entry("title") })
		add("STORY", func() { g.entry("story") })
		add("RULES", func() { g.Return, g.Scene = editor, rules })
		add("SAVE CAMPAIGN", g.saveCampaign)
		add("PLAY", func() { g.SelectedLeader = g.Pending.Opponent; g.begin() })
	case textEntry:
		add("SHIFT", func() { g.KeyboardShift = !g.KeyboardShift })
		add("SPACE", func() { g.EntryBuffer += " " })
		add("BACKSPACE", func() {
			if len(g.EntryBuffer) > 0 {
				g.EntryBuffer = g.EntryBuffer[:len(g.EntryBuffer)-1]
			}
		})
		add("NEW LINE", func() {
			if g.EntryField == "story" {
				g.EntryBuffer += "\n"
			}
		})
		add("DONE", g.finishTouchEntry)
		add("CANCEL", func() { g.Scene = editor })
	case options:
		add("FIGHT DELAY", func() { g.FightDelay = next(g.FightDelay, []int{900, 1800, 3600}) })
		add("SOUND ON / OFF", func() { g.Sound = !g.Sound })
		back()
	case help:
		back()
	}
	columns := 1
	if available >= 520 {
		columns = 2
	}
	buttonWidth := (available - (columns-1)*12) / columns
	rows := (len(actions) + columns - 1) / columns
	buttonHeight := min(100, (height-96)/max(1, rows)-12)
	for i, a := range actions {
		g.largeButton(dst, left+(i%columns)*(buttonWidth+12), 76+(i/columns)*(buttonHeight+12), buttonWidth, buttonHeight, a.title, a.call)
	}
}
