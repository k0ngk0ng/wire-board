package game

import (
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerSpiceMapOfficialComponentsAndDiscovery(t *testing.T) {
	// English 2025 mission guide p16 layout, and German 2025 pp3/20 component
	// faces. The English spice table prints two 9s despite its total of six;
	// the actual six parrot discs on German p3 are 3,4,5,6,9,10.
	for n := 2; n <= 4; n++ {
		for trial := 0; trial < 12; trial++ {
			g, b, err := newCatanExplorerBoard(n, "spices-for-catan", "variable")
			if err != nil {
				t.Fatal(err)
			}
			if len(g.Tiles) != 65 || b.Target != 15 || len(b.Starting) != 15 || len(b.Hidden) != 26 || len(b.HarborStarts) != 14 || len(b.Opening) != 0 || b.Council == nil || b.Council.Tile != 30 || b.FramePasture != 27 || b.FrameSea != 37 {
				t.Fatal("official spice geometry", b.publicView())
			}
			if !slices.Equal(b.Regions[0], []int{4, 5, 6, 7, 12, 13, 14, 15, 16, 21, 22, 24, 26}) || !slices.Equal(b.Regions[1], []int{42, 43, 45, 47, 52, 53, 54, 55, 56, 61, 62, 63, 64}) {
				t.Fatal("official regional footprint")
			}
			counts := map[int]int{}
			for _, h := range g.Tiles {
				counts[h.Resource]++
			}
			if !reflect.DeepEqual(counts, map[int]int{0: 4, 1: 2, 2: 4, 3: 2, 4: 3, CatanSea: 24, CatanFog: 26}) {
				t.Fatal("loose components plus frame and council", counts)
			}
			for i, want := range []int{11, 9, 3, 8, 4, 10, 6, 12, 8, 10, 4, 11, 6, 3, 5} {
				if g.Tiles[b.Starting[i]].Number != want {
					t.Fatal("starting number position", i)
				}
			}
			if g.Tiles[b.FramePasture].Resource != 2 || g.Tiles[b.FramePasture].Number != 6 {
				t.Fatal("fixed pasture changed")
			}
			for region, want := range [2][]int{{3, 4, 5, 6, 9, 10}, {4, 5, 8, 9, 10, 11}} {
				if !catanExplorerSameInventory(b.Numbers[region], want) {
					t.Fatal("physical number components")
				}
			}
			farms := [2]map[string]int{{}, {}}
			faces := [2][]int{}
			ordinarySea := [2]int{}
			terrain := [2]map[int]int{{}, {}}
			for _, h := range b.Hidden {
				terrain[h.Region][h.Resource]++
				if h.Resource == CatanSea && h.Fish == 0 {
					ordinarySea[h.Region]++
				}
				if h.Fish > 0 {
					faces[h.Region] = append(faces[h.Region], h.Fish)
				}
				if h.Farm != "" {
					farms[h.Region][h.Farm]++
					if h.Resource != CatanDesert || h.Number != 0 {
						t.Fatal("farm is nonproductive land")
					}
				}
				if h.Farm == "pirate" && h.PirateDie != []int{5, 4}[h.Region] {
					t.Fatal("physical pirate die/back pairing")
				}
				if h.Farm != "pirate" && h.PirateDie != 0 {
					t.Fatal("non-pirate farm grants chase bonus")
				}
			}
			for region := 0; region < 2; region++ {
				want := map[int]int{0: 1, 1: 1, 2: 1, 3: 1, 4: 2, CatanSea: 4, CatanDesert: 3}
				if region == 1 {
					want[3], want[4] = 2, 1
				}
				if !reflect.DeepEqual(terrain[region], want) || !reflect.DeepEqual(farms[region], map[string]int{"swift": 1, "pirate": 1, "gold": 1}) || ordinarySea[region] != 1 || !catanExplorerSameInventory(faces[region], []int{1 + 3*region, 2 + 3*region, 3 + 3*region}) {
					t.Fatal("regional physical inventory", region, terrain, farms, faces)
				}
			}
			island := g.Tiles[b.Council.Tile]
			if island.Resource != CatanSea || island.Number != 0 || len(b.Council.Anchors) != 2 {
				t.Fatal("council terrain")
			}
			for i, anchor := range b.Council.Anchors {
				v := g.Vertices[anchor]
				dy := -g.HexSize
				if i == 1 {
					dy = g.HexSize
				}
				if math.Abs(v.X-island.X) > 1e-8 || math.Abs(v.Y-island.Y-dy) > 1e-8 || catanExplorerLandVertex(g, anchor) {
					t.Fatal("north/south delivery anchors")
				}
			}
			v := b.publicView()
			if len(v.Farms) != 0 || len(v.Shoals) != 0 || v.Unexplored != [2]int{13, 13} || v.NumbersLeft != [2]int{6, 6} {
				t.Fatal("undiscovered farm/shoal leaked", v)
			}
			g, b = explorerMapRestore(t, g, b)
			for i := len(b.Hidden) - 1; i >= 0; i-- {
				h := b.Hidden[i]
				before := len(b.Numbers[h.Region])
				got, err := b.reveal(g, h.Tile)
				if err != nil {
					t.Fatal(err)
				}
				if got.Farm != h.Farm || got.PirateDie != h.PirateDie || got.Fish != h.Fish {
					t.Fatal("printed component changed on reveal")
				}
				consumed := 0
				if h.Resource < CatanDesert {
					consumed = 1
				}
				if len(b.Numbers[h.Region]) != before-consumed {
					t.Fatal("nonproductive hex consumed a number")
				}
				if err = b.validate(g); err != nil {
					t.Fatal(err)
				}
			}
			g, b = explorerMapRestore(t, g, b)
			v = b.publicView()
			if len(v.Farms) != 6 || len(v.Shoals) != 6 || v.Unexplored != [2]int{} || v.NumbersLeft != [2]int{} {
				t.Fatal("complete public discoveries", v)
			}
			for _, farm := range v.Farms {
				if g.Tiles[farm.Tile].Resource != CatanDesert || g.Tiles[farm.Tile].Number != 0 {
					t.Fatal("farms must not produce resources")
				}
			}
			// Public projections must never alias the authoritative saved state.
			raw, _ := json.Marshal(b)
			v.Farms[0].Ability = "altered"
			v.Council.Anchors[0] = -1
			after, _ := json.Marshal(b)
			if string(raw) != string(after) {
				t.Fatal("public view aliases private state")
			}
		}
	}
}

func TestCatanExplorerSpiceMapHiddenPrivacyAndCorruption(t *testing.T) {
	g, b, err := newCatanExplorerBoard(3, "spices-for-catan", "variable")
	if err != nil {
		t.Fatal(err)
	}
	other := clone(*b)
	// Swap unrevealed non-producing physical farms within one region. This is
	// another legal shuffle; observers must not see their secret abilities.
	swift, gold, pirate := -1, -1, -1
	for i, h := range other.Hidden {
		if h.Region != 0 {
			continue
		}
		switch h.Farm {
		case "swift":
			swift = i
		case "gold":
			gold = i
		case "pirate":
			pirate = i
		}
	}
	other.Hidden[swift].Farm, other.Hidden[gold].Farm = other.Hidden[gold].Farm, other.Hidden[swift].Farm
	if err := other.validate(g); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.publicView(), other.publicView()) {
		t.Fatal("secret farm identity changes public view")
	}
	for _, mutate := range []func(*catanExplorerBoard){
		func(x *catanExplorerBoard) { x.Hidden[pirate].PirateDie = 4 },
		func(x *catanExplorerBoard) { x.Hidden[gold].Farm = "swift" },
		func(x *catanExplorerBoard) { x.Hidden[swift].PirateDie = 5 },
		func(x *catanExplorerBoard) { x.Hidden[gold].Number = 9 },
		func(x *catanExplorerBoard) {
			for i, h := range x.Hidden {
				if h.Fish > 0 {
					x.Hidden[i].Fish = 0
					break
				}
			}
		},
		func(x *catanExplorerBoard) { x.Council.Anchors[0] = x.Starting[0] },
	} {
		bad := clone(*b)
		mutate(&bad)
		if bad.validate(g) == nil {
			t.Fatal("invalid spice components accepted")
		}
	}
	for _, n := range []int{1, 5, 6} {
		if _, _, err := newCatanExplorerBoard(n, "spices-for-catan", "variable"); err == nil {
			t.Fatal("unsupported map size accepted")
		}
	}
	if _, _, err := newCatanExplorerBoard(3, "spices-for-catan", "fixed"); err == nil {
		t.Fatal("invented fixed layout accepted")
	}
	if _, err := newCatanExplorerSpiceState(3); err != nil {
		t.Fatal(err)
	}
}
