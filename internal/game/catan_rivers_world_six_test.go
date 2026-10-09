package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Exercise the third river ending at an interior sea. Unlike the other two,
// its mouth produces wool and must retain a number disc.
func preparedRiverWorldSixFixture(t *testing.T, n int) *CatanRiversWorldMap {
	t.Helper()
	s, err := newCatanRiversWorld(n)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	base := []riverSeaCandidate{}
	for _, c := range g.Rivers.Map.Channels[:2] {
		base = append(base, riverSeaCandidate{tiles: c.Tiles, outlet: c.Outlet})
	}
	for _, third := range g.riverWorldCandidates(3) {
		if len(g.Edges[third.outlet].Tiles) != 2 {
			continue
		}
		cs := append(slices.Clone(base), third)
		used, forcedSea := map[int]int{}, map[int]bool{}
		valid := true
		for i, c := range cs {
			for j, id := range c.tiles {
				if _, ok := used[id]; ok {
					valid = false
				}
				used[id] = riverWorldChannels(n)[i][j]
			}
			for _, id := range g.Edges[c.outlet].Tiles {
				if id != c.tiles[len(c.tiles)-1] {
					forcedSea[id] = true
				}
			}
		}
		for id := range forcedSea {
			if _, ok := used[id]; ok {
				valid = false
			}
		}
		if !valid {
			continue
		}
		counts := riverWorldTerrain(n)
		for _, r := range used {
			counts[r]--
		}
		counts[CatanSea] -= len(forcedSea)
		terrain, numbers := []int{}, []int{}
		for r, count := range counts {
			for range count {
				terrain = append(terrain, r)
			}
		}
		for number, count := range riverWorldNumbers(n) {
			if number > 0 {
				for range count {
					numbers = append(numbers, number)
				}
			}
		}
		for tries := 0; tries < 100; tries++ {
			shuffle(terrain)
			at := 0
			for i := range g.Tiles {
				r, ok := used[i]
				if !ok {
					if forcedSea[i] {
						r = CatanSea
					} else {
						r = terrain[at]
						at++
					}
				}
				g.Tiles[i].Resource, g.Tiles[i].Number = r, 0
			}
			if g.newWorldNumbers(numbers) != nil {
				continue
			}
			g.Rivers.Map = g.riverWorldMapFor(cs)
			g.Rivers.SeaLayout = riverWorldLayout(n, true)
			g.Seafarers.Islands = g.findIslands()
			if g.validateRivers() == nil {
				return g.RiversWorldMap()
			}
		}
	}
	t.Fatal("no valid third interior river fixture")
	return nil
}

func TestCatanRiversWorldSixMapAndProduction(t *testing.T) {
	for _, n := range []int{5, 6} {
		m := preparedRiverWorldSixFixture(t, n)
		s, err := newCatanRiversWorldWithMap(n, m)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if len(g.Tiles) != 63 || len(g.Rivers.Map.Channels) != 3 || len(g.Rivers.Map.Bridges) != 10 || len(g.Rivers.Map.Swamps) != 2 || g.Rivers.Bank != 152 || len(g.newWorld().Ports) != 11 || g.Paired == nil || !g.Options.FiveSix || g.Robber != -1 || g.Seafarers.Pirate != -1 {
			t.Fatal("extended setup")
		}
		if err = validateRiverWorldInventory(g.Tiles, n, false); err != nil {
			t.Fatal(err)
		}
		c := g.Rivers.Map.Channels[2]
		mouth := g.Tiles[c.Tiles[2]]
		if mouth.Resource != 2 || mouth.Number == 0 || slices.Contains(g.Rivers.Map.Swamps, mouth.ID) || !slices.Contains(g.worldPortEdges(), c.Outlet) || g.edgeTerrain(c.Outlet, true) || g.edgeTerrain(c.Outlet, false) {
			t.Fatal("productive river mouth/port/bridge")
		}
		for s.Phase == "catan_world_ports" {
			s.AutoCatanPending()
			riversSeaRestore(t, s)
		}
		g = s.Catan
		if !reflect.DeepEqual(m, g.RiversWorldMap()) {
			t.Fatal("approved map changed")
		}
		g.SetupStep = g.SetupLimit()
		s.Phase = "catan_turn"
		s.Turn = 0
		g.Vertices[mouth.Vertices[0]].Owner, g.Vertices[mouth.Vertices[0]].Level = 0, 2
		before := g.Players[0].Resources[2]
		if err = s.catanRollProduction(mouth.Number); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Players[0].Resources[2] != before+2 || s.Catan.Rivers.Gold[0] != 0 {
			t.Fatal("third mouth did not produce wool")
		}
		riversSeaRestore(t, s)
		riverConserved(t, s)
		for name, mutate := range map[string]func(*CatanRiversWorldMap){
			"two rivers": func(m *CatanRiversWorldMap) { m.Channels = m.Channels[:2] },
			"overlap":    func(m *CatanRiversWorldMap) { m.Channels[2] = m.Channels[1] },
			"third swamp": func(m *CatanRiversWorldMap) {
				id := m.Channels[2].Tiles[2]
				m.Hexes[id].Resource = catanSwamp
				m.Hexes[id].Number = 0
			},
		} {
			b := clone(*m)
			mutate(&b)
			if ValidateCatanRiversWorldMap(n, &b) == nil {
				t.Fatal("accepted", name)
			}
		}
		bad := clone(*s)
		bad.Catan.Rivers.SeaLayout = "prepared"
		if bad.Catan.validateRivers() == nil {
			t.Fatal("three-player marker accepted")
		}
		m.Hexes[0].Resource = 99
		m.Channels[2].Tiles[0] = -1
		riversSeaRestore(t, s)
	}
}

func TestCatanRiversWorldSixNaturalEngine(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, prepared := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/prepared%t/events%t", n, prepared, events), func(t *testing.T) {
					var s *State
					var err error
					if prepared {
						s, err = newCatanRiversWorldWithMap(n, preparedRiverWorldSixFixture(t, n))
					} else {
						s, err = newCatanRiversWorld(n)
					}
					if err != nil {
						t.Fatal(err)
					}
					m := s.Catan.RiversWorldMap()
					if events {
						if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
							t.Fatal(err)
						}
					}
					second := false
					checkedSecond := map[int]bool{}
					for step := 0; step < 24000 && !s.Finished; step++ {
						if s.Catan.Paired.Second {
							second = true
							if s.Phase == "catan_turn" && !checkedSecond[s.Turn] {
								checkedSecond[s.Turn] = true
								riverReject(t, s, s.Turn, Action{Type: "catan_roll"})
								riverReject(t, s, s.Turn, Action{Type: "catan_trade_offer", Give: []int{1, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0}})
							}
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
					if !s.Finished || !second {
						t.Fatal("unfinished or no paired phase", s.Round)
					}
					if !reflect.DeepEqual(m, s.Catan.RiversWorldMap()) {
						t.Fatal("map changed")
					}
					riversSeaRestore(t, s)
					t.Log("round", s.Round, "winners", s.Winners)
				})
			}
		}
	}
}
