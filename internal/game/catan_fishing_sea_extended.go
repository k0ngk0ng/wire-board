package game

import (
	"errors"
	"slices"
	"sort"
)

// The official 2025 combination sheets give 3/4-player replacements only.
// This version pins the missing extended recipes without changing those maps.
const CatanFishingSeaExtendedRecipe = "wire-board-fishing-sea-5-6-v1"

func fishingExtendedSeaScenario(scenario string) bool {
	return scenario == "six_islands" || scenario == "desert" || scenario == "tribe" || scenario == "cloth"
}

type fishingExtendedLakeSwap struct{ tile, resource, number, recipient, recipientResource, recipientNumber int }

func fishingExtendedLakeSwaps(scenario string) []fishingExtendedLakeSwap {
	switch scenario {
	case "desert":
		// Both positions are surrounded by six land hexes on the home region.
		// Retain the displaced discs on ordinary hexes of the same terrain type.
		return []fishingExtendedLakeSwap{{37, 3, 3, 20, 3, 2}, {32, 0, 4, 23, 0, 10}}
	case "tribe":
		// The enlarged mainland has pasture-12 rather than field-12; the field-2
		// supplies the second lake. As in the official tribe recipe, remove discs.
		return []fishingExtendedLakeSwap{{28, 2, 12, -1, 0, 0}, {33, 3, 2, -1, 0, 0}}
	}
	return nil
}
func (g *Catan) fishingExtendedSeaCoasts() ([]CatanFishingCoast, []int, error) {
	if g.Seafarers == nil || !fishingExtendedSeaScenario(g.Seafarers.Scenario) || len(g.Players) < 5 || len(g.Players) > 6 || g.Seafarers.Variable {
		return nil, nil, errors.New("五六人捕鱼海图需要已标记的固定组合地图")
	}
	want := 56
	if g.Seafarers.Scenario == "desert" || g.Seafarers.Scenario == "tribe" {
		want = 63
	}
	if len(g.Tiles) != want {
		return nil, nil, errors.New("五六人捕鱼海图地块数量无效")
	}
	islands := g.findIslands()
	counts := map[int]int{}
	for _, id := range islands {
		if id >= 0 {
			counts[id]++
		}
	}
	order := []int{}
	for id := range counts {
		order = append(order, id)
	}
	sort.Slice(order, func(i, j int) bool {
		if counts[order[i]] != counts[order[j]] {
			return counts[order[i]] < counts[order[j]]
		}
		return order[i] < order[j]
	})
	coasts := g.fishingCoasts(islands)
	switch g.Seafarers.Scenario {
	case "six_islands":
		sizes := []int{}
		for _, id := range order {
			sizes = append(sizes, counts[id])
		}
		if !slices.Equal(sizes, []int{5, 5, 5, 5, 6, 6}) || !slices.Equal(islands, g.Seafarers.Islands) {
			return nil, nil, errors.New("六岛捕鱼岛屿不符")
		}
	case "cloth":
		if g.cloth() == nil || len(g.cloth().HomeTiles) != 26 || len(g.Seafarers.StartIslands) != 2 {
			return nil, nil, errors.New("五六人布匹捕鱼主岛不符")
		}
		order = slices.Clone(g.Seafarers.StartIslands)
		if order[0] == order[1] || counts[order[0]] != 13 || counts[order[1]] != 13 || !slices.Equal(islands, g.Seafarers.Islands) {
			return nil, nil, errors.New("布匹捕鱼需要两座十三格主岛")
		}
		homes := []int{}
		for id, island := range islands {
			if slices.Contains(order, island) {
				homes = append(homes, id)
			}
		}
		if !slices.Equal(homes, g.cloth().HomeTiles) {
			return nil, nil, errors.New("布匹捕鱼主岛地块不符")
		}
		coasts = slices.DeleteFunc(coasts, func(c CatanFishingCoast) bool { return !slices.Contains(order, c.Island) })
	case "tribe":
		if g.tribe() == nil || len(g.Seafarers.StartIslands) != 1 || counts[g.Seafarers.StartIslands[0]] != 29 || !slices.Equal(islands, g.Seafarers.Islands) {
			return nil, nil, errors.New("五六人部落捕鱼主岛不符")
		}
		order = slices.Clone(g.Seafarers.StartIslands)
		coasts = slices.DeleteFunc(coasts, func(c CatanFishingCoast) bool { return c.Island != order[0] })
	case "desert":
		if !slices.Equal(g.Seafarers.Islands, g.findLandRegions(true)) {
			return nil, nil, errors.New("穿越沙漠捕鱼探索区域不符")
		}
	}
	return coasts, order, nil
}

