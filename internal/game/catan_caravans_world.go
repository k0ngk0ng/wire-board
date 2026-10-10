package game

import (
	"errors"
	"reflect"
	"slices"
	"sort"
)

// Site recipe: use the closest interior sea hexes to the frame centre.
// Replacing sea preserves all productive terrain and number discs.
func (g *Catan) caravanWorldHoles() []int {
	ids := []int{}
	for i, t := range g.Tiles {
		if t.Resource == CatanSea {
			if _, err := caravanStarts(g, []int{i}); err == nil {
				ids = append(ids, i)
			}
		}
	}
	frame := newWorldFrame(len(g.Players))
	centre := frame[len(frame)/2]
	distance := func(id int) int { q, r := frame[id].Q-centre.Q, frame[id].R-centre.R; return q*q + q*r + r*r }
	sort.Slice(ids, func(i, j int) bool {
		a, b := distance(ids[i]), distance(ids[j])
		if a == b {
			return ids[i] < ids[j]
		}
		return a < b
	})
	count := 1
	if len(g.Players) > 4 {
		count = 2
	}
	if len(ids) < count {
		return nil
	}
	return ids[:count]
}
func NewCatanCaravansWorld(n int) (*State, error) {
	if n == 2 {
		return newCatanTwoCaravansSea("new_world")
	}
	return newCatanCaravansWorld(n)
}
func newCatanCaravansWorld(n int) (*State, error) {
	if n < 3 || n > 6 {
		return nil, errors.New("商队新世界需要三至六人")
	}
	for attempt := 0; attempt < 100; attempt++ {
		s, err := NewCatanNewWorld(n, CatanOptions{FiveSix: n > 4})
		if err != nil {
			return nil, err
		}
		if err = installCaravansWorld(s); err != nil {
			continue
		}
		return s, nil
	}
	return nil, errors.New("无法生成具有足够港口位置的商队新世界")
}
func (g *Catan) validateCaravansWorld() error {
	n := len(g.Players)
	c, sea := g.Caravans, g.Seafarers
	if !g.caravansSea() || n < 3 && !g.twoCaravansSea() || n > 6 || c.WorldBase == nil || sea.NewWorld == nil || sea.Rules != CatanSeafarersRules || sea.Layout != "prepared" || !sea.Variable || sea.VictoryPoints != g.caravansSeaVictoryPoints(14) || sea.IslandBonus != 1 || len(sea.Seats) != n || len(c.ExtraNumbers) != 0 || sea.Scenario != "new_world" || sea.Fog != nil || sea.Tribe != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil || len(sea.StartIslands) != 0 || sea.NumberRecipe != "" {
		return errors.New("商队新世界配置无效")
	}
	recipeSeats := n
	if n == 2 {
		recipeSeats = 4
	}
	ref, err := catanNewWorldMapGeometry(recipeSeats, c.WorldBase)
	if err != nil {
		return err
	}
	ref.Players = g.Players
	ref.Caravans = c
	holes := ref.caravanWorldHoles()
	for _, id := range holes {
		ref.Tiles[id].Resource = catanWateringHole
	}
	starts, err := caravanStarts(ref, holes)
	if err != nil {
		return err
	}
	supply := 22
	if n > 4 {
		supply = 33
	}
	expected := &catanCaravanMap{WateringHoles: holes, Starts: starts, Supply: supply}
	if !reflect.DeepEqual(expected, c.Map) || !reflect.DeepEqual(ref.Tiles, g.Tiles) || !slices.Equal(g.findIslands(), sea.Islands) || len(ref.Edges) != len(g.Edges) || len(ref.Vertices) != len(g.Vertices) {
		return errors.New("商队新世界地图无效")
	}
	for i, v := range g.Vertices {
		w := ref.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("新世界交点无效")
		}
	}
	for i, e := range g.Edges {
		w := ref.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("新世界路线无效")
		}
	}
	if g.Robber < -1 || g.Robber >= len(g.Tiles) || g.Robber >= 0 && !g.robberLandAllowed(g.Robber) || sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("商队新世界强盗或海盗位置无效")
	}
	w := sea.NewWorld
	ports := []int{-1, -1, -1, -1, -1, 0, 1, 2, 3, 4}
	if n > 4 {
		ports = append(ports, 2)
	}
	got := slices.Clone(w.Ports)
	slices.Sort(got)
	slices.Sort(ports)
	if !slices.Equal(got, ports) || w.Index < 0 || w.Index > len(ports) || len(g.Ports) != w.Index {
		return errors.New("新世界港口库存或进度无效")
	}
	occupied := map[int]bool{}
	for i, p := range g.Ports {
		if p.Edge < 0 || p.Edge >= len(g.Edges) || p.Resource != w.Ports[i] {
			return errors.New("新世界港口无效")
		}
		e := g.Edges[p.Edge]
		if occupied[e.A] || occupied[e.B] || !g.edgeTerrain(e.ID, true) || !g.edgeTerrain(e.ID, false) {
			return errors.New("新世界港口位置无效")
		}
		occupied[e.A], occupied[e.B] = true, true
	}
	return nil
}

