package game

import (
	"errors"
	"reflect"
	"slices"
)

const catanRiversShoresExtendedLayout = "extended-mainland"
const catanRiversShoresExtendedNotice = "本站五六人河流新海岸：以五六人河流岛替换30格主岛，外岛与探索分区不变；三条河、两处沼泽、10个桥位、11个港口、152金币，14分获胜，沿用配对回合"

// Row-major embedding of the 30-hex mainland in the printed 56-hex sea frame.
func riverShoresMainlandIDs() []int {
	ids := []int{}
	offset := 0
	widths := []int{7, 8, 9, 8, 9, 8, 7}
	for row, count := range []int{3, 4, 5, 6, 5, 4, 3} {
		start := 2
		if row == 3 {
			start = 1
		}
		for col := start; col < start+count; col++ {
			ids = append(ids, offset+col)
		}
		offset += widths[row]
	}
	return ids
}

func (g *Catan) riverShoresExtendedMap() *catanRiversMap {
	ids := riverShoresMainlandIDs()
	paths, _, outlets := catanRiverRecipe(true)
	candidates := make([]riverSeaCandidate, len(paths))
	for i, path := range paths {
		for _, id := range path {
			candidates[i].tiles = append(candidates[i].tiles, ids[id])
		}
		candidates[i].outlet = catanFishingSide(g, ids[path[len(path)-1]], outlets[i])
	}
	m := g.riverWorldMapFor(candidates)
	m.NumberRecipe = CatanExtendedNumberRecipe
	return m
}

func newCatanRiversShoresExtended(n int) (*State, error) {
	if n < 5 || n > 6 {
		return nil, errors.New("扩大河流新海岸需要五或六人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "shores", Layout: "variable"}, nil)
	if err != nil {
		return nil, err
	}
	main, err := NewCatanRivers(n, CatanOptions{FiveSix: true})
	if err != nil {
		return nil, err
	}
	g := s.Catan
	for i, id := range riverShoresMainlandIDs() {
		g.Tiles[id].Resource, g.Tiles[id].Number = main.Catan.Tiles[i].Resource, main.Catan.Tiles[i].Number
	}
	g.Rivers = &CatanRivers{Rules: CatanRiversRules, Sea: CatanRiversSeafarersRules, SeaLayout: catanRiversShoresExtendedLayout, Map: g.riverShoresExtendedMap(), Gold: make([]int, n), Bank: 152}
	g.Robber = -1
	s.Phase = "catan_rivers_start"
	s.Log = append(s.Log, catanRiversShoresExtendedNotice, catanExtendedNumberNotice, "先选择一处沼泽作为强盗起点；河岸造船领1金币，移出先退1金币，桥位禁止普通道路与船", "本站移船补充：移入河岸按新位置领1金币；从河岸移出须先有金币可退，即使目标也在河岸；金矿仍领取任选资源而非金币")
	s.catanScores()
	return s, g.validateRivers()
}

func (g *Catan) validateRiversShoresExtended() error {
	n := len(g.Players)
	sea, r := g.Seafarers, g.Rivers
	if n < 5 || n > 6 || sea == nil || r == nil || r.Map == nil || sea.Scenario != "shores" || sea.Rules != CatanSeafarersRules || sea.Layout != "variable" || !sea.Variable || sea.NumberRecipe != CatanExtendedNumberRecipe || r.SeaLayout != catanRiversShoresExtendedLayout || sea.VictoryPoints != g.riversSeaVictoryPoints(14) || sea.IslandBonus != 2 || len(sea.Seats) != n || sea.Fog != nil || sea.Tribe != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil {
		return errors.New("五六人河流新海岸配置无效")
	}
	expected, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "shores", Layout: "variable"}, nil)
	if err != nil {
		return err
	}
	b := expected.Catan
	if len(g.Tiles) != len(b.Tiles) || len(g.Vertices) != len(b.Vertices) || len(g.Edges) != len(b.Edges) || !slices.Equal(sea.Islands, b.Seafarers.Islands) || !slices.Equal(sea.StartIslands, b.Seafarers.StartIslands) || !reflect.DeepEqual(r.Map, b.riverShoresExtendedMap()) {
		return errors.New("五六人河流新海岸尺寸、河道或探索分区无效")
	}
	main, err := NewCatanRivers(n, CatanOptions{FiveSix: true})
	if err != nil {
		return err
	}
	for i, id := range riverShoresMainlandIDs() {
		main.Catan.Tiles[i].Resource, main.Catan.Tiles[i].Number = g.Tiles[id].Resource, g.Tiles[id].Number
		b.Tiles[id].Resource, b.Tiles[id].Number = g.Tiles[id].Resource, g.Tiles[id].Number
	}
	// Validate inventory, printed river terrain and the spiral using a geometry
	// projection only. The projection never becomes a live game or owns actions.
	if err = main.Catan.validateRivers(); err != nil {
		return err
	}
	if !reflect.DeepEqual(g.Tiles, b.Tiles) {
		return errors.New("河流新海岸外岛或地块几何无效")
	}
	for i, v := range g.Vertices {
		w := b.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("河流新海岸交点无效")
		}
	}
	for i, e := range g.Edges {
		w := b.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Warship, e.Damaged = w.Owner, w.Ship, w.Bridge, w.Warship, w.Damaged
		if !reflect.DeepEqual(e, w) {
			return errors.New("河流新海岸边无效")
		}
	}
	if sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("河流新海岸海盗位置无效")
	}
	if len(g.Ports) != len(b.Ports) {
		return errors.New("河流新海岸港口数量无效")
	}
	actual, want := []int{}, []int{}
	for i, p := range g.Ports {
		if p.Edge != b.Ports[i].Edge {
			return errors.New("河流新海岸港口位置无效")
		}
		actual = append(actual, p.Resource)
		want = append(want, b.Ports[i].Resource)
	}
	slices.Sort(actual)
	slices.Sort(want)
	if !slices.Equal(actual, want) {
		return errors.New("河流新海岸港口库存无效")
	}
	return nil
}
