package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"ancientartofwar/internal/game"
	"ancientartofwar/internal/original"
	"github.com/hajimehoshi/ebiten/v2"
)

func (g *Game) openFormation(edit bool) {
	g.EditFormations = edit
	g.FormationMan = -1
	g.Formation = 1
	if edit {
		g.Return = g.Scene
	} else {
		if g.State == nil || g.Selected < 0 {
			g.toast("Select an allied squad")
			return
		}
		q := &g.State.Squads[g.Selected]
		if q.Side != game.Player || !q.Alive() || q.BattleID != 0 {
			g.toast("Select an allied squad outside a battle")
			return
		}
		g.Formation = q.Formation
		g.Return = war
	}
	g.Scene = formations
}
func (g *Game) formationSource() *[10]game.Formation {
	if !g.EditFormations && g.State != nil {
		return &g.State.Formations
	}
	return &g.Library.Formations
}
func (g *Game) selectFormation(n int) {
	g.Formation = (n+8)%9 + 1
	if !g.EditFormations {
		g.result(g.State.SetFormation(g.Selected, g.Formation))
	}
}
func (g *Game) updateFormation() {
	if just(ebiten.KeyArrowRight) && (!g.EditFormations || g.FormationMan < 0) {
		g.selectFormation(g.Formation + 1)
	}
	if just(ebiten.KeyArrowLeft) && (!g.EditFormations || g.FormationMan < 0) {
		g.selectFormation(g.Formation - 1)
	}
	if just(ebiten.KeyEscape) || just(ebiten.KeyEnter) {
		g.Scene = g.Return
	}
	if !g.EditFormations || g.Formation < 6 {
		return
	}
	f := &g.formationSource()[g.Formation]
	if g.Pointer.Pressed {
		x, y := g.Pointer.X, g.Pointer.Y
		best, d := -1, 10000
		for i := 0; i < 14; i++ {
			px, py := formationXY(f.X[i], f.Y[i])
			dd := (x-px)*(x-px) + (y-py)*(y-py)
			if dd < d && dd < 400 {
				best = i
				d = dd
			}
		}
		if best >= 0 {
			g.FormationMan = best
		} else if g.FormationMan >= 0 && x > 40 && x < 650 && y > 150 && y < 610 {
			f.X[g.FormationMan] = min(74, max(44, 40+(x-50)/14))
			f.Y[g.FormationMan] = min(76, max(20, 10+(y-150)/6))
		}
	}
	if g.FormationMan >= 0 {
		i := g.FormationMan
		if repeat(ebiten.KeyArrowLeft) {
			f.X[i] = max(44, f.X[i]-1)
		}
		if repeat(ebiten.KeyArrowRight) {
			f.X[i] = min(74, f.X[i]+1)
		}
		if repeat(ebiten.KeyArrowUp) {
			f.Y[i] = max(20, f.Y[i]-2)
		}
		if repeat(ebiten.KeyArrowDown) {
			f.Y[i] = min(76, f.Y[i]+2)
		}
		if just(ebiten.KeyA) {
			f.Kind[i] = game.Archer
		}
		if just(ebiten.KeyK) {
			f.Kind[i] = game.Knight
		}
		if just(ebiten.KeyB) {
			f.Kind[i] = game.Barbarian
		}
	}
	if just(ebiten.KeyS) {
		g.saveFormations()
	}
}
func formationXY(x, y int) (int, int) { return 50 + (x-40)*14, 150 + (y-10)*6 }
func (g *Game) saveFormations() {
	err := writeAtomic(filepath.Join(g.DataRoot, "formations.json"), func(f *os.File) error { return json.NewEncoder(f).Encode(g.Library.Formations) })
	if err != nil {
		g.result(err)
	} else {
		g.toast("Formations saved")
	}
}
func (g *Game) drawFormation(dst *ebiten.Image) {
	g.top(dst, "SQUAD FORMATIONS", "Five fixed formations and four that you can teach your squads")
	for i := 1; i <= 9; i++ {
		n := i
		g.button(dst, 32+(i-1)*108, 88, 98, fmt.Sprintf("%d", i), func() { g.selectFormation(n) })
	}
	rect(dst, 40, 144, 610, 472, panel)
	f := g.formationSource()[g.Formation]
	for i := 0; i < 14; i++ {
		x, y := formationXY(f.X[i], f.Y[i])
		ch := "A"
		if f.Kind[i] == game.Knight {
			ch = "K"
		}
		if f.Kind[i] == game.Barbarian {
			ch = "B"
		}
		label(dst, ch, x-4, y-6, 1.2, paper)
		if i == g.FormationMan {
			stroke(dst, float32(x-14), float32(y-14), 28, 28, gold)
		}
	}
	label(dst, fmt.Sprintf("FORMATION %d", g.Formation), 690, 160, 1.5, gold)
	paragraph(dst, []string{"A  Archer", "K  Knight", "B  Barbarian", "", "Protect archers behind your", "front line. Choose a formation", "before an encounter starts."}, 690, 208, 24, paper)
	if g.EditFormations {
		paragraph(dst, []string{"Select formation 6, 7, 8 or 9.", "Click a man, then a new spot.", "Arrows move the selected man.", "A / K / B changes his type."}, 690, 425, 24, muted)
		g.button(dst, 690, 556, 290, "SAVE FORMATIONS  [S]", g.saveFormations)
	}
	g.button(dst, 40, 638, 250, "DONE  [Enter]", func() { g.Scene = g.Return })
	g.footer(dst, "1-5  Permanent formations     6-9  Custom formations     Enter / Esc  Return")
}

