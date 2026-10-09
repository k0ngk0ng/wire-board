package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishHelperGame(t *testing.T, n int, scene string, all bool) *State {
	t.Helper()
	opts := CatanOptions{FiveSix: n > 4, Helpers: true, AllHelpers: all}
	var s *State
	var err error
	switch scene {
	case "":
		s, err = NewCatanFishing(n, opts)
	case "new_world":
		var layout *CatanNewWorldMap
		layout, err = GenerateCatanFishingNewWorldMap(n)
		if err == nil {
			s, err = NewCatanFishingNewWorld(n, opts, layout)
		}
	default:
		s, err = NewCatanFishingSeafarers(n, opts, CatanSeafarersSetup{Scenario: scene}, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func fishHelperStep(t *testing.T, s *State) Action {
	t.Helper()
	p := s.CatanPendingActor()
	if p < 0 {
		p = s.Turn
	}
	if s.Phase == "catan_discard" {
		for i, due := range s.Catan.DiscardDue {
			if due > 0 {
				p = i
				break
			}
		}
	}
	a, err := s.BotAction(p)
	if err != nil {
		t.Fatal(s.Phase, err)
	}
	if err = s.Apply(p, a); err != nil {
		t.Fatal(s.Phase, a, err)
	}
	return a
}
func finishFishHelperSetup(t *testing.T, s *State) {
	t.Helper()
	for step := 0; step < 200 && s.Catan.setup(); step++ {
		fishHelperStep(t, s)
	}
	if s.Catan.setup() {
		t.Fatal("setup stalled", s.Phase)
	}
}
func TestCatanFishingHelpersConfiguration(t *testing.T) {
	for _, scene := range []string{"", "shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		for n := 3; n <= 6; n++ {
			for _, all := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/%t", scene, n, all), func(t *testing.T) {
					s := fishHelperGame(t, n, scene, all)
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
						raw, _ := json.Marshal(s)
						var copy State
						if err := json.Unmarshal(raw, &copy); err != nil {
							t.Fatal(err)
						}
						s = &copy
					}
					if err := s.validateFishingHelpers(); err != nil {
						t.Fatal(err)
					}
					if s.Catan.Fishing.Helpers != CatanFishingHelpersRules {
						t.Fatal("missing recipe")
					}
				})
			}
		}
	}
}
func TestCatanFishingHelpersNaturalGames(t *testing.T) {
	for i, scene := range []string{"", "shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		t.Run(scene, func(t *testing.T) {
			n := 3 + i%4
			s := fishHelperGame(t, n, scene, true)
			if i%2 == 0 {
				if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
			}
			responses, fish := 0, 0
			for step := 0; step < 16000 && !s.Finished; step++ {
				a := fishHelperStep(t, s)
				if a.Type == "catan_helper_choice" {
					responses++
				}
				if len(a.Type) > 11 && a.Type[:11] == "catan_fish_" {
					fish++
				}
				if step%43 == 0 {
					raw, _ := json.Marshal(s)
					var copy State
					if err := json.Unmarshal(raw, &copy); err != nil {
						t.Fatal(err)
					}
					s = &copy
				}
			}
			if !s.Finished || responses == 0 {
				t.Fatal("incomplete", s.Round, s.Phase, responses)
			}
			t.Log("finished", n, s.Round, "helpers", responses, "fish", fish)
		})
	}
}
func TestCatanFishingHelpersLakeAndDesertActions(t *testing.T) {
	for _, scene := range []string{"", "shores", "desert", "cloth"} {
		for _, id := range []int{10, 11} {
			t.Run(fmt.Sprintf("%s/%d", scene, id), func(t *testing.T) {
				s := fishHelperGame(t, 3, scene, true)
				finishFishHelperSetup(t, s)
				s.Phase = "catan_turn"
				p := s.Turn
				eventAssignHelper(t, s, p, id)
				g := s.Catan
				// Directly set a public robber location to isolate the helper's retreat.
				if len(g.Fishing.Map.Lakes) > 0 {
					g.Robber = g.Fishing.Map.Lakes[0].Tile
				} else {
					for _, tile := range g.Tiles {
						if tile.Resource >= 0 && tile.Resource < 5 && g.clothLand(tile.ID) {
							g.Robber = tile.ID
							break
						}
					}
				}
				source := g.Tiles[g.Robber].Resource
				color := source
				if color >= 5 {
					color = 0
				}
				before := g.Players[p].Resources[color]
				helperApply(t, s, p, Action{Type: "catan_helper", Color: color})
				g = s.Catan
				if g.Players[p].Resources[color] != before+1 || s.Phase != "catan_helper" {
					t.Fatal("missing reward")
				}
				if id == 10 {
					haveDesert := false
					for _, tile := range g.Tiles {
						haveDesert = haveDesert || tile.Resource == CatanDesert && g.clothLand(tile.ID)
					}
					if haveDesert && (g.Robber < 0 || g.Tiles[g.Robber].Resource != CatanDesert) || !haveDesert && g.Robber != -1 {
						t.Fatal("wrong retreat")
					}
				}
				helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
				if !s.Catan.Players[p].Helper.Moon {
					t.Fatal("no flip")
				}
				helperReject(t, s, p, Action{Type: "catan_helper", Color: color})
			})
		}
	}
}
func TestCatanFishingHelpersVersionIsolation(t *testing.T) {
	s := fishHelperGame(t, 3, "", true)
	finishFishHelperSetup(t, s)
	for _, mutate := range []func(*State){
		func(s *State) { s.Catan.Fishing.Helpers = "" },
		func(s *State) { s.Catan.Fishing.Helpers = "unknown" },
		func(s *State) { s.Catan.Options.Helpers = false },
		func(s *State) { s.Catan.HelperDisplay = append(s.Catan.HelperDisplay, s.Catan.Players[0].Helper.ID) },
	} {
		copy := clone(*s)
		mutate(&copy)
		helperReject(t, &copy, copy.Turn, Action{Type: "catan_roll"})
	}
	base := fishingGame(t, 3)
	if base.Catan.Fishing.Helpers != "" || len(base.Catan.HelperDisplay) > 0 {
		t.Fatal("disabled helper leak")
	}
	for _, p := range base.Catan.Players {
		if p.Helper != nil {
			t.Fatal("disabled helper leak")
		}
	}

}

