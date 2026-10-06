package game

import (
	"errors"
	"math"
	"slices"
)

const catanCastle = 12
const catanAttackRules = "catan-barbarian-attack-2025"

type catanAttackMap struct {
	Castles []int `json:"castles"`
	// Productive coastal hexes in the official clockwise battle order.
	Coast      []int `json:"coast"`
	Barbarians int   `json:"barbarians"`
	Gold       int   `json:"gold"`
}

type catanAttackRecipe struct {
	castles, deserts, coast, numbers []int
	coastalResources, innerResources [5]int
}

// Printed numeric faces, not the shared A–Zc spiral. T&B 2025 p15 and
// T&B 5–6 2025 p8. Tile IDs are row-major from the top-left hex.
func catanAttackBoardRecipe(extended bool) catanAttackRecipe {
	if extended {
		return catanAttackRecipe{
			castles: []int{12, 17}, deserts: []int{11, 18},
			coast:            []int{7, 3, 0, 1, 2, 6, 22, 26, 29, 28, 27, 23},
			numbers:          []int{12, 9, 3, 4, 5, 10, 8, 5, 11, 6, 3, 0, 0, 8, 10, 4, 6, 0, 0, 3, 8, 11, 9, 6, 4, 9, 10, 2, 5, 11},
			coastalResources: [5]int{3, 3, 2, 2, 2}, innerResources: [5]int{2, 2, 3, 4, 3},
		}
	}
	return catanAttackRecipe{
		castles: []int{16}, deserts: []int{2},
		coast:            []int{12, 7, 3, 0, 1, 6, 11, 15, 18, 17},
		numbers:          []int{3, 8, 0, 9, 4, 5, 10, 12, 6, 10, 8, 11, 4, 9, 3, 5, 0, 6, 2},
		coastalResources: [5]int{2, 2, 3, 2, 1}, innerResources: [5]int{1, 1, 1, 2, 2},
	}
}

// Internal board/piece foundation only. This does not create a playable
// State: invasions, special cards and end-turn battles must be integrated first.
func newCatanAttackBoard(n int) (*Catan, *catanAttackMap, error) {
	if n < 2 || n > 6 {
		return nil, nil, errors.New("蛮族进攻地图需要2至6位玩家")
	}
	g := &Catan{Players: make([]CatanPlayer, n), Options: CatanOptions{FiveSix: n > 4}}
	g.makeMap()
	recipe := catanAttackBoardRecipe(n > 4)
	resources := func(counts [5]int) []int {
		result := []int{}
		for color, count := range counts {
			for range count {
				result = append(result, color)
			}
		}
		shuffle(result)
		return result
	}
	coast, inner := resources(recipe.coastalResources), resources(recipe.innerResources)
	for id := range g.Tiles {
		tile := &g.Tiles[id]
		tile.Number = recipe.numbers[id]
		switch {
		case slices.Contains(recipe.castles, id):
			tile.Resource = catanCastle
		case slices.Contains(recipe.deserts, id):
			tile.Resource = CatanDesert
		case slices.Contains(recipe.coast, id):
			tile.Resource = coast[0]
			coast = coast[1:]
		default:
			tile.Resource = inner[0]
			inner = inner[1:]
		}
	}
	_, ports, _ := catanFishingFrame(n > 4)
	for i, p := range ports {
		g.Ports[i].Edge = catanFishingSide(g, p[0], p[1])
	}
	g.Robber = -1
	m := &catanAttackMap{Castles: recipe.castles, Coast: recipe.coast, Barbarians: 36, Gold: 100}
	if n > 4 {
		m.Barbarians = 48
		m.Gold = 152
	}
	return g, m, m.validate(g)
}

