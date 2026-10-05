package game

import (
	"fmt"
	"slices"
	"testing"
)

func TestCatanTwoDevelopmentBetweenProductionRolls(t *testing.T) {
	// FAQ31 explicitly permits the knight between rolls; the other standard
	// development cards keep their normal before-roll timing and shared limit.
	for kind := 0; kind < 4; kind++ {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			s := twoCoreFixture(t)
			p := s.Turn
			if err := twoCoreRoll(t, s, 1, 1); err != nil {
				t.Fatal(err)
			}
			rollID := s.Catan.RollID
			catanCard(s.Catan, p, kind)
			action := Action{Type: "catan_dev", Card: kind}
			if kind == 2 {
				action.Take = []int{1, 1, 0, 0, 0}
			}
			if kind == 3 {
				action.Color = 0
			}
			helperApply(t, s, p, action)
			responses := 0
			for steps := 0; s.Phase != "catan_roll" && steps < 12; steps++ {
				if s.Phase == "catan_two_build" {
					responses++
				}
				for _, a := range []Action{{Type: "catan_two_trade"}, {Type: "catan_two_robber"}, {Type: "catan_two_knight"}, {Type: "catan_dev", Card: kind}, {Type: "catan_roll"}} {
					helperReject(t, s, p, a)
				}
				twoCoreRestore(t, s)
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				helperApply(t, s, p, a)
			}
			if s.Phase != "catan_roll" || s.Catan.RollID != rollID || !slices.Equal(s.Catan.Two.Rolls, []int{2}) || !s.Catan.PlayedDev {
				t.Fatal("development interrupted/duplicated production", s.Phase, s.Catan.Two.Rolls)
			}
			if kind == 1 && responses != 2 {
				t.Fatal("free roads did not each build neutral", responses)
			}
			catanCard(s.Catan, p, 2)
			helperReject(t, s, p, Action{Type: "catan_dev", Card: 2, Take: []int{1, 1, 0, 0, 0}})
			twoCoreRestore(t, s)
			// Snapshot before production and independently count actual unblocked12
			// terrain. The old2roll must not produce a second time after any card.
			hands := [][]int{slices.Clone(s.Catan.Players[0].Resources), slices.Clone(s.Catan.Players[1].Resources)}
			for _, tile := range s.Catan.Tiles {
				if tile.Number == 12 && tile.ID != s.Catan.Robber && tile.Resource < 5 {
					for _, v := range tile.Vertices {
						x := s.Catan.Vertices[v]
						if x.Owner >= 0 && x.Level > 0 {
							hands[x.Owner][tile.Resource] += x.Level
						}
					}
				}
			}
			if err := twoCoreRoll(t, s, 6, 6); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_turn" || s.Catan.RollID != rollID+1 || !slices.Equal(s.Catan.Two.Rolls, []int{2, 12}) {
				t.Fatal("second roll not resumed")
			}
			for p := range hands {
				if !slices.Equal(hands[p], s.Catan.Players[p].Resources) {
					t.Fatal("duplicate/missing production", kind, p, hands, s.Catan.Players)
				}
			}
			twoCoreRestore(t, s)
		})
	}
}

func TestCatanTwoFirstSevenMustFinishBeforeExtraActions(t *testing.T) {
	s := twoCoreFixture(t)
	p := s.Turn
	for i := range 2 {
		twoTokenHand(s, i, []int{2, 2, 2, 2, 0})
	}
	catanCard(s.Catan, p, 0)
	if err := twoCoreRoll(t, s, 3, 4); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for steps := 0; s.Phase != "catan_roll" && steps < 10; steps++ {
		seen[s.Phase] = true
		if s.catanTwoTokenWindow(p) {
			t.Fatal("window inside production resolution")
		}
		for _, a := range []Action{{Type: "catan_two_trade"}, {Type: "catan_two_robber"}, {Type: "catan_two_knight"}, {Type: "catan_dev", Card: 0}} {
			helperReject(t, s, p, a)
		}
		actor := p
		if s.Phase == "catan_discard" {
			for i, n := range s.Catan.DiscardDue {
				if n > 0 {
					actor = i
					break
				}
			}
		}
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, actor, a)
		twoCoreRestore(t, s)
	}
	if !seen["catan_discard"] || !seen["catan_robber"] || s.Phase != "catan_roll" || !s.catanTwoTokenWindow(p) {
		t.Fatal("first7resolution incomplete", seen, s.Phase)
	}
	before := s.Catan.RollID
	helperApply(t, s, p, Action{Type: "catan_two_trade"})
	twoCoreRestore(t, s)
	helperReject(t, s, p, Action{Type: "catan_dev", Card: 0})
	s.AutoCatanPending()
	if s.Phase != "catan_roll" || s.Catan.RollID != before || !slices.Equal(s.Catan.Two.Rolls, []int{7}) {
		t.Fatal("return lost first seven")
	}
	if err := twoCoreRoll(t, s, 3, 4); err == nil {
		t.Fatal("same7accepted")
	}
	helperApply(t, s, p, Action{Type: "catan_dev", Card: 0})
	for steps := 0; s.Phase != "catan_roll" && steps < 5; steps++ {
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, p, a)
	}
	if s.Phase != "catan_roll" || s.Catan.RollID != before || !s.Catan.Two.Spent || s.Catan.Players[p].Knights != 1 {
		t.Fatal("knight between rolls failed")
	}
	// Exchanging this now-faceup knight does not clear the development limit
	// and is independent of the already-spent trade-token allowance.
	tokens := s.Catan.Two.Tokens[p]
	helperApply(t, s, p, Action{Type: "catan_two_knight"})
	if s.Catan.Two.Tokens[p] != tokens+2 || !s.Catan.PlayedDev || !s.Catan.Two.Spent || s.Catan.Players[p].Knights != 0 {
		t.Fatal("independent knight conversion")
	}
	helperReject(t, s, p, Action{Type: "catan_two_knight"})
	if err := twoCoreRoll(t, s, 6, 6); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_turn" || !slices.Equal(s.Catan.Two.Rolls, []int{7, 12}) {
		t.Fatal("second production missing")
	}
}

func TestCatanTwoRetreatBeforeSecondProduction(t *testing.T) {
	s := twoCoreFixture(t)
	p := s.Turn
	if err := twoCoreRoll(t, s, 1, 1); err != nil {
		t.Fatal(err)
	}
	for _, tile := range s.Catan.Tiles {
		if tile.Resource < 5 {
			s.Catan.Robber = tile.ID
			break
		}
	}
	hands := [][]int{slices.Clone(s.Catan.Players[0].Resources), slices.Clone(s.Catan.Players[1].Resources)}
	tokens, rollID := s.Catan.Two.Tokens[p], s.Catan.RollID
	helperApply(t, s, p, Action{Type: "catan_two_robber"})
	if s.Phase != "catan_roll" || s.Catan.RollID != rollID || s.Catan.Robber != s.Catan.twoDesert() || s.Catan.Two.Tokens[p] != tokens-1 {
		t.Fatal("retreat changed production")
	}
	for i := range 2 {
		if !slices.Equal(hands[i], s.Catan.Players[i].Resources) {
			t.Fatal("retreat stole resources")
		}
	}
	twoCoreRestore(t, s)
	helperReject(t, s, p, Action{Type: "catan_two_robber"})
}
