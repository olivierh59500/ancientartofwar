package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"ancientartofwar/internal/game"
	"ancientartofwar/internal/original"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type scene int

const (
	title scene = iota
	campaigns
	briefing
	rules
	leaders
	war
	tactical
	formations
	editor
	options
	help
	transfer
	textEntry
)

const (
	width    = 1024
	height   = 768
	mapX     = 24
	mapY     = 76
	mapScale = 1.5
)

type button struct {
	r     image.Rectangle
	label string
	fn    func()
}
type Game struct {
	DataRoot                                   string
	Touch                                      bool
	CanvasWidth                                int
	LastInputFrame                             int
	Screensaver                                bool
	ScreensaverSprites                         [4]*ebiten.Image
	TouchErase                                 bool
	KeyboardShift                              bool
	Pointer                                    pointer
	Library                                    *original.Library
	State                                      *game.State
	Campaigns                                  []game.Campaign
	Scene, Return, HelpReturn                  scene
	SelectedCampaign, Selected, SelectedLeader int
	Pending                                    game.Campaign
	Map                                        *ebiten.Image
	Pictures                                   map[string]*ebiten.Image
	BattleTexture                              *ebiten.Image
	BattlePixels                               [16000]byte
	BattleRGBA                                 [320 * 200 * 4]byte
	Buttons                                    []button
	SavePath                                   string
	Seed                                       uint64
	Frame                                      int
	SmokeFrames                                int
	Screenshot                                 string
	shotDone                                   bool
	Demo                                       bool
	Paused                                     bool
	Overview                                   bool
	Speed                                      int
	Message                                    string
	MessageUntil                               int
	Cursor                                     game.Point
	DrawingRoute                               bool
	Waypoints                                  []game.Point
	BattleID                                   int
	TacticGroup                                int
	Formation                                  int
	FormationMan                               int
	EditFormations                             bool
	RuleIndex                                  int
	FightDelay                                 int
	TransferFrom, TransferTo                   int
	TransferMen                                [4]int
	EditorTool                                 int
	EditorTile                                 byte
	EditorSide                                 game.Side
	EditorSquad                                int
	EntryField, EntryBuffer                    string
	StoryScroll                                int
	EventCount                                 int
	Sound                                      bool
	audio                                      *soundPlayer
}

func New(l *original.Library, save string, seed uint64) *Game {
	return NewWithOptions(l, Options{SavePath: save, DataRoot: ".", Seed: seed})
}

type Options struct {
	SavePath, DataRoot string
	Touch              bool
	Seed               uint64
}

func NewWithOptions(l *original.Library, options Options) *Game {
	save, seed := options.SavePath, options.Seed
	g := &Game{Library: l, Campaigns: append([]game.Campaign(nil), l.Campaigns...), SavePath: save, Seed: seed, Selected: -1, EditorSquad: -1, Speed: 1, Scene: title, Formation: 1, EditorTile: 1, FightDelay: 1800, TacticGroup: -2, Sound: true, Pictures: make(map[string]*ebiten.Image)}
	g.DataRoot = options.DataRoot
	g.Touch = options.Touch
	g.loadUserCampaigns()
	g.loadFormations()
	return g
}
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	g.CanvasWidth = width
	if g.Touch && outsideHeight > 0 {
		g.CanvasWidth = max(width, (outsideWidth*height+outsideHeight-1)/outsideHeight)
	}
	return g.CanvasWidth, height
}
func just(k ebiten.Key) bool { return inpututil.IsKeyJustPressed(k) }
func repeat(k ebiten.Key) bool {
	n := inpututil.KeyPressDuration(k)
	return n == 1 || n > 20 && n%4 == 0
}
func shift() bool                { return ebiten.IsKeyPressed(ebiten.KeyShift) }
func (g *Game) toast(msg string) { g.Message = msg; g.MessageUntil = g.Frame + 240 }
func (g *Game) result(err error) {
	if err != nil {
		g.toast(err.Error())
	}
}
func (g *Game) refreshMap(w *game.World) { g.Map = ebiten.NewImageFromImage(original.MapImage(w)) }
func (g *Game) choose(n int) {
	g.SelectedCampaign = (n + len(g.Campaigns)) % len(g.Campaigns)
	g.Pending = g.Campaigns[g.SelectedCampaign]
	g.Pending.Flags = append([]game.Flag(nil), g.Pending.Flags...)
	g.SelectedLeader = g.Pending.Opponent
	g.refreshMap(&g.Pending.World)
	g.StoryScroll = 0
}
func (g *Game) Start(n int) error {
	if n < 0 || n >= len(g.Campaigns) {
		return fmt.Errorf("campaign index outside catalogue")
	}
	g.choose(n)
	g.begin()
	return nil
}
func (g *Game) begin() {
	g.Pending.Opponent = g.SelectedLeader
	g.State = game.New(g.Pending, g.Library.Leaders[g.SelectedLeader], g.Library.Formations, g.Seed)
	g.State.Options.FightDelay = g.FightDelay
	g.Scene = war
	g.Paused = false
	g.Overview = false
	g.DrawingRoute = false
	g.Selected = -1
	g.EventCount = 0
	g.refreshMap(&g.State.World)
	g.nextSquad()
}

