package game

import (
	"errors"
	"slices"
)

// Site adaptation: fish extends an inactive OWN knight to five paths. It never
// activates the knight, changes combat strength, or extends displacement.
func (c *catanAttackCity) fishDestinations(g *Catan, player, from int) []int {
	out := []int{}
	i := c.at(from)
	if !g.fishingAttack() || i < 0 || c.Knights[i].Owner != player || c.Knights[i].Active {
		return out
	}
	for to, d := range c.distances(g, from, 5) {
		if d > 0 && c.at(to) < 0 && !g.Attack.castleEdge(g, to) {
			out = append(out, to)
		}
	}
	slices.Sort(out)
	return out
}
func (c *catanAttackCity) moveOrder(g *Catan, player int, order catanAttackCityOrder) error {
	if !order.Fish {
		if len(order.Tokens) > 0 {
			return errors.New("普通移动不能附加鱼支付")
		}
		return c.move(g, player, order.From, order.To)
	}
	if order.Retreat != -1 || !slices.Contains(c.fishDestinations(g, player, order.From), order.To) {
		return errors.New("请选择未激活的己方骑士及五步以内的空边，鱼不能用于驱逐")
	}
	if err := g.Fishing.Tokens.spend(player, order.Tokens, g.fishActionCost(player, "catan_fish_knight")); err != nil {
		return err
	}
	i := c.at(order.From)
	c.Knights[i].Edge = order.To
	return nil
}

// Live plans publish movement to support an opponent's retreat, but reserve
// fish only in a clone. This also gives undo exact inventory restoration.
func (g *Catan) attackCityFishPreview() (*Catan, error) {
	if !g.fishingAttack() || !g.attackKnights() || g.Attack.City.Plan == nil {
		return g, nil
	}
	trial := clone(*g)
	q := g.Attack.City.Plan
	for _, order := range q.Orders {
		if order.Fish {
			if err := trial.Fishing.Tokens.spend(q.Player, order.Tokens, trial.fishActionCost(q.Player, "catan_fish_knight")); err != nil {
				return nil, err
			}
		}
	}
	return &trial, nil
}
func publicAttackCityOrders(orders []catanAttackCityOrder) []catanAttackCityOrder {
	out := clone(orders)
	for i := range out {
		out[i].Tokens = nil
	}
	return out
}
