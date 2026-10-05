package game

import (
	"errors"
	"slices"
)

// July 2025 Fishing + Seafarers p.2: no lake, freely chosen grounds on
// the large and small islands. The scenario retains its absent pirate.
func (g *Catan) fishingWondersCoasts() ([]CatanFishingCoast, error) {
	if g.Seafarers == nil || g.Seafarers.Scenario != "wonders" || g.wonders() == nil ||
		len(g.Players) < 3 || len(g.Players) > 6 || len(g.Seafarers.StartIslands) != 1 {
		return nil, errors.New("仅已核对的三至六人奇迹捕鱼布局")
	}
	wantTiles, mainland := 49, 25
	wantSizes := []int{2, 3, 25}
	if len(g.Players) > 4 {
		wantTiles, mainland, wantSizes = 63, 35, []int{1, 1, 2, 35}
		if !g.Options.FiveSix || g.Paired == nil || g.Seafarers.Variable {
			return nil, errors.New("五至六人奇迹捕鱼须保留扩充地图和配对回合")
		}
	}
	if len(g.Tiles) != wantTiles {
		return nil, errors.New("奇迹捕鱼地图大小不符")
	}
	islands := g.findIslands()
	if !slices.Equal(islands, g.Seafarers.Islands) {
		return nil, errors.New("奇迹捕鱼岛屿不符")
	}
	counts := map[int]int{}
	for _, island := range islands {
		if island >= 0 {
			counts[island]++
		}
	}
	sizes := []int{}
	for _, size := range counts {
		sizes = append(sizes, size)
	}
	slices.Sort(sizes)
	if !slices.Equal(sizes, wantSizes) || counts[g.Seafarers.StartIslands[0]] != mainland {
		return nil, errors.New("奇迹捕鱼必须保留对应人数的主岛和小岛")
	}
	return g.fishingCoasts(islands), nil
}

func (g *Catan) makeFishingWonders(placements []CatanFishingGroundPlacement) (*catanFishingMap, error) {
	coasts, err := g.fishingWondersCoasts()
	if err != nil {
		return nil, err
	}
	f, err := makeFishingCoastalMap(coasts, placements, len(g.Players))
	if err != nil {
		return nil, err
	}
	if err = f.validateWonders(g); err != nil {
		return nil, err
	}
	return f, nil
}

func (f catanFishingMap) validateWonders(g *Catan) error {
	coasts, err := g.fishingWondersCoasts()
	if err != nil {
		return err
	}
	if len(f.Lakes) != 0 || len(f.Grounds) != len(catanFishingGroundNumbers(len(g.Players))) || len(f.ExtraNumbers) != 0 {
		return errors.New("奇迹捕鱼不使用湖泊，渔场数量须符合人数")
	}
	if err = f.validateCoastalGrounds(coasts, len(g.Players)); err != nil {
		return err
	}
	for _, tile := range g.Tiles {
		if tile.Resource == catanLake {
			return errors.New("奇迹捕鱼不能放置湖泊")
		}
	}
	if g.Seafarers.Pirate != -1 {
		return errors.New("奇迹捕鱼不使用海盗")
	}
	if g.Robber < -1 || g.Robber >= len(g.Tiles) || g.Robber >= 0 && g.Tiles[g.Robber].Resource == CatanSea {
		return errors.New("奇迹捕鱼强盗必须位于陆地或场外")
	}
	return nil
}