func installCaravansWorld(s *State) error {
	n := len(s.Catan.Players)
	g := s.Catan
	g.Seafarers.Rules, g.Seafarers.Layout = CatanSeafarersRules, "prepared"
	g.Caravans = &catanCaravans{Sea: CatanCaravansSeafarersRules, Rules: CatanCaravansRules, Wagons: []catanCaravanWagon{}}
	base := g.NewWorldMap()
	holes := g.caravanWorldHoles()
	if len(holes) == 0 {
		return errors.New("商队新世界水源或海岸容量不足")
	}
	for _, id := range holes {
		g.Tiles[id].Resource = catanWateringHole
	}
	starts, err := caravanStarts(g, holes)
	if err != nil {
		return errors.New("商队新世界水源或海岸容量不足")
	}
	supply := 22
	if n > 4 {
		supply = 33
	}
	g.Caravans.Map = &catanCaravanMap{WateringHoles: holes, Starts: starts, Supply: supply}
	g.Caravans.WorldBase = base
	g.Seafarers.Islands = g.findIslands()
	g.Seafarers.VictoryPoints = 14
	if len(g.worldPortEdges()) < 3*len(g.newWorld().Ports)-2 {
		return errors.New("商队新世界水源或海岸容量不足")
	}
	s.Log = append(s.Log, "商队新世界：14分获胜。本站随机配方以最靠近地图中心的内海替换水源，保留全部生产地形与数字；五六人两处水源、33辆马车。先放港口，再起始建设。")
	s.catanScores()
	return s.validateCaravans()
}

// Layout describes the original sea and terrain, before automatic water sources.
// Copy it into fresh geometry so edits to the caller's draft cannot alter play.
func NewCatanCaravansWorldWithMap(n int, layout *CatanNewWorldMap) (*State, error) {
	recipeSeats := n
	if n == 2 {
		recipeSeats = 4
	}
	if n < 2 || n > 6 {
		return nil, errors.New("商队新世界需要二至六人")
	}
	s, err := NewCatanNewWorldWithMap(recipeSeats, CatanOptions{FiveSix: n > 4}, layout)
	if err != nil {
		return nil, err
	}
	if err = installCaravansWorld(s); err != nil {
		return nil, err
	}
	if n == 2 {
		return newCatanTwoCaravansSeaRecipe(s)
	}
	return s, nil
}
func GenerateCatanCaravansWorldMap(n int) (*CatanNewWorldMap, error) {
	s, err := NewCatanCaravansWorld(n)
	if err != nil {
		return nil, err
	}
	return s.Catan.Caravans.WorldBase, nil
}
func ValidateCatanCaravansWorldMap(n int, layout *CatanNewWorldMap) error {
	_, err := NewCatanCaravansWorldWithMap(n, layout)
	return err
}
