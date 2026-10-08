package game

import (
	"fmt"
	"slices"
	"testing"
)

func eventVariantGame(t *testing.T, n int, scenario, mode string, friendly, harbors bool) *State {
	t.Helper()
	options := CatanOptions{FiveSix: n > 4, Helpers: mode == "helpers", AllHelpers: mode == "helpers"}
	var s *State
	var err error
	if scenario == "" {
		if mode == "knights" {
			s, err = NewCatanCitiesKnights(n, options)
		} else {
			s, err = NewCatan(n, options)
		}
	} else {
		var world *CatanNewWorldMap
		if scenario == "new_world" {
			world, err = GenerateCatanNewWorldMap(n)
			if err != nil {
				t.Fatal(err)
			}
		}
		setup := CatanSeafarersSetup{Scenario: scenario}
		if mode == "knights" {
			s, err = NewCatanCitiesKnightsSeafarers(n, options, setup, world)
		} else {
			s, err = NewCatanSeafarers(n, options, setup, world)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if friendly {
		if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	if harbors {
		if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	for step := 0; s.Catan.setup() && step < 100; step++ {
		p := ckActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		helperApply(t, s, p, a)
	}
	if s.Catan.setup() {
		t.Fatal("opening stalled")
	}
	return referenceEventRestore(t, s)
}

func TestCatanEventVariantsMapsAndFaces(t *testing.T) {
	for _, scene := range []string{"", "shores", "islands", "fog", "desert", "cloth", "wonders", "new_world"} {
		for n := 3; n <= 6; n++ {
			for _, mode := range []string{"ordinary", "helpers", "knights"} {
				for variant := 1; variant <= 3; variant++ {
					t.Run(fmt.Sprintf("%s/%d/%s/%d", scene, n, mode, variant), func(t *testing.T) {
						seed := eventVariantGame(t, n, scene, mode, variant&1 != 0, variant&2 != 0)
						for kind := range catanCardEventNames {
							copy := clone(*seed)
							s := &copy
							referenceEventTop(t, s, kind)
							if err := s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
								t.Fatal(kind, err)
							}
							s = finishEventKnightsResponses(t, s)
							g := s.Catan
							if g.RollID != 1 || !g.RevealedEvent.ProductionStarted || len(g.EventDeck.Deck.Discard) != 1 {
								t.Fatal(kind, "production/deck")
							}
							if (g.FriendlyRobber != nil) != (variant&1 != 0) || (g.Harbors != nil) != (variant&2 != 0) {
								t.Fatal("lost variants")
							}
							if mode == "knights" {
								ckProgressStock(t, g)
							} else {
								seaEventConserved(t, s)
							}
							if n > 4 {
								helperApply(t, s, s.Turn, Action{Type: "catan_end"})
								if !s.Catan.Paired.Second || s.Catan.RollID != 1 {
									t.Fatal("paired secondary drew")
								}
							}
						}
					})
				}
			}
		}
	}
}

func TestCatanEventVariantsSevenFleeAndCardTheft(t *testing.T) {
	for _, kind := range []string{"robber_attacks", "robber_flees", "conflict", "trade_advantage"} {
		t.Run(kind, func(t *testing.T) {
			s := eventVariantGame(t, 3, "", "ordinary", true, true)
			g := s.Catan
			// Isolate actual card handling with one protected target holding a resource.
			for p := range g.Players {
				catanMove(g.Players[p].Resources, g.Bank, slices.Clone(g.Players[p].Resources))
			}
			target := (s.Turn + 1) % 3
			helperGrant(s, target, []int{1, 0, 0, 0, 0})
			if !g.friendlyProtected(target) {
				t.Fatal("target must start protected")
			}
			if kind == "conflict" {
				g.ArmyOwner = s.Turn
				g.Players[s.Turn].Knights = 3
			}
			if kind == "trade_advantage" {
				g.LongestOwner = s.Turn
			}
			robber := g.Robber
			referenceEventTop(t, s, kind)
			helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
			g = s.Catan
			if kind == "conflict" || kind == "trade_advantage" {
				if !slices.Contains(g.cardTheftTargets(s.Turn), target) || s.Phase != "catan_card_event" {
					t.Fatal("event theft wrongly protected")
				}
				helperApply(t, s, s.Turn, Action{Type: "catan_event_steal", Target: target})
				if s.Catan.Robber != robber {
					t.Fatal("card effect moved robber")
				}
			} else if kind == "robber_flees" {
				s = finishEventKnightsResponses(t, s)
				if s.Catan.Robber < 0 || s.Catan.Tiles[s.Catan.Robber].Resource != CatanDesert || len(s.Catan.Victims) > 0 {
					t.Fatal("flee stole or ignored desert")
				}
			} else {
				if s.Phase != "catan_robber" {
					t.Fatal("seven must enter robber", s.Phase)
				}
				for _, tile := range g.Tiles {
					if g.robberAllowed(tile.ID) && tile.Resource != CatanDesert && g.friendlyRobberBlocks(tile.ID) {
						t.Fatal("protected land allowed")
					}
				}
				s = finishEventKnightsResponses(t, s)
				if s.Catan.Players[target].Resources[0] != 1 {
					t.Fatal("seven stole protected hand")
				}
			}
			referenceEventRestore(t, s)
		})
	}
}

func TestCatanEventVariantsRejectCorruptAndDetached(t *testing.T) {
	seed := eventVariantGame(t, 3, "cloth", "ordinary", true, true)
	for name, mutate := range map[string]func(*Catan){
		"harbor-rule":       func(g *Catan) { g.Harbors.Rules = "unknown" },
		"harbor-owner":      func(g *Catan) { g.Harbors.Owner = 3 },
		"friendly-rule":     func(g *Catan) { g.FriendlyRobber.Rules = "unknown" },
		"friendly-fallback": func(g *Catan) { g.FriendlyRobber.Fallback = "unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			copy := clone(*seed)
			s := &copy
			mutate(s.Catan)
			if s.validateCatanEventSession() == nil {
				t.Fatal("corrupt variant accepted")
			}
			helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
		})
	}
	for _, s := range []*State{func() *State {
		s, e := newCatanTwoReferenceEvents()
		if e != nil {
			t.Fatal(e)
		}
		return s
	}(), func() *State {
		s, e := newCatanRiversReferenceEvents(3)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}()} {
		s.enableCatanHarbors()
		if s.validateCatanEventSession() == nil {
			t.Fatal("unimplemented scenario triple accepted")
		}
	}
}

func TestCatanEventVariantsPillageUpdatesHarborAndProtectionBeforeSeven(t *testing.T) {
	s := eventVariantGame(t, 3, "", "knights", true, true)
	g := s.Catan
	// Explicit board fixture: one player's sole city and village hold the award.
	harborBuildings(g, []int{3, 0, 0})
	city := -1
	for _, v := range g.Vertices {
		if v.Owner == 0 && v.Level == 2 {
			city = v.ID
		}
	}
	if city < 0 {
		t.Fatal("fixture city")
	}
	for p := range g.Players {
		catanMove(g.Players[p].Resources, g.Bank, slices.Clone(g.Players[p].Resources))
	}
	helperGrant(s, 0, []int{8, 0, 0, 0, 0, 0, 0, 0})
	g.CitiesKnights.BarbarianPosition = 6
	s.catanScores()
	if g.Harbors.Owner != 0 || g.friendlyProtected(0) || g.Players[0].Score != 5 {
		t.Fatal("initial award/protection")
	}
	referenceEventTop(t, s, "robber_attacks")
	// Server random callback yields barbarian on its second call.
	call := 0
	if err := s.catanDrawEventRandom(func(int) int {
		call++
		if call == 1 {
			return 0
		}
		return 3
	}); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_pillage" {
		t.Fatal("must pillage before seven", s.Phase)
	}
	s = referenceEventRestore(t, s)
	helperApply(t, s, 0, Action{Type: "catan_pillage", Vertex: city})
	g = s.Catan
	if g.Harbors.Owner != -1 || g.Players[0].Score != 2 || !g.friendlyProtected(0) || s.Phase != "catan_discard" || g.DiscardDue[0] != 4 {
		t.Fatal("pillage award/protection/discard order", s.Phase, g.Players[0].Score)
	}
	s = finishEventKnightsResponses(t, s)
	if sum(s.Catan.Players[0].Resources) != 4 {
		t.Fatal("seven stole newly protected hand")
	}
	if s.Catan.victoryTarget() != 14 {
		t.Fatal("harbor target increment lost")
	}
	referenceEventRestore(t, s)
}
