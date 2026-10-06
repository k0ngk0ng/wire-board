package game

import (
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerFishMapOfficialComponentsAndReveals(t *testing.T) {
	excluded := map[int]bool{}
	for n := 2; n <= 4; n++ {
		for trial := 0; trial < 12; trial++ {
			g, b, err := newCatanExplorerBoard(n, "fish-for-catan", "variable")
			if err != nil {
				t.Fatal(err)
			}
			if len(g.Tiles) != 58 || b.Target != 15 || len(b.Starting) != 15 || len(b.HarborStarts) != 14 || b.Council == nil || b.Council.Tile != 27 || len(b.Council.Anchors) != 2 || len(b.Opening) != 0 {
				t.Fatal("fish scenario geometry/target", b.publicView())
			}
			if !slices.Equal(b.Regions[0], []int{4, 5, 6, 11, 12, 13, 14, 19, 20, 22, 23}) || !slices.Equal(b.Regions[1], []int{38, 39, 41, 42, 47, 48, 49, 50, 55, 56, 57}) {
				t.Fatal("wrong official region footprint")
			}
			counts := map[int]int{}
			for _, tile := range g.Tiles {
				counts[tile.Resource]++
			}
			if !reflect.DeepEqual(counts, map[int]int{0: 4, 1: 2, 2: 4, 3: 2, 4: 3, CatanSea: 21, CatanFog: 22}) {
				t.Fatal("loose + printed + council inventory", counts)
			}
			for i, want := range []int{11, 9, 3, 8, 4, 10, 6, 12, 8, 10, 4, 11, 6, 3, 5} {
				if g.Tiles[b.Starting[i]].Number != want {
					t.Fatal("starting number position", i)
				}
			}
			if g.Tiles[b.FramePasture].Resource != 2 || g.Tiles[b.FramePasture].Number != 6 || g.Tiles[b.FrameSea].Resource != CatanSea {
				t.Fatal("printed frame changed")
			}
			island := g.Tiles[b.Council.Tile]
			if island.Resource != CatanSea || island.Number != 0 {
				t.Fatal("council is not productive land")
			}
			for i, anchor := range b.Council.Anchors {
				v := g.Vertices[anchor]
				dy := -g.HexSize
				if i == 1 {
					dy = g.HexSize
				}
				if math.Abs(v.X-island.X) > 1e-8 || math.Abs(v.Y-island.Y-dy) > 1e-8 || catanExplorerLandVertex(g, anchor) {
					t.Fatal("anchor must be north/south sea vertex")
				}
			}
			// Council delivery anchors are distinct from the starting coast. The island
			// touches land only on its western side, whose road/building rules are shared.
			coast, seaEdges := 0, 0
			for _, edge := range g.Edges {
				if slices.Contains(edge.Tiles, island.ID) {
					if !catanExplorerSeaEdge(g, edge.ID) {
						t.Fatal("council coastline not navigable")
					}
					seaEdges++
					if len(edge.Tiles) == 2 {
						other := edge.Tiles[0]
						if other == island.ID {
							other = edge.Tiles[1]
						}
						if g.Tiles[other].Resource < CatanSea {
							coast++
						}
					}
				}
			}
			if coast != 1 || seaEdges != 6 {
				t.Fatal("council adjacency", coast, seaEdges)
			}
			faces := map[int]bool{}
			regionCounts := [2]map[int]int{{}, {}}
			for _, h := range b.Hidden {
				regionCounts[h.Region][h.Resource]++
				if h.Fish > 0 {
					faces[h.Fish] = true
				}
			}
			if !reflect.DeepEqual(regionCounts[0], map[int]int{0: 1, 1: 1, 2: 1, 3: 1, 4: 2, CatanSea: 2, CatanGold: 3}) || !reflect.DeepEqual(regionCounts[1], map[int]int{0: 1, 1: 1, 2: 1, 3: 2, 4: 1, CatanSea: 3, CatanGold: 2}) {
				t.Fatal("regional fish/gold/ordinary inventory", regionCounts)
			}
			if len(faces) != 5 || !faces[4] || !faces[5] || !faces[6] {
				t.Fatal("physical fish die faces", faces)
			}
			for face := 1; face <= 3; face++ {
				if !faces[face] {
					excluded[face] = true
				}
			}
			if len(b.publicView().Shoals) != 0 {
				t.Fatal("hidden shoals leaked")
			}
			g, b = explorerMapRestore(t, g, b)
			for i := len(b.Hidden) - 1; i >= 0; i-- {
				h := b.Hidden[i]
				before := len(b.Numbers[h.Region])
				reveal, err := b.reveal(g, h.Tile)
				if err != nil {
					t.Fatal(err)
				}
				if reveal.Fish != h.Fish {
					t.Fatal("printed face changed on discovery")
				}
				if h.Fish > 0 {
					if g.Tiles[h.Tile].Resource != CatanSea || g.Tiles[h.Tile].Number != 0 || len(b.Numbers[h.Region]) != before {
						t.Fatal("shoal consumed ordinary number or became production")
					}
					found := false
					for _, public := range b.publicView().Shoals {
						if public.Tile == h.Tile && public.Number == h.Fish {
							found = true
						}
					}
					if !found {
						t.Fatal("discovered shoal missing its public die face")
					}
				}
				if i%7 == 0 {
					g, b = explorerMapRestore(t, g, b)
				}
			}
			if len(b.publicView().Shoals) != 5 || len(b.Numbers[0])+len(b.Numbers[1]) != 0 || len(b.Liberated) != 0 {
				t.Fatal("full reveal counts")
			}
		}
	}
	// All choices are possible, but randomness coverage is not required for
	// correctness: never turn a rare random selection into a flaky CI failure.
	t.Logf("random omitted parrot faces observed: %v", excluded)
}

func TestCatanExplorerFishMapPrivacyAndTamperRejection(t *testing.T) {
	g, b, err := newCatanExplorerBoard(3, "fish-for-catan", "variable")
	if err != nil {
		t.Fatal(err)
	}
	other := clone(*b)
	ids := []int{}
	for i, h := range other.Hidden {
		if h.Region == 0 && h.Fish > 0 {
			ids = append(ids, i)
		}
	}
	// Replace which physical parrot face was left in the box, preserving all
	// publicly visible board state. Public metadata must not disclose the choice.
	used := map[int]bool{}
	for _, id := range ids {
		used[other.Hidden[id].Fish] = true
	}
	for face := 1; face <= 3; face++ {
		if !used[face] {
			other.Hidden[ids[0]].Fish = face
			break
		}
	}
	if err := other.validate(g); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.publicView(), other.publicView()) {
		t.Fatal("hidden omitted fish face leaked")
	}
	view := b.publicView()
	view.Council.Anchors[0] = -1
	if b.Council.Anchors[0] < 0 {
		t.Fatal("public council aliases authoritative state")
	}
	for _, mutate := range []func(*Catan, *catanExplorerBoard){
		func(_ *Catan, b *catanExplorerBoard) { b.Council = nil },
		func(_ *Catan, b *catanExplorerBoard) { b.Council.Tile = b.FrameSea },
		func(g *Catan, b *catanExplorerBoard) { b.Council.Anchors[0] = g.Tiles[b.Council.Tile].Vertices[0] },
		func(_ *Catan, b *catanExplorerBoard) { b.Hidden[ids[0]].Fish = b.Hidden[ids[1]].Fish },
		func(_ *Catan, b *catanExplorerBoard) { b.Hidden[ids[0]].Fish = 4 },
		func(_ *Catan, b *catanExplorerBoard) { b.Hidden[ids[0]].Fish = 0 },
		func(_ *Catan, b *catanExplorerBoard) {
			for i, h := range b.Hidden {
				if h.Resource == CatanGold {
					b.Hidden[i].Fish = 1
					return
				}
			}
		},
		func(g *Catan, b *catanExplorerBoard) { g.Tiles[b.Hidden[ids[0]].Tile].Number = b.Hidden[ids[0]].Fish },
	} {
		ng, nb := clone(*g), clone(*b)
		mutate(&ng, &nb)
		if err := nb.validate(&ng); err == nil {
			t.Fatal("damaged fish map accepted")
		}
	}
	// Normal scenarios cannot gain fish/council components from a corrupt save.
	ng, nb, err := newCatanExplorerBoard(3, "pirate-lairs", "fixed")
	if err != nil {
		t.Fatal(err)
	}
	nb.Council = clone(b.Council)
	if err := nb.validate(ng); err == nil {
		t.Fatal("council admitted into lairs scenario")
	}
	nb.Council = nil
	nb.Hidden[0].Fish = 1
	if err := nb.validate(ng); err == nil {
		t.Fatal("fish admitted into lairs scenario")
	}
	before, _ := json.Marshal(b)
	if _, err := b.reveal(g, b.Council.Tile); err == nil {
		t.Fatal("revealed council as fog")
	}
	after, _ := json.Marshal(b)
	if string(before) != string(after) {
		t.Fatal("invalid reveal mutated map")
	}
}
