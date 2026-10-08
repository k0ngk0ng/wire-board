package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func pirateCityFixture(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanCitiesKnightsSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "pirate_islands"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Catan.SetupStep = s.Catan.SetupLimit()
	s.Turn = 0
	s.Phase = "catan_turn"
	return s
}
func TestCatanPirateKnightsUpgradeTimingAndBots(t *testing.T) {
	s := pirateCityFixture(t, 3)
	g := s.Catan
	k := g.CitiesKnights
	var vertex int = -1
	for _, v := range g.Vertices {
		if g.knightRecruitable(0, v.ID) {
			vertex = v.ID
			break
		}
	}
	if vertex < 0 {
		t.Fatal("no knight location")
	}
	helperGrant(s, 0, []int{0, 0, 2, 3, 2, 0, 0, 0})
	helperApply(t, s, 0, Action{Type: "catan_knight_recruit", Vertex: vertex})
	helperApply(t, s, 0, Action{Type: "catan_knight_activate", Vertex: vertex})
	helperReject(t, s, 0, Action{Type: "catan_knight_warship", Vertex: vertex})
	s.Catan.CitiesKnights.ActionSerial++
	helperReject(t, s, 1, Action{Type: "catan_knight_warship", Vertex: vertex})
	a, err := s.BotAction(0)
	if err != nil || a.Type != "catan_knight_warship" {
		t.Fatal("bot must upgrade", a, err)
	}
	ship := s.Catan.pirateNextWarship(0)
	helperApply(t, s, 0, a)
	g = s.Catan
	k = g.CitiesKnights
	if !g.Edges[ship].Warship || g.knightAt(vertex).Active || g.warships(0) != 1 || g.Players[0].Knights != 0 {
		t.Fatal("warship conversion")
	}
	helperApply(t, s, 0, Action{Type: "catan_knight_activate", Vertex: vertex})
	helperReject(t, s, 0, Action{Type: "catan_knight_warship", Vertex: vertex})
	if len(k.Knights) != 1 {
		t.Fatal("conversion removed knight")
	}
	for _, f := range g.pirateIslands().Fortresses {
		if g.knightPlaceable(0, f.Vertex) {
			t.Fatal("unconquered fort accepts knight")
		}
	}
	ckProgressStock(t, s.Catan)
}

func TestCatanPirateKnightsEventsAllFaces(t *testing.T) {
	for n := 3; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			seed, err := NewCatanCitiesKnightsSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "pirate_islands"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = seed.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			if err = seed.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			if err = seed.EnableCatanEvents(CatanEventCatalogue); err != nil {
				t.Fatal(err)
			}
			for step := 0; seed.Catan.setup() && step < 120; step++ {
				a, e := seed.BotAction(ckActor(seed))
				if e != nil {
					t.Fatal(e)
				}
				helperApply(t, seed, ckActor(seed), a)
			}
			if seed.Catan.setup() {
				t.Fatal("opening stalled")
			}
			for _, awake := range []bool{false, true} {
				for kind := range catanCardEventNames {
					cp := clone(*seed)
					s := &cp
					if awake {
						s.Catan.CitiesKnights.Invasions = 1
						s.Catan.Seafarers.Pirate = s.Catan.CitiesKnights.PirateStart
					}
					before := s.Catan.Seafarers.Pirate
					referenceEventTop(t, s, kind)
					if err = s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
						t.Fatal(kind, err)
					}
					if err = s.catanPirateSeven(); err != nil {
						t.Fatal(err)
					}
					s = finishEventKnightsResponses(t, s)
					ckProgressStock(t, s.Catan)
					if err = s.validatePirateKnights(); err != nil {
						t.Fatal(kind, err)
					}
					if !s.Catan.RevealedEvent.ProductionStarted || !s.Catan.EventDeck.Fleet.Resolved {
						t.Fatal("production missing")
					}
					want := before
					if awake {
						path := s.Catan.pirateIslands().FleetPath
						want = path[(slices.Index(path, before)+3)%len(path)]
					}
					if s.Catan.Seafarers.Pirate != want {
						t.Fatal("wrong movement", kind, awake)
					}
					if n > 4 {
						deck := clone(s.Catan.EventDeck)
						helperApply(t, s, s.Turn, Action{Type: "catan_end"})
						if !s.Catan.Paired.Second || !reflect.DeepEqual(deck, s.Catan.EventDeck) {
							t.Fatal("paired redrew")
						}
					}
				}
			}
		})
	}
}

