package game

import (
	"fmt"
	"slices"
	"testing"
)

func twoSeaKnightsGame(t *testing.T, scenario, layout string, fishing, events bool) *State {
	t.Helper()
	var world *CatanNewWorldMap
	var err error
	if scenario == "new_world" {
		layout = "prepared"
		if fishing {
			world, err = GenerateCatanFishingNewWorldMap(4)
		} else {
			world, err = GenerateCatanNewWorldMap(4)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err := NewCatanTwoSeafarersCitiesKnights(2, CatanOptions{}, CatanSeafarersSetup{Scenario: scenario, Layout: layout}, world, fishing)
	if err != nil {
		t.Fatal(err)
	}
	if events {
		if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
			t.Fatal(err)
		}
	}
	return s
}
func TestCatanTwoSeafarersKnightsCompleteGames(t *testing.T) {
	for i, scene := range twoSeaScenarios {
		for _, fish := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fish%t", scene, fish), func(t *testing.T) {
				s := twoSeaKnightsGame(t, scene, "fixed", fish, i%2 == 1)
				if err := s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
					t.Fatal(err)
				}
				if err := s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
					t.Fatal(err)
				}
				seen := map[string]int{}
				for steps := 0; steps < 7500 && !s.Finished; steps++ {
					a := twoSeaStep(t, s)
					seen[a.Type]++
					if steps%31 == 0 {
						twoSeaRestore(t, s)
						if err := s.Catan.validateFishing(); err != nil {
							t.Fatal(err)
						}
					}
				}
				if !s.Finished || len(s.Winners) == 0 {
					t.Fatal("unfinished", s.Round, s.Phase, seen)
				}
				t.Log(s.Round, seen)
			})
		}
	}
}

// Cloth scoring must refresh even when nobody needs an aqueduct response.
// When a response is due it still precedes settlement of the win.
func TestCatanCityClothProductionScoreAndAqueduct(t *testing.T) {
	for _, n := range []int{2, 3} {
		for _, aqueduct := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/aqueduct%t", n, aqueduct), func(t *testing.T) {
				var s *State
				if n == 2 {
					s = twoSeaKnightsGame(t, "cloth", "fixed", false, false)
				} else {
					s = ckClothFixture(t, n, "fixed")
				}
				for s.Catan.setup() {
					a, err := s.BotAction(s.Turn)
					if err != nil {
						t.Fatal(err)
					}
					if err = s.Apply(s.Turn, a); err != nil {
						t.Fatal(err)
					}
				}
				g, p := s.Catan, s.Turn
				c := g.cloth()
				c.Held[p] = 1
				c.Stock--
				c.Villages[0].Traders = []int{p}
				s.catanScores()
				g.CitiesKnights.Players[p].DefenderPoints = g.victoryTargetFor(p) - g.Players[p].Score - 1
				s.catanScores()
				if aqueduct {
					g.CitiesKnights.Players[p].Improvements[CatanScience] = 3
				}
				if err := s.catanProduceCloth(c.Villages[0].Number); err != nil {
					t.Fatal(err)
				}
				s.catanAfterProduction(make([]int, n))
				if g.Players[p].Score != g.victoryTargetFor(p) || s.Finished == aqueduct {
					t.Fatal("cloth points or response ordering", g.Players[p].Score, s.Phase)
				}
				if aqueduct {
					if err := s.catanCityChoice(p, Action{Type: "catan_aqueduct", Choice: "skip"}); err != nil {
						t.Fatal(err)
					}
					if !s.Finished {
						t.Fatal("did not settle after aqueduct")
					}
				}
			})
		}
	}
}

