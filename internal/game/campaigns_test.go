package game_test

import (
	"fmt"
	"os"
	"testing"

	"ancientartofwar/internal/game"
	"ancientartofwar/internal/original"
)

func TestAllOriginalCampaignsRun(t *testing.T) {
	b, err := os.ReadFile("../../assets/original/disk1.img")
	if err != nil {
		t.Fatal(err)
	}
	l, err := original.Load(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range l.Campaigns {
		for _, seed := range []uint64{1, 17, 841} {
			t.Run(fmt.Sprintf("%s/%d", c.ID, seed), func(t *testing.T) {
				s := game.New(c, l.Leaders[c.Opponent], l.Formations, seed)
				s.Options.AutoAcceptSurrender = true
				for tick := 0; tick < 36000 && s.Outcome == game.Running; tick++ {
					if tick%240 == 0 {
						s.AutoCommand(game.Player)
					}
					s.Step()
					if tick%600 == 0 {
						if err := s.Validate(); err != nil {
							t.Fatalf("tick %d: %v", tick, err)
						}
					}
				}
				if err := s.Validate(); err != nil {
					t.Fatal(err)
				}
				t.Logf("ticks=%d outcome=%d men=%v battles=%d", s.Tick, s.Outcome, s.Totals(), len(s.Battles))
			})
		}
	}
}

func TestOriginalTacticalBattlesComplete(t *testing.T) {
	b, err := os.ReadFile("../../assets/original/disk1.img")
	if err != nil {
		t.Fatal(err)
	}
	l, err := original.Load(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]game.Kind{{game.Archer, game.Knight}, {game.Knight, game.Barbarian}, {game.Barbarian, game.Archer}} {
		t.Run(pair[0].String()+"/"+pair[1].String(), func(t *testing.T) {
			wins := 0
			for seed := uint64(1); seed <= 8; seed++ {
				c := l.Campaigns[0]
				c.Opponent = 7
				for i := range c.World.Tiles {
					c.World.Tiles[i] = 1
				}
				for i := range c.Squads {
					c.Squads[i].Men = [4]int{}
				}
				for i := 0; i < 2; i++ {
					q := &c.Squads[i]
					q.Side = game.Side(i)
					q.Position = game.Point{X: 12 + i, Y: 120 + i}
					q.Men[pair[i]] = 14
					q.Food = 49
					q.Condition = 49
					q.Formation = 1
				}
				s := game.New(c, l.Leaders[7], l.Formations, seed)
				for i := 0; i < 4; i++ {
					s.Step()
				}
				battle, err := s.Zoom(0)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.CommandBattle(battle.ID, -1, game.Attack); err != nil {
					t.Fatal(err)
				}
				for n := 0; n < 60 && !battle.Done; n++ {
					s.Step()
				}
				if !battle.Done {
					if err := s.CommandBattle(battle.ID, -1, game.Attack); err != nil {
						t.Fatal(err)
					}
				}
				for n := 0; n < 12000 && !battle.Done; n++ {
					s.Step()
				}
				if !battle.Done || s.Fault != "" {
					t.Fatalf("tactical battle stalled at seed %d: %s", seed, s.Fault)
				}
				if s.Squads[0].Alive() {
					wins++
				}
				if err := s.Validate(); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("player survived %d/8 original tactical battles", wins)
		})
	}
}
