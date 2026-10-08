package game

import (
	"errors"
	"slices"
)

const catanLake = 9

type catanFishingLake struct {
	Tile    int   `json:"tile"`
	Numbers []int `json:"numbers"`
}

type catanFishingGround struct {
	SeaTile  *int   `json:"seaTile,omitempty"`
	Number   int    `json:"number"`
	Edges    [2]int `json:"edges"`
	Vertices [3]int `json:"vertices"`
}

type catanFishingMap struct {
	NumberRecipe string                    `json:"numberRecipe,omitempty"`
	Lakes        []catanFishingLake        `json:"lakes"`
	Grounds      []catanFishingGround      `json:"grounds"`
	ExtraNumbers []catanFishingExtraNumber `json:"extraNumbers,omitempty"`
}

type catanFishingExtraNumber struct {
	Tile   int `json:"tile"`
	Number int `json:"number"`
}

// Tile/side pairs transcribed from the light-blue Vs and harbor piers in
// T&B 2025 p.9 / 5–6 p.5. Tile rows run from top to bottom; side 0 starts
// at the southeast corner, clockwise. A ground spans TWO boundary edges
// meeting at a concave coastal vertex, and produces at all three vertices.
func catanFishingFrame(extended bool) (inner []int, ports [][2]int, grounds [][4]int) {
	if extended {
		return []int{4, 5, 8, 9, 10, 13, 14, 15, 16, 19, 20, 21, 24, 25},
			[][2]int{{0, 4}, {2, 4}, {6, 5}, {22, 5}, {29, 5}, {28, 0}, {27, 1}, {23, 2}, {12, 2}, {3, 2}, {0, 2}},
			[][4]int{{1, 4, 2, 3}, {2, 5, 6, 4}, {11, 5, 17, 4}, {22, 0, 26, 5}, {27, 0, 28, 1}, {23, 1, 27, 2}, {12, 1, 18, 2}, {7, 2, 12, 3}}
	}
	return []int{4, 5, 8, 9, 10, 13, 14},
		[][2]int{{0, 3}, {1, 4}, {6, 4}, {11, 5}, {15, 0}, {17, 0}, {16, 1}, {12, 2}, {3, 2}},
		[][4]int{{0, 4, 1, 3}, {6, 5, 11, 4}, {11, 0, 15, 5}, {16, 0, 17, 1}, {12, 1, 16, 2}, {0, 2, 3, 3}}
}

func catanFishingSide(g *Catan, tile, side int) int {
	if tile < 0 || tile >= len(g.Tiles) || side < 0 || side >= 6 || len(g.Tiles[tile].Vertices) != 6 {
		return -1
	}
	v := g.Tiles[tile].Vertices
	a, b := v[side], v[(side+1)%6]
	for _, e := range g.Edges {
		if e.A == a && e.B == b || e.A == b && e.B == a {
			return e.ID
		}
	}
	return -1
}

