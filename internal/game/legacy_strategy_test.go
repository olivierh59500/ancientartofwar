package game

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

type objectiveCase struct {
	Leader, Selected int
	Seed             uint64
	Parameters       [19]int
	Tiles            [200]byte
	Influence        [200]int
	Rules            [9]int
	Brain            struct {
		HomeFlag   int `json:"home_flag"`
		HomeCell   int `json:"home_cell"`
		TargetFlag int `json:"target_flag"`
		TargetCell int `json:"target_cell"`
		Alert      bool
	}
	Squads []struct {
		Cell                    int
		Side                    Side
		Men                     [4]int
		Condition, Food, Pace   int
		Moving, Engaged, Active bool
		Target                  int
	}
	Flags []struct {
		Cell int
		Side Side
	}
	Expected struct {
		Planned     bool
		PlanGoal    int `json:"plan_goal"`
		SquadTarget int `json:"squad_target"`
		Pace        int
		Active      bool
		TargetCell  int `json:"target_cell"`
		TargetFlag  int `json:"target_flag"`
		Alert       bool
		RandomState uint64 `json:"random_state"`
		RandomCalls int    `json:"random_calls"`
	}
}

func TestObjectivesMatchOriginalMachineCode(t *testing.T) {
	f, err := os.Open("testdata/ai-original.json.gz")
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
		Cases   []objectiveCase
	}
	if err := json.NewDecoder(r).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Routine != "overlay5:79fc+7b27" || len(fixture.Cases) < 512 {
		t.Fatal("unexpected original AI fixture provenance")
	}
	for i, c := range fixture.Cases {
		s := originalObjectiveState(c)
		planned := s.originalObjective(c.Selected)
		q := s.Squads[c.Selected]
		e := c.Expected
		if planned != e.Planned || q.AITarget != e.SquadTarget || q.Pace != e.Pace || q.AIActive != e.Active || s.Brain.TargetCell != e.TargetCell || s.Brain.TargetFlag != e.TargetFlag || s.Brain.Alert != e.Alert || s.Random != e.RandomState {
			t.Fatalf("case %d leader %d: planned %t/%t squad target %d/%d pace %d/%d active %t/%t; brain %+v vs cell %d flag %d alert %t; random %x/%x (%d original calls)", i, c.Leader, planned, e.Planned, q.AITarget, e.SquadTarget, q.Pace, e.Pace, q.AIActive, e.Active, s.Brain, e.TargetCell, e.TargetFlag, e.Alert, s.Random, e.RandomState, e.RandomCalls)
		}
	}
}

func originalObjectiveState(c objectiveCase) *State {
	s := &State{Random: c.Seed, Leader: Leader{Parameters: c.Parameters}, LeaderIndex: c.Leader, Influence: c.Influence, World: World{Tiles: c.Tiles}}
	s.Rules = Rules{VillageFood: c.Rules[0] != 0, FortFood: c.Rules[1] != 0, SupplyRange: c.Rules[2], Recruitment: c.Rules[3], Fitness: c.Rules[4], Visibility: c.Rules[5], WaterSpeed: c.Rules[6], MountainSpeed: c.Rules[7], ForestSpeed: c.Rules[8]}
	s.Brain = StrategyBrain{Initialized: true, HomeFlag: c.Brain.HomeFlag, HomeCell: c.Brain.HomeCell, TargetFlag: c.Brain.TargetFlag, TargetCell: c.Brain.TargetCell, Alert: c.Brain.Alert}
	for id, q := range c.Squads {
		s.Squads[id] = Squad{ID: id, Side: q.Side, Position: cellCenter(q.Cell), Men: q.Men, Pace: q.Pace, Food: q.Food, Condition: q.Condition, AITarget: q.Target, AIActive: q.Active}
		if q.Moving {
			s.Squads[id].Route = []Point{cellCenter(q.Cell)}
		}
		if q.Engaged {
			s.Squads[id].BattleID = 1
		}
	}
	for _, f := range c.Flags {
		s.Flags = append(s.Flags, Flag{Position: cellCenter(f.Cell), Side: f.Side})
	}
	return s
}