func (m catanAttackMap) validate(g *Catan) error {
	n := len(g.Players)
	tileCount, vertexCount, edgeCount, size, supply, gold := 19, 54, 72, 62.0, 36, 100
	if n > 4 {
		tileCount, vertexCount, edgeCount, size, supply, gold = 30, 80, 109, 46, 48, 152
	}
	if n < 2 || n > 6 || len(g.Tiles) != tileCount || len(g.Vertices) != vertexCount || len(g.Edges) != edgeCount || g.HexSize != size || m.Barbarians != supply || m.Gold != gold {
		return errors.New("蛮族进攻地图或组件数量不符")
	}
	recipe := catanAttackBoardRecipe(n > 4)
	if !slices.Equal(m.Castles, recipe.castles) || !slices.Equal(m.Coast, recipe.coast) {
		return errors.New("城堡位置或战斗顺序不符")
	}
	counts := [2][5]int{}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	// Record each geometric hex side independently, detecting missing/duplicated
	// edges and corrupted edge-to-hex associations on restore.
	sides := map[[2]int][]int{}
	for id, tile := range g.Tiles {
		if tile.ID != id || tile.Number != recipe.numbers[id] || len(tile.Vertices) != 6 || !finite(tile.X) || !finite(tile.Y) {
			return errors.New("蛮族进攻地块或固定数字无效")
		}
		switch {
		case slices.Contains(recipe.castles, id):
			if tile.Resource != catanCastle {
				return errors.New("城堡不能替换成普通地形")
			}
		case slices.Contains(recipe.deserts, id):
			if tile.Resource != CatanDesert {
				return errors.New("沙漠位置不符")
			}
		default:
			if tile.Resource < 0 || tile.Resource >= 5 {
				return errors.New("普通地形无效")
			}
			region := 1
			if slices.Contains(recipe.coast, id) {
				region = 0
			}
			counts[region][tile.Resource]++
		}
		for k, v := range tile.Vertices {
			if v < 0 || v >= vertexCount {
				return errors.New("地块交点无效")
			}
			point := g.Vertices[v]
			angle := (30 + float64(k)*60) * math.Pi / 180
			if point.ID != v || !finite(point.X) || !finite(point.Y) || math.Hypot(point.X-tile.X-size*math.Cos(angle), point.Y-tile.Y-size*math.Sin(angle)) > 0.001 {
				return errors.New("地块与交点几何不符")
			}
			a, b := v, tile.Vertices[(k+1)%6]
			if a > b {
				a, b = b, a
			}
			key := [2]int{a, b}
			sides[key] = append(sides[key], id)
		}
	}
	if counts[0] != recipe.coastalResources || counts[1] != recipe.innerResources {
		return errors.New("沿海或内陆资源库存不符")
	}
	for i, e := range g.Edges {
		a, b := e.A, e.B
		if a > b {
			a, b = b, a
		}
		key := [2]int{a, b}
		ids, ok := sides[key]
		if !ok || e.ID != i || len(ids) < 1 || len(ids) > 2 || !slices.Equal(ids, e.Tiles) {
			return errors.New("蛮族进攻路线拓扑无效")
		}
		delete(sides, key)
	}
	if len(sides) != 0 {
		return errors.New("蛮族进攻路线缺失")
	}
	for id, tile := range g.Tiles {
		boundary := false
		for _, e := range g.Edges {
			if len(e.Tiles) == 1 && e.Tiles[0] == id {
				boundary = true
			}
		}
		if (tile.Resource < 5 && boundary) != slices.Contains(m.Coast, id) {
			return errors.New("沿海战斗地块不符")
		}
	}
	_, ports, _ := catanFishingFrame(n > 4)
	if len(g.Ports) != len(ports) {
		return errors.New("蛮族进攻港口数量不符")
	}
	portCounts := [6]int{}
	for i, p := range ports {
		actual := g.Ports[i]
		if actual.Edge != catanFishingSide(g, p[0], p[1]) || actual.Resource < -1 || actual.Resource > 4 {
			return errors.New("蛮族进攻港口位置或类型不符")
		}
		portCounts[actual.Resource+1]++
	}
	want := [6]int{4, 1, 1, 1, 1, 1}
	if n > 4 {
		want = [6]int{5, 1, 1, 2, 1, 1}
	}
	if portCounts != want {
		return errors.New("蛮族进攻港口库存不符")
	}
	return nil
}

// The castle prints purple on sides 0/3, green on 1/4 and brown on 2/5.
// Compare actual edge vectors so this remains correct on both board sizes.
func catanAttackLossOrientation(die int) int {
	switch die {
	case 1, 6:
		return 0
	case 2, 5:
		return 1
	case 3, 4:
		return 2
	}
	return -1
}
func (m catanAttackMap) edgeOrientation(g *Catan, edge int) int {
	if edge < 0 || edge >= len(g.Edges) || len(m.Castles) == 0 {
		return -1
	}
	e := g.Edges[edge]
	a, b := g.Vertices[e.A], g.Vertices[e.B]
	for side := 0; side < 3; side++ {
		ref := g.Edges[catanFishingSide(g, m.Castles[0], side)]
		c, d := g.Vertices[ref.A], g.Vertices[ref.B]
		if math.Abs((b.X-a.X)*(d.Y-c.Y)-(b.Y-a.Y)*(d.X-c.X)) < 0.001 {
			return side
		}
	}
	return -1
}
