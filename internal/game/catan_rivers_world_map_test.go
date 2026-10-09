package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func preparedRiverWorldFixture(t *testing.T, n int) *CatanRiversWorldMap {
	return preparedRiverWorldFixtureContact(t, n, false)
}

func preparedRiverWorldFixtureContact(t *testing.T, n int, touching bool) *CatanRiversWorldMap {
	t.Helper()
	s, err := newCatanRiversWorld(n)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	// Put at least one outlet in the interior, pointing at an actual sea hex.
	long := g.riverWorldCandidates(4)
	short := g.riverWorldCandidates(3)
	for _, a := range long {
		if len(g.Edges[a.outlet].Tiles) != 2 {
			continue
		}
		for _, b := range short {
			if g.riversSeparate(a.tiles, b.tiles) == touching {
				continue
			}
			overlap := false
			for _, id := range a.tiles {
				if slices.Contains(b.tiles, id) {
					overlap = true
				}
			}
			if overlap {
				continue
			}
			c := clone(*s)
			g = c.Catan
			river := map[int]int{}
			for i, channel := range []riverSeaCandidate{a, b} {
				rs := []int{4, 1, 2, catanSwamp}
				if i == 1 {
					rs = []int{4, 1, catanSwamp}
				}
				for j, id := range channel.tiles {
					river[id] = rs[j]
				}
			}
			forcedSea := map[int]bool{}
			for _, channel := range []riverSeaCandidate{a, b} {
				for _, id := range g.Edges[channel.outlet].Tiles {
					if id != channel.tiles[len(channel.tiles)-1] {
						forcedSea[id] = true
					}
				}
			}
			conflict := false
			for id := range forcedSea {
				if _, ok := river[id]; ok {
					conflict = true
				}
			}
			if conflict {
				continue
			}
			terrain := []int{}
			for resource, count := range []int{4, 2, 4, 5, 2, 1, 16, 1} {
				if resource == CatanSea {
					count -= len(forcedSea)
				}
				for range count {
					terrain = append(terrain, resource)
				}
			}
			// One forest becomes gold and one sea becomes desert; stay within inventory.
			for tries := 0; tries < 100; tries++ {
				shuffle(terrain)
				at := 0
				for i := range g.Tiles {
					r, ok := river[i]
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
				numbers := []int{}
				for number, count := range []int{0, 0, 1, 3, 3, 3, 2, 0, 2, 3, 3, 2, 1} {
					for range count {
						numbers = append(numbers, number)
					}
				}
				if g.newWorldNumbers(numbers) != nil {
					continue
				}
				g.Rivers.Map = g.riverTribeMapFor(a, b)
				g.Rivers.SeaLayout = "prepared"
				g.Seafarers.Islands = g.findIslands()
				if g.validateRivers() == nil {
					return g.RiversWorldMap()
				}
			}
		}
	}
	t.Fatal("no valid interior river fixture")
	return nil
}

func TestCatanRiversWorldPreparedMapPreserved(t *testing.T) {
	for _, n := range []int{3, 4} {
		m := preparedRiverWorldFixture(t, n)
		before, _ := json.Marshal(m)
		if err := ValidateCatanRiversWorldMap(n, m); err != nil {
			t.Fatal(err)
		}
		for repeat := 0; repeat < 3; repeat++ {
			s, err := newCatanRiversWorldWithMap(n, m)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(m, s.Catan.RiversWorldMap()) {
				t.Fatal("map rerolled")
			}
			copied := s.Catan.RiversWorldMap()
			copied.Hexes[0].Resource = 99
			copied.Channels[0].Tiles[0] = -1
			if !reflect.DeepEqual(m, s.Catan.RiversWorldMap()) {
				t.Fatal("export aliased")
			}
			for s.Phase == "catan_world_ports" {
				s.AutoCatanPending()
				riversSeaRestore(t, s)
			}
			if !reflect.DeepEqual(m, s.Catan.RiversWorldMap()) {
				t.Fatal("setup changed map")
			}
			if s.Catan.Robber != -1 {
				t.Fatal("robber not on frame")
			}
		}
		after, _ := json.Marshal(m)
		if string(before) != string(after) {
			t.Fatal("input mutated")
		}
		s, err := newCatanRiversWorldWithMap(n, m)
		if err != nil {
			t.Fatal(err)
		}
		m.Hexes[0].Resource = 99
		m.Channels[0].Tiles[0] = -1
		if err = s.Catan.validateRivers(); err != nil {
			t.Fatal("input aliased", err)
		}
	}
}

func TestCatanRiversWorldPreparedInvalid(t *testing.T) {
	m := preparedRiverWorldFixture(t, 3)
	for name, mutate := range map[string]func(*CatanRiversWorldMap){
		"shape":        func(m *CatanRiversWorldMap) { m.Hexes = m.Hexes[1:] },
		"river count":  func(m *CatanRiversWorldMap) { m.Channels = m.Channels[:1] },
		"river length": func(m *CatanRiversWorldMap) { m.Channels[0].Tiles = m.Channels[0].Tiles[:3] },
		"river index":  func(m *CatanRiversWorldMap) { m.Channels[0].Tiles[0] = 999 },
		"river outlet": func(m *CatanRiversWorldMap) { m.Channels[0].Outlet = -1 },
		"terrain":      func(m *CatanRiversWorldMap) { m.Hexes[0].Resource = 99 },
		"number":       func(m *CatanRiversWorldMap) { m.Hexes[0].Number = 7 },
		"river source": func(m *CatanRiversWorldMap) { m.Hexes[m.Channels[0].Tiles[0]].Resource = 0 },
		"swamp disc":   func(m *CatanRiversWorldMap) { m.Hexes[m.Channels[0].Tiles[3]].Number = 2 },
		"gold red": func(m *CatanRiversWorldMap) {
			for i, h := range m.Hexes {
				if h.Resource == CatanGold {
					m.Hexes[i].Number = 6
					return
				}
			}
		},
		"inventory": func(m *CatanRiversWorldMap) {
			for i, h := range m.Hexes {
				if h.Resource == 0 {
					m.Hexes[i].Resource = CatanGold
				}
			}
		},
	} {
		b := clone(*m)
		mutate(&b)
		if ValidateCatanRiversWorldMap(3, &b) == nil {
			t.Fatal("accepted", name)
		}
	}
	for _, n := range []int{0, 2, 5, 6, 7} {
		if ValidateCatanRiversWorldMap(n, m) == nil {
			t.Fatal("mismatched map count accepted", n)
		}
	}
	if ValidateCatanRiversWorldMap(3, nil) == nil {
		t.Fatal("nil map")
	}
	// Rivers-only terrain must never leak into ordinary New World admission.
	if ValidateCatanNewWorldMap(3, &CatanNewWorldMap{Hexes: m.Hexes}) == nil {
		t.Fatal("base admitted river map")
	}
}

func TestCatanRiversWorldPreparedNaturalEngine(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				m := preparedRiverWorldFixture(t, n)
				s, err := newCatanRiversWorldWithMap(n, m)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
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
					if step%97 == 0 {
						riversSeaRestore(t, s)
						riverConserved(t, s)
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round)
				}
				if !reflect.DeepEqual(m, s.Catan.RiversWorldMap()) {
					t.Fatal("game changed approved map")
				}
				riversSeaRestore(t, s)
				t.Log("round", s.Round, "winners", s.Winners)
			})
		}
	}
}

