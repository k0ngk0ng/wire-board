package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishShoresGame(t *testing.T, n int, layout string) *State {
	t.Helper()
	s, err := NewCatanFishingSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "shores", Layout: layout}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestCatanFishingShoresMaps(t *testing.T) {
	for n := 3; n <= 6; n++ {
		for _, layout := range []string{"fixed", "variable"} {
			if n > 4 && layout == "fixed" {
				continue
			}
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				for sample := 0; sample < 24; sample++ {
					s := fishShoresGame(t, n, layout)
					g := s.Catan
					f := g.Fishing
					lakes, grounds, tokens := 1, 6, 30
					if n > 4 {
						lakes, grounds, tokens = 2, 8, 44
					}
					if len(f.Map.Lakes) != lakes || len(f.Map.Grounds) != grounds || len(f.Tokens.DrawPile) != tokens || g.Robber != -1 || g.victoryTargetFor(0) != 14 {
						t.Fatal("wrong shores components")
					}
					if err := g.validateFishing(); err != nil {
						t.Fatal(err)
					}
					for _, lake := range f.Map.Lakes {
						for _, v := range g.Tiles[lake.Tile].Vertices {
							for level := 1; level <= 2; level++ {
								g.Vertices[v].Owner, g.Vertices[v].Level = 0, level
								for roll := 2; roll <= 12; roll++ {
									due, err := f.Map.production(g, roll)
									if err != nil {
										t.Fatal(err)
									}
									want := 0
									for _, other := range f.Map.Lakes {
										if slices.Contains(other.Numbers, roll) && slices.Contains(g.Tiles[other.Tile].Vertices, v) {
											want += level
										}
									}
									// An inland lake vertex may also touch a coastline ground.
									for _, ground := range f.Map.Grounds {
										if ground.Number == roll && slices.Contains(ground.Vertices[:], v) {
											want += level
										}
									}
									if due[0] != want {
										t.Fatal("lake production", roll, due, want)
									}
								}
								g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
							}
						}
					}
					raw, _ := json.Marshal(s)
					var restored State
					if err := json.Unmarshal(raw, &restored); err != nil {
						t.Fatal(err)
					}
					if err := restored.Catan.validateFishing(); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(s.Catan.Fishing, restored.Catan.Fishing) {
						t.Fatal("restore changed fishing")
					}
				}
			})
		}
	}
}
func TestCatanFishingShoresCorruptionAndBaseIsolation(t *testing.T) {
	for _, n := range []int{3, 4, 6} {
		for _, bad := range []func(*Catan){
			func(g *Catan) { g.Fishing.Map.SeaRecipe = "" },
			func(g *Catan) { g.Fishing.Map.SeaRecipe = CatanFishingSeaExtendedRecipe },
			func(g *Catan) { g.Fishing.Map.Lakes[0].Tile = 0 },
			func(g *Catan) { g.Fishing.Map.Lakes[0].Numbers = []int{7} },
			func(g *Catan) { g.Fishing.Map.Grounds[0] = g.Fishing.Map.Grounds[1] },
			func(g *Catan) { g.Fishing.Map.ExtraNumbers = []catanFishingExtraNumber{{0, 4}} },
			func(g *Catan) { g.Fishing.Map.NumberRecipe = CatanExtendedNumberRecipe },
			func(g *Catan) { g.Tiles[g.Fishing.Map.Lakes[0].Tile].Resource = CatanDesert },
		} {
			s := fishShoresGame(t, n, "")
			bad(s.Catan)
			if s.Catan.validateFishing() == nil {
				t.Fatal("corruption accepted", n)
			}
		}
		base, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "shores"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if base.Catan.Fishing != nil || base.Catan.Robber < 0 {
			t.Fatal("base scenario changed")
		}
		for _, tile := range base.Catan.Tiles {
			if tile.Resource == catanLake {
				t.Fatal("lake leaked to plain shores")
			}
		}
	}
	for _, opts := range []CatanOptions{{Helpers: true}, {AllHelpers: true}} {
		if _, err := NewCatanFishingSeafarers(3, opts, CatanSeafarersSetup{Scenario: "shores"}, nil); err == nil {
			t.Fatal("helpers bypassed")
		}
	}
}
func TestCatanFishingShoresNaturalGames(t *testing.T) {
	for n := 3; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := fishShoresGame(t, n, "")
			if err := s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			if err := s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
				t.Fatal(err)
			}
			for step := 0; step < 12000 && !s.Finished; step++ {
				p := s.CatanPendingActor()
				if p < 0 {
					p = s.Turn
				}
				if s.Phase == "catan_discard" {
					for i, due := range s.Catan.DiscardDue {
						if due > 0 {
							p = i
							break
						}
					}
				}
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if err = s.Apply(p, a); err != nil {
					t.Fatal(step, s.Phase, a, err)
				}
				if step%43 == 0 {
					raw, _ := json.Marshal(s)
					var restored State
					if err = json.Unmarshal(raw, &restored); err != nil {
						t.Fatal(err)
					}
					s = &restored
				}
			}
			if !s.Finished {
				t.Fatal("stalled", s.Round, s.Phase)
			}
			t.Log("finished round", s.Round)
		})
	}
}
