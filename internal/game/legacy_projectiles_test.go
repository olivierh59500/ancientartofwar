package game

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestProjectilesMatchOriginalMachineCode(t *testing.T) {
	checkTacticalFixture(t, "testdata/projectiles-original.json.gz", "overlay6:77b7+7e00+80db+7eb1+53be")
}

func TestCompleteFrameMatchesOriginalMachineCode(t *testing.T) {
	checkTacticalFixture(t, "testdata/frames-original.json.gz", "overlay6:4105..4361")
}

func checkTacticalFixture(t *testing.T, path, routine string) {
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
			Mode                                                             string
			Layout, Leader, Intro, Countdown, Retreat, Aggression, Objective int
			Selected                                                         int
			Interactive, Demo, Started                                       bool
			Seed                                                             uint64
			Orders, Preferred, Budgets                                       [3]int
			Skill, Conditions, Formations                                    [2]int
			Men                                                              [2][4]int
			Positions                                                        [2]Point
			Actors                                                           [28]legacyActor
			Projectiles                                                      [10]legacyProjectile
			Command                                                          legacyBattleCommand
			Expected                                                         arenaExpectation
		}
	}
	if err := json.NewDecoder(r).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Routine != routine || len(fixture.Cases) < 1024 {
		t.Fatal("unexpected projectile fixture provenance")
	}
	primary, auxiliary := tacticalFiles(t)
	for index, c := range fixture.Cases {
		data, err := newLegacyAnimation(primary, auxiliary, c.Layout)
		if err != nil {
			t.Fatal(err)
		}
		b := &legacyArena{Data: data, Actors: c.Actors, Projectiles: c.Projectiles, Layout: c.Layout, Leader: c.Leader, Intro: c.Intro, Countdown: c.Countdown, Retreat: c.Retreat, Interactive: c.Interactive, Demo: c.Demo, Started: c.Started, Orders: c.Orders, Preferred: c.Preferred, Budgets: c.Budgets, Skill: c.Skill, Conditions: c.Conditions, Formations: c.Formations, Men: c.Men, Positions: c.Positions, Aggression: c.Aggression, Objective: c.Objective, EnemyCell: c.Positions[Enemy].Cell(), CurrentTarget: -1}
		b.Pending = c.Command
		random := &State{Random: c.Seed}
		returned := false
		switch c.Mode {
		case "aim":
			returned = b.aimShot(c.Selected, random.rand128)
		case "release":
			b.releaseShot(c.Selected)
		case "flight":
			b.advanceProjectiles()
		case "collisions":
			b.collideProjectiles(random.rand128)
		case "combat":
			if err := b.stepActors(random.rand128, func(i int) bool { return b.aimShot(i, random.rand128) }, b.releaseShot); err != nil {
				t.Fatalf("case %d: %v", index, err)
			}
			b.advanceProjectiles()
			b.collideProjectiles(random.rand128)
		case "frame":
			if err := b.stepFrame(random.rand128); err != nil {
				t.Fatalf("case %d: %v", index, err)
			}
		default:
			t.Fatalf("unknown oracle phase %q", c.Mode)
		}
		actors := b.Actors
		for i := range actors {
			actors[i].Frame, actors[i].Released, actors[i].Escaped = 0, false, false
		}
		objective := c.Objective
		if b.ClearObjective {
			objective = -1
		}
		actual := arenaExpectation{Actors: actors, Projectiles: b.Projectiles, Returned: returned, Done: b.Done, Intro: b.Intro, Countdown: b.Countdown, Retreat: b.Retreat, Started: b.Started, Orders: b.Orders, Budgets: b.Budgets, Moved: b.Moved, Skill: b.Skill, Conditions: b.Conditions, Formations: b.Formations, Men: b.Men, Positions: b.Positions, Casualties: b.Casualties, Captured: b.Captured, Objective: objective, CurrentTarget: b.CurrentTarget, ClearRoutes: b.ClearRoutes, ResetEnemyAI: b.ResetEnemyAI, RandomState: random.Random, RandomCalls: c.Expected.RandomCalls}
		if !reflect.DeepEqual(actual, c.Expected) {
			for i, actor := range actors {
				if actor != c.Expected.Actors[i] {
					t.Errorf("case %d (%s) actor %d: Go %+v, original %+v", index, c.Mode, i, actor, c.Expected.Actors[i])
				}
			}
			if b.Projectiles != c.Expected.Projectiles {
				t.Errorf("case %d (%s) projectiles: Go %+v, original %+v", index, c.Mode, b.Projectiles, c.Expected.Projectiles)
			}
			t.Fatalf("case %d (%s) globals: Go %+v, original %+v", index, c.Mode, actual, c.Expected)
		}
	}
}
