package game

import (
	"fmt"
	"testing"
)

func TestCatanTwoFishingKnightsCompleteGames(t *testing.T) {
	for _, events := range []bool{false, true} {
		t.Run(fmt.Sprint(events), func(t *testing.T) {
			s, err := NewCatanTwoFishingCitiesKnights(2, CatanOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if events {
				if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
			}
			if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			seen := map[string]int{}
			for steps := 0; steps < 6000 && !s.Finished; steps++ {
				setup := s.Catan.setup()
				a := twoSeaStep(t, s)
				seen[a.Type]++
				if setup {
					for _, hand := range s.Catan.Fishing.Tokens.Hands {
						if len(hand) != 5 {
							t.Fatal("extra starting fish")
						}
					}
				}
				if steps%37 == 0 {
					twoSeaRestore(t, s)
					if err = s.Catan.validateFishing(); err != nil {
						t.Fatal(err)
					}
				}
				if s.Catan.Two.Bank != 0 || s.Catan.Two.Tokens[0] != 0 || s.Catan.Two.Tokens[1] != 0 || s.catanTwoCanExchangeKnight(s.Turn) {
					t.Fatal("token economy returned")
				}
			}
			if !s.Finished || s.Catan.Players[s.Turn].Score < s.Catan.victoryTargetFor(s.Turn) {
				t.Fatal("incomplete or incorrect victory", s.Phase, s.Round, seen)
			}
			if seen["catan_two_build"] == 0 {
				t.Fatal("missing combination actions", seen)
			}
			t.Log(s.Round, seen)
		})
	}
}

func twoFishingKnightsFixture(t *testing.T, rolls int) *State {
	t.Helper()
	s, err := NewCatanTwoFishingCitiesKnights(2, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		twoSeaStep(t, s)
	}
	s.Turn = 0
	s.Catan.Two.Rolls = []int{2, 3}[:rolls]
	s.Catan.RollID = rolls
	s.Catan.Dice = []int{1, 2}
	s.Catan.CitiesKnights.EventDie = 0
	if rolls == 0 {
		s.Catan.CitiesKnights.EventDie = -1
	}
	s.Phase = "catan_roll"
	if rolls == 2 {
		s.Phase = "catan_turn"
	}
	return s
}

func TestCatanTwoFishingKnightsProgressAndNoTokens(t *testing.T) {
	for rolls := 0; rolls <= 2; rolls++ {
		t.Run(fmt.Sprint(rolls), func(t *testing.T) {
			s := twoFishingKnightsFixture(t, rolls)
			fishCityProgressTop(t, s, 0)
			phase := s.Phase
			payment := s.Catan.fishPayment(0, s.Catan.fishActionCost(0, "catan_fish_progress"))
			helperReject(t, s, 1, Action{Type: "catan_fish_progress", Color: 0, Tokens: payment})
			helperApply(t, s, 0, Action{Type: "catan_fish_progress", Color: 0, Tokens: payment})
			twoSeaRestore(t, s)
			if s.Phase != phase || len(s.Catan.Two.Rolls) != rolls || len(s.Catan.CitiesKnights.Players[0].Progress) != 1 || s.Catan.CitiesKnights.Players[0].Progress[0] != 0 {
				t.Fatal("progress changed production or wrong draw")
			}
			for _, kind := range []string{"catan_two_knight", "catan_two_trade", "catan_two_robber"} {
				helperReject(t, s, 0, Action{Type: kind})
			}
			if s.catanTwoCanExchangeKnight(0) || len(s.Catan.twoKnightTokenVertices(0)) != 0 || s.catanTwoTokenWindow(0) {
				t.Fatal("trade tokens enabled")
			}
			for _, change := range []func(*State){func(s *State) { s.Catan.Fishing.TwoKnights = "" }, func(s *State) { s.Catan.Fishing.TwoKnights = "unknown" }, func(s *State) { s.Catan.Two.Knights = "" }} {
				next := clone(*s)
				change(&next)
				if err := next.Catan.validateFishing(); err == nil {
					t.Fatal("corrupt rule marker accepted")
				}
			}
		})
	}
}

func TestCatanTwoFishingKnightsReplacementBeforeSecondProduction(t *testing.T) {
	s := twoFishingKnightsFixture(t, 1)
	g := s.Catan
	for i := range g.Tiles {
		if g.Tiles[i].Resource < 5 {
			g.Tiles[i].Number = 6
		}
	}
	lake := g.Fishing.Map.Lakes[0].Tile
	vertex := -1
	for _, v := range g.Tiles[lake].Vertices {
		if g.Vertices[v].Level == 0 {
			vertex = v
			break
		}
	}
	if vertex < 0 {
		t.Fatal("no lake vertex")
	}
	g.Vertices[vertex].Owner, g.Vertices[vertex].Level = 1, 1
	g.CitiesKnights.Players[1].Improvements[CatanScience] = 3
	tokens, err := newCatanFishingTokens(2)
	if err != nil {
		t.Fatal(err)
	}
	g.Fishing.Tokens = *tokens
	fishOwn(&g.Fishing.Tokens, 1, 0, 1, 2, 3, 4, 5, 6)
	fishTop(&g.Fishing.Tokens, 21)
	if err := s.catanRollProduction(2); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != 1 || g.CitiesKnights.Pending != nil {
		t.Fatal("replacement did not precede aqueduct")
	}
	twoSeaRestore(t, s)
	helperReject(t, s, 0, Action{Type: "catan_roll"})
	helperApply(t, s, 1, Action{Type: "catan_fish_replace", Card: 0})
	twoSeaRestore(t, s)
	if s.Phase != "catan_aqueduct" || s.CatanPendingActor() != 1 || s.Catan.Fishing.Pending != nil {
		t.Fatal("missing fish-only aqueduct")
	}
	before := s.Catan.Players[1].Resources[0]
	helperApply(t, s, 1, Action{Type: "catan_aqueduct", Color: 0})
	twoSeaRestore(t, s)
	if s.Phase != "catan_roll" || len(s.Catan.Two.Rolls) != 1 || s.Catan.Players[1].Resources[0] != before+1 {
		t.Fatal("lost second production or duplicate resource")
	}
}
