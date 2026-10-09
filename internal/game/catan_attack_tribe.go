package game

import (
	"errors"
	"reflect"
	"slices"
)

func newCatanAttackTribeBoard(n int) (*Catan, *catanAttackMap, error) {
	if n < 3 || n > 6 {
		return nil, nil, errors.New("蛮族部落地图需要三至六人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "tribe", Layout: "fixed"}, nil)
	if err != nil {
		return nil, nil, err
	}
	g := s.Catan
	castles, coast := []int{21}, []int{13, 14, 15, 16, 17, 18, 33, 32, 31, 30, 29, 28}
	if n <= 4 {
		for id, t := range map[int]seaTerrain{13: {2, 9}, 14: {0, 8}, 15: {4, 10}, 16: {1, 3}, 17: {3, 4}, 18: {4, 2}, 21: {catanCastle, 0}, 22: {1, 4}, 23: {2, 8}, 24: {3, 5}, 25: {0, 6}, 26: {CatanDesert, 0}, 28: {0, 12}, 29: {1, 6}, 30: {4, 5}, 31: {2, 2}, 32: {1, 11}, 33: {3, 12}} {
			g.Tiles[id].Resource, g.Tiles[id].Number = t.resource, t.number
		}
	} else {
		// Site extension: opposite-corner castles retain a connected mainland.
		// Every coastal number occurs twice, applying the printed tribe exception
		// consistently to all doubled coast numbers on this larger board.
		castles = []int{26, 36}
		coast = []int{17, 18, 19, 20, 21, 22, 23, 24, 25, 35, 45, 44, 43, 42, 41, 40, 39, 38, 37, 27}
		numbers := []int{2, 3, 4, 5, 6, 8, 9, 10, 11, 12, 2, 3, 4, 5, 6, 8, 9, 10, 11, 12}
		for i, id := range coast {
			g.Tiles[id].Number = numbers[i]
		}
		for _, id := range castles {
			g.Tiles[id].Resource, g.Tiles[id].Number = catanCastle, 0
		}
	}
	g.Robber, g.Seafarers.Pirate = -1, -1
	g.Seafarers.Islands = g.findIslands()
	g.Seafarers.StartIslands = []int{g.Seafarers.Islands[coast[0]]}
	g.Seafarers.VictoryPoints = 13
	supply, gold := 36, 100
	if n > 4 {
		supply, gold = 48, 152
	}
	return g, &catanAttackMap{Sea: CatanAttackSeafarersRules, Castles: castles, Coast: coast, Barbarians: supply, Gold: gold}, nil
}

func NewCatanAttackTribe(n int) (*State, error) {
	if n == 2 {
		board, m, err := newCatanAttackTribeBoard(4)
		if err != nil {
			return nil, err
		}
		return newCatanTwoAttackSeaBoard(board, m)
	}
	board, m, err := newCatanAttackTribeBoard(n)
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
	if err = s.reserveAttackTribeRewards(); err != nil {
		return nil, err
	}
	s.Log = []string{"蛮族＋遗忘部落：2和12点在两块对应沿海地同时登陆；奖励牌来自蛮族专用牌堆，领取后立即结算；不使用强盗、海盗，13分获胜。"}
	if n > 4 {
		s.Log = append(s.Log, "本站五六人配方：两处对角城堡、二十格沿海各点数两块，同时登陆；六张奖励牌、48蛮族、152金币、配对回合。")
	}
	s.catanScores()
	return s, s.validateCatanAttack()
}

func (m catanAttackMap) validateTribeSea(g *Catan) error {
	n := len(g.Players)
	seats := n
	if n == 2 {
		seats = 4
	}
	if n < 2 || n > 6 || m.Sea != CatanAttackSeafarersRules || m.Transport != "" || m.Caravans != "" || m.Rivers != "" {
		return errors.New("蛮族部落组合无效")
	}
	ref, want, err := newCatanAttackTribeBoard(seats)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(m, *want) || !reflect.DeepEqual(g.Tiles, ref.Tiles) || len(g.Edges) != len(ref.Edges) || len(g.Vertices) != len(ref.Vertices) || g.HexSize != ref.HexSize {
		return errors.New("蛮族部落地图无效")
	}
	sea := g.Seafarers
	if sea == nil || sea.Scenario != "tribe" || sea.Rules != CatanSeafarersRules || sea.Layout != "fixed" || sea.Variable || sea.VictoryPoints != 13 || sea.IslandBonus != 0 || sea.Pirate != -1 || sea.NumberRecipe != "" || len(sea.Seats) != n || !slices.Equal(sea.Islands, ref.Seafarers.Islands) || !slices.Equal(sea.StartIslands, ref.Seafarers.StartIslands) || sea.Fog != nil || sea.Cloth != nil || sea.NewWorld != nil || sea.Wonders != nil || sea.PirateIslands != nil {
		return errors.New("蛮族部落海图配置无效")
	}
	for i, v := range g.Vertices {
		w := ref.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("蛮族部落交点无效")
		}
	}
	for i, e := range g.Edges {
		w := ref.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("蛮族部落边无效")
		}
	}
	// Piece initialization validates the board before replacing basic reward cards.
	if g.Attack == nil {
		return nil
	}
	return g.validateAttackTribeRewards(ref)
}

