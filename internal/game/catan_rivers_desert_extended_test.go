package game

import (
	"fmt"
	"slices"
	"testing"
)

func TestCatanRiversDesertExtended(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, layout := range []string{"rivers-across", "desert-belt"} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/events%t", n, layout, events), func(t *testing.T) {
					s, err := newCatanRiversDesertExtended(n, layout)
					if err != nil {
						t.Fatal(err)
					}
					g := s.Catan
					if len(g.Rivers.Map.Channels) != 3 || len(g.Rivers.Map.Bridges) != 10 || len(g.Rivers.Map.Swamps) != 2 || g.Rivers.Bank != 152 || g.Tiles[g.Robber].Resource != CatanDesert {
						t.Fatal("components")
					}
					for i, c := range g.Rivers.Map.Channels {
						for j := i + 1; j < 3; j++ {
							if !g.riversSeparate(c.Tiles, g.Rivers.Map.Channels[j].Tiles) {
								t.Fatal("rivers touch")
							}
						}
						for _, id := range c.Tiles {
							if g.Tiles[id].Resource != catanSwamp && g.Tiles[id].Number == 0 {
								t.Fatal("missing number")
							}
						}
					}
					if !slices.Contains(g.Seafarers.StartIslands, g.Seafarers.Islands[40]) {
						t.Fatal("home region")
					}
					if events {
						if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
							t.Fatal(err)
						}
					}
					for step := 0; step < 24000 && !s.Finished; step++ {
						p := twoFullActor(s)
						a, e := s.BotAction(p)
						if e != nil {
							t.Fatal(step, e)
						}
						if e = s.Apply(p, a); e != nil {
							t.Fatal(step, s.Phase, a, e)
						}
						if step%131 == 0 {
							riversSeaRestore(t, s)
							riverConserved(t, s)
						}
					}
					if !s.Finished {
						t.Fatal("unfinished", s.Round)
					}
					riversSeaRestore(t, s)
					t.Log("round", s.Round)
				})
			}
		}
	}
}

func TestCatanRiversDesertExtendedRegionsAndSaves(t *testing.T) {
	for _, layout := range []string{"rivers-across", "desert-belt"} {
		s, err := newCatanRiversDesertExtended(6, layout)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		home := g.Seafarers.StartIslands[0]
		if (g.Seafarers.Islands[4] == home) != (layout == "rivers-across") {
			t.Fatal("crossing did not connect northern region")
		}
		for _, v := range g.Vertices {
			if g.seaSetupAllowed(v.ID) != (g.islandAt(v.ID) == home) {
				t.Fatal("setup region")
			}
		}
		for _, c := range g.Rivers.Map.Channels {
			e := g.Edges[c.Outlet]
			sea := len(e.Tiles) == 1
			for _, id := range e.Tiles {
				sea = sea || g.Tiles[id].Resource == CatanSea
			}
			if !sea {
				t.Fatal("inland outlet")
			}
		}
		for _, id := range g.Rivers.Map.Bridges {
			if g.edgeTerrain(id, true) || g.edgeTerrain(id, false) {
				t.Fatal("route on bridge")
			}
		}
		for name, mutate := range map[string]func(*Catan){
			"layout": func(g *Catan) { g.Rivers.SeaLayout = "unknown" }, "river": func(g *Catan) { g.Rivers.Map.Channels[0].Tiles[0] = 0 },
			"outer": func(g *Catan) { g.Tiles[0].Number = 2 }, "home": func(g *Catan) { g.Seafarers.StartIslands = nil },
			"port": func(g *Catan) { g.Ports[0].Resource = 9 }, "bank": func(g *Catan) { g.Rivers.Bank = 100 },
			"number": func(g *Catan) { g.Tiles[40].Number = 7 }, "pair": func(g *Catan) { g.Paired = nil },
		} {
			b := clone(*s)
			mutate(b.Catan)
			if b.Catan.validateRivers() == nil {
				t.Fatal("accepted", name)
			}
		}
	}
}
