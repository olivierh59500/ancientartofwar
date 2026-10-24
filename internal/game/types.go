// Package game contains the native simulation. It has no graphics, audio,
// operating-system, CPU emulation, or Ebitengine dependencies.
package game

import "fmt"

const (
	StateVersion  = 2
	MaxSquads     = 20
	MaxMen        = 14
	MaxFlags      = 10
	MapWidth      = 80  // one original framebuffer byte = four horizontal pixels
	MapHeight     = 200 // two interlaced pixel rows per logical row
	MaxRouteSteps = 1000
)

type Side int

const (
	Player Side = iota
	Enemy
)

func (s Side) String() string {
	if s == Player {
		return "Allies"
	}
	return "Enemy"
}
func (s Side) Other() Side { return 1 - s }

// Kind order matches the four 20-byte arrays in the DOS campaign files.
type Kind int

const (
	Archer Kind = iota
	Knight
	Barbarian
	Spy
)

func (k Kind) String() string {
	if k < Archer || k > Spy {
		return "?"
	}
	return [...]string{"Archers", "Knights", "Barbarians", "Spies"}[k]
}

type Point struct{ X, Y int }

func (p Point) Valid() bool   { return p.X >= 0 && p.X < MapWidth-1 && p.Y >= 0 && p.Y < MapHeight-5 }
func (p Point) Cell() int     { return min(199, max(0, (p.Y+5)/10*10+(p.X+1)/8)) }
func Distance(a, b Point) int { return max(abs(a.X-b.X)*2, abs(a.Y-b.Y)) }
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func sign(x int) int {
	if x < 0 {
		return -1
	}
	if x > 0 {
		return 1
	}
	return 0
}

type Terrain uint8

const (
	Plain Terrain = iota
	Forest
	Mountain
	Water
	Hill
	Bridge
	Fort
	Village
)

func (t Terrain) String() string {
	return [...]string{"Plain", "Forest", "Mountain", "Water", "Hill", "Bridge", "Fort", "Village"}[min(int(t), 7)]
}
func IsWaterTile(t byte) bool { return t == 7 || t == 8 || t >= 13 && t <= 15 || t >= 19 && t <= 36 }
func IsMountainTile(t byte) bool {
	return t >= 37 && t <= 54 || t == 65 || t == 66 || t == 71 || t == 72
}
func IsForestTile(t byte) bool { return t >= 55 && t <= 57 || t >= 61 && t <= 63 || t >= 67 && t <= 69 }

type Rules struct {
	VillageFood   bool
	FortFood      bool
	SupplyRange   int
	Recruitment   int
	Fitness       int
	Visibility    int
	WaterSpeed    int
	MountainSpeed int
	ForestSpeed   int
}

func (r Rules) Validate() error {
	for _, item := range []struct {
		v       int
		choices []int
	}{
		{r.SupplyRange, []int{1, 2, 4}}, {r.Recruitment, []int{30, 60, 120}},
		{r.Fitness, []int{1, 2, 4}}, {r.Visibility, []int{20, 40, 80}},
		{r.WaterSpeed, []int{10, 20, 40}}, {r.MountainSpeed, []int{4, 8, 16}}, {r.ForestSpeed, []int{3, 6}},
	} {
		ok := false
		for _, v := range item.choices {
			ok = ok || v == item.v
		}
		if !ok {
			return fmt.Errorf("invalid rule value %d", item.v)
		}
	}
	return nil
}

type Squad struct {
	ID       int
	Side     Side
	Position Point
	Men      [4]int
	// Original counters: 1 = fast march, 2 = march, 3 = slow march.
	Pace         int
	Condition    int // 0..49; tactical retreat can temporarily raise it to 98
	Food         int // original 1..49 scale
	Formation    int
	Route        []Point
	Stopped      bool
	MoveClock    int
	BattleID     int // zero when not engaged, otherwise battle ID + 1
	Cooldown     int
	Visible      bool
	Sighted      bool // original visibility word, including otherwise visible allies
	AITarget     int  // map cell; -1 when unset
	AIActive     bool
	AIPathActive bool
	AIPathOffset int
	FineActive   bool
	FineOffset   int
	CountCached  bool
	CountCache   int // DOS 5442 temporarily omits spies until the next 0c43
}

