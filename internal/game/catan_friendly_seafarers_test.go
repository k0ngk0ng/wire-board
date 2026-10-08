package game

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCatanFriendlySeafarersBotsComplete(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, info := range CatanSeafarersScenarios(n) {
			if !CatanFriendlySeafarersSupported(n, info.ID) {
				continue
			}
			for _, layout := range info.Layouts {
				t.Run(fmt.Sprintf("%d/%s/%s", n, info.ID, layout), func(t *testing.T) {
					testCatanFriendlySeaGame(t, n, info, layout, false)
				})
			}
		}
	}
}

func TestCatanFriendlySeafarersHarborsBotsComplete(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, info := range CatanSeafarersScenarios(n) {
			if !CatanFriendlySeafarersSupported(n, info.ID) {
				continue
			}
			t.Run(fmt.Sprintf("%d/%s", n, info.ID), func(t *testing.T) { testCatanFriendlySeaGame(t, n, info, info.Layouts[0], true) })
		}
	}
}
func testCatanFriendlySeaGame(t *testing.T, n int, info CatanSeafarersScenario, layout string, harbors bool) {
	t.Helper()
	s, err := NewCatanFriendlySeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: info.ID, Layout: layout})
	if err != nil {
		t.Fatal(err)
	}
	if harbors {
		if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	g := s.Catan
	target := info.VictoryPoints
	if harbors {
		target++
	}
	if (info.ID == "shores" && n > 3) || info.ID == "desert" || info.ID == "wonders" {
		deserts := 0
		for _, tile := range g.Tiles {
			if tile.Resource == CatanDesert && g.robberLandAllowed(tile.ID) {
				deserts++
			}
		}
		if deserts == 0 {
			t.Fatal("friendly retreat requires desert for this recipe")
		}
	}
	initialDev := len(g.DevDeck)
	if g.victoryTarget() != target {
		t.Fatal("variant changed victory threshold")
	}
	actions := map[string]int{}
	steps := 0
	for ; steps < 12000 && !s.Finished; steps++ {
		actor := ckActor(s)
		action, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(steps, s.Phase, err)
		}
		g = s.Catan
		if action.Type == "catan_robber" && action.Tile >= 0 && g.Tiles[action.Tile].Resource != CatanDesert && g.friendlyRobberBlocks(action.Tile) {
			t.Fatal("bot targeted protected land")
		}
		if action.Type == "catan_pirate" && action.Tile >= 0 && g.friendlyPirateBlocks(action.Tile) {
			t.Fatal("bot targeted protected ships")
		}
		actions[action.Type]++
		helperApply(t, s, actor, action)
		g = s.Catan
		fleetSupply(t, g)
		dev := len(g.DevDeck) + len(g.DevDiscard)
		for i, p := range g.Players {
			dev += sum(p.Dev)
			roads, villages, cities := g.pieces(i)
			if roads > 15 || villages > 5 || cities > 4 || g.shipCount(i) > 15 {
				t.Fatal("piece inventory")
			}
			if !g.setup() && (info.ID == "cloth" || info.ID == "pirate_islands") {
				if villages+2*cities < 3 || g.friendlyProtected(i) {
					t.Fatal("three-building scenario fell below protection threshold", i, p.Score)
				}
			}
		}
		if dev != initialDev {
			t.Fatal("development conservation", dev, initialDev)
		}
		if steps%37 == 0 {
			restored := clone(*s)
			if !reflect.DeepEqual(*s, restored) {
				t.Fatal("save roundtrip")
			}
			s = &restored
		}
	}
	if !s.Finished || len(s.Winners) == 0 || info.ID != "cloth" && len(s.Winners) != 1 {
		t.Fatal("game did not finish", steps, s.Phase)
	}
	winner := s.Winners[0]
	switch info.ID {
	case "wonders":
		if !s.Catan.wonderVictory(winner) {
			t.Fatal("wonder victory")
		}
	case "pirate_islands":
		if s.Catan.pirateIslands().Fortresses[winner].Strength != 0 || s.Catan.Players[winner].Score < target {
			t.Fatal("fortress victory")
		}
	case "cloth":
		if s.Catan.Players[winner].Score < target {
			c := s.Catan.cloth()
			empty := 0
			for _, v := range c.Villages {
				if v.Stock == 0 {
					empty++
				}
			}
			if empty < c.EmptyLimit {
				t.Fatal("cloth ended before depletion/score")
			}
			for i, p := range s.Catan.Players {
				if p.Score > s.Catan.Players[winner].Score || p.Score == s.Catan.Players[winner].Score && c.Held[i] > c.Held[winner] {
					t.Fatal("wrong depleted cloth winner")
				}
			}
			tied := []int{}
			for i, p := range s.Catan.Players {
				if p.Score == s.Catan.Players[winner].Score && c.Held[i] == c.Held[winner] {
					tied = append(tied, i)
				}
			}
			if !reflect.DeepEqual(s.Winners, tied) {
				t.Fatal("equal score and cloth must share the win")
			}

		}
	default:
		if s.Catan.Players[winner].Score < target {
			t.Fatal("score victory")
		}
	}
	t.Logf("steps=%d robber=%d pirate=%d score=%d", steps, actions["catan_robber"], actions["catan_pirate"], s.Catan.Players[winner].Score)
}

