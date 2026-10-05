package game

import (
	"errors"
	"slices"
)

// July 2025 Fishing + Seafarers p.2: no lake, freely chosen grounds on
// the large and small islands. The scenario retains its absent pirate.
func (g *Catan) fishingWondersCoasts() ([]CatanFishingCoast, error) {
	if g.Seafarers == nil || g.Seafarers.Scenario != "wonders" || g.wonders() == nil ||
		len(g.Players) < 3 || len(g.Players) > 4 || len(g.Tiles) != 49 || len(g.Seafarers.StartIslands) != 1 {
		return nil, errors.New("仅已核对的三/四人奇迹捕鱼布局")
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
	if !slices.Equal(sizes, []int{2, 3, 25}) || counts[g.Seafarers.StartIslands[0]] != 25 {
		return nil, errors.New("奇迹捕鱼必须保留主岛和两座小岛")
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
	if len(f.Lakes) != 0 || len(f.Grounds) != 6 || len(f.ExtraNumbers) != 0 {
		return errors.New("奇迹捕鱼不使用湖泊，必须有六个渔场")
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
