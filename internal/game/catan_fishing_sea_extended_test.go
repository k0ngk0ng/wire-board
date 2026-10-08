package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func newFishingExtendedSeaTest(t *testing.T, n int, scene string) *State {
	t.Helper()
	s, e := NewCatanFishingSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: scene, Layout: "fixed"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestCatanFishingExtendedFourSeaMaps(t *testing.T) {
	for _, scene := range []string{"islands", "desert", "tribe", "cloth"} {
		for _, n := range []int{5, 6} {
			t.Run(fmt.Sprintf("%s/%d", scene, n), func(t *testing.T) {
				for sample := 0; sample < 8; sample++ {
					s := newFishingExtendedSeaTest(t, n, scene)
					g := s.Catan
					f := g.Fishing
					if f.Map.SeaRecipe != CatanFishingSeaExtendedRecipe || len(f.Map.Grounds) != 8 || len(f.Tokens.DrawPile) != 44 {
						t.Fatal("extended components missing")
					}
					if e := s.Catan.validateFishing(); e != nil {
						t.Fatal(e)
					}
					raw, e := json.Marshal(s)
					if e != nil {
						t.Fatal(e)
					}
					var restored State
					if e = json.Unmarshal(raw, &restored); e != nil {
						t.Fatal(e)
					}
					if e = restored.Catan.validateFishing(); e != nil {
						t.Fatal(e)
					}
					placements := []CatanFishingGroundPlacement{}
					for _, ground := range f.Map.Grounds {
						placements = append(placements, CatanFishingGroundPlacement{ground.Number, [2]int{ground.Edges[1], ground.Edges[0]}})
					}
					again, e := NewCatanFishingSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: scene}, placements)
					if e != nil {
						t.Fatal(e)
					}
					if !reflect.DeepEqual(f.Map, again.Catan.Fishing.Map) {
						t.Fatal("placement normalization changed recipe")
					}
					if scene == "desert" {
						for _, extra := range f.Map.ExtraNumbers {
							tile := g.Tiles[extra.Tile]
							if !g.tileProduces(tile, extra.Number) || !g.tileProduces(tile, tile.Number) {
								t.Fatal("dual resource production missing")
							}
						}
					}
				}
			})
		}
	}
}
func TestCatanFishingExtendedSeaProductionAndCorruptSaves(t *testing.T) {
	for _, scene := range []string{"islands", "desert", "tribe", "cloth"} {
		t.Run(scene, func(t *testing.T) {
			s := newFishingExtendedSeaTest(t, 6, scene)
			g := s.Catan
			f := g.Fishing.Map
			for _, bad := range []func(*State){
				func(s *State) { s.Catan.Fishing.Map.SeaRecipe = "" },
				func(s *State) { s.Catan.Fishing.Map.SeaRecipe = "unknown" },
				func(s *State) { s.Catan.Fishing.Map.Grounds = s.Catan.Fishing.Map.Grounds[:7] },
				func(s *State) {
					s.Catan.Fishing.Map.Grounds[0].Vertices[0] = s.Catan.Fishing.Map.Grounds[0].Vertices[1]
				},
				func(s *State) { s.Catan.Fishing.Map.Grounds[0].Number = 12 },
				func(s *State) { s.Catan.Fishing.Map.Grounds[0].Edges = s.Catan.Fishing.Map.Grounds[1].Edges },
				func(s *State) { s.Catan.Fishing.Map.NumberRecipe = CatanExtendedNumberRecipe },
				func(s *State) { s.Catan.Seafarers.Variable = true },
			} {
				copy := clone(*s)
				bad(&copy)
				if copy.Catan.validateFishing() == nil {
					t.Fatal("corrupt map accepted")
				}
			}
			for _, lake := range f.Lakes {
				copy := clone(*s)
				copy.Catan.Fishing.Map.Lakes[0].Tile = 0
				if copy.Catan.validateFishing() == nil {
					t.Fatal("coastal lake accepted")
				}
				for _, vertex := range g.Tiles[lake.Tile].Vertices {
					for level := 1; level <= 2; level++ {
						g.Vertices[vertex].Owner, g.Vertices[vertex].Level = 0, level
						for roll := 2; roll <= 12; roll++ {
							for _, robber := range []int{-1, lake.Tile} {
								g.Robber = robber
								due, e := f.production(g, roll)
								if e != nil {
									t.Fatal(e)
								}
								expected := 0
								for _, other := range f.Lakes {
									if other.Tile != robber && slices.Contains(other.Numbers, roll) && slices.Contains(g.Tiles[other.Tile].Vertices, vertex) {
										expected += level
									}
								}
								for _, ground := range f.Grounds {
									if ground.Number == roll && slices.Contains(ground.Vertices[:], vertex) && (ground.SeaTile == nil || *ground.SeaTile != g.Seafarers.Pirate) {
										expected += level
									}
								}
								if due[0] != expected {
									t.Fatal("lake aggregation", due, expected)
								}
							}
						}
						g.Vertices[vertex].Owner, g.Vertices[vertex].Level = -1, 0
					}
				}
			}
			g.Robber = -1
			for _, ground := range f.Grounds {
				v := ground.Vertices[1]
				g.Vertices[v].Owner, g.Vertices[v].Level = 0, 2
				if ground.SeaTile != nil {
					g.Seafarers.Pirate = *ground.SeaTile
				} else {
					g.Seafarers.Pirate = -1
				}
				blocked, e := f.production(g, ground.Number)
				if e != nil {
					t.Fatal(e)
				}
				g.Seafarers.Pirate = -1
				unblocked, e := f.production(g, ground.Number)
				if e != nil {
					t.Fatal(e)
				}
				if ground.SeaTile != nil && unblocked[0]-blocked[0] < 2 {
					t.Fatal("pirate did not block extended ground")
				}
				g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
			}
		})
	}
}
func TestCatanFishingExtendedSeaNaturalGames(t *testing.T) {
	for _, scene := range []string{"islands", "desert", "tribe", "cloth"} {
		for _, n := range []int{5, 6} {
			t.Run(fmt.Sprintf("%s/%d", scene, n), func(t *testing.T) {
				s := newFishingExtendedSeaTest(t, n, scene)
				if n == 6 {
					if e := s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); e != nil {
						t.Fatal(e)
					}
					if e := s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); e != nil {
						t.Fatal(e)
					}
					if e := s.EnableCatanEvents(CatanEventCatalogue); e != nil {
						t.Fatal(e)
					}
				}
				fishActions := 0
				for step := 0; step < 16000 && !s.Finished; step++ {
					p := s.CatanPendingActor()
					if p < 0 {
						p = s.Turn
					}
					if s.Phase == "catan_discard" {
						for i, n := range s.Catan.DiscardDue {
							if n > 0 {
								p = i
								break
							}
						}
					}
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(step, s.Phase, e)
					}
					if len(a.Type) > 11 && a.Type[:11] == "catan_fish_" {
						fishActions++
					}
					if e = s.Apply(p, a); e != nil {
						t.Fatal(step, s.Phase, a, e)
					}
					if step%43 == 0 {
						raw, _ := json.Marshal(s)
						var restore State
						if e = json.Unmarshal(raw, &restore); e != nil {
							t.Fatal(e)
						}
						if e = restore.Catan.validateFishing(); e != nil {
							t.Fatal(e)
						}
						s = &restore
					}
				}
				if !s.Finished {
					t.Fatal("natural game stalled", s.Round, s.Phase)
				}
				if e := s.Catan.validateFishing(); e != nil {
					t.Fatal(e)
				}
				t.Log("completed", s.Round, "rounds", fishActions, "fish actions")
			})
		}
	}
}