func (g *Game) Update() error {
	g.Frame++
	g.updatePointer()
	if g.SmokeFrames > 0 && g.Frame > g.SmokeFrames {
		return ebiten.Termination
	}
	if g.Touch {
		active := g.Pointer.Pressed || g.Pointer.Down || len(inpututil.AppendJustPressedKeys(nil)) > 0
		if active {
			g.LastInputFrame = g.Frame
			if g.Screensaver {
				g.Screensaver = false
				return nil
			}
		}
		running := (g.Scene == war || g.Scene == tactical) && !g.Paused && g.State != nil && !g.State.SurrenderOffered && g.State.Outcome == game.Running && g.State.Fault == ""
		if !running && g.Frame-g.LastInputFrame > 12*60 {
			g.Screensaver = true
			return nil
		}
	}
	if just(ebiten.KeyF11) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	if just(ebiten.KeyF1) && g.Scene != textEntry {
		if g.Scene == help {
			g.Scene = g.HelpReturn
		} else {
			g.HelpReturn = g.Scene
			g.Scene = help
		}
		return nil
	}
	if g.Pointer.Pressed {
		x, y := g.Pointer.X, g.Pointer.Y
		for _, b := range g.Buttons {
			if image.Pt(x, y).In(b.r) {
				b.fn()
				return nil
			}
		}
	}
	switch g.Scene {
	case title:
		if just(ebiten.KeyEnter) {
			g.choose(g.SelectedCampaign)
			g.Scene = campaigns
		}
		if just(ebiten.KeyL) {
			g.load()
		}
		if just(ebiten.KeyR) && g.State != nil {
			g.Scene = war
			g.Paused = true
			g.refreshMap(&g.State.World)
		}
		if just(ebiten.KeyE) {
			g.choose(0)
			g.beginEditor(false)
		}
		if just(ebiten.KeyO) {
			g.Return = title
			g.Scene = options
		}
		if just(ebiten.KeyQ) {
			return ebiten.Termination
		}
	case campaigns:
		if repeat(ebiten.KeyArrowDown) {
			g.choose(g.SelectedCampaign + 1)
		}
		if repeat(ebiten.KeyArrowUp) {
			g.choose(g.SelectedCampaign - 1)
		}
		if just(ebiten.KeyEnter) {
			g.Scene = briefing
		}
		if just(ebiten.KeyE) {
			g.beginEditor(false)
		}
		if just(ebiten.KeyN) {
			g.beginEditor(true)
		}
		if just(ebiten.KeyEscape) {
			g.Scene = title
		}
	case briefing:
		if just(ebiten.KeyEnter) {
			g.Scene = rules
		}
		if repeat(ebiten.KeyArrowDown) {
			g.StoryScroll++
		}
		if repeat(ebiten.KeyArrowUp) {
			g.StoryScroll = max(0, g.StoryScroll-1)
		}
		if just(ebiten.KeyEscape) {
			g.Scene = campaigns
		}
	case rules:
		if repeat(ebiten.KeyArrowDown) {
			g.RuleIndex = (g.RuleIndex + 1) % 9
		}
		if repeat(ebiten.KeyArrowUp) {
			g.RuleIndex = (g.RuleIndex + 8) % 9
		}
		if just(ebiten.KeySpace) || just(ebiten.KeyArrowRight) {
			g.cycleRule(g.RuleIndex)
		}
		if just(ebiten.KeyEnter) {
			if g.Return == editor {
				g.Scene = editor
				g.Return = title
			} else {
				g.Scene = leaders
			}
		}
		if just(ebiten.KeyEscape) {
			if g.Return == editor {
				g.Scene = editor
				g.Return = title
			} else {
				g.Scene = briefing
			}
		}
	case leaders:
		if just(ebiten.KeyArrowRight) || just(ebiten.KeyArrowDown) {
			g.SelectedLeader = (g.SelectedLeader + 1) % 8
		}
		if just(ebiten.KeyArrowLeft) || just(ebiten.KeyArrowUp) {
			g.SelectedLeader = (g.SelectedLeader + 7) % 8
		}
		if just(ebiten.KeyEnter) {
			if g.Return == editor {
				g.Pending.Opponent = g.SelectedLeader
				g.Scene = editor
				g.Return = title
			} else {
				g.begin()
			}
		}
		if just(ebiten.KeyEscape) {
			if g.Return == editor {
				g.Scene = editor
				g.Return = title
			} else {
				g.Scene = rules
			}
		}
	case war:
		g.updateWar()
	case tactical:
		g.updateTactical()
	case formations:
		g.updateFormation()
	case transfer:
		g.updateTransfer()
	case editor:
		g.updateEditor()
	case textEntry:
		g.updateEntry()
	case options:
		if just(ebiten.KeySpace) {
			g.FightDelay = next(g.FightDelay, []int{900, 1800, 3600})
		}
		if just(ebiten.KeyM) {
			g.Sound = !g.Sound
		}
		if just(ebiten.KeyEscape) || just(ebiten.KeyEnter) {
			g.Scene = g.Return
		}
	case help:
		if just(ebiten.KeyEscape) || just(ebiten.KeyEnter) {
			g.Scene = g.HelpReturn
		}
	}
	return nil
}

