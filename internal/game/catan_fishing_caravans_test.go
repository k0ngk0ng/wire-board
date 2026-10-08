package game

import (
	"encoding/json"
	"fmt"
	"testing"
)

func fishingCaravanRestore(t *testing.T, s *State) {
	t.Helper()
	raw, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var restored State
	if e = json.Unmarshal(raw, &restored); e != nil {
		t.Fatal(e)
	}
	if e = restored.validateCaravans(); e != nil {
		t.Fatal(e)
	}
	if e = restored.Catan.validateFishing(); e != nil {
		t.Fatal(e)
	}
	if e = restored.validateCatanTwo(); e != nil {
		t.Fatal(e)
	}
	if e = restored.validateCatanEventSession(); e != nil {
		t.Fatal(e)
	}
	*s = restored
}
func TestCatanFishingCaravansNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, e := newCatanFishingCaravans(n, knights)
					if e != nil {
						t.Fatal(e)
					}
					if events {
						if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
							t.Fatal(e)
						}
					}
					phases := map[string]int{}
					for step := 0; step < 16000 && !s.Finished; step++ {
						p := twoFullActor(s)
						a, e := s.BotAction(p)
						if e != nil {
							t.Fatal(step, s.Phase, e)
						}
						if e = s.Apply(p, a); e != nil {
							t.Fatal(step, s.Phase, a, e)
						}
						phases[a.Type]++
						if step%83 == 0 {
							fishingCaravanRestore(t, s)
							for viewer := -1; viewer < n; viewer++ {
								s.View(viewer)
							}
						}
					}
					if !s.Finished {
						t.Fatal("unfinished", s.Round, s.Phase)
					}
					t.Log("rounds", s.Round, "fish roads", phases["catan_fish_road"], "bids", phases["catan_caravan_bid"])
				})
			}
		}
	}
}

func TestCatanFishingCaravansMapProductionAndIsolation(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s, e := NewCatanFishingCaravans(n, CatanOptions{FiveSix: n > 4}, false)
		if e != nil {
			t.Fatal(e)
		}
		g := s.Catan
		for _, extra := range g.Fishing.Map.ExtraNumbers {
			tile := g.Tiles[extra.Tile]
			if !g.tileProduces(tile, 2) || !g.tileProduces(tile, 12) || g.tileProduces(tile, 7) {
				t.Fatal("double production", tile)
			}
		}
		for _, lake := range g.Fishing.Map.Lakes {
			v := g.Tiles[lake.Tile].Vertices[0]
			previous := g.Vertices[v]
			g.Vertices[v].Owner = 0
			g.Vertices[v].Level = 2
			for _, number := range lake.Numbers {
				due, e := g.Fishing.Map.production(g, number)
				if e != nil || due[0] != 2 {
					t.Fatal("lake city production", n, number, due, e)
				}
				g.Robber = lake.Tile
				due, e = g.Fishing.Map.production(g, number)
				if e != nil || due[0] != 0 {
					t.Fatal("robber must block lake", due, e)
				}
				g.Robber = -1
			}
			g.Vertices[v] = previous
		}
		if g.victoryTargetFor(0) != 12 {
			t.Fatal("ordinary target")
		}
		for _, mutate := range []func(*Catan){
			func(g *Catan) { g.Fishing.Caravans = "" },
			func(g *Catan) { g.Fishing.Map.Lakes[0].Numbers = []int{6, 8} },
			func(g *Catan) { g.Fishing.Map.ExtraNumbers[0].Number = 11 },
			func(g *Catan) { g.Fishing.Map.ExtraNumbers = nil },
			func(g *Catan) { g.Fishing.Rivers = CatanFishingRiversRules },
			func(g *Catan) { g.Tiles[g.Fishing.Map.Lakes[0].Tile].Resource = 0 },
			func(g *Catan) { g.Caravans.Map.WateringHoles[0] = 99 },
		} {
			bad := clone(*s)
			mutate(bad.Catan)
			if bad.Catan.validateFishing() == nil {
				t.Fatal("accepted damaged combination")
			}
		}
		var base *State
		if n == 2 {
			base, e = NewCatanTwoCaravans(n, CatanOptions{})
		} else {
			base, e = NewCatanCaravans(n, CatanOptions{FiveSix: n > 4})
		}
		if e != nil || base.Catan.Fishing != nil || base.validateCaravans() != nil {
			t.Fatal("ordinary caravan changed", e)
		}
		if n == 2 && (base.Catan.Two.Bank != 10 || sum(base.Catan.Two.Tokens) != 10) {
			t.Fatal("ordinary trade chips changed")
		}
	}
}

func TestCatanFishingCaravansInventionRestore(t *testing.T) {
	for _, n := range []int{2, 6} {
		s, e := newCatanFishingCaravans(n, true)
		if e != nil {
			t.Fatal(e)
		}
		for s.Catan.setup() {
			p := twoFullActor(s)
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
			t.Fatal("no invention numbers")
		}
		if e = s.catanInvention(Action{Tile: choices[0].Tile, Target: choices[1].Tile}); e != nil {
			t.Fatal(e)
		}
		fishingCaravanRestore(t, s)
		if s.Catan.victoryTargetFor(0) != 15 {
			t.Fatal("knight target")
		}
		for _, extra := range s.Catan.Fishing.Map.ExtraNumbers {
			if e = s.catanInvention(Action{Tile: extra.Tile, Target: choices[0].Tile, Tokens: []int{1, 0}}); e == nil {
				t.Fatal("12 must remain fixed")
			}
		}
		s.Catan.Fishing.Map.NumberSwaps = nil
		if s.Catan.validateFishing() == nil {
			t.Fatal("missing receipt accepted")
		}
	}
}