func (g *Catan) validateAttackTribeRewards(ref *Catan) error {
	tr, initial := g.tribe(), ref.tribe()
	if tr == nil || tr.AttackRules != CatanAttackTribeRewardRules || tr.ProgressRules != "" || len(tr.Points) != len(g.Players) || len(tr.HeldPorts) != len(g.Players) {
		return errors.New("蛮族部落奖励座位或版本无效")
	}
	if _, err := g.attackTribeReservedCards(); err != nil {
		return err
	}
	for _, d := range tr.Development {
		if !slices.ContainsFunc(initial.Development, func(x CatanTribeDevelopment) bool { return x.Edge == d.Edge }) || g.Edges[d.Edge].Owner != -1 {
			return errors.New("蛮族部落预留牌位置无效")
		}
	}
	total := len(tr.Tokens)
	seen := map[int]bool{}
	for _, edge := range tr.Tokens {
		if seen[edge] || !slices.Contains(initial.Tokens, edge) || g.Edges[edge].Owner != -1 {
			return errors.New("部落胜利点位置无效")
		}
		seen[edge] = true
	}
	for _, p := range tr.Points {
		if p < 0 {
			return errors.New("负数部落分数")
		}
		total += p
	}
	if total != len(initial.Tokens) {
		return errors.New("部落分数不守恒")
	}
	counts, want := [6]int{}, [6]int{}
	seen = map[int]bool{}
	for _, p := range initial.Ports {
		want[p.Resource+1]++
	}
	occupied := map[int]bool{}
	for _, p := range tr.Ports {
		if p.Resource < -1 || p.Resource > 4 || seen[p.Edge] || !slices.ContainsFunc(initial.Ports, func(x CatanPort) bool { return x.Edge == p.Edge }) || g.Edges[p.Edge].Owner != -1 {
			return errors.New("部落预留港口无效")
		}
		seen[p.Edge] = true
		counts[p.Resource+1]++
		e := g.Edges[p.Edge]
		if occupied[e.A] || occupied[e.B] {
			return errors.New("部落预留港口相邻")
		}
		occupied[e.A], occupied[e.B] = true, true
	}
	for _, p := range g.Ports {
		if p.Edge < 0 || p.Edge >= len(g.Edges) || p.Resource < -1 || p.Resource > 4 {
			return errors.New("部落已放港口无效")
		}
		e := g.Edges[p.Edge]
		if occupied[e.A] || occupied[e.B] || !g.edgeTerrain(e.ID, true) || !g.edgeTerrain(e.ID, false) {
			return errors.New("部落已放港口位置无效")
		}
		occupied[e.A], occupied[e.B] = true, true
		counts[p.Resource+1]++
	}
	for _, held := range tr.HeldPorts {
		for _, r := range held {
			if r < -1 || r > 4 {
				return errors.New("持有港口无效")
			}
			counts[r+1]++
		}
	}
	if counts != want {
		return errors.New("部落港口不守恒")
	}
	if q := tr.Pending; q != nil {
		if q.Player < 0 || q.Player >= len(g.Players) || q.Resume != "catan_turn" || q.Helper || len(tr.HeldPorts[q.Player]) == 0 || len(g.tribePortEdges(q.Player)) == 0 {
			return errors.New("部落港口回应无效")
		}
	}
	return nil
}
