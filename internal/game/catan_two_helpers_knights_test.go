package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func twoHelperKnightGame(t *testing.T, scene string, fish, all bool) *State {
	t.Helper()
	o := CatanOptions{Helpers: true, AllHelpers: all}
	var s *State
	var err error
	if scene == "" {
		if fish {
			s, err = NewCatanTwoFishingCitiesKnights(2, o)
		} else {
			s, err = NewCatanTwoCitiesKnights(2, o)
		}
	} else {
		var world *CatanNewWorldMap
		if scene == "new_world" {
			if fish {
				world, err = GenerateCatanFishingNewWorldMap(4)
			} else {
				world, err = GenerateCatanNewWorldMap(4)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		s, err = NewCatanTwoSeafarersCitiesKnights(2, o, CatanSeafarersSetup{Scenario: scene}, world, fish)
	}
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func twoHelperKnightRestore(t *testing.T, s *State) {
	t.Helper()
	twoCoreRestore(t, s)
	helperCityRestore(t, s)
	if err := s.Catan.validateFishing(); err != nil {
		t.Fatal(err)
	}
}
func twoHelperKnightReady(t *testing.T, id int, fish bool) *State {
	t.Helper()
	s := twoHelperKnightGame(t, "", fish, true)
	finishFishHelperSetup(t, s)
	g := s.Catan
	s.Phase = "catan_turn"
	g.TurnSerial = 10
	g.Two.Rolls = []int{2, 3}
	g.RollID = 2
	g.Dice = []int{1, 2}
	g.CitiesKnights.EventDie = 0
	eventAssignHelper(t, s, s.Turn, id)
	twoHelperKnightRestore(t, s)
	return s
}
func TestCatanTwoHelpersKnightsConfigurations(t *testing.T) {
	for _, scene := range append([]string{""}, twoSeaScenarios...) {
		for _, fish := range []bool{false, true} {
			for _, all := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/fish%t/all%t", scene, fish, all), func(t *testing.T) {
					s := twoHelperKnightGame(t, scene, fish, all)
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
					for i := 0; i < 30; i++ {
						twoSeaStep(t, s)
						twoHelperKnightRestore(t, s)
					}
				})
			}
		}
	}
}
func TestCatanTwoHelpersKnightsNaturalGames(t *testing.T) {
	for i, scene := range append([]string{""}, twoSeaScenarios...) {
		for _, fish := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fish%t", scene, fish), func(t *testing.T) {
				s := twoHelperKnightGame(t, scene, fish, true)
				if i%2 == 0 {
					if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				seen := map[string]int{}
				for step := 0; step < 12000 && !s.Finished; step++ {
					a := twoSeaStep(t, s)
					seen[a.Type]++
					if step%67 == 0 {
						twoHelperKnightRestore(t, s)
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round, s.Phase, seen)
				}
				twoHelperKnightRestore(t, s)
				t.Log("rounds", s.Round, "actions", seen)
			})
		}
	}
}
func TestCatanTwoHelpersKnightsProgressPrivacyAndInventory(t *testing.T) {
	for _, fish := range []bool{false, true} {
		t.Run(fmt.Sprint(fish), func(t *testing.T) {
			s := twoHelperKnightReady(t, 6, fish)
			p := s.Turn
			twoKnightsHand(s, p, []int{0, 0, 1, 1, 1, 0, 0, 0})
			helperApply(t, s, p, Action{Type: "catan_helper", Choice: "progress_buy", Color: CatanScience})
			twoHelperKnightRestore(t, s)
			q := s.Catan.HelperPending
			if q == nil || q.Kind != "progress" || len(q.Cards) != 3 {
				t.Fatal("missing private choice")
			}
			for _, viewer := range []int{1 - p, -1} {
				view := s.View(viewer)["catan"].(map[string]any)["helperPending"].(map[string]any)
				if _, ok := view["cards"]; ok {
					t.Fatal("private candidates leaked")
				}
			}
			card := q.Cards[0]
			helperReject(t, s, 1-p, Action{Type: "catan_helper_choice", Card: card})
			helperApply(t, s, p, Action{Type: "catan_helper_choice", Card: card})
			helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
			twoHelperKnightRestore(t, s)
			if s.Phase != "catan_turn" || s.Catan.Two.Pending != nil {
				t.Fatal("choice changed neutral turn")
			}
		})
	}
}
func TestCatanTwoHelpersKnightsGregorNeutralChain(t *testing.T) {
	for _, kind := range []string{"settlement", "city"} {
		for _, fish := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fish%t", kind, fish), func(t *testing.T) {
				s := twoHelperKnightReady(t, 8, fish)
				g, p := s.Catan, s.Turn
				site := -1
				for step := 0; step < 12 && site < 0; step++ {
					for _, v := range g.Vertices {
						if kind == "city" && v.Owner == p && v.Level == 1 || kind == "settlement" && g.canSettlement(p, v.ID, false) {
							site = v.ID
							break
						}
					}
					if site < 0 {
						for _, e := range g.Edges {
							if g.canRoad(p, e.ID) {
								g.Edges[e.ID].Owner = p
								break
							}
						}
					}
				}
				if site < 0 {
					t.Fatal("no build site")
				}
				knight, neutral := -1, -1
				for _, v := range g.Vertices {
					if v.Level == 0 {
						if knight < 0 {
							knight = v.ID
						} else {
							neutral = v.ID
							break
						}
					}
				}
				if kind == "settlement" {
					knight = site
				}
				if neutral == knight {
					for _, v := range g.Vertices {
						if v.Level == 0 && v.ID != knight {
							neutral = v.ID
							break
						}
					}
				}
				g.CitiesKnights.Knights = append(g.CitiesKnights.Knights, CatanKnight{Owner: p, Vertex: knight, Strength: 2}, CatanKnight{Owner: -2, Vertex: neutral, Strength: 1})
				twoKnightsHand(s, p, []int{1, 1, 0, 1, 2, 0, 0, 0})
				tokens, bank := g.Two.Tokens[p], g.Two.Bank
				a := Action{Type: "catan_" + kind, Skill: "helper", Vertex: site, Target: neutral}
				helperReject(t, s, p, a)
				a.Target = knight
				helperApply(t, s, p, a)
				if s.Catan.knightAt(knight) != nil || s.Catan.knightAt(neutral) == nil || len(s.Catan.HelperExile) != 0 {
					t.Fatal("wrong knight removal")
				}
				earned := s.Catan.Two.Tokens[p] - tokens
				want := 0
				if kind == "settlement" && !fish {
					want = min(bank, g.twoSettlementTokens(p, site))
				}
				if earned != want {
					t.Fatal("sacrificed knight earned tokens", earned, want)
				}
				if s.Catan.Two.Pending != nil {
					t.Fatal("neutral before helper choice")
				}
				twoHelperKnightRestore(t, s)
				helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
				if kind == "settlement" {
					if s.Catan.Two.Pending == nil || s.Catan.Two.Pending.Kind != "settlement" {
						t.Fatal("missing neutral village")
					}
					twoSeaStep(t, s)
				}
				if s.Phase != "catan_turn" || s.Catan.Two.AfterHelper != "" || s.Catan.Two.Tokens[p] != tokens+earned {
					t.Fatal("bad continuation")
				}
				twoHelperKnightRestore(t, s)
			})
		}
	}
}
func TestCatanTwoHelpersKnightsMarkersAndOldSaves(t *testing.T) {
	s := twoHelperKnightReady(t, 2, false)
	for _, mutate := range []func(*State){func(s *State) { s.Catan.Two.Helpers = "" }, func(s *State) { s.Catan.CitiesKnights.Helpers = nil }} {
		b, _ := json.Marshal(s)
		var bad State
		json.Unmarshal(b, &bad)
		mutate(&bad)
		if bad.validateCatanTwo() == nil {
			t.Fatal("missing marker accepted")
		}
	}
	plain, err := NewCatanTwoCitiesKnights(2, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Catan.CitiesKnights.Helpers != nil || plain.Catan.Two.Helpers != "" {
		t.Fatal("base enabled helpers")
	}
	twoHelperKnightRestore(t, plain)
	if !slices.Equal(plain.Catan.HelperDisplay, []int(nil)) && len(plain.Catan.HelperDisplay) > 0 {
		t.Fatal("base helpers")
	}
}

func TestCatanTwoHelpersKnightsDoubleProductionAqueduct(t *testing.T) {
	for _, fish := range []bool{false, true} {
		for _, skip := range []bool{false, true} {
			for _, other := range []bool{false, true} {
				t.Run(fmt.Sprintf("fish%t/skip%t/other%t", fish, skip, other), func(t *testing.T) {
					s := twoHelperKnightReady(t, 3, fish)
					g, p := s.Catan, s.Turn
					if other {
						p = 1 - p
						eventAssignHelper(t, s, p, 3)
					}
					g.Two.Rolls = nil
					s.Phase = "catan_roll"
					for i := range g.Tiles {
						if g.Tiles[i].Resource < 5 {
							g.Tiles[i].Number = 6
						}
					}
					g.CitiesKnights.Players[p].Improvements[CatanScience] = 3
					before := sum(g.Players[p].Resources)
					if err := s.catanCityRoll(2, 2, 1); err != nil {
						t.Fatal(err)
					}
					if s.Phase != "catan_aqueduct" || g.HelperPending != nil {
						t.Fatal("aqueduct must precede Hilda")
					}
					twoHelperKnightRestore(t, s)
					helperApply(t, s, p, Action{Type: "catan_aqueduct", Color: 0})
					if s.Catan.HelperPending == nil || s.Catan.HelperPending.Player != p {
						t.Fatal("missing Hilda")
					}
					helperReject(t, s, 1-p, Action{Type: "catan_helper_choice", Color: 1})
					a := Action{Type: "catan_helper_choice", Color: 1}
					if skip {
						a.Choice = "skip"
					}
					helperApply(t, s, p, a)
					if !skip {
						helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
					}
					if s.Phase != "catan_roll" || len(s.Catan.Two.Rolls) != 1 {
						t.Fatal("did not resume second production")
					}
					twoHelperKnightRestore(t, s)
					if err := s.catanCityRoll(2, 3, 1); err != nil {
						t.Fatal(err)
					}
					helperApply(t, s, p, Action{Type: "catan_aqueduct", Color: 0})
					if (s.Catan.HelperPending != nil) != skip {
						t.Fatal("Hilda repeated after use or skipped Hilda lost")
					}
					if skip {
						helperApply(t, s, p, Action{Type: "catan_helper_choice", Color: 1})
						helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
					}
					if s.Phase != "catan_turn" || sum(s.Catan.Players[p].Resources) != before+3 {
						t.Fatal("double production compensation/continuation")
					}
					twoHelperKnightRestore(t, s)
				})
			}
		}
	}
}
