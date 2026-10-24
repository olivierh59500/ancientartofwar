package game

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func testState() *State {
	c := Campaign{ID: "test", Title: "Test", Rules: Rules{true, false, 2, 60, 1, 40, 10, 4, 3}, Flags: []Flag{{Point{4, 180}, Player}, {Point{70, 20}, Enemy}}}
	for i := range c.World.Tiles {
		c.World.Tiles[i] = 1
	}
	for i := range c.World.Pixels {
		c.World.Pixels[i] = 0x11
	}
	for i := range c.Squads {
		c.Squads[i] = Squad{ID: i, Position: Point{4, 180}, Pace: 2, Condition: 49, Food: 49, Formation: 1, AITarget: -1}
	}
	c.Squads[0].Men = [4]int{4, 6, 4, 0}
	c.Squads[1].Men = [4]int{4, 6, 4, 0}
	c.Squads[1].Side = Enemy
	c.Squads[1].Position = Point{70, 20}
	l := Leader{Name: "Test", Parameters: [19]int{4, 100, 100, 70, -25, 25, 3, 500, 20, 48, 65, 4, 20, 110, -5, 2, 1, 2, -40}}
	return New(c, l, [10]Formation{}, 1)
}
func TestSupplyAndConditionFromDOSRules(t *testing.T) {
	s := testState()
	q := &s.Squads[0]
	s.World.Tiles[q.Position.Cell()] = 4
	q.Condition = 20
	q.Food = 2
	s.UpdateInfluence()
	s.updateSupplies()
	if q.Food != 49 || q.Condition != 22 {
		t.Fatalf("village recovery: food=%d condition=%d", q.Food, q.Condition)
	}
	s.World.Tiles[q.Position.Cell()] = 1
	q.Food = 2
	q.Route = []Point{{5, 180}}
	q.Pace = 1
	s.Rules.Fitness = 4
	s.UpdateInfluence()
	s.updateSupplies()
	if q.Condition != 11 || q.Food != 1 || q.Pace != 2 {
		t.Fatalf("fast underfed march: condition=%d food=%d pace=%d", q.Condition, q.Food, q.Pace)
	}
	s.updateSupplies()
	if q.Condition != 4 || q.Pace != 3 {
		t.Fatalf("exhaustion did not force slow march: %+v", q)
	}
}

