package game

import (
	"errors"
	"math"
	"slices"
)

const CatanAttackTransportRules = "catan-attack-transport-2025"

// Board-only foundation. Public admission stays closed until the shared
// invasion, development, combat and transport action controllers are integrated.
type catanAttackTransportBoard struct {
	Rules     string            `json:"rules"`
	Attack    catanAttackMap    `json:"attack"`
	Transport catanTransportMap `json:"transport"`
}

func attackTransportRecipe(extended bool) catanAttackRecipe {
	if !extended {
		r := catanAttackBoardRecipe(false)
		r.deserts = nil
		r.coastalResources = [5]int{1, 2, 2, 2, 1}
		return r
	}
	// Site extension: retain all seven transport depots and replace its two
	// deserts with knight castles. Ordinary terrain still totals 28 hexes.
	// Quarries have no number; coastal glassworks start at 12/12/2 and
	// produce wood. The inland cargo castle produces wool on 6.
	return catanAttackRecipe{
		castles:          []int{5, 31},
		coast:            []int{15, 9, 4, 1, 2, 3, 8, 14, 27, 32, 36, 35, 34, 28, 22},
		numbers:          []int{0, 3, 8, 12, 9, 0, 3, 12, 10, 4, 5, 10, 5, 9, 11, 12, 8, 12, 6, 2, 6, 0, 9, 9, 4, 9, 5, 6, 11, 2, 11, 0, 3, 0, 8, 4, 2},
		coastalResources: [5]int{2, 2, 3, 3, 2}, innerResources: [5]int{4, 3, 3, 3, 3},
	}
}
func (b catanAttackTransportBoard) productiveCommodity(tile int) bool {
	for _, site := range b.Transport.Sites {
		if site.Tile == tile {
			return site.Kind == "castle" || site.Kind == "glassworks"
		}
	}
	return false
}
func (b catanAttackTransportBoard) productionResource(tile CatanTile) int {
	for _, site := range b.Transport.Sites {
		if site.Tile != tile.ID {
			continue
		}
		switch site.Kind {
		case "castle":
			return 2
		case "glassworks":
			return 0
		default:
			return CatanDesert
		}
	}
	return tile.Resource
}

