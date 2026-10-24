package ui

import (
	"fmt"
	"image"
	"image/color"

	"ancientartofwar/internal/game"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

var (
	ink   = color.RGBA{14, 22, 27, 255}
	panel = color.RGBA{24, 36, 42, 255}
	line  = color.RGBA{59, 76, 76, 255}
	paper = color.RGBA{227, 219, 193, 255}
	muted = color.RGBA{151, 165, 159, 255}
	gold  = color.RGBA{225, 181, 101, 255}
	green = color.RGBA{113, 183, 149, 255}
	red   = color.RGBA{222, 114, 105, 255}
)

func rect(dst *ebiten.Image, x, y, w, h float32, c color.Color) {
	vector.DrawFilledRect(dst, x, y, w, h, c, false)
}
func stroke(dst *ebiten.Image, x, y, w, h float32, c color.Color) {
	vector.StrokeRect(dst, x, y, w, h, 1, c, false)
}
func label(dst *ebiten.Image, s string, x, y int, scale float64, c color.Color) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(x), float64(y)+11*scale)
	op.ColorScale.ScaleWithColor(c)
	text.DrawWithOptions(dst, s, basicfont.Face7x13, op)
}
func paragraph(dst *ebiten.Image, lines []string, x, y, spacing int, c color.Color) {
	for _, s := range lines {
		label(dst, s, x, y, 1.1, c)
		y += spacing
	}
}
func (g *Game) button(dst *ebiten.Image, x, y, w int, s string, fn func()) {
	r := image.Rect(x, y, x+w, y+34)
	c := panel
	mx, my := g.Pointer.X, g.Pointer.Y
	if image.Pt(mx, my).In(r) {
		c = line
	}
	rect(dst, float32(x), float32(y), float32(w), 34, c)
	stroke(dst, float32(x), float32(y), float32(w), 34, line)
	label(dst, s, x+12, y+10, 1.05, paper)
	g.Buttons = append(g.Buttons, button{r, s, fn})
}
func (g *Game) top(dst *ebiten.Image, s, sub string) {
	label(dst, s, 24, 20, 1.65, paper)
	label(dst, sub, 24, 48, 1, muted)
	rect(dst, 24, 65, 976, 1, line)
}
func (g *Game) footer(dst *ebiten.Image, s string) {
	rect(dst, 24, 714, 976, 1, line)
	label(dst, s, 24, 730, 1, muted)
}
func drawMap(dst, im *ebiten.Image, x, y int, scale float64) {
	if im == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(im, op)
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(ink)
	g.Buttons = g.Buttons[:0]
	if g.Screensaver {
		g.drawScreensaver(screen)
		g.capture(screen)
		return
	}
	switch g.Scene {
	case title:
		g.drawTitle(screen)
	case campaigns:
		g.drawCampaigns(screen)
	case briefing:
		g.drawBriefing(screen)
	case rules:
		g.drawRules(screen)
	case leaders:
		g.drawLeaders(screen)
	case war:
		g.drawWar(screen)
	case tactical:
		if g.Touch {
			g.drawTouchTactical(screen)
		} else {
			g.drawTactical(screen)
		}
	case formations:
		g.drawFormation(screen)
	case editor:
		g.drawEditor(screen)
	case transfer:
		g.drawTransfer(screen)
	case options:
		g.drawOptions(screen)
	case help:
		g.drawHelp(screen)
	case textEntry:
		if g.Touch {
			g.drawTouchEntry(screen)
		} else {
			g.drawEntry(screen)
		}
	}
	g.drawTouchExtras(screen)
	if g.Frame < g.MessageUntil && g.Message != "" {
		rect(screen, 18, 677, 988, 31, panel)
		label(screen, g.Message, 30, 686, 1.05, gold)
	}
	g.capture(screen)
}

