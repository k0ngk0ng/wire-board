package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanRiversShoresExtendedMap(t *testing.T) {
	for _, n := range []int{5, 6} {
		for sample := 0; sample < 12; sample++ {
			s, err := newCatanRiversShores(n)
			if err != nil {
				t.Fatal(n, sample, err)
			}
			g := s.Catan
			if len(g.Tiles) != 56 || len(g.Rivers.Map.Channels) != 3 || len(g.Rivers.Map.Bridges) != 10 || len(g.Rivers.Map.Swamps) != 2 || g.Rivers.Bank != 152 || len(g.Ports) != 11 || g.Paired == nil || !g.Options.FiveSix || g.Seafarers.VictoryPoints != 14 || g.Robber != -1 {
				t.Fatal("extended inventory")
			}
			counts := make([]int, 11)
			for _, tile := range g.Tiles {
				counts[tile.Resource]++
			}
			if !slices.Equal(counts, []int{7, 7, 7, 7, 7, 0, 16, 3, 0, 0, 2}) {
				t.Fatal("terrain", counts)
			}
			for id, want := range map[int]seaTerrain{0: {7, 9}, 7: {4, 11}, 15: {2, 8}, 32: {1, 4}, 41: {0, 2}, 49: {7, 5}, 6: {4, 6}, 23: {1, 12}, 40: {3, 3}, 55: {7, 10}} {
				if g.Tiles[id].Resource != want.resource || g.Tiles[id].Number != want.number {
					t.Fatal("outer terrain", id)
				}
			}
			regions := map[int]int{}
			for _, id := range g.Seafarers.Islands {
				if id >= 0 {
					regions[id]++
				}
			}
			sizes := []int{}
			for _, size := range regions {
				sizes = append(sizes, size)
			}
			slices.Sort(sizes)
			if !slices.Equal(sizes, []int{2, 2, 3, 3, 30}) || g.Seafarers.Islands[6] != g.Seafarers.Islands[23] || g.Seafarers.Islands[40] != g.Seafarers.Islands[55] {
				t.Fatal("exploration regions", sizes)
			}
			for _, c := range g.Rivers.Map.Channels {
				if c.Outlet < 0 {
					t.Fatal("missing outlet")
				}
				sea := len(g.Edges[c.Outlet].Tiles) == 1
				for _, id := range g.Edges[c.Outlet].Tiles {
					sea = sea || g.Tiles[id].Resource == CatanSea
				}
				if !sea {
					t.Fatal("inland outlet", c)
				}
				for j := 1; j < len(c.Tiles); j++ {
					connected := false
					for _, edge := range g.Rivers.Map.Bridges {
						e := g.Edges[edge]
						connected = connected || slices.Contains(e.Tiles, c.Tiles[j-1]) && slices.Contains(e.Tiles, c.Tiles[j])
					}
					if !connected {
						t.Fatal("disconnected river", c)
					}
				}
			}
			for _, edge := range g.Rivers.Map.Bridges {
				if g.edgeTerrain(edge, true) || g.edgeTerrain(edge, false) {
					t.Fatal("bridge accepts road/ship")
				}
			}
			c := g.Rivers.Map.Channels[2]
			mouth := g.Tiles[c.Tiles[len(c.Tiles)-1]]
			if mouth.Resource != 2 || mouth.Number == 0 || slices.Contains(g.Rivers.Map.Swamps, mouth.ID) {
				t.Fatal("third mouth")
			}
			riversSeaRestore(t, s)
			riverConserved(t, s)
		}
	}
}