// Only the board fields are replaced, after all validation succeeds.
func (g *Catan) makeFishingMap() (*catanFishingMap, error) {
	n := len(g.Players)
	if n < 3 || n > 6 || g.SetupStep != 0 || g.Seafarers != nil || g.BaseSetup != nil || g.CitiesKnights != nil {
		return nil, errors.New("捕鱼地图只能用于尚未建设的基础地图")
	}
	for _, v := range g.Vertices {
		if v.Level != 0 || v.Owner >= 0 {
			return nil, errors.New("捕鱼地图不能覆盖已放置的建筑")
		}
	}
	for _, e := range g.Edges {
		if e.Owner >= 0 {
			return nil, errors.New("捕鱼地图不能覆盖已放置的路线")
		}
	}
	inner, ports, grounds := catanFishingFrame(n > 4)
	// Rejection sampling conditions the existing variable setup on both
	// deserts being inside, retaining its nonadjacent red numbers and the
	// extended spiral. Swapping terrain after numbering would break those.
	var board *Catan
	for range 1024 {
		next := &Catan{Players: g.Players}
		next.makeMap()
		valid := true
		for _, tile := range next.Tiles {
			if tile.Resource == CatanDesert && !slices.Contains(inner, tile.ID) {
				valid = false
			}
		}
		if valid {
			board = next
			break
		}
	}
	if board == nil {
		return nil, errors.New("无法生成内陆湖泊地图，请重新创建")
	}
	f := &catanFishingMap{}
	if n > 4 {
		f.NumberRecipe = CatanExtendedNumberRecipe
	}
	lakes := []int{}
	for i, tile := range board.Tiles {
		if tile.Resource == CatanDesert {
			lakes = append(lakes, i)
			board.Tiles[i].Resource = catanLake
		}
	}
	shuffle(lakes) // Either inner lake may carry the extension's 4/10 face.
	for i, tile := range lakes {
		numbers := []int{2, 3, 11, 12}
		if i == 1 {
			numbers = []int{4, 10}
		}
		f.Lakes = append(f.Lakes, catanFishingLake{tile, numbers})
	}
	// Use the existing randomized port-type inventory, with the actual frame
	// positions from the fishing diagram instead of the generic base spacing.
	for i, at := range ports {
		board.Ports[i].Edge = catanFishingSide(board, at[0], at[1])
	}
	numbers := []int{4, 5, 6, 8, 9, 10}
	if n > 4 {
		numbers = append(numbers, 5, 9)
	}
	shuffle(numbers)
	for i, at := range grounds {
		a, b := catanFishingSide(board, at[0], at[1]), catanFishingSide(board, at[2], at[3])
		if a < 0 || b < 0 {
			return nil, errors.New("捕鱼海岸边缺失")
		}
		ea, eb := board.Edges[a], board.Edges[b]
		joint, first, last := ea.A, ea.B, eb.A
		if joint != eb.A && joint != eb.B {
			joint, first = ea.B, ea.A
		}
		if last == joint {
			last = eb.B
		}
		f.Grounds = append(f.Grounds, catanFishingGround{Number: numbers[i], Edges: [2]int{a, b}, Vertices: [3]int{first, joint, last}})
	}
	board.Robber = -1
	if err := f.validate(board); err != nil {
		return nil, err
	}
	g.Tiles, g.Vertices, g.Edges, g.Ports = board.Tiles, board.Vertices, board.Edges, board.Ports
	g.HexSize, g.Robber = board.HexSize, -1
	return f, nil
}

