package game

import (
	"errors"
	"slices"
)

const catanSwamp = 10

// Public map data. Every
// channel includes its mountain source and its coastal outlet bridge site.
type catanRiverChannel struct {
	Tiles  []int `json:"tiles"`
	Outlet int   `json:"outlet"`
}
type catanRiversMap struct {
	NumberSwaps      []CatanNumberSwap   `json:"numberSwaps,omitempty"`
	NumberRecipe     string              `json:"numberRecipe,omitempty"`
	Channels         []catanRiverChannel `json:"channels"`
	Bridges          []int               `json:"bridges"`
	Swamps           []int               `json:"swamps"`
	DoubleNumberTile int                 `json:"doubleNumberTile"` // Base: 2 and 12. Extension: -1.
}

// 2025 T&B p.11 and T&B 5–6 p.6. Row-major tiles, source to mouth.
// The second outlet rotates with the reversed short river in the extension.
func catanRiverRecipe(extended bool) (channels [][]int, terrain [][]int, outlets []int) {
	if extended {
		return [][]int{{13, 19, 24, 28}, {8, 4, 1}, {15, 16, 17}},
			[][]int{{4, 1, 2, catanSwamp}, {4, 1, catanSwamp}, {4, 2, 2}}, []int{1, 3, 4}
	}
	return [][]int{{3, 8, 13, 17}, {5, 10, 15}},
		[][]int{{4, 1, 2, catanSwamp}, {4, 1, catanSwamp}}, []int{1, 5}
}

func catanRiverNumberRecipe(extended bool) (order, numbers []int) {
	if extended {
		return catanExtendedNumberRecipe()
	}
	// Normal counterclockwise spiral from the upper-right corner, omitting
	// the B/2 disc and skipping both swamps. Its 2 is placed with the 12.
	return []int{2, 1, 0, 3, 7, 12, 16, 17, 18, 15, 11, 6, 5, 4, 8, 13, 14, 10, 9},
		[]int{5, 6, 3, 8, 10, 9, 12, 11, 4, 8, 10, 9, 4, 5, 6, 3, 11}
}

func (g *Catan) makeRiversMap() (*catanRiversMap, error) {
	n := len(g.Players)
	if n < 2 || n > 6 || n == 2 && g.Two == nil || g.SetupStep != 0 || g.Rivers != nil || g.Seafarers != nil || g.BaseSetup != nil || g.Fishing != nil || g.CitiesKnights != nil || g.Harbors != nil || g.FriendlyRobber != nil || g.CardEvent != nil || g.RevealedEvent != nil || g.Options.Helpers || (n > 4) != g.Options.FiveSix {
		return nil, errors.New("河流地图仅用于尚未建设的对应人数基础地图")
	}
	for _, v := range g.Vertices {
		if v.Level != 0 || v.Owner != -1 {
			return nil, errors.New("河流地图不能覆盖建筑")
		}
	}
	for _, e := range g.Edges {
		if e.Owner != -1 {
			return nil, errors.New("河流地图不能覆盖路线")
		}
	}
	board := &Catan{Players: make([]CatanPlayer, n)}
	board.makeMap()
	paths, terrain, outlets := catanRiverRecipe(n > 4)
	f := &catanRiversMap{DoubleNumberTile: -1}
	if n > 4 {
		f.NumberRecipe = CatanExtendedNumberRecipe
	}
	reserved := map[int]bool{}
	for i, path := range paths {
		f.Channels = append(f.Channels, catanRiverChannel{Tiles: slices.Clone(path), Outlet: catanFishingSide(board, path[len(path)-1], outlets[i])})
		for j, id := range path {
			reserved[id] = true
			board.Tiles[id].Resource = terrain[i][j]
			if terrain[i][j] == catanSwamp {
				f.Swamps = append(f.Swamps, id)
			}
			if j > 0 {
				edge := -1
				for _, e := range board.Edges {
					if slices.Contains(e.Tiles, path[j-1]) && slices.Contains(e.Tiles, id) {
						edge = e.ID
						break
					}
				}
				if edge < 0 {
					return nil, errors.New("河流地块不相邻")
				}
				f.Bridges = append(f.Bridges, edge)
			}
		}
		f.Bridges = append(f.Bridges, f.Channels[i].Outlet)
	}
	// These are the ordinary hexes left in the box after the prescribed
	// removals, not a randomization of the printed river components.
	counts := []int{4, 1, 2, 4, 1}
	if n > 4 {
		counts = []int{6, 3, 3, 6, 2}
	}
	remaining := []int{}
	for color, count := range counts {
		for range count {
			remaining = append(remaining, color)
		}
	}
	shuffle(remaining)
	at := 0
	for i := range board.Tiles {
		if !reserved[i] {
			board.Tiles[i].Resource = remaining[at]
			at++
		}
		board.Tiles[i].Number = 0
	}
	order, numbers := catanRiverNumberRecipe(n > 4)
	at = 0
	for _, id := range order {
		if board.Tiles[id].Resource == catanSwamp {
			continue
		}
		board.Tiles[id].Number = numbers[at]
		at++
		if n <= 4 && numbers[at-1] == 12 {
			f.DoubleNumberTile = id
		}
	}
	_, ports, _ := catanFishingFrame(n > 4)
	for i, p := range ports {
		board.Ports[i].Edge = catanFishingSide(board, p[0], p[1])
	}
	board.Robber = -1 // The first player must choose either swamp before setup.
	if err := f.validate(board); err != nil {
		return nil, err
	}
	g.Tiles, g.Vertices, g.Edges, g.Ports = board.Tiles, board.Vertices, board.Edges, board.Ports
	g.HexSize, g.Robber = board.HexSize, -1
	return f, nil
}

