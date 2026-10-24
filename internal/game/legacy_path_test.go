package game

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestPathMatchesOriginalMachineCode(t *testing.T) {
	f, err := os.Open("testdata/path-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var fixtures struct {
		ExecutableSHA256 string `json:"executable_sha256"`
		Routine          string `json:"routine"`
		Cases            []struct {
			Start, Goal, Risk int
			Costs, Influence  [200]int
			Success           bool
			Directions        []byte
			RandomCalls       int `json:"random_calls"`
			Pool              []byte
			PoolAfter         []byte `json:"pool_after"`
			Slot              int
		}
	}
	if err := json.NewDecoder(r).Decode(&fixtures); err != nil {
		t.Fatal(err)
	}
	if fixtures.ExecutableSHA256 != "e55a202df53ec1d1beba20f0aef75f86a286987cc53fbe4b354a10c50b6c939e" || fixtures.Routine != "overlay5:85ce" || len(fixtures.Cases) < 256 {
		t.Fatal("unexpected reference provenance")
	}
	for i, c := range fixtures.Cases {
		directions, success := legacyPlan(c.Start, c.Goal, c.Costs, c.Influence, c.Risk)
		if len(c.Pool) > 0 {
			var pool [200]byte
			copy(pool[:], c.Pool)
			var path []byte
			path, success = legacySearch(c.Start, c.Goal, c.Costs, c.Influence, c.Risk)
			if success {
				slot, stored := originalStorePath(&pool, path)
				success = stored
				if slot != c.Slot {
					t.Fatalf("case %d allocator slot %d, original %d", i, slot, c.Slot)
				}
				directions = nil
				for p := slot; p < 200 && pool[p] != 99; p++ {
					directions = append(directions, pool[p])
				}
			}
			if !slices.Equal(pool[:], c.PoolAfter) {
				t.Fatalf("case %d shared path pool differs", i)
			}
		}
		if success != c.Success || !slices.Equal(directions, c.Directions) {
			t.Fatalf("case %d %d -> %d: Go (%t %v), original (%t %v)", i, c.Start, c.Goal, success, directions, c.Success, c.Directions)
		}
		if c.RandomCalls != 0 {
			t.Fatal("unexpected random sampling in path routine")
		}
	}
}
