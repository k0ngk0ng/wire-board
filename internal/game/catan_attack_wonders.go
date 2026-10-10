package game

import (
	"errors"
	"reflect"
	"slices"
)

func (g *Catan) attackWonders() bool {
	return g.attackSea() && g.wonders() != nil && g.Seafarers.Scenario == "wonders"
}

func newCatanAttackWondersBoard(n int) (*Catan, *catanAttackMap, error) {
	if n < 3 || n > 6 {
		return nil, nil, errors.New("蛮族奇迹地图需要三至六人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "wonders", Layout: "fixed"}, nil)
	if err != nil {
		return nil, nil, err
	}
	g := s.Catan
	castle := 23
	extra := []catanFishingExtraNumber{{Tile: 46, Number: 12}}
	if n <= 4 {
		for id, t := range map[int]seaTerrain{13: {4, 11}, 15: {1, 10}, 16: {4, 6}, 23: {catanCastle, 0}, 39: {1, 8}} {
			g.Tiles[id].Resource, g.Tiles[id].Number = t.resource, t.number
		}
	} else {
		// Site extension: preserve the extended wonders frame and markers.
		castle = 31
		extra = nil
		g.Tiles[castle].Resource, g.Tiles[castle].Number = catanCastle, 0
	}
	g.Seafarers.Islands = g.findIslands()
	g.Seafarers.StartIslands = []int{g.Seafarers.Islands[castle]}
	mainland := g.Seafarers.StartIslands[0]
	tiles, deserts := []int{}, []int{}
	for _, t := range g.Tiles {
		if g.Seafarers.Islands[t.ID] != mainland {
			continue
		}
		if t.Resource == CatanDesert {
			deserts = append(deserts, t.ID)
		} else if t.Number > 0 {
			tiles = append(tiles, t.ID)
		}
	}
	g.Robber, g.Seafarers.Pirate = -1, -1
	g.Seafarers.VictoryPoints = 12
	supply, gold := 12*len(deserts), 100
	if n > 4 {
		gold = 152
	}
	return g, &catanAttackMap{Sea: CatanAttackSeafarersRules, Castles: []int{castle}, Coast: tiles, Reserves: deserts, ExtraNumbers: extra, Barbarians: supply, Gold: gold}, nil
}

func NewCatanAttackWonders(n int) (*State, error) {
	if n == 2 {
		board, m, err := newCatanAttackWondersBoard(4)
		if err != nil {
			return nil, err
		}
		return newCatanTwoAttackSeaBoard(board, m)
	}
	board, m, err := newCatanAttackWondersBoard(n)
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
	s.Log = []string{"蛮族＋奇迹：每块沙漠放12名蛮族；登陆从最多的沙漠调出，填入同点数中较少的一格，平局由行动玩家选择。每级奇迹都需满足条件，被征服建筑不计。完成四级或12分且奇迹等级独占领先时获胜，无强盗海盗。"}
	if n > 4 {
		s.Log = append(s.Log, "本站五六人配方：保留扩大奇迹图与标记，中央矿山改为城堡，四块沙漠各12蛮族，使用配对回合。")
	}
	s.catanScores()
	return s, s.validateCatanAttack()
}

func (m catanAttackMap) validateWondersSea(g *Catan) error {
	seats := len(g.Players)
	if seats == 2 {
		seats = 4
	}
	if len(g.Players) < 2 || len(g.Players) > 6 || m.Sea != CatanAttackSeafarersRules || m.Rivers != "" || m.Caravans != "" || m.Transport != "" {
		return errors.New("蛮族奇迹组合无效")
	}
	ref, want, err := newCatanAttackWondersBoard(seats)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(m, *want) || !reflect.DeepEqual(g.Tiles, ref.Tiles) || g.HexSize != ref.HexSize || len(g.Edges) != len(ref.Edges) || len(g.Vertices) != len(ref.Vertices) || len(g.Ports) != len(ref.Ports) {
		return errors.New("蛮族奇迹地图无效")
	}
	got, expected := []int{}, []int{}
	for i, p := range g.Ports {
		if p.Edge != ref.Ports[i].Edge {
			return errors.New("蛮族奇迹港口位置无效")
		}
		got = append(got, p.Resource)
		expected = append(expected, ref.Ports[i].Resource)
	}
	slices.Sort(got)
	slices.Sort(expected)
	if !slices.Equal(got, expected) {
		return errors.New("蛮族奇迹港口库存无效")
	}
	for i, v := range g.Vertices {
		w := ref.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("蛮族奇迹交点无效")
		}
	}
	for i, e := range g.Edges {
		w := ref.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("蛮族奇迹边无效")
		}
	}
	sea, w := g.Seafarers, ref.Seafarers
	if sea == nil || sea.Scenario != "wonders" || sea.Rules != w.Rules || sea.Layout != "fixed" || sea.Variable || sea.Pirate != -1 || sea.VictoryPoints != g.attackSeaVictoryPoints(12) || sea.IslandBonus != 1 || sea.NumberRecipe != "" || sea.Fog != nil || sea.Tribe != nil || sea.Cloth != nil || sea.NewWorld != nil || sea.PirateIslands != nil || len(sea.Seats) != len(g.Players) || !slices.Equal(sea.Islands, w.Islands) || !slices.Equal(sea.StartIslands, w.StartIslands) {
		return errors.New("蛮族奇迹海图配置无效")
	}
	if sea.Wonders == nil || !reflect.DeepEqual(sea.Wonders.Markers, w.Wonders.Markers) || !slices.Equal(sea.Wonders.SetupBlocked, w.Wonders.SetupBlocked) || len(sea.Wonders.Cards) != len(w.Wonders.Cards) {
		return errors.New("奇迹标记或数量无效")
	}
	owners := map[int]bool{}
	for i, c := range sea.Wonders.Cards {
		if c.ID != i || c.Owner < -1 || c.Owner >= len(g.Players) || c.Level < 0 || c.Level > 4 || c.Owner == -1 && c.Level != 0 || c.Owner >= 0 && owners[c.Owner] {
			return errors.New("奇迹归属或等级无效")
		}
		if c.Owner >= 0 {
			owners[c.Owner] = true
		}
	}
	return nil
}