func TestCatanPirateKnightsAlchemyDormancyAndRetiredFleet(t *testing.T) {
	for _, events := range []bool{false, true} {
		s := pirateCityFixture(t, 3)
		if events {
			if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
				t.Fatal(err)
			}
		}
		for _, invasion := range []bool{false, true} {
			if invasion {
				s.Catan.CitiesKnights.BarbarianPosition = 6
			}
			s.Phase = "catan_roll"
			ckProgressGive(t, s, 0, 0)
			helperApply(t, s, 0, Action{Type: "catan_progress", Card: 0, Tokens: []int{2, 5}})
			s = finishEventKnightsResponses(t, s)
			if !invasion && s.Catan.CitiesKnights.Invasions == 0 && (s.Catan.Seafarers.Pirate != -1 || s.Catan.pirateIslands().SevenPending) {
				t.Fatal("dormancy lost")
			}
			if err := s.validatePirateKnights(); err != nil {
				t.Fatal(err)
			}
			if events && len(s.Catan.EventDeck.Deck.Discard) != 0 {
				t.Fatal("alchemy drew event")
			}
		}
		// Explicit first-invasion completion cannot revive an already retired fleet.
		g := s.Catan
		g.CitiesKnights.Invasions = 0
		for i := range g.pirateIslands().Fortresses {
			g.pirateIslands().Fortresses[i].Strength = 0
		}
		g.Seafarers.Pirate = -1
		s.catanFinishBarbarians()
		if g.Seafarers.Pirate != -1 || g.Robber != -1 {
			t.Fatal("retired fleet revived")
		}
	}
}

func TestCatanPirateKnightsFirstInvasionBeforeFleet(t *testing.T) {
	for _, events := range []bool{false, true} {
		s, err := NewCatanCitiesKnightsSeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: "pirate_islands"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if events {
			if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
				t.Fatal(err)
			}
		}
		for s.Catan.setup() {
			p := ckActor(s)
			a, e := s.BotAction(p)
			if e != nil {
				t.Fatal(e)
			}
			helperApply(t, s, p, a)
		}
		s.Catan.CitiesKnights.BarbarianPosition = 6
		if events {
			referenceEventTop(t, s, "beautiful_day")
			err = s.catanDrawEventRandom(func(int) int { return 3 })
		} else {
			err = s.catanCityRoll(4, 4, 3)
		}
		if err != nil {
			t.Fatal(err)
		}
		if s.Phase != "catan_pillage" || s.Catan.Seafarers.Pirate != -1 || s.Catan.CitiesKnights.Invasions != 0 {
			t.Fatal("fleet activated before invasion responses", events, s.Phase)
		}
		s = referenceEventRestore(t, s)
		f := s.Catan.pirateIslands().CityFleet
		if events {
			f = s.Catan.EventDeck.Fleet
			if s.Catan.RevealedEvent.ProductionStarted {
				t.Fatal("production started before responses")
			}
		}
		if f.Resolved {
			t.Fatal("fleet resolved before responses")
		}
		s = finishEventKnightsResponses(t, s)
		g := s.Catan
		path := g.pirateIslands().FleetPath
		want := path[(slices.Index(path, g.CitiesKnights.PirateStart)+4)%len(path)]
		if g.CitiesKnights.Invasions != 1 || g.Seafarers.Pirate != want || g.RollID != 1 {
			t.Fatal("first invasion did not resume fleet exactly once", events)
		}
		if err = s.validatePirateKnights(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCatanPirateKnightsDiplomacyAndForcedLoss(t *testing.T) {
	s := pirateCityFixture(t, 4)
	helperGrant(s, 0, []int{15, 0, 15, 0, 0, 0, 0, 0})
	navyComplete(t, s)
	g := s.Catan
	route := append([]int{}, g.pirateIslands().Fortresses[0].Route...)
	tail := route[len(route)-1]
	g.Edges[tail].Warship = true
	if !slices.Contains(g.diplomacyRoads(), tail) {
		t.Fatal("tail not open")
	}
	ckProgressGive(t, s, 0, 16)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 16, Edge: tail})
	if s.Phase != "catan_diplomacy" || !s.Catan.CitiesKnights.Pending.Warship || s.Catan.Edges[tail].Warship {
		t.Fatal("warship removal state")
	}
	s = referenceEventRestore(t, s)
	helperApply(t, s, 0, Action{Type: "catan_diplomacy", Edge: tail})
	g = s.Catan
	if !g.Edges[tail].Warship || !slices.Equal(route, g.pirateIslands().Fortresses[0].Route) {
		t.Fatal("diplomacy lost warship/path")
	}
	vertices, _ := g.pirateRouteVertices(0)
	// A knight on the penultimate vertex loses both incident tail ships.
	v := vertices[len(vertices)-2]
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: v, Strength: 1, Active: true}}
	if !g.pirateKnightSite(v) {
		t.Fatal("fixture knight on fort")
	}
	if !s.catanAttackFortress(0, 6) {
		t.Fatal("attack unavailable")
	}
	if g.knightAt(v) != nil || g.pirateFortressReady(0) {
		t.Fatal("stranded knight or deleted route survived")
	}
	if err := s.validatePirateKnights(); err != nil {
		t.Fatal(err)
	}
	ckProgressStock(t, g)
}

