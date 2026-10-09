package game

import (
	"errors"
	"fmt"
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
	if n < 3 || n > 6 {
		return nil, errors.New("河流新世界预备地图暂支持三至六人")
	}
	specs := newWorldFrame(n)
	if layout == nil || len(layout.Hexes) != len(specs) || len(layout.Channels) != len(riverWorldChannels(n)) {
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
	s, err := NewCatanNewWorld(n, CatanOptions{FiveSix: n > 4})
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
	candidates := make([]riverSeaCandidate, len(layout.Channels))
	for i, c := range layout.Channels {
		if len(c.Tiles) != len(riverWorldChannels(n)[i]) {
			return nil, errors.New("河流长度无效")
		}
		found := false
		for _, candidate := range g.riverWorldCandidates(len(c.Tiles)) {
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
	g.Rivers = &CatanRivers{Rules: CatanRiversRules, Sea: CatanRiversSeafarersRules, SeaLayout: riverWorldLayout(n, true), Map: g.riverWorldMapFor(candidates), Gold: make([]int, n), Bank: riverWorldBank(n)}
	g.Seafarers.Rules, g.Seafarers.Layout = CatanSeafarersRules, "prepared"
	g.Seafarers.Islands = g.findIslands()
	g.Robber = -1 // New World starts on the frame even if the approved map has deserts.
	if err = g.validateRivers(); err != nil {
		return nil, err
	}
	s.Log = []string{"河流＋新世界：使用开局前确认的地形、数字和河道；12分获胜", fmt.Sprintf("先轮流放置%d个港口，再开始建设；强盗和海盗从海框外出发", riverWorldPorts(n)), "本站移船补充：移入河岸领1金币，移出河岸须先退1金币；桥位不能造船"}
	if n > 4 {
		s.Log = append(s.Log, catanRiversWorldExtendedNotice)
	}
	s.catanScores()
	return s, nil
}

func (g *Catan) riverWorldCandidates(length int) []riverSeaCandidate {
	result := g.riverSeaCandidates(length, func(int) bool { return true }, true)
	return append(result, g.riverSeaCandidates(length, func(int) bool { return true }, false)...)
}

func validateRiverWorldInventory(tiles []CatanTile, n int, prepared bool) error {
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
		if !slices.Equal(terrain, riverWorldTerrain(n)) || !slices.Equal(numbers, riverWorldNumbers(n)) {
			return errors.New("河流新世界组件不守恒")
		}
		return nil
	}
	// Base+Seafarers inventory after replacing hills2/pasture1/mountains2/sea2
	// with the river tiles. The productive river hexes replace like resources.
	caps := []int{5, 5, 5, 5, 5, 3, 17, 2, 0, 0, 2}
	if n > 4 {
		caps = []int{7, 7, 7, 7, 7, 5, 24, 4, 0, 0, 2}
	}
	for resource, count := range caps {
		if terrain[resource] > count {
			return errors.New("河流新世界地形超出组件库存")
		}
	}
	if terrain[catanSwamp] != 2 {
		return errors.New("河流新世界必须有两个沼泽")
	}
	numberCaps := []int{0, 0, 2, 3, 3, 3, 3, 0, 3, 3, 3, 3, 2}
	if n > 4 {
		numberCaps = []int{0, 0, 3, 5, 5, 5, 5, 0, 5, 5, 5, 5, 3}
	}
	for number, count := range numberCaps {
		if number > 0 && numbers[number] > count {
			return errors.New("河流新世界数字超出组件库存")
		}
	}
	return nil
}
