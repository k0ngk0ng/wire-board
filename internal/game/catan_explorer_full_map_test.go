package game

import (
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerFullMapOfficialComponents(t *testing.T) {
	// Independent transcription of English 2025 pp6–7 / German pp23–24.
	for n := 2; n <= 4; n++ {
		for trial := 0; trial < 12; trial++ {
			g, b, err := newCatanExplorerBoard(n, "explorers-and-pirates", "variable")
			if err != nil {
				t.Fatal(err)
			}
			if len(g.Tiles) != 72 || b.Target != 17 || len(b.Starting) != 15 || len(b.Hidden) != 32 || len(b.HarborStarts) != 14 || len(b.Opening) != 0 || b.Council == nil || b.Council.Tile != 33 || b.FramePasture != 30 || b.FrameSea != 41 {
				t.Fatal("full map geometry")
			}
			if !slices.Equal(b.Regions[0], []int{4, 5, 6, 7, 8, 13, 14, 15, 16, 17, 18, 23, 24, 26, 28, 29}) || !slices.Equal(b.Regions[1], []int{46, 47, 49, 51, 52, 57, 58, 59, 60, 61, 62, 67, 68, 69, 70, 71}) {
				t.Fatal("full map region footprint")
			}
			counts := map[int]int{}
			for _, tile := range g.Tiles {
				counts[tile.Resource]++
			}
			if !reflect.DeepEqual(counts, map[int]int{0: 4, 1: 2, 2: 4, 3: 2, 4: 3, CatanSea: 25, CatanFog: 32}) {
				t.Fatal("full board inventory", counts)
			}
			for i, want := range []int{11, 9, 3, 8, 4, 10, 6, 12, 8, 10, 4, 11, 6, 3, 5} {
				if g.Tiles[b.Starting[i]].Number != want {
					t.Fatal("starting numbers")
				}
			}
			if g.Tiles[30].Resource != 2 || g.Tiles[30].Number != 6 {
				t.Fatal("printed frame pasture changed")
			}
			for region := 0; region < 2; region++ {
				terrain := map[int]int{}
				faces := []int{}
				farms := map[string]int{}
				sea := 0
				for _, h := range b.Hidden {
					if h.Region == region {
						terrain[h.Resource]++
						if h.Fish > 0 {
							faces = append(faces, h.Fish)
						}
						if h.Resource == CatanSea && h.Fish == 0 {
							sea++
						}
						if h.Farm != "" {
							farms[h.Farm]++
							if h.Farm == "pirate" && h.PirateDie != 5-region {
								t.Fatal("farm back/die mismatch")
							}
						}
					}
				}
				want := map[int]int{0: 1, 1: 1, 2: 1, 3: 1, 4: 2, CatanGold: 3, CatanSea: 4, CatanDesert: 3}
				if region == 1 {
					want[3], want[4] = 2, 1
				}
				if !reflect.DeepEqual(terrain, want) || sea != 1 || !reflect.DeepEqual(farms, map[string]int{"swift": 1, "pirate": 1, "gold": 1}) || !catanExplorerSameInventory(faces, []int{1 + 3*region, 2 + 3*region, 3 + 3*region}) {
					t.Fatal("regional physical components")
				}
			}
			g, b = explorerMapRestore(t, g, b)
			if v := b.publicView(); v.Unexplored != [2]int{16, 16} || len(v.Farms)+len(v.Shoals) != 0 {
				t.Fatal("hidden components leaked")
			}
			for _, h := range slices.Clone(b.Hidden) {
				before := len(b.Numbers[h.Region])
				if _, err := b.reveal(g, h.Tile); err != nil {
					t.Fatal(err)
				}
				used := 0
				if h.Resource < CatanDesert {
					used = 1
				}
				if len(b.Numbers[h.Region]) != before-used {
					t.Fatal("mission tile consumed resource number")
				}
			}
			g, b = explorerMapRestore(t, g, b)
			v := b.publicView()
			if len(v.Farms) != 6 || len(v.Shoals) != 6 || v.Unexplored != [2]int{} || v.NumbersLeft != [2]int{} {
				t.Fatal("all discoveries")
			}
			gold := 0
			for _, tile := range g.Tiles {
				if tile.Resource == CatanGold {
					gold++
					if tile.Number != 0 {
						t.Fatal("unliberated gold produced")
					}
				}
			}
			if gold != 6 {
				t.Fatal("missing gold fields")
			}
		}
	}
}

func TestCatanExplorerFullMapPrivacyAndInvalidComponents(t *testing.T) {
	g, b, err := newCatanExplorerBoard(3, "explorers-and-pirates", "variable")
	if err != nil {
		t.Fatal(err)
	}
	other := clone(*b)
	// Reverse complete hidden physical pieces, preserving only board locations.
	for region := 0; region < 2; region++ {
		ids := []int{}
		for i, h := range other.Hidden {
			if h.Region == region {
				ids = append(ids, i)
			}
		}
		for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
			a, z := ids[i], ids[j]
			at, zt := other.Hidden[a].Tile, other.Hidden[z].Tile
			other.Hidden[a], other.Hidden[z] = other.Hidden[z], other.Hidden[a]
			other.Hidden[a].Tile, other.Hidden[z].Tile = at, zt
		}
		slices.Reverse(other.Numbers[region])
	}
	if err := other.validate(g); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.publicView(), other.publicView()) {
		t.Fatal("secret layout affected public view")
	}
	for _, mutate := range []func(*catanExplorerBoard){
		func(x *catanExplorerBoard) { x.Target = 15 },
		func(x *catanExplorerBoard) { x.Council.Tile-- },
		func(x *catanExplorerBoard) {
			for i, h := range x.Hidden {
				if h.Resource == CatanGold {
					x.Hidden[i].Resource = CatanSea
					return
				}
			}
		},
		func(x *catanExplorerBoard) {
			for i, h := range x.Hidden {
				if h.Fish > 0 {
					x.Hidden[i].Fish = 0
					return
				}
			}
		},
		func(x *catanExplorerBoard) {
			for i, h := range x.Hidden {
				if h.Farm == "pirate" {
					x.Hidden[i].PirateDie = 6
					return
				}
			}
		},
	} {
		bad := clone(*b)
		mutate(&bad)
		if bad.validate(g) == nil {
			t.Fatal("corrupt full scenario accepted")
		}
	}
	for _, n := range []int{1, 7} {
		if _, _, err := newCatanExplorerBoard(n, "explorers-and-pirates", "variable"); err == nil {
			t.Fatal("unsupported player map")
		}
	}
	if _, _, err := newCatanExplorerBoard(3, "explorers-and-pirates", "fixed"); err == nil {
		t.Fatal("invented fixed map")
	}
}
