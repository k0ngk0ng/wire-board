package game

import (
	"errors"
	"slices"
)

const CatanAttackKnightsRules = "catan-attack-knights-2025"

// Edge knights deliberately remain separate from vertex knights and the
// ordinary Attack army. Public construction is enabled only after all of the
// combination's phase and progress-card handlers are connected.
type catanAttackCityKnight struct {
	Owner       int    `json:"owner"`
	Edge        int    `json:"edge"`
	Strength    int    `json:"strength"`
	Active      bool   `json:"active"`
	ActivatedAt uint64 `json:"activatedAt"`
	PromotedAt  uint64 `json:"promotedAt"`
}
type catanAttackCity struct {
	NumberSwaps []CatanNumberSwap       `json:"numberSwaps,omitempty"`
	Rules       string                  `json:"rules"`
	Knights     []catanAttackCityKnight `json:"knights"`
	Issued      int                     `json:"issued"`
}

func newCatanAttackCityCore(n int) (*State, error) {
	if n < 3 || n > 6 {
		return nil, errors.New("此内部组合核心暂用于三至六人；双人接续另行接入")
	}
	s, err := NewCatanAttack(n)
	if err != nil {
		return nil, err
	}
	s.enableCitiesKnights()
	a := s.Catan.Attack
	a.City = &catanAttackCity{Rules: CatanAttackKnightsRules, Knights: []catanAttackCityKnight{}}
	a.Deck, a.Discard = []string{}, []string{}
	return s, a.City.validate(s.Catan)
}
func (g *Catan) attackKnights() bool {
	return g.Attack != nil && g.Attack.City != nil && g.Attack.City.Rules == CatanAttackKnightsRules && g.CitiesKnights != nil
}
func (c *catanAttackCity) validate(g *Catan) error {
	if g == nil || !g.attackKnights() || g.Attack.Map == nil || len(g.CitiesKnights.Players) != len(g.Players) || c.Rules != CatanAttackKnightsRules || g.CitiesKnights.BarbarianPosition != 0 || g.CitiesKnights.Invasions != 0 || len(g.CitiesKnights.Knights) != 0 || len(g.Attack.Knights) != 0 || len(g.Attack.Deck) != 0 || len(g.Attack.Discard) != 0 || g.Attack.Pending != nil || c.Issued < 0 || c.Issued > catanGoldLedgerLimit {
		return errors.New("蛮族城市骑士组件或供应记录无效")
	}
	if len(g.Attack.Barbarians) != len(g.Tiles) || len(g.Attack.Prisoners) != len(g.Players) {
		return errors.New("组合蛮族库存维度无效")
	}
	total := 0
	for tile, n := range g.Attack.Barbarians {
		if n < 0 || n > 3 || n > 0 && !slices.Contains(g.Attack.Map.Coast, tile) {
			return errors.New("组合蛮族位置或数量无效")
		}
		total += n
	}
	for _, n := range g.Attack.Prisoners {
		if n < 0 || n > catanGoldLedgerLimit {
			return errors.New("组合俘虏数量无效")
		}
		total += n
	}
	if total > g.Attack.Map.Barbarians+c.Issued {
		return errors.New("组合蛮族总库存超过已发放数量")
	}
	if err := c.validateNumbers(g); err != nil {
		return err
	}
	used := map[int]bool{}
	counts := make([][3]int, len(g.Players))
	for _, k := range c.Knights {
		if k.Owner < 0 || k.Owner >= len(g.Players) || g.Players[k.Owner].Eliminated || k.Edge < 0 || k.Edge >= len(g.Edges) || used[k.Edge] || k.Strength < 1 || k.Strength > 3 || k.ActivatedAt > g.CitiesKnights.ActionSerial || k.PromotedAt > g.CitiesKnights.ActionSerial {
			return errors.New("道路骑士位置、等级或锁定记录无效")
		}
		used[k.Edge] = true
		counts[k.Owner][k.Strength-1]++
		if counts[k.Owner][k.Strength-1] > 2 {
			return errors.New("每位玩家每级骑士最多两枚")
		}
	}
	return nil
}
func (c *catanAttackCity) at(edge int) int {
	for i, k := range c.Knights {
		if k.Edge == edge {
			return i
		}
	}
	return -1
}
func (c *catanAttackCity) count(player, strength int) int {
	count := 0
	for _, k := range c.Knights {
		if k.Owner == player && k.Strength == strength {
			count++
		}
	}
	return count
}
func (c *catanAttackCity) recruitEdges(g *Catan, player int) []int {
	out := []int{}
	if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || c.count(player, 1) >= 2 {
		return out
	}
	for _, e := range g.Edges {
		if g.Attack.castleEdge(g, e.ID) && c.at(e.ID) < 0 {
			out = append(out, e.ID)
		}
	}
	return out
}
func (s *State) catanAttackCityKnightAction(player int, action Action) error {
	g := s.Catan
	if !g.attackKnights() || s.Finished || s.Phase != "catan_turn" || player != s.Turn {
		return errors.New("请在自己的行动阶段操作道路骑士")
	}
	c := g.Attack.City
	i := c.at(action.Edge)
	cost := []int{0, 0, 1, 0, 1}
	switch action.Type {
	case "catan_attack_knight_recruit":
		if !slices.Contains(c.recruitEdges(g, player), action.Edge) {
			return errors.New("请选择城堡周围的空边，并保留一级骑士库存")
		}
	case "catan_attack_knight_activate", "catan_attack_knight_promote":
		if i < 0 || c.Knights[i].Owner != player {
			return errors.New("请选择己方道路骑士")
		}
		k := c.Knights[i]
		if action.Type == "catan_attack_knight_activate" {
			if k.Active {
				return errors.New("骑士已经激活")
			}
			cost = []int{0, 0, 0, 1, 0}
		} else if k.Strength >= 3 || k.PromotedAt == g.CitiesKnights.ActionSerial || c.count(player, k.Strength+1) >= 2 || k.Strength == 2 && g.CitiesKnights.Players[player].Improvements[CatanPolitics] < 3 {
			return errors.New("骑士升级等级、库存或政治建设不满足")
		}
	default:
		return errors.New("未知道路骑士行动")
	}
	if !catanHas(g.Players[player].Resources, cost) {
		return errors.New("道路骑士行动所需资源不足")
	}
	catanMove(g.Players[player].Resources, g.Bank, cost)
	g.Trade = nil
	switch action.Type {
	case "catan_attack_knight_recruit":
		c.Knights = append(c.Knights, catanAttackCityKnight{Owner: player, Edge: action.Edge, Strength: 1})
	case "catan_attack_knight_activate":
		c.Knights[i].Active = true
		c.Knights[i].ActivatedAt = g.CitiesKnights.ActionSerial
	case "catan_attack_knight_promote":
		c.Knights[i].Strength++
		c.Knights[i].PromotedAt = g.CitiesKnights.ActionSerial
	}
	return nil
}

