package game

import (
	"errors"
	"slices"
)

// 2025 Fishing + Seafarers p.2 identifies the printed terrain and number.
// Fixed Through the Desert: 3p field-2 -> lake, 2 -> pasture-12;
// 4p hills-11 -> lake, 11 -> hills-2. Variable layouts need a separate recipe.
type fishingDesertRecipe struct {
	lake, resource, moved, recipient, recipientResource, original int
}

func (g *Catan) fishingDesertRecipe() (fishingDesertRecipe, error) {
	if g.Seafarers == nil || g.Seafarers.Scenario != "desert" || g.Seafarers.Variable {
		return fishingDesertRecipe{}, errors.New("仅已核对的三/四人固定穿越沙漠捕鱼布局")
	}
	switch len(g.Players) {
	case 3:
		return fishingDesertRecipe{22, 3, 2, 8, 2, 12}, nil
	case 4:
		return fishingDesertRecipe{20, 1, 11, 17, 1, 2}, nil
	default:
		return fishingDesertRecipe{}, errors.New("穿越沙漠捕鱼五至六人配方尚未接入")
	}
}

func (g *Catan) makeFishingDesert(placements []CatanFishingGroundPlacement) (*catanFishingMap, error) {
	r, err := g.fishingDesertRecipe()
	if err != nil {
		return nil, err
	}
	if g.SetupStep != 0 || r.lake >= len(g.Tiles) || r.recipient >= len(g.Tiles) ||
		g.Tiles[r.lake].Resource != r.resource || g.Tiles[r.lake].Number != r.moved ||
		g.Tiles[r.recipient].Resource != r.recipientResource || g.Tiles[r.recipient].Number != r.original {
		return nil, errors.New("穿越沙漠捕鱼替换地块与固定地图不符")
	}
	for _, v := range g.Vertices {
		if v.Owner >= 0 || v.Level != 0 {
			return nil, errors.New("捕鱼地图不能覆盖已有建筑")
		}
	}
	for _, e := range g.Edges {
		if e.Owner >= 0 {
			return nil, errors.New("捕鱼地图不能覆盖已有路线")
		}
	}
	board := *g
	board.Tiles = slices.Clone(g.Tiles)
	board.Tiles[r.lake].Resource, board.Tiles[r.lake].Number = catanLake, 0
	f, err := makeFishingCoastalMap(board.fishingCoasts(board.findIslands()), placements)
	if err != nil {
		return nil, err
	}
	f.Lakes = []catanFishingLake{{Tile: r.lake, Numbers: []int{2, 3, 11, 12}}}
	f.ExtraNumbers = []catanFishingExtraNumber{{Tile: r.recipient, Number: r.moved}}
	if err = f.validateDesert(&board); err != nil {
		return nil, err
	}
	// Lake remains land and does not join regions across the desert belt.
	g.Tiles = board.Tiles
	return f, nil
}

func (f catanFishingMap) validateDesert(g *Catan) error {
	r, err := g.fishingDesertRecipe()
	if err != nil {
		return err
	}
	wantTiles := 35
	if len(g.Players) == 4 {
		wantTiles = 42
	}
	if len(g.Tiles) != wantTiles || len(f.Lakes) != 1 || len(f.Grounds) != 6 || len(f.ExtraNumbers) != 1 {
		return errors.New("穿越沙漠捕鱼组件数量不符")
	}
	lake, extra := f.Lakes[0], f.ExtraNumbers[0]
	if lake.Tile != r.lake || !slices.Equal(lake.Numbers, []int{2, 3, 11, 12}) ||
		g.Tiles[r.lake].Resource != catanLake || g.Tiles[r.lake].Number != 0 ||
		extra.Tile != r.recipient || extra.Number != r.moved ||
		g.Tiles[r.recipient].Resource != r.recipientResource || g.Tiles[r.recipient].Number != r.original {
		return errors.New("穿越沙漠湖泊或双生产点数不符")
	}
	for _, tile := range g.Tiles {
		if tile.Resource == catanLake && tile.ID != r.lake {
			return errors.New("多余湖泊")
		}
	}
	if !slices.Equal(g.Seafarers.Islands, g.findLandRegions(true)) {
		return errors.New("穿越沙漠探索区域不符")
	}
	if g.Robber < -1 || g.Robber >= len(g.Tiles) || g.Robber >= 0 && (g.Tiles[g.Robber].Resource == CatanSea || g.Tiles[g.Robber].Resource == CatanFog) {
		return errors.New("非法强盗位置")
	}
	return f.validateCoastalGrounds(g.fishingCoasts(g.findIslands()))
}

// Production and AI use all publicly printed number discs. A matched hex
// produces once, regardless of which of its two numbers was rolled.
func (g *Catan) tileProduces(t CatanTile, number int) bool {
	if t.Number == number {
		return true
	}
	if g.Fishing != nil {
		for _, extra := range g.Fishing.Map.ExtraNumbers {
			if extra.Tile == t.ID && extra.Number == number {
				return true
			}
		}
	}
	return false
}

func (g *Catan) tileNumberWeight(t CatanTile) int {
	weight := 0
	if t.Number >= 2 && t.Number <= 12 && t.Number != 7 {
		weight = 6 - absCatan(7-t.Number)
	}
	if g.Fishing != nil {
		for _, extra := range g.Fishing.Map.ExtraNumbers {
			if extra.Tile == t.ID && extra.Number != t.Number {
				weight += 6 - absCatan(7-extra.Number)
			}
		}
	}
	return weight
}
