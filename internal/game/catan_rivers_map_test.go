package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanRiversPrintedMap(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			varied := map[string]bool{}
			for range 40 {
				s, err := NewCatan(n, CatanOptions{FiveSix: n > 4})
				if err != nil {
					t.Fatal(err)
				}
				before := clone(*s.Catan)
				f, err := s.Catan.makeRiversMap()
				if err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if err = f.validate(g); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(g.Players, before.Players) || !reflect.DeepEqual(g.Bank, before.Bank) || !reflect.DeepEqual(g.DevDeck, before.DevDeck) || !reflect.DeepEqual(g.Paired, before.Paired) || g.Options != before.Options || g.SetupStep != 0 || g.Robber != -1 {
					t.Fatal("map touched non-map state or selected robber")
				}
				wantBridges, boundary := 7, 2
				riverTiles := []int{3, 5, 8, 10, 13, 15, 17}
				swamps := []int{17, 15}
				if n > 4 {
					wantBridges, boundary = 10, 3
					riverTiles = []int{1, 4, 8, 13, 15, 16, 17, 19, 24, 28}
					swamps = []int{28, 1}
				}
				if len(f.Bridges) != wantBridges || !slices.Equal(f.Swamps, swamps) {
					t.Fatal("printed bridge/swamp inventory")
				}
				riverSet := []int{}
				for _, c := range f.Channels {
					riverSet = append(riverSet, c.Tiles...)
				}
				slices.Sort(riverSet)
				if !slices.Equal(riverSet, riverTiles) {
					t.Fatal("river tile positions")
				}
				coastal, seen := 0, map[int]bool{}
				for _, id := range f.Bridges {
					e := g.Edges[id]
					if seen[id] || e.Owner != -1 {
						t.Fatal("repeated/occupied bridge site")
					}
					seen[id] = true
					if len(e.Tiles) == 1 {
						coastal++
					} else if len(e.Tiles) != 2 {
						t.Fatal("bridge topology")
					}
					for _, tile := range e.Tiles {
						if !slices.Contains(riverTiles, tile) {
							t.Fatal("bridge left river")
						}
					}
				}
				if coastal != boundary {
					t.Fatal("river mouth missing")
				}
				numbers := make([]int, 13)
				for _, tile := range g.Tiles {
					if tile.Number > 0 {
						numbers[tile.Number]++
					}
				}
				if n <= 4 {
					if f.DoubleNumberTile < 0 || g.Tiles[f.DoubleNumberTile].Number != 12 {
						t.Fatal("missing 2/12 shared hex")
					}
					numbers[2]++
					if !slices.Equal(numbers, []int{0, 0, 1, 2, 2, 2, 2, 0, 2, 2, 2, 2, 1}) {
						t.Fatal("base disc inventory", numbers)
					}
				} else {
					if f.DoubleNumberTile != -1 || !slices.Equal(numbers, []int{0, 0, 2, 3, 3, 3, 3, 0, 3, 3, 3, 3, 2}) {
						t.Fatal("extension must not merge 2/12", numbers)
					}
				}
				data, _ := json.Marshal(struct {
					Game *Catan
					Map  *catanRiversMap
				}{g, f})
				var restored struct {
					Game *Catan
					Map  *catanRiversMap
				}
				if err = json.Unmarshal(data, &restored); err != nil || restored.Map.validate(restored.Game) != nil || !reflect.DeepEqual(restored.Game, g) || !reflect.DeepEqual(restored.Map, f) {
					t.Fatal("map restore", err)
				}
				raw := []int{}
				for _, tile := range g.Tiles {
					if !slices.Contains(riverTiles, tile.ID) {
						raw = append(raw, tile.Resource)
					}
				}
				varied[fmt.Sprint(raw)] = true
			}
			if len(varied) < 2 {
				t.Fatal("ordinary terrain was not randomized")
			}
		})
	}
}

func TestCatanRiversMapRejectsUsedOrIncompatibleBoards(t *testing.T) {
	for name, mutate := range map[string]func(*Catan){
		"built":     func(g *Catan) { g.Vertices[0].Owner, g.Vertices[0].Level = 0, 1 },
		"road":      func(g *Catan) { g.Edges[0].Owner = 0 },
		"setup":     func(g *Catan) { g.SetupStep = 1 },
		"seafarers": func(g *Catan) { g.Seafarers = &CatanSeafarers{} },
		"helpers":   func(g *Catan) { g.Options.Helpers = true },
		"fishing":   func(g *Catan) { g.Fishing = &CatanFishing{} },
		"harbors":   func(g *Catan) { g.Harbors = &CatanHarbors{} },
		"friendly":  func(g *Catan) { g.FriendlyRobber = &CatanFriendlyRobber{} },
		"fiveSix":   func(g *Catan) { g.Options.FiveSix = true },
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := NewCatan(3, CatanOptions{})
			mutate(s.Catan)
			before, _ := json.Marshal(s.Catan)
			_, err := s.Catan.makeRiversMap()
			after, _ := json.Marshal(s.Catan)
			if err == nil || string(before) != string(after) {
				t.Fatal("accepted or mutated invalid board")
			}
		})
	}
}

func TestCatanRiversMapRejectsAlteredComponents(t *testing.T) {
	for name, mutate := range map[string]func(*Catan, *catanRiversMap){
		"river terrain": func(g *Catan, f *catanRiversMap) { g.Tiles[f.Channels[0].Tiles[0]].Resource = 0 },
		"swamp number":  func(g *Catan, f *catanRiversMap) { g.Tiles[f.Swamps[0]].Number = 6 },
		"bridge":        func(g *Catan, f *catanRiversMap) { f.Bridges[0] = f.Bridges[1] },
		"outlet":        func(g *Catan, f *catanRiversMap) { f.Channels[0].Outlet = f.Bridges[0] },
		"source":        func(g *Catan, f *catanRiversMap) { f.Channels[0].Tiles[0] = 0 },
		"port":          func(g *Catan, f *catanRiversMap) { g.Ports[0].Edge = f.Bridges[0] },
		"port stock":    func(g *Catan, f *catanRiversMap) { g.Ports[0].Resource = (g.Ports[0].Resource + 1) % 5 },
		"robber":        func(g *Catan, f *catanRiversMap) { g.Robber = 0 },
		"double number": func(g *Catan, f *catanRiversMap) { f.DoubleNumberTile = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := NewCatan(3, CatanOptions{})
			f, err := s.Catan.makeRiversMap()
			if err != nil {
				t.Fatal(err)
			}
			mutate(s.Catan, f)
			if err = f.validate(s.Catan); err == nil {
				t.Fatal("accepted changed components")
			}
		})
	}
}
