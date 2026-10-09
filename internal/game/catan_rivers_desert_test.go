package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func riverDesertFixture(t *testing.T, n int, layout string) *State {
	t.Helper()
	s, err := newCatanRiversPrintedSeaLayout(n, "desert", layout)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestCatanRiversDesertPrintedMaps(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"rivers-across", "desert-belt"} {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				s := riverDesertFixture(t, n, layout)
				g := s.Catan
				if s.Phase != "catan_setup_settlement" || g.Tiles[g.Robber].Resource != CatanDesert || g.Seafarers.VictoryPoints != 14 || g.Seafarers.IslandBonus != 2 {
					t.Fatal("setup")
				}
				if len(g.Rivers.Map.Bridges) != 7 || len(g.Rivers.Map.Swamps) != 2 {
					t.Fatal("river inventory")
				}
				deserts := 0
				for _, tile := range g.Tiles {
					if tile.Resource == CatanDesert {
						deserts++
					}
					if tile.Resource == catanSwamp && tile.Number != 0 {
						t.Fatal("swamp number")
					}
				}
				wantDeserts := 1
				if layout == "desert-belt" {
					wantDeserts = 3
				}
				if deserts != wantDeserts {
					t.Fatal("desert inventory", deserts)
				}
				for _, c := range g.Rivers.Map.Channels {
					if g.Tiles[c.Tiles[0]].Resource != 4 || g.Tiles[c.Tiles[len(c.Tiles)-1]].Resource != catanSwamp {
						t.Fatal("river endpoints", c)
					}
					e := g.Edges[c.Outlet]
					sea := len(e.Tiles) == 1
					for _, id := range e.Tiles {
						sea = sea || g.Tiles[id].Resource == CatanSea
					}
					if !sea {
						t.Fatal("mouth not on coast", c)
					}
				}
				home := g.Seafarers.StartIslands[0]
				if (g.Seafarers.Islands[0] == home) != (layout == "rivers-across") {
					t.Fatal("desert region split")
				}
				for _, v := range g.Vertices {
					if g.seaSetupAllowed(v.ID) != (g.islandAt(v.ID) == home) {
						t.Fatal("setup escaped home region")
					}
				}
				if layout == "rivers-across" && g.Rivers.Map.DoubleNumberTile != -1 {
					t.Fatal("unprinted double token")
				}
				if layout == "desert-belt" && (g.Rivers.Map.DoubleNumberTile < 0 || g.Tiles[g.Rivers.Map.DoubleNumberTile].Number != 12) {
					t.Fatal("missing double token")
				}
				riversSeaRestore(t, s)
				for name, mutate := range map[string]func(*Catan){
					"layout":         func(g *Catan) { g.Rivers.SeaLayout = "unknown" },
					"missing layout": func(g *Catan) { g.Rivers.SeaLayout = "" },
					"region":         func(g *Catan) { g.Seafarers.Islands = g.findIslands() },
					"home":           func(g *Catan) { g.Seafarers.StartIslands = nil },
					"river":          func(g *Catan) { g.Rivers.Map.Channels[0].Tiles[0] = 0 },
					"terrain":        func(g *Catan) { g.Tiles[0].Resource = 3 },
				} {
					b := clone(*s)
					mutate(b.Catan)
					if b.Catan.validateRivers() == nil {
						t.Fatal("accepted", name)
					}
				}
			})
		}
	}
}
func TestCatanRiversDesertNaturalEngine(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"rivers-across", "desert-belt"} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/events%t", n, layout, events), func(t *testing.T) {
					s := riverDesertFixture(t, n, layout)
					if events {
						if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
							t.Fatal(err)
						}
					}
					for step := 0; step < 16000 && !s.Finished; step++ {
						p := twoFullActor(s)
						a, err := s.BotAction(p)
						if err != nil {
							t.Fatal(step, s.Phase, err)
						}
						if err = s.Apply(p, a); err != nil {
							t.Fatal(step, s.Phase, a, err)
						}
						if step%83 == 0 {
							riversSeaRestore(t, s)
							riverConserved(t, s)
							s.View(-1)
						}
					}
					if !s.Finished {
						t.Fatal("unfinished", s.Round)
					}
					riversSeaRestore(t, s)
					t.Log("round", s.Round, "winner", s.Winners)
				})
			}
		}
	}
}
func TestCatanRiversDesertRegionBonus(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"rivers-across", "desert-belt"} {
			s := riverDesertFixture(t, n, layout)
			g := s.Catan
			home := -1
			far := -1
			outer := -1
			for _, v := range g.Vertices {
				island := g.islandAt(v.ID)
				if island == g.Seafarers.StartIslands[0] {
					home = v.ID
				}
				if island == g.Seafarers.Islands[0] {
					far = v.ID
				}
				outerTile := 3
				if n == 4 {
					outerTile = 4
				}
				if island == g.Seafarers.Islands[outerTile] {
					outer = v.ID
				}
			}
			if home < 0 || far < 0 || outer < 0 {
				t.Fatal("region sites")
			}
			s.catanSettleIsland(0, home, true)
			s.catanSettleIsland(0, far, false)
			want := 0
			if layout == "desert-belt" {
				want = 2
			}
			if g.Seafarers.Seats[0].IslandPoints != want {
				t.Fatal("new mainland region bonus")
			}
			s.catanSettleIsland(0, far, false)
			if g.Seafarers.Seats[0].IslandPoints != want {
				t.Fatal("repeat bonus")
			}
			s.catanSettleIsland(0, outer, false)
			if g.Seafarers.Seats[0].IslandPoints != want+2 {
				t.Fatal("outer island bonus")
			}
			riversSeaRestore(t, s)
		}
	}
}
func TestCatanRiversDesertProductionAndIsolation(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, roll := range []int{2, 12} {
			s := riverDesertFixture(t, n, "desert-belt")
			g := s.Catan
			g.SetupStep = g.SetupLimit()
			s.Phase = "catan_turn"
			tile := g.Tiles[g.Rivers.Map.DoubleNumberTile]
			g.Vertices[tile.Vertices[0]].Owner = 0
			g.Vertices[tile.Vertices[0]].Level = 2
			if err := s.catanRollProduction(roll); err != nil {
				t.Fatal(err)
			}
			if g.Players[0].Resources[tile.Resource] != 2 || g.Rivers.Gold[0] != 0 {
				t.Fatal("double production")
			}
			riversSeaRestore(t, s)
		}
		base, err := NewCatanSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "desert", Layout: "fixed"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		original := clone(*base.Catan)
		riverDesertFixture(t, n, "rivers-across")
		if !reflect.DeepEqual(original, *base.Catan) || base.Catan.Rivers != nil || !slices.Equal(base.Catan.Seafarers.Islands, base.Catan.findLandRegions(true)) {
			t.Fatal("ordinary desert mutated")
		}
	}
}

