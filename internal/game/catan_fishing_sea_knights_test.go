package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishingSeaKnightsGame(t *testing.T, n int, scene string) *State {
	t.Helper()
	var world *CatanNewWorldMap
	if scene == "new_world" {
		var err error
		world, err = GenerateCatanFishingNewWorldMap(n)
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err := NewCatanFishingCitiesKnightsSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: scene}, world)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestCatanFishingSeaKnightsConfigurations(t *testing.T) {
	for _, scene := range []string{"shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		for n := 3; n <= 6; n++ {
			t.Run(fmt.Sprintf("%s/%d", scene, n), func(t *testing.T) {
				s := fishingSeaKnightsGame(t, n, scene)
				if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
				if err := s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
					t.Fatal(err)
				}
				if err := s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if g.Fishing.SeaKnights != CatanFishingSeaKnightsRules || g.Robber != -1 || g.Seafarers.Pirate != -1 || len(g.Bank) != 8 || len(g.DevDeck) != 0 {
					t.Fatal("wrong setup")
				}
				want := map[string]int{"shores": 17, "islands": 16, "fog": 15, "desert": 17, "tribe": 16, "cloth": 17, "wonders": 13, "new_world": 15}[scene]
				if g.victoryTargetFor(0) != want {
					t.Fatal("target", g.victoryTargetFor(0), want)
				}
				finishFishHelperSetup(t, s)
				for p := range g.Players {
					settlements, cities := 0, 0
					for _, v := range s.Catan.Vertices {
						if v.Owner == p {
							if v.Level == 1 {
								settlements++
							}
							if v.Level == 2 {
								cities++
							}
						}
					}
					wantVillages := 1
					if scene == "cloth" {
						wantVillages = 2
					}
					if cities != 1 || settlements != wantVillages {
						t.Fatal("starting buildings", p, settlements, cities)
					}
					if sum(s.Catan.Players[p].Resources[5:]) != 0 {
						t.Fatal("starting commodity")
					}
				}
				for i := 0; i < 24; i++ {
					fishHelperStep(t, s)
					copy := clone(*s)
					s = &copy
				}
				if err := s.Catan.validateFishing(); err != nil {
					t.Fatal(err)
				}
				ckProgressStock(t, s.Catan)
			})
		}
	}
}
func TestCatanFishingSeaKnightsNaturalGames(t *testing.T) {
	for i, scene := range []string{"shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		t.Run(scene, func(t *testing.T) {
			n := 3 + i%4
			s := fishingSeaKnightsGame(t, n, scene)
			if i%2 == 0 {
				if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
			}
			paid := 0
			phases := map[string]bool{}
			for step := 0; step < 20000 && !s.Finished; step++ {
				phases[s.Phase] = true
				a := fishHelperStep(t, s)
				if _, ok := catanFishCosts[a.Type]; ok {
					paid++
				}
				ckProgressStock(t, s.Catan)
				ckKnightStock(t, s.Catan)
				if step%43 == 0 {
					copy := clone(*s)
					s = &copy
				}
			}
			if !s.Finished || len(s.Winners) == 0 {
				t.Fatal("not finished", s.Round, s.Phase, phases)
			}
			t.Log("finished", n, s.Round, "fish payments", paid, phases)
		})
	}
}
func TestCatanFishingSeaKnightsPaymentsAndIsolation(t *testing.T) {
	for _, scene := range []string{"shores", "islands", "tribe", "cloth", "wonders", "new_world"} {
		t.Run(scene, func(t *testing.T) {
			s := fishingSeaKnightsGame(t, 3, scene)
			finishFishHelperSetup(t, s)
			s.Phase = "catan_turn"
			p := s.Turn
			s.Catan.Fishing.Tokens = *fishingTokens(t, 3)
			fishOwn(&s.Catan.Fishing.Tokens, p, 0, 1, 11, 12, 21, 22, 23)
			legal := s.catanFishLegal(p)
			if slices.Contains(legal["actions"].([]string), "catan_fish_dev") {
				t.Fatal("offered development cards")
			}
			fishGameReject(t, s, p, Action{Type: "catan_fish_dev", Tokens: []int{0, 21, 22}})
			fishGameReject(t, s, p, Action{Type: "catan_fish_resource", Tokens: []int{0, 21}, Color: CatanCoin})
			a, err := s.BotAction(p)
			if err != nil {
				t.Fatal(err)
			}
			hidden := clone(*s)
			for i := range hidden.Catan.CitiesKnights.ProgressDecks {
				slices.Reverse(hidden.Catan.CitiesKnights.ProgressDecks[i])
			}
			slices.Reverse(hidden.Catan.Fishing.Tokens.DrawPile)
			b, err := hidden.BotAction(p)
			if err != nil || !reflect.DeepEqual(a, b) {
				t.Fatal("bot peeks", a, b, err)
			}
			deck := s.Catan.CitiesKnights.ProgressDecks[0]
			card := deck[len(deck)-1]
			helperApply(t, s, p, Action{Type: "catan_fish_progress", Tokens: []int{0, 21, 22}, Color: 0})
			if !slices.Contains(s.Catan.CitiesKnights.Players[p].Progress, card) && !slices.Contains(s.Catan.CitiesKnights.Players[p].PublicProgress, card) {
				t.Fatal("no fish progress")
			}
			for _, marker := range []string{"", "invalid"} {
				bad := clone(*s)
				bad.Catan.Fishing.SeaKnights = marker
				fishGameReject(t, &bad, p, Action{Type: "catan_fish_resource", Tokens: []int{1, 23}, Color: 0})
			}
			g := s.Catan
			ships := s.catanFishLegal(p)["ships"].([]int)
			if len(ships) > 0 {
				helperApply(t, s, p, Action{Type: "catan_fish_ship", Edge: ships[0], Tokens: []int{11, 23}})
				if !s.Catan.Edges[ships[0]].Ship || s.Catan.Edges[ships[0]].Owner != p {
					t.Fatal("fish ship missing")
				}
			}
			if err := g.validateFishing(); err != nil {
				t.Fatal(err)
			}
		})
	}
	base := fishCityGame(t, 3)
	if base.Catan.Fishing.SeaKnights != "" {
		t.Fatal("base fishing knights changed")
	}
	base.Catan.Fishing.SeaKnights = CatanFishingSeaKnightsRules
	if err := base.Catan.validateFishing(); err == nil {
		t.Fatal("marker accepted without sea")
	}
}