func next(v int, values []int) int {
	for i, x := range values {
		if x == v {
			return values[(i+1)%len(values)]
		}
	}
	return values[0]
}
func (g *Game) cycleRule(i int) {
	r := &g.Pending.Rules
	switch i {
	case 0:
		r.VillageFood = !r.VillageFood
	case 1:
		r.FortFood = !r.FortFood
	case 2:
		r.SupplyRange = next(r.SupplyRange, []int{1, 2, 4})
	case 3:
		r.Recruitment = next(r.Recruitment, []int{30, 60, 120})
	case 4:
		r.Fitness = next(r.Fitness, []int{4, 2, 1})
	case 5:
		r.Visibility = next(r.Visibility, []int{20, 40, 80})
	case 6:
		r.WaterSpeed = next(r.WaterSpeed, []int{10, 20, 40})
	case 7:
		r.MountainSpeed = next(r.MountainSpeed, []int{4, 8, 16})
	case 8:
		r.ForestSpeed = next(r.ForestSpeed, []int{3, 6})
	}
}

func (g *Game) pointAtMouse() (game.Point, bool) {
	x, y := g.Pointer.X, g.Pointer.Y
	p := game.Point{X: int(float64(x-mapX) / (4 * mapScale)), Y: int(float64(y-mapY) / (2 * mapScale))}
	return p, x >= mapX && y >= mapY && x < mapX+480 && y < mapY+600 && p.Valid()
}
func (g *Game) squadAt(p game.Point, visible bool) int {
	best := -1
	d := 9
	for i, q := range g.State.Squads {
		if !q.Alive() || visible && !q.Visible {
			continue
		}
		v := game.Distance(p, q.Position)
		if v < d {
			d = v
			best = i
		}
	}
	return best
}
func (g *Game) nextSquad() {
	if g.State == nil {
		return
	}
	for n := 1; n <= 20; n++ {
		id := (g.Selected + n + 20) % 20
		q := &g.State.Squads[id]
		if q.Alive() && q.Side == game.Player {
			g.Selected = id
			g.Cursor = q.Position
			return
		}
	}
}
func (g *Game) moveCursor() {
	if repeat(ebiten.KeyArrowLeft) {
		g.Cursor.X = max(0, g.Cursor.X-1)
	}
	if repeat(ebiten.KeyArrowRight) {
		g.Cursor.X = min(78, g.Cursor.X+1)
	}
	if repeat(ebiten.KeyArrowUp) {
		g.Cursor.Y = max(0, g.Cursor.Y-1)
	}
	if repeat(ebiten.KeyArrowDown) {
		g.Cursor.Y = min(194, g.Cursor.Y+1)
	}
}