func TestCatanFishingHelpersPrivacyAndFishPayments(t *testing.T) {
	for _, id := range []int{2, 6} {
		s := fishHelperGame(t, 3, "", true)
		finishFishHelperSetup(t, s)
		s.Phase = "catan_turn"
		p := s.Turn
		eventAssignHelper(t, s, p, id)
		s.Catan.Fishing.Tokens = *fishingTokens(t, 3)
		fishOwn(&s.Catan.Fishing.Tokens, p, 0, 1, 2, 3, 4, 5, 6)
		fishOwn(&s.Catan.Fishing.Tokens, (p+1)%3, 11)
		for viewer := -1; viewer < 3; viewer++ {
			v := s.View(viewer)["catan"].(map[string]any)["fishing"].(map[string]any)
			if v["helpers"] != CatanFishingHelpersRules {
				t.Fatal("missing public helper recipe")
			}
			raw, _ := json.Marshal(v)
			var public map[string]any
			_ = json.Unmarshal(raw, &public)
			tokens := public["tokens"].(map[string]any)
			if tokens["drawPile"] != nil || tokens["hands"] != nil || public["pending"] != nil {
				t.Fatal("private save exposed")
			}
			for seat, raw := range tokens["players"].([]any) {
				_, faces := raw.(map[string]any)["tokens"]
				if faces != (seat == viewer && len(s.Catan.Fishing.Tokens.Hands[seat]) > 0) {
					t.Fatal("fish faces exposed")
				}
			}
		}
		a := Action{Type: "catan_fish_dev"}
		if id == 2 {
			a.Type = "catan_fish_road"
			a.Edge = -1
			for _, e := range s.Catan.Edges {
				if s.Catan.canRoad(p, e.ID) {
					a.Edge = e.ID
					break
				}
			}
			if a.Edge < 0 {
				t.Fatal("no road fixture")
			}
		}
		a.Tokens = s.Catan.fishPayment(p, catanFishCosts[a.Type])
		bad := a
		bad.Skill = "helper"
		helperReject(t, s, p, bad)
		before := clone(s.Catan.Players[p].Helper)
		helperApply(t, s, p, a)
		if !reflect.DeepEqual(before, s.Catan.Players[p].Helper) || s.Catan.HelperPending != nil {
			t.Fatal("fish payment consumed helper")
		}
		if id == 6 {
			helperGrant(s, p, []int{1, 0, 1, 1, 0})
			helperApply(t, s, p, Action{Type: "catan_buy_dev", Skill: "helper", Tokens: []int{1, 0, 1, 1, 0}})
			for viewer := -1; viewer < 3; viewer++ {
				q := s.View(viewer)["catan"].(map[string]any)["helperPending"].(map[string]any)
				_, has := q["cards"]
				if has != (viewer == p) {
					t.Fatal("helper candidates privacy")
				}
			}
			restored := clone(*s)
			s = &restored
			helperApply(t, s, p, Action{Type: "catan_helper_choice", Card: s.Catan.HelperPending.Cards[0]})
			helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
		}
	}
}

