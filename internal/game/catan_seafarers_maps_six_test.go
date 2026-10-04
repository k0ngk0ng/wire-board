package game

import (
	"reflect"
	"slices"
	"testing"
)

func TestCatanSeafarersFiveSixScenarioComponents(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, shores := range []bool{false, true} {
			s := seaSixGame(t, n)
			g := s.Catan
			build := (*Catan).makeSeafarersIslandsSix
			if shores {
				build = (*Catan).makeSeafarersShoresSix
			}
			if err := build(g); err != nil {
				t.Fatal(err)
			}
			terrain, numbers := make([]int, 8), make([]int, 13)
			for _, tile := range g.Tiles {
				terrain[tile.Resource]++
				numbers[tile.Number]++
			}
			wantTerrain := []int{7, 6, 7, 6, 6, 0, 24, 0}
			wantNumbers := []int{24, 0, 2, 3, 4, 4, 4, 0, 3, 4, 4, 2, 2}
			regions := []int{5, 5, 5, 5, 6, 6}
			if shores {
				wantTerrain = []int{7, 7, 7, 7, 7, 2, 16, 3}
				wantNumbers = []int{18, 0, 3, 4, 4, 4, 4, 0, 4, 4, 4, 4, 3}
				regions = []int{2, 2, 3, 3, 30}
			}
			if len(g.Tiles) != 56 || !reflect.DeepEqual(terrain, wantTerrain) || !reflect.DeepEqual(numbers, wantNumbers) {
				t.Fatal("component inventory", n, shores, terrain, numbers)
			}
			counts := map[int]int{}
			for id, region := range g.Seafarers.Islands {
				if g.Tiles[id].Resource == CatanSea && region != -1 {
					t.Fatal("sea counted as land")
				}
				if region >= 0 {
					counts[region]++
				}
			}
			sizes := []int{}
			for _, size := range counts {
				sizes = append(sizes, size)
			}
			slices.Sort(sizes)
			if !reflect.DeepEqual(sizes, regions) {
				t.Fatal("printed exploration regions", sizes)
			}
			ports := map[int]int{}
			used := map[int]bool{}
			for _, p := range g.Ports {
				ports[p.Resource]++
				if !g.edgeTerrain(p.Edge, false) || !g.edgeTerrain(p.Edge, true) {
					t.Fatal("noncoastal port")
				}
				e := g.Edges[p.Edge]
				for _, v := range []int{e.A, e.B} {
					if used[v] {
						t.Fatal("ports share vertex")
					}
					used[v] = true
				}
			}
			if !reflect.DeepEqual(ports, map[int]int{-1: 5, 0: 1, 1: 1, 2: 2, 3: 1, 4: 1}) {
				t.Fatal("port inventory", ports)
			}
			if g.Seafarers.Pirate != -1 || g.Paired == nil || g.Paired.Secondary != (g.StartPlayer+3)%n {
				t.Fatal("starting markers")
			}
			for _, v := range g.Vertices {
				if len(g.touching(v.ID)) > 3 {
					t.Fatal("nonphysical vertex")
				}
			}
			if shores {
				if !g.Seafarers.Variable || len(g.Seafarers.StartIslands) != 1 || counts[g.Seafarers.StartIslands[0]] != 30 || g.Tiles[g.Robber].Resource != CatanDesert {
					t.Fatal("main island or robber")
				}
				for _, fixture := range []struct{ id, resource, number int }{{0, 7, 9}, {7, 4, 11}, {15, 2, 8}, {32, 1, 4}, {41, 0, 2}, {49, 7, 5}, {6, 4, 6}, {23, 1, 12}, {40, 3, 3}, {55, 7, 10}} {
					tile := g.Tiles[fixture.id]
					if tile.Resource != fixture.resource || tile.Number != fixture.number {
						t.Fatal("outer island randomized", fixture.id)
					}
					for _, v := range tile.Vertices {
						if g.seaSetupAllowed(v) {
							t.Fatal("starting on outer island")
						}
					}
				}
				for _, e := range g.Edges {
					if len(e.Tiles) == 2 {
						a, b := g.Tiles[e.Tiles[0]].Number, g.Tiles[e.Tiles[1]].Number
						if (a == 6 || a == 8) && (b == 6 || b == 8) {
							t.Fatal("adjacent reds on shores")
						}
					}
				}
			} else {
				if len(g.Seafarers.StartIslands) != 0 || g.Robber != 0 || g.Tiles[g.Robber].Resource != 2 || g.Tiles[g.Robber].Number != 12 {
					t.Fatal("six islands setup")
				}
			}
			// The three/four-player shuffler must not silently alter the official
			// six-player recipe, particularly New Shores' fixed outer islands.
			before := clone(*s)
			if err := g.randomizeSeafarersMap(); err == nil || !reflect.DeepEqual(g, before.Catan) {
				t.Fatal("wrong variable recipe accepted")
			}
			for g.setup() {
				player := s.Turn
				if actor := s.CatanPendingActor(); actor >= 0 {
					player = actor
				}
				action, err := s.BotAction(player)
				if err != nil {
					t.Fatal(err)
				}
				helperApply(t, s, player, action)
				g = s.Catan
			}
			for player, seat := range g.Seafarers.Seats {
				actual := []int{}
				for _, v := range g.Vertices {
					if v.Owner == player && v.Level > 0 {
						id := g.islandAt(v.ID)
						if !slices.Contains(actual, id) {
							actual = append(actual, id)
						}
					}
				}
				slices.Sort(actual)
				homes := append([]int(nil), seat.HomeIslands...)
				slices.Sort(homes)
				if !reflect.DeepEqual(actual, homes) || len(homes) < 1 || len(homes) > 2 || seat.IslandPoints != 0 {
					t.Fatal("personal starting islands", player, actual, seat)
				}
				if shores && !reflect.DeepEqual(homes, g.Seafarers.StartIslands) {
					t.Fatal("starting outside main island")
				}
			}
		}
	}
}

func TestCatanSeafarersShoresFiveSixBlueRegions(t *testing.T) {
	s := seaSixGame(t, 6)
	if err := s.Catan.makeSeafarersShoresSix(); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]int{{6, 23}, {40, 55}} {
		g := s.Catan
		if g.Seafarers.Islands[pair[0]] != g.Seafarers.Islands[pair[1]] {
			t.Fatal("split printed blue region")
		}
		before := g.Seafarers.Seats[0].IslandPoints
		s.catanSettleIsland(0, g.Tiles[pair[0]].Vertices[0], false)
		restored := clone(*s)
		s = &restored
		g = s.Catan
		s.catanSettleIsland(0, g.Tiles[pair[1]].Vertices[0], false)
		if g.Seafarers.Seats[0].IslandPoints != before+2 {
			t.Fatal("awarded same exploration area twice")
		}
	}
}

func seaSixGame(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatan(n, CatanOptions{FiveSix: true})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
