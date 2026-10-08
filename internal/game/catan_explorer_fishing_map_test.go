package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

type explorerFishingMapConfig struct {
	n                int
	scenario, layout string
	cities, lakes    bool
}

func explorerFishingMapConfigs() []explorerFishingMapConfig {
	result := []explorerFishingMapConfig{}
	for n := 2; n <= 6; n++ {
		for _, scenario := range []string{"land-ho", "pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			for _, layout := range []string{"fixed", "variable"} {
				if scenario == "land-ho" && (n > 4 || layout != "fixed") || scenario != "land-ho" && scenario != "pirate-lairs" && layout != "variable" || n > 4 && layout != "variable" {
					continue
				}
				for _, cities := range []bool{false, true} {
					if cities && (n < 3 || layout != "variable" || scenario == "land-ho") {
						continue
					}
					for _, lakes := range []bool{false, true} {
						result = append(result, explorerFishingMapConfig{n, scenario, layout, cities, lakes})
					}
				}
			}
		}
	}
	return result
}

func TestCatanExplorerFishingMapRecipesAndRestore(t *testing.T) {
	for _, cfg := range explorerFishingMapConfigs() {
		t.Run(fmt.Sprintf("%d/%s/%s/cities%t/lakes%t", cfg.n, cfg.scenario, cfg.layout, cfg.cities, cfg.lakes), func(t *testing.T) {
			for range 4 {
				g, b, f, err := newCatanExplorerFishingMap(cfg.n, cfg.scenario, cfg.layout, cfg.cities, cfg.lakes)
				if err != nil {
					t.Fatal(err)
				}
				base, spec, err := catanExplorerGeometryVariant(cfg.n, cfg.scenario, cfg.layout, cfg.cities)
				if err != nil {
					t.Fatal(err)
				}
				// Compare physical inventory independently against the untouched
				// Explorer recipe; only mountains and designated discs disappear.
				terrain, numbers := make([]int, 10), make([]int, 13)
				originalTerrain, originalNumbers := make([]int, 10), make([]int, 13)
				for _, h := range g.Tiles {
					terrain[h.Resource]++
					numbers[h.Number]++
				}
				for _, h := range base.Tiles {
					originalTerrain[h.Resource]++
					originalNumbers[h.Number]++
				}
				wantLakes, wantGrounds := 0, 6
				if cfg.n > 4 {
					wantGrounds = 8
				}
				if cfg.lakes {
					wantLakes = 1
					originalNumbers[12]--
					originalNumbers[0]++
					if cfg.n > 4 {
						wantLakes = 2
						originalNumbers[2]--
						originalNumbers[0]++
					}
				}
				originalTerrain[4] -= wantLakes
				originalTerrain[catanLake] += wantLakes
				if !slices.Equal(terrain, originalTerrain) || !slices.Equal(numbers, originalNumbers) || len(f.Lakes) != wantLakes || len(f.Grounds) != wantGrounds {
					t.Fatal("terrain, disc or component inventory", terrain, originalTerrain, numbers, originalNumbers)
				}
				if b.Target != spec.Target || !reflect.DeepEqual(b.Opening, spec.Opening) || !slices.Equal(b.HarborStarts, spec.HarborStarts) || g.Tiles[b.FramePasture].Resource != 2 || g.Tiles[b.FramePasture].Number != 6 || len(g.Ports) != 0 {
					t.Fatal("base Explorer victory, printed pieces or frame changed")
				}
				for _, lake := range f.Lakes {
					for _, id := range g.Tiles[lake.Tile].Vertices {
						if !catanExplorerLandVertex(g, id) {
							t.Fatal("lake shore must permit a settlement", id)
						}
					}
					for _, e := range g.Edges {
						if slices.Contains(e.Tiles, lake.Tile) && (!catanExplorerLandEdge(g, e.ID) || catanExplorerSeaEdge(g, e.ID) || len(e.Tiles) != 2) {
							t.Fatal("lake must be inland with buildable roads, no ships", e)
						}
					}
				}
				for _, ground := range f.Grounds {
					touch := 0
					for _, h := range g.Tiles {
						if slices.Contains(h.Vertices, ground.Vertices[1]) {
							touch++
						}
					}
					if touch != 2 || ground.SeaTile != nil {
						t.Fatal("ground is not an outer-frame concave V", ground)
					}
				}
				g, b = explorerMapRestore(t, g, b)
				data, _ := json.Marshal(f)
				var restored catanFishingMap
				if err = json.Unmarshal(data, &restored); err != nil {
					t.Fatal(err)
				}
				if err = restored.validateExplorer(g, b); err != nil || !reflect.DeepEqual(*f, restored) {
					t.Fatal("fish map restore", err)
				}
				// All discoveries preserve the starting fishing recipe and the
				// ordinary hidden-region terrain/disc inventories.
				for _, h := range slices.Clone(b.Hidden) {
					if _, err = b.reveal(g, h.Tile); err != nil {
						t.Fatal("reveal", err)
					}
					if err = restored.validateExplorer(g, b); err != nil {
						t.Fatal("fishing after reveal", err)
					}
				}
				public := b.publicView()
				if public.Fishing != catanExplorerFishingRule(cfg.n) || public.FishingLakes != cfg.lakes {
					t.Fatal("missing recipe in map view")
				}
			}
		})
	}
}

