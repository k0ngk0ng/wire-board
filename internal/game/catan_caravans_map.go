package game

import (
	"errors"
	"math"
	"slices"
)

const catanWateringHole = 11

// An edge alone is not a placement: two heads may reach the same empty edge
// from opposite ends. Keep the direction so a merged train cannot branch.
type catanCaravanWagon struct {
	Edge int `json:"edge"`
	From int `json:"from"`
}
type catanCaravanMap struct {
	NumberRecipe  string              `json:"numberRecipe,omitempty"`
	WateringHoles []int               `json:"wateringHoles"`
	Starts        []catanCaravanWagon `json:"starts"`
	Supply        int                 `json:"supply"`
}

// Pinned T&B 2025 pp13–14 / T&B 5–6 p7. Extended production numbers
// use the explicitly labelled site recipe.
func catanCaravanRecipe(extended bool) (holes, order, numbers []int) {
	if extended {
		order, numbers := catanExtendedNumberRecipe()
		return []int{8, 21}, order, numbers
	}
	return []int{9}, []int{2, 1, 0, 3, 7, 12, 16, 17, 18, 15, 11, 6, 5, 4, 8, 13, 14, 10, 9},
		[]int{5, 2, 6, 3, 8, 10, 9, 12, 11, 4, 8, 10, 9, 4, 5, 6, 3, 11}
}

func caravanStarts(g *Catan, holes []int) ([]catanCaravanWagon, error) {
	starts := []catanCaravanWagon{}
	for _, id := range holes {
		if id < 0 || id >= len(g.Tiles) || len(g.Tiles[id].Vertices) != 6 {
			return nil, errors.New("商队水源地块无效")
		}
		// Three huts face screen bottom, northwest and northeast;
		// corners are southeast=0, south=1, southwest=2, northwest=3, north=4, northeast=5.
		for _, corner := range []int{1, 3, 5} {
			from := g.Tiles[id].Vertices[corner]
			found := -1
			for _, e := range g.Edges {
				if (e.A == from || e.B == from) && !slices.Contains(e.Tiles, id) {
					if found >= 0 {
						return nil, errors.New("水源出口不唯一")
					}
					found = e.ID
				}
			}
			if found < 0 {
				return nil, errors.New("水源出口缺失")
			}
			starts = append(starts, catanCaravanWagon{found, from})
		}
	}
	return starts, nil
}

func (g *Catan) makeCaravansMap() (*catanCaravanMap, error) {
	n := len(g.Players)
	if n < 2 || n == 2 && g.Two == nil || n > 6 || (n > 4) != g.Options.FiveSix || g.SetupStep != 0 || g.BaseSetup != nil || g.Rivers != nil || g.Fishing != nil || g.Seafarers != nil || g.CitiesKnights != nil || g.Harbors != nil || g.FriendlyRobber != nil || g.CardEvent != nil || g.RevealedEvent != nil || g.Options.Helpers {
		return nil, errors.New("商队地图仅用于尚未建设的对应人数基础地图")
	}
	for _, v := range g.Vertices {
		if v.Level != 0 || v.Owner >= 0 {
			return nil, errors.New("商队地图不能覆盖建筑")
		}
	}
	for _, e := range g.Edges {
		if e.Owner != -1 {
			return nil, errors.New("商队地图不能覆盖路线")
		}
	}
	for _, t := range g.Tiles {
		if t.Resource > 5 {
			return nil, errors.New("商队地图不能覆盖其他剧本地形")
		}
	}
	board := &Catan{Players: make([]CatanPlayer, n), Two: g.Two}
	board.makeMap()
	holes, order, numbers := catanCaravanRecipe(n > 4)
	f := &catanCaravanMap{WateringHoles: holes, Supply: 22}
	if n > 4 {
		f.Supply = 33
		f.NumberRecipe = CatanExtendedNumberRecipe
	}
	terrain := []int{}
	counts := []int{4, 3, 4, 4, 3}
	if n > 4 {
		counts = []int{6, 5, 6, 6, 5}
	}
	for color, count := range counts {
		for range count {
			terrain = append(terrain, color)
		}
	}
	shuffle(terrain)
	at := 0
	for i := range board.Tiles {
		board.Tiles[i].Number = 0
		if slices.Contains(holes, i) {
			board.Tiles[i].Resource = catanWateringHole
		} else {
			board.Tiles[i].Resource = terrain[at]
			at++
		}
	}
	at = 0
	for _, i := range order {
		if !slices.Contains(holes, i) {
			board.Tiles[i].Number = numbers[at]
			at++
		}
	}
	// The physical frame positions are shared with the T&B fishing layout.
	_, ports, _ := catanFishingFrame(n > 4)
	for i, p := range ports {
		board.Ports[i].Edge = catanFishingSide(board, p[0], p[1])
	}
	var err error
	f.Starts, err = caravanStarts(board, holes)
	if err != nil {
		return nil, err
	}
	board.Robber = -1
	if err = f.validate(board); err != nil {
		return nil, err
	}
	g.Tiles, g.Vertices, g.Edges, g.Ports = board.Tiles, board.Vertices, board.Edges, board.Ports
	g.HexSize, g.Robber = board.HexSize, -1
	return f, nil
}