func (g *Catan) makeFishingSeaExtended(placements []CatanFishingGroundPlacement) (*catanFishingMap, error) {
	coasts, groups, err := g.fishingExtendedSeaCoasts()
	if err != nil {
		return nil, err
	}
	if g.SetupStep != 0 {
		return nil, errors.New("捕鱼海图只能在开局前初始化")
	}
	for _, v := range g.Vertices {
		if v.Owner >= 0 || v.Level != 0 {
			return nil, errors.New("捕鱼海图不能覆盖建筑")
		}
	}
	for _, e := range g.Edges {
		if e.Owner >= 0 {
			return nil, errors.New("捕鱼海图不能覆盖路线")
		}
	}
	board := *g
	board.Tiles = slices.Clone(g.Tiles)
	for _, swap := range fishingExtendedLakeSwaps(g.Seafarers.Scenario) {
		tile := &board.Tiles[swap.tile]
		if tile.Resource != swap.resource || tile.Number != swap.number {
			return nil, errors.New("捕鱼湖泊与扩大固定地图不符")
		}
		tile.Resource, tile.Number = catanLake, 0
	}
	if placements == nil && (g.Seafarers.Scenario == "six_islands" || g.Seafarers.Scenario == "cloth") {
		type group struct {
			island  int
			numbers []int
		}
		wanted := []group{}
		if g.Seafarers.Scenario == "six_islands" {
			for i, nums := range [][]int{{4, 8}, {6, 10}, {5}, {9}, {5}, {9}} {
				wanted = append(wanted, group{groups[i], nums})
			}
		} else {
			wanted = []group{{groups[0], []int{4, 5, 6, 9}}, {groups[1], []int{5, 8, 9, 10}}}
		}
		used := map[int]bool{}
		for _, part := range wanted {
			var choose func(int, int) bool
			choose = func(start, number int) bool {
				if number == len(part.numbers) {
					return true
				}
				for i := start; i < len(coasts); i++ {
					c := coasts[i]
					if c.Island != part.island || used[c.Edges[0]] || used[c.Edges[1]] {
						continue
					}
					used[c.Edges[0]], used[c.Edges[1]] = true, true
					placements = append(placements, CatanFishingGroundPlacement{part.numbers[number], c.Edges})
					if choose(i+1, number+1) {
						return true
					}
					placements = placements[:len(placements)-1]
					delete(used, c.Edges[0])
					delete(used, c.Edges[1])
				}
				return false
			}
			if !choose(0, 0) {
				return nil, errors.New("该岛海岸无法放齐不重叠的渔场")
			}
		}
	}
	f, err := makeFishingCoastalMap(coasts, placements, len(g.Players))
	if err != nil {
		return nil, err
	}
	f.SeaRecipe = CatanFishingSeaExtendedRecipe
	for i, swap := range fishingExtendedLakeSwaps(g.Seafarers.Scenario) {
		numbers := []int{2, 3, 11, 12}
		if i == 1 {
			numbers = []int{4, 10}
		}
		f.Lakes = append(f.Lakes, catanFishingLake{Tile: swap.tile, Numbers: numbers})
		if swap.recipient >= 0 {
			f.ExtraNumbers = append(f.ExtraNumbers, catanFishingExtraNumber{Tile: swap.recipient, Number: swap.number})
		}
	}
	if err = f.validateSeaExtended(&board); err != nil {
		return nil, err
	}
	g.Tiles = board.Tiles
	return f, nil
}
func (f catanFishingMap) validateSeaExtended(g *Catan) error {
	coasts, groups, err := g.fishingExtendedSeaCoasts()
	if err != nil {
		return err
	}
	if f.SeaRecipe != CatanFishingSeaExtendedRecipe || f.NumberRecipe != "" {
		return errors.New("五六人捕鱼海图规则版本无效")
	}
	if err = f.validateCoastalGrounds(coasts, len(g.Players)); err != nil {
		return err
	}
	original, err := f.numbersBeforeInvention(g)
	if err != nil {
		return err
	}
	swaps := fishingExtendedLakeSwaps(g.Seafarers.Scenario)
	extras := 0
	lakeIDs := map[int]bool{}
	if len(f.Lakes) != len(swaps) {
		return errors.New("五六人捕鱼湖泊数量不符")
	}
	for i, swap := range swaps {
		lake := f.Lakes[i]
		numbers := []int{2, 3, 11, 12}
		if i == 1 {
			numbers = []int{4, 10}
		}
		if lake.Tile != swap.tile || !slices.Equal(lake.Numbers, numbers) || g.Tiles[swap.tile].Resource != catanLake || g.Tiles[swap.tile].Number != 0 {
			return errors.New("五六人捕鱼湖泊位置或点数不符")
		}
		lakeIDs[swap.tile] = true
		neighbors := map[int]bool{}
		for _, edge := range g.Edges {
			if slices.Contains(edge.Tiles, swap.tile) {
				if len(edge.Tiles) != 2 {
					return errors.New("湖泊必须被陆地包围")
				}
				for _, tile := range edge.Tiles {
					if tile != swap.tile {
						neighbors[tile] = true
						if g.Tiles[tile].Resource == CatanSea || g.Tiles[tile].Resource == CatanFog {
							return errors.New("湖泊不能邻接海洋")
						}
					}
				}
			}
		}
		if len(neighbors) != 6 {
			return errors.New("湖泊周围必须有六格陆地")
		}
		if swap.recipient >= 0 {
			if extras >= len(f.ExtraNumbers) || (f.ExtraNumbers[extras].Tile != swap.recipient || original[CatanNumberToken{swap.recipient, 1}] != swap.number) || g.Tiles[swap.recipient].Resource != swap.recipientResource || original[CatanNumberToken{swap.recipient, 0}] != swap.recipientNumber {
				return errors.New("扩大沙漠捕鱼迁移数字不符")
			}
			extras++
		}
	}
	if len(f.ExtraNumbers) != extras {
		return errors.New("多余的捕鱼生产数字")
	}
	for _, tile := range g.Tiles {
		if tile.Resource == catanLake && !lakeIDs[tile.ID] {
			return errors.New("多余湖泊")
		}
	}
	counts := map[int]int{}
	byNumber := map[int][]int{}
	for _, ground := range f.Grounds {
		for _, coast := range coasts {
			if coast.Edges == ground.Edges {
				counts[coast.Island]++
				byNumber[ground.Number] = append(byNumber[ground.Number], coast.Island)
				break
			}
		}
	}
	switch g.Seafarers.Scenario {
	case "six_islands":
		a, b := byNumber[4][0], byNumber[6][0]
		if a == b || !slices.Contains(groups[:4], a) || !slices.Contains(groups[:4], b) || byNumber[8][0] != a || byNumber[10][0] != b {
			return errors.New("六岛捕鱼的4/8与6/10须各在一座不同小岛")
		}
		for _, id := range groups {
			want := 1
			if id == a || id == b {
				want = 2
			}
			if counts[id] != want {
				return errors.New("六岛各需渔场，只有两座小岛各放两处")
			}
		}
	case "cloth":
		if counts[groups[0]] != 4 || counts[groups[1]] != 4 {
			return errors.New("扩大布匹捕鱼两座主岛各需四处渔场")
		}
	}
	if g.Robber < -1 || g.Robber >= len(g.Tiles) {
		return errors.New("非法强盗位置")
	}
	if g.Robber >= 0 {
		tile := g.Tiles[g.Robber]
		if tile.Resource == CatanSea || tile.Resource == CatanFog {
			return errors.New("强盗必须位于陆地")
		}
		if g.Seafarers.Scenario == "cloth" && !g.clothLand(tile.ID) {
			return errors.New("布匹强盗必须位于主岛")
		}
		if g.Seafarers.Scenario == "tribe" && !g.tribeLand(tile.ID) && tile.ID != 0 && !(g.EventDeck != nil && tile.Resource == CatanDesert) {
			return errors.New("部落强盗必须位于主岛或原起点")
		}
	}
	return nil
}