func (g *Game) openTransfer(join bool) {
	if g.Selected < 0 || g.State == nil {
		return
	}
	q := g.State.Squads[g.Selected]
	if !q.Alive() || q.Side != game.Player || q.BattleID != 0 {
		g.toast("Select an allied squad outside a battle")
		return
	}
	g.TransferFrom = g.Selected
	g.TransferTo = -1
	g.TransferMen = [4]int{}
	if join {
		g.TransferTo = g.Selected
		g.TransferFrom = -1
		best := 9
		for _, other := range g.State.Squads {
			if other.ID == q.ID || other.Side != q.Side || !other.Alive() || other.BattleID != 0 {
				continue
			}
			d := game.Distance(q.Position, other.Position)
			if d < best {
				best = d
				g.TransferFrom = other.ID
			}
		}
		if g.TransferFrom < 0 {
			g.toast("No adjacent allied squad")
			return
		}
	}
	g.Scene = transfer
}
func (g *Game) changeTransfer(k game.Kind, delta int) {
	q := g.State.Squads[g.TransferFrom]
	g.TransferMen[k] = min(q.Men[k], max(0, g.TransferMen[k]+delta))
}
func (g *Game) finishTransfer() {
	var err error
	if g.TransferTo < 0 {
		var id int
		id, err = g.State.Detach(g.TransferFrom, g.TransferMen)
		if err == nil {
			g.Selected = id
		}
	} else {
		err = g.State.Transfer(g.TransferFrom, g.TransferTo, g.TransferMen)
	}
	if err != nil {
		g.result(err)
		return
	}
	g.Scene = war
}
func (g *Game) updateTransfer() {
	if just(ebiten.KeyEscape) {
		g.Scene = war
	}
	if just(ebiten.KeyEnter) {
		g.finishTransfer()
	}
	for i, k := range []ebiten.Key{ebiten.KeyA, ebiten.KeyK, ebiten.KeyB, ebiten.KeyS} {
		if just(k) {
			d := 1
			if shift() {
				d = -1
			}
			g.changeTransfer(game.Kind(i), d)
		}
	}
}
func (g *Game) drawTransfer(dst *ebiten.Image) {
	t := "DETACH A NEW SQUAD"
	dest := "New squad"
	if g.TransferTo >= 0 {
		t = "JOIN / TRANSFER MEN"
		dest = fmt.Sprintf("Squad %02d", g.TransferTo+1)
	}
	g.top(dst, t, fmt.Sprintf("Squad %02d -> %s", g.TransferFrom+1, dest))
	q := g.State.Squads[g.TransferFrom]
	for k := game.Archer; k <= game.Spy; k++ {
		y := 130 + int(k)*90
		n := k
		label(dst, fmt.Sprintf("%s     Available: %d     Transfer: %d", k, q.Men[k], g.TransferMen[k]), 44, y, 1.3, paper)
		g.button(dst, 700, y-4, 100, "-", func() { g.changeTransfer(n, -1) })
		g.button(dst, 814, y-4, 100, "+", func() { g.changeTransfer(n, 1) })
	}
	g.button(dst, 44, 550, 260, "CONFIRM  [Enter]", g.finishTransfer)
	g.button(dst, 320, 550, 200, "CANCEL  [Esc]", func() { g.Scene = war })
	g.footer(dst, "A / K / B / S  Add an archer / knight / barbarian / spy     Shift + key  Remove")
}

