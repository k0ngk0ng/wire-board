package game

import (
	"errors"
	"slices"
)

// Historical results are public, but restored saves must not be able to inject
// impossible moves, rewards or pieces through the last-battle projection.
func (s *State) validateAttackCityEnd() error {
	g := s.Catan
	c := g.Attack.City
	q := c.End
	if q == nil {
		return nil
	}
	n := len(g.Players)
	if g.setup() || c.Sequence < 1 || q.Player < 0 || q.Player >= n || len(q.Orders) > g.attackCityMoveLimit() || len(q.Battles) > len(g.Attack.Map.Coast) {
		return errors.New("道路骑士历史结算无效")
	}
	used := map[int]bool{}
	for _, m := range q.Orders {
		if m.From < 0 || m.From >= len(g.Edges) || m.To < 0 || m.To >= len(g.Edges) || m.From == m.To || used[m.From] || g.Attack.castleEdge(g, m.To) || m.Retreat < -1 || m.Retreat >= len(g.Edges) || m.Retreat >= 0 && (m.Retreat == m.To || g.Attack.castleEdge(g, m.Retreat)) {
			return errors.New("道路骑士历史移动无效")
		}
		used[m.From] = true
	}
	previous := -1
	for index, b := range q.Battles {
		coast := slices.Index(g.Attack.Map.Coast, b.Tile)
		if coast <= previous || b.Barbarians < 1 || b.Barbarians > 3 || len(b.Knights) > 6 || len(b.Knights) == 0 || len(b.Strength) != n || len(b.Prisoners) != n || len(b.Gold) != n || sum(b.Prisoners) != b.Barbarians || b.LossDie < 0 || b.LossDie > 6 || b.LossDie == 0 && (!s.Finished || index != len(q.Battles)-1) {
			return errors.New("道路骑士历史战斗顺序或库存无效")
		}
		previous = coast
		strength := make([]int, n)
		pieces := map[int]catanAttackCityKnight{}
		for _, k := range b.Knights {
			_, duplicate := pieces[k.Edge]
			if k.Owner < 0 || k.Owner >= n || k.Edge < 0 || k.Edge >= len(g.Edges) || duplicate || !slices.Contains(g.Edges[k.Edge].Tiles, b.Tile) || !k.Active || k.Strength < 1 || k.Strength > 3 || k.ActivatedAt > g.CitiesKnights.ActionSerial || k.PromotedAt > g.CitiesKnights.ActionSerial {
				return errors.New("道路骑士历史参战棋子无效")
			}
			pieces[k.Edge] = k
			strength[k.Owner] += k.Strength
		}
		if !slices.Equal(strength, b.Strength) || sum(strength) <= b.Barbarians {
			return errors.New("道路骑士历史战斗力量无效")
		}
		losses := make([]int, n)
		changed := map[int]bool{}
		for kind, group := range [][]catanAttackCityKnight{b.Lost, b.Downgraded} {
			for _, k := range group {
				old, ok := pieces[k.Edge]
				if !ok || changed[k.Edge] || b.LossDie == 0 || g.Attack.Map.edgeOrientation(g, k.Edge) != catanAttackLossOrientation(b.LossDie) {
					return errors.New("道路骑士历史损失位置无效")
				}
				if kind == 0 && old != k || kind == 1 && (k.Strength < 1 || k.Strength >= old.Strength) {
					return errors.New("道路骑士历史降级无效")
				}
				normalized := k
				normalized.Strength = old.Strength
				if normalized != old {
					return errors.New("道路骑士历史损失棋子无效")
				}
				changed[k.Edge] = true
				losses[k.Owner]++
			}
		}
		for _, k := range b.Knights {
			if b.LossDie > 0 && g.Attack.Map.edgeOrientation(g, k.Edge) == catanAttackLossOrientation(b.LossDie) && !changed[k.Edge] {
				return errors.New("道路骑士历史遗漏损失")
			}
		}
		for p := range n {
			if b.Prisoners[p] < 0 || b.Gold[p] < 3*losses[p] || b.Gold[p]%3 != 0 || strength[p] == 0 && (b.Prisoners[p] > 0 || b.Gold[p] > 0) {
				return errors.New("道路骑士历史奖励无效")
			}
		}
		if len(b.Contests) > 256 {
			return errors.New("道路骑士历史争夺过长")
		}
		for _, roll := range b.Contests {
			if len(roll.Players) < 2 || len(roll.Players) > n || len(roll.Dice) != len(roll.Players) {
				return errors.New("道路骑士历史争夺维度无效")
			}
			for i, p := range roll.Players {
				if p < 0 || p >= n || strength[p] == 0 || slices.Contains(roll.Players[:i], p) || roll.Dice[i] < 1 || roll.Dice[i] > 6 {
					return errors.New("道路骑士历史争夺点数无效")
				}
			}
		}
	}
	return nil
}
