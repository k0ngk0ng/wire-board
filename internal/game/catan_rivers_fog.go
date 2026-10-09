package game

import (
	"errors"
	"reflect"
	"slices"
)

// 2025 Rivers + Seafarers, pp. 2–3. Hidden piles and ports retain the
// corresponding ordinary Fog Islands recipe; visible river islands change.
func riverFogRecipe(n int) (map[int]seaTerrain, [][]int, []int, int, []catanFishingExtraNumber) {
	if n == 3 {
		return map[int]seaTerrain{3: {3, 8}, 4: {4, 11}, 9: {0, 5}, 10: {1, 3}, 16: {2, 6}, 17: {catanSwamp, 0}, 23: {0, 4}, 18: {4, 6}, 19: {3, 5}, 25: {1, 11}, 26: {0, 9}, 32: {2, 8}, 33: {3, 10}, 38: {catanSwamp, 0}},
			[][]int{{18, 25, 32, 38}, {4, 10, 17}}, []int{0, 0}, 25, []catanFishingExtraNumber{{Tile: 10, Number: 12}}
	}
	return map[int]seaTerrain{2: {1, 4}, 3: {4, 10}, 4: {4, 11}, 8: {2, 9}, 9: {1, 8}, 10: {0, 12}, 16: {2, 10}, 17: {3, 6}, 23: {catanSwamp, 0}, 30: {0, 5}, 18: {0, 3}, 24: {3, 8}, 25: {4, 4}, 31: {0, 9}, 32: {1, 6}, 37: {2, 5}, 38: {catanSwamp, 0}},
		[][]int{{3, 9, 16, 23}, {25, 32, 38}}, []int{1, 0}, 10, nil
}

// Compare revealed hexes + remaining piles against the original inventories.
// Only the reconstructed board is changed; the live game and hidden order
// remain untouched, including when a view or restore triggers validation.
func (g *Catan) validateRiverFog(board *Catan) error {
	fog, initial := g.Seafarers.Fog, board.Seafarers.Fog
	if fog == nil || initial == nil || len(g.Tiles) != len(board.Tiles) || !slices.Equal(fog.StartTiles, initial.StartTiles) {
		return errors.New("河流迷雾起始地块不符")
	}
	terrain, numbers := slices.Clone(fog.Terrain), slices.Clone(fog.Numbers)
	missing := 0
	for i, want := range board.Tiles {
		if want.Resource != CatanFog {
			continue
		}
		tile := g.Tiles[i]
		if tile.Resource == CatanFog {
			missing++
			continue
		}
		if tile.Resource < 0 || tile.Resource > CatanGold {
			return errors.New("河流迷雾发现地形无效")
		}
		terrain = append(terrain, tile.Resource)
		if tile.Resource == CatanSea || tile.Resource == CatanDesert {
			if tile.Number != 0 {
				return errors.New("河流迷雾海洋或沙漠不能有数字")
			}
		} else {
			numbers = append(numbers, tile.Number)
		}
		board.Tiles[i].Resource = tile.Resource
		board.Tiles[i].Number = tile.Number
	}
	if missing != len(fog.Terrain) {
		return errors.New("河流迷雾探索堆数量不符")
	}
	expectedTerrain, expectedNumbers := slices.Clone(initial.Terrain), slices.Clone(initial.Numbers)
	slices.Sort(terrain)
	slices.Sort(numbers)
	slices.Sort(expectedTerrain)
	slices.Sort(expectedNumbers)
	if !reflect.DeepEqual(terrain, expectedTerrain) || !reflect.DeepEqual(numbers, expectedNumbers) {
		return errors.New("河流迷雾探索组件不守恒")
	}
	return nil
}
