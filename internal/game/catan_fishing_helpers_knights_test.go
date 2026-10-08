package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func fishKnightHelperGame(t *testing.T, n int, scene string, all bool) *State {
	t.Helper()
	o := CatanOptions{FiveSix: n > 4, Helpers: true, AllHelpers: all}
	var s *State
	var err error
	if scene == "" {
		s, err = NewCatanFishingCitiesKnights(n, o)
	} else {
		var world *CatanNewWorldMap
		if scene == "new_world" {
			world, err = GenerateCatanFishingNewWorldMap(n)
			if err != nil {
				t.Fatal(err)
			}
		}
		s, err = NewCatanFishingCitiesKnightsSeafarers(n, o, CatanSeafarersSetup{Scenario: scene}, world)
	}
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func fishKnightHelperRestore(t *testing.T, s *State) {
	t.Helper()
	helperCityRestore(t, s)
	if err := s.Catan.validateFishing(); err != nil {
		t.Fatal(err)
	}
	if err := s.validateFishingHelpers(); err != nil {
		t.Fatal(err)
	}
}
func TestCatanFishingHelpersKnightsConfigurations(t *testing.T) {
	for _, scene := range []string{"", "shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		for n := 3; n <= 6; n++ {
			for _, all := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/%t", scene, n, all), func(t *testing.T) {
					s := fishKnightHelperGame(t, n, scene, all)
					if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
					if err := s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
						t.Fatal(err)
					}
					if err := s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
						t.Fatal(err)
					}
					finishFishHelperSetup(t, s)
					for step := 0; step < 18; step++ {
						fishHelperStep(t, s)
						fishKnightHelperRestore(t, s)
					}
					if !s.Catan.cityHelpers() || !s.Catan.fishingHelpers() {
						t.Fatal("lost combined rules")
					}
				})
			}
		}
	}
}
func TestCatanFishingHelpersKnightsNaturalGames(t *testing.T) {
	for _, tc := range []struct {
		n      int
		scene  string
		events bool
	}{{3, "", false}, {6, "", true}, {3, "shores", true}, {4, "islands", false}, {5, "fog", true}, {6, "desert", false}, {3, "tribe", true}, {5, "cloth", false}, {3, "wonders", true}, {4, "new_world", false}} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			s := fishKnightHelperGame(t, tc.n, tc.scene, true)
			if tc.events {
				if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
			}
			helper, fish := 0, 0
			for step := 0; step < 15000 && !s.Finished; step++ {
				a := fishHelperStep(t, s)
				if strings.HasPrefix(a.Type, "catan_helper") {
					helper++
				}
				if strings.HasPrefix(a.Type, "catan_fish") {
					fish++
				}
				if step%83 == 0 {
					fishKnightHelperRestore(t, s)
				}
			}
			if !s.Finished {
				t.Fatal("unfinished natural game", s.Round, s.Phase)
			}
			fishKnightHelperRestore(t, s)
			t.Log("rounds", s.Round, "helper", helper, "fish", fish)
		})
	}
}
func TestCatanFishingHelpersKnightsFishGoldAqueductHilda(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := fishKnightHelperGame(t, n, "shores", true)
			finishFishHelperSetup(t, s)
			g := s.Catan
			s.Turn, s.Phase = 0, "catan_roll"
			g.TurnSerial = 10
			g.RollID++
			g.Dice = []int{1, 1}
			eventAssignHelper(t, s, 1, 3)
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
				t.Fatal("missing gold")
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
			before := sum(g.Players[1].Resources)
			if err := s.catanRollProduction(2); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_fish_replace" || g.HelperPending != nil || g.CitiesKnights.Helpers.Production != nil {
				t.Fatal("fish must finish first")
			}
			fishKnightHelperRestore(t, s)
			helperApply(t, s, 1, Action{Type: "catan_fish_keep"})
			if s.Phase != "catan_gold" || s.Catan.HelperPending != nil {
				t.Fatal("gold must finish second")
			}
			fishKnightHelperRestore(t, s)
			helperApply(t, s, 0, Action{Type: "catan_gold", Take: []int{2, 0, 0, 0, 0}})
			if s.Phase != "catan_aqueduct" || s.CatanPendingActor() != 1 || s.Catan.CitiesKnights.Helpers.Production == nil {
				t.Fatal("aqueduct must precede helper")
			}
			fishKnightHelperRestore(t, s)
			helperApply(t, s, 1, Action{Type: "catan_aqueduct", Color: 0})
			if s.Phase != "catan_helper" || s.CatanPendingActor() != 1 || s.Catan.HelperPending.Kind != "resource" {
				t.Fatal("fish or aqueduct cancelled Hilda")
			}
			fishKnightHelperRestore(t, s)
			helperApply(t, s, 1, Action{Type: "catan_helper_choice", Color: 1})
			helperApply(t, s, 1, Action{Type: "catan_helper_choice", Choice: "flip"})
			if s.Phase != "catan_turn" || s.Turn != 0 || sum(s.Catan.Players[1].Resources) != before+2 || len(s.Catan.Fishing.Tokens.Hands[1]) != 7 {
				t.Fatal("incorrect compensation continuation")
			}
			fishKnightHelperRestore(t, s)
		})
	}
}
func TestCatanFishingHelpersKnightsFishCannotPayForHelpers(t *testing.T) {
	s := fishKnightHelperGame(t, 3, "", true)
	finishFishHelperSetup(t, s)
	g := s.Catan
	p := s.Turn
	s.Phase = "catan_turn"
	g.TurnSerial = 10
	eventAssignHelper(t, s, p, 6)
	g.Fishing.Tokens = *fishingTokens(t, 3)
	fishOwn(&g.Fishing.Tokens, p, 0, 21, 22)
	cards := slices.Clone(g.CitiesKnights.ProgressDecks[0])
	helperReject(t, s, p, Action{Type: "catan_fish_progress", Color: 0, Tokens: []int{0, 21, 22}, Skill: "helper"})
	helperApply(t, s, p, Action{Type: "catan_fish_progress", Color: 0, Tokens: []int{0, 21, 22}})
	if s.Catan.HelperPending != nil || !s.Catan.helperReady(p, 6) || !slices.Equal(s.Catan.CitiesKnights.ProgressDecks[0], cards[:len(cards)-1]) {
		t.Fatal("fish incorrectly invoked Diara")
	}
	for c := 2; c < 5; c++ {
		s.Catan.Players[p].Resources[c]++
		s.Catan.Bank[c]--
	}
	helperApply(t, s, p, Action{Type: "catan_helper", Choice: "progress_buy", Color: 1, Tokens: []int{0, 0, 1, 1, 1}})
	for viewer := -1; viewer < 3; viewer++ {
		raw, _ := json.Marshal(s.View(viewer))
		var view map[string]any
		json.Unmarshal(raw, &view)
		q := view["catan"].(map[string]any)["helperPending"].(map[string]any)
		if (q["cards"] != nil) != (viewer == p) {
			t.Fatal("private candidates leaked")
		}
	}
	fishKnightHelperRestore(t, s)
	helperApply(t, s, p, Action{Type: "catan_helper_choice", Card: s.Catan.HelperPending.Cards[0]})
	helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
	fishKnightHelperRestore(t, s)
}
func TestCatanFishingHelpersKnightsLakeAndEmptyOrdinaryBank(t *testing.T) {
	for _, helper := range []int{10, 11} {
		t.Run(fmt.Sprint(helper), func(t *testing.T) {
			s := fishKnightHelperGame(t, 3, "", true)
			finishFishHelperSetup(t, s)
			g := s.Catan
			p := s.Turn
			s.Phase = "catan_turn"
			g.TurnSerial = 10
			eventAssignHelper(t, s, p, helper)
			helperReject(t, s, p, Action{Type: "catan_helper", Color: 0}) // The robber is dormant.
			g.CitiesKnights.Invasions = 1
			g.Robber = g.Fishing.Map.Lakes[0].Tile
			before := g.Players[p].Resources[0]
			helperReject(t, s, p, Action{Type: "catan_helper", Color: CatanCoin})
			helperApply(t, s, p, Action{Type: "catan_helper", Color: 0})
			if s.Catan.Players[p].Resources[0] != before+1 {
				t.Fatal("lake reward")
			}
			if helper == 10 && s.Catan.Robber != -1 {
				t.Fatal("no-desert robber should leave board")
			}
			helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
			fishKnightHelperRestore(t, s)
			if helper == 10 {
				g = s.Catan
				g.TurnSerial++
				g.Robber = g.Fishing.Map.Lakes[0].Tile
				for c := 0; c < 5; c++ {
					g.Players[(p+1)%3].Resources[c] += g.Bank[c]
					g.Bank[c] = 0
				}
				helperApply(t, s, p, Action{Type: "catan_helper", Color: 0})
				if s.Catan.Robber != -1 {
					t.Fatal("commodity-only bank prevented retreat")
				}
			}
		})
	}
}