func (s Squad) Count() int { return s.Men[0] + s.Men[1] + s.Men[2] + s.Men[3] }
func (s Squad) originalCount() int {
	if s.CountCached {
		return s.CountCache
	}
	return s.Count()
}
func (s Squad) Fighters() int { return s.Count() - s.Men[Spy] }
func (s Squad) Alive() bool   { return s.Count() > 0 }
func (s Squad) Moving() bool {
	return (s.FineActive || len(s.Route) > 0) && !s.Stopped && s.BattleID == 0
}

type Flag struct {
	Position Point
	Side     Side
}
type Formation struct {
	Kind [14]Kind
	X, Y [14]int
}

type World struct {
	Tiles [200]byte
	// Two original 16,000-byte CGA planes; a row is 80 bytes.
	Pixels [32000]byte
}

func (w *World) Tile(p Point) byte { return w.Tiles[p.Cell()] }

// TerrainAt translates resident 1000:64ee (overlay 5). Terrain is sampled
// from the real tile pixels, so river banks, trails and passes remain usable.
func (w *World) TerrainAt(p Point) Terrain {
	t := w.Tile(p)
	switch t {
	case 3:
		return Fort
	case 4:
		return Village
	case 5, 6:
		return Hill
	case 9, 10:
		return Bridge
	}
	for n, dy := range []int{5, 4} {
		i := min(15998, max(0, (p.Y+dy)*80+p.X))
		has := func(v byte) bool {
			for _, b := range w.Pixels[i : i+2] {
				if b&15 == v || b>>4 == v {
					return true
				}
			}
			return false
		}
		if has(5) {
			return Forest
		}
		if has(4) {
			return Plain
		}
		if has(6) && IsWaterTile(t) {
			if n == 1 {
				return Water
			}
			continue
		}
		if IsMountainTile(t) {
			if has(9) {
				return Mountain
			}
			if has(0) {
				if n == 1 {
					return Mountain
				}
				continue
			}
		}
		return Plain
	}
	return Plain
}

type Campaign struct {
	ID, Title, Story string
	World            World
	Squads           [MaxSquads]Squad
	Flags            []Flag
	Rules            Rules
	Opponent         int
	Tactical         TacticalResources
}

type Leader struct {
	Name        string
	Description string
	// Exact 19-word parameter row from O2 + 0x277 + index*38.
	Parameters [19]int
}

type Options struct {
	FightDelay          int
	AutoAcceptSurrender bool
}

func DefaultOptions() Options { return Options{FightDelay: 1800} }

type Event struct {
	Tick     int
	Text     string
	Position Point
}
type Outcome int

const (
	Running Outcome = iota
	PlayerWon
	EnemyWon
)

type Stats struct{ Casualties, Captured, Flags [2]int }

type State struct {
	Version               int
	CampaignID, Title     string
	World                 World
	Squads                [MaxSquads]Squad
	Flags                 []Flag
	Rules                 Rules
	Leader                Leader
	LeaderIndex           int
	Formations            [10]Formation
	Options               Options
	Tick                  int
	WorldTick             int // original main-loop clock, frozen during tactical zoom
	Random                uint64
	Battles               []Battle
	NextBattleID          int
	Events                []Event
	Outcome               Outcome
	Reason                string
	Stats                 Stats
	SurrenderOffered      bool
	PendingInitialization bool
	WorldResume           int
	AutomaticCursor       int
	EncounterCursor       int
	EncounterSeen         [MaxSquads]bool
	MovementCursor        int
	MovementDelay         [MaxSquads]int
	Influence             [200]int
	AICursor              int
	Brain                 StrategyBrain
	Tactical              TacticalResources
	TacticalScratch       [28]legacyActor
	Fault                 string
	RoutePool             [1000]byte
	RouteCursor           int
	ReservedSquad         int
}