func (g *Game) beginEditor(blank bool) {
	if g.Map == nil {
		g.choose(0)
	}
	g.Pending.ID = "custom"
	g.EditorTool = 0
	g.EditorTile = 1
	g.EditorSquad = -1
	if blank {
		g.Pending.Title = "A New Campaign"
		g.Pending.Story = "Write the story of your campaign."
		g.Pending.Flags = nil
		for i := range g.Pending.World.Tiles {
			g.Pending.World.Tiles[i] = 1
		}
		for i := range g.Pending.Squads {
			g.Pending.Squads[i] = game.Squad{ID: i, Pace: 2, Condition: 49, Food: 49, Formation: 1, AITarget: -1}
		}
		g.result(g.Library.BuildWorld(&g.Pending.World))
	}
	g.refreshMap(&g.Pending.World)
	g.Scene = editor
}
func (g *Game) updateEditor() {
	if just(ebiten.KeyEscape) {
		g.choose(g.SelectedCampaign)
		g.Scene = campaigns
		return
	}
	if just(ebiten.Key1) {
		g.EditorTool = 0
	}
	if just(ebiten.Key2) {
		g.EditorTool = 1
	}
	if just(ebiten.Key3) {
		g.EditorTool = 2
	}
	if just(ebiten.KeyTab) {
		g.EditorSide = g.EditorSide.Other()
	}
	if repeat(ebiten.KeyEqual) || repeat(ebiten.KeyPageDown) {
		g.EditorTile = g.EditorTile%72 + 1
	}
	if repeat(ebiten.KeyMinus) || repeat(ebiten.KeyPageUp) {
		g.EditorTile = (g.EditorTile+70)%72 + 1
	}
	if just(ebiten.KeyF5) {
		g.saveCampaign()
	}
	if just(ebiten.KeyEnter) {
		g.SelectedLeader = g.Pending.Opponent
		g.begin()
		return
	}
	if g.EditorTool == 1 && g.EditorSquad >= 0 {
		for i, k := range []ebiten.Key{ebiten.KeyA, ebiten.KeyK, ebiten.KeyB, ebiten.KeyS} {
			if just(k) {
				d := 1
				if shift() {
					d = -1
				}
				g.editMen(game.Kind(i), d)
			}
		}
	}
	if p, ok := g.pointAtMouse(); ok {
		left := g.Pointer.Pressed && !g.TouchErase
		right := g.Pointer.Right || g.TouchErase && g.Pointer.Pressed
		if g.EditorTool == 0 && (g.Pointer.Down || right) {
			cell := min(199, (p.Y/10)*10+p.X/8)
			t := g.EditorTile
			if right || g.TouchErase {
				t = 1
			}
			if g.Pending.World.Tiles[cell] != t {
				g.Pending.World.Tiles[cell] = t
				g.result(g.Library.BuildWorld(&g.Pending.World))
				g.refreshMap(&g.Pending.World)
			}
		}
		if g.EditorTool == 1 && (left || right) {
			id := -1
			for i, q := range g.Pending.Squads {
				if q.Alive() && game.Distance(q.Position, p) < 8 {
					id = i
					break
				}
			}
			if right {
				if id >= 0 {
					g.Pending.Squads[id].Men = [4]int{}
				}
				g.EditorSquad = -1
			} else if id >= 0 {
				g.EditorSquad = id
			} else {
				for i, q := range g.Pending.Squads {
					if !q.Alive() {
						g.Pending.Squads[i] = game.Squad{ID: i, Side: g.EditorSide, Position: p, Men: [4]int{4, 6, 4, 0}, Pace: 2, Condition: 49, Food: 49, Formation: 1, AITarget: -1}
						g.EditorSquad = i
						break
					}
				}
			}
		}
		if g.EditorTool == 2 && (left || right) {
			id := -1
			for i, f := range g.Pending.Flags {
				if game.Distance(f.Position, p) < 6 {
					id = i
					break
				}
			}
			if right {
				if id >= 0 {
					g.Pending.Flags = append(g.Pending.Flags[:id], g.Pending.Flags[id+1:]...)
				}
			} else if id >= 0 {
				g.Pending.Flags[id].Side = g.Pending.Flags[id].Side.Other()
			} else if len(g.Pending.Flags) < 10 {
				g.Pending.Flags = append(g.Pending.Flags, game.Flag{Position: p, Side: g.EditorSide})
			}
		}
	}
}
func (g *Game) editMen(k game.Kind, delta int) {
	if g.EditorSquad < 0 {
		return
	}
	q := &g.Pending.Squads[g.EditorSquad]
	if delta > 0 && q.Count() >= 14 {
		return
	}
	q.Men[k] = max(0, q.Men[k]+delta)
}
func (g *Game) saveCampaign() {
	var totals [2]int
	for _, q := range g.Pending.Squads {
		totals[q.Side] += q.Count()
	}
	if totals[0] == 0 || totals[1] == 0 {
		g.toast("Place at least one squad for each side")
		return
	}
	name := strings.Map(func(r rune) rune {
		if r > 127 {
			return -1
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return '_'
	}, g.Pending.Title)
	name = strings.Trim(name, "_")
	if name == "" {
		name = "campaign"
	}
	path := filepath.Join(g.DataRoot, "campaigns", name+".json")
	err := writeAtomic(path, func(f *os.File) error { return original.SaveCampaign(f, g.Pending) })
	if err != nil {
		g.result(err)
		return
	}
	g.Pending.ID = path
	found := false
	for i, c := range g.Campaigns {
		if c.ID == path {
			g.Campaigns[i] = g.Pending
			g.SelectedCampaign = i
			found = true
		}
	}
	if !found {
		g.Campaigns = append(g.Campaigns, g.Pending)
		g.SelectedCampaign = len(g.Campaigns) - 1
	}
	if g.Touch {
		g.toast("Campaign saved")
	} else {
		g.toast("Campaign saved: " + path)
	}
}
func (g *Game) deleteCampaign() {
	if g.SelectedCampaign < 11 {
		g.toast("The original campaigns are preserved")
		return
	}
	c := g.Campaigns[g.SelectedCampaign]
	if g.Message != "Click DELETE again to confirm" || g.Frame >= g.MessageUntil {
		g.toast("Click DELETE again to confirm")
		return
	}
	if err := os.Remove(c.ID); err != nil {
		g.result(err)
		return
	}
	g.Campaigns = append(g.Campaigns[:g.SelectedCampaign], g.Campaigns[g.SelectedCampaign+1:]...)
	g.choose(max(0, g.SelectedCampaign-1))
	g.Scene = campaigns
}
func (g *Game) entry(field string) {
	g.EntryField = field
	g.EntryBuffer = g.Pending.Title
	if field == "story" {
		g.EntryBuffer = g.Pending.Story
	}
	g.Scene = textEntry
}
func (g *Game) updateEntry() {
	for _, c := range ebiten.AppendInputChars(nil) {
		if c >= 32 && c <= 126 {
			g.EntryBuffer += string(c)
		}
	}
	if repeat(ebiten.KeyBackspace) && len(g.EntryBuffer) > 0 {
		g.EntryBuffer = g.EntryBuffer[:len(g.EntryBuffer)-1]
	}
	limit := 80
	if g.EntryField == "story" {
		limit = 800
	}
	if len(g.EntryBuffer) > limit {
		g.EntryBuffer = g.EntryBuffer[:limit]
	}
	if just(ebiten.KeyEnter) {
		if shift() && g.EntryField == "story" {
			g.EntryBuffer += "\n"
		} else {
			if g.EntryField == "story" {
				g.Pending.Story = g.EntryBuffer
			} else if strings.TrimSpace(g.EntryBuffer) != "" {
				g.Pending.Title = g.EntryBuffer
			}
			g.Scene = editor
		}
	}
	if just(ebiten.KeyEscape) {
		g.Scene = editor
	}
}

func (g *Game) drawEditor(dst *ebiten.Image) {
	g.top(dst, "GAME GENERATOR", g.Pending.Title)
	drawMap(dst, g.Map, mapX, mapY, mapScale)
	for _, f := range g.Pending.Flags {
		x, y := onMap(f.Position)
		c := paper
		if f.Side == game.Enemy {
			c = red
		}
		rect(dst, x, y-12, 2, 15, c)
		rect(dst, x, y-12, 10, 6, c)
	}
	for _, q := range g.Pending.Squads {
		if !q.Alive() {
			continue
		}
		x, y := onMap(q.Position)
		c := paper
		if q.Side == game.Enemy {
			c = red
		}
		rect(dst, x-3, y-10, 8, 12, c)
		label(dst, fmt.Sprintf("%02d", q.ID+1), int(x)+7, int(y)-10, 1, ink)
		if q.ID == g.EditorSquad {
			stroke(dst, x-6, y-14, 28, 20, gold)
		}
	}
	for i, s := range []string{"TERRAIN [1]", "SQUADS [2]", "FLAGS [3]"} {
		n := i
		g.button(dst, 544+i*154, 84, 144, s, func() { g.EditorTool = n })
	}
	label(dst, fmt.Sprintf("Placing: %s  [Tab]", g.EditorSide), 544, 136, 1.2, gold)
	switch g.EditorTool {
	case 0:
		label(dst, fmt.Sprintf("MAP PIECE %02d / 72", g.EditorTile), 544, 184, 1.3, paper)
		// Show the tile with the original image's even/odd scanline layout.
		im := image.NewRGBA(image.Rect(0, 0, 32, 20))
		data := g.Library.Tiles[g.EditorTile-1]
		for y := 0; y < 20; y++ {
			for x := 0; x < 32; x++ {
				b := data[y*8+x/4]
				im.SetRGBA(x, y, original.CGA[b>>uint(6-(x%4)*2)&3])
			}
		}
		tile := ebiten.NewImageFromImage(im)
		drawMap(dst, tile, 544, 220, 5)
		tile.Dispose()
		g.button(dst, 544, 342, 144, "PREVIOUS [-]", func() { g.EditorTile = (g.EditorTile+70)%72 + 1 })
		g.button(dst, 704, 342, 144, "NEXT [+]", func() { g.EditorTile = g.EditorTile%72 + 1 })
		paragraph(dst, []string{"Paint by clicking or dragging.", "Right click restores flatland.", "Use Page Up / Down or +/-", "to browse the map pieces."}, 544, 402, 23, muted)
	case 1:
		if g.EditorSquad >= 0 {
			q := &g.Pending.Squads[g.EditorSquad]
			label(dst, fmt.Sprintf("SQUAD %02d  /  %d MEN", q.ID+1, q.Count()), 544, 184, 1.3, paper)
			for k := game.Archer; k <= game.Spy; k++ {
				n := k
				y := 226 + int(k)*44
				label(dst, fmt.Sprintf("%s: %d", k, q.Men[k]), 544, y+6, 1.1, paper)
				g.button(dst, 790, y, 90, "-", func() { g.editMen(n, -1) })
				g.button(dst, 896, y, 90, "+", func() { g.editMen(n, 1) })
			}
			g.button(dst, 544, 416, 210, "CHANGE SIDE", func() { q.Side = q.Side.Other() })
			g.button(dst, 770, 416, 216, "CLEAR MEN", func() { q.Men = [4]int{} })
			g.button(dst, 544, 460, 210, fmt.Sprintf("CONDITION %d%%", q.Condition*100/49), func() { q.Condition = next(q.Condition, []int{1, 10, 25, 49}) })
			g.button(dst, 770, 460, 216, fmt.Sprintf("FOOD %d%%", q.Food*100/49), func() { q.Food = next(q.Food, []int{1, 10, 25, 49}) })
			g.button(dst, 544, 504, 210, fmt.Sprintf("PACE %d", q.Pace), func() { q.Pace = q.Pace%3 + 1 })
			g.button(dst, 770, 504, 216, fmt.Sprintf("FORMATION %d", q.Formation), func() { q.Formation = q.Formation%9 + 1 })
		} else {
			paragraph(dst, []string{"Click to place a squad.", "Click an existing squad to edit.", "Right click to remove it.", "Maximum: 20 squads, 14 men each."}, 544, 190, 25, paper)
		}
	case 2:
		paragraph(dst, []string{"Click to place a flag.", "Click it again to change side.", "Right click to remove it.", "Maximum: 10 flags."}, 544, 190, 25, paper)
	}
	g.button(dst, 544, 555, 140, "TITLE", func() { g.entry("title") })
	g.button(dst, 698, 555, 140, "STORY", func() { g.entry("story") })
	g.button(dst, 852, 555, 140, "RULES", func() { g.Return = editor; g.Scene = rules })
	g.button(dst, 544, 599, 220, "OPPONENT", func() { g.SelectedLeader = g.Pending.Opponent; g.Return = editor; g.Scene = leaders })
	g.button(dst, 778, 599, 214, "SAVE [F5]", g.saveCampaign)
	g.button(dst, 544, 643, 220, "PLAY [Enter]", func() { g.SelectedLeader = g.Pending.Opponent; g.begin() })
	g.button(dst, 778, 643, 214, "DELETE CUSTOM", g.deleteCampaign)
	g.footer(dst, "Click  Place / edit     Right click  Remove     1/2/3  Tool     Tab  Side     Esc  Catalogue")
}
func (g *Game) drawEntry(dst *ebiten.Image) {
	g.top(dst, "WRITE THE "+strings.ToUpper(g.EntryField), "Enter saves your changes; Shift+Enter adds a story line")
	paragraph(dst, wrap(g.EntryBuffer+"_", 110), 40, 110, 24, paper)
	g.button(dst, 40, 638, 240, "DONE  [Enter]", func() {
		if g.EntryField == "story" {
			g.Pending.Story = g.EntryBuffer
		} else {
			g.Pending.Title = g.EntryBuffer
		}
		g.Scene = editor
	})
	g.footer(dst, "Use the keyboard to edit. Backspace removes a character. Esc cancels.")
}