func (g *Game) updateWar() {
	if g.State == nil {
		g.Scene = title
		return
	}
	if just(ebiten.KeyEscape) {
		if g.DrawingRoute {
			g.DrawingRoute = false
			g.Waypoints = nil
		} else {
			g.Paused = !g.Paused
		}
	}
	if just(ebiten.KeySpace) {
		g.Paused = !g.Paused
	}
	if just(ebiten.KeyT) {
		g.Speed = (g.Speed + 1) % 4
	}
	if just(ebiten.KeyV) {
		g.Overview = !g.Overview
	}
	if just(ebiten.KeyF5) {
		g.save()
	}
	if just(ebiten.KeyF9) {
		g.load()
		return
	}
	if just(ebiten.KeyTab) {
		g.nextSquad()
	}
	if just(ebiten.KeyC) {
		g.State.Events = nil
	}
	if g.State.SurrenderOffered {
		if just(ebiten.KeyY) {
			g.State.AcceptSurrender()
		}
		if just(ebiten.KeyN) {
			g.State.DeclineSurrender()
		}
	}
	if just(ebiten.KeyF) {
		g.openFormation(false)
	}
	if just(ebiten.KeyD) {
		g.openTransfer(false)
	}
	if just(ebiten.KeyJ) {
		g.openTransfer(true)
	}
	if just(ebiten.KeyZ) {
		g.zoom()
	}
	if just(ebiten.Key1) {
		g.result(g.State.SetPace(g.Selected, 3))
	}
	if just(ebiten.Key2) {
		g.result(g.State.SetPace(g.Selected, 2))
	}
	if just(ebiten.Key3) {
		g.result(g.State.SetPace(g.Selected, 1))
	}
	if just(ebiten.Key0) {
		g.result(g.State.SetPace(g.Selected, 0))
	}
	previousCursor := g.Cursor
	g.moveCursor()
	if g.DrawingRoute && g.Cursor != previousCursor {
		g.Waypoints = append(g.Waypoints, g.Cursor)
	}
	if p, ok := g.pointAtMouse(); ok {
		if g.Pointer.Pressed {
			g.Cursor = p
			if g.DrawingRoute {
				g.Waypoints = append(g.Waypoints, p)
			} else {
				g.Selected = g.squadAt(p, true)
			}
		}
		if g.DrawingRoute && g.Pointer.Down {
			if len(g.Waypoints) == 0 || g.Waypoints[len(g.Waypoints)-1] != p {
				g.Waypoints = append(g.Waypoints, p)
			}
		}
		if g.Pointer.Right {
			g.Cursor = p
			if g.DrawingRoute {
				g.Waypoints = append(g.Waypoints, p)
				g.commitRoute()
			} else {
				g.result(g.State.Move(g.Selected, []game.Point{p}))
			}
		}
	}
	if just(ebiten.KeyM) {
		if g.DrawingRoute {
			g.commitRoute()
		} else if g.Selected >= 0 && g.State.Squads[g.Selected].Side == game.Player {
			g.DrawingRoute = true
			g.Waypoints = nil
			g.Cursor = g.State.Squads[g.Selected].Position
		}
	}
	if just(ebiten.KeyEnter) {
		if g.DrawingRoute {
			g.Waypoints = append(g.Waypoints, g.Cursor)
			g.commitRoute()
		} else {
			g.Selected = g.squadAt(g.Cursor, true)
		}
	}
	if !g.Paused && !g.Overview {
		steps := []int{1, 1, 2, 4}[g.Speed]
		if g.Speed == 0 && g.Frame%2 == 0 {
			steps = 0
		}
		for n := 0; n < steps; n++ {
			if g.Demo && g.State.Tick%240 == 0 {
				g.State.AutoCommand(game.Player)
			}
			g.State.Step()
		}
	}
	if len(g.State.Events) > g.EventCount && g.Sound {
		g.beep()
	}
	g.EventCount = len(g.State.Events)
}
func (g *Game) commitRoute() {
	if len(g.Waypoints) == 0 {
		g.Waypoints = append(g.Waypoints, g.Cursor)
	}
	g.result(g.State.Move(g.Selected, g.Waypoints))
	g.Waypoints = nil
	g.DrawingRoute = false
}
func (g *Game) zoom() {
	b, err := g.State.Zoom(g.Selected)
	if err != nil {
		g.result(err)
		return
	}
	g.BattleID = b.ID
	g.TacticGroup = -2
	g.Scene = tactical
}
func (g *Game) updateTactical() {
	b := g.State.Battle(g.BattleID)
	if b == nil || b.Done {
		g.Scene = war
		return
	}
	if just(ebiten.KeySpace) {
		g.Paused = !g.Paused
	}
	if just(ebiten.KeyR) {
		g.result(g.State.Retreat(b.ID))
	}
	if !b.Attacking && just(ebiten.KeyA) {
		g.result(g.State.CommandBattle(b.ID, -1, game.Attack))
	} else if g.TacticGroup < -1 {
		switch {
		case just(ebiten.KeyA):
			g.TacticGroup = int(game.Archer)
		case just(ebiten.KeyB):
			g.TacticGroup = int(game.Barbarian)
		case just(ebiten.KeyK):
			g.TacticGroup = int(game.Knight)
		case just(ebiten.KeyS):
			g.TacticGroup = -1
		}
	} else {
		order := -1
		switch {
		case just(ebiten.KeyA):
			order = int(game.Attack)
		case just(ebiten.KeyF):
			order = int(game.Advance)
		case just(ebiten.KeyB):
			order = int(game.BackOff)
		case just(ebiten.KeyS):
			order = int(game.Hold)
		}
		if order >= 0 {
			g.result(g.State.CommandBattle(b.ID, g.TacticGroup, game.Tactic(order)))
			g.TacticGroup = -2
		}
	}
	if just(ebiten.KeyBackspace) {
		g.TacticGroup = -2
	}
	if !g.Paused {
		for i := 0; i < []int{1, 1, 2, 4}[g.Speed]; i++ {
			g.State.Step()
		}
	}
}