func TestCatanFishingHelpersHildaAfterFish(t *testing.T) {
	for _, skip := range []bool{false, true} {
		s := fishHelperGame(t, 3, "", true)
		finishFishHelperSetup(t, s)
		p := (s.Turn + 1) % 3
		eventAssignHelper(t, s, p, 3)
		g := s.Catan
		// Isolate lake production from resource production without inventing fish.
		for i := range g.Vertices {
			g.Vertices[i].Owner = -1
			g.Vertices[i].Level = 0
		}
		// Hilda only pays out when this roll grants the player no ordinary
		// resource, so pin a lake number that no other tile at this corner
		// shares. The random map otherwise makes this assertion flaky.
		number, v := 0, -1
		for _, l := range g.Fishing.Map.Lakes {
			for _, candidate := range l.Numbers {
				for _, corner := range g.Tiles[l.Tile].Vertices {
					shared := false
					for i := range g.Tiles {
						if i != l.Tile && slices.Contains(g.Tiles[i].Vertices, corner) && g.Tiles[i].Number == candidate {
							shared = true
							break
						}
					}
					if !shared {
						number, v = candidate, corner
						break
					}
				}
				if v >= 0 {
					break
				}
			}
			if v >= 0 {
				break
			}
		}
		if v < 0 {
			t.Fatal("no isolated lake number on this map")
		}
		g.Vertices[v].Owner, g.Vertices[v].Level = p, 1
		g.Fishing.Tokens = *fishingTokens(t, 3)
		fishOwn(&g.Fishing.Tokens, p, 0, 1, 2, 3, 4, 5, 6)
		fishTop(&g.Fishing.Tokens, 21)
		g.Dice = []int{number / 2, number - number/2}
		g.RollID++
		if err := s.catanRollProduction(number); err != nil {
			t.Fatal(err)
		}
		if s.Phase != "catan_fish_replace" || g.HelperPending != nil {
			t.Fatal("fish must finish before Hilda")
		}
		helperApply(t, s, p, Action{Type: "catan_fish_keep"})
		if s.Catan.HelperPending == nil || s.Catan.HelperPending.Player != p || !s.Catan.HelperPending.Optional {
			t.Fatal("fish incorrectly suppressed Hilda")
		}
		restored := clone(*s)
		s = &restored
		if skip {
			helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "skip"})
			if !s.Catan.helperReady(p, 3) {
				t.Fatal("skip consumed helper")
			}
		} else {
			before := s.Catan.Players[p].Resources[0]
			helperApply(t, s, p, Action{Type: "catan_helper_choice", Color: 0})
			helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
			if s.Catan.Players[p].Resources[0] != before+1 {
				t.Fatal("missing compensation")
			}
		}
		if s.Phase != "catan_turn" {
			t.Fatal("production did not resume")
		}
	}
}
func TestCatanFishingHelpersThorolfFishExcluded(t *testing.T) {
	for _, held := range []int{7, 8} {
		s := fishHelperGame(t, 3, "", true)
		finishFishHelperSetup(t, s)
		p := (s.Turn + 1) % 3
		other := (p + 1) % 3
		eventAssignHelper(t, s, p, 5)
		g := s.Catan
		for i := range g.Players {
			catanMove(g.Players[i].Resources, g.Bank, slices.Clone(g.Players[i].Resources))
		}
		helperGrant(s, p, []int{held, 0, 0, 0, 0})
		helperGrant(s, other, []int{0, 8, 0, 0, 0})
		g.Fishing.Tokens = *fishingTokens(t, 3)
		fishOwn(&g.Fishing.Tokens, p, 0, 1, 2, 3, 4, 5, 6)
		g.Dice = []int{3, 4}
		g.RollID++
		if err := s.catanRollProduction(7); err != nil {
			t.Fatal(err)
		}
		if g.DiscardDue[p] != 0 || g.DiscardDue[other] != 4 || g.HelperPending == nil || g.HelperPending.Player != p {
			t.Fatal("wrong seven response")
		}
		helperReject(t, s, other, Action{Type: "catan_discard", Tokens: []int{0, 4, 0, 0, 0}})
		if held == 7 {
			helperApply(t, s, p, Action{Type: "catan_helper_choice", Color: 2})
		}
		helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
		if s.Phase != "catan_discard" || sum(s.Catan.Players[p].Resources) != 8 {
			t.Fatal("wrong hand count or resume")
		}
		helperApply(t, s, other, Action{Type: "catan_discard", Tokens: []int{0, 4, 0, 0, 0}})
		if s.Phase != "catan_robber" || len(s.Catan.Fishing.Tokens.Hands[p]) != 7 {
			t.Fatal("fish discarded or robber not resumed")
		}
	}
}

