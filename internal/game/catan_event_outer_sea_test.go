package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanEventOuterSeaAllFaces(t *testing.T) {
	for _, scene := range []string{"tribe", "pirate_islands"} {
		for n := 3; n <= 6; n++ {
			for mode := 0; mode < 3; mode++ {
				if mode == 2 && (scene != "tribe" || n > 4) {
					continue
				}
				t.Run(fmt.Sprintf("%s/%d/mode%d", scene, n, mode), func(t *testing.T) {
					opts := CatanOptions{FiveSix: n > 4, Helpers: mode == 1, AllHelpers: mode == 1}
					setup := CatanSeafarersSetup{Scenario: scene}
					var seed *State
					var err error
					if mode == 2 {
						seed, err = NewCatanFishingSeafarers(n, opts, setup, nil)
					} else {
						seed, err = NewCatanSeafarers(n, opts, setup, nil)
					}
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
					for step := 0; seed.Catan.setup() && step < 160; step++ {
						seed = referenceEventRestore(t, seed)
						p := ckActor(seed)
						a, e := seed.BotAction(p)
						if e != nil {
							t.Fatal(e)
						}
						helperApply(t, seed, p, a)
					}
					if seed.Catan.setup() {
						t.Fatal("opening stalled")
					}
					for kind := range catanCardEventNames {
						copy := clone(*seed)
						s := &copy
						referenceEventTop(t, s, kind)
						before := s.Catan.Seafarers.Pirate
						if err := s.catanDrawEventRandom(func(int) int { return 2 }); err != nil {
							t.Fatal(kind, err)
						}
						// Public applyCatan performs this post-action continuation.
						if err := s.catanPirateSeven(); err != nil {
							t.Fatal(err)
						}
						s = finishEventKnightsResponses(t, s)
						if !s.Catan.RevealedEvent.ProductionStarted {
							t.Fatal("production missing")
						}
						if scene == "tribe" {
							cardTribeInventory(t, s.Catan)
							if kind == "robber_flees" && s.Catan.Seafarers.Pirate != before {
								t.Fatal("flee moved pirate")
							}
						} else {
							g := s.Catan
							p := g.pirateIslands()
							f := g.EventDeck.Fleet
							want := p.FleetPath[(slices.Index(p.FleetPath, before)+3)%len(p.FleetPath)]
							if f == nil || f.Dice != [2]int{3, 3} || !f.Resolved || g.Seafarers.Pirate != want || g.Robber != -1 {
								t.Fatal("fleet must move exactly once after each card", kind)
							}
							fleetSupply(t, g)
						}
						if n > 4 {
							before := clone(s.Catan.EventDeck)
							helperApply(t, s, s.Turn, Action{Type: "catan_end"})
							if !s.Catan.Paired.Second || !reflect.DeepEqual(before, s.Catan.EventDeck) {
								t.Fatal("paired seat redrew")
							}
						}
					}
				})
			}
		}
	}
}

func TestCatanEventFleetRewardEpidemicAndCorruptRestore(t *testing.T) {
	s := fleetFixture(t, 4, 1, 1, 2)
	g := s.Catan
	g.Seafarers.Rules, g.Seafarers.Layout = CatanSeafarersRules, "fixed"
	g.Vertices[g.pirateIslands().Fortresses[1].StartVertex].Level = 2
	for i := range g.Tiles {
		g.Tiles[i].Number = 0
	}
	for i := range g.Tiles {
		if g.Tiles[i].Resource == 3 && slices.Contains(g.Tiles[i].Vertices, g.pirateIslands().Fortresses[1].StartVertex) {
			g.Tiles[i].Number = 6
			break
		}
	}
	if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	d := &s.Catan.EventDeck.Deck
	at := slices.Index(d.DrawPile, 10)
	d.DrawPile[at], d.DrawPile[len(d.DrawPile)-1] = d.DrawPile[len(d.DrawPile)-1], d.DrawPile[at]
	if err := s.catanDrawEventRandom(func(int) int { return 0 }); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_fleet_reward" || s.CatanPendingActor() != 1 || s.Catan.RevealedEvent.ProductionStarted || sum(s.Catan.Players[1].Resources) != 0 {
		t.Fatal("fleet reward must precede production")
	}
	s = referenceEventRestore(t, s)
	for _, mutate := range []func(*State){
		func(b *State) { b.Catan.EventDeck.FleetRules = "unknown" },
		func(b *State) { b.Catan.EventDeck.Fleet.RollID++ },
		func(b *State) { b.Catan.EventDeck.Fleet.Resolved = false },
		func(b *State) { b.Catan.pirateIslands().Raid.Total = 8 },
		func(b *State) { b.Catan.pirateIslands().Raid.Epidemic = false },
	} {
		bad := clone(*s)
		mutate(&bad)
		if bad.validateCatanEventSession() == nil {
			t.Fatal("corrupt fleet accepted")
		}
	}
	helperReject(t, s, 0, Action{Type: "catan_fleet_reward", Color: 0})
	pirate := s.Catan.Seafarers.Pirate
	helperApply(t, s, 1, Action{Type: "catan_fleet_reward", Color: 4})
	g = s.Catan
	if s.Phase != "catan_turn" || g.Seafarers.Pirate != pirate || g.Players[1].Resources[3] != 1 || g.Players[1].Resources[4] != 1 || sum(g.Players[1].Resources) != 2 {
		t.Fatal("epidemic or reward lost on resume", g.Players[1].Resources)
	}
	referenceEventRestore(t, s)
	fleetSupply(t, g)
}

func TestCatanEventFleetLossBeforeSeven(t *testing.T) {
	s := fleetFixture(t, 4, 1, 2, 1)
	g := s.Catan
	g.Seafarers.Rules, g.Seafarers.Layout = CatanSeafarersRules, "fixed"
	fleetGive(g, 1, []int{4, 2, 1, 1, 1})
	g.Vertices[g.pirateIslands().Fortresses[1].StartVertex].Level = 2
	if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	referenceEventTop(t, s, "robber_attacks")
	i := 0
	if err := s.catanDrawEventRandom(func(int) int {
		i++
		if i == 1 {
			return 1
		}
		return 4
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.catanPirateSeven(); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_steal" || sum(s.Catan.Players[1].Resources) != 7 || s.Catan.DiscardDue[1] != 0 || s.Catan.Robber != -1 {
		t.Fatal("fleet loss should precede discard", s.Phase, s.Catan.Players[1].Resources)
	}
	referenceEventRestore(t, s)
}