func TestCatanRiversDesertBridgeAndSettlementActions(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"rivers-across", "desert-belt"} {
			s := riverDesertFixture(t, n, layout)
			g := s.Catan
			g.SetupStep = g.SetupLimit()
			s.Turn = 0
			s.Phase = "catan_turn"
			id := g.Rivers.Map.Bridges[len(g.Rivers.Map.Bridges)-1]
			e := g.Edges[id]
			g.Vertices[e.A].Owner = 0
			g.Vertices[e.A].Level = 1
			catanGive(g, 0, 0, 1)
			catanGive(g, 0, 1, 2)
			riverReject(t, s, 0, Action{Type: "catan_ship", Edge: id})
			riverReject(t, s, 0, Action{Type: "catan_road", Edge: id})
			if err := s.Apply(0, Action{Type: "catan_bridge", Edge: id}); err != nil {
				t.Fatal(err)
			}
			if s.Catan.Rivers.Gold[0] != 3 || !s.Catan.Edges[id].Bridge {
				t.Fatal("bridge reward")
			}
			riversSeaRestore(t, s)
			// Settle the river-bearing land on the far side through the public action.
			s = riverDesertFixture(t, n, layout)
			g = s.Catan
			g.SetupStep = g.SetupLimit()
			s.Turn = 0
			s.Phase = "catan_turn"
			g.Seafarers.Seats[0].HomeIslands = slices.Clone(g.Seafarers.StartIslands)
			g.Seafarers.Seats[0].SettledIslands = slices.Clone(g.Seafarers.StartIslands)
			target := -1
			for _, v := range g.Vertices {
				if g.islandAt(v.ID) != g.Seafarers.Islands[0] || !g.riverVertex(v.ID) {
					continue
				}
				for _, edge := range g.touching(v.ID) {
					if slices.Contains(g.Rivers.Map.Bridges, edge) || !g.edgeTerrain(edge, false) {
						continue
					}
					g.Edges[edge].Owner = 0
					if g.canSettlement(0, v.ID, false) {
						target = v.ID
						break
					}
					g.Edges[edge].Owner = -1
				}
				if target >= 0 {
					break
				}
			}
			if target < 0 {
				t.Fatal("missing far river settlement")
			}
			helperGrant(s, 0, catanPrices["catan_settlement"])
			if err := s.Apply(0, Action{Type: "catan_settlement", Vertex: target}); err != nil {
				t.Fatal(err)
			}
			bonus := 0
			if layout == "desert-belt" {
				bonus = 2
			}
			if s.Catan.Seafarers.Seats[0].IslandPoints != bonus || s.Catan.Rivers.Gold[0] != 1 {
				t.Fatal("river and region rewards")
			}
			helperGrant(s, 0, catanPrices["catan_city"])
			if err := s.Apply(0, Action{Type: "catan_city", Vertex: target}); err != nil {
				t.Fatal(err)
			}
			if s.Catan.Seafarers.Seats[0].IslandPoints != bonus || s.Catan.Rivers.Gold[0] != 1 {
				t.Fatal("upgrade repeated rewards")
			}
			riverConserved(t, s)
			riversSeaRestore(t, s)
		}
	}
}
func TestCatanRiversDesertGoldAndStartRestrictions(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"rivers-across", "desert-belt"} {
			s := riverDesertFixture(t, n, layout)
			g := s.Catan
			// Desert combinations begin setup immediately and cannot reselect a swamp.
			riverReject(t, s, s.Turn, Action{Type: "catan_rivers_start", Tile: g.Rivers.Map.Swamps[0]})
			g.SetupStep = g.SetupLimit()
			s.Turn = 0
			s.Phase = "catan_turn"
			gold := -1
			for _, tile := range g.Tiles {
				if tile.Resource == CatanGold {
					gold = tile.ID
					break
				}
			}
			if gold < 0 {
				t.Fatal("gold field missing")
			}
			tile := g.Tiles[gold]
			g.Vertices[tile.Vertices[0]].Owner = 0
			g.Vertices[tile.Vertices[0]].Level = 2
			if err := s.catanRollProduction(tile.Number); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_gold" || g.GoldPending.Claims[0].Count != 2 || g.Rivers.Gold[0] != 0 {
				t.Fatal("gold minted coins")
			}
			riversSeaRestore(t, s)
			if err := s.Apply(0, Action{Type: "catan_gold", Take: []int{0, 1, 0, 1, 0}}); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_turn" || s.Catan.Rivers.Gold[0] != 0 {
				t.Fatal("gold continuation")
			}
			riverConserved(t, s)
		}
	}
	for _, scenario := range []string{"shores", "fog"} {
		if _, err := newCatanRiversPrintedSeaLayout(3, scenario, "desert-belt"); err == nil {
			t.Fatal("foreign layout accepted")
		}
	}
	for _, layout := range []string{"unknown", "variable"} {
		if _, err := newCatanRiversPrintedSeaLayout(3, "desert", layout); err == nil {
			t.Fatal("unknown layout")
		}
	}
}

func TestCatanRiversDesertLayoutCannotEnterOrdinarySave(t *testing.T) {
	s, err := NewCatanRivers(3, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s.Catan.Rivers.SeaLayout = "desert-belt"
	if s.Catan.validateRivers() == nil {
		t.Fatal("ordinary river accepted a sea layout")
	}
}
