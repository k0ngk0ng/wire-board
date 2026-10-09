package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func riversCaravansRestore(t *testing.T, s *State) {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var next State
	if err = json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	if err = next.Catan.validateRivers(); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCaravans(); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanEventSession(); err != nil {
		t.Fatal(err)
	}
	if next.Catan.Two != nil {
		if err = next.validateCatanTwo(); err != nil {
			t.Fatal(err)
		}
	}
	*s = next
}
func TestCatanRiversCaravansConstruction(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%t", n, knights), func(t *testing.T) {
				s, err := NewCatanRiversCaravans(n, CatanOptions{FiveSix: n > 4}, knights)
				if err != nil {
					t.Fatal(err)
				}
				riversCaravansRestore(t, s)
				if s.Phase != "catan_rivers_start" {
					t.Fatal("missing robber start")
				}
				want := 12
				if knights {
					want = 15
				}
				if s.Catan.victoryTarget() != want {
					t.Fatal("wrong target")
				}
				for _, extra := range s.Catan.Caravans.ExtraNumbers {
					tile := s.Catan.Tiles[extra.Tile]
					if tile.Number != 3 || !s.Catan.tileProduces(tile, 3) || !s.Catan.tileProduces(tile, extra.Number) || s.Catan.tileNumberWeight(tile) != 3 {
						t.Fatal("double production", tile, extra)
					}
				}
			})
		}
	}
}
func TestCatanRiversCaravansNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, err := NewCatanRiversCaravans(n, CatanOptions{FiveSix: n > 4}, knights)
					if err != nil {
						t.Fatal(err)
					}
					if events {
						if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
							t.Fatal(err)
						}
					}
					for step := 0; step < 16000 && !s.Finished; step++ {
						actor := twoFullActor(s)
						a, err := s.BotAction(actor)
						if err != nil {
							t.Fatal(step, s.Phase, err)
						}
						if err = s.Apply(actor, a); err != nil {
							t.Fatal(step, s.Phase, a, err)
						}
						if step%97 == 0 {
							riversCaravansRestore(t, s)
							for viewer := -1; viewer < n; viewer++ {
								s.View(viewer)
							}
						}
					}
					if !s.Finished {
						t.Fatal("unfinished", s.Round)
					}
					riversCaravansRestore(t, s)
					t.Log("complete", s.Round, "wagons", len(s.Catan.Caravans.Wagons))
				})
			}
		}
	}
}

func TestCatanRiversCaravansBridgeAndCorruptSave(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s, e := NewCatanRiversCaravans(n, CatanOptions{FiveSix: n > 4}, false)
		if e != nil {
			t.Fatal(e)
		}
		g := s.Catan
		// Follow a legal caravan head until it reaches a river bridge site.
		var path func(catanCaravans, int) []catanCaravanWagon
		path = func(c catanCaravans, depth int) []catanCaravanWagon {
			if depth == 0 {
				return nil
			}
			for _, w := range c.choices(g) {
				if slices.Contains(g.Rivers.Map.Bridges, w.Edge) {
					return []catanCaravanWagon{w}
				}
				next := c
				next.Wagons = append(slices.Clone(c.Wagons), w)
				if rest := path(next, depth-1); rest != nil {
					return append([]catanCaravanWagon{w}, rest...)
				}
			}
			return nil
		}
		wagons := path(*g.Caravans, 7)
		if len(wagons) == 0 {
			t.Fatal("no bridge reachable")
		}
		for _, w := range wagons {
			if e = g.Caravans.place(g, w); e != nil {
				t.Fatal(e)
			}
		}
		edge := wagons[len(wagons)-1].Edge
		if g.Edges[edge].Owner != -1 || g.Caravans.roadWeight(edge) != 2 {
			t.Fatal("empty bridge placement")
		}
		g.Edges[edge].Owner = 0
		g.Edges[edge].Bridge = true
		if e = g.validateRivers(); e != nil {
			t.Fatal("bridge cannot coexist", e)
		}
		if g.roadLength(0) != 2 {
			t.Fatal("bridge must count double", g.roadLength(0))
		}
		for _, change := range []func(*Catan){
			func(g *Catan) { g.Caravans.Rivers = "" },
			func(g *Catan) { g.Edges[0].A = -1 },
			func(g *Catan) { g.Vertices[0].X += 1 },
			func(g *Catan) { g.Caravans.ExtraNumbers[0].Number = 6 },
			func(g *Catan) { g.Caravans.Map.WateringHoles[0] = 0 },
			func(g *Catan) { g.Caravans.Map.Starts[0].From = -1 },
			func(g *Catan) { g.Rivers.Map.Bridges[0] = -1 },
		} {
			bad := clone(*s)
			change(bad.Catan)
			if bad.Catan.validateRivers() == nil && bad.validateCaravans() == nil {
				t.Fatal("corrupt combination accepted")
			}
		}
	}
}
func TestCatanRiversCaravansInventionAndTwoRetreat(t *testing.T) {
	for _, knights := range []bool{false, true} {
		s, e := NewCatanRiversCaravans(2, CatanOptions{}, knights)
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i < 2000 && s.Phase != "catan_turn"; i++ {
			p := twoFullActor(s)
			a, e := s.BotAction(p)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.Apply(p, a); e != nil {
				t.Fatal(e)
			}
		}
		if s.Phase != "catan_turn" {
			t.Fatal("setup incomplete")
		}
		g := s.Catan
		if knights {
			ref := g.Caravans.ExtraNumbers[0]
			other := -1
			for _, c := range g.inventionNumbers() {
				if c.Tile != ref.Tile && c.Number != 3 {
					other = c.Tile
					break
				}
			}
			if other < 0 {
				t.Fatal("no invention partner")
			}
			if e = s.catanInvention(Action{Tile: ref.Tile, Target: other}); e != nil {
				t.Fatal(e)
			}
			riversCaravansRestore(t, s)
			g = s.Catan
			if !g.tileProduces(g.Tiles[ref.Tile], ref.Number) {
				t.Fatal("invention moved extra disc")
			}
			g.CitiesKnights.Invasions = 1
		}
		g.Robber = g.Rivers.Map.Swamps[0]
		if targets := g.twoRetreatTiles(); len(targets) != 1 || targets[0] != g.Rivers.Map.Swamps[1] {
			t.Fatal("combined retreat must use swamps", targets)
		}
	}
}