func TestCatanFishingHelpersOtherAbilitiesAndPairedTurn(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, id := range []int{1, 2, 4, 6, 7, 8, 9, 12} {
			t.Run(fmt.Sprintf("%d/%d", n, id), func(t *testing.T) {
				s := fishHelperGame(t, n, "", true)
				finishFishHelperSetup(t, s)
				s.Phase = "catan_turn"
				if n == 6 {
					helperApply(t, s, s.Turn, Action{Type: "catan_end"})
					if !s.Catan.Paired.Second {
						t.Fatal("not secondary turn")
					}
				}
				p := s.Turn
				other := (p + 1) % n
				eventAssignHelper(t, s, p, id)
				g := s.Catan
				for i := range g.Players {
					catanMove(g.Players[i].Resources, g.Bank, slices.Clone(g.Players[i].Resources))
				}
				fish := clone(g.Fishing.Tokens)
				roll := g.RollID
				switch id {
				case 1:
					helperGrant(s, p, []int{1, 0, 0, 0, 0})
					helperGrant(s, other, []int{0, 1, 0, 0, 0})
					helperApply(t, s, p, Action{Type: "catan_helper", Color: 1, Targets: []int{other}, Cards: []int{0}})
					if s.Catan.Players[p].Resources[1] != 1 || s.Catan.Players[other].Resources[0] != 1 {
						t.Fatal("trade failed")
					}
				case 2:
					edge := -1
					for _, e := range g.Edges {
						if g.canRoad(p, e.ID) {
							edge = e.ID
							break
						}
					}
					helperGrant(s, p, []int{1, 0, 1, 0, 0})
					helperApply(t, s, p, Action{Type: "catan_road", Edge: edge, Skill: "helper", Tokens: []int{1, 0, 1, 0, 0}})
					if s.Catan.Edges[edge].Owner != p {
						t.Fatal("road missing")
					}
				case 4:
					from, to := -1, -1
					for _, e := range g.Edges {
						if g.helperEndRoad(p, e.ID) {
							g.Edges[e.ID].Owner = -1
							for _, next := range g.Edges {
								if next.ID != e.ID && g.canRoad(p, next.ID) {
									from, to = e.ID, next.ID
									break
								}
							}
							g.Edges[e.ID].Owner = p
							if from >= 0 {
								break
							}
						}
					}
					if from < 0 {
						t.Fatal("no end road fixture")
					}
					helperApply(t, s, p, Action{Type: "catan_helper", Edge: from, Target: to})
					if s.Catan.Edges[from].Owner != -1 || s.Catan.Edges[to].Owner != p {
						t.Fatal("road not moved")
					}
				case 6:
					helperGrant(s, p, []int{1, 0, 1, 1, 0})
					helperApply(t, s, p, Action{Type: "catan_buy_dev", Skill: "helper", Tokens: []int{1, 0, 1, 1, 0}})
					if len(s.Catan.HelperPending.Cards) != 3 {
						t.Fatal("missing development candidates")
					}
					card := s.Catan.HelperPending.Cards[0]
					helperApply(t, s, p, Action{Type: "catan_helper_choice", Card: card})
					if s.Catan.Players[p].NewDev[card] != 1 {
						t.Fatal("new development not locked")
					}
				case 7:
					g.Players[other].Score = g.Players[p].Score + 1
					helperGrant(s, other, []int{0, 0, 0, 0, 1})
					helperApply(t, s, p, Action{Type: "catan_helper", Target: other})
					for viewer := -1; viewer < n; viewer++ {
						q := s.View(viewer)["catan"].(map[string]any)["helperPending"].(map[string]any)
						if (q["resources"] != nil) != (viewer == p) {
							t.Fatal("leader hand privacy")
						}
					}
					helperApply(t, s, p, Action{Type: "catan_helper_choice", Color: 4})
					if s.Catan.Players[p].Resources[4] != 1 || s.Catan.Players[other].Resources[4] != 0 {
						t.Fatal("leader theft failed")
					}
				case 8:
					vertex := -1
					for _, v := range g.Vertices {
						if v.Owner == p && v.Level == 1 {
							vertex = v.ID
							break
						}
					}
					// Move three real knights from the deck into the played stack.
					for i := 0; i < 3; i++ {
						at := slices.Index(g.DevDeck, 0)
						g.DevDeck = slices.Delete(g.DevDeck, at, at+1)
						g.DevDiscard = append(g.DevDiscard, 0)
					}
					g.Players[p].Knights = 3
					g.ArmyOwner = p
					helperGrant(s, p, []int{0, 0, 0, 1, 2})
					helperApply(t, s, p, Action{Type: "catan_city", Vertex: vertex, Skill: "helper"})
					if s.Catan.Vertices[vertex].Level != 2 || s.Catan.Players[p].Knights != 2 || len(s.Catan.HelperExile) != 1 || s.Catan.ArmyOwner != -1 {
						t.Fatal("knight construction failed")
					}
				case 9:
					helperGrant(s, p, []int{4, 0, 0, 0, 0})
					helperApply(t, s, p, Action{Type: "catan_helper", Give: []int{4, 0, 0, 0, 0}, Take: []int{0, 0, 1, 1, 0}})
					if !slices.Equal(s.Catan.Players[p].Resources, []int{0, 0, 1, 1, 0}) {
						t.Fatal("bank trade failed")
					}
				case 12:
					card := g.DevDeck[0]
					g.DevDeck = g.DevDeck[1:]
					g.Players[p].Dev[card]++
					next := g.DevDeck[len(g.DevDeck)-1]
					helperApply(t, s, p, Action{Type: "catan_helper", Card: card})
					if s.Catan.DevDeck[0] != card || s.Catan.Players[p].NewDev[next] != 1 {
						t.Fatal("development exchange failed")
					}
				}
				helperApply(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
				if s.Phase != "catan_turn" || !s.Catan.Players[p].Helper.Moon || !reflect.DeepEqual(fish, s.Catan.Fishing.Tokens) || s.Catan.RollID != roll {
					t.Fatal("helper changed fish, turn, or production")
				}
			})
		}
	}
}
func TestCatanFishingHelpersEliminationAndBotPrivacy(t *testing.T) {
	s := fishHelperGame(t, 4, "", true)
	finishFishHelperSetup(t, s)
	s.Phase = "catan_turn"
	p := s.Turn
	other := (p + 1) % 4
	s.Catan.Fishing.Tokens = *fishingTokens(t, 4)
	fishOwn(&s.Catan.Fishing.Tokens, p, 0, 1, 2)
	fishOwn(&s.Catan.Fishing.Tokens, other, 11)
	before, err := s.BotAction(p)
	if err != nil {
		t.Fatal(err)
	}
	changed := clone(*s)
	f := &changed.Catan.Fishing.Tokens
	slices.Reverse(f.DrawPile)
	f.Hands[other][0], f.DrawPile[0] = f.DrawPile[0], f.Hands[other][0]
	after, err := changed.BotAction(p)
	if err != nil || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(s.View(p), changed.View(p)) {
		t.Fatal("bot knows hidden fish", err)
	}
	id := s.Catan.Players[other].Helper.ID
	s.Turn = other
	if err := s.EliminateCatan(other); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Players[other].Helper != nil || !slices.Contains(s.Catan.HelperDisplay, id) || len(s.Catan.Fishing.Tokens.Hands[other]) != 0 {
		t.Fatal("leaving player retained pieces")
	}
	if err := s.validateFishingHelpers(); err != nil {
		t.Fatal(err)
	}
}
