package game

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestSharedFineRoutesMatchOriginalMachineCode(t *testing.T) {
	f, err := os.Open("testdata/fine-original.json.gz")
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
			From, To            Point
			Pool                [1000]byte
			Cursor, Pace, Limit int
			Expected            struct {
				Pool                          [1000]byte
				Offset, Clock, Status, Cursor int
				Active                        bool
			}
		}
	}
	if err := json.NewDecoder(r).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Routine != "overlay5:4b07+14a3+84e4" || len(fixture.Cases) < 512 {
		t.Fatal("unexpected fine-route fixture provenance")
	}
	for index, c := range fixture.Cases {
		s := &State{RoutePool: c.Pool, RouteCursor: c.Cursor}
		q := &s.Squads[1]
		q.Side, q.Position, q.Pace = Enemy, c.From, c.Pace
		s.originalInstallFine(q, c.To, c.Limit)
		status := 0
		if q.Moving() {
			status = q.Pace
		}
		e := c.Expected
		if s.RoutePool != e.Pool {
			for at, value := range s.RoutePool {
				if value != e.Pool[at] {
					t.Fatalf("case %d pool at %d: Go %d, original %d", index, at, value, e.Pool[at])
				}
			}
		}
		if q.FineOffset != e.Offset || q.FineActive != e.Active || q.MoveClock != e.Clock || status != e.Status || s.RouteCursor != e.Cursor {
			t.Fatalf("case %d: Go offset %d active %v clock %d status %d cursor %d; original %+v", index, q.FineOffset, q.FineActive, q.MoveClock, status, s.RouteCursor, e)
		}
	}
}