func TestFortTrainsDuringAutomaticCombat(t *testing.T) {
	s := testState()
	q := &s.Squads[0]
	q.Men = [4]int{1, 0, 0, 0}
	q.BattleID = 1
	s.Rules.Recruitment = 120
	s.World.Tiles[q.Position.Cell()] = 3
	for i := 0; i < 20; i++ {
		s.recruit()
	}
	if q.Count() < 2 || q.Count() > 14 {
		t.Fatalf("fort recruitment in an automatic encounter: %d men", q.Count())
	}
}
func TestDetachAndTransferAreAtomic(t *testing.T) {
	s := testState()
	before := s.Totals()
	q := s.Squads[0]
	if _, err := s.Detach(0, [4]int{5, 0, 0, 0}); err == nil {
		t.Fatal("accepted unavailable men")
	}
	if !reflect.DeepEqual(s.Squads[0], q) {
		t.Fatal("failed detach mutated squad")
	}
	id, err := s.Detach(0, [4]int{2, 1, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Totals(); got != before {
		t.Fatalf("detach lost men %v -> %v", before, got)
	}
	s.Squads[id].Condition = 5
	s.Squads[id].Food = 3
	if err := s.Transfer(id, 0, [4]int{2, 1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if s.Squads[0].Condition != 49 || s.Squads[0].Food != 49 || s.Squads[id].Alive() || s.Totals() != before {
		t.Fatal("join did not preserve men and receiving supplies")
	}
	for i := range s.Squads {
		if !s.Squads[i].Alive() {
			s.Squads[i].Men[Knight] = 1
		}
	}
	if _, err := s.Detach(0, [4]int{1, 0, 0, 0}); err == nil {
		t.Fatal("exceeded global 20-squad cap")
	}
}
func TestStopKeepsRouteAndBudgetIsAtomic(t *testing.T) {
	s := testState()
	if err := s.Move(0, []Point{{40, 180}, {40, 120}}); err != nil {
		t.Fatal(err)
	}
	route := append([]Point(nil), s.Squads[0].Route...)
	s.SetPace(0, 0)
	for i := 0; i < 20; i++ {
		s.Step()
	}
	if !reflect.DeepEqual(route, s.Squads[0].Route) {
		t.Fatal("stop consumed route")
	}
	s.Squads[1].Route = make([]Point, MaxRouteSteps)
	if err := s.Move(0, []Point{{50, 150}}); err == nil {
		t.Fatal("accepted route above shared budget")
	}
	if !reflect.DeepEqual(route, s.Squads[0].Route) {
		t.Fatal("failed move cleared previous route")
	}
}
func TestSpyVisionAndConversion(t *testing.T) {
	s := testState()
	s.Squads[0].Position = Point{10, 50}
	s.Squads[1].Position = Point{30, 60}
	s.Rules.Visibility = 40
	s.updateSightings(false)
	if s.Squads[1].Visible {
		t.Fatal("ordinary squad sees too far")
	}
	s.Squads[0].Men[Spy] = 1
	s.updateSightings(false)
	if !s.Squads[1].Visible {
		t.Fatal("spy did not double sight")
	}
	c := Campaign{World: s.World, Squads: s.Squads, Flags: s.Flags, Rules: s.Rules}
	c.Rules.Visibility = 80
	got := New(c, s.Leader, s.Formations, 1)
	if got.Squads[0].Men[Spy] != 0 || got.Squads[0].Men[Barbarian] != 5 {
		t.Fatal("always-visible game did not replace spy")
	}
}
func TestEncounterDelayAndLateZoom(t *testing.T) {
	s := testState()
	s.Squads[1].Position = Point{5, 181}
	s.Options.FightDelay = 60
	s.encounters()
	if len(s.Battles) != 1 {
		t.Fatal("missing encounter")
	}
	b := &s.Battles[0]
	for i := 0; i < 59; i++ {
		s.Step()
	}
	if b.Started != 0 {
		t.Fatal("fought before delay")
	}
	s.Step()
	if b.Started == 0 {
		t.Fatal("did not start battle")
	}
	if _, err := s.Zoom(0); err == nil {
		t.Fatal("allowed late zoom")
	}
}
func TestUnopposedFlagCaptureAndMissingFlag(t *testing.T) {
	s := testState()
	s.Squads[0].Position = s.Flags[1].Position
	s.Squads[1].Position = Point{50, 100}
	s.captureFlags()
	s.checkOutcome()
	if s.Outcome != PlayerWon {
		t.Fatalf("capture did not win: %v", s.Reason)
	}
	c := Campaign{World: s.World, Squads: s.Squads, Flags: []Flag{{Point{20, 20}, Enemy}}, Rules: s.Rules}
	a, b := New(c, s.Leader, s.Formations, 4), New(c, s.Leader, s.Formations, 4)
	if len(a.Flags) != 2 || !reflect.DeepEqual(a.Flags, b.Flags) {
		t.Fatal("missing flag not deterministically added")
	}
}
func TestSaveReplay(t *testing.T) {
	s := testState()
	s.Move(0, []Point{{40, 170}, {40, 120}})
	for i := 0; i < 900; i++ {
		s.Step()
	}
	var b bytes.Buffer
	if err := s.Encode(&b); err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(&b)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2000; i++ {
		s.Step()
		loaded.Step()
	}
	if !reflect.DeepEqual(s, loaded) {
		t.Fatal("save/load changed deterministic continuation")
	}
	loaded.Squads[0].BattleID = 999999
	b.Reset()
	json.NewEncoder(&b).Encode(loaded)
	if _, err := Decode(&b); err == nil {
		t.Fatal("accepted corrupted battle pointer")
	}
}
func TestManualBattleRetreatAndSave(t *testing.T) {
	s := testState()
	prepareTacticalState(t, s)
	s.Squads[1].Position = Point{5, 181}
	s.encounters()
	b, err := s.Zoom(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CommandBattle(b.ID, -1, Attack); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		s.Step()
	}
	var buf bytes.Buffer
	if err := s.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.Retreat(b.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		loaded.Step()
	}
	if loaded.Battle(b.ID).Done == false {
		t.Fatal("retreat never completed")
	}
	if loaded.Squads[0].BattleID != 0 || len(loaded.Squads[0].Route) != 0 {
		t.Fatal("retreat left battle/route active")
	}
}

func prepareTacticalState(t *testing.T, s *State) {
	t.Helper()
	files := originalTestFiles(t)
	s.Tactical = TacticalResources{Primary: files["O6"], Auxiliary: files["O7"]}
	for layout, name := range [...]string{"P0", "P4", "P5", "P2", "P1", "PA", "P6", "P8", "P3", "P7", "P9", "PB", "PC", "PD"} {
		s.Tactical.Backgrounds[layout] = files[name]
	}
	s.Formations = originalTestFormations(files)
}

func TestTacticalSaveReplaysAnimationAndRandomState(t *testing.T) {
	s := testState()
	prepareTacticalState(t, s)
	s.Squads[1].Position = Point{5, 181}
	s.encounters()
	b, err := s.Zoom(0)
	if err != nil {
		t.Fatal(err)
	}
	s.CommandBattle(b.ID, -1, Attack)
	for i := 0; i < 60; i++ {
		s.Step()
	}
	s.CommandBattle(b.ID, -1, Attack)
	for i := 0; i < 80; i++ {
		s.Step()
	}
	var buf bytes.Buffer
	if err := s.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1200; i++ {
		if i == 180 && !s.Battle(b.ID).Done {
			s.Retreat(b.ID)
			loaded.Retreat(b.ID)
		}
		s.Step()
		loaded.Step()
	}
	if s.Fault != "" || !s.Battle(b.ID).Done {
		t.Fatalf("tactical continuation failed: %s", s.Fault)
	}
	if !reflect.DeepEqual(s, loaded) {
		t.Fatal("save/load changed original combat continuation")
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSurrenderOfferPausesAndCanBeDeclined(t *testing.T) {
	s := testState()
	s.Squads[2].Men[Knight] = 14
	s.Squads[1].Men = [4]int{1, 0, 0, 0}
	s.Leader.Parameters[7] = 2
	s.Tick = 100
	s.reassessArmies(-1, -1)
	if !s.SurrenderOffered {
		t.Fatal("overwhelmed enemy did not offer surrender")
	}
	tick := s.Tick
	s.Step()
	if s.Tick != tick {
		t.Fatal("surrender prompt did not pause simulation")
	}
	s.DeclineSurrender()
	s.Step()
	if s.Tick == tick || s.SurrenderOffered || s.Leader.Parameters[7] != 500 || s.Leader.Parameters[12] != -100 {
		t.Fatal("declining surrender did not resume war with original parameter changes")
	}
	s.SurrenderOffered = true
	s.AcceptSurrender()
	if s.Outcome != PlayerWon {
		t.Fatal("surrender not accepted")
	}
}