func TestCatanExplorerFishingGroundPrintedPositions(t *testing.T) {
	g, b, f, err := newCatanExplorerFishingMap(3, "land-ho", "fixed", false, true)
	if err != nil {
		t.Fatal(err)
	}
	// Independent literal tile IDs for the official Land Ho map, transcribed
	// against the 2025 E&P + T&B figure. Each pair names the two land hexes.
	want := [][2]int{{0, 1}, {0, 6}, {13, 21}, {21, 30}, {38, 45}, {45, 46}}
	for i, ground := range f.Grounds {
		lands := []int{g.Edges[ground.Edges[0]].Tiles[0], g.Edges[ground.Edges[1]].Tiles[0]}
		if !catanExplorerSameInventory(lands, want[i][:]) {
			t.Fatal("printed coast mismatch", i, lands, want[i])
		}
	}
	if len(f.Lakes) != 1 || f.Lakes[0].Tile != 22 || b.Starting[7] != 22 {
		t.Fatal("printed lake not at original 12")
	}
}

func TestCatanExplorerFishingProductionBuildingsAndNoMutation(t *testing.T) {
	for _, n := range []int{2, 4, 6} {
		for _, cities := range []bool{false, true} {
			if cities && n == 2 {
				continue
			}
			g, b, f, err := newCatanExplorerFishingMap(n, "explorers-and-pirates", "variable", cities, true)
			if err != nil {
				t.Fatal(err)
			}
			g.Explorer = &catanExplorer{Board: b}
			if cities {
				g.CitiesKnights = &CatanCitiesKnights{}
			}
			vertices := []int{}
			for _, ground := range f.Grounds {
				vertices = append(vertices, ground.Vertices[:]...)
			}
			for _, lake := range f.Lakes {
				vertices = append(vertices, g.Tiles[lake.Tile].Vertices...)
			}
			for _, id := range vertices {
				for _, kind := range []string{"settlement", "harbor", "city", "neutral", "eliminated"} {
					if kind == "city" && !cities {
						continue
					}
					v := &g.Vertices[id]
					v.Owner, v.Level, v.Harbor = 0, 1, false
					level := 1
					if kind == "harbor" {
						v.Level, v.Harbor = 2, true
					}
					if kind == "city" {
						v.Level, level = 2, 2
					}
					if kind == "neutral" {
						v.Owner = -2
						level = 0
					}
					if kind == "eliminated" {
						g.Players[0].Eliminated = true
						level = 0
					}
					before, _ := json.Marshal(g)
					for total := 2; total <= 12; total++ {
						want := 0
						for _, ground := range f.Grounds {
							if ground.Number == total && slices.Contains(ground.Vertices[:], id) {
								want += level
							}
						}
						for _, lake := range f.Lakes {
							if slices.Contains(lake.Numbers, total) && slices.Contains(g.Tiles[lake.Tile].Vertices, id) {
								want += level
							}
						}
						due, err := f.production(g, total)
						if err != nil || due[0] != want || sum(due) != want {
							t.Fatal(n, cities, kind, id, total, due, want, err)
						}
					}
					after, _ := json.Marshal(g)
					if string(before) != string(after) {
						t.Fatal("claims altered resources or hidden region order")
					}
					v.Owner, v.Level, v.Harbor = -1, 0, false
					g.Players[0].Eliminated = false
				}
			}
		}
	}
}

