package game

import "errors"

// Drawn is persisted to prevent refresh from rerolling the random draw. Only
// the active player sees it; all participants see their own resulting hands.
type CatanTwoTrade struct {
	Resume string `json:"resume"`
	Drawn  []int  `json:"drawn"`
}

func (s *State) validateCatanTwoTokens() error {
	g, q := s.Catan, s.Catan.Two
	if len(q.Tokens) != 2 || q.Bank < 0 || q.Bank > 20 || q.Tokens[0] < 0 || q.Tokens[0] > 20 || q.Tokens[1] < 0 || q.Tokens[1] > 20 || q.Bank+q.Tokens[0]+q.Tokens[1] != 20 {
		return errors.New("双人贸易筹码总量无效")
	}
	if g.setup() && (q.Spent || q.KnightExchanged || q.Trade != nil) {
		return errors.New("起始建设不能使用贸易筹码")
	}
	if trade := q.Trade; trade != nil {
		if s.Finished || s.Turn < 0 || s.Turn >= len(g.Players) || q.Pending != nil || !q.Spent || q.Sequence == 0 || s.Phase != "catan_two_trade" || g.Trade != nil || !catanBundle(trade.Drawn) || sum(trade.Drawn) < 1 || sum(trade.Drawn) > 2 || !catanHas(g.Players[s.Turn].Resources, trade.Drawn) || sum(g.Players[s.Turn].Resources) < 2 {
			return errors.New("双人强制交易响应无效")
		}
		if (trade.Resume != "catan_roll" || len(q.Rolls) >= 2) && (trade.Resume != "catan_turn" || len(q.Rolls) != 2) {
			return errors.New("强制交易返回阶段无效")
		}
	} else if s.Phase == "catan_two_trade" {
		return errors.New("强制交易响应缺失")
	}
	return nil
}

func (s *State) catanTwoTokenWindow(player int) bool {
	g := s.Catan
	// 2025 T&B p8 permits spending before rolling dice, including the second
	// production roll. A complete first production (discard/robber/theft) must
	// finish before the state returns to catan_roll; never interrupt a choice.
	return g.Two != nil && !s.Finished && player == s.Turn && player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated && !g.setup() && g.Two.Pending == nil && g.Two.Trade == nil && ((s.Phase == "catan_roll" && len(g.Two.Rolls) < 2) || (s.Phase == "catan_turn" && len(g.Two.Rolls) == 2))
}

func (g *Catan) twoTokenCost(player int) int {
	if g.Players[player].Score-g.hiddenVictoryPoints(player) > g.Players[1-player].Score-g.hiddenVictoryPoints(1-player) {
		return 2
	}
	return 1
}

// Supply exhaustion is not yet sourced. This internal-only protective guard
// rolls back the entire action instead of minting tokens or silently paying a
// partial reward. Resolving the official rule remains a release gate.
func (s *State) catanTwoEarn(player, count int) error {
	q := s.Catan.Two
	if q.Bank < count {
		return errors.New("贸易筹码供应不足，该边界规则尚待核实")
	}
	q.Bank -= count
	q.Tokens[player] += count
	if count > 0 {
		s.catanLog(player, "获得 %d 枚贸易筹码", count)
	}
	return nil
}

func (g *Catan) twoDesert() int {
	for _, tile := range g.Tiles {
		if tile.Resource == CatanDesert {
			return tile.ID
		}
	}
	return -1
}