func TestCatanFriendlySeafarersConstructorGates(t *testing.T) {
	for _, scenario := range []string{"unknown"} {
		if _, err := NewCatanFriendlySeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: scenario}); err == nil {
			t.Fatal("unverified scenario accepted", scenario)
		}
	}
	for _, options := range []CatanOptions{{Helpers: true}, {AllHelpers: true}} {
		if _, err := NewCatanFriendlySeafarers(3, options, CatanSeafarersSetup{Scenario: "desert"}); err == nil {
			t.Fatal("unverified Helpers combination")
		}
	}
	if _, err := NewCatanFriendlySeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: "desert", Rules: "future"}); err == nil {
		t.Fatal("unknown sea version")
	}
}

func TestCatanFriendlySeafarersDesertRetreatAndKnight(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, scenario := range []string{"shores", "desert", "wonders"} {
			if scenario == "shores" && n == 3 {
				continue
			}
			t.Run(fmt.Sprintf("%d/%s", n, scenario), func(t *testing.T) {
				s, err := NewCatanFriendlySeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: scenario})
				if err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				// Deliberate topology fixture: every ordinary destination is protected.
				// Complete games above verify legal placement and inventory separately.
				for i := range g.Vertices {
					g.Vertices[i].Owner = 1
					g.Vertices[i].Level = 1
				}
				for i := range g.Players {
					g.Players[i].Score = 2
				}
				g.SetupStep = g.SetupLimit()
				g.ResumePhase = "catan_turn"
				g.Seafarers.Pirate = -1
				s.Turn = 0
				s.Phase = "catan_robber"
				for _, tile := range g.Tiles {
					if tile.Resource == CatanDesert {
						g.Robber = tile.ID
						break
					}
				}
				start := g.Robber
				if !g.robberAllowed(start) {
					t.Fatal("current desert fallback missing")
				}
				for _, tile := range g.Tiles {
					if g.robberAllowed(tile.ID) && tile.Resource != CatanDesert {
						t.Fatal("protected terrain in legal hints")
					}
				}
				helperApply(t, s, 0, Action{Type: "catan_robber", Tile: start})
				if s.Phase != "catan_turn" || len(s.Catan.Victims) > 0 {
					t.Fatal("retreat did not complete without theft")
				}

				// A fresh legal setup verifies using a knight while every
				// opponent is protected, without impossible piece counts.
				s, err = NewCatanFriendlySeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: scenario})
				if err != nil {
					t.Fatal(err)
				}
				for s.Catan.setup() {
					actor := ckActor(s)
					a, e := s.BotAction(actor)
					if e != nil {
						t.Fatal(e)
					}
					helperApply(t, s, actor, a)
				}
				s.Turn = 0
				s.Phase = "catan_turn"
				s.Catan.Players[0].Dev[0] = 1
				helperApply(t, s, 0, Action{Type: "catan_dev", Card: 0})
				a, err := s.BotAction(0)
				if err != nil {
					t.Fatal(err)
				}
				before := make([][]int, n)
				for i, p := range s.Catan.Players {
					before[i] = append([]int{}, p.Resources...)
				}
				helperApply(t, s, 0, a)
				if s.Phase != "catan_turn" || s.Catan.Players[0].Knights != 1 {
					t.Fatal("knight without theft did not resume")
				}
				for i, p := range s.Catan.Players {
					if !reflect.DeepEqual(p.Resources, before[i]) {
						t.Fatal("knight stole from protected opponent")
					}
				}

			})
		}
	}
}