func (g *Game) drawTitle(dst *ebiten.Image) {
	if g.Map == nil {
		g.choose(0)
	}
	if art := g.picture("PI"); art != nil {
		drawMap(dst, art, 32, 24, 3)
	}
	label(dst, "GO TO WAR", 270, 193, 2.3, ink)
	g.Buttons = append(g.Buttons, button{image.Rect(220, 180, 570, 230), "Go to war", func() { g.choose(g.SelectedCampaign); g.Scene = campaigns }})
	g.button(dst, 32, 650, 184, "GO TO WAR [Enter]", func() { g.choose(g.SelectedCampaign); g.Scene = campaigns })
	g.button(dst, 226, 650, 184, "LOAD GAME [L]", g.load)
	g.button(dst, 420, 650, 184, "MAKE CHANGES [E]", func() { g.choose(0); g.beginEditor(false) })
	g.button(dst, 614, 650, 184, "FORMATIONS", func() { g.openFormation(true) })
	g.button(dst, 808, 650, 184, "OPTIONS [O]", func() { g.Return = title; g.Scene = options })
	if g.State != nil {
		g.button(dst, 752, 696, 240, "RESUME CAMPAIGN [R]", func() { g.Scene = war; g.Paused = true; g.refreshMap(&g.State.World) })
	}
	label(dst, "F1  Field manual     F11  Full screen", 32, 724, 1, muted)
}

func (g *Game) drawCampaigns(dst *ebiten.Image) {
	g.top(dst, "CHOOSE A CAMPAIGN", "Eleven original campaigns and your own battlefields")
	drawMap(dst, g.Map, 674, 98, 1)
	stroke(dst, 673, 97, 322, 402, line)
	start := max(0, g.SelectedCampaign-15)
	for i := start; i < min(len(g.Campaigns), start+16); i++ {
		c := g.Campaigns[i]
		y := 88 + (i-start)*32
		if i == g.SelectedCampaign {
			rect(dst, 24, float32(y), 620, 30, panel)
			rect(dst, 24, float32(y), 3, 30, gold)
		}
		n := i
		label(dst, fmt.Sprintf("%02d   %s", i+1, c.Title), 40, y+8, 1.12, paper)
		g.Buttons = append(g.Buttons, button{image.Rect(24, y, 644, y+30), c.Title, func() { g.choose(n) }})
	}
	var totals [2]int
	for _, q := range g.Pending.Squads {
		totals[q.Side] += q.Count()
	}
	paragraph(dst, []string{fmt.Sprintf("Allies %d / Enemy %d", totals[0], totals[1]), g.Library.Leaders[g.Pending.Opponent].Name}, 674, 523, 25, muted)
	g.button(dst, 674, 590, 320, "PREPARE FOR WAR  [Enter]", func() { g.Scene = briefing })
	g.button(dst, 24, 635, 190, "EDIT A COPY  [E]", func() { g.beginEditor(false) })
	g.button(dst, 228, 635, 190, "NEW CAMPAIGN  [N]", func() { g.beginEditor(true) })
	g.button(dst, 674, 635, 320, "BACK  [Esc]", func() { g.Scene = title })
	g.footer(dst, "Up / Down  Choose campaign     Enter  Continue     F1  Help")
}

func (g *Game) drawBriefing(dst *ebiten.Image) {
	g.top(dst, g.Pending.Title, "THE STORY")
	lines := wrap(g.Pending.Story, 65)
	start := min(g.StoryScroll, max(0, len(lines)-25))
	end := min(len(lines), start+25)
	paragraph(dst, lines[start:end], 40, 100, 20, paper)
	drawMap(dst, g.Map, 720, 100, .8)
	g.button(dst, 40, 635, 260, "CAMPAIGN RULES  [Enter]", func() { g.Scene = rules })
	g.button(dst, 316, 635, 180, "BACK  [Esc]", func() { g.Scene = campaigns })
	g.footer(dst, "Up / Down  Scroll story     Enter  Continue")
}

