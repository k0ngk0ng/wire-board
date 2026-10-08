package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func eventSeaEndgame(t *testing.T, n int, scenario, mode string) *State {
	t.Helper()
	options := CatanOptions{FiveSix: n > 4, Helpers: mode == "helpers", AllHelpers: mode == "helpers"}
	setup := CatanSeafarersSetup{Scenario: scenario}
	if n == 4 && scenario != "new_world" {
		setup.Layout = "variable"
	}
	var world *CatanNewWorldMap
	var err error
	if scenario == "new_world" {
		world, err = GenerateCatanNewWorldMap(n)
		if err != nil {
			t.Fatal(err)
		}
	}
	var s *State
	if mode == "knights" {
		s, err = NewCatanCitiesKnightsSeafarers(n, options, setup, world)
	} else {
		s, err = NewCatanSeafarers(n, options, setup, world)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	for steps := 0; s.Catan.setup() && steps < 100; steps++ {
		s = referenceEventRestore(t, s)
		if len(s.Catan.EventDeck.Deck.Discard) != 0 || s.Catan.RollID != 0 {
			t.Fatal("setup drew event")
		}
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

func TestCatanEventSeaEndgameAllFaces(t *testing.T) {
	for _, scenario := range []string{"cloth", "wonders", "new_world"} {
		for n := 3; n <= 6; n++ {
			for _, mode := range []string{"ordinary", "helpers", "knights"} {
				t.Run(fmt.Sprintf("%s/%d/%s", scenario, n, mode), func(t *testing.T) {
					seed := eventSeaEndgame(t, n, scenario, mode)
					for kind := range catanCardEventNames {
						t.Run(kind, func(t *testing.T) {
							copy := clone(*seed)
							s := &copy
							pirate := s.Catan.Seafarers.Pirate
							referenceEventTop(t, s, kind)
							if err := s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
								t.Fatal(err)
							}
							s = finishEventKnightsResponses(t, s)
							g := s.Catan
							if g.RollID != 1 || g.RevealedEvent.Kind != kind || !g.RevealedEvent.ProductionStarted {
								t.Fatal("wrong production")
							}
							if kind == "robber_flees" {
								if g.Seafarers.Pirate != pirate {
									t.Fatal("flee moved pirate")
								}
								if mode == "knights" && g.Robber != -1 {
									t.Fatal("flee woke dormant robber")
								}
								if mode != "knights" && ((len(g.fleeDeserts()) == 0 && g.Robber != -1) || g.Robber >= 0 && !slices.Contains(g.fleeDeserts(), g.Robber)) {
									t.Fatal("incorrect desert/frame")
								}
							}
							for _, e := range g.Edges {
								if e.Ship && e.Damaged {
									t.Fatal("damaged ship")
								}
							}
							if mode == "knights" {
								ckProgressStock(t, g)
							} else {
								seaEventConserved(t, s)
							}
							if err := g.validateClothSupply(); err != nil {
								t.Fatal(err)
							}
							if n > 4 {
								helperApply(t, s, s.Turn, Action{Type: "catan_end"})
								if !s.Catan.Paired.Second || s.Catan.RollID != 1 {
									t.Fatal("secondary redrew")
								}
							}
						})
					}
				})
			}
		}
	}
}

func TestCatanEventSeaEndgameRejectsDetachedScenarioState(t *testing.T) {
	for _, scenario := range []string{"cloth", "wonders", "new_world"} {
		t.Run(scenario, func(t *testing.T) {
			s := eventSeaEndgame(t, 3, scenario, "ordinary")
			g := s.Catan
			switch scenario {
			case "cloth":
				g.Seafarers.Cloth = nil
			case "wonders":
				g.Seafarers.Wonders = nil
			case "new_world":
				g.Seafarers.NewWorld = nil
			}
			if s.validateCatanEventSession() == nil {
				t.Fatal("missing module accepted")
			}
			helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
		})
	}
}

func TestCatanEventClothEpidemicDepletionAndHilda(t *testing.T) {
	for _, mode := range []string{"ordinary", "helpers", "knights"} {
		t.Run(mode, func(t *testing.T) {
			s := eventSeaEndgame(t, 6, "cloth", mode)
			g := s.Catan
			c := g.cloth()
			// Isolated legal stocks: four depleted villages, a fifth with one
			// cloth, and three traders. Move removed cloth back to common supply.
			for i := 0; i < 4; i++ {
				c.Stock += c.Villages[i].Stock
				c.Villages[i].Stock = 0
			}
			c.Stock += c.Villages[4].Stock - 1
			c.Villages[4].Stock = 1
			c.Villages[4].Number = 6
			c.Villages[4].Traders = []int{0, 1, 2}
			for i := range c.Villages {
				if i != 4 && c.Villages[i].Number == 6 {
					c.Villages[i].Number = 5
				}
			}
			for i := range g.Tiles {
				if g.Tiles[i].Number == 6 {
					g.Tiles[i].Number = 5
				}
			}
			if mode == "helpers" {
				eventAssignHelper(t, s, 1, 3)
			}
			// Select the six-point Epidemic face from the actual deck.
			d := &g.EventDeck.Deck
			at := slices.Index(d.DrawPile, 10)
			if at < 0 {
				t.Fatal("epidemic absent")
			}
			d.DrawPile[at], d.DrawPile[len(d.DrawPile)-1] = d.DrawPile[len(d.DrawPile)-1], d.DrawPile[at]
			before := slices.Clone(c.Held)
			if err := s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
				t.Fatal(err)
			}
			g = s.Catan
			c = g.cloth()
			for p := range 3 {
				if c.Held[p] != before[p]+1 {
					t.Fatal("epidemic altered cloth or shortage denied trader")
				}
			}
			if s.Finished {
				t.Fatal("depletion ended before action")
			}
			if mode == "helpers" && (g.HelperPending == nil || g.HelperPending.Player != 1) {
				t.Fatal("cloth suppressed Hilda")
			}
			s = finishEventKnightsResponses(t, s)
			s.catanScores()
			best, held := -1, -1
			want := []int{}
			for p, player := range s.Catan.Players {
				if player.Score > best || player.Score == best && s.Catan.cloth().Held[p] > held {
					best, held, want = player.Score, s.Catan.cloth().Held[p], []int{p}
				} else if player.Score == best && s.Catan.cloth().Held[p] == held {
					want = append(want, p)
				}
			}
			helperApply(t, s, s.Turn, Action{Type: "catan_end"})
			if !s.Finished || !reflect.DeepEqual(s.Winners, want) {
				t.Fatal("lost depletion ending", s.Winners, want)
			}
			referenceEventRestore(t, s)
		})
	}
}

func TestCatanEventClothFallbackVersionAndOrdinaryMoves(t *testing.T) {
	for _, mode := range []string{"ordinary", "helpers", "knights"} {
		t.Run(mode, func(t *testing.T) {
			s := eventSeaEndgame(t, 3, "cloth", mode)
			g := s.Catan
			if g.EventDeck.ClothFallback != CatanEventClothFallbackRules {
				t.Fatal("missing supplemental rule")
			}
			if mode == "knights" {
				g.CitiesKnights.Invasions = 1
				g.Robber = g.CitiesKnights.RobberStart
			}
			deserts := []int{}
			for _, tile := range g.Tiles {
				if tile.Resource == CatanDesert {
					deserts = append(deserts, tile.ID)
				}
			}
			if len(deserts) == 0 || len(g.fleeDeserts()) != 0 {
				t.Fatal("fixture needs inaccessible island deserts")
			}
			for _, tile := range deserts {
				if g.robberAllowed(tile) {
					t.Fatal("ordinary robber gained access to small island")
				}
			}
			for _, version := range []string{"", "unknown"} {
				bad := clone(*s)
				bad.Catan.EventDeck.ClothFallback = version
				if bad.validateCatanEventSession() == nil {
					t.Fatal("unversioned fallback accepted")
				}
				helperReject(t, &bad, bad.Turn, Action{Type: "catan_roll"})
			}
			pirate := g.Seafarers.Pirate
			referenceEventTop(t, s, "robber_flees")
			helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
			s = finishEventKnightsResponses(t, s)
			if s.Catan.Robber != -1 || s.Catan.Seafarers.Pirate != pirate || len(s.Catan.Victims) != 0 {
				t.Fatal("fallback moved pirate or stole")
			}
			referenceEventRestore(t, s)
		})
	}
	base := referenceEventGame(t, 3, true)
	base.Catan.EventDeck.ClothFallback = CatanEventClothFallbackRules
	if base.validateCatanEventSession() == nil {
		t.Fatal("detached fallback accepted on base")
	}
}
