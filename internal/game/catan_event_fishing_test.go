package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func eventFishingGame(t *testing.T, n int, scene string, knights, variants bool) *State {
	t.Helper()
	options := CatanOptions{FiveSix: n > 4}
	var s *State
	var err error
	switch {
	case knights:
		s, err = NewCatanFishingCitiesKnights(n, options)
	case scene == "":
		s, err = NewCatanFishing(n, options)
	case scene == "new_world":
		world, e := GenerateCatanFishingNewWorldMap(n)
		if e != nil {
			t.Fatal(e)
		}
		s, err = NewCatanFishingNewWorld(n, options, world)
	default:
		s, err = NewCatanFishingSeafarers(n, options, CatanSeafarersSetup{Scenario: scene}, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	if variants {
		if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
			t.Fatal(err)
		}
		if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	for step := 0; s.Catan.setup() && step < 150; step++ {
		s = referenceEventRestore(t, s)
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

func TestCatanEventFishingAllFaces(t *testing.T) {
	for _, scene := range []string{"", "knights", "islands", "fog", "desert", "cloth", "wonders", "new_world"} {
		for n := 3; n <= 6; n++ {
			if n > 4 && (scene == "islands" || scene == "desert" || scene == "cloth") {
				continue
			}
			for _, variants := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/variants=%t", scene, n, variants), func(t *testing.T) {
					seed := eventFishingGame(t, n, scene, scene == "knights", variants)
					for kind := range catanCardEventNames {
						copy := clone(*seed)
						s := &copy
						referenceEventTop(t, s, kind)
						if err := s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
							t.Fatal(kind, err)
						}
						s = finishEventKnightsResponses(t, s)
						if s.Catan.RollID != 1 || s.Catan.Fishing.LastRollID != 1 || !s.Catan.RevealedEvent.ProductionStarted {
							t.Fatal("missing production", kind)
						}
						if err := s.Catan.validateFishing(); err != nil {
							t.Fatal(err)
						}
						if n > 4 {
							before := clone(s.Catan.Fishing.Tokens)
							helperApply(t, s, s.Turn, Action{Type: "catan_end"})
							if !s.Catan.Paired.Second || !reflect.DeepEqual(before, s.Catan.Fishing.Tokens) {
								t.Fatal("secondary reproduced fish")
							}
						}
					}
				})
			}
		}
	}
}

func TestCatanEventFishingEpidemicResourceAndFullFish(t *testing.T) {
	for _, knights := range []bool{false, true} {
		t.Run(fmt.Sprint(knights), func(t *testing.T) {
			s := eventFishingGame(t, 3, "", knights, false)
			g := s.Catan
			for i := range g.Vertices {
				g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
			}
			for p := range g.Players {
				catanMove(g.Players[p].Resources, g.Bank, slices.Clone(g.Players[p].Resources))
			}
			atGround := slices.IndexFunc(g.Fishing.Map.Grounds, func(v catanFishingGround) bool { return v.Number == 6 })
			ground := &g.Fishing.Map.Grounds[atGround]
			vertex := ground.Vertices[1]
			g.Vertices[vertex].Owner, g.Vertices[vertex].Level = 0, 2
			tile := -1
			for i := range g.Tiles {
				if g.Tiles[i].Resource < 5 {
					g.Tiles[i].Number = 5
					if slices.Contains(g.Tiles[i].Vertices, vertex) {
						tile = i
					}
				}
			}
			if tile < 0 {
				t.Fatal("coastal city must adjoin resource")
			}
			g.Tiles[tile].Resource, g.Tiles[tile].Number = 0, 6
			g.Robber = -1
			// Explicit midgame fixture: pool the exact physical token inventory.
			g.Fishing.Tokens = *fishingTokens(t, 3)
			fishTop(&g.Fishing.Tokens, 21, 22)
			d := &g.EventDeck.Deck
			at := slices.Index(d.DrawPile, 10)
			if at < 0 {
				t.Fatal("epidemic six missing")
			}
			d.DrawPile[at], d.DrawPile[len(d.DrawPile)-1] = d.DrawPile[len(d.DrawPile)-1], d.DrawPile[at]
			if err := s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
				t.Fatal(err)
			}
			s = finishEventKnightsResponses(t, s)
			g = s.Catan
			if g.Players[0].Resources[0] != 1 || sum(g.Players[0].Resources) != 1 || len(g.Fishing.Tokens.Hands[0]) != 2 {
				t.Fatal("epidemic must reduce resources only", g.Players[0].Resources, g.Fishing.Tokens.Hands[0])
			}
		})
	}
}

