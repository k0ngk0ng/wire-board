package game

import (
	"errors"
	"slices"
)

// Only the initially faceup islands may carry grounds (2025 combination p.2).
// Reconstruct their coastline from the same printed recipe as setup, without
// generating a board or consulting the hidden stacks. Discovered sea must not
// create new legal ground positions; discovered land cannot move old ones.
func (g *Catan) fishingFogCoasts() ([]CatanFishingCoast, error) {
	if g.Seafarers == nil || g.Seafarers.Scenario != "fog" || g.Seafarers.Fog == nil || len(g.Players) < 3 || len(g.Players) > 6 {
		return nil, errors.New("仅已核对的三至六人迷雾群岛捕鱼位置")
	}
	if len(g.Players) > 4 && (!g.Options.FiveSix || g.Paired == nil || g.Seafarers.Variable) {
		return nil, errors.New("五至六人迷雾捕鱼须保留扩充地图和配对回合")
	}
	rows := catanFogThreeRows()
	if len(g.Players) == 4 {
		rows = catanFogFourRows()
	} else if len(g.Players) > 4 {
		rows = catanFogSixRows()
	}
	board := *g
	board.Tiles = slices.Clone(g.Tiles)
	initial, id := []int{}, 0
	for _, row := range rows {
		for _, tile := range row {
			if id >= len(board.Tiles) {
				return nil, errors.New("迷雾捕鱼地图大小不符")
			}
			board.Tiles[id].Resource = tile.resource
			if tile.resource != CatanSea && tile.resource != CatanFog {
				initial = append(initial, id)
			}
			id++
		}
	}
	if id != len(board.Tiles) || !slices.Equal(initial, g.Seafarers.Fog.StartTiles) {
		return nil, errors.New("迷雾捕鱼起始岛屿不符")
	}
	return board.fishingCoasts(board.findIslands()), nil
}

func (g *Catan) makeFishingFog(placements []CatanFishingGroundPlacement) (*catanFishingMap, error) {
	coasts, err := g.fishingFogCoasts()
	if err != nil {
		return nil, err
	}
	f, err := makeFishingCoastalMap(coasts, placements, len(g.Players))
	if err != nil {
		return nil, err
	}
	if err = f.validateFog(g); err != nil {
		return nil, err
	}
	return f, nil
}

func (f catanFishingMap) validateFog(g *Catan) error {
	coasts, err := g.fishingFogCoasts()
	if err != nil {
		return err
	}
	if len(f.Lakes) != 0 || len(f.Grounds) != len(catanFishingGroundNumbers(len(g.Players))) || len(f.ExtraNumbers) != 0 {
		return errors.New("迷雾捕鱼不使用湖泊或额外数字，渔场数量须符合人数")
	}
	if err := f.validateCoastalGrounds(coasts, len(g.Players)); err != nil {
		return err
	}
	for _, tile := range g.Tiles {
		if tile.Resource == catanLake {
			return errors.New("迷雾捕鱼不能放置湖泊")
		}
	}
	if slices.Contains(g.Seafarers.Fog.Terrain, catanLake) {
		return errors.New("迷雾探索堆不能包含湖泊")
	}
	if g.Robber < -1 || g.Robber >= len(g.Tiles) || g.Robber >= 0 && (g.Tiles[g.Robber].Resource == CatanSea || g.Tiles[g.Robber].Resource == CatanFog) {
		return errors.New("非法强盗位置")
	}
	return nil
}
