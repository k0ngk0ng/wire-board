package game

import (
	"fmt"
	"testing"
)

func TestCatanCaravansTribeNatural(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanCaravansTribeSea(n)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				if s.Catan.victoryTarget() != 15 || s.Catan.Robber != s.Catan.Caravans.Map.WateringHoles[0] {
					t.Fatal("setup")
				}
				for step := 0; step < 18000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(step, err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					if step%131 == 0 {
						b := clone(*s)
						s = &b
						if err = s.validateCaravans(); err != nil {
							t.Fatal(err)
						}
						if err = s.validateCatanEventSession(); err != nil {
							t.Fatal(err)
						}
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round)
				}
				t.Log("round", s.Round)
			})
		}
	}
}

func TestCatanCaravansTribeRewards(t *testing.T) {
	for _, reward := range []string{"point", "development", "port"} {
		t.Run(reward, func(t *testing.T) {
			s, err := newCatanCaravansTribeSea(4)
			if err != nil {
				t.Fatal(err)
			}
			g := s.Catan
			s.Turn, s.Phase = 0, "catan_turn"
			g.SetupStep = g.SetupLimit()
			g.Seafarers.Pirate = -1
			edge := g.tribe().Tokens[0]
			card := -1
			if reward == "development" {
				edge, card = g.tribe().Development[0].Edge, g.tribe().Development[0].Card
			}
			if reward == "port" {
				edge = g.tribe().Ports[0].Edge
			}
			riverTribeRewardRoute(t, s, edge)
			helperGrant(s, 0, []int{1, 0, 1, 0, 0})
			helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
			g = s.Catan
			if reward == "point" && g.tribe().Points[0] != 1 {
				t.Fatal("point")
			}
			if card >= 0 && (g.Players[0].Dev[card] != 1 || g.Players[0].NewDev[card] != 1) {
				t.Fatal("card")
			}
			if reward == "port" {
				if s.Phase != "catan_port" {
					t.Fatal("port response")
				}
				a, e := s.BotAction(0)
				if e != nil {
					t.Fatal(e)
				}
				helperApply(t, s, 0, a)
			}
			if err = s.validateCaravans(); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_turn" || s.Catan.Caravans.Pending != nil {
				t.Fatal("ship incorrectly triggered building vote")
			}
		})
	}
}

func TestCatanCaravansTribeWateringHole(t *testing.T) {
	s, err := newCatanCaravansTribeSea(3)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	hole := g.Caravans.Map.WateringHoles[0]
	if !g.tribeLand(hole) {
		t.Fatal("water hole incorrectly excluded from mainland")
	}
	g.Robber = 0
	if !g.robberAllowed(hole) {
		t.Fatal("robber cannot return to water hole")
	}
	for _, v := range g.Tiles[hole].Vertices {
		if !g.landVertex(v) {
			t.Fatal("water hole not buildable")
		}
	}
}
