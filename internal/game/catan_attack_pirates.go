package game

import (
	"errors"
	"reflect"
	"slices"
)

func (g *Catan) attackPirates() bool         { return g.attackSea() && g.pirateIslands() != nil }
func (g *Catan) attackBalancedLanding() bool { return g.attackWonders() || g.attackPirates() }

func newCatanAttackPiratesBoard(n int) (*Catan, *catanAttackMap, error) {
	if n < 3 || n > 6 {
		return nil, nil, errors.New("蛮族海盗地图需要三至六人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "pirate_islands", Layout: "fixed"}, nil)
	if err != nil {
		return nil, nil, err
	}
	g := s.Catan
	castle := 27
	extra := []catanFishingExtraNumber{{Tile: 18, Number: 12}}
	if n <= 4 {
		for id, t := range map[int]seaTerrain{4: {3, 10}, 5: {1, 4}, 11: {4, 5}, 12: {0, 9}, 18: {3, 2}, 25: {1, 6}, 26: {0, 9}, 27: {catanCastle, 0}, 33: {0, 11}, 34: {2, 5}, 41: {4, 8}, 42: {2, 4}, 47: {3, 3}, 48: {1, 10}} {
			g.Tiles[id].Resource, g.Tiles[id].Number = t.resource, t.number
		}
	} else {
		castle = 33
		extra = nil
		g.Tiles[castle].Resource, g.Tiles[castle].Number = catanCastle, 0
	}
	// The normal navy starts with settlements and ships on the east island.
	// No barbarian is placed until the first post-setup building is constructed.
	coast := []int{}
	for _, id := range g.pirateIslands().HomeTiles {
		if g.Tiles[id].Number > 0 {
			coast = append(coast, id)
		}
	}
	g.Robber = -1
	g.Seafarers.VictoryPoints = 12
	supply, gold := 36, 100
	if n > 4 {
		supply, gold = 48, 152
	}
	return g, &catanAttackMap{Sea: CatanAttackSeafarersRules, Castles: []int{castle}, Coast: coast, ExtraNumbers: extra, Barbarians: supply, Gold: gold}, nil
}

func NewCatanAttackPirates(n int) (*State, error) {
	if n == 2 {
		return newCatanTwoAttackPirates()
	}
	board, m, err := newCatanAttackPiratesBoard(n)
	if err != nil {
		return nil, err
	}
	s, err := NewCatan(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		return nil, err
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports, g.HexSize, g.Seafarers = board.Tiles, board.Vertices, board.Edges, board.Ports, board.HexSize, board.Seafarers
	g.Robber = -1
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	g.Attack, err = newCatanAttackPieces(g, m)
	if err != nil {
		return nil, err
	}
	s.Log = []string{"蛮族＋海盗群岛：开局全为村庄，建造建筑后主岛登陆；同点数较少蛮族的地块优先，平局自己选择。授勋可升级远征线最近的普通船为战舰，或在城堡招募骑士。夺回要塞后不能继续造船或升级战舰；夺回自己的要塞且达到12分获胜。"}
	if n > 4 {
		s.Log = append(s.Log, "本站五六人配方：保留扩大海盗框架、舰队与要塞，主岛中部粮田改城堡，其他地形和数字不变，48蛮族、152金币、配对回合。")
	}
	s.catanScores()
	return s, s.validateCatanAttack()
}

func newCatanTwoAttackPirates() (*State, error) {
	board, m, err := newCatanAttackPiratesBoard(4)
	if err != nil {
		return nil, err
	}
	// Site two-player map keeps the blue/red expeditions; unoccupied printed
	// orange/white pieces return to the box rather than becoming extra actors.
	for i := 2; i < 4; i++ {
		f := board.pirateIslands().Fortresses[i]
		board.Vertices[f.StartVertex].Owner, board.Vertices[f.StartVertex].Level = -1, 0
		board.Edges[f.StartShip].Owner, board.Edges[f.StartShip].Ship = -1, false
	}
	board.pirateIslands().Fortresses = board.pirateIslands().Fortresses[:2]
	board.pirateIslands().Colors = board.pirateIslands().Colors[:2]
	return newCatanTwoAttackSeaBoard(board, m)
}

func (m catanAttackMap) validatePirateSea(g *Catan) error {
	n := len(g.Players)
	seats := n
	if n == 2 {
		seats = 4
	}
	if n < 2 || n > 6 || m.Sea != CatanAttackSeafarersRules || m.Transport != "" || m.Caravans != "" || m.Rivers != "" {
		return errors.New("蛮族海盗组合无效")
	}
	ref, want, err := newCatanAttackPiratesBoard(seats)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(m, *want) || !reflect.DeepEqual(g.Tiles, ref.Tiles) || len(g.Vertices) != len(ref.Vertices) || len(g.Edges) != len(ref.Edges) || g.HexSize != ref.HexSize || len(g.Ports) != len(ref.Ports) {
		return errors.New("蛮族海盗地图无效")
	}
	got, expected := []int{}, []int{}
	for i, p := range g.Ports {
		if p.Edge != ref.Ports[i].Edge {
			return errors.New("蛮族海盗港口位置无效")
		}
		got = append(got, p.Resource)
		expected = append(expected, ref.Ports[i].Resource)
	}
	slices.Sort(got)
	slices.Sort(expected)
	if !slices.Equal(got, expected) {
		return errors.New("蛮族海盗港口库存无效")
	}
	for i, v := range g.Vertices {
		w := ref.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("蛮族海盗交点无效")
		}
	}
	for i, e := range g.Edges {
		w := ref.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("蛮族海盗边无效")
		}
	}
	sea, w := g.Seafarers, ref.Seafarers
	if sea == nil || sea.Scenario != "pirate_islands" || sea.Rules != w.Rules || sea.Layout != "fixed" || sea.Variable || sea.VictoryPoints != g.attackSeaVictoryPoints(12) || sea.IslandBonus != 0 || len(sea.Seats) != n || !slices.Equal(sea.Islands, w.Islands) || !slices.Equal(sea.StartIslands, w.StartIslands) || sea.NumberRecipe != "" || sea.Fog != nil || sea.Tribe != nil || sea.Cloth != nil || sea.Wonders != nil || sea.NewWorld != nil {
		return errors.New("蛮族海盗海图配置无效")
	}
	p, initial := g.pirateIslands(), ref.pirateIslands()
	if p == nil || p.KnightsRules != "" || p.CityFleet != nil || !slices.Equal(p.HomeTiles, initial.HomeTiles) || !slices.Equal(p.FleetPath, initial.FleetPath) || p.SafeTile != initial.SafeTile || len(p.Fortresses) != n || !slices.Equal(p.Colors, initial.Colors[:n]) {
		return errors.New("蛮族海盗要塞或舰队无效")
	}
	all := true
	for player, f := range p.Fortresses {
		original := initial.Fortresses[player]
		if f.StartVertex != original.StartVertex || f.StartShip != original.StartShip || f.Beachhead != original.Beachhead || f.Vertex != original.Vertex || f.Strength < 0 || f.Strength > 3 {
			return errors.New("要塞初始位置或防御无效")
		}
		vertices, ok := g.pirateRouteVertices(player)
		if !ok || len(f.Route) > 15 || f.Root < 0 || !g.pirateHomeVertex(f.Root) || g.Vertices[f.Root].Owner != player || g.Vertices[f.Root].Level < 1 {
			return errors.New("远征线缺少己方主岛根节点")
		}
		beachPassed := false
		for i, id := range f.Route {
			if !g.edgeTerrain(id, true) {
				return errors.New("远征线不是海路")
			}
			target := f.Beachhead
			if beachPassed {
				target = f.Vertex
			}
			dist := g.pirateSeaDistances(target)
			if vertices[i] != target && dist[vertices[i+1]] != dist[vertices[i]]-1 {
				return errors.New("远征线不是最短合法路线")
			}
			if vertices[i+1] == f.Beachhead {
				beachPassed = true
			}
		}
		if f.Strength > 0 {
			all = false
			if g.Vertices[f.Vertex].Level != 0 || g.Vertices[f.Vertex].Owner != -1 {
				return errors.New("未夺回要塞被普通建筑占用")
			}
		} else if g.Vertices[f.Vertex].Owner != player || g.Vertices[f.Vertex].Level < 1 {
			return errors.New("夺回要塞村庄无效")
		}
	}
	if all && sea.Pirate != -1 || !all && !slices.Contains(p.FleetPath, sea.Pirate) {
		return errors.New("海盗舰队位置无效")
	}
	for _, e := range g.Edges {
		if e.Warship && (!e.Ship || e.Owner < 0 || e.Owner >= n) {
			return errors.New("孤立战舰无效")
		}
	}
	if p.Raid != nil {
		if slices.ContainsFunc(p.Raid.Rewards, func(player int) bool { return player < 0 || player >= n }) || p.Raid.Total < 2 || p.Raid.Total > 12 {
			return errors.New("舰队奖励无效")
		}
	}
	return nil
}