func TestCatanRiversWorldPreparedGold(t *testing.T) {
	m := preparedRiverWorldFixture(t, 3)
	s, err := newCatanRiversWorldWithMap(3, m)
	if err != nil {
		t.Fatal(err)
	}
	for s.Phase == "catan_world_ports" {
		s.AutoCatanPending()
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_turn"
	s.Turn = 0
	id := slices.IndexFunc(g.Tiles, func(t CatanTile) bool { return t.Resource == CatanGold })
	tile := g.Tiles[id]
	g.Vertices[tile.Vertices[0]].Owner, g.Vertices[tile.Vertices[0]].Level = 0, 2
	if err = s.catanRollProduction(tile.Number); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_gold" || g.GoldPending.Claims[0].Count != 2 || g.Rivers.Gold[0] != 0 {
		t.Fatal("gold minted coins")
	}
	riversSeaRestore(t, s)
	before := slices.Clone(s.Catan.Players[0].Resources)
	helperApply(t, s, 0, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 1}})
	if s.Catan.Players[0].Resources[0] != before[0]+1 || s.Catan.Players[0].Resources[4] != before[4]+1 || s.Catan.Rivers.Gold[0] != 0 {
		t.Fatal("gold choice")
	}
	riversSeaRestore(t, s)
	riverConserved(t, s)
}

func TestCatanRiversWorldPreparedTouchingRivers(t *testing.T) {
	m := preparedRiverWorldFixtureContact(t, 4, true)
	s, err := newCatanRiversWorldWithMap(4, m)
	if err != nil {
		t.Fatal(err)
	}
	a, b := s.Catan.Rivers.Map.Channels[0].Tiles, s.Catan.Rivers.Map.Channels[1].Tiles
	if s.Catan.riversSeparate(a, b) {
		t.Fatal("fixture does not touch")
	}
	for s.Phase == "catan_world_ports" {
		s.AutoCatanPending()
		riversSeaRestore(t, s)
	}
	if !reflect.DeepEqual(m, s.Catan.RiversWorldMap()) {
		t.Fatal("touching rivers changed")
	}
}

func TestCatanRiversWorldGeneratedMapApproval(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		m, err := GenerateCatanRiversWorldMap(n)
		if err != nil {
			t.Fatal(err)
		}
		s, err := newCatanRiversWorldWithMap(n, m)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(m, s.Catan.RiversWorldMap()) {
			t.Fatal("generated map changed on approval")
		}
		riversSeaRestore(t, s)
	}
}
