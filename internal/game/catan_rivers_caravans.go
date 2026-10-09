package game

import (
	"errors"
	"math"
	"slices"
)

const CatanRiversCaravansRules = "catan-rivers-caravans-2025"

func (g *Catan) riversCaravans() bool {
	return g.Rivers != nil && g.Caravans != nil && g.Caravans.Rivers == CatanRiversCaravansRules
}

// The extended frame has a printed river through tile 8. Its two watering
// holes therefore use non-river tiles 9 and 21 (explicit site recipe).
func riversCaravanNumbers(extended bool) (holes, order, numbers []int) {
	holes = []int{9}
	_, order, numbers = catanCaravanRecipe(extended)
	if extended {
		holes = []int{9, 21}
	}
	numbers = slices.Clone(numbers)
	for _, face := range []int{2, 12} {
		at := slices.Index(numbers, face)
		numbers = append(numbers[:at], numbers[at+1:]...)
	}
	return
}

func NewCatanRiversCaravans(n int, options CatanOptions, knights bool) (*State, error) {
	var s *State
	var err error
	if n == 2 {
		s, err = NewCatanTwoRivers(n, options)
	} else {
		s, err = NewCatanRivers(n, options)
	}
	if err != nil {
		return nil, err
	}
	g := s.Catan
	holes, order, numbers := riversCaravanNumbers(n > 4)
	river := map[int]bool{}
	for _, ch := range g.Rivers.Map.Channels {
		for _, id := range ch.Tiles {
			river[id] = true
		}
	}
	for _, id := range holes {
		if g.Tiles[id].Resource != 2 {
			found := -1
			for _, tile := range g.Tiles {
				if !river[tile.ID] && !slices.Contains(holes, tile.ID) && tile.Resource == 2 {
					found = tile.ID
					break
				}
			}
			if found < 0 {
				return nil, errors.New("河流商队缺少可替换的草地")
			}
			g.Tiles[found].Resource = g.Tiles[id].Resource
		}
		g.Tiles[id].Resource = catanWateringHole
	}
	m := &catanCaravanMap{WateringHoles: holes, Supply: 22}
	if n > 4 {
		m.Supply = 33
		m.NumberRecipe = CatanExtendedNumberRecipe
	}
	m.Starts, err = caravanStarts(g, holes)
	if err != nil {
		return nil, err
	}
	c := &catanCaravans{Rules: CatanCaravansRules, Rivers: CatanRiversCaravansRules, Map: m, Wagons: []catanCaravanWagon{}}
	at, extra := 0, 0
	for _, id := range order {
		g.Tiles[id].Number = 0
		if slices.Contains(holes, id) || slices.Contains(g.Rivers.Map.Swamps, id) {
			continue
		}
		g.Tiles[id].Number = numbers[at]
		at++
		if numbers[at-1] == 3 && extra < 2 {
			c.ExtraNumbers = append(c.ExtraNumbers, catanFishingExtraNumber{Tile: id, Number: []int{2, 12}[extra]})
			extra++
		}
	}
	g.Rivers.Map.DoubleNumberTile = -1
	g.Caravans = c
	if knights {
		s.enableCitiesKnights()
		g.Rivers.Knights = CatanRiversKnightsRules
		c.Knights = CatanCaravansKnightsRules
		if g.Two != nil {
			g.Two.Knights = CatanTwoKnightsRules
		}
	}
	s.Log = append(s.Log, "河流＋商队：2与12分别叠放在两块3点地上；桥位无论有无桥都可放马车，桥梁按道路计商队加成；12分获胜")
	if n > 4 {
		s.Log = append(s.Log, "本站五六人配方：保留三条河流，水源移至非河流的9号与21号索引地块；两枚数字叠在前两块3点地，其余数字沿原螺旋保留")
	}
	if knights {
		s.Log = append(s.Log, "本站三模块适配：商队以木材和砖块出价，保留河流骑士金币规则与商队骑士15分目标")
	}
	s.catanScores()
	if err = g.validateRivers(); err != nil {
		return nil, err
	}
	if err = s.validateCaravans(); err != nil {
		return nil, err
	}
	if g.Two != nil {
		err = s.validateCatanTwo()
	}
	return s, err
}

