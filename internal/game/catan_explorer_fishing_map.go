package game

import (
	"errors"
	"slices"
)

// Official 2025 E&P + T&B p1 for 2–4. The separately tagged 5–6 recipe
// below is a site adaptation, not a claim of an official combined setup.
const catanExplorerFishingRules = "explorer-fishing-2025"
const catanExplorerFishingSixRules = "wire-board-explorer-fishing-six-v1"

func catanExplorerFishingRule(players int) string {
	if players > 4 {
		return catanExplorerFishingSixRules
	}
	return catanExplorerFishingRules
}

// Reconstructing the recipe is shared by creation and restore validation.
// Optional lakes are installed before shuffling terrain; no live board swap.
func catanExplorerGeometryFishing(players int, scenario, layout string, cities bool, fishing string, lakes bool) (*Catan, *catanExplorerBoard, error) {
	if fishing != "" && fishing != catanExplorerFishingRule(players) || fishing == "" && lakes {
		return nil, nil, errors.New("探险渔夫地图版本或湖泊选项无效")
	}
	g, b, err := catanExplorerGeometryVariant(players, scenario, layout, cities)
	if err != nil {
		return nil, nil, err
	}
	b.Fishing, b.FishingLakes = fishing, lakes
	if !lakes {
		return g, b, nil
	}
	slots := []int{7} // The printed 12, immediately east of the frame pasture.
	if players > 4 {
		// Site recipe: two inner hexes east of the printed frame pasture.
		// Preserve every other disc: exchange their 5/4 with the printed 12/2
		// before removing those two rare numbers under the lake faces.
		slots = []int{10, 11}
		for i, old := range []int{20, 0} {
			a, z := b.Starting[slots[i]], b.Starting[old]
			g.Tiles[a].Number, g.Tiles[z].Number = g.Tiles[z].Number, g.Tiles[a].Number
		}
	}
	for _, slot := range slots {
		tile := b.Starting[slot]
		// Return one mountain per lake, never the terrain that happened to
		// occupy the chosen number slot. Fixed 2–4 recipes already have ore.
		ore := -1
		for _, id := range b.Starting {
			if g.Tiles[id].Resource == 4 {
				ore = id
				break
			}
		}
		if ore < 0 {
			return nil, nil, errors.New("探险湖泊缺少可归还的山地")
		}
		g.Tiles[ore].Resource, g.Tiles[tile].Resource = g.Tiles[tile].Resource, catanLake
		g.Tiles[tile].Number = 0
	}
	return g, b, nil
}

// Index pairs refer to the first land hex of adjacent printed rows. The
// six Vs on the official diagram occupy only the outer frame (not the
// mission sea). Extended left-coast positions are the labelled site recipe.
func catanExplorerFishingFrame(players int) [][4]int {
	if players > 4 {
		return [][4]int{{0, 4, 1, 3}, {0, 2, 2, 3}, {4, 2, 6, 3}, {6, 2, 9, 3},
			{9, 1, 13, 2}, {13, 1, 16, 2}, {18, 1, 20, 2}, {20, 0, 21, 1}}
	}
	return [][4]int{{0, 4, 1, 3}, {0, 2, 2, 3}, {4, 2, 6, 3},
		{6, 1, 9, 2}, {11, 1, 13, 2}, {13, 0, 14, 1}}
}