func ruleLabels(r game.Rules) []string {
	food := func(v bool) string {
		if v {
			return "supply food"
		}
		return "do not supply food"
	}
	lookup := func(v int, values []int, names []string) string {
		for i, n := range values {
			if v == n {
				return names[i]
			}
		}
		return "?"
	}
	return []string{"Villages " + food(r.VillageFood), "Forts " + food(r.FortFood), "Supply lines: " + lookup(r.SupplyRange, []int{1, 2, 4}, []string{"short", "medium", "long"}), "Forts train men: " + lookup(r.Recruitment, []int{30, 60, 120}, []string{"rarely", "often", "very often"}), "Your men are " + lookup(r.Fitness, []int{4, 2, 1}, []string{"out of shape", "in good shape", "in prime shape"}), "Enemy is seen: " + lookup(r.Visibility, []int{20, 40, 80}, []string{"when close", "when far", "always"}), "Water: " + lookup(r.WaterSpeed, []int{10, 20, 40}, []string{"shallow and calm", "deep but calm", "deep and dangerous"}), "Mountains: " + lookup(r.MountainSpeed, []int{4, 8, 16}, []string{"low and safe", "high but safe", "high and risky"}), "Forest: " + lookup(r.ForestSpeed, []int{3, 6}, []string{"sparse", "dense"})}
}
func (g *Game) drawRules(dst *ebiten.Image) {
	g.top(dst, "CAMPAIGN RULES", g.Pending.Title)
	for i, s := range ruleLabels(g.Pending.Rules) {
		y := 92 + i*52
		if g.RuleIndex == i {
			rect(dst, 24, float32(y), 700, 46, panel)
			rect(dst, 24, float32(y), 3, 46, gold)
		}
		label(dst, s, 44, y+16, 1.25, paper)
		n := i
		g.button(dst, 756, y+6, 220, "CHANGE", func() { g.RuleIndex = n; g.cycleRule(n) })
	}
	g.button(dst, 40, 625, 270, "CONTINUE  [Enter]", func() {
		if g.Return == editor {
			g.Scene = editor
			g.Return = title
		} else {
			g.Scene = leaders
		}
	})
	g.footer(dst, "Up / Down  Select rule     Space / Right  Change     Enter  Continue")
}
func (g *Game) drawLeaders(dst *ebiten.Image) {
	g.top(dst, "KNOW YOUR ENEMY", g.Pending.Title)
	portraits := g.picture("PJ")
	for i, l := range g.Library.Leaders {
		x, y := 32+(i%2)*492, 94+(i/2)*116
		n := i
		if i == g.SelectedLeader {
			rect(dst, float32(x), float32(y), 472, 100, panel)
			rect(dst, float32(x), float32(y), 3, 100, gold)
		}
		if portraits != nil {
			r := image.Rect(i%4*80, 20+i/4*90, i%4*80+80, 100+i/4*90)
			drawMap(dst, portraits.SubImage(r).(*ebiten.Image), x+14, y+10, 1)
		}
		label(dst, l.Name, x+110, y+10, 1.25, paper)
		paragraph(dst, wrap(l.Description, 44), x+110, y+38, 16, muted)
		g.Buttons = append(g.Buttons, button{image.Rect(x, y, x+472, y+100), l.Name, func() { g.SelectedLeader = n }})
	}
	g.button(dst, 32, 624, 300, "GO TO WAR  [Enter]", func() {
		if g.Return == editor {
			g.Pending.Opponent = g.SelectedLeader
			g.Scene = editor
			g.Return = title
		} else {
			g.begin()
		}
	})
	g.footer(dst, "Arrow keys  Choose opponent     Enter  Continue     Esc  Back")
}