func (s *State) catanTwoTokenAction(player int, a Action) error {
	if !s.catanTwoTokenWindow(player) {
		return errors.New("请在自己的掷骰前或行动阶段使用贸易筹码")
	}
	g, q := s.Catan, s.Catan.Two
	if len(a.Give)+len(a.Take)+len(a.Cards)+len(a.Tokens) != 0 {
		return errors.New("此行动不能预选资源牌")
	}
	if a.Type == "catan_two_knight" {
		if q.KnightExchanged || g.Players[player].Knights <= 0 {
			return errors.New("每回合只能弃一张已打出的骑士换取筹码")
		}
		if err := s.catanTwoEarn(player, 2); err != nil {
			return err
		}
		// Played knights already entered DevDiscard when they were played.
		// Decrement only the face-up army count, never duplicate that card.
		g.Players[player].Knights--
		q.KnightExchanged = true
		s.catanLog(player, "弃一张已打出的骑士，换取 2 枚贸易筹码")
		s.catanScores()
		s.catanVictory()
		return nil
	}
	cost := g.twoTokenCost(player)
	if q.Spent || q.Tokens[player] < cost {
		return errors.New("本回合已使用贸易筹码，或筹码不足")
	}
	switch a.Type {
	case "catan_two_trade":
		hand := g.Players[1-player].Resources
		count := min(2, sum(hand))
		if count == 0 || sum(g.Players[player].Resources)+count < 2 {
			return errors.New("对手必须有资源，且取牌后应能交还两张")
		}
		trade := &CatanTwoTrade{Resume: s.Phase, Drawn: make([]int, 5)}
		for range count {
			n := catanRandom(sum(hand))
			for color, amount := range hand {
				if n < amount {
					hand[color]--
					g.Players[player].Resources[color]++
					trade.Drawn[color]++
					break
				}
				n -= amount
			}
		}
		q.Trade = trade
		q.Sequence++
		g.Trade = nil
		s.Phase = "catan_two_trade"
		s.catanLog(player, "花费 %d 枚贸易筹码，从玩家 %d 随机取 %d 张资源，待选择交还 2 张", cost, 2-player, count)
	case "catan_two_robber":
		desert := g.twoDesert()
		if desert < 0 || g.Robber == desert {
			return errors.New("强盗已经在沙漠，或没有可用沙漠")
		}
		g.Robber = desert
		s.catanLog(player, "花费 %d 枚贸易筹码，将强盗移回沙漠，不偷牌", cost)
	default:
		return errors.New("未知贸易筹码行动")
	}
	q.Tokens[player] -= cost
	q.Bank += cost
	q.Spent = true
	return nil
}

func (s *State) catanTwoReturn(player int, a Action) error {
	g, q := s.Catan, s.Catan.Two
	if q == nil || q.Trade == nil || s.Phase != "catan_two_trade" || player != s.Turn || a.Type != "catan_two_return" || !catanBundle(a.Give) || sum(a.Give) != 2 || !catanHas(g.Players[player].Resources, a.Give) {
		return errors.New("请由取牌者选择两张资源交还对手")
	}
	for color, count := range a.Give {
		g.Players[player].Resources[color] -= count
		g.Players[1-player].Resources[color] += count
	}
	s.Phase = q.Trade.Resume
	q.Trade = nil
	s.catanLog(player, "向玩家 %d 交还 2 张资源，完成强制交易", 2-player)
	return nil
}

func (s *State) catanTwoReturnBot(player int) (Action, error) {
	g := s.Catan
	if g.Two == nil || g.Two.Trade == nil || player != s.Turn {
		return Action{}, errors.New("inactive forced trade")
	}
	hand := append([]int{}, g.Players[player].Resources...)
	give := make([]int, 5)
	for range 2 {
		best := 0
		for color := range hand {
			if hand[color] > hand[best] {
				best = color
			}
		}
		if hand[best] == 0 {
			return Action{}, errors.New("incomplete forced trade hand")
		}
		give[best]++
		hand[best]--
	}
	return Action{Type: "catan_two_return", Give: give}, nil
}

// Decisions use the bot's own hand and public scores, buildings, army and
// opponent hand count. Never inspect opponent colors or the development deck.
func (s *State) catanTwoOptionalBot(player int) (Action, bool) {
	if !s.catanTwoTokenWindow(player) {
		return Action{}, false
	}
	g, q := s.Catan, s.Catan.Two
	if !q.KnightExchanged && q.Tokens[player] < 2 && q.Bank >= 2 && g.Players[player].Knights > 0 && g.ArmyOwner != player {
		return Action{Type: "catan_two_knight"}, true
	}
	if q.Spent || q.Tokens[player] < g.twoTokenCost(player) {
		return Action{}, false
	}
	if g.Robber >= 0 && g.Robber != g.twoDesert() {
		for _, vertex := range g.Tiles[g.Robber].Vertices {
			if g.Vertices[vertex].Owner == player && g.Vertices[vertex].Level > 0 {
				return Action{Type: "catan_two_robber"}, true
			}
		}
	}
	if sum(g.Players[1-player].Resources) >= 2 {
		for _, count := range g.Players[player].Resources {
			if count >= 3 {
				return Action{Type: "catan_two_trade"}, true
			}
		}
	}
	return Action{}, false
}
