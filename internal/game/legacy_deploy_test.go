package game

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func originalTestFormations(files map[string][]byte) (formations [10]Formation) {
	f := files["F"]
	for n := range formations {
		for i := range formations[n].Kind {
			formations[n].Kind[i] = [...]Kind{Knight, Barbarian, Archer}[f[n*14+i]]
			formations[n].X[i] = int(f[140+n*14+i])
			formations[n].Y[i] = int(f[280+n*14+i])
		}
	}
	return
}

func TestDeploymentMatchesOriginalMachineCode(t *testing.T) {
	f, err := os.Open("testdata/deployment-original.json.gz")
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
			Leader, Selected       int
			Seed                   uint64
			Tiles                  [2]byte
			Formations, Conditions [2]int
			Men                    [2][4]int
			Expected               struct {
				Actors                    [28]legacyActor
				Layout                    int
				Formations, Skill         [2]int
				Intro, Countdown, Retreat int
				RandomState               uint64 `json:"random_state"`
			}
		}
	}
	if err := json.NewDecoder(r).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Routine != "overlay6:482a+0a21+2fe0" || len(fixture.Cases) < 512 {
		t.Fatal("unexpected deployment fixture provenance")
	}
	files := originalTestFiles(t)
	formations := originalTestFormations(files)
	for index, c := range fixture.Cases {
		s := &State{Random: c.Seed, LeaderIndex: c.Leader, Formations: formations, Tactical: TacticalResources{Primary: files["O6"], Auxiliary: files["O7"]}}
		s.Squads[0] = Squad{Side: Player, Position: Point{3, 0}, Men: c.Men[0], Formation: c.Formations[0], Condition: c.Conditions[0]}
		s.Squads[1] = Squad{Side: Enemy, Position: Point{75, 190}, Men: c.Men[1], Formation: c.Formations[1], Condition: c.Conditions[1]}
		for i := range s.World.Tiles {
			s.World.Tiles[i] = 1
		}
		s.World.Tiles[0], s.World.Tiles[199] = c.Tiles[0], c.Tiles[1]
		b, err := s.newOriginalArena(&Battle{Squads: [2]int{0, 1}})
		if err != nil {
			t.Fatalf("case %d: %v", index, err)
		}
		if b.Layout != c.Expected.Layout || b.Formations != c.Expected.Formations || b.Skill != c.Expected.Skill || b.Intro != c.Expected.Intro || b.Countdown != c.Expected.Countdown || b.Retreat != c.Expected.Retreat || s.Random != c.Expected.RandomState {
			t.Fatalf("case %d: Go layout %d formations %v skill %v rng %x; original %+v", index, b.Layout, b.Formations, b.Skill, s.Random, c.Expected)
		}
		for i, a := range b.Actors {
			if a != c.Expected.Actors[i] {
				t.Fatalf("case %d actor %d: Go %+v, original %+v", index, i, a, c.Expected.Actors[i])
			}
		}
	}
}
