package game

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

type arenaExpectation struct {
	Actors                        [28]legacyActor
	Projectiles                   [10]legacyProjectile
	Returned                      bool
	Done                          bool
	Intro, Countdown, Retreat     int
	Started                       bool
	Orders, Budgets               [3]int
	Moved                         [3]bool
	Skill, Conditions, Formations [2]int
	Men                           [2][4]int
	Positions                     [2]Point
	Casualties, Captured          [2]int
	Objective                     int
	CurrentTarget                 int     `json:"current_target"`
	ClearRoutes                   [2]bool `json:"clear_routes"`
	ResetEnemyAI                  bool    `json:"reset_enemy_ai"`
	AimCalls                      int     `json:"aim_calls"`
	ReleaseCalls                  int     `json:"release_calls"`
	RandomState                   uint64  `json:"random_state"`
	RandomCalls                   int     `json:"random_calls"`
}

func TestActorControllerMatchesOriginalMachineCode(t *testing.T) {
	f, err := os.Open("testdata/arena-original.json.gz")
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
			Layout, Leader, Intro, Countdown, Retreat, Aggression, Objective int
			Interactive, Demo, Started                                       bool
			Seed                                                             uint64
			Orders, Preferred, Budgets                                       [3]int
			Skill, Conditions, Formations                                    [2]int
			Men                                                              [2][4]int
			Positions                                                        [2]Point
			Actors                                                           [28]legacyActor
			Expected                                                         arenaExpectation
		}
	}
	if err := json.NewDecoder(r).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Routine != "overlay6:53be" || len(fixture.Cases) < 256 {
		t.Fatal("unexpected actor-controller fixture provenance")
	}
	primary, auxiliary := tacticalFiles(t)
	for index, c := range fixture.Cases {
		data, err := newLegacyAnimation(primary, auxiliary, c.Layout)
		if err != nil {
			t.Fatal(err)
		}
		b := &legacyArena{Data: data, Actors: c.Actors, Layout: c.Layout, Leader: c.Leader, Intro: c.Intro, Countdown: c.Countdown, Retreat: c.Retreat, Interactive: c.Interactive, Demo: c.Demo, Started: c.Started, Orders: c.Orders, Preferred: c.Preferred, Budgets: c.Budgets, Skill: c.Skill, Conditions: c.Conditions, Formations: c.Formations, Men: c.Men, Positions: c.Positions, Aggression: c.Aggression, Objective: c.Objective, EnemyCell: c.Positions[Enemy].Cell(), CurrentTarget: -1}
		random := &State{Random: c.Seed}
		aimCalls, releaseCalls := 0, 0
		if err := b.stepActors(random.rand128, func(int) bool { aimCalls++; return false }, func(int) { releaseCalls++ }); err != nil {
			t.Fatalf("case %d: %v", index, err)
		}
		e := c.Expected
		for actor := range b.Actors {
			a, expected := b.Actors[actor], e.Actors[actor]
			if a.Kind != expected.Kind || a.Side != expected.Side || a.X != expected.X || a.Y != expected.Y || a.State != expected.State || a.Action != expected.Action || a.Program != expected.Program || a.Sprite != expected.Sprite || a.Target != expected.Target || a.Shot != expected.Shot {
				t.Fatalf("case %d actor %d: Go %+v, original %+v", index, actor, a, expected)
			}
		}
		if b.Intro != e.Intro || b.Countdown != e.Countdown || b.Retreat != e.Retreat || b.Started != e.Started || b.Orders != e.Orders || b.Budgets != e.Budgets || b.Moved != e.Moved || b.Skill != e.Skill || b.Men != e.Men || b.Conditions != e.Conditions || b.Formations != e.Formations || b.Positions != e.Positions || b.Casualties != e.Casualties || b.Captured != e.Captured || b.CurrentTarget != e.CurrentTarget || b.ClearRoutes != e.ClearRoutes || b.ResetEnemyAI != e.ResetEnemyAI || aimCalls != e.AimCalls || releaseCalls != e.ReleaseCalls || random.Random != e.RandomState {
			t.Fatalf("case %d tactical globals differ: Go intro %d countdown %d retreat %d orders %v budgets %v moved %v skill %v men %v target %d rng %x, original %+v", index, b.Intro, b.Countdown, b.Retreat, b.Orders, b.Budgets, b.Moved, b.Skill, b.Men, b.CurrentTarget, random.Random, e)
		}
		objective := c.Objective
		if b.ClearObjective {
			objective = -1
		}
		if objective != e.Objective {
			t.Fatalf("case %d objective %d, original %d", index, objective, e.Objective)
		}
	}
}
