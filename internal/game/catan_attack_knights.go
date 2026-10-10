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
	Treason     *catanAttackCityTreasonPending `json:"treason,omitempty"`
	Sequence    int                            `json:"sequence"`
	Plan        *catanAttackCityPlan           `json:"plan,omitempty"`
	End         *catanAttackCityEnd            `json:"end,omitempty"`
	NumberSwaps []CatanNumberSwap              `json:"numberSwaps,omitempty"`
	Rules       string                         `json:"rules"`
	Knights     []catanAttackCityKnight        `json:"knights"`
	Issued      int                            `json:"issued"`
}

// NewCatanAttackCitiesKnights uses the combined road-knight rules for 3–6 seats.
func NewCatanAttackCitiesKnights(n int) (*State, error) {
	if n < 3 {
		return nil, errors.New("双人道路骑士组合请使用双人规则入口")
	}
	s, err := newCatanAttackCityCore(n)
	if err != nil {
		return nil, err
	}
	return s, s.validateAttackCityState()
}

func newCatanAttackCityCore(n int) (*State, error) {
	if n < 2 || n > 6 {
		return nil, errors.New("蛮族城市骑士需要二至六位玩家")
	}
	s, err := newCatanAttackState(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		return nil, err
	}
	s.enableCitiesKnights()
	if n == 2 {
		s.Catan.Two.Knights = CatanTwoKnightsRules
		s.Catan.Attack.TwoRules = CatanTwoAttackKnightsRules
		s.Log = []string{"双人蛮族城市骑士：每回合两次完整生产，13分获胜；贸易筹码有限", "本站补充规则：采用两家中立道路骑士，不使用共享骑士；当前玩家代办中立移动和回应，骑士不激活、不参战"}
	}
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
		if n < 0 || n > 3 && !slices.Contains(g.Attack.Map.Reserves, tile) || n > 12 || n > 0 && !slices.Contains(g.attackBattleTiles(), tile) {
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
	if g.attackTransport() {
		if err := g.validateAttackTransportPieces(); err != nil {
			return err
		}
	}
	if err := c.validateNumbers(g); err != nil {
		return err
	}
	used := map[int]bool{}
	counts := map[int][3]int{}
	for _, k := range c.Knights {
		if !g.attackCityKnightOwner(k.Owner) || k.Edge < 0 || k.Edge >= len(g.Edges) || used[k.Edge] || k.Strength < 1 || k.Strength > 3 || k.ActivatedAt > g.CitiesKnights.ActionSerial || k.PromotedAt > g.CitiesKnights.ActionSerial {
			return errors.New("道路骑士位置、等级或锁定记录无效")
		}
		used[k.Edge] = true
		if k.Owner < 0 && (k.Active || k.ActivatedAt != 0 || k.Strength > 2) {
			return errors.New("中立道路骑士不能激活或升级三级")
		}
		stock := counts[k.Owner]
		stock[k.Strength-1]++
		counts[k.Owner] = stock
		if stock[k.Strength-1] > 2 {
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
	if !g.attackCityKnightOwner(player) || c.count(player, 1) >= 2 {
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
		s.catanLog(player, "支付 羊毛×1、矿石×1，在城堡路线 #%d 招募一级骑士", action.Edge+1)
	case "catan_attack_knight_activate":
		c.Knights[i].Active = true
		c.Knights[i].ActivatedAt = g.CitiesKnights.ActionSerial
		s.catanLog(player, "支付 粮食×1，激活路线 #%d 的骑士", action.Edge+1)
	case "catan_attack_knight_promote":
		c.Knights[i].Strength++
		c.Knights[i].PromotedAt = g.CitiesKnights.ActionSerial
		s.catanLog(player, "支付 羊毛×1、矿石×1，将路线 #%d 的骑士升至 %d 级", action.Edge+1, c.Knights[i].Strength)
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
	if g.attackTransport() {
		return s.attackTransportCityLanding(dice)
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
	g.clearAttackConqueredMerchant()
	return targets, nil
}

// clearAttackConqueredMerchant returns the trade merchant when its tile is
// conquered, which every path that changes conquest has to call.
func (g *Catan) clearAttackConqueredMerchant() {
	if g == nil || g.CitiesKnights == nil || g.Attack == nil {
		return
	}
	if m := g.CitiesKnights.Merchant; m != nil && g.Attack.conquered(m.Tile) {
		g.CitiesKnights.Merchant = nil
	}
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

func (s *State) catanAttackCityBuildLanding(roll func() [2]int) error {
	if roll == nil || s.Finished || s.Catan.setup() || s.Phase != "catan_turn" {
		return errors.New("当前不能进行建设登陆")
	}
	used := map[int]bool{}
	for len(used) < 3 {
		dice := roll()
		total := dice[0] + dice[1]
		if dice[0] < 1 || dice[0] > 6 || dice[1] < 1 || dice[1] > 6 {
			return errors.New("建设登陆骰子无效")
		}
		if total == 7 || used[total] {
			continue
		}
		used[total] = true
		if _, err := s.catanAttackCityLanding(dice); err != nil {
			return err
		}
	}
	s.catanScores()
	s.catanVictory()
	return nil
}
