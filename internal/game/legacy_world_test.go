package game

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type worldSquadExpectation struct {
	advanceSquad
	Position                   Point
	Side                       Side
	Condition, Food, Formation int
}

type worldExpectation struct {
	Squads       [20]worldSquadExpectation
	Flags        []Flag
	CoarsePool   [200]byte  `json:"coarse_pool"`
	FinePool     [1000]byte `json:"fine_pool"`
	HomeFlag     int        `json:"home_flag"`
	HomeCell     int        `json:"home_cell"`
	TargetCell   int        `json:"target_cell"`
	TargetFlag   int        `json:"target_flag"`
	Alert        bool
	AICursor     int    `json:"ai_cursor"`
	Random       uint64 `json:"random_state"`
	Casualties   [2]int
	FlagCaptures [2]int `json:"flag_captures"`
	Delays       [20]int
	Terrain      [20]int
	Parameters   [19]int
	Captured     [2]int
	Links        [20]int
	Visible      [20]bool
	Phases       [20]int
	Chances      [20]int
	RawSides     [20]Side `json:"raw_sides"`
	Counts       [20]int
	Influence    [200]int
	Outcome      Outcome
	Offers       int
}

type worldFixtureCase struct {
	Seed         uint64
	Leader       int
	Parameters   [19]int
	Tiles        [200]byte
	Influence    [200]int
	Rules        Rules
	Squads       [20]Squad
	Flags        []Flag
	Brain        StrategyBrain
	CoarsePool   [200]byte  `json:"coarse_pool"`
	FinePool     [1000]byte `json:"fine_pool"`
	FineCursor   int        `json:"fine_cursor"`
	AICursor     int        `json:"ai_cursor"`
	Reserved     int
	Battles      []Battle
	Delay        int
	Iterations   int
	Answer       string
	Participants [2]int
	Frames       int
	Checkpoints  []struct {
		Frame    int
		Expected worldExpectation
	}
	Expected worldExpectation
}

func worldSquadSnapshot(q Squad) worldSquadExpectation {
	status := 0
	if !q.Alive() {
		status = 9999
	} else if q.Moving() {
		status = q.Pace
	}
	return worldSquadExpectation{
		advanceSquad: advanceSquad{Men: q.Men, Pace: q.Pace, Status: status, Clock: q.MoveClock,
			Target: q.AITarget, Active: q.AIActive, CoarseActive: q.AIPathActive, CoarseOffset: q.AIPathOffset,
			FineActive: q.FineActive, FineOffset: q.FineOffset},
		Position: q.Position, Side: q.Side, Condition: q.Condition, Food: q.Food, Formation: q.Formation,
	}
}

// Build the input framebuffer directly from M. The oracle samples those same
// unchanged disk pixels with the actual resident 64ee/1e4f routines.
func originalWorldPixels(w *World, pieces []byte) {
	for cell, tile := range w.Tiles {
		start := (int(tile)-1)*164 + 4
		for row := 0; row < 10; row++ {
			for plane := 0; plane < 2; plane++ {
				at := plane*16000 + (cell/10*10+row)*80 + cell%10*8
				copy(w.Pixels[at:at+8], pieces[start+row*16+plane*8:start+row*16+plane*8+8])
			}
		}
	}
}

func (c worldFixtureCase) native(files map[string][]byte) *State {
	s := &State{Random: c.Seed, World: World{Tiles: c.Tiles}, Leader: Leader{Parameters: c.Parameters},
		LeaderIndex: c.Leader, Influence: c.Influence, Rules: c.Rules, Squads: c.Squads,
		Flags: append([]Flag(nil), c.Flags...), Brain: c.Brain, RoutePool: c.FinePool,
		RouteCursor: c.FineCursor, AICursor: c.AICursor, ReservedSquad: c.Reserved}
	s.Brain.Initialized, s.Brain.PathPool = true, c.CoarsePool
	s.Options = DefaultOptions()
	if c.Delay != 0 {
		s.Options.FightDelay = c.Delay
	}
	s.Battles = append([]Battle(nil), c.Battles...)
	s.NextBattleID = len(s.Battles)
	originalWorldPixels(&s.World, files["M"])
	for i := range s.Squads {
		s.Squads[i].Visible = s.Squads[i].Side == Player || s.Rules.Visibility == 80
		if q := &s.Squads[i]; q.FineActive {
			s.refreshFineView(q)
		}
	}
	return s
}

