package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func twoHelperGame(t *testing.T, fishing, all bool) *State {
	t.Helper()
	build := NewCatanTwo
	if fishing {
		build = NewCatanTwoFishing
	}
	s, err := build(2, CatanOptions{Helpers: true, AllHelpers: all})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func twoHelperReady(t *testing.T, id int) *State {
	t.Helper()
	s := twoHelperGame(t, false, true)
	finishFishHelperSetup(t, s)
	s.Phase = "catan_turn"
	s.Catan.Two.Rolls = []int{6, 8}
	eventAssignHelper(t, s, s.Turn, id)
	return s
}
func TestCatanTwoHelpersRecipes(t *testing.T) {
	for _, f := range []bool{false, true} {
		for _, all := range []bool{false, true} {
			for v := range 4 {
				for _, events := range []bool{false, true} {
					t.Run(fmt.Sprintf("fish=%t/all=%t/variants=%d/events=%t", f, all, v, events), func(t *testing.T) {
						s := twoHelperGame(t, f, all)
						if err := s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: v&1 != 0}); err != nil {
							t.Fatal(err)
						}
						if err := s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: v&2 != 0}); err != nil {
							t.Fatal(err)
						}
						if events {
							if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
								t.Fatal(err)
							}
						}
						finishFishHelperSetup(t, s)
						for p, h := range s.Catan.Players {
							if h.Helper.ID != (p-s.Catan.StartPlayer+2)%2+1 {
								t.Fatal("wrong starting helper")
							}
						}
						for range 35 {
							fishHelperStep(t, s)
							twoCoreRestore(t, s)
						}
						if s.Catan.Two.Helpers != CatanTwoHelpersRules {
							t.Fatal("missing supplement")
						}
					})
				}
			}
		}
	}
}
func TestCatanTwoHelpersBuildResponseChain(t *testing.T) {
	for _, kind := range []string{"road", "settlement", "city"} {
		t.Run(kind, func(t *testing.T) {
			id := 8
			if kind == "road" {
				id = 2
			}
			s := twoHelperReady(t, id)
			g, p := s.Catan, s.Turn
			a := Action{Type: "catan_" + kind, Skill: "helper", Edge: -1, Vertex: -1}
			if kind == "road" {
				for _, e := range g.Edges {
					if g.canRoad(p, e.ID) {
						a.Edge = e.ID
						break
					}
				}
				a.Tokens = []int{1, 0, 1, 0, 0}
				helperGrant(s, p, a.Tokens)
			} else {
				// Extend an actual legal own route until it reaches a buildable village.
				if kind == "settlement" {
					for attempts := 0; attempts < 12 && a.Vertex < 0; attempts++ {
						for _, v := range g.Vertices {
							if g.canSettlement(p, v.ID, false) {
								a.Vertex = v.ID
								break
							}
						}
						if a.Vertex >= 0 {
							break
						}
						for _, e := range g.Edges {
							if g.canRoad(p, e.ID) {
								g.Edges[e.ID].Owner = p
								break
							}
						}
					}
				} else {
					for _, v := range g.Vertices {
						if v.Owner == p && v.Level == 1 {
							a.Vertex = v.ID
							break
						}
					}
				}
				if a.Vertex < 0 {
					t.Fatal("no fixture build site")
				}
				i := slices.Index(g.DevDeck, 0)
				g.DevDeck = slices.Delete(g.DevDeck, i, i+1)
				g.DevDiscard = append(g.DevDiscard, 0)
				g.Players[p].Knights = 1
				if kind == "city" {
					helperGrant(s, p, []int{0, 0, 0, 1, 2})
				} else {
					helperGrant(s, p, []int{1, 1, 0, 0, 0})
				}
			}
			seq, tokens := g.Two.Sequence, g.Two.Tokens[p]
			helperApply(t, s, p, a)
			g = s.Catan
			if g.HelperPending == nil || g.Two.Pending != nil || (kind != "city" && g.Two.AfterHelper != kind) || (kind == "city" && g.Two.AfterHelper != "") {
				t.Fatal("overlapping or missing chain")
			}
			if kind != "road" && (g.Players[p].Knights != 0 || len(g.HelperExile) != 1) {
				t.Fatal("knight not exiled")
			}
			earned := g.Two.Tokens[p] - tokens
			raw, _ := json.Marshal(s)
			var restored State
			if err := json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			s = &restored
			helperReject(t, s, 1-p, Action{Type: "catan_helper_choice", Choice: "flip"})
			helperReject(t, s, p, Action{Type: "catan_two_build", Target: 0})
			helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
			if kind == "city" {
				if s.Catan.Two.Pending != nil || s.Phase != "catan_turn" {
					t.Fatal("city triggered neutral")
				}
				return
			}
			if s.Catan.Two.Pending == nil || s.Catan.Two.Pending.Kind != kind || s.Catan.Two.Sequence != seq+1 || s.Catan.Two.AfterHelper != "" {
				t.Fatal("neutral build not resumed")
			}
			fishHelperStep(t, s)
			if s.Phase != "catan_turn" || s.Catan.Two.Tokens[p] != tokens+earned {
				t.Fatal("wrong resume or duplicate tokens")
			}
		})
	}
}
func TestCatanTwoHelpersProductionOncePerTurn(t *testing.T) {
	for _, skip := range []bool{false, true} {
		t.Run(fmt.Sprint(skip), func(t *testing.T) {
			s := twoHelperReady(t, 3)
			g, p := s.Catan, s.Turn
			s.Phase = "catan_roll"
			g.Two.Rolls = nil
			// Zero production tiles isolate Hilda without changing buildings or supplies.
			for i := range g.Tiles {
				g.Tiles[i].Number = 0
			}
			if err := s.catanTwoRoll(2, 2); err != nil {
				t.Fatal(err)
			}
			if g.HelperPending == nil || g.HelperPending.Player != p {
				t.Fatal("first production compensation missing")
			}
			a := Action{Type: "catan_helper_choice", Color: 0}
			if skip {
				a.Choice = "skip"
			}
			helperApply(t, s, p, a)
			if !skip {
				helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
			}
			if s.Phase != "catan_roll" {
				t.Fatal("did not resume second production")
			}
			if err := s.catanTwoRoll(2, 3); err != nil {
				t.Fatal(err)
			}
			if (s.Catan.HelperPending != nil) != skip {
				t.Fatal("used helper repeated, or skip consumed it")
			}
		})
	}
}
func TestCatanTwoHelpersTradeAndRelocation(t *testing.T) {
	s := twoHelperReady(t, 1)
	p := s.Turn
	other := 1 - p
	helperGrant(s, other, []int{0, 0, 0, 0, 3})
	helperGrant(s, p, []int{2, 0, 0, 0, 0})
	helperReject(t, s, p, Action{Type: "catan_helper", Color: 4, Targets: []int{other, other}, Cards: []int{0, 0}})
	helperReject(t, s, p, Action{Type: "catan_helper", Color: 4, Targets: []int{-2}, Cards: []int{0}})
	tokens := s.Catan.Two.Tokens[p]
	helperApply(t, s, p, Action{Type: "catan_helper", Color: 4, Targets: []int{other}, Cards: []int{0}})
	helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "exchange", Card: 4})
	if s.Catan.Two.Spent || s.Catan.Two.Tokens[p] != tokens {
		t.Fatal("helper consumed trade-token action")
	}
	helperReject(t, s, p, Action{Type: "catan_helper"})
	s.Catan.TurnSerial++
	g := s.Catan
	// Isolate relocation from random setup roads that can join both villages.
	for i := range g.Edges {
		if g.Edges[i].Owner == p {
			g.Edges[i].Owner = -1
		}
	}
	for _, e := range g.Edges {
		if g.canRoad(p, e.ID) {
			g.Edges[e.ID].Owner = p
			break
		}
	}
	for _, e := range g.Edges {
		if g.helperEndRoad(p, e.ID) {
			trial := clone(*g)
			trial.Edges[e.ID].Owner = -1
			for _, to := range trial.Edges {
				if to.ID != e.ID && trial.canRoad(p, to.ID) {
					helperApply(t, s, p, Action{Type: "catan_helper", Edge: e.ID, Target: to.ID})
					if s.Catan.Two.AfterHelper != "" || s.Catan.Two.Pending != nil {
						t.Fatal("relocation built neutral road")
					}
					helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
					return
				}
			}
		}
	}
	t.Fatal("no relocation fixture")
}
func TestCatanTwoHelpersRejectCorruptAndUnmarked(t *testing.T) {
	base := twoHelperReady(t, 2)
	for name, mutate := range map[string]func(*State){
		"marker":    func(s *State) { s.Catan.Two.Helpers = "" },
		"version":   func(s *State) { s.Catan.Two.Helpers = "unknown" },
		"options":   func(s *State) { s.Catan.Options.FiveSix = true },
		"missing":   func(s *State) { s.Catan.Players[0].Helper = nil },
		"duplicate": func(s *State) { s.Catan.HelperDisplay = append(s.Catan.HelperDisplay, s.Catan.Players[0].Helper.ID) },
		"deferred":  func(s *State) { s.Catan.Two.AfterHelper = "road" },
	} {
		t.Run(name, func(t *testing.T) {
			s := clone(*base)
			mutate(&s)
			if s.validateCatanTwo() == nil {
				t.Fatal("accepted corrupt save")
			}
		})
	}
	for _, build := range []func(int, CatanOptions) (*State, error){NewCatanTwoRivers, NewCatanTwoCaravans} {
		if _, err := build(2, CatanOptions{Helpers: true}); err == nil {
			t.Fatal("unverified combination opened")
		}
	}
	s, err := NewCatanTwo(2, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Catan.Options != (CatanOptions{}) || s.Catan.Two.Helpers != "" || len(s.Catan.HelperDisplay) != 0 {
		t.Fatal("base isolation")
	}
}

func TestCatanTwoHelpersRemainingAbilitiesAndPrivacy(t *testing.T) {
	for _, fish := range []bool{false, true} {
		for _, id := range []int{5, 6, 7, 9, 10, 11, 12} {
			t.Run(fmt.Sprintf("fish=%t/helper=%d", fish, id), func(t *testing.T) {
				s := twoHelperGame(t, fish, true)
				finishFishHelperSetup(t, s)
				s.Phase = "catan_turn"
				s.Catan.Two.Rolls = []int{6, 8}
				p := s.Turn
				other := 1 - p
				eventAssignHelper(t, s, p, id)
				g := s.Catan
				for i := range g.Players {
					catanMove(g.Players[i].Resources, g.Bank, slices.Clone(g.Players[i].Resources))
				}
				switch id {
				case 5:
					s.Phase = "catan_roll"
					g.Two.Rolls = nil
					helperGrant(s, other, []int{8, 0, 0, 0, 0})
					if err := s.catanTwoRoll(3, 4); err != nil {
						t.Fatal(err)
					}
					if g.HelperPending == nil || g.HelperPending.Player != p || g.DiscardDue[other] != 4 {
						t.Fatal("seven protection sequence")
					}
					helperApply(t, s, p, Action{Type: "catan_helper_choice", Color: 1})
				case 6:
					helperGrant(s, p, []int{1, 0, 1, 1, 0})
					helperApply(t, s, p, Action{Type: "catan_buy_dev", Skill: "helper", Tokens: []int{1, 0, 1, 1, 0}})
					for viewer := -1; viewer < 2; viewer++ {
						q := s.View(viewer)["catan"].(map[string]any)["helperPending"].(map[string]any)
						if (q["cards"] != nil) != (viewer == p) {
							t.Fatal("private candidates leaked")
						}
					}
					twoCoreRestore(t, s)
					card := s.Catan.HelperPending.Cards[0]
					helperApply(t, s, p, Action{Type: "catan_helper_choice", Card: card})
					if s.Catan.Players[p].NewDev[card] != 1 {
						t.Fatal("new development unlocked")
					}
				case 7:
					// Upgrade an existing opposing village so the lead is public and real.
					for i, v := range g.Vertices {
						if v.Owner == other {
							g.Vertices[i].Level = 2
							break
						}
					}
					s.catanScores()
					helperGrant(s, other, []int{0, 0, 0, 0, 1})
					helperApply(t, s, p, Action{Type: "catan_helper", Target: other})
					for viewer := -1; viewer < 2; viewer++ {
						q := s.View(viewer)["catan"].(map[string]any)["helperPending"].(map[string]any)
						if (q["resources"] != nil) != (viewer == p) {
							t.Fatal("leader hand leaked")
						}
					}
					twoCoreRestore(t, s)
					helperApply(t, s, p, Action{Type: "catan_helper_choice", Color: 4})
					if s.Catan.Players[p].Resources[4] != 1 {
						t.Fatal("leader resource missing")
					}
				case 9:
					helperGrant(s, p, []int{4, 0, 0, 0, 0})
					helperApply(t, s, p, Action{Type: "catan_helper", Give: []int{4, 0, 0, 0, 0}, Take: []int{0, 0, 1, 1, 0}})
					if !slices.Equal(s.Catan.Players[p].Resources, []int{0, 0, 1, 1, 0}) {
						t.Fatal("2:1 trade failed")
					}
				case 10, 11:
					if fish {
						g.Robber = g.Fishing.Map.Lakes[0].Tile
					} else {
						for _, tile := range g.Tiles {
							if tile.Resource == 0 {
								g.Robber = tile.ID
								break
							}
						}
					}
					if id == 10 {
						s.Phase = "catan_roll"
						g.Two.Rolls = []int{6}
					}
					helperApply(t, s, p, Action{Type: "catan_helper", Color: 0})
					if s.Catan.Players[p].Resources[0] != 1 {
						t.Fatal("robber helper reward")
					}
					if id == 10 && (fish && s.Catan.Robber != -1 || !fish && s.Catan.Tiles[s.Catan.Robber].Resource != CatanDesert) {
						t.Fatal("wrong retreat")
					}
				case 12:
					card := g.DevDeck[0]
					g.DevDeck = g.DevDeck[1:]
					g.Players[p].Dev[card]++
					top := g.DevDeck[len(g.DevDeck)-1]
					helperApply(t, s, p, Action{Type: "catan_helper", Card: card})
					if s.Catan.DevDeck[0] != card || s.Catan.Players[p].NewDev[top] != 1 {
						t.Fatal("card swap timing")
					}
				}
				helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
				if id == 5 {
					for step := 0; step < 12 && s.Phase != "catan_roll"; step++ {
						fishHelperStep(t, s)
					}
					if s.Phase != "catan_roll" {
						t.Fatal("seven did not resume second production")
					}
				}
				if id == 10 && (s.Phase != "catan_roll" || len(s.Catan.Two.Rolls) != 1) {
					t.Fatal("Digur consumed or skipped second production")
				}
				if s.Catan.helperReady(p, id) {
					t.Fatal("helper twice in real turn")
				}
			})
		}
	}
}
