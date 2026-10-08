package game

import (
	"errors"
	"slices"
)

// Fishing + Seafarers 2025 p.2: the shared 3/4-player printed map replaces
// field-12 with the lake, with six freely placed grounds on the main island.
// Variable and 5/6 layouts need their own verified replacement recipe.
func (g *Catan) fishingTribeCoasts() ([]CatanFishingCoast, error) {
	if g.Seafarers == nil || g.Seafarers.Scenario != "tribe" || g.tribe() == nil || g.Seafarers.Variable ||
		len(g.Players) < 3 || len(g.Players) > 4 || len(g.Tiles) != 49 || len(g.Seafarers.StartIslands) != 1 {
		return nil, errors.New("仅已核对的三/四人固定遗忘部落捕鱼布局")
	}
	islands := g.findIslands()
	if !slices.Equal(islands, g.Seafarers.Islands) {
		return nil, errors.New("遗忘部落捕鱼岛屿不符")
	}
	mainland := g.Seafarers.StartIslands[0]
	mainSize := 0
	for _, island := range islands {
		if island == mainland {
			mainSize++
		}
	}
	if islands[24] != mainland || mainSize != 18 {
		return nil, errors.New("遗忘部落捕鱼主岛不符")
	}
	coasts := g.fishingCoasts(islands)
	return slices.DeleteFunc(coasts, func(c CatanFishingCoast) bool { return c.Island != mainland }), nil
}

func (g *Catan) makeFishingTribe(placements []CatanFishingGroundPlacement) (*catanFishingMap, error) {
	coasts, err := g.fishingTribeCoasts()
	if err != nil {
		return nil, err
	}
	if g.SetupStep != 0 || g.Tiles[24].Resource != 3 || g.Tiles[24].Number != 12 {
		return nil, errors.New("遗忘部落捕鱼替换地块与固定地图不符")
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
	f, err := makeFishingCoastalMap(coasts, placements, len(g.Players))
	if err != nil {
		return nil, err
	}
	board := *g
	board.Tiles = slices.Clone(g.Tiles)
	board.Tiles[24].Resource, board.Tiles[24].Number = catanLake, 0
	f.Lakes = []catanFishingLake{{Tile: 24, Numbers: []int{2, 3, 11, 12}}}
	if err = f.validateTribe(&board); err != nil {
		return nil, err
	}
	g.Tiles = board.Tiles
	return f, nil
}

func (f catanFishingMap) validateTribe(g *Catan) error {
	coasts, err := g.fishingTribeCoasts()
	if err != nil {
		return err
	}
	if len(f.Lakes) != 1 || len(f.Grounds) != 6 || len(f.ExtraNumbers) != 0 ||
		f.Lakes[0].Tile != 24 || !slices.Equal(f.Lakes[0].Numbers, []int{2, 3, 11, 12}) ||
		g.Tiles[24].Resource != catanLake || g.Tiles[24].Number != 0 {
		return errors.New("遗忘部落捕鱼湖泊或组件不符")
	}
	for _, tile := range g.Tiles {
		if tile.Resource == catanLake && tile.ID != 24 {
			return errors.New("多余湖泊")
		}
	}
	// The printed robber starts on the outer desert (47); later ordinary
	// robber moves are confined to the mainland, including the new lake.
	// Robber Flees explicitly returns to a desert; its event-specific exception
	// must survive later event draws. Ordinary seven/knight destinations still
	// use tribeLand and cannot choose these unnumbered outer islands.
	eventDesert := g.EventDeck != nil && g.Robber >= 0 && g.Robber < len(g.Tiles) && g.Tiles[g.Robber].Resource == CatanDesert
	if g.Robber < -1 || g.Robber >= len(g.Tiles) || g.Robber >= 0 && g.Robber != 47 &&
		g.Seafarers.Islands[g.Robber] != g.Seafarers.StartIslands[0] && !eventDesert {
		return errors.New("遗忘部落捕鱼强盗位置不符")
	}
	return f.validateCoastalGrounds(coasts, len(g.Players))
}

func (g *Catan) fishingGroundEdge(edge int) bool {
	if g.Fishing == nil {
		return false
	}
	for _, ground := range g.Fishing.Map.Grounds {
		if ground.Edges[0] == edge || ground.Edges[1] == edge {
			return true
		}
	}
	return false
}