func TestCatanExplorerFishingMapCorruptionAndBaseIsolation(t *testing.T) {
	for _, n := range []int{2, 6} {
		g, b, f, err := newCatanExplorerFishingMap(n, "pirate-lairs", "variable", false, true)
		if err != nil {
			t.Fatal(err)
		}
		mutations := []func(*Catan, *catanExplorerBoard, *catanFishingMap){
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { b.Fishing = "" },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { b.Fishing = "unknown" },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { b.FishingLakes = false },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { f.Lakes = nil },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { f.Lakes[0].Tile = b.FramePasture },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { f.Lakes[0].Numbers[0] = 7 },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { g.Tiles[f.Lakes[0].Tile].Number = 12 },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { g.Tiles[f.Lakes[0].Tile].Resource = 4 },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { f.Grounds[0].Vertices[0]++ },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { f.Grounds[0].Number = 7 },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { f.Grounds[0] = f.Grounds[1] },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) {
				id := b.FrameSea
				f.Grounds[0].SeaTile = &id
			},
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { f.NumberRecipe = CatanExtendedNumberRecipe },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) {
				f.ExtraNumbers = []catanFishingExtraNumber{{b.FramePasture, 2}}
			},
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) { g.Robber = f.Lakes[0].Tile },
			func(g *Catan, b *catanExplorerBoard, f *catanFishingMap) {
				b.NumberSwaps = map[int]int{f.Lakes[0].Tile: 3}
			},
		}
		for i, mutate := range mutations {
			ng, nb, nf := clone(*g), clone(*b), clone(*f)
			mutate(&ng, &nb, &nf)
			if nf.validateExplorer(&ng, &nb) == nil {
				t.Fatal("accepted corrupt map", n, i)
			}
		}
		base, spec, err := newCatanExplorerBoard(n, "pirate-lairs", "variable")
		if err != nil {
			t.Fatal(err)
		}
		if spec.Fishing != "" || spec.FishingLakes {
			t.Fatal("base enabled fishing")
		}
		for _, tile := range base.Tiles {
			if tile.Resource == catanLake {
				t.Fatal("base gained a lake")
			}
		}
		encoded, _ := json.Marshal(spec)
		var public map[string]any
		if err = json.Unmarshal(encoded, &public); err != nil {
			t.Fatal(err)
		}
		if _, ok := public["fishing"]; ok {
			t.Fatal("legacy map representation changed")
		}
		if err = f.validateExplorer(base, spec); err == nil {
			t.Fatal("fish map accepted on base Explorer")
		}
	}
	// A map component must not accidentally enable an incomplete runtime or
	// allow a saved addon board to run without its token/action controller.
	s, err := NewCatanExplorerLandHo(3)
	if err != nil {
		t.Fatal(err)
	}
	g, b, _, err := newCatanExplorerFishingMap(3, "land-ho", "fixed", false, true)
	if err != nil {
		t.Fatal(err)
	}
	s.Catan.Tiles, s.Catan.Explorer.Board = g.Tiles, b
	if err = s.validateCatanExplorer(); err == nil {
		t.Fatal("map foundation enabled an incomplete Explorer fishing session")
	}
	if _, err = NewCatanFishing(2, CatanOptions{}); err == nil {
		t.Fatal("native two-player token support enabled unfinished base two-player rules")
	}
}

func TestCatanExplorerFishingInventionKeepsLakeAndDiscProvenance(t *testing.T) {
	for _, n := range []int{3, 6} {
		g, b, f, err := newCatanExplorerFishingMap(n, "explorers-and-pirates", "variable", true, true)
		if err != nil {
			t.Fatal(err)
		}
		g.CitiesKnights = &CatanCitiesKnights{}
		ordinary := -1
		for _, tile := range b.Starting {
			if catanInventionNumber(g.Tiles[tile].Number) {
				ordinary = tile
				break
			}
		}
		for _, lake := range f.Lakes {
			before, _ := json.Marshal([]any{g, b, f})
			if err = b.swapNumbers(g, lake.Tile, ordinary); err == nil {
				t.Fatal("lake's multi-number face was treated as a movable disc")
			}
			after, _ := json.Marshal([]any{g, b, f})
			if string(before) != string(after) {
				t.Fatal("rejected lake swap changed state")
			}
		}
		swapped := false
		for _, hidden := range slices.Clone(b.Hidden) {
			if hidden.Resource >= CatanDesert {
				continue
			}
			if _, err = b.reveal(g, hidden.Tile); err != nil {
				t.Fatal(err)
			}
			if !catanInventionNumber(g.Tiles[hidden.Tile].Number) || g.Tiles[hidden.Tile].Number == g.Tiles[ordinary].Number {
				continue
			}
			before := clone(b.Hidden)
			if err = b.swapNumbers(g, ordinary, hidden.Tile); err != nil {
				t.Fatal(err)
			}
			g, b = explorerMapRestore(t, g, b)
			if err = f.validateExplorer(g, b); err != nil || !reflect.DeepEqual(before, b.Hidden) {
				t.Fatal("invention broke fishing or disc provenance", err)
			}
			if err = b.swapNumbers(g, ordinary, hidden.Tile); err != nil || len(b.NumberSwaps) != 0 {
				t.Fatal("restoring original discs retained a fabricated swap", err)
			}
			swapped = true
			break
		}
		if !swapped {
			t.Fatal("no cross-region invention exercised")
		}
	}
}
