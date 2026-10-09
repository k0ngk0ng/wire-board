package game

import (
	"fmt"
	"slices"
	"testing"
)

func TestCatanTransportSeaModulesAdmission(t *testing.T) {
	for _, scene := range []string{"shores", "desert"} {
		for _, layout := range []string{"fixed", "variable"} {
			for n := 2; n <= 6; n++ {
				for bits := 0; bits < 4; bits++ {
					t.Run(fmt.Sprintf("%s/%s/%d/%d", scene, layout, n, bits), func(t *testing.T) {
						s, err := NewCatanTransportSea(n, CatanTransportSeaSetup{Scenario: scene, Layout: layout}, bits&1 != 0, bits&2 != 0)
						if err != nil {
							t.Fatal(err)
						}
						target := 17
						if bits&1 != 0 {
							target = 19
						} else if bits&2 != 0 {
							target = 16
						}
						if s.Catan.victoryTarget() != target {
							t.Fatal("target")
						}
						if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
							t.Fatal(err)
						}
						if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
							t.Fatal(err)
						}
						if err = s.EnableCatanTradersHelpers(true); err != nil {
							t.Fatal(err)
						}
						if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
							t.Fatal(err)
						}
						for steps := 0; s.Catan.setup() && steps < 100; steps++ {
							p := ckActor(s)
							a, e := s.BotAction(p)
							if e != nil {
								t.Fatal(e)
							}
							if e = s.Apply(p, a); e != nil {
								t.Fatal(a, e)
							}
						}
						if s.Catan.setup() {
							t.Fatal("setup incomplete")
						}
						cp := clone(*s)
						if err = cp.validateCatanTransport(); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}

func TestCatanTransportSeaModulesInventionAndInventory(t *testing.T) {
	s, err := NewCatanTransportSea(3, CatanTransportSeaSetup{Scenario: "shores", Layout: "variable"}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		p := ckActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(e)
		}
	}
	choices := s.Catan.inventionNumbers()
	if len(choices) < 2 {
		t.Fatal("missing invention discs")
	}
	a, b := choices[0], choices[1]
	if err = s.catanInvention(Action{Tile: a.Tile, Target: b.Tile, Tokens: []int{a.Slot, b.Slot}}); err != nil {
		t.Fatal(err)
	}
	if len(s.Catan.Transport.Map.NumberSwaps) != 1 {
		t.Fatal("swap not recorded")
	}
	cp := clone(*s)
	if err = cp.validateCatanTransport(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Catan){func(g *Catan) { g.Seafarers.Layout = "fixed" }, func(g *Catan) { g.Transport.SeaKnights = "bad" }, func(g *Catan) { g.Transport.Map.SeaLayout = "bad" }, func(g *Catan) { g.Transport.Map.SeaForest = nil }, func(g *Catan) { g.Fishing.Map.SeaRecipe = "bad" }, func(g *Catan) { g.Seafarers.Islands = nil }} {
		cp := clone(*s)
		mutate(cp.Catan)
		if cp.validateCatanTransport() == nil {
			t.Fatal("corrupt combo accepted")
		}
	}
	for _, edge := range s.Catan.Edges {
		if len(edge.Tiles) == 2 {
			n := []int{s.Catan.Tiles[edge.Tiles[0]].Number, s.Catan.Tiles[edge.Tiles[1]].Number}
			if (n[0] == 6 || n[0] == 8) && (n[1] == 6 || n[1] == 8) {
				t.Fatal("adjacent red numbers")
			}
		}
	}
	for _, ground := range s.Catan.Fishing.Map.Grounds {
		for _, edge := range ground.Edges {
			if !s.Catan.edgeTerrain(edge, true) || !s.Catan.edgeTerrain(edge, false) {
				t.Fatal("fishing ground on water frame")
			}
			for _, port := range s.Catan.Ports {
				if port.Edge == edge {
					t.Fatal("fishing ground overlaps port")
				}
			}
		}
	}
	before := slices.Clone(s.Catan.Players[0].Resources)
	if len(before) != 8 {
		t.Fatal("missing commodity hand")
	}
}

func TestCatanTransportSeaModulesNatural(t *testing.T) {
	for _, scene := range []string{"shores", "desert"} {
		for _, c := range []struct{ n, bits int }{{2, 1}, {3, 2}, {2, 3}, {6, 3}} {
			t.Run(fmt.Sprintf("%s/%d/%d", scene, c.n, c.bits), func(t *testing.T) {
				s, err := NewCatanTransportSea(c.n, CatanTransportSeaSetup{Scenario: scene, Layout: "variable"}, c.bits&1 != 0, c.bits&2 != 0)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
					t.Fatal(err)
				}
				if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
					t.Fatal(err)
				}
				if err = s.EnableCatanTradersHelpers(true); err != nil {
					t.Fatal(err)
				}
				if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
				for step := 0; step < 16000 && !s.Finished; step++ {
					p := ckActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(step, s.Phase, e)
					}
					if e = s.Apply(p, a); e != nil {
						t.Fatal(step, s.Phase, a, e)
					}
					if step%137 == 0 {
						cp := clone(*s)
						s = &cp
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round, s.Phase)
				}
				t.Logf("completed round %d", s.Round)
			})
		}
	}
}
