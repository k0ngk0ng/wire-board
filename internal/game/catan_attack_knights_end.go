package game

import (
	"errors"
	"slices"
)

type catanAttackCityOrder struct {
	Fish    bool  `json:"fish,omitempty"`
	Tokens  []int `json:"tokens,omitempty"`
	From    int   `json:"from"`
	To      int   `json:"to"`
	Retreat int   `json:"retreat"` // -1 for a normal move; owner-selected edge otherwise.
}
type catanAttackCityEnd struct {
	Player  int                     `json:"player"`
	Orders  []catanAttackCityOrder  `json:"orders"`
	Battles []catanAttackCityBattle `json:"battles"`
}

// Internal transaction for a fully confirmed sequence. A live controller must
// collect opponent-owned retreats before invoking this kernel; the acting
// player cannot submit Retreat on the public action endpoint.
func (s *State) catanAttackCityResolveEnd(orders []catanAttackCityOrder, die func() int) (*catanAttackCityEnd, error) {
	if s.Catan == nil || !s.Catan.attackKnights() || s.Finished || s.Phase != "catan_turn" || s.Turn < 0 || s.Turn >= len(s.Catan.Players) || s.Catan.Players[s.Turn].Eliminated || die == nil {
		return nil, errors.New("当前不能执行组合骑士回合末结算")
	}
	if err := s.Catan.Attack.City.validate(s.Catan); err != nil {
		return nil, err
	}
	next := clone(*s)
	g := next.Catan
	c := g.Attack.City
	result := &catanAttackCityEnd{Player: s.Turn, Orders: slices.Clone(orders), Battles: []catanAttackCityBattle{}}
	originals := map[int]bool{}
	for _, k := range c.Knights {
		if g.attackCityCanMove(k.Owner, s.Turn) {
			originals[k.Edge] = true
		}
	}
	moved := map[int]bool{}
	displacements := 0
	for _, order := range orders {
		if !originals[order.From] || moved[order.From] {
			return nil, errors.New("每名己方骑士在回合末最多移动一次")
		}
		// Another piece may reach a vacated original edge, but that does not give
		// it a second move. Reject it through the arrival set as well.
		for _, old := range result.Orders[:len(moved)] {
			if old.To == order.From {
				return nil, errors.New("同一骑士不能借空出的位置重复移动")
			}
		}
		if len(order.Tokens) > 0 && !order.Fish {
			return nil, errors.New("普通移动不能附加鱼支付")
		}
		if c.at(order.To) >= 0 {
			if order.Fish {
				return nil, errors.New("鱼不能用于驱逐骑士")
			}
			if displacements > 0 {
				return nil, errors.New("每回合末最多用一名骑士驱逐对手")
			}
			q, err := c.beginDisplacement(g, s.Turn, order.From, order.To)
			if err != nil {
				return nil, err
			}
			if q != nil {
				if err = c.completeDisplacement(g, q, q.Player, order.Retreat); err != nil {
					return nil, err
				}
			}
			displacements++
		} else {
			if order.Retreat != -1 {
				return nil, errors.New("普通移动不能夹带退让选择")
			}
			if err := c.moveOrder(g, s.Turn, order); err != nil {
				return nil, err
			}
		}
		moved[order.From] = true
	}
	for _, k := range c.Knights {
		if g.attackCityCanMove(k.Owner, s.Turn) && g.Attack.castleEdge(g, k.Edge) && (len(c.destinations(g, k.Edge)) > 0 || displacements == 0 && len(c.displacementTargets(g, k.Edge)) > 0) {
			return nil, errors.New("请将所有己方城堡骑士移出")
		}
	}
	if err := next.catanAttackCityFinishBattles(result, die); err != nil {
		return nil, err
	}

	if !next.Finished {
		next.catanNext()
		next.catanVictory()
	}
	if err := next.Catan.Attack.City.validate(next.Catan); err != nil {
		return nil, err
	}
	*s = next
	return result, nil
}

func (s *State) catanAttackCityFinishBattles(result *catanAttackCityEnd, die func() int) error {
	for _, tile := range s.Catan.Attack.Map.Coast {
		battle, err := s.catanAttackCityBattle(tile, die)
		if err != nil {
			return err
		}
		if battle != nil {
			result.Battles = append(result.Battles, *battle)
			s.catanLog(result.Player, "地块 #%d 战斗胜利：骑士总力量 %d 击退 %d 个蛮族，恢复生产及被征服建筑", tile+1, sum(battle.Strength), battle.Barbarians)
			for player, count := range battle.Prisoners {
				if count > 0 {
					s.catanLog(player, "获得 %d 个俘虏，现有 %d 个（%d 分）", count, s.Catan.Attack.Prisoners[player], s.Catan.Attack.Prisoners[player]/3)
				}
				if battle.Gold[player] > 0 {
					s.catanLog(player, "战斗补偿：金币×%d", battle.Gold[player])
				}
			}
			if battle.LossDie > 0 {
				s.catanLog(result.Player, "损失骰掷出 %d，%d 名骑士返回供应、%d 名骑士降级", battle.LossDie, len(battle.Lost), len(battle.Downgraded))
			}
		}
		if s.Finished {
			break
		}
	}
	return nil
}