func TestCatanRiversShoresExtendedNaturalEngine(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanRiversShores(n)
				if err != nil {
					t.Fatal(err)
				}
				tiles := clone(s.Catan.Tiles)
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				second := false
				for step := 0; step < 24000 && !s.Finished; step++ {
					if s.Catan.Paired.Second {
						second = true
					}
					p := twoFullActor(s)
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(step, s.Phase, err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					if step%97 == 0 {
						riversSeaRestore(t, s)
						riverConserved(t, s)
						s.View(-1)
					}
				}
				if !s.Finished || !second || len(s.Winners) == 0 {
					t.Fatal("unfinished/paired/winner", s.Round)
				}
				if !reflect.DeepEqual(tiles, s.Catan.Tiles) {
					t.Fatal("map changed")
				}
				riversSeaRestore(t, s)
				riverConserved(t, s)
				t.Log("round", s.Round, "winners", s.Winners)
			})
		}
	}
}

func TestCatanRiversShoresExtendedRestoreRejects(t *testing.T) {
	for _, n := range []int{5, 6} {
		s, err := newCatanRiversShores(n)
		if err != nil {
			t.Fatal(err)
		}
		for name, mutate := range map[string]func(*Catan){
			"layout":        func(g *Catan) { g.Rivers.SeaLayout = "" },
			"number recipe": func(g *Catan) { g.Rivers.Map.NumberRecipe = "unknown" },
			"sea recipe":    func(g *Catan) { g.Seafarers.NumberRecipe = "unknown" },
			"river":         func(g *Catan) { g.Rivers.Map.Channels[0].Tiles[0] = 0 },
			"third swamp": func(g *Catan) {
				id := g.Rivers.Map.Channels[2].Tiles[2]
				g.Tiles[id].Resource = catanSwamp
				g.Tiles[id].Number = 0
			},
			"mainland terrain": func(g *Catan) { g.Tiles[riverShoresMainlandIDs()[0]].Resource = CatanSea },
			"mainland number":  func(g *Catan) { g.Tiles[riverShoresMainlandIDs()[0]].Number = 7 },
			"outer terrain":    func(g *Catan) { g.Tiles[0].Resource = 0 },
			"outer number":     func(g *Catan) { g.Tiles[0].Number = 3 },
			"region":           func(g *Catan) { g.Seafarers.Islands[23] = g.Seafarers.StartIslands[0] },
			"start region":     func(g *Catan) { g.Seafarers.StartIslands = []int{g.Seafarers.Islands[0]} },
			"port location":    func(g *Catan) { g.Ports[0].Edge = -1 },
			"port inventory":   func(g *Catan) { g.Ports[0].Resource = 9 },
			"pirate":           func(g *Catan) { g.Seafarers.Pirate = 0 },
			"pairing":          func(g *Catan) { g.Paired = nil },
			"gold":             func(g *Catan) { g.Rivers.Bank = 100 },
			"target":           func(g *Catan) { g.Seafarers.VictoryPoints = 12 },
		} {
			b := clone(*s)
			mutate(b.Catan)
			if b.Catan.validateRivers() == nil {
				t.Fatal("accepted", n, name)
			}
		}
	}
}

func riverShoresExtendedTurn(t *testing.T, n int) *State {
	t.Helper()
	s, err := newCatanRiversShores(n)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	g.Robber = g.Rivers.Map.Swamps[0]
	s.Phase = "catan_turn"
	s.Turn = 0
	return s
}

