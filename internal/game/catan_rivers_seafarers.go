package game

import (
	"errors"
	"reflect"
	"slices"
)

const CatanRiversSeafarersRules = "catan-rivers-seafarers-2025"

func (g *Catan) riversSea() bool {
	return g != nil && g.Rivers != nil && g.Rivers.Sea == CatanRiversSeafarersRules && g.Seafarers != nil
}

// Internal admission uses printed three/four-player diagrams and a labelled
// five/six-player recipe. Public admission remains a separate acceptance gate.
func newCatanRiversShores(n int) (*State, error) {
	if n > 4 {
		return newCatanRiversShoresExtended(n)
	}
	return newCatanRiversPrintedSea(n, "shores")
}

func newCatanRiversPrintedSea(n int, scenario string) (*State, error) {
	return newCatanRiversPrintedSeaLayout(n, scenario, "")
}

func newCatanRiversPrintedSeaLayout(n int, scenario, layout string) (*State, error) {
	if (n != 3 && n != 4) || (scenario != "shores" && scenario != "fog" && scenario != "desert") {
		return nil, errors.New("河流固定组合图暂支持三或四人新海岸、迷雾及沙漠")
	}
	if scenario == "desert" && layout == "" {
		layout = "rivers-across"
	}
	if scenario == "desert" && layout != "rivers-across" && layout != "desert-belt" || scenario != "desert" && layout != "" {
		return nil, errors.New("河流海图布局无效")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: scenario, Layout: "fixed"}, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, err := g.makeRiversPrintedSeaMap(layout)
	if err != nil {
		return nil, err
	}
	g.Rivers = &CatanRivers{Rules: CatanRiversRules, Sea: CatanRiversSeafarersRules, SeaLayout: layout, Map: m, Gold: make([]int, n), Bank: 100}
	g.Robber = -1
	s.Phase = "catan_rivers_start"
	s.Log = []string{"河流＋新海岸：使用官方固定组合图，14分获胜；河岸建船领1金币，移走河岸船先退1金币，桥位禁止造船", "本站移船补充：移入河岸按新位置领1金币；从河岸移出须先有金币可退，即使目标也在河岸；金矿仍领取任选资源而非金币"}
	if scenario == "fog" {
		s.Log[0] = "河流＋迷雾群岛：使用官方固定组合图，12分获胜；河岸建船领1金币，移走河岸船先退1金币，桥位禁止造船"
	}
	if scenario == "desert" {
		g.Robber = 5
		if n == 4 {
			g.Robber = 6
		}
		s.Phase = "catan_setup_settlement"
		s.Log[0] = "河流＋穿越沙漠：使用官方固定组合图，14分获胜；强盗从沙漠出发，金矿仍产出任选资源"
		if layout == "rivers-across" {
			s.Log = append(s.Log, "河流穿过原沙漠带：主岛已连通，首次在外岛建村仍加2分")
		} else {
			s.Log = append(s.Log, "保留沙漠带：首次在沙漠另一侧区域或外岛建村加2分")
		}
	}
	s.catanScores()
	return s, g.validateRivers()
}

