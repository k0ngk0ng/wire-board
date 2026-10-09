package game

import (
	"errors"
	"slices"
)

// Only printed terrain/discs and river orientation are supplied by clients.
// Coordinates, bridges, island identities, pieces and port order are derived
// or initialized by the server, never accepted from an approved map.
type CatanRiversWorldMap struct {
	Hexes    []CatanNewWorldHex        `json:"hexes"`
	Channels []CatanRiversWorldChannel `json:"channels"`
}
type CatanRiversWorldChannel struct {
	Tiles  []int `json:"tiles"`
	Outlet int   `json:"outlet"`
}

func (g *Catan) RiversWorldMap() *CatanRiversWorldMap {
	if !g.riversSea() || g.newWorld() == nil {
		return nil
	}
	m := &CatanRiversWorldMap{Hexes: g.NewWorldMap().Hexes}
	for _, c := range g.Rivers.Map.Channels {
		m.Channels = append(m.Channels, CatanRiversWorldChannel{Tiles: slices.Clone(c.Tiles), Outlet: c.Outlet})
	}
	return m
}

func GenerateCatanRiversWorldMap(n int) (*CatanRiversWorldMap, error) {
	s, err := newCatanRiversWorld(n)
	if err != nil {
		return nil, err
	}
	return s.Catan.RiversWorldMap(), nil
}

func ValidateCatanRiversWorldMap(n int, layout *CatanRiversWorldMap) error {
	_, err := newCatanRiversWorldWithMap(n, layout)
	return err
}

// Internal until public room setup and frontend approval have been accepted.
func newCatanRiversWorldWithMap(n int, layout *CatanRiversWorldMap) (*State, error) {
	if n != 3 && n != 4 {
		return nil, errors.New("河流新世界预备地图暂支持三或四人")
	}
	specs := newWorldFrame(n)
	if layout == nil || len(layout.Hexes) != len(specs) || len(layout.Channels) != 2 {
		return nil, errors.New("河流新世界预备地图尺寸或河流数量无效")
	}
	for i, h := range layout.Hexes {
		if h.Resource < 0 || h.Resource > CatanGold && h.Resource != catanSwamp {
			return nil, errors.New("河流新世界地形无效")
		}
		specs[i].Resource, specs[i].Number = h.Resource, h.Number
		if h.Resource == catanSwamp {
			specs[i].Resource = CatanDesert
		}
	}
	s, err := NewCatanNewWorld(n, CatanOptions{})
	if err != nil {
		return nil, err
	}
	g := s.Catan
	if err = g.makeScenarioMap(specs); err != nil {
		return nil, err
	}
	for i, h := range layout.Hexes {
		g.Tiles[i].Resource = h.Resource
	}
	candidates := make([]riverSeaCandidate, 2)
	for i, c := range layout.Channels {
		if len(c.Tiles) != 4-i {
			return nil, errors.New("河流长度无效")
		}
		found := false
		for _, candidate := range g.riverWorldCandidates(4 - i) {
			if c.Outlet == candidate.outlet && slices.Equal(c.Tiles, candidate.tiles) {
				candidates[i] = candidate
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("河流必须连贯且河口朝海")
		}
	}
	g.Rivers = &CatanRivers{Rules: CatanRiversRules, Sea: CatanRiversSeafarersRules, SeaLayout: "prepared", Map: g.riverTribeMapFor(candidates[0], candidates[1]), Gold: make([]int, n), Bank: 100}
	g.Seafarers.Rules, g.Seafarers.Layout = CatanSeafarersRules, "prepared"
	g.Seafarers.Islands = g.findIslands()
	g.Robber = -1 // New World starts on the frame even if the approved map has deserts.
	if err = g.validateRivers(); err != nil {
		return nil, err
	}
	s.Log = []string{"河流＋新世界：使用开局前确认的地形、数字和河道；12分获胜", "先轮流放置10个港口，再开始建设；强盗和海盗从海框外出发", "本站移船补充：移入河岸领1金币，移出河岸须先退1金币；桥位不能造船"}
	s.catanScores()
	return s, nil
}

func (g *Catan) riverWorldCandidates(length int) []riverSeaCandidate {
	result := g.riverSeaCandidates(length, func(int) bool { return true }, true)
	return append(result, g.riverSeaCandidates(length, func(int) bool { return true }, false)...)
}

func validateRiverWorldInventory(tiles []CatanTile, prepared bool) error {
	terrain, numbers := make([]int, 11), make([]int, 13)
	for _, t := range tiles {
		if t.Resource < 0 || t.Resource >= len(terrain) || t.Number < 0 || t.Number >= len(numbers) || t.Number == 1 || t.Number == 7 {
			return errors.New("新世界地块无效")
		}
		terrain[t.Resource]++
		numbers[t.Number]++
		if (t.Resource == CatanSea || t.Resource == catanSwamp || t.Resource == CatanDesert) != (t.Number == 0) {
			return errors.New("新世界数字位置无效")
		}
		if t.Resource == CatanGold && (t.Number == 6 || t.Number == 8) {
			return errors.New("金矿不能放红色数字")
		}
	}
	if !prepared {
		if !slices.Equal(terrain, []int{5, 4, 5, 5, 4, 0, 17, 0, 0, 0, 2}) || !slices.Equal(numbers, []int{19, 0, 1, 3, 3, 3, 2, 0, 2, 3, 3, 2, 1}) {
			return errors.New("河流新世界组件不守恒")
		}
		return nil
	}
	// Base+Seafarers inventory after replacing hills2/pasture1/mountains2/sea2
	// with the river tiles. The productive river hexes replace like resources.
	for resource, count := range []int{5, 5, 5, 5, 5, 3, 17, 2, 0, 0, 2} {
		if terrain[resource] > count {
			return errors.New("河流新世界地形超出组件库存")
		}
	}
	if terrain[catanSwamp] != 2 {
		return errors.New("河流新世界必须有两个沼泽")
	}
	for number, count := range []int{0, 0, 2, 3, 3, 3, 3, 0, 3, 3, 3, 3, 2} {
		if number > 0 && numbers[number] > count {
			return errors.New("河流新世界数字超出组件库存")
		}
	}
	return nil
}
