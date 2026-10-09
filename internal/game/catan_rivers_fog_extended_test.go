package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanRiversFogExtendedMap(t *testing.T) {
	for _, n := range []int{5, 6} {
		s, err := newCatanRiversFogExtended(n)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if len(g.Rivers.Map.Channels) != 3 || len(g.Rivers.Map.Bridges) != 10 || len(g.Rivers.Map.Swamps) != 2 || g.Rivers.Bank != 152 || len(g.Ports) != 11 || len(g.Seafarers.Fog.Terrain) != 18 || len(g.Seafarers.Fog.Numbers) != 14 || g.Paired == nil {
			t.Fatal("inventory")
		}
		t.Log("map", g.Rivers.Map.Channels)
		for _, c := range g.Rivers.Map.Channels {
			e := g.Edges[c.Outlet]
			sea := len(e.Tiles) == 1
			for _, id := range e.Tiles {
				sea = sea || g.Tiles[id].Resource == CatanSea
			}
			if !sea {
				t.Fatal("inland mouth")
			}
		}
		for i, c := range g.Rivers.Map.Channels {
			for j := i + 1; j < 3; j++ {
				if !g.riversSeparate(c.Tiles, g.Rivers.Map.Channels[j].Tiles) {
					t.Fatal("touching rivers")
				}
			}
		}
		for _, id := range g.Rivers.Map.Bridges {
			if g.edgeTerrain(id, true) || g.edgeTerrain(id, false) {
				t.Fatal("bridge route")
			}
		}
		riversSeaRestore(t, s)
		riverConserved(t, s)
	}
}
func TestCatanRiversFogExtendedNaturalEngine(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanRiversFogExtended(n)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				paired := false
				for step := 0; step < 24000 && !s.Finished; step++ {
					paired = paired || s.Catan.Paired.Second
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
				if !s.Finished || !paired || len(s.Winners) == 0 {
					t.Fatal("unfinished", s.Round)
				}
				riversSeaRestore(t, s)
				riverConserved(t, s)
				t.Log("round", s.Round, "fog left", len(s.Catan.Seafarers.Fog.Terrain))
			})
		}
	}
}
func TestCatanRiversFogExtendedPrivacyAndDiscovery(t *testing.T) {
	for _, n := range []int{5, 6} {
		s, err := newCatanRiversFogExtended(n)
		if err != nil {
			t.Fatal(err)
		}
		b := clone(*s)
		slices.Reverse(b.Catan.Seafarers.Fog.Terrain)
		slices.Reverse(b.Catan.Seafarers.Fog.Numbers)
		for p := -1; p < n; p++ {
			x, _ := json.Marshal(s.View(p))
			y, _ := json.Marshal(b.View(p))
			if string(x) != string(y) {
				t.Fatal("hidden order exposed")
			}
		}
		a, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		other, err := b.BotAction(b.Turn)
		if err != nil || !reflect.DeepEqual(a, other) {
			t.Fatal("bot hidden order")
		}
		g := s.Catan
		g.SetupStep = g.SetupLimit()
		g.Robber = g.Rivers.Map.Swamps[0]
		s.Phase = "catan_turn"
		for _, e := range g.Edges {
			if _, err = s.catanDiscover(0, e.ID); err != nil {
				t.Fatal(err)
			}
			if err = g.validateRivers(); err != nil {
				t.Fatal(err)
			}
		}
		if len(g.Seafarers.Fog.Terrain) != 0 || len(g.Seafarers.Fog.Numbers) != 0 {
			t.Fatal("incomplete discovery")
		}
		riversSeaRestore(t, s)
		riverConserved(t, s)
	}
}

func TestCatanRiversFogExtendedRejectsCorruptSave(t *testing.T) {
	for _, n := range []int{5, 6} {
		s, err := newCatanRiversFogExtended(n)
		if err != nil {
			t.Fatal(err)
		}
		for name, mutate := range map[string]func(*Catan){
			"marker":       func(g *Catan) { g.Rivers.SeaLayout = "unknown" },
			"main number":  func(g *Catan) { g.Tiles[0].Number = 7 },
			"main terrain": func(g *Catan) { g.Tiles[0].Resource = CatanSea },
			"river":        func(g *Catan) { g.Rivers.Map.Channels[0].Tiles[0] = 0 },
			"bridge":       func(g *Catan) { g.Rivers.Map.Bridges[0] = -1 },
			"gold":         func(g *Catan) { g.Rivers.Bank = 100 },
			"pairing":      func(g *Catan) { g.Paired = nil },
			"port":         func(g *Catan) { g.Ports[0].Resource = 9 },
			"fog terrain":  func(g *Catan) { g.Seafarers.Fog.Terrain[0] = catanSwamp },
			"fog number":   func(g *Catan) { g.Seafarers.Fog.Numbers[0] = 7 },
			"fog size":     func(g *Catan) { g.Seafarers.Fog.Terrain = g.Seafarers.Fog.Terrain[1:] },
			"fog start":    func(g *Catan) { g.Seafarers.Fog.StartTiles = nil },
			"fog revealed": func(g *Catan) { g.Tiles[2].Resource = CatanGold; g.Tiles[2].Number = 6 },
			"robber":       func(g *Catan) { g.Robber = 2 },
			"pirate":       func(g *Catan) { g.Seafarers.Pirate = 0 },
			"region":       func(g *Catan) { g.Seafarers.Islands[0] = -1 },
			"variable":     func(g *Catan) { g.Seafarers.Variable = true },
		} {
			b := clone(*s)
			mutate(b.Catan)
			if b.Catan.validateRivers() == nil {
				t.Fatal("accepted", name)
			}
		}
	}
}

