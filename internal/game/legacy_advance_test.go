package game

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type advanceSquad struct {
	Men                              [4]int
	Pace, Status, Clock, Target      int
	Active, CoarseActive, FineActive bool
	CoarseOffset, FineOffset         int
}

func TestAdvancePursuitAndJoinMatchOriginalMachineCode(t *testing.T) {
	checkStrategicFixture(t, "testdata/advance-original.json.gz", "overlay5:7d57", false)
}

func TestCommanderSequencesMatchOriginalMachineCode(t *testing.T) {
	checkStrategicFixture(t, "testdata/commander-original.json.gz", "overlay5:790e+79c3", true)
}

func TestFreshCommandersMatchOriginalMachineCode(t *testing.T) {
	checkStrategicFixture(t, "testdata/initial-ai-original.json.gz", "overlay1:garrison-cap+overlay5:8b0c+346b+815c+3a3b+5x790e", true)
}

func checkStrategicFixture(t *testing.T, path, routine string, sequence bool) {
	t.Helper()
	f, err := os.Open(path)
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
		Cases   []struct {
			objectiveCase
			CoarsePool   [200]byte  `json:"coarse_pool"`
			FinePool     [1000]byte `json:"fine_pool"`
			CoarseOffset int        `json:"coarse_offset"`
			FineCursor   int        `json:"fine_cursor"`
			AICursor     int        `json:"ai_cursor"`
			Iterations   int
			Expected     struct {
				Squads     [20]advanceSquad
				CoarsePool [200]byte  `json:"coarse_pool"`
				FinePool   [1000]byte `json:"fine_pool"`
				TargetCell int        `json:"target_cell"`
				TargetFlag int        `json:"target_flag"`
				Alert      bool
				AICursor   int    `json:"ai_cursor"`
				Random     uint64 `json:"random_state"`
				HomeFlag   int    `json:"home_flag"`
				HomeCell   int    `json:"home_cell"`
				Influence  [200]int
				Parameters [19]int
			}
		}
	}
	if err := json.NewDecoder(r).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Routine != routine || len(fixture.Cases) < 512 {
		t.Fatal("unexpected AI advance fixture provenance")
	}
	for index, c := range fixture.Cases {
		s := originalObjectiveState(c.objectiveCase)
		initialize := fixture.Routine == "overlay1:garrison-cap+overlay5:8b0c+346b+815c+3a3b+5x790e"
		s.Brain.PathPool, s.RoutePool, s.RouteCursor = c.CoarsePool, c.FinePool, c.FineCursor
		q := &s.Squads[c.Selected]
		q.AIPathActive, q.AIPathOffset = true, c.CoarseOffset
		for i := range s.Squads {
			if !s.Squads[i].Alive() {
				s.Squads[i].Route = nil
			}
		}
		if initialize {
			campaign := Campaign{World: s.World, Squads: s.Squads, Flags: s.Flags, Rules: s.Rules, Opponent: s.LeaderIndex}
			s = New(campaign, s.Leader, [10]Formation{}, c.Seed)
		} else if sequence {
			s.AICursor = c.AICursor
			for n := 0; n < c.Iterations; n++ {
				s.think()
			}
		} else {
			s.originalAdvance(q)
		}
		var squads [20]advanceSquad
		for i, q := range s.Squads {
			status := 0
			if q.Moving() {
				status = q.Pace
			} else if !q.Alive() {
				status = 9999
			}
			squads[i] = advanceSquad{Men: q.Men, Pace: q.Pace, Status: status, Clock: q.MoveClock, Target: q.AITarget, Active: q.AIActive, CoarseActive: q.AIPathActive, CoarseOffset: q.AIPathOffset, FineActive: q.FineActive, FineOffset: q.FineOffset}
		}
		e := c.Expected
		if initialize && (s.Brain.HomeFlag != e.HomeFlag || s.Brain.HomeCell != e.HomeCell || s.Influence != e.Influence || s.Leader.Parameters != e.Parameters) {
			t.Fatalf("case %d initial campaign: Go home %d/%d parameters %v; original home %d/%d parameters %v", index, s.Brain.HomeFlag, s.Brain.HomeCell, s.Leader.Parameters, e.HomeFlag, e.HomeCell, e.Parameters)
		}
		if sequence && (s.Brain.TargetFlag != e.TargetFlag || s.Brain.Alert != e.Alert || s.AICursor != e.AICursor) {
			t.Fatalf("case %d commander: Go cursor %d target flag %d alert %t, original cursor %d target flag %d alert %t", index, s.AICursor, s.Brain.TargetFlag, s.Brain.Alert, e.AICursor, e.TargetFlag, e.Alert)
		}
		if !reflect.DeepEqual(squads, e.Squads) {
			for i, got := range squads {
				if got != e.Squads[i] {
					t.Errorf("case %d squad %d: Go %+v, original %+v", index, i, got, e.Squads[i])
				}
			}
			t.FailNow()
		}
		if s.Brain.PathPool != e.CoarsePool || s.RoutePool != e.FinePool || s.Brain.TargetCell != e.TargetCell || s.Random != e.Random {
			for at, value := range s.RoutePool {
				if value != e.FinePool[at] {
					t.Errorf("case %d fine pool at %d: Go %d, original %d", index, at, value, e.FinePool[at])
					break
				}
			}
			t.Fatalf("case %d pool/objective/random state differs: target %d/%d random %x/%x", index, s.Brain.TargetCell, e.TargetCell, s.Random, e.Random)
		}
	}
}