func (f catanFishingMap) validate(g *Catan) error {
	if g != nil && g.Explorer != nil {
		return f.validateExplorer(g, g.Explorer.Board)
	}
	if !validCatanExtendedNumberRecipe(f.NumberRecipe, len(g.Players)) || (f.NumberRecipe != "" && g.Seafarers != nil) {
		return errors.New("渔夫数字配置版本无效")
	}
	if len(f.ExtraNumbers) > 0 && (g.Seafarers == nil || g.Seafarers.Scenario != "desert") {
		return errors.New("此捕鱼地图不能迁移生产点数")
	}
	if g.Seafarers != nil {
		if g.Seafarers.Scenario == "new_world" {
			return f.validateNewWorld(g)
		}
		if g.Seafarers.Scenario == "wonders" {
			return f.validateWonders(g)
		}
		if g.Seafarers.Scenario == "cloth" {
			return f.validateCloth(g)
		}
		if g.Seafarers.Scenario == "tribe" {
			return f.validateTribe(g)
		}
		if g.Seafarers.Scenario == "desert" {
			return f.validateDesert(g)
		}
		if g.Seafarers.Scenario == "fog" {
			return f.validateFog(g)
		}
		return f.validateFourIslands(g)
	}
	n := len(g.Players)
	inner, portPositions, groundPositions := catanFishingFrame(n > 4)
	wantTiles, wantLakes := 19, 1
	if n > 4 {
		wantTiles, wantLakes = 30, 2
	}
	if n < 3 || n > 6 || len(g.Tiles) != wantTiles || len(f.Lakes) != wantLakes || len(f.Grounds) != len(groundPositions) || len(g.Ports) != len(portPositions) {
		return errors.New("invalid fishing map inventory")
	}
	lakes := map[int]bool{}
	for i, lake := range f.Lakes {
		want := []int{2, 3, 11, 12}
		if i == 1 {
			want = []int{4, 10}
		}
		if !slices.Contains(inner, lake.Tile) || lakes[lake.Tile] || !slices.Equal(lake.Numbers, want) || g.Tiles[lake.Tile].Resource != catanLake || g.Tiles[lake.Tile].Number != 0 {
			return errors.New("invalid fishing lake")
		}
		lakes[lake.Tile] = true
	}
	for i, t := range g.Tiles {
		if !lakes[i] && (t.Resource < 0 || t.Resource >= 5) {
			return errors.New("unexpected fishing terrain")
		}
	}
	used := map[int]bool{}
	for i, port := range g.Ports {
		at := portPositions[i]
		if port.Edge < 0 || port.Edge >= len(g.Edges) || port.Edge != catanFishingSide(g, at[0], at[1]) || port.Resource < -1 || port.Resource > 4 || used[port.Edge] || len(g.edgeTiles(port.Edge)) != 1 {
			return errors.New("invalid fishing port")
		}
		used[port.Edge] = true
	}
	numbers := []int{}
	for i, ground := range f.Grounds {
		if ground.SeaTile != nil {
			return errors.New("基础捕鱼渔场不能关联海洋地块")
		}
		at := groundPositions[i]
		if ground.Edges != [2]int{catanFishingSide(g, at[0], at[1]), catanFishingSide(g, at[2], at[3])} {
			return errors.New("fishing ground is not on its frame section")
		}
		seen := map[int]bool{}
		for _, id := range ground.Vertices {
			if id < 0 || id >= len(g.Vertices) || seen[id] {
				return errors.New("invalid fishing ground vertex")
			}
			seen[id] = true
		}
		for k, id := range ground.Edges {
			if id < 0 || id >= len(g.Edges) || used[id] || len(g.edgeTiles(id)) != 1 {
				return errors.New("fishing ground overlaps another ground/port or leaves coast")
			}
			e := g.Edges[id]
			a, b := ground.Vertices[k], ground.Vertices[k+1]
			if !(e.A == a && e.B == b || e.A == b && e.B == a) {
				return errors.New("fishing ground vertices do not follow its edges")
			}
			used[id] = true
		}
		numbers = append(numbers, ground.Number)
	}
	slices.Sort(numbers)
	want := []int{4, 5, 6, 8, 9, 10}
	if n > 4 {
		want = []int{4, 5, 5, 6, 8, 9, 9, 10}
	}
	if !slices.Equal(numbers, want) || g.Robber < -1 || g.Robber >= len(g.Tiles) {
		return errors.New("invalid fishing production numbers or robber")
	}
	return nil
}

// Aggregate the entire production before the token economy draws anything.
// Fish never enter ordinary-resource received counts (notably Aqueduct).
func (f catanFishingMap) production(g *Catan, total int) ([]int, error) {
	if err := f.validate(g); err != nil {
		return nil, err
	}
	if total < 2 || total > 12 {
		return nil, errors.New("invalid fishing production total")
	}
	due := make([]int, len(g.Players))
	add := func(vertices []int) {
		for _, id := range vertices {
			v := g.Vertices[id]
			if v.Level > 0 && v.Owner >= 0 && v.Owner < len(due) && !g.Players[v.Owner].Eliminated {
				level := v.Level
				if g.Explorer != nil && catanExplorerHarborAt(g, id) {
					level = 1 // A harbor settlement is not a two-producing city.
				}
				due[v.Owner] += level
			}
		}
	}
	for _, lake := range f.Lakes {
		if lake.Tile != g.Robber && slices.Contains(lake.Numbers, total) {
			add(g.Tiles[lake.Tile].Vertices)
		}
	}
	for _, ground := range f.Grounds {
		if ground.Number == total && !(g.Seafarers != nil && ground.SeaTile != nil && g.Seafarers.Pirate == *ground.SeaTile) {
			add(ground.Vertices[:])
		}
	}
	return due, nil
}