func (g *Game) save() {
	if g.State == nil {
		return
	}
	if err := writeAtomic(g.SavePath, func(f *os.File) error { return g.State.Encode(f) }); err != nil {
		g.result(err)
	} else {
		if g.Touch {
			g.toast("Game saved")
		} else {
			g.toast("Game saved to " + g.SavePath)
		}
	}
}
func (g *Game) load() {
	f, err := os.Open(g.SavePath)
	if err != nil {
		g.result(err)
		return
	}
	defer f.Close()
	s, err := game.Decode(f)
	if err != nil {
		g.result(err)
		return
	}
	g.State = s
	g.Scene = war
	g.Selected = -1
	g.Paused = true
	g.Overview = false
	g.DrawingRoute = false
	g.refreshMap(&s.World)
	g.nextSquad()
	for _, b := range s.Battles {
		if b.Manual && !b.Done {
			g.Scene = tactical
			g.BattleID = b.ID
			break
		}
	}
	if s.Outcome != game.Running {
		g.toast("Game loaded")
	} else if s.SurrenderOffered && g.Touch {
		g.toast("Enemy surrender — choose ACCEPT or CONTINUE")
	} else if s.SurrenderOffered {
		g.toast("Enemy surrender — Y accepts, N continues the war")
	} else if g.Touch {
		g.toast("Game loaded — tap RESUME")
	} else {
		g.toast("Game loaded — Space to resume")
	}
}
func writeAtomic(path string, write func(*os.File) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ancient-war-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := write(f); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (g *Game) loadUserCampaigns() {
	paths, _ := filepath.Glob(filepath.Join(g.DataRoot, "campaigns/*.json"))
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		c, err := g.Library.LoadCampaign(f, path)
		f.Close()
		if err == nil {
			g.Campaigns = append(g.Campaigns, c)
		}
	}
}
func (g *Game) loadFormations() {
	b, err := os.ReadFile(filepath.Join(g.DataRoot, "formations.json"))
	if err != nil {
		return
	}
	var fs [10]game.Formation
	if json.Unmarshal(b, &fs) != nil {
		return
	}
	for n := 6; n <= 9; n++ {
		valid := true
		for i, k := range fs[n].Kind {
			if k < game.Archer || k > game.Barbarian || fs[n].X[i] < 0 || fs[n].X[i] > 79 || fs[n].Y[i] < 0 || fs[n].Y[i] > 79 {
				valid = false
			}
		}
		if valid {
			g.Library.Formations[n] = fs[n]
		}
	}
}

func (g *Game) capture(screen *ebiten.Image) {
	if g.Screenshot == "" || g.shotDone || g.SmokeFrames == 0 || g.Frame < g.SmokeFrames-1 {
		return
	}
	g.shotDone = true
	im := image.NewRGBA(screen.Bounds())
	screen.ReadPixels(im.Pix)
	g.result(writeAtomic(g.Screenshot, func(f *os.File) error { return png.Encode(f, im) }))
}

func wrap(s string, cols int) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			out = append(out, "")
			continue
		}
		var row string
		for _, w := range strings.Fields(line) {
			if len(row)+len(w)+1 > cols && row != "" {
				out = append(out, row)
				row = ""
			}
			if row != "" {
				row += " "
			}
			row += w
		}
		out = append(out, row)
	}
	return out
}