func TestCatanFishingSeaKnightsFishGoldAqueductOrder(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := fishingSeaKnightsGame(t, n, "shores")
			finishFishHelperSetup(t, s)
			g := s.Catan
			s.Turn, s.Phase = 0, "catan_roll"
			g.RollID++
			// Production-only fixture keeps the map and component inventory, isolates
			// fish-only city income from a different player's gold income.
			for i := range g.Vertices {
				g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
			}
			gold := -1
			for i := range g.Tiles {
				if g.Tiles[i].Resource < 5 {
					g.Tiles[i].Number = 6
				}
				if g.Tiles[i].Resource == CatanGold {
					gold = i
					g.Tiles[i].Number = 2
				}
			}
			if gold < 0 {
				t.Fatal("no gold")
			}
			v := g.Tiles[g.Fishing.Map.Lakes[0].Tile].Vertices[0]
			g.Vertices[v].Owner, g.Vertices[v].Level = 1, 2
			v = g.Tiles[gold].Vertices[0]
			g.Vertices[v].Owner, g.Vertices[v].Level = 0, 2
			g.CitiesKnights.Players[0].Improvements[CatanScience] = 3
			g.CitiesKnights.Players[1].Improvements[CatanScience] = 3
			g.Fishing.Tokens = *fishingTokens(t, n)
			fishOwn(&g.Fishing.Tokens, 1, 0, 1, 2, 3, 4, 5, 6)
			fishTop(&g.Fishing.Tokens, 21, 22)
			before := slices.Clone(g.Players[1].Resources)
			if err := s.catanRollProduction(2); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != 1 || g.GoldPending != nil || g.CitiesKnights.Pending != nil {
				t.Fatal("fish first")
			}
			restored := clone(*s)
			s = &restored
			helperApply(t, s, 1, Action{Type: "catan_fish_keep"})
			if s.Phase != "catan_gold" || s.CatanPendingActor() != 0 || s.Catan.Fishing.Pending != nil {
				t.Fatal("gold second", s.Phase)
			}
			helperApply(t, s, 0, Action{Type: "catan_gold", Take: []int{2, 0, 0, 0, 0}})
			if s.Phase != "catan_aqueduct" || s.CatanPendingActor() != 1 {
				t.Fatal("fish should not suppress aqueduct", s.Phase)
			}
			restored = clone(*s)
			s = &restored
			fishGameReject(t, s, 0, Action{Type: "catan_aqueduct", Color: 0})
			helperApply(t, s, 1, Action{Type: "catan_aqueduct", Color: 0})
			if s.Phase != "catan_turn" || s.Turn != 0 || s.Catan.Players[1].Resources[0] != before[0]+1 {
				t.Fatal("aqueduct did not resume owner")
			}
			if len(s.Catan.Fishing.Tokens.Hands[1]) != 7 {
				t.Fatal("fish cap")
			}
		})
	}
}