func (f catanCaravanMap) validate(g *Catan) error {
	if !validCatanExtendedNumberRecipe(f.NumberRecipe, len(g.Players)) {
		return errors.New("商队数字配置版本无效")
	}
	n := len(g.Players)
	tiles, vertices, edges, supply := 19, 54, 72, 22
	if n > 4 {
		tiles, vertices, edges, supply = 30, 80, 109, 33
	}
	if n < 2 || n == 2 && g.Two == nil || n > 6 || len(g.Tiles) != tiles || len(g.Vertices) != vertices || len(g.Edges) != edges || f.Supply != supply {
		return errors.New("商队地图尺寸或马车供应不符")
	}
	holes, order, numbers := catanCaravanRecipe(n > 4)
	if !slices.Equal(f.WateringHoles, holes) {
		return errors.New("商队水源位置不符")
	}
	counts := make([]int, 5)
	at := 0
	expectedSize := 62.0
	if n > 4 {
		expectedSize = 46
	}
	if g.HexSize != expectedSize {
		return errors.New("商队六边形尺寸不符")
	}
	for i, v := range g.Vertices {
		if v.ID != i || math.IsNaN(v.X) || math.IsNaN(v.Y) || math.IsInf(v.X, 0) || math.IsInf(v.Y, 0) {
			return errors.New("商队交点坐标损坏")
		}
	}
	for i, t := range g.Tiles {
		if t.ID != i || len(t.Vertices) != 6 || math.IsNaN(t.X) || math.IsNaN(t.Y) || math.IsInf(t.X, 0) || math.IsInf(t.Y, 0) {
			return errors.New("商队地块结构损坏")
		}
		for k, v := range t.Vertices {
			if v < 0 || v >= len(g.Vertices) {
				return errors.New("商队地块交点损坏")
			}
			angle := (30 + float64(k)*60) * math.Pi / 180
			if math.Hypot(g.Vertices[v].X-t.X-g.HexSize*math.Cos(angle), g.Vertices[v].Y-t.Y-g.HexSize*math.Sin(angle)) > .001 {
				return errors.New("商队地块与交点坐标不符")
			}
		}
		if slices.Contains(holes, i) {
			if t.Resource != catanWateringHole || t.Number != 0 {
				return errors.New("水源不可生产资源")
			}
		} else {
			if t.Resource < 0 || t.Resource >= 5 {
				return errors.New("商队普通地形不符")
			}
			counts[t.Resource]++
		}
	}
	want := []int{4, 3, 4, 4, 3}
	if n > 4 {
		want = []int{6, 5, 6, 6, 5}
	}
	if !slices.Equal(counts, want) {
		return errors.New("商队地形库存不符")
	}
	for _, id := range order {
		if !slices.Contains(holes, id) {
			if g.Tiles[id].Number != numbers[at] {
				return errors.New("商队生产数字顺序不符")
			}
			at++
		}
	}
	for i, e := range g.Edges {
		if e.ID != i || e.A < 0 || e.A >= vertices || e.B < 0 || e.B >= vertices || e.A == e.B || len(e.Tiles) < 1 || len(e.Tiles) > 2 {
			return errors.New("商队边结构损坏")
		}
		for _, id := range e.Tiles {
			if id < 0 || id >= tiles || !slices.Contains(g.Tiles[id].Vertices, e.A) || !slices.Contains(g.Tiles[id].Vertices, e.B) {
				return errors.New("商队边与地块不一致")
			}
			a, b := slices.Index(g.Tiles[id].Vertices, e.A), slices.Index(g.Tiles[id].Vertices, e.B)
			if (a+1)%6 != b && (b+1)%6 != a {
				return errors.New("商队边不是六边形边界")
			}
		}
		if len(e.Tiles) == 2 && e.Tiles[0] == e.Tiles[1] {
			return errors.New("商队边重复关联地块")
		}
	}
	starts, err := caravanStarts(g, holes)
	if err != nil || !slices.Equal(starts, f.Starts) {
		return errors.New("商队起点或朝向不符")
	}
	_, ports, _ := catanFishingFrame(n > 4)
	if len(g.Ports) != len(ports) {
		return errors.New("商队港口数量不符")
	}
	portCounts := make([]int, 6)
	for i, p := range ports {
		saved := g.Ports[i]
		if saved.Edge != catanFishingSide(g, p[0], p[1]) || saved.Resource < -1 || saved.Resource > 4 {
			return errors.New("商队港口位置不符")
		}
		portCounts[saved.Resource+1]++
	}
	wantedPorts := []int{4, 1, 1, 1, 1, 1}
	if n > 4 {
		wantedPorts = []int{5, 1, 1, 2, 1, 1}
	}
	if !slices.Equal(portCounts, wantedPorts) {
		return errors.New("商队港口库存不符")
	}
	return nil
}