func TestCatanTwoSeaKnightsGoldShipContinuation(t *testing.T) {
	for _, mode := range []string{"paid", "free", "fish_before", "fish_between", "fish_after"} {
		t.Run(mode, func(t *testing.T) {
			fish := mode != "paid" && mode != "free"
			s, edge := prepareTwoSeaFogFixture(t, twoSeaKnightsGame(t, "fog", "fixed", fish, false), false, 0)
			g := s.Catan
			rolls := 2
			if mode == "fish_before" {
				rolls = 0
			}
			if mode == "fish_between" {
				rolls = 1
			}
			g.Two.Rolls = []int{4, 5}[:rolls]
			g.RollID = rolls
			g.CitiesKnights.EventDie = -1
			if rolls > 0 {
				g.CitiesKnights.EventDie = 0
			}
			resume := "catan_turn"
			if rolls < 2 {
				resume = "catan_roll"
			}
			s.Phase = resume
			a := Action{Type: "catan_ship", Edge: edge}
			if fish {
				a.Type = "catan_fish_ship"
				a.Tokens = g.fishPayment(0, g.fishActionCost(0, a.Type))
			} else if mode == "paid" {
				helperGrant(s, 0, catanPrices[a.Type])
			} else {
				s.Phase = "catan_roads"
				g.FreeRoads = 2
				g.ResumePhase = "catan_turn"
				resume = "catan_roads"
			}
			seq := g.Two.Sequence
			helperApply(t, s, 0, a)
			if s.Phase != "catan_gold" || s.Catan.Two.Pending != nil {
				t.Fatal("gold must precede neutral", s.Phase)
			}
			twoSeaRestore(t, s)
			helperApply(t, s, 0, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}})
			twoSeaRestore(t, s)
			if s.Phase != "catan_two_build" || s.Catan.Two.Sequence != seq+1 {
				t.Fatal("lost neutral ship", s.Phase)
			}
			a = twoSeaStep(t, s)
			twoSeaRestore(t, s)
			if s.Phase != resume || s.Catan.Two.Sequence != seq+1 || !s.Catan.Edges[a.Edge].Ship || len(s.Catan.Two.Rolls) != rolls {
				t.Fatal("wrong continuation", s.Phase)
			}
		})
	}
}
func TestCatanTwoSeaKnightsRejectsMarkers(t *testing.T) {
	for _, fish := range []bool{false, true} {
		s := twoSeaKnightsGame(t, "tribe", "fixed", fish, false)
		twoSeaRestore(t, s)
		for _, field := range []string{"sea", "knights", "combo", "target"} {
			q := clone(s)
			switch field {
			case "sea":
				q.Catan.Two.Seafarers = ""
			case "knights":
				q.Catan.Two.Knights = ""
			case "combo":
				q.Catan.Two.SeaKnights = "wrong"
			case "target":
				q.Catan.Seafarers.VictoryPoints--
			}
			if q.validateCatanTwo() == nil {
				t.Fatal("accepted corrupt marker", fish, field)
			}
		}
	}
}

func TestCatanTwoSeaKnightsDiplomacyShipRestore(t *testing.T) {
	s, edge := prepareTwoSeaFogFixture(t, twoSeaKnightsGame(t, "fog", "fixed", false, false), false, 0)
	g := s.Catan
	g.RollID = 2
	g.CitiesKnights.EventDie = 0
	helperGrant(s, 0, catanPrices["catan_ship"])
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	helperApply(t, s, 0, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}})
	twoSeaStep(t, s)
	if !slices.Contains(s.Catan.diplomacyRoads(), edge) {
		t.Fatal("ship not open")
	}
	seq := s.Catan.Two.Sequence
	ckProgressGive(t, s, 0, 16)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 16, Edge: edge})
	twoSeaRestore(t, s)
	if !s.Catan.CitiesKnights.Pending.Ship {
		t.Fatal("lost ship type")
	}
	helperReject(t, s, 1, Action{Type: "catan_diplomacy", Edge: edge})
	helperApply(t, s, 0, Action{Type: "catan_diplomacy", Edge: edge})
	twoSeaRestore(t, s)
	if s.Phase != "catan_turn" || !s.Catan.Edges[edge].Ship || s.Catan.Two.Pending != nil || s.Catan.Two.Sequence != seq {
		t.Fatal("diplomacy triggered neutral construction")
	}
}

func TestCatanTwoSeaKnightsVariableOpenings(t *testing.T) {
	for _, scene := range []string{"shores", "islands", "fog", "desert", "tribe", "cloth", "wonders"} {
		for _, fish := range []bool{false, true} {
			if fish && (scene == "desert" || scene == "tribe") {
				continue
			}
			t.Run(fmt.Sprintf("%s/fish%t", scene, fish), func(t *testing.T) {
				s := twoSeaKnightsGame(t, scene, "variable", fish, true)
				for step := 0; step < 400 && s.Catan.TurnSerial < 3; step++ {
					twoSeaStep(t, s)
					twoSeaRestore(t, s)
				}
				if s.Catan.TurnSerial < 3 || s.Catan.setup() {
					t.Fatal("variable opening stalled", s.Phase)
				}
			})
		}
	}
}
