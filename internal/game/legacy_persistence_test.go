package game

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestVersionOneSaveMigrationPreservesCampaign(t *testing.T) {
	s := testState()
	s.Squads[1].Position = Point{5, 181}
	s.Options.FightDelay = 60
	s.encounters()
	for n := 0; n < 20; n++ {
		s.Step()
	}
	var old map[string]any
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&old); err != nil {
		t.Fatal(err)
	}
	old["Version"] = 1
	delete(old, "WorldTick")
	for _, item := range old["Battles"].([]any) {
		b := item.(map[string]any)
		delete(b, "AutoPhase")
		delete(b, "AutoChance")
		delete(b, "AutoSide")
	}
	raw, err = json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != StateVersion || loaded.WorldTick != loaded.Tick || loaded.Totals() != s.Totals() || loaded.Random != s.Random || loaded.Battles[0].AutoPhase != s.Battles[0].AutoPhase {
		t.Fatalf("legacy save migration: version %d, world/tick %d/%d, men %v/%v, RNG %x/%x, phase %v/%v, created %d", loaded.Version, loaded.WorldTick, loaded.Tick, loaded.Totals(), s.Totals(), loaded.Random, s.Random, loaded.Battles[0].AutoPhase, s.Battles[0].AutoPhase, s.Battles[0].Created)
	}
}

func TestSaveDuringSurrenderResumesOriginalWorldFrame(t *testing.T) {
	files := originalTestFiles(t)
	cases := readWorldFixtures(t, "surrender-world", "overlay5:310a..3201 with surrender interruptions")
	checked := false
	for _, c := range cases {
		s := c.native(files)
		s.Version = StateVersion
		for n := 0; n < c.Frames && !s.SurrenderOffered; n++ {
			s.Step()
		}
		if !s.SurrenderOffered {
			continue
		}
		if s.WorldResume == 0 {
			t.Fatal("surrender lost its world continuation")
		}
		var saved bytes.Buffer
		if err := s.Encode(&saved); err != nil {
			t.Fatal(err)
		}
		loaded, err := Decode(&saved)
		if err != nil {
			t.Fatal(err)
		}
		tick := s.Tick
		s.DeclineSurrender()
		loaded.DeclineSurrender()
		s.Step()
		loaded.Step()
		if s.Tick != tick || loaded.Tick != tick || s.WorldResume != 0 || !reflect.DeepEqual(s, loaded) {
			t.Fatal("saved surrender changed the interrupted frame continuation")
		}
		checked = true
		break
	}
	if !checked {
		t.Fatal("no original surrender case exercised persistence")
	}
}

func TestSavePreservesOriginalRecruitmentWithSpies(t *testing.T) {
	s := testState()
	q := &s.Squads[0]
	q.Men, q.CountCached, q.CountCache = [4]int{13, 0, 0, 1}, true, 13
	s.World.Tiles[q.Position.Cell()] = 3
	s.Rules.Recruitment, s.Random = 120, 1
	s.recruit()
	if q.Fighters() != 14 || q.Men[Spy] != 1 {
		t.Fatal("lost the original cached-count recruitment behavior")
	}
	var saved bytes.Buffer
	if err := s.Encode(&saved); err != nil {
		t.Fatal(err)
	}
	loaded, err := Decode(&saved)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Squads[0].Men != q.Men {
		t.Fatal("save dropped spies after original fort recruitment")
	}
}