// Geometry, depot spokes and crossed edges match the transport diagram. The
// separate knight castle is not a cargo destination.
func attackTransportGeometry(n int) (*Catan, *catanAttackTransportBoard, error) {
	g, m, err := catanTransportGeometry(n)
	if err != nil {
		return nil, nil, err
	}
	r := attackTransportRecipe(n > 4)
	b := &catanAttackTransportBoard{Rules: CatanAttackTransportRules, Transport: *m, Attack: catanAttackMap{Castles: slices.Clone(r.castles), Coast: slices.Clone(r.coast), Barbarians: 36, Gold: m.Gold}}
	if n > 4 {
		b.Attack.Barbarians = 48
	}
	b.Attack.Transport = CatanAttackTransportRules
	b.Transport.Attack = CatanAttackTransportRules
	for _, id := range r.castles {
		g.Tiles[id].Resource = catanCastle
	}
	for i := range g.Tiles {
		g.Tiles[i].Number = r.numbers[i]
	}
	return g, b, nil
}
func newCatanAttackTransportBoard(n int) (*Catan, *catanAttackTransportBoard, error) {
	g, b, err := attackTransportGeometry(n)
	if err != nil {
		return nil, nil, err
	}
	r := attackTransportRecipe(n > 4)
	pool := func(counts [5]int) []int {
		out := []int{}
		for color, count := range counts {
			for range count {
				out = append(out, color)
			}
		}
		shuffle(out)
		return out
	}
	coast, inner := pool(r.coastalResources), pool(r.innerResources)
	for i := range g.Tiles {
		t := &g.Tiles[i]
		if t.Resource == catanCastle || t.Resource == catanTransportTerrain {
			continue
		}
		if slices.Contains(r.coast, i) {
			if len(coast) == 0 {
				return nil, nil, errors.New("沿海配方不足")
			}
			t.Resource = coast[0]
			coast = coast[1:]
		} else {
			if len(inner) == 0 {
				return nil, nil, errors.New("内陆配方不足")
			}
			t.Resource = inner[0]
			inner = inner[1:]
		}
	}
	if len(coast)+len(inner) != 0 {
		return nil, nil, errors.New("蛮族运输地形配方余量无效")
	}
	ports := []int{-1, -1, -1, -1, 0, 1, 2, 3, 4}
	if n > 4 {
		ports = append(ports, -1, 2)
	}
	shuffle(ports)
	for i, color := range ports {
		g.Ports[i].Resource = color
	}
	return g, b, b.validate(g)
}
func (b catanAttackTransportBoard) validate(g *Catan) error {
	if g == nil {
		return errors.New("蛮族运输地图缺失")
	}
	base, want, err := attackTransportGeometry(len(g.Players))
	if err != nil {
		return err
	}
	if b.Attack.Transport != CatanAttackTransportRules || b.Transport.Attack != CatanAttackTransportRules || b.Rules != CatanAttackTransportRules || b.Attack.Rivers != "" || b.Attack.Caravans != "" || b.Transport.Rivers != "" || b.Transport.Caravans != "" || len(b.Transport.NumberSwaps) != 0 || b.Transport.Rules != want.Transport.Rules || b.Transport.Gold != want.Transport.Gold || b.Transport.Barbarians != want.Transport.Barbarians || b.Attack.Barbarians != want.Attack.Barbarians || b.Attack.Gold != want.Attack.Gold || !slices.Equal(b.Attack.Castles, want.Attack.Castles) || !slices.Equal(b.Attack.Coast, want.Attack.Coast) {
		return errors.New("蛮族运输地图版本或组件不符")
	}
	if len(g.Tiles) != len(base.Tiles) || len(g.Vertices) != len(base.Vertices) || len(g.Edges) != len(base.Edges) || len(g.Ports) != len(base.Ports) || len(b.Transport.Sites) != len(want.Transport.Sites) || g.HexSize != base.HexSize {
		return errors.New("蛮族运输地图尺寸不符")
	}
	for i, site := range b.Transport.Sites {
		w := want.Transport.Sites[i]
		if site.Tile != w.Tile || site.Kind != w.Kind || site.Center != w.Center || !slices.Equal(site.Paths, w.Paths) || !slices.Equal(site.Blocked, w.Blocked) {
			return errors.New("蛮族运输货物站或禁建边无效")
		}
	}
	near := func(a, b float64) bool { return !math.IsNaN(a) && !math.IsInf(a, 0) && math.Abs(a-b) < 0.001 }
	counts := [2][5]int{}
	for i, t := range g.Tiles {
		w := base.Tiles[i]
		if t.ID != i || t.Number != w.Number || !near(t.X, w.X) || !near(t.Y, w.Y) || !slices.Equal(t.Vertices, w.Vertices) {
			return errors.New("蛮族运输地块数字或拓扑无效")
		}
		if w.Resource == catanCastle || w.Resource == catanTransportTerrain {
			if t.Resource != w.Resource {
				return errors.New("蛮族运输固定地形无效")
			}
		} else {
			if t.Resource < 0 || t.Resource >= 5 {
				return errors.New("蛮族运输随机地形无效")
			}
			region := 1
			if slices.Contains(b.Attack.Coast, i) {
				region = 0
			}
			counts[region][t.Resource]++
		}
		coastal := false
		for _, e := range base.Edges {
			if len(e.Tiles) == 1 && e.Tiles[0] == i && b.Transport.siteAt(e.A) < 0 && b.Transport.siteAt(e.B) < 0 {
				coastal = true
				break
			}
		}
		if (coastal && (t.Resource < 5 || b.productiveCommodity(i))) != slices.Contains(b.Attack.Coast, i) {
			return errors.New("蛮族运输登陆地块无效")
		}
	}
	r := attackTransportRecipe(len(g.Players) > 4)
	if counts[0] != r.coastalResources || counts[1] != r.innerResources {
		return errors.New("蛮族运输地形库存不符")
	}
	for i, v := range g.Vertices {
		w := base.Vertices[i]
		if v.ID != i || !near(v.X, w.X) || !near(v.Y, w.Y) {
			return errors.New("蛮族运输交点几何无效")
		}
	}
	for i, e := range g.Edges {
		w := base.Edges[i]
		if e.ID != i || e.A != w.A || e.B != w.B || !slices.Equal(e.Tiles, w.Tiles) {
			return errors.New("蛮族运输路线拓扑无效")
		}
	}
	ports := [6]int{}
	for i, p := range g.Ports {
		if p.Edge != base.Ports[i].Edge || p.Resource < -1 || p.Resource > 4 {
			return errors.New("蛮族运输港口无效")
		}
		ports[p.Resource+1]++
	}
	wantPorts := [6]int{4, 1, 1, 1, 1, 1}
	if len(g.Players) > 4 {
		wantPorts = [6]int{5, 1, 1, 2, 1, 1}
	}
	if ports != wantPorts {
		return errors.New("蛮族运输港口库存无效")
	}
	return nil
}