func readWorldFixtures(t *testing.T, name, routine string) []worldFixtureCase {
	t.Helper()
	f, err := os.Open("testdata/" + name + "-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var fixture struct {
		Routine string
		Cases   []worldFixtureCase
	}
	if err := json.NewDecoder(r).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	minimum := 512
	if name == "world" || name == "surrender-world" {
		minimum = 256
	}
	if fixture.Routine != routine || len(fixture.Cases) < minimum {
		t.Fatal("unexpected world fixture provenance")
	}
	return fixture.Cases
}

func checkWorldSnapshot(t *testing.T, index int, s *State, e worldExpectation) {
	t.Helper()
	for i, q := range s.Squads {
		got := worldSquadSnapshot(q)
		if !q.Alive() {
			// An inactive DOS slot can contain a former countdown in Side.
			got.Side = e.Squads[i].Side
		}
		if got != e.Squads[i] {
			t.Errorf("case %d squad %d: Go %+v, original %+v", index, i, got, e.Squads[i])
		}
	}
	if s.Random != e.Random || s.Stats.Casualties != e.Casualties || s.Stats.Flags != e.FlagCaptures || !reflect.DeepEqual(s.Flags, e.Flags) || s.Brain.PathPool != e.CoarsePool || s.RoutePool != e.FinePool || s.AICursor != e.AICursor || s.Brain.HomeFlag != e.HomeFlag || s.Brain.HomeCell != e.HomeCell || s.Brain.TargetCell != e.TargetCell || s.Brain.TargetFlag != e.TargetFlag || s.Brain.Alert != e.Alert {
		t.Errorf("case %d casualties %v/%v, captures %v/%v, cursor %d/%d, alert %t/%t, coarse equal %t, fine equal %t", index, s.Stats.Casualties, e.Casualties, s.Stats.Flags, e.FlagCaptures, s.AICursor, e.AICursor, s.Brain.Alert, e.Alert, s.Brain.PathPool == e.CoarsePool, s.RoutePool == e.FinePool)
		t.Errorf("case %d globals differ: Go RNG %x, flags %v, home %d/%d, target %d/%d; original RNG %x, flags %v, home %d/%d, target %d/%d", index, s.Random, s.Flags, s.Brain.HomeFlag, s.Brain.HomeCell, s.Brain.TargetFlag, s.Brain.TargetCell, e.Random, e.Flags, e.HomeFlag, e.HomeCell, e.TargetFlag, e.TargetCell)
	}
	if t.Failed() {
		t.FailNow()
	}
}

func checkCombatSnapshot(t *testing.T, index int, s *State, e worldExpectation, automatic, coupled bool) {
	t.Helper()
	if s.Leader.Parameters != e.Parameters || s.Stats.Captured != e.Captured {
		t.Errorf("case %d commander parameters %v/%v or captured %v/%v", index, s.Leader.Parameters, e.Parameters, s.Stats.Captured, e.Captured)
	}
	for i, q := range s.Squads {
		link, phase, chance := 0, -2, 0
		for bi := range s.Battles {
			b := &s.Battles[bi]
			if b.Squads[q.Side] == i {
				phase, chance = b.AutoPhase[q.Side], b.AutoChance[q.Side]
				if !b.Done {
					link = b.Squads[q.Side.Other()] + 1
				}
			}
		}
		if link != e.Links[i] || phase != e.Phases[i] || (!coupled || link != 0) && chance != e.Chances[i] {
			t.Errorf("case %d squad %d: battle link %d/%d, phase %d/%d, chance %d/%d", index, i, link, e.Links[i], phase, e.Phases[i], chance, e.Chances[i])
		}
		if automatic {
			// A dead DOS slot can retain its positive countdown as an invalid
			// side value. Reallocation overwrites it before that slot is used.
			if q.originalCount() != e.Counts[i] || q.Alive() && s.originalSide(&q) != e.RawSides[i] {
				t.Errorf("case %d squad %d: cached strength %d/%d, raw side %d/%d", index, i, q.originalCount(), e.Counts[i], s.originalSide(&q), e.RawSides[i])
			}
		}
		if (!automatic || coupled) && q.Alive() && q.Visible != e.Visible[i] {
			t.Errorf("case %d squad %d: visible %t/%t", index, i, q.Visible, e.Visible[i])
		}
	}
}

