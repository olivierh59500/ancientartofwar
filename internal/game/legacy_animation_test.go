package game

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"

	"ancientartofwar/internal/disk"
)

func tacticalFiles(t *testing.T) ([]byte, []byte) {
	files := originalTestFiles(t)
	return files["O6"], files["O7"]
}

func originalTestFiles(t *testing.T) map[string][]byte {
	t.Helper()
	image, err := os.ReadFile("../../assets/original/disk1.img")
	if err != nil {
		t.Fatal(err)
	}
	files, err := disk.Read(image)
	if err != nil {
		t.Fatal(err)
	}
	decoded := make(map[string][]byte)
	for _, file := range files {
		decoded[file.Name] = file.Data
	}
	if decoded["O6"] == nil || decoded["O7"] == nil {
		t.Fatal("original tactical data missing")
	}
	return decoded
}

func TestAnimationMatchesOriginalMachineCode(t *testing.T) {
	f, err := os.Open("testdata/poses-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var fixture struct {
		Routine        string
		Initialization []struct {
			Layout          int
			Primary         []byte
			AuxiliaryPrefix []byte `json:"auxiliary_prefix"`
			Frames          []int
		}
		Cases []struct {
			Layout, Retreat int
			Actor           legacyPose
			Expected        struct {
				X, Y, State, Program, Sprite, Draw int
				Released                           bool
				RandomCalls                        int `json:"random_calls"`
			}
		}
	}
	if err := json.NewDecoder(reader).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Routine != "overlay6:2fe0+5eca" || len(fixture.Initialization) != 14 || len(fixture.Cases) < 512 {
		t.Fatal("unexpected original tactical fixture provenance")
	}
	primary, aux := tacticalFiles(t)
	data := make([]*legacyAnimationData, 14)
	frames := make([][]int, 14)
	for _, c := range fixture.Initialization {
		d, err := newLegacyAnimation(primary, aux, c.Layout)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(d.Primary, c.Primary) {
			for i, v := range d.Primary {
				if v != c.Primary[i] {
					t.Fatalf("layout %d primary initialization differs at %04x: Go %02x original %02x", c.Layout, i, v, c.Primary[i])
				}
			}
		}
		if !bytes.Equal(d.Auxiliary[:0x314], c.AuxiliaryPrefix) {
			for i, v := range d.Auxiliary[:0x314] {
				if v != c.AuxiliaryPrefix[i] {
					t.Fatalf("layout %d auxiliary initialization differs at %04x: Go %02x original %02x", c.Layout, i, v, c.AuxiliaryPrefix[i])
				}
			}
		}
		data[c.Layout] = d
		frames[c.Layout] = c.Frames
	}
	for i, c := range fixture.Cases {
		a := c.Actor
		a.Frame = -1
		if err := data[c.Layout].stepPose(&a, c.Layout, c.Retreat); err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		draw := -1
		if a.Frame >= 0 {
			draw = frames[c.Layout][a.Frame]
		}
		e := c.Expected
		if a.X != e.X || a.Y != e.Y || a.State != e.State || a.Program != e.Program || a.Sprite != e.Sprite || draw != e.Draw || a.Released != e.Released || e.RandomCalls != 0 {
			t.Fatalf("case %d layout %d retreat %d: actor %+v, draw %d, original %+v", i, c.Layout, c.Retreat, a, draw, e)
		}
	}
}
