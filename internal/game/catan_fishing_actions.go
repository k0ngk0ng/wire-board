package game

import (
	"errors"
	"slices"
)

var catanFishCosts = map[string]int{
	"catan_fish_robber": 2, "catan_fish_steal": 3, "catan_fish_resource": 4,
	"catan_fish_road": 5, "catan_fish_dev": 7,
}

// 2025 T&B p.10 says "during your turn", not only the Action phase. Complete
// any current action/response before starting another fish action. The second
// paired player has their own Action phase and passes the same active-seat test.
func (s *State) catanFishActionReady(player int) bool {
	g := s.Catan
	return g != nil && g.Fishing != nil && !s.Finished && !g.setup() && player == s.Turn &&
		player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated &&
		s.CatanPendingActor() < 0 && (s.Phase == "catan_roll" || s.Phase == "catan_turn")
}

func (g *Catan) fishBootTargets(player int) []int {
	result := []int{}
	if g.Fishing == nil || g.Fishing.Tokens.BootOwner != player || player < 0 || player >= len(g.Players) {
		return result
	}
	points := g.Players[player].Score - g.hiddenVictoryPoints(player)
	for p, seat := range g.Players {
		if p != player && !seat.Eliminated && seat.Score-g.hiddenVictoryPoints(p) >= points {
			result = append(result, p)
		}
	}
	return result
}

// At most seven tokens: examine all subsets. Minimize overpayment first,
// then free as many slots as possible, using only the owner's known faces.
func (g *Catan) fishPayment(player, cost int) []int {
	if g.Fishing == nil || player < 0 || player >= len(g.Players) || cost < 1 {
		return nil
	}
	hand := g.Fishing.Tokens.Hands[player]
	best, paid := []int(nil), 999
	for mask := 1; mask < 1<<len(hand); mask++ {
		ids, amount := []int{}, 0
		for i, id := range hand {
			if mask&(1<<i) != 0 {
				ids = append(ids, id)
				amount += catanFishValue(id)
			}
		}
		if amount >= cost && (amount < paid || amount == paid && len(ids) > len(best)) {
			best, paid = ids, amount
		}
	}
	return best
}