func (g *Game) drawWar(dst *ebiten.Image) {
	s := g.State
	if s == nil {
		return
	}
	status := fmt.Sprintf("%s  /  %02d:%02d", s.Leader.Name, s.Tick/3600, s.Tick/60%60)
	if g.Paused {
		status += "  /  PAUSED"
	}
	if g.Demo {
		status += "  /  DEMO"
	}
	g.top(dst, s.Title, status)
	drawMap(dst, g.Map, mapX, mapY, mapScale)
	if g.Overview {
		g.drawInfluence(dst)
	}
	if g.Selected >= 0 && g.Selected < 20 {
		q := &s.Squads[g.Selected]
		last := q.Position
		for _, p := range q.Route {
			if g.Frame%2 == 0 || p.X%2 == 0 {
				x1, y1 := onMap(last)
				x2, y2 := onMap(p)
				vector.StrokeLine(dst, x1, y1, x2, y2, 1, gold, false)
			}
			last = p
		}
	}
	for _, f := range s.Flags {
		x, y := onMap(f.Position)
		c := paper
		if f.Side == game.Enemy {
			c = red
		}
		vector.StrokeLine(dst, x, y-14, x, y+2, 2, ink, false)
		rect(dst, x+1, y-14, 10, 7, c)
	}
	for _, q := range s.Squads {
		if !q.Alive() || !q.Visible && s.Outcome == game.Running {
			continue
		}
		x, y := onMap(q.Position)
		c := paper
		if q.Side == game.Enemy {
			c = red
		}
		if q.BattleID != 0 && g.Frame%30 < 12 {
			c = gold
		}
		rect(dst, x-3, y-14, 7, 10, ink)
		vector.DrawFilledCircle(dst, x, y-13, 2.5, c, false)
		rect(dst, x-2, y-10, 5, 6, c)
		vector.StrokeLine(dst, x, y-5, x-3, y, 2, c, false)
		vector.StrokeLine(dst, x, y-5, x+3, y, 2, c, false)
		label(dst, fmt.Sprintf("%02d", q.ID+1), int(x)+5, int(y)-10, .85, ink)
		if q.ID == g.Selected {
			stroke(dst, x-7, y-19, 28, 22, gold)
		}
	}
	if g.DrawingRoute {
		last := s.Squads[g.Selected].Position
		for _, p := range g.Waypoints {
			x1, y1 := onMap(last)
			x2, y2 := onMap(p)
			vector.StrokeLine(dst, x1, y1, x2, y2, 2, gold, false)
			last = p
		}
		label(dst, "TRACE YOUR ROUTE / M TO CONFIRM", 32, 690, 1, gold)
	}
	x, y := onMap(g.Cursor)
	vector.StrokeLine(dst, x-4, y, x+4, y, 1, paper, false)
	vector.StrokeLine(dst, x, y-4, x, y+4, 1, paper, false)
	men := s.Totals()
	label(dst, "THE WAR", 544, 84, 1.3, gold)
	label(dst, fmt.Sprintf("Allies %d    Enemy %d", men[0], men[1]), 544, 112, 1.15, paper)
	if g.Selected >= 0 {
		q := &s.Squads[g.Selected]
		label(dst, fmt.Sprintf("SQUAD %02d / %s", q.ID+1, q.Side), 544, 154, 1.3, paper)
		label(dst, fmt.Sprintf("A:%d   K:%d   B:%d   Sp:%d", q.Men[0], q.Men[1], q.Men[2], q.Men[3]), 544, 184, 1.1, muted)
		bar(dst, "Condition", q.Condition, 544, 218, green)
		bar(dst, "Food", q.Food, 544, 246, gold)
		pace := "Resting"
		if q.Moving() {
			pace = []string{"", "Fast march", "March", "Slow march"}[q.Pace]
		}
		label(dst, fmt.Sprintf("%s / %s / Formation %d", pace, s.World.TerrainAt(q.Position), q.Formation), 544, 281, 1, muted)
		moveLabel := "MOVE [M]"
		if g.DrawingRoute {
			moveLabel = "GO [M]"
		}
		g.button(dst, 544, 310, 140, moveLabel, g.toggleRoute)
		g.button(dst, 696, 310, 140, "FORMATION [F]", func() { g.openFormation(false) })
		g.button(dst, 848, 310, 150, "ZOOM  [Z]", g.zoom)
		g.button(dst, 544, 354, 140, "DETACH  [D]", func() { g.openTransfer(false) })
		g.button(dst, 696, 354, 140, "JOIN  [J]", func() { g.openTransfer(true) })
		g.button(dst, 848, 354, 150, "STOP / GO", func() {
			if q.Stopped {
				g.result(s.SetPace(q.ID, q.Pace))
			} else {
				g.result(s.SetPace(q.ID, 0))
			}
		})
	}
	dispatchY := 420
	if g.Touch {
		dispatchY = 488
	}
	label(dst, "DISPATCHES", 544, dispatchY, 1.2, gold)
	yy := dispatchY + 28
	for i := len(s.Events) - 1; i >= 0 && yy < 600; i-- {
		ev := s.Events[i]
		lines := wrap(ev.Text, 54)
		paragraph(dst, lines, 544, yy, 17, muted)
		yy += len(lines)*17 + 8
	}
	if s.Outcome != game.Running {
		rect(dst, 536, 603, 466, 93, panel)
		headline := "VICTORY"
		c := green
		if s.Outcome == game.EnemyWon {
			headline = "DEFEAT"
			c = red
		}
		label(dst, headline, 550, 616, 1.6, c)
		label(dst, s.Reason, 550, 647, 1.1, paper)
		g.button(dst, 810, 654, 180, "CAMPAIGNS", func() { g.choose(g.SelectedCampaign); g.Scene = campaigns })
	} else {
		g.button(dst, 544, 610, 140, "SAVE  [F5]", g.save)
		g.button(dst, 696, 610, 140, []string{"0.5x", "1x", "2x", "4x"}[g.Speed]+"  [T]", func() { g.Speed = (g.Speed + 1) % 4 })
		g.button(dst, 848, 610, 150, "PAUSE [Space]", func() { g.Paused = !g.Paused })
		if s.SurrenderOffered {
			g.button(dst, 544, 656, 140, "ACCEPT [Y]", func() { s.AcceptSurrender() })
			g.button(dst, 696, 656, 140, "DECLINE [N]", func() { s.DeclineSurrender() })
		} else {
			g.button(dst, 544, 656, 140, "SURRENDER", func() {
				if g.Message == "Click SURRENDER again to confirm" && g.Frame < g.MessageUntil {
					s.Surrender()
				} else {
					g.toast("Click SURRENDER again to confirm")
				}
			})
		}
		g.button(dst, 848, 656, 150, "MENU", func() { g.choose(g.SelectedCampaign); g.Scene = campaigns })
	}
	g.footer(dst, "Click  Select     Right click  Move     Tab  Next squad     1/2/3  Pace     V  Influence     F1  Help")
}