func TestCatanRiversShoresExtendedProductionAndRewards(t *testing.T) {
	for _, n := range []int{5, 6} {
		s := riverShoresExtendedTurn(t, n)
		g := s.Catan
		// The third river mouth produces wool, not swamp silence or coin income.
		c := g.Rivers.Map.Channels[2]
		tile := g.Tiles[c.Tiles[len(c.Tiles)-1]]
		g.Vertices[tile.Vertices[0]].Owner, g.Vertices[tile.Vertices[0]].Level = 0, 2
		if err := s.catanRollProduction(tile.Number); err != nil {
			t.Fatal(err)
		}
		if g.Players[0].Resources[2] != 2 || g.Rivers.Gold[0] != 0 {
			t.Fatal("mouth production")
		}
		riversSeaRestore(t, s)
		riverConserved(t, s)
		s = riverShoresExtendedTurn(t, n)
		g = s.Catan
		tile = g.Tiles[0]
		g.Vertices[tile.Vertices[0]].Owner, g.Vertices[tile.Vertices[0]].Level = 0, 2
		if err := s.catanRollProduction(tile.Number); err != nil {
			t.Fatal(err)
		}
		if s.Phase != "catan_gold" || g.GoldPending.Claims[0].Count != 2 || g.Rivers.Gold[0] != 0 {
			t.Fatal("gold production")
		}
		riversSeaRestore(t, s)
		if err := s.Apply(0, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 1}}); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Players[0].Resources[0] != 1 || s.Catan.Players[0].Resources[4] != 1 || s.Catan.Rivers.Gold[0] != 0 {
			t.Fatal("gold reward")
		}
		riversSeaRestore(t, s)
		riverConserved(t, s)
		s = riverShoresExtendedTurn(t, n)
		g = s.Catan
		for _, pair := range [][2]int{{6, 23}, {40, 55}} {
			before := s.Catan.Seafarers.Seats[0].IslandPoints
			for _, id := range pair {
				g = s.Catan
				v := g.Tiles[id].Vertices[0]
				if g.seaSetupAllowed(v) {
					t.Fatal("outer setup allowed")
				}
				s.catanSettleIsland(0, v, false)
				riversSeaRestore(t, s)
			}
			if s.Catan.Seafarers.Seats[0].IslandPoints != before+2 {
				t.Fatal("duplicate region reward")
			}
		}
	}
}

func TestCatanRiversShoresExtendedPaidRoutes(t *testing.T) {
	for _, n := range []int{5, 6} {
		s := riverShoresExtendedTurn(t, n)
		g := s.Catan
		id := g.Rivers.Map.Bridges[0]
		e := g.Edges[id]
		g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 1
		catanGive(g, 0, 0, 1)
		catanGive(g, 0, 1, 2)
		riverReject(t, s, 0, Action{Type: "catan_road", Edge: id})
		riverReject(t, s, 0, Action{Type: "catan_ship", Edge: id})
		if err := s.Apply(0, Action{Type: "catan_bridge", Edge: id}); err != nil {
			t.Fatal(err)
		}
		g = s.Catan
		if !g.Edges[id].Bridge || g.Rivers.Gold[0] != 3 || g.Rivers.Bank != 149 || g.Players[0].Resources[0] != 0 || g.Players[0].Resources[1] != 0 {
			t.Fatal("bridge payment/reward")
		}
		riversSeaRestore(t, s)
		riverConserved(t, s)
		s = riverShoresExtendedTurn(t, n)
		g = s.Catan
		id = -1
		for _, e := range g.Edges {
			if !g.riverEdge(e.ID) || !g.edgeTerrain(e.ID, true) {
				continue
			}
			for _, v := range []int{e.A, e.B} {
				if !g.landVertex(v) {
					continue
				}
				g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
				if g.canShip(0, e.ID) {
					id = e.ID
					break
				}
				g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
			}
			if id >= 0 {
				break
			}
		}
		if id < 0 {
			t.Fatal("no river ship location")
		}
		catanGive(g, 0, 0, 1)
		catanGive(g, 0, 2, 1)
		if err := s.Apply(0, Action{Type: "catan_ship", Edge: id}); err != nil {
			t.Fatal(err)
		}
		g = s.Catan
		if !g.Edges[id].Ship || g.Rivers.Gold[0] != 1 || g.Rivers.Bank != 151 || g.Players[0].Resources[0] != 0 || g.Players[0].Resources[2] != 0 {
			t.Fatal("ship payment/reward")
		}
		riversSeaRestore(t, s)
		riverConserved(t, s)
		g = s.Catan
		g.Paired.Second = true
		riverReject(t, s, 0, Action{Type: "catan_roll"})
		riverReject(t, s, 0, Action{Type: "catan_trade_offer", Give: []int{1, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0}})
	}
}