func TestCatanRiversFogExtendedShipGoldContinuation(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, phase := range []string{"catan_turn", "catan_roads", "catan_setup_road"} {
			t.Run(fmt.Sprintf("%d/%s", n, phase), func(t *testing.T) {
				s, err := newCatanRiversFogExtended(n)
				if err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				g.SetupStep = g.SetupLimit()
				g.Robber = g.Rivers.Map.Swamps[0]
				s.Turn = 0
				s.Phase = phase
				if phase == "catan_roads" {
					g.FreeRoads = 2
					g.ResumePhase = "catan_turn"
				}
				if phase == "catan_setup_road" {
					g.SetupStep = 0
					g.StartPlayer = 0
				}
				edge := -1
				for _, e := range g.Edges {
					if len(g.fogAtRoute(e.ID)) == 0 || !g.edgeTerrain(e.ID, true) || g.pirateBlocks(e.ID) {
						continue
					}
					for _, v := range []int{e.A, e.B} {
						if !g.landVertex(v) {
							continue
						}
						g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
						g.SetupVertex = v
						if g.canShip(0, e.ID) {
							edge = e.ID
							break
						}
						g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
					}
					if edge >= 0 {
						break
					}
				}
				if edge < 0 {
					t.Fatal("no fog ship")
				}
				pile := g.Seafarers.Fog.Terrain
				at := slices.Index(pile, CatanGold)
				pile[at], pile[len(pile)-1] = pile[len(pile)-1], pile[at]
				if phase == "catan_turn" {
					helperGrant(s, 0, catanPrices["catan_ship"])
				}
				gold := 0
				if g.riverEdge(edge) {
					gold = 1
				}
				if err = s.Apply(0, Action{Type: "catan_ship", Edge: edge}); err != nil {
					t.Fatal(err)
				}
				if s.Phase != "catan_gold" || s.Catan.GoldPending.AfterRoute == nil || s.Catan.GoldPending.AfterRoute.Edge != edge || s.Catan.Rivers.Gold[0] != gold {
					t.Fatal("pending route")
				}
				riversSeaRestore(t, s)
				for s.Phase == "catan_gold" {
					p := s.CatanPendingActor()
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(err)
					}
				}
				if phase == "catan_setup_road" && s.Catan.SetupStep != 1 || phase == "catan_turn" && s.Phase != phase || phase == "catan_roads" && (s.Phase != phase || s.Catan.FreeRoads != 1) {
					t.Fatal("route not resumed")
				}
				riversSeaRestore(t, s)
				riverConserved(t, s)
			})
		}
	}
}

func TestCatanRiversFogExtendedTerrainAndDesert(t *testing.T) {
	for _, n := range []int{5, 6} {
		s, err := newCatanRiversFogExtended(n)
		if err != nil {
			t.Fatal(err)
		}
		base, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "fog", Layout: "fixed"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		actual, want := make([]int, 11), make([]int, 11)
		numbers, expected := make([]int, 13), make([]int, 13)
		for _, id := range s.Catan.Seafarers.Fog.StartTiles {
			actual[s.Catan.Tiles[id].Resource]++
			numbers[s.Catan.Tiles[id].Number]++
			want[base.Catan.Tiles[id].Resource]++
			expected[base.Catan.Tiles[id].Number]++
		}
		want[3] -= 2
		want[catanSwamp] = 2
		expected[2]--
		expected[12]--
		expected[0] += 2
		if !slices.Equal(actual, want) || !slices.Equal(numbers, expected) {
			t.Fatal("visible inventory", actual, numbers)
		}
		g := s.Catan
		g.SetupStep = g.SetupLimit()
		g.Robber = g.Rivers.Map.Swamps[0]
		s.Phase = "catan_turn"
		// The hidden desert consumes a terrain tile but no number; use a route
		// touching exactly one fog hex so another reveal cannot mask the result.
		edge := -1
		for _, e := range g.Edges {
			if len(g.fogAtRoute(e.ID)) == 1 {
				edge = e.ID
				break
			}
		}
		if edge < 0 {
			t.Fatal("no single fog edge")
		}
		pile := g.Seafarers.Fog.Terrain
		at := slices.Index(pile, CatanDesert)
		pile[at], pile[len(pile)-1] = pile[len(pile)-1], pile[at]
		before := len(g.Seafarers.Fog.Numbers)
		if gold, err := s.catanDiscover(0, edge); err != nil || gold != 0 {
			t.Fatal(gold, err)
		}
		if len(g.Seafarers.Fog.Numbers) != before {
			t.Fatal("desert consumed number")
		}
		riversSeaRestore(t, s)
		riverConserved(t, s)
	}
}
