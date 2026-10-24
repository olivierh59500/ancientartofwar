package original

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image/png"
	"os"
	"reflect"
	"testing"

	"ancientartofwar/internal/game"
)

func library(t *testing.T) *Library {
	t.Helper()
	b, err := os.ReadFile("../../assets/original/disk1.img")
	if err != nil {
		t.Fatal(err)
	}
	l, err := Load(b)
	if err != nil {
		t.Fatal(err)
	}
	return l
}
func TestOriginalCampaigns(t *testing.T) {
	l := library(t)
	counts := [][2]int{{70, 196}, {130, 130}, {2, 2}, {48, 140}, {1, 100}, {86, 168}, {47, 101}, {99, 154}, {67, 91}, {72, 81}, {92, 101}}
	if len(l.Campaigns) != 11 {
		t.Fatal("missing campaigns")
	}
	for i, c := range l.Campaigns {
		var total [2]int
		for _, q := range c.Squads {
			total[q.Side] += q.Count()
		}
		if total != counts[i] {
			t.Errorf("%s totals %v want %v", c.ID, total, counts[i])
		}
		s := game.New(c, l.Leaders[c.Opponent], l.Formations, 1)
		if err := s.Validate(); err != nil {
			t.Errorf("%s state: %v", c.ID, err)
		}
	}
	if l.Leaders[5].Parameters != [19]int{5, 90, 90, 65, -40, 40, 2, 4, 20, 45, 35, 2, -10, 65, -50, 4, 2, 2, -120} {
		t.Fatal("leader row layout changed")
	}
	if l.Formations[1].X[0] != 74 || l.Formations[1].Y[0] != 46 || l.Formations[1].Kind[0] != game.Archer {
		t.Fatal("formation layout changed")
	}
}
func TestMapPixels(t *testing.T) {
	l := library(t)
	var b bytes.Buffer
	if err := png.Encode(&b, MapImage(&l.Campaigns[0].World)); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(b.Bytes())); got != "f74bb67343e6a3adff054c4e6964ba26f6028dc15ae2c88ad86df6a5764fa490" {
		t.Fatalf("original map changed: %s", got)
	}
}
func TestNativeCampaignRoundTrip(t *testing.T) {
	l := library(t)
	for _, c := range l.Campaigns {
		var buf bytes.Buffer
		if err := SaveCampaign(&buf, c); err != nil {
			t.Fatal(err)
		}
		got, err := l.LoadCampaign(&buf, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(c.Squads, got.Squads) || c.World != got.World || c.Rules != got.Rules || !reflect.DeepEqual(c.Flags, got.Flags) || c.Title != got.Title {
			t.Fatalf("%s campaign changed on round trip", c.ID)
		}
	}
}
func TestRejectInvalidCampaign(t *testing.T) {
	l := library(t)
	b := append([]byte(nil), l.Files["T0"]...)
	b[840] = 73
	if _, err := l.DecodeCampaign("bad", "Bad", b); err == nil {
		t.Fatal("accepted invalid tile")
	}
	if _, err := l.DecodeCampaign("bad", "Bad", b[:1400]); err == nil {
		t.Fatal("accepted truncated campaign")
	}
}

func TestOriginalArtworkAndSprites(t *testing.T) {
	l := library(t)
	for _, name := range []string{"PE", "PF", "PG", "PH", "PI", "PJ", "PK", "PM", "PN", "PO"} {
		im, err := l.Picture(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if im.Bounds().Dx() != 320 || im.Bounds().Dy() != 200 {
			t.Errorf("%s dimensions: %v", name, im.Bounds())
		}
	}
	for _, offset := range []int{0, 0x74, 0x158, 0x204, 0x408, 0x524, 0xaf4, 0xb68, 0xc14, 0xc88, 0xec8, 0xf74, 0x10cc, 0x11d0, 0x166c, 0x16e8, 0x1764, 0x181c} {
		if _, err := Sprite(l.Files["O6"], offset, true); err != nil {
			t.Errorf("frame %x: %v", offset, err)
		}
	}
	if _, err := Sprite([]byte{255, 255, 255, 255}, 0, true); err == nil {
		t.Fatal("accepted impossible sprite dimensions")
	}
	l.Files["bad"] = []byte{255, 0, 255, 0}
	if _, err := l.Picture("bad"); err == nil {
		t.Fatal("accepted truncated picture")
	}
}