func TestWorldRoutinesMatchOriginalMachineCode(t *testing.T) {
	files := originalTestFiles(t)
	for _, item := range []struct{ name, routine string }{
		{"supply", "overlay5:5683+6ca1"}, {"flags", "overlay5:3869+815c+3a3b"},
		{"recruit", "overlay5:58c7+59c8+1731"}, {"movement", "overlay5:5bc9+6273+64ee"},
		{"encounters", "overlay5:4c8c+087e+0921+346b"},
		{"automatic", "overlay5:5172+5442+346b"},
		{"assessment", "overlay5:346b army assessment and surrender"},
	} {
		t.Run(item.name, func(t *testing.T) {
			for index, c := range readWorldFixtures(t, item.name, item.routine) {
				s := c.native(files)
				switch item.name {
				case "supply":
					s.updateSupplies()
				case "flags":
					s.captureFlags()
				case "recruit":
					s.recruit()
				case "encounters":
					s.encounters()
					checkCombatSnapshot(t, index, s, c.Expected, false, false)
				case "automatic":
					for n := 0; n < c.Iterations; n++ {
						s.Tick++
						s.stepAutomaticBattles()
					}
					checkCombatSnapshot(t, index, s, c.Expected, true, false)
				case "assessment":
					s.reassessArmies(c.Participants[0], c.Participants[1])
					if s.SurrenderOffered {
						if c.Answer == "accept" {
							s.AcceptSurrender()
						} else {
							s.DeclineSurrender()
						}
					}
					if s.Outcome != c.Expected.Outcome || s.Leader.Parameters != c.Expected.Parameters {
						t.Errorf("case %d outcome %d/%d, parameters %v/%v", index, s.Outcome, c.Expected.Outcome, s.Leader.Parameters, c.Expected.Parameters)
					}
				case "movement":
					if delays := s.updateMovement(); delays != c.Expected.Delays {
						t.Errorf("case %d movement delays: Go %v, original %v", index, delays, c.Expected.Delays)
					}
					for i, q := range s.Squads {
						terrain := map[Terrain]int{Forest: 0x55, Water: 0x66, Mountain: 0x99, Plain: 0x11, Fort: 0x44, Village: 0x44, Hill: 0x44, Bridge: 0x44}[s.World.TerrainAt(q.Position)]
						if terrain != c.Expected.Terrain[i] && !(terrain == 0x11 && c.Expected.Terrain[i] == 0x44) {
							t.Errorf("case %d squad %d terrain: Go %x, original %x", index, i, terrain, c.Expected.Terrain[i])
						}
					}
				}
				checkWorldSnapshot(t, index, s, c.Expected)
			}
		})
	}
}

func TestCompleteWorldFramesMatchOriginalMachineCode(t *testing.T) {
	checkWorldSequences(t, "world", "overlay5:310a..3201 complete world frames")
}

func TestSurrenderInterruptionsMatchOriginalMachineCode(t *testing.T) {
	checkWorldSequences(t, "surrender-world", "overlay5:310a..3201 with surrender interruptions")
}

func checkWorldSequences(t *testing.T, name, routine string) {
	t.Helper()
	files := originalTestFiles(t)
	for index, c := range readWorldFixtures(t, name, routine) {
		s := c.native(files)
		checkpoint, offers := 0, 0
		for frame := 1; frame <= c.Frames; frame++ {
			s.Step()
			for s.SurrenderOffered && s.Outcome == Running {
				offers++
				s.DeclineSurrender()
				if s.WorldResume != 0 {
					s.Step()
				}
			}
			if checkpoint < len(c.Checkpoints) && c.Checkpoints[checkpoint].Frame == frame {
				e := c.Checkpoints[checkpoint].Expected
				if offers != e.Offers {
					t.Errorf("case %d frame %d surrender offers %d/%d", index, frame, offers, e.Offers)
				}
				checkCombatSnapshot(t, index, s, e, true, true)
				if s.Influence != e.Influence {
					for cell, value := range s.Influence {
						if value != e.Influence[cell] {
							t.Errorf("case %d frame %d influence cell %d: Go %d, original %d, initial %d", index, frame, cell, value, e.Influence[cell], c.Influence[cell])
						}
					}
				}
				checkWorldSnapshot(t, index, s, e)
				checkpoint++
			}
		}
	}
}
