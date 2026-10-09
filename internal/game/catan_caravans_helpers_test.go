package game

import (
	"fmt"
	"testing"
)

func TestCatanCaravansHelpersNatural(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := NewCatanCaravansWorld(n)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.EnableCatanCaravansSeaHelpers(true); err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				for step := 0; step < 18000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(e)
					}
					if e = s.Apply(p, a); e != nil {
						t.Fatal(step, s.Phase, e)
					}
					if step%137 == 0 {
						b := clone(*s)
						s = &b
						if e = s.validateCaravans(); e != nil {
							t.Fatal(e)
						}
					}
				}
				if !s.Finished {
					t.Fatal("unfinished")
				}
				t.Log("round", s.Round)
			})
		}
	}
}

func TestCatanCaravanWaterSourceHelpers(t *testing.T) {
	for _, id := range []int{10, 11} {
		s, err := NewCatanCaravansIslandsSeafarers(3)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.EnableCatanCaravansSeaHelpers(true); err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		g.SetupStep = g.SetupLimit()
		g.TurnSerial = 10
		s.Turn = 0
		s.Phase = "catan_turn"
		g.Players[0].Helper = &CatanHelperSeat{ID: id, AcquiredTurn: 1}
		g.Robber = g.Caravans.Map.WateringHoles[0]
		before := g.Players[0].Resources[1]
		if err = s.catanHelperAction(0, Action{Type: "catan_helper", Color: 1}); err != nil {
			t.Fatal(err)
		}
		if g.Players[0].Resources[1] != before+1 || s.Phase != "catan_helper" {
			t.Fatal("resource or exchange")
		}
		if id == 10 && g.Robber != -1 {
			t.Fatal("no-desert retreat")
		}
	}
}
