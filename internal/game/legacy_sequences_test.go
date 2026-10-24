package game

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func nativeTacticalSnapshot(b *legacyArena, random uint64) arenaExpectation {
	actors := b.Actors
	for i := range actors {
		actors[i].Frame, actors[i].Released, actors[i].Escaped = 0, false, false
	}
	objective := b.Objective
	if b.ClearObjective {
		objective = -1
	}
	return arenaExpectation{Actors: actors, Projectiles: b.Projectiles, Done: b.Done, Intro: b.Intro, Countdown: b.Countdown, Retreat: b.Retreat, Started: b.Started, Orders: b.Orders, Budgets: b.Budgets, Moved: b.Moved, Skill: b.Skill, Conditions: b.Conditions, Formations: b.Formations, Men: b.Men, Positions: b.Positions, Casualties: b.Casualties, Captured: b.Captured, Objective: objective, CurrentTarget: b.CurrentTarget, ClearRoutes: b.ClearRoutes, ResetEnemyAI: b.ResetEnemyAI, RandomState: random}
}

func TestCompleteBattlesAndPixelsMatchOriginalMachineCode(t *testing.T) {
	f, err := os.Open("testdata/battles-original.json.gz")
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
			Leader, Selected, Objective, Aggression, Layout, Frames int
			Seed                                                    uint64
			Tiles                                                   [2]byte
			Formations, Conditions                                  [2]int
			Men                                                     [2][4]int
			Commands                                                []struct {
				Frame int
				legacyBattleCommand
			}
			Checkpoints []struct {
				Frame        int
				Expected     arenaExpectation
				Draws        []legacyDraw
				Palette      [4]uint8
				PixelsSHA256 string `json:"pixels_sha256"`
			}
		}
	}
	if err := json.NewDecoder(r).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Routine != "overlay6:complete initialized tactical battles" || len(fixture.Cases) < 56 {
		t.Fatal("unexpected complete-battle fixture provenance")
	}
	files := originalTestFiles(t)
	formations := originalTestFormations(files)
	resources := TacticalResources{Primary: files["O6"], Auxiliary: files["O7"]}
	for layout, name := range [...]string{"P0", "P4", "P5", "P2", "P1", "PA", "P6", "P8", "P3", "P7", "P9", "PB", "PC", "PD"} {
		resources.Backgrounds[layout] = files[name]
	}
	layouts, totalFrames, totalCheckpoints := make(map[int]bool), 0, 0
	for index, c := range fixture.Cases {
		s := &State{Random: c.Seed, LeaderIndex: c.Leader, Formations: formations, Tactical: resources}
		s.Brain.TargetCell, s.Leader.Parameters[3] = c.Objective, c.Aggression
		s.Squads[0] = Squad{Side: Player, Position: Point{3, 0}, Men: c.Men[0], Formation: c.Formations[0], Condition: c.Conditions[0]}
		s.Squads[1] = Squad{Side: Enemy, Position: Point{75, 190}, Men: c.Men[1], Formation: c.Formations[1], Condition: c.Conditions[1]}
		for i := range s.World.Tiles {
			s.World.Tiles[i] = 1
		}
		s.World.Tiles[0], s.World.Tiles[199] = c.Tiles[0], c.Tiles[1]
		s.Battles = []Battle{{ID: 0, Squads: [2]int{0, 1}, Manual: true}}
		a, err := s.newOriginalArena(&s.Battles[0])
		if err != nil {
			t.Fatalf("battle %d: %v", index, err)
		}
		s.Battles[0].Arena = a
		if err := a.stepActors(s.rand128, func(i int) bool { return a.aimShot(i, s.rand128) }, a.releaseShot); err != nil {
			t.Fatal(err)
		}
		a.choosePalette(s.rand128)
		a.Interactive = true
		layouts[a.Layout] = true
		checkpoint := 0
		for frame := 0; frame <= c.Frames; frame++ {
			if frame != 0 {
				for _, command := range c.Commands {
					if command.Frame == frame {
						a.Pending = command.legacyBattleCommand
					}
				}
				if err := a.stepFrame(s.rand128); err != nil {
					t.Fatalf("battle %d frame %d: %v", index, frame, err)
				}
				totalFrames++
			}
			if checkpoint >= len(c.Checkpoints) || c.Checkpoints[checkpoint].Frame != frame {
				continue
			}
			e := c.Checkpoints[checkpoint]
			checkpoint++
			totalCheckpoints++
			actual := nativeTacticalSnapshot(a, s.Random)
			e.Expected.RandomCalls = 0 // final RNG state is the compared observable
			if !reflect.DeepEqual(actual, e.Expected) {
				for actor, got := range actual.Actors {
					if got != e.Expected.Actors[actor] {
						t.Errorf("battle %d frame %d actor %d: Go %+v, original %+v", index, frame, actor, got, e.Expected.Actors[actor])
					}
				}
				actual.Actors, e.Expected.Actors = [28]legacyActor{}, [28]legacyActor{}
				t.Fatalf("battle %d frame %d state: Go %+v, original %+v", index, frame, actual, e.Expected)
			}
			if !reflect.DeepEqual(a.Draws, e.Draws) || a.Palette != e.Palette {
				t.Fatalf("battle %d frame %d rendering commands: Go %+v palette %v; original %+v palette %v", index, frame, a.Draws, a.Palette, e.Draws, e.Palette)
			}
			var pixels [16000]byte
			if _, err := s.BattlePixels(0, pixels[:]); err != nil {
				t.Fatal(err)
			}
			if hash := fmt.Sprintf("%x", sha256.Sum256(pixels[:])); hash != e.PixelsSHA256 {
				t.Fatalf("battle %d frame %d CGA pixels: Go %s, original %s", index, frame, hash, e.PixelsSHA256)
			}
		}
		if !a.Done || a.Layout != c.Layout || checkpoint != len(c.Checkpoints) {
			t.Fatalf("battle %d did not reach the original ending", index)
		}
	}
	if len(layouts) != 14 {
		t.Fatalf("whole-battle coverage includes %d/14 layouts", len(layouts))
	}
	t.Logf("%d complete battles, %d frames, %d state/pixel checkpoints, all 14 layouts", len(fixture.Cases), totalFrames, totalCheckpoints)
}