// The component stock is logically unlimited in this combination. Issued
// records replacement pieces corresponding to the printed prisoner-to-VP
// exchange, without losing the player's cumulative prisoner score.
func (s *State) catanAttackCityLanding(dice [2]int) ([]int, error) {
	g := s.Catan
	if !g.attackKnights() || dice[0] < 1 || dice[0] > 6 || dice[1] < 1 || dice[1] > 6 {
		return nil, errors.New("道路蛮族登陆骰无效")
	}
	a := g.Attack
	c := a.City
	targets := []int{}
	if dice[0]+dice[1] == 7 {
		return targets, nil
	}
	for _, id := range a.Map.Coast {
		if g.Tiles[id].Number == dice[0]+dice[1] && a.Barbarians[id] < 3 {
			targets = append(targets, id)
		}
	}
	missing := max(0, len(targets)-(a.supply()+c.Issued))
	if c.Issued > catanGoldLedgerLimit-missing {
		return nil, errors.New("蛮族记账数量超过上限")
	}
	c.Issued += missing
	for _, id := range targets {
		a.Barbarians[id]++
	}
	if m := g.CitiesKnights.Merchant; m != nil && a.conquered(m.Tile) {
		g.CitiesKnights.Merchant = nil
	}
	return targets, nil
}

func (g *Catan) attackCityMetropolis(vertex int) bool {
	return g.attackKnights() && slices.Contains(g.CitiesKnights.Metropolises[:], vertex)
}
func (g *Catan) attackCityImprovementBlocked(player, track int) bool {
	if !g.attackKnights() || track < 0 || track > 2 {
		return false
	}
	vertex := g.CitiesKnights.Metropolises[track]
	return vertex >= 0 && vertex < len(g.Vertices) && g.Vertices[vertex].Owner == player && g.Attack.conqueredBuilding(g, vertex)
}
func (g *Catan) attackCityInventionTiles() []int {
	out := []int{}
	if !g.attackKnights() {
		return out
	}
	for _, t := range g.Tiles {
		if !slices.Contains(g.Attack.Map.Coast, t.ID) && inventionNumber(t.Number) {
			out = append(out, t.ID)
		}
	}
	return out
}