func riverShoresRecipe(n int) (map[int]seaTerrain, [][]int, []int, int, []seaPort) {
	if n == 3 {
		return map[int]seaTerrain{10: {3, 6}, 11: {4, 12}, 15: {4, 5}, 16: {2, 4}, 17: {1, 6}, 20: {0, 8}, 21: {1, 3}, 22: {2, 9}, 23: {catanSwamp, 0}, 26: {3, 11}, 27: {2, 8}, 28: {1, 10}, 31: {3, 10}, 32: {catanSwamp, 0}},
			[][]int{{15, 21, 27, 32}, {11, 17, 23}}, []int{0, 0}, 11,
			[]seaPort{{2, 1, 2, 3}, {2, 2, 5, -1}, {3, 0, 2, 4}, {4, 0, 1, -1}, {4, 3, 4, 2}, {5, 2, 5, 0}, {6, 0, 2, 1}, {6, 1, 5, -1}}
	}
	return map[int]seaTerrain{12: {2, 5}, 13: {0, 8}, 14: {4, 4}, 18: {catanSwamp, 0}, 19: {2, 10}, 20: {1, 3}, 21: {4, 11}, 24: {1, 6}, 25: {0, 9}, 26: {3, 11}, 27: {3, 6}, 28: {0, 12}, 31: {catanSwamp, 0}, 32: {1, 4}, 33: {4, 5}, 34: {2, 9}, 37: {3, 3}, 38: {0, 8}, 39: {3, 10}},
		[][]int{{21, 20, 19, 18}, {33, 32, 31}}, []int{2, 2}, 28,
		[]seaPort{{2, 1, 2, -1}, {2, 3, 3, 4}, {3, 3, 4, -1}, {3, 0, 2, 2}, {4, 0, 1, 1}, {4, 4, 0, 3}, {6, 0, 2, -1}, {6, 1, 1, 0}, {6, 2, 5, -1}}
}
func (g *Catan) makeRiversPrintedSeaMap(layout string) (*catanRiversMap, error) {
	recipe, paths, outlets, double, ports := riverShoresRecipe(len(g.Players))
	var extra []catanFishingExtraNumber
	if g.Seafarers.Scenario == "fog" {
		recipe, paths, outlets, double, extra = riverFogRecipe(len(g.Players))
		ports = nil // Printed combination keeps the ordinary fog ports.
	}
	if g.Seafarers.Scenario == "desert" {
		recipe, paths, outlets, double = riverDesertRecipe(len(g.Players), layout)
		ports = nil
	}
	for id, t := range recipe {
		g.Tiles[id].Resource = t.resource
		g.Tiles[id].Number = t.number
	}
	m := &catanRiversMap{DoubleNumberTile: double, ExtraNumbers: extra}
	for i, path := range paths {
		outlet := catanFishingSide(g, path[len(path)-1], outlets[i])
		m.Channels = append(m.Channels, catanRiverChannel{Tiles: slices.Clone(path), Outlet: outlet})
		for j, id := range path {
			if g.Tiles[id].Resource == catanSwamp {
				m.Swamps = append(m.Swamps, id)
			}
			if j > 0 {
				edge := -1
				for _, e := range g.Edges {
					if slices.Contains(e.Tiles, id) && slices.Contains(e.Tiles, path[j-1]) {
						edge = e.ID
						break
					}
				}
				if edge < 0 {
					return nil, errors.New("海图河流路径不连续")
				}
				m.Bridges = append(m.Bridges, edge)
			}
		}
		m.Bridges = append(m.Bridges, outlet)
	}
	sizes := []int{5, 6, 7, 6, 7, 6, 5}
	if len(g.Players) == 3 && g.Seafarers.Scenario == "shores" {
		sizes = []int{4, 5, 6, 5, 6, 5, 4}
	}
	for i, p := range ports {
		id := p.col
		for row := 0; row < p.row; row++ {
			id += sizes[row]
		}
		g.Ports[i] = CatanPort{Edge: catanFishingSide(g, id, p.side), Resource: p.resource}
	}
	g.Seafarers.Islands = g.findLandRegions(g.Seafarers.Scenario == "desert")
	if g.Seafarers.Scenario == "desert" {
		home := 22
		if len(g.Players) == 4 {
			home = 26
		}
		g.Seafarers.StartIslands = []int{g.Seafarers.Islands[home]}
	}
	return m, nil
}
func (g *Catan) validateRiversSeaMap() error {
	if g.riversSea() && g.Seafarers.Scenario == "shores" && len(g.Players) > 4 {
		return g.validateRiversShoresExtended()
	}
	if g.riversSea() && g.Seafarers.Scenario == "new_world" {
		return g.validateRiversWorldMap()
	}
	if g.riversSea() && g.Seafarers.Scenario == "tribe" {
		return g.validateRiversTribeMap()
	}
	if !g.riversSea() || g.Rivers.Map == nil || (g.Seafarers.Scenario != "shores" && g.Seafarers.Scenario != "fog" && g.Seafarers.Scenario != "desert") || g.Seafarers.Layout != "fixed" || len(g.Players) < 2 || len(g.Players) == 2 && !g.twoRiversSea() || len(g.Players) > 4 {
		return errors.New("河流海图配方尚未接入或标记无效")
	}
	sea := g.Seafarers
	if sea.Scenario == "desert" && g.Rivers.SeaLayout != "rivers-across" && g.Rivers.SeaLayout != "desert-belt" || sea.Scenario != "desert" && g.Rivers.SeaLayout != "" {
		return errors.New("河流海图布局记录无效")
	}
	if sea.Rules != CatanSeafarersRules || sea.Variable || sea.NumberRecipe != "" || sea.NewWorld != nil || sea.Wonders != nil || sea.PirateIslands != nil || sea.Cloth != nil || sea.Tribe != nil || (sea.Fog != nil) != (sea.Scenario == "fog") || len(sea.Seats) != len(g.Players) {
		return errors.New("河流固定组合图包含不适用的海图状态")
	}
	if sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("河流海图海盗位置无效")
	}
	recipeSeats := len(g.Players)
	if g.twoRiversSea() {
		recipeSeats = 4
	}
	expected, err := NewCatanSeafarers(recipeSeats, CatanOptions{}, CatanSeafarersSetup{Scenario: sea.Scenario, Layout: "fixed"}, nil)
	if err != nil {
		return err
	}
	board := expected.Catan
	m, err := board.makeRiversPrintedSeaMap(g.Rivers.SeaLayout)
	if err != nil {
		return err
	}
	if sea.Scenario == "fog" {
		if err := g.validateRiverFog(board); err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(m, g.Rivers.Map) || len(g.Tiles) != len(board.Tiles) || !reflect.DeepEqual(g.Ports, board.Ports) || !reflect.DeepEqual(g.Seafarers.Islands, g.findLandRegions(sea.Scenario == "desert")) || !slices.Equal(g.Seafarers.StartIslands, board.Seafarers.StartIslands) || g.Seafarers.VictoryPoints != board.Seafarers.VictoryPoints || g.Seafarers.IslandBonus != board.Seafarers.IslandBonus {
		return errors.New("河流海图河道、港口或岛屿配置不符")
	}
	if !reflect.DeepEqual(g.Tiles, board.Tiles) || len(g.Vertices) != len(board.Vertices) || len(g.Edges) != len(board.Edges) {
		return errors.New("河流海图地形数字或尺寸不符")
	}
	for i, v := range g.Vertices {
		w := board.Vertices[i]
		v.Owner = w.Owner
		v.Level = w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("河流海图交点不符")
		}
	}
	for i, e := range g.Edges {
		w := board.Edges[i]
		e.Owner = w.Owner
		e.Ship = w.Ship
		e.Bridge = w.Bridge
		e.Damaged = w.Damaged
		e.Warship = w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("河流海图路线不符")
		}
	}
	return nil
}
