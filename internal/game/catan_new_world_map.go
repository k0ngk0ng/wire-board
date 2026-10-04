package game

import "fmt"

// An approved board contains terrain and number discs only. Clients cannot
// submit coordinates, piece owners, island identities, or the hidden port deck.
// Hexes use the printed frame's row-major order.
type CatanNewWorldHex struct {
	Resource int `json:"resource"`
	Number   int `json:"number"`
}
type CatanNewWorldMap struct {
	Hexes []CatanNewWorldHex `json:"hexes"`
}

func newWorldFrame(n int) []CatanHexSpec {
	rows := []int{5, 6, 7, 6, 7, 6, 5}
	if n > 4 {
		rows = []int{8, 9, 10, 9, 10, 9, 8}
	}
	starts := []int{0, -1, -2, -2, -3, -3, -3}
	specs := []CatanHexSpec{}
	for row, count := range rows {
		for col := 0; col < count; col++ {
			specs = append(specs, CatanHexSpec{Q: starts[row] + col, R: row})
		}
	}
	return specs
}

func (g *Catan) NewWorldMap() *CatanNewWorldMap {
	if g.newWorld() == nil {
		return nil
	}
	result := &CatanNewWorldMap{Hexes: make([]CatanNewWorldHex, len(g.Tiles))}
	for i, t := range g.Tiles {
		result.Hexes[i] = CatanNewWorldHex{Resource: t.Resource, Number: t.Number}
	}
	return result
}

// Prepare a default random board BEFORE players ready up. Starting later with
// NewCatanNewWorldWithMap preserves exactly this terrain and these discs.
func GenerateCatanNewWorldMap(n int) (*CatanNewWorldMap, error) {
	s, err := NewCatanNewWorld(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		return nil, err
	}
	return s.Catan.NewWorldMap(), nil
}

func ValidateCatanNewWorldMap(n int, layout *CatanNewWorldMap) error {
	_, err := catanNewWorldMapGeometry(n, layout)
	return err
}

func catanNewWorldMapGeometry(n int, layout *CatanNewWorldMap) (*Catan, error) {
	if n < 3 || n > 6 {
		return nil, fmt.Errorf("新世界需要3至6位玩家")
	}
	specs := newWorldFrame(n)
	if layout == nil || len(layout.Hexes) != len(specs) {
		return nil, fmt.Errorf("当前新世界地图需要%d块地形", len(specs))
	}
	// Physical inventories: base + Seafarers; for 5–6 add both extensions.
	// Seafarers 5–6 explicitly uses base + base-extension discs (46), returning
	// the ten Seafarers discs to the box.
	terrainMax := []int{5, 5, 5, 5, 5, 3, 19, 2}
	numberMax := []int{0, 0, 2, 3, 3, 3, 3, 0, 3, 3, 3, 3, 2}
	if n > 4 {
		terrainMax = []int{7, 7, 7, 7, 7, 5, 26, 4}
		numberMax = []int{0, 0, 3, 5, 5, 5, 5, 0, 5, 5, 5, 5, 3}
	}
	terrainUsed, numberUsed := make([]int, 8), make([]int, 13)
	productive := 0
	for i, h := range layout.Hexes {
		if h.Resource < 0 || h.Resource > CatanGold {
			return nil, fmt.Errorf("地块%d的地形无效", i+1)
		}
		if h.Resource == CatanDesert || h.Resource == CatanSea {
			if h.Number != 0 {
				return nil, fmt.Errorf("沙漠和海洋不能放数字牌（地块%d）", i+1)
			}
		} else {
			if h.Number < 2 || h.Number > 12 || h.Number == 7 {
				return nil, fmt.Errorf("资源地块%d需要2至12且非7的数字", i+1)
			}
			if h.Resource == CatanGold && (h.Number == 6 || h.Number == 8) {
				return nil, fmt.Errorf("金矿不能放红色数字（地块%d）", i+1)
			}
			productive++
		}
		terrainUsed[h.Resource]++
		if terrainUsed[h.Resource] > terrainMax[h.Resource] {
			return nil, fmt.Errorf("%s超出盒内组件数量（最多%d块）", []string{"森林", "山丘", "牧场", "田地", "山脉", "沙漠", "海洋", "金矿"}[h.Resource], terrainMax[h.Resource])
		}
		if h.Number > 0 {
			numberUsed[h.Number]++
			if numberUsed[h.Number] > numberMax[h.Number] {
				return nil, fmt.Errorf("数字%d超出盒内组件数量（最多%d枚）", h.Number, numberMax[h.Number])
			}
		}
		specs[i].Resource, specs[i].Number = h.Resource, h.Number
	}
	if productive == 0 {
		return nil, fmt.Errorf("地图至少需要一块能够生产资源的地形")
	}
	g := &Catan{Seafarers: &CatanSeafarers{}}
	if err := g.makeScenarioMap(specs); err != nil {
		return nil, err
	}
	for _, e := range g.Edges {
		if len(e.Tiles) == 2 {
			a, b := g.Tiles[e.Tiles[0]].Number, g.Tiles[e.Tiles[1]].Number
			if (a == 6 || a == 8) && (b == 6 || b == 8) {
				return nil, fmt.Errorf("红色数字6和8不能相邻")
			}
		}
	}
	// Every hex-grid coastal vertex has degree at most two. One placed port
	// removes at most three candidate edges, so this bound guarantees any legal
	// placement order still has space for all of the printed ports. Exotic maps
	// that need a coordinated placement order remain unsupported, not silently
	// repaired, given extra ports, or accepted and allowed to strand setup.
	coast := 0
	for _, e := range g.Edges {
		if g.edgeTerrain(e.ID, true) && g.edgeTerrain(e.ID, false) {
			coast++
		}
	}
	ports := 10
	if n > 4 {
		ports = 11
	}
	if coast < 3*ports-2 {
		return nil, fmt.Errorf("海岸线太短，无法保证%d个港口都能安放，请增加海岸边", ports)
	}
	// A settlement occupies one vertex and blocks at most its three neighbors.
	// This conservative bound keeps every legal order of 2N starting placements
	// possible, rather than merely proving a carefully chosen arrangement exists.
	land := 0
	for _, v := range g.Vertices {
		if g.landVertex(v.ID) {
			land++
		}
	}
	if land < 8*n-3 {
		return nil, fmt.Errorf("陆地交点不足以放置所有玩家的起始村庄")
	}
	return g, nil
}

func NewCatanNewWorldWithMap(n int, options CatanOptions, layout *CatanNewWorldMap) (*State, error) {
	geometry, err := catanNewWorldMapGeometry(n, layout)
	if err != nil {
		return nil, err
	}
	s, err := NewCatanNewWorld(n, options)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.HexSize = geometry.Tiles, geometry.Vertices, geometry.Edges, geometry.HexSize
	g.Ports = []CatanPort{}
	g.Robber = -1
	g.Seafarers.Islands = g.findIslands()
	s.Log = append(s.Log, "按开局前确认的地图开始新世界，地形与数字保持不变")
	return s, nil
}