func TestCatanEventFishingReplacementThenAqueductAndAlchemy(t *testing.T) {
	for _, alchemy := range []bool{false, true} {
		t.Run(fmt.Sprint(alchemy), func(t *testing.T) {
			s := eventFishingGame(t, 3, "", true, false)
			g := s.Catan
			for i := range g.Vertices {
				g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
			}
			lake := g.Fishing.Map.Lakes[0].Tile
			g.Vertices[g.Tiles[lake].Vertices[0]].Owner, g.Vertices[g.Tiles[lake].Vertices[0]].Level = 1, 2
			for p := range g.Players {
				catanMove(g.Players[p].Resources, g.Bank, slices.Clone(g.Players[p].Resources))
			}
			g.CitiesKnights.Players[1].Improvements[CatanScience] = 3
			g.Fishing.Tokens = *fishingTokens(t, 3)
			fishOwn(&g.Fishing.Tokens, 1, 0, 1, 2, 3, 4, 5, 6)
			fishTop(&g.Fishing.Tokens, 21, 22)
			deck := clone(g.EventDeck.Deck)
			if alchemy {
				ckProgressGive(t, s, s.Turn, 0)
				helperApply(t, s, s.Turn, Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}})
			} else {
				// Three-point Conflict: no active knights, no reward; lake produces two fish.
				d := &g.EventDeck.Deck
				at := slices.Index(d.DrawPile, 1)
				d.DrawPile[at], d.DrawPile[len(d.DrawPile)-1] = d.DrawPile[len(d.DrawPile)-1], d.DrawPile[at]
				if err := s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
					t.Fatal(err)
				}
			}
			if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != 1 || s.Catan.CitiesKnights.Pending != nil {
				t.Fatal("wrong first response", s.Phase)
			}
			s = referenceEventRestore(t, s)
			helperReject(t, s, 0, Action{Type: "catan_fish_keep"})
			helperApply(t, s, 1, Action{Type: "catan_fish_replace", Card: 0})
			if s.Phase != "catan_aqueduct" || s.CatanPendingActor() != 1 {
				t.Fatal("aqueduct lost", s.Phase)
			}
			s = referenceEventRestore(t, s)
			helperApply(t, s, 1, Action{Type: "catan_aqueduct", Color: 0})
			if s.Phase != "catan_turn" || s.Catan.Fishing.LastRollID != 1 || s.Catan.Players[1].Resources[0] != 1 {
				t.Fatal("continued production")
			}
			if alchemy && !reflect.DeepEqual(deck, s.Catan.EventDeck.Deck) {
				t.Fatal("alchemy drew an event")
			}
			referenceEventRestore(t, s)
		})
	}
}

func TestCatanEventFishingReplacementBeforeGold(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, gold := fishWondersGoldGame(t, n, "fixed")
			if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
				t.Fatal(err)
			}
			g := s.Catan
			s.Turn, s.Phase = 0, "catan_roll"
			g.SetupStep, g.TurnSerial, g.Robber = g.SetupLimit(), 1, -1
			if g.Paired != nil {
				g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, false
			}
			ground := g.Fishing.Map.Grounds[0]
			city, village := ground.Vertices[0], ground.Vertices[2]
			if !slices.Contains(g.Tiles[gold].Vertices, city) {
				city, village = village, city
			}
			g.Vertices[city].Owner, g.Vertices[city].Level = 0, 2
			g.Vertices[village].Owner, g.Vertices[village].Level = 1, 1
			fishOwn(&g.Fishing.Tokens, 0, 0, 1, 2, 3, 4, 5, 6)
			fishOwn(&g.Fishing.Tokens, 1, 11, 12, 13, 14, 15, 16, 17)
			fishTop(&g.Fishing.Tokens, 21, 22, 23)
			d := &g.EventDeck.Deck
			at := slices.IndexFunc(d.DrawPile, func(id int) bool {
				if id >= len(catanEventReferenceFaces) {
					return false
				}
				face := catanEventReferenceFaces[id]
				return face.Production == ground.Number && face.Kind == "beautiful_day"
			})
			if at < 0 {
				t.Fatal("no calm production face for gold coast")
			}
			d.DrawPile[at], d.DrawPile[len(d.DrawPile)-1] = d.DrawPile[len(d.DrawPile)-1], d.DrawPile[at]
			if err := s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
				t.Fatal(err)
			}
			for _, p := range []int{0, 1} {
				if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != p || s.Catan.GoldPending != nil {
					t.Fatal("fish must finish before gold", p, s.Phase)
				}
				s = referenceEventRestore(t, s)
				helperReject(t, s, 2, Action{Type: "catan_fish_keep"})
				helperApply(t, s, p, Action{Type: "catan_fish_keep"})
			}
			if s.Phase != "catan_gold" || s.CatanPendingActor() != 0 || s.Catan.GoldPending.Claims[0].Count != 2 {
				t.Fatal("lost gold after card and fish")
			}
			s = referenceEventRestore(t, s)
			helperApply(t, s, 0, Action{Type: "catan_gold", Take: []int{0, 0, 0, 2, 0}})
			if s.Phase != "catan_turn" || s.Catan.Fishing.LastRollID != 1 {
				t.Fatal("production did not resume exactly once")
			}
			referenceEventRestore(t, s)
		})
	}
}

func TestCatanEventFishingRejectCorruptProduction(t *testing.T) {
	seed := eventFishingGame(t, 3, "", false, false)
	pre := clone(*seed)
	pre.Catan.Fishing.LastRollID = 0
	if pre.validateCatanEventSession() == nil {
		t.Fatal("pre-draw production accepted")
	}
	referenceEventTop(t, seed, "beautiful_day")
	if err := seed.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
		t.Fatal(err)
	}
	seed = finishEventKnightsResponses(t, seed)
	for _, mutate := range []func(*State){
		func(s *State) { s.Catan.Fishing.LastRollID-- },
		func(s *State) { s.Catan.RevealedEvent.ProductionStarted = false },
		func(s *State) { s.Phase = "catan_fish_replace" },
	} {
		bad := clone(*seed)
		mutate(&bad)
		before := clone(bad)
		if bad.validateCatanEventSession() == nil || bad.catanDrawEventRandom(func(int) int { return 2 }) == nil {
			t.Fatal("corrupt production accepted")
		}
		if !reflect.DeepEqual(before, bad) {
			t.Fatal("rejection mutated state")
		}
	}
}
