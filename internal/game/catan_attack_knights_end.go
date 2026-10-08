package game

import (
	"errors"
	"slices"
)

type catanAttackCityOrder struct {
	From    int `json:"from"`
	To      int `json:"to"`
	Retreat int `json:"retreat"` // -1 for a normal move; owner-selected edge otherwise.
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
		if k.Owner == s.Turn {
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
		if c.at(order.To) >= 0 {
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
			if err := c.move(g, s.Turn, order.From, order.To); err != nil {
				return nil, err
			}
		}
		moved[order.From] = true
	}
	for _, k := range c.Knights {
		if k.Owner == s.Turn && g.Attack.castleEdge(g, k.Edge) {
			return nil, errors.New("请将所有己方城堡骑士移出")
		}
	}
	for _, tile := range g.Attack.Map.Coast {
		battle, err := next.catanAttackCityBattle(tile, die)
		if err != nil {
			return nil, err
		}
		if battle != nil {
			result.Battles = append(result.Battles, *battle)
		}
		if next.Finished {
			break
		}
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