func onMap(p game.Point) (float32, float32) {
	return float32(mapX) + float32(p.X*4+2)*mapScale, float32(mapY) + float32(p.Y*2+10)*mapScale
}
func bar(dst *ebiten.Image, s string, v, x, y int, c color.Color) {
	label(dst, s, x, y, 1, muted)
	rect(dst, float32(x+105), float32(y+2), 232, 9, line)
	rect(dst, float32(x+105), float32(y+2), float32(v)/49*232, 9, c)
	label(dst, fmt.Sprintf("%3d%%", v*100/49), x+350, y, 1, paper)
}
func (g *Game) drawInfluence(dst *ebiten.Image) {
	for i, v := range g.State.Influence {
		c := color.RGBA{20, 30, 40, 165}
		if v < -1 {
			c = color.RGBA{150, 215, 176, 175}
		} else if v > 1 && g.State.Rules.Visibility == 80 {
			c = color.RGBA{220, 105, 90, 175}
		}
		rect(dst, float32(mapX+i%10*48), float32(mapY+i/10*30), 48, 30, c)
	}
}

func (g *Game) drawTactical(dst *ebiten.Image) {
	b := g.State.Battle(g.BattleID)
	if b == nil {
		return
	}
	g.top(dst, "IN THE EYE OF THE STORM", fmt.Sprintf("Squad %02d vs %02d  /  %s", b.Squads[0]+1, b.Squads[1]+1, g.State.Leader.Name))
	g.drawBattleField(dst, b.ID, 32, 120, 960, 540)
	group := "A  Archers   B  Barbarians   K  Knights   S  Squad"
	if g.TacticGroup >= -1 {
		group = "A  Attack   F  Forward   B  Back off   S  Stop   Backspace  Cancel"
	}
	if !b.Attacking {
		group = "A  Attack   /   Give your first order before the enemy attacks"
	}
	label(dst, group, 32, 86, 1.05, paper)
	g.button(dst, 32, 670, 200, "ALL ATTACK", func() { g.result(g.State.CommandBattle(b.ID, -1, game.Attack)) })
	g.button(dst, 246, 670, 200, "ARCHERS HOLD", func() { g.result(g.State.CommandBattle(b.ID, int(game.Archer), game.Hold)) })
	g.button(dst, 460, 670, 200, "BACK OFF", func() { g.result(g.State.CommandBattle(b.ID, -1, game.BackOff)) })
	g.button(dst, 792, 670, 200, "RETREAT  [R]", func() { g.result(g.State.Retreat(b.ID)) })
	g.footer(dst, "Select a troop type, then an order. Archers need distance. Space pauses the battle.")
}