func (g *Catan) validateRiversCaravanMap() error {
	if !g.riversCaravans() || g.Fishing != nil || g.Attack != nil || g.Transport != nil || g.Explorer != nil {
		return errors.New("河流商队组合标记无效")
	}
	c, r := g.Caravans, g.Rivers
	expectedVertices, expectedEdges, expectedSize := 54, 72, 62.0
	if len(g.Players) > 4 {
		expectedVertices, expectedEdges, expectedSize = 80, 109, 46
	}
	if len(g.Vertices) != expectedVertices || len(g.Edges) != expectedEdges || g.HexSize != expectedSize {
		return errors.New("河流商队棋盘尺寸无效")
	}
	for i, v := range g.Vertices {
		if v.ID != i || math.IsNaN(v.X) || math.IsNaN(v.Y) || math.IsInf(v.X, 0) || math.IsInf(v.Y, 0) {
			return errors.New("河流商队交点坐标无效")
		}
	}
	for i, tile := range g.Tiles {
		if tile.ID != i || len(tile.Vertices) != 6 || math.IsNaN(tile.X) || math.IsNaN(tile.Y) || math.IsInf(tile.X, 0) || math.IsInf(tile.Y, 0) {
			return errors.New("河流商队地块结构无效")
		}
		for corner, v := range tile.Vertices {
			if v < 0 || v >= len(g.Vertices) {
				return errors.New("河流商队交点索引无效")
			}
			angle := (30 + float64(corner)*60) * math.Pi / 180
			if math.Hypot(g.Vertices[v].X-tile.X-g.HexSize*math.Cos(angle), g.Vertices[v].Y-tile.Y-g.HexSize*math.Sin(angle)) > .001 {
				return errors.New("河流商队地块几何无效")
			}
		}
	}
	for i, e := range g.Edges {
		if e.ID != i || e.A < 0 || e.B < 0 || e.A >= len(g.Vertices) || e.B >= len(g.Vertices) || e.A == e.B || len(e.Tiles) < 1 || len(e.Tiles) > 2 {
			return errors.New("河流商队边结构无效")
		}
		if len(e.Tiles) == 2 && e.Tiles[0] == e.Tiles[1] {
			return errors.New("河流商队边重复地块")
		}
		for _, id := range e.Tiles {
			if id < 0 || id >= len(g.Tiles) {
				return errors.New("河流商队边地块索引无效")
			}
			a, b := slices.Index(g.Tiles[id].Vertices, e.A), slices.Index(g.Tiles[id].Vertices, e.B)
			if a < 0 || b < 0 || ((a+1)%6 != b && (b+1)%6 != a) {
				return errors.New("河流商队边不在地块边界")
			}
		}
	}
	holes, order, numbers := riversCaravanNumbers(len(g.Players) > 4)
	supply := 22
	if len(g.Players) > 4 {
		supply = 33
	}
	if c.Map == nil || r.Map == nil || c.Map.Supply != supply || !slices.Equal(c.Map.WateringHoles, holes) || !validCatanExtendedNumberRecipe(c.Map.NumberRecipe, len(g.Players)) || r.Map.DoubleNumberTile != -1 || !slices.Equal(r.Map.NumberSwaps, c.Map.NumberSwaps) {
		return errors.New("河流商队地图配置无效")
	}
	if len(g.Tiles) != len(order) {
		return errors.New("河流商队地块数量无效")
	}
	original := map[CatanNumberToken]int{}
	for _, tile := range g.Tiles {
		original[CatanNumberToken{tile.ID, 0}] = tile.Number
	}
	original, err := rewindCatanInvention(g, original, r.Map.NumberSwaps)
	if err != nil {
		return err
	}
	at := 0
	extras := []catanFishingExtraNumber{}
	for _, id := range order {
		if slices.Contains(holes, id) || slices.Contains(r.Map.Swamps, id) {
			if g.Tiles[id].Number != 0 {
				return errors.New("水源或沼泽不放数字")
			}
			continue
		}
		if at >= len(numbers) || original[CatanNumberToken{id, 0}] != numbers[at] {
			return errors.New("河流商队数字螺旋无效")
		}
		if numbers[at] == 3 && len(extras) < 2 {
			extras = append(extras, catanFishingExtraNumber{Tile: id, Number: []int{2, 12}[len(extras)]})
		}
		at++
	}
	if at != len(numbers) || !slices.Equal(extras, c.ExtraNumbers) {
		return errors.New("河流商队叠放数字无效")
	}
	// Validate the shared river frame using its original number sequence. The
	// combined recipe above has already checked every actual production disc.
	board := *g
	board.Tiles = slices.Clone(g.Tiles)
	board.Caravans = nil
	board.Robber = -1
	base := *r.Map
	base.NumberSwaps = nil
	base.DoubleNumberTile = -1
	for _, id := range holes {
		if board.Tiles[id].Resource != catanWateringHole {
			return errors.New("河流商队水源地形无效")
		}
		board.Tiles[id].Resource = 2
	}
	riverOrder, riverNumbers := catanRiverNumberRecipe(len(g.Players) > 4)
	at = 0
	for _, id := range riverOrder {
		if slices.Contains(base.Swamps, id) {
			continue
		}
		board.Tiles[id].Number = riverNumbers[at]
		if len(g.Players) <= 4 && riverNumbers[at] == 12 {
			base.DoubleNumberTile = id
		}
		at++
	}
	if err = base.validate(&board); err != nil {
		return err
	}
	starts, err := caravanStarts(g, holes)
	if err != nil || !slices.Equal(starts, c.Map.Starts) {
		return errors.New("河流商队出口无效")
	}
	return nil
}