func TestCatanFishingHelpersKnightsThorolfWallsExcludeFish(t *testing.T) {
	for _, held := range []int{8, 10} {
		t.Run(fmt.Sprint(held), func(t *testing.T) {
			s := fishKnightHelperGame(t, 3, "", true)
			finishFishHelperSetup(t, s)
			g := s.Catan
			p := (s.Turn + 1) % 3
			g.TurnSerial = 10
			eventAssignHelper(t, s, p, 5)
			for i := range g.Players {
				catanMove(g.Players[i].Resources, g.Bank, slices.Clone(g.Players[i].Resources))
			}
			g.Players[p].Resources[CatanCoin] = held
			g.Bank[CatanCoin] -= held
			city := -1
			for _, v := range g.Vertices {
				if v.Owner == p && v.Level == 2 {
					city = v.ID
				}
			}
			if city < 0 {
				t.Fatal("no city")
			}
			g.CitiesKnights.Walls = []int{city}
			g.Fishing.Tokens = *fishingTokens(t, 3)
			fishOwn(&g.Fishing.Tokens, p, 0, 1, 2, 3, 4, 5, 6)
			g.RollID++
			g.Dice = []int{3, 4}
			if err := s.catanRollProduction(7); err != nil {
				t.Fatal(err)
			}
			if g.DiscardDue[p] != 0 || g.HelperPending == nil {
				t.Fatal("wall or fish count misapplied")
			}
			if held == 8 {
				helperApply(t, s, p, Action{Type: "catan_helper_choice", Color: 0})
			}
			helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
			want := held
			if held == 8 {
				want++
			}
			if sum(s.Catan.Players[p].Resources) != want || len(s.Catan.Fishing.Tokens.Hands[p]) != 7 {
				t.Fatal("wrong protected inventory")
			}
			fishKnightHelperRestore(t, s)
		})
	}
}