func catanExplorerFishingCoasts(g *Catan, b *catanExplorerBoard) ([]CatanFishingCoast, error) {
	if g == nil || b == nil || b.Fishing != catanExplorerFishingRule(len(g.Players)) || len(b.Starting) != 15 && len(b.Starting) != 22 {
		return nil, errors.New("探险渔夫起始岛或地图版本缺失")
	}
	coasts := []CatanFishingCoast{}
	for _, at := range catanExplorerFishingFrame(len(g.Players)) {
		if at[0] >= len(b.Starting) || at[2] >= len(b.Starting) {
			return nil, errors.New("探险渔场起始岛索引无效")
		}
		left, right := b.Starting[at[0]], b.Starting[at[2]]
		a, z := catanFishingSide(g, left, at[1]), catanFishingSide(g, right, at[3])
		if a < 0 || z < 0 || a == z || len(g.Edges[a].Tiles) != 1 || len(g.Edges[z].Tiles) != 1 {
			return nil, errors.New("探险渔场必须位于外框两条不同海岸边")
		}
		key := fishingEdgeKey([2]int{a, z})
		ea, eb := g.Edges[key[0]], g.Edges[key[1]]
		joint, first, last := ea.A, ea.B, eb.A
		if joint != eb.A && joint != eb.B {
			joint, first = ea.B, ea.A
		}
		if joint != eb.A && joint != eb.B {
			return nil, errors.New("探险渔场海岸边必须相接")
		}
		if last == joint {
			last = eb.B
		}
		coasts = append(coasts, CatanFishingCoast{Edges: key, Vertices: [3]int{first, joint, last}, Island: 0, SeaTile: -1})
	}
	return coasts, nil
}

// Private map component only; public room creation remains gated until the
// complete fish controller, bots, views and UI have been accepted together.
func newCatanExplorerFishingMap(players int, scenario, layout string, cities, lakes bool) (*Catan, *catanExplorerBoard, *catanFishingMap, error) {
	g, b, err := newCatanExplorerBoardFishing(players, scenario, layout, cities, catanExplorerFishingRule(players), lakes)
	if err != nil {
		return nil, nil, nil, err
	}
	coasts, err := catanExplorerFishingCoasts(g, b)
	if err != nil {
		return nil, nil, nil, err
	}
	numbers := catanFishingGroundNumbers(players)
	shuffle(numbers)
	f := &catanFishingMap{Lakes: []catanFishingLake{}, Grounds: []catanFishingGround{}}
	for i, coast := range coasts {
		f.Grounds = append(f.Grounds, fishingSeaGround(numbers[i], coast))
	}
	for _, tile := range b.Starting {
		if g.Tiles[tile].Resource == catanLake {
			numbers := []int{2, 3, 11, 12}
			if len(f.Lakes) > 0 {
				numbers = []int{4, 10}
			}
			f.Lakes = append(f.Lakes, catanFishingLake{Tile: tile, Numbers: numbers})
		}
	}
	return g, b, f, f.validateExplorer(g, b)
}

func (f catanFishingMap) validateExplorer(g *Catan, b *catanExplorerBoard) error {
	if b == nil {
		return errors.New("探险捕鱼地图缺少起始岛")
	}
	if err := b.validate(g); err != nil {
		return err
	}
	coasts, err := catanExplorerFishingCoasts(g, b)
	if err != nil {
		return err
	}
	want := 0
	if b.FishingLakes {
		want = 1
		if len(g.Players) > 4 {
			want = 2
		}
	}
	if f.NumberRecipe != "" || len(f.ExtraNumbers) != 0 || len(f.Lakes) != want || g.Robber != -1 || g.Seafarers != nil {
		return errors.New("探险捕鱼湖泊数量、数字或强盗不符")
	}
	seen := map[int]bool{}
	for i, lake := range f.Lakes {
		numbers := []int{2, 3, 11, 12}
		if i == 1 {
			numbers = []int{4, 10}
		}
		slot := 7
		if len(g.Players) > 4 {
			slot = 10 + i
		}
		if lake.Tile != b.Starting[slot] || seen[lake.Tile] || !slices.Equal(lake.Numbers, numbers) || g.Tiles[lake.Tile].Resource != catanLake || g.Tiles[lake.Tile].Number != 0 {
			return errors.New("探险捕鱼湖泊位置或点数不符")
		}
		seen[lake.Tile] = true
	}
	return f.validateCoastalGrounds(coasts, len(g.Players))
}