func (g *Game) drawOptions(dst *ebiten.Image) {
	g.top(dst, "YE OLDE OPTIONS SHOPPE", "Choose your pace before the campaign")
	g.button(dst, 44, 130, 530, fmt.Sprintf("FIGHT DELAY: %d SECONDS  [Space]", g.FightDelay/60), func() { g.FightDelay = next(g.FightDelay, []int{900, 1800, 3600}) })
	g.button(dst, 44, 188, 530, fmt.Sprintf("SOUND: %t  [M]", g.Sound), func() { g.Sound = !g.Sound })
	g.button(dst, 44, 298, 280, "RETURN  [Enter]", func() { g.Scene = g.Return })
	g.footer(dst, "F11  Full screen     Enter / Esc  Return")
}
func (g *Game) drawHelp(dst *ebiten.Image) {
	g.top(dst, "FIELD MANUAL", "Capture every enemy flag, or defeat the entire enemy army")
	paragraph(dst, []string{"ON THE MAP", "Click a squad to select it. Right click a destination to trace a direct route.", "M begins a route; click or drag waypoints, then M or Enter to confirm.", "The army follows your route: use bridges to avoid water losses.", "Tab selects the next allied squad. Arrow keys move the marker; Enter selects.", "1 / 2 / 3 choose slow / normal / fast march; 0 stops without clearing the route.", "F chooses a formation. D detaches men; J transfers men into a nearby squad.", "Z opens an encounter before fighting starts. Late zooms cannot take command.", "Space pauses. T changes time. V shows influence and pauses. C clears dispatches.", "F5 saves; F9 loads. The loaded campaign starts paused.", "", "IN BATTLE", "A starts the attack. Select A / B / K / S for archers / barbarians / knights / all.", "Then choose A / B / F / S for attack / back off / forward / stop. R retreats.", "Knights beat barbarians, barbarians beat archers, archers beat knights.", "Archers cannot fight at close quarters. Fort walls protect defending archers.", "", "SUPPLIES AND CONDITION", "Friendly control of villages and forts opens supply lines when rules allow it.", "Rest your squads to recover. Food below 20% harms condition.", "Forts replenish squads up to 14 men. At most 20 squads may exist in a campaign.", "Spies see twice as far and do not fight. With visibility set to always, they", "are replaced by barbarians.", "", "EDITOR", "Paint with 72 original terrain pieces. Add squads and flags for either side.", "Edit the title, story, rules and opponent, then save your campaign or play it."}, 40, 92, 21, paper)
	g.button(dst, 40, 652, 260, "RETURN  [Enter]", func() { g.Scene = g.HelpReturn })
	g.footer(dst, "F1 / Enter / Esc  Return to the game")
}