func TestCatanPirateKnightsTaxationWithoutRobber(t *testing.T) {
	s := pirateCityFixture(t, 3)
	g := s.Catan
	g.CitiesKnights.Invasions = 1
	g.Seafarers.Pirate = g.CitiesKnights.PirateStart
	ckProgressGive(t, s, 0, 21)
	helperGrant(s, 1, []int{0, 0, 0, 0, 0, 2, 0, 0})
	target := -1
	for _, tile := range g.Tiles {
		if tile.Number > 0 && slices.Contains(tile.Vertices, g.pirateIslands().Fortresses[1].StartVertex) {
			target = tile.ID
			break
		}
	}
	if target < 0 || !slices.Contains(g.taxationTiles(), target) {
		t.Fatal("taxation target")
	}
	pirate := g.Seafarers.Pirate
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: target})
	g = s.Catan
	if g.Robber != -1 || g.Seafarers.Pirate != pirate || g.Players[0].Resources[5] != 1 || g.Players[1].Resources[5] != 1 {
		t.Fatal("taxation failed")
	}
	if err := s.validatePirateKnights(); err != nil {
		t.Fatal(err)
	}
}

func TestCatanPirateKnightsFleetRewardsAndEmptyResourceBank(t *testing.T) {
	for _, events := range []bool{false, true} {
		for _, empty := range []bool{false, true} {
			s := fleetFixture(t, 4, 1, 1, 2)
			g := s.Catan
			pirate := g.Seafarers.Pirate
			s.enableCitiesKnights()
			g.pirateIslands().KnightsRules = CatanPirateKnightsRules
			g.CitiesKnights.PirateStart = g.pirateIslands().FleetPath[0]
			g.CitiesKnights.Invasions = 1
			g.Seafarers.Pirate = pirate
			g.Seafarers.Layout = "fixed"
			g.Seafarers.Rules = CatanSeafarersRules
			s.Turn = 0
			s.Phase = "catan_roll"
			for i := range g.Tiles {
				g.Tiles[i].Number = 0
			}
			if empty {
				for c := 0; c < 5; c++ {
					g.Players[0].Resources[c] = g.Bank[c]
					g.Bank[c] = 0
				}
			}
			if events {
				if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.catanCityRoll(1, 5, 0); err != nil {
				t.Fatal(err)
			}
			if empty {
				if s.Phase == "catan_fleet_reward" || s.Catan.pirateIslands().Raid != nil {
					t.Fatal("commodity-only bank stalled reward")
				}
			} else {
				if s.Phase != "catan_fleet_reward" {
					t.Fatal("reward missing", s.Phase)
				}
				s = referenceEventRestore(t, s)
				helperReject(t, s, 0, Action{Type: "catan_fleet_reward", Color: 0})
				helperReject(t, s, 1, Action{Type: "catan_fleet_reward", Color: 5})
				for c := 0; c < 5; c++ {
					s.Catan.Players[1].Resources[c] = 5
				}
				s.Catan.Players[1].Resources[2] = 0
				a, err := s.BotAction(1)
				if err != nil || a.Color != 2 {
					t.Fatal("bot chose unavailable commodity", a, err)
				}
				// Restore the synthetic hand before applying a real reward.
				for c := 0; c < 5; c++ {
					s.Catan.Players[1].Resources[c] = 0
				}
				helperApply(t, s, 1, a)
				if s.Phase != "catan_turn" || s.Catan.Players[1].Resources[2] != 1 {
					t.Fatal("reward did not resume production")
				}
			}
			if err := s.validatePirateKnights(); err != nil {
				t.Fatal(err)
			}
			if events {
				if err := s.validateCatanEventSession(); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestCatanPirateKnightsRestoreRejectsCorruptFleet(t *testing.T) {
	s := pirateCityFixture(t, 3)
	s.Phase = "catan_roll"
	if err := s.catanCityRoll(1, 5, 0); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Catan){
		func(g *Catan) { g.pirateIslands().KnightsRules = "bad" },
		func(g *Catan) { g.pirateIslands().CityFleet.RollID++ },
		func(g *Catan) { g.pirateIslands().CityFleet.Resolved = false },
		func(g *Catan) { g.pirateIslands().CityFleet.Dice[0] = 7 },
		func(g *Catan) { g.Seafarers.Pirate = g.pirateIslands().FleetPath[0] },
		func(g *Catan) { g.Robber = 0 },
		func(g *Catan) { g.pirateIslands().Fortresses[0].Route = []int{-1} },
		func(g *Catan) { g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: -1}} },
		func(g *Catan) { g.CitiesKnights.Knights = []CatanKnight{{Owner: len(g.Players), Vertex: 0}} },
	} {
		bad := clone(*s)
		mutate(bad.Catan)
		if bad.validatePirateKnights() == nil {
			t.Fatal("corrupt fleet accepted")
		}
		helperReject(t, &bad, 0, Action{Type: "catan_end"})
	}
}