func (f catanRiversMap) validate(g *Catan) error {
	if g.Transport != nil {
		return g.validateRiversTransportMap()
	}
	if g.Attack != nil {
		return g.validateRiversAttackMap()
	}
	if g.Caravans != nil {
		return g.validateRiversCaravanMap()
	}
	if !validCatanExtendedNumberRecipe(f.NumberRecipe, len(g.Players)) {
		return errors.New("河流数字配置版本无效")
	}
	n := len(g.Players)
	expectedTiles, expectedVertices, expectedEdges := 19, 54, 72
	if n > 4 {
		expectedTiles, expectedVertices, expectedEdges = 30, 80, 109
	}
	if n < 2 || n > 6 || len(g.Tiles) != expectedTiles || len(g.Vertices) != expectedVertices || len(g.Edges) != expectedEdges {
		return errors.New("河流棋盘尺寸不符")
	}
	paths, terrain, outlets := catanRiverRecipe(n > 4)
	if len(f.Channels) != len(paths) || len(f.Swamps) != 2 {
		return errors.New("河流组件数量不符")
	}
	bridges, swamps := []int{}, []int{}
	for i, path := range paths {
		c := f.Channels[i]
		if !slices.Equal(c.Tiles, path) || c.Outlet != catanFishingSide(g, path[len(path)-1], outlets[i]) || c.Outlet < 0 || len(g.edgeTiles(c.Outlet)) != 1 {
			return errors.New("河流源头、路径或河口不符")
		}
		for j, id := range path {
			if g.Tiles[id].Resource != terrain[i][j] {
				return errors.New("印刷河流地形不能打乱")
			}
			if terrain[i][j] == catanSwamp {
				swamps = append(swamps, id)
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
					return errors.New("河流不连续")
				}
				bridges = append(bridges, edge)
			}
		}
		bridges = append(bridges, c.Outlet)
	}
	if !slices.Equal(f.Bridges, bridges) || !slices.Equal(f.Swamps, swamps) {
		return errors.New("河流桥位或沼泽不符")
	}
	counts := make([]int, catanSwamp+1)
	order, numbers := catanRiverNumberRecipe(n > 4)
	original := map[CatanNumberToken]int{}
	for _, t := range g.Tiles {
		original[CatanNumberToken{t.ID, 0}] = t.Number
	}
	if len(f.NumberSwaps) > 0 && !g.riverKnights() {
		return errors.New("普通河流不能交换数字")
	}
	original, err := rewindCatanInvention(g, original, f.NumberSwaps)
	if err != nil {
		return err
	}
	at, double := 0, -1
	for _, id := range order {
		t := g.Tiles[id]
		if t.Resource < 0 || t.Resource > catanSwamp {
			return errors.New("河流地形无效")
		}
		counts[t.Resource]++
		if slices.Contains(swamps, id) {
			if t.Number != 0 {
				return errors.New("沼泽不放数字")
			}
		} else {
			if original[CatanNumberToken{id, 0}] != numbers[at] {
				return errors.New("河流数字螺旋不符")
			}
			if n <= 4 && t.Number == 12 {
				double = id
			}
			at++
		}
	}
	want := []int{4, 3, 3, 4, 3, 0, 0, 0, 0, 0, 2}
	if n > 4 {
		want = []int{6, 5, 6, 6, 5, 0, 0, 0, 0, 0, 2}
	}
	if !slices.Equal(counts, want) || f.DoubleNumberTile != double || g.Robber != -1 && !slices.Contains(swamps, g.Robber) {
		return errors.New("河流地形库存、双数字或强盗位置不符")
	}
	_, ports, _ := catanFishingFrame(n > 4)
	if len(g.Ports) != len(ports) {
		return errors.New("河流港口数量不符")
	}
	portCounts := make([]int, 6)
	for i, p := range g.Ports {
		if p.Resource < -1 || p.Resource > 4 || p.Edge != catanFishingSide(g, ports[i][0], ports[i][1]) || slices.Contains(bridges, p.Edge) {
			return errors.New("河流港口位置或种类不符")
		}
		portCounts[p.Resource+1]++
	}
	portWant := []int{4, 1, 1, 1, 1, 1}
	if n > 4 {
		portWant = []int{5, 1, 1, 2, 1, 1}
	}
	if !slices.Equal(portCounts, portWant) {
		return errors.New("河流港口库存不符")
	}
	return nil
}
