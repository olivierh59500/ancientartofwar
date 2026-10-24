package game

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestCasualtySamplingMatchesOriginalMachineCode(t *testing.T) {
	f, err := os.Open("testdata/casualty-original.json.gz")
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
			Attacker, Defender [4]int
			Self, Terrain      bool
			Side               Side
			Seed               uint64
			Expected           struct {
				Men        [4]int
				Count      int
				Casualties [2]int
				Random     uint64 `json:"random_state"`
			}
		}
	}
	if err := json.NewDecoder(r).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Routine != "overlay5:5442" || len(fixture.Cases) < 1024 {
		t.Fatal("unexpected casualty fixture provenance")
	}
	for i, c := range fixture.Cases {
		s := State{Random: c.Seed}
		a := Squad{Side: c.Side, Men: c.Attacker}
		d := Squad{Side: c.Side, Men: c.Defender}
		defender := &d
		if c.Self {
			defender = &a
		}
		s.casualty(&a, defender, c.Terrain)
		if defender.Men != c.Expected.Men || defender.originalCount() != c.Expected.Count || s.Stats.Casualties != c.Expected.Casualties || s.Random != c.Expected.Random {
			t.Fatalf("case %d: Go men %v, losses %v, RNG %x; original men %v, losses %v, RNG %x", i, defender.Men, s.Stats.Casualties, s.Random, c.Expected.Men, c.Expected.Casualties, c.Expected.Random)
		}
	}
}