func (s *State) catanFishAction(player int, a Action) error {
	g := s.Catan
	if !s.catanFishActionReady(player) {
		return errors.New("请在自己的回合且完成当前步骤后使用鱼或传递旧靴子")
	}
	f := g.Fishing
	if len(a.Take)+len(a.Give)+len(a.Cards) > 0 || a.Skill != "" {
		return errors.New("每次鱼行动单独支付，不能附加资源交易、选牌或助手")
	}
	if a.Type == "catan_fish_boot" {
		if len(a.Tokens) > 0 || !slices.Contains(g.fishBootTargets(player), a.Target) {
			return errors.New("旧靴子只能交给公开分数不少于你的在场对手")
		}
		points := make([]int, len(g.Players))
		for p := range points {
			points[p] = g.Players[p].Score - g.hiddenVictoryPoints(p)
		}
		if err := f.Tokens.passBoot(player, a.Target, points); err != nil {
			return err
		}
		g.Trade = nil
		s.catanLog(player, "把旧靴子交给玩家 %d，对方获胜需要额外 1 分", a.Target+1)
		s.catanVictory() // Passing the boot can immediately lower our winning target.
		return nil
	}
	cost, ok := catanFishCosts[a.Type]
	if !ok {
		return errors.New("未知捕鱼行动")
	}
	// Validate the concrete effect before touching payment. applyCatan also
	// clones the entire state, including route completion and victory effects.
	switch a.Type {
	case "catan_fish_robber":
		if g.Robber < 0 {
			return errors.New("强盗已经在场外")
		}
	case "catan_fish_steal":
		if !slices.Contains(g.cardTheftTargets(player), a.Target) {
			return errors.New("请选择有资源手牌的在场对手")
		}
	case "catan_fish_resource":
		if a.Color < 0 || a.Color >= 5 || g.Bank[a.Color] <= 0 {
			return errors.New("请选择银行仍有库存的一种普通资源")
		}
	case "catan_fish_road":
		roads, _, _ := g.pieces(player)
		if roads >= 15 || !g.canRoad(player, a.Edge) {
			return errors.New("请选择合法连接位置，且需有剩余道路棋子")
		}
	case "catan_fish_dev":
		if len(g.DevDeck) == 0 {
			return errors.New("发展卡牌堆已空")
		}
	}
	if err := f.Tokens.spend(player, a.Tokens, cost); err != nil {
		return err
	}
	g.Trade = nil
	paid := 0
	for _, id := range a.Tokens {
		paid += catanFishValue(id)
	}
	s.catanLog(player, "支付 %d 鱼（费用 %d，多付不找零）", paid, cost)
	switch a.Type {
	case "catan_fish_robber":
		g.Robber = -1
		s.catanLog(player, "用鱼将强盗移到场外，不偷取资源")
	case "catan_fish_steal":
		hand := g.Players[a.Target].Resources
		n := catanRandom(sum(hand))
		for color, count := range hand {
			if n < count {
				hand[color]--
				g.Players[player].Resources[color]++
				break
			}
			n -= count
		}
		s.catanLog(player, "用鱼从玩家 %d 随机偷取一张资源", a.Target+1)
	case "catan_fish_resource":
		g.Bank[a.Color]--
		g.Players[player].Resources[a.Color]++
		s.catanLog(player, "用鱼从银行领取 %s×1", CatanResources[a.Color])
	case "catan_fish_road":
		g.Edges[a.Edge].Owner = player
		s.catanLog(player, "用鱼修建道路 #%d", a.Edge+1)
		// A paid fish road is not one of Road Building's free roads. Keep
		// the current roll/action phase while using all normal route effects.
		return s.catanAfterRoute(CatanRouteCompletion{Player: player, Edge: a.Edge})
	case "catan_fish_dev":
		card := g.DevDeck[len(g.DevDeck)-1]
		g.DevDeck = g.DevDeck[:len(g.DevDeck)-1]
		g.Players[player].Dev[card]++
		g.Players[player].NewDev[card]++
		s.catanLog(player, "用鱼购买一张发展卡（牌面保密）")
		s.catanScores()
		s.catanVictory()
	}
	return nil
}

func (s *State) catanFishLegal(player int) map[string]any {
	legal := map[string]any{"costs": clone(catanFishCosts), "actions": []string{}, "resources": []int{}, "targets": []int{}, "roads": []int{}, "bootTargets": []int{}}
	if !s.catanFishActionReady(player) {
		return legal
	}
	g := s.Catan
	legal["bootTargets"] = g.fishBootTargets(player)
	actions := []string{}
	for _, kind := range []string{"catan_fish_robber", "catan_fish_steal", "catan_fish_resource", "catan_fish_road", "catan_fish_dev"} {
		if g.fishPayment(player, catanFishCosts[kind]) == nil {
			continue
		}
		available := false
		switch kind {
		case "catan_fish_robber":
			available = g.Robber >= 0
		case "catan_fish_steal":
			targets := g.cardTheftTargets(player)
			legal["targets"], available = targets, len(targets) > 0
		case "catan_fish_resource":
			resources := []int{}
			for color, count := range g.Bank[:5] {
				if count > 0 {
					resources = append(resources, color)
				}
			}
			legal["resources"], available = resources, len(resources) > 0
		case "catan_fish_road":
			roads := []int{}
			count, _, _ := g.pieces(player)
			if count < 15 {
				for _, e := range g.Edges {
					if g.canRoad(player, e.ID) {
						roads = append(roads, e.ID)
					}
				}
			}
			legal["roads"], available = roads, len(roads) > 0
		case "catan_fish_dev":
			available = len(g.DevDeck) > 0
		}
		if available {
			actions = append(actions, kind)
		}
	}
	legal["actions"] = actions
	return legal
}
