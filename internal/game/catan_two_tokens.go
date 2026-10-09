package game

import (
	"errors"
	"slices"
)

// Drawn is persisted to prevent refresh from rerolling the random draw. Only
// the active player sees it; all participants see their own resulting hands.
type CatanTwoTrade struct {
	Mixed  bool   `json:"mixed,omitempty"`
	Resume string `json:"resume"`
	Drawn  []int  `json:"drawn"`
}

func (s *State) validateCatanTwoTokens() error {
	g, q := s.Catan, s.Catan.Two
	if g.twoFishing() {
		if len(q.Tokens) != 2 || q.Tokens[0] != 0 || q.Tokens[1] != 0 || q.Bank != 0 || q.TokensIssued != 0 || q.Spent || q.KnightExchanged || q.Trade != nil || s.Phase == "catan_two_trade" {
			return errors.New("双人渔夫不使用贸易筹码")
		}
		return nil
	}
	if q.TokensIssued < 0 || q.TokensIssued > catanTwoTokenLedgerLimit {
		return errors.New("双人贸易筹码记账无效")
	}
	supply := 20 + q.TokensIssued
	if len(q.Tokens) != 2 || q.Bank < 0 || q.Bank > supply || q.Tokens[0] < 0 || q.Tokens[0] > supply || q.Tokens[1] < 0 || q.Tokens[1] > supply || int64(q.Bank)+int64(q.Tokens[0])+int64(q.Tokens[1]) != int64(supply) {
		return errors.New("双人贸易筹码总量无效")
	}
	if g.setup() && (q.Spent || q.KnightExchanged || q.Trade != nil) {
		return errors.New("起始建设不能使用贸易筹码")
	}
	if trade := q.Trade; trade != nil {
		if s.Finished || s.Turn < 0 || s.Turn >= len(g.Players) || q.Pending != nil || !q.Spent || q.Sequence == 0 || s.Phase != "catan_two_trade" || g.Trade != nil || !g.cardBundle(trade.Drawn) || trade.Mixed && !g.twoKnights() || g.twoKnights() && !trade.Mixed && sum(trade.Drawn[5:]) != 0 || sum(trade.Drawn) < 1 || sum(trade.Drawn) > 2 || !catanHas(g.Players[s.Turn].Resources, trade.Drawn) || sum(g.Players[s.Turn].Resources) < 2 {
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
	return g.Two != nil && !g.twoFishing() && !s.Finished && player == s.Turn && player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated && !g.setup() && g.Two.Pending == nil && g.Two.Trade == nil && ((s.Phase == "catan_roll" && len(g.Two.Rolls) < 2) || (s.Phase == "catan_turn" && len(g.Two.Rolls) == 2))
}

func (g *Catan) twoTokenCost(player int) int {
	if g.Players[player].Score-g.hiddenVictoryPoints(player) > g.Players[1-player].Score-g.hiddenVictoryPoints(1-player) {
		return 2
	}
	return 1
}

// Site supplement: continue recording earned trade tokens after the physical
// supply is exhausted. Returned tokens are reused before issuing any more.
const catanTwoTokenLedgerLimit = 1_000_000_000

func (q *CatanTwo) tokenShortfall(count int) (int, error) {
	if count < 0 || count > catanTwoTokenLedgerLimit || q.TokensIssued < 0 || q.TokensIssued > catanTwoTokenLedgerLimit || q.Bank < 0 {
		return 0, errors.New("贸易筹码记账数量无效")
	}
	missing := max(0, count-q.Bank)
	if missing > catanTwoTokenLedgerLimit-q.TokensIssued {
		return 0, errors.New("贸易筹码记账超出安全范围")
	}
	return missing, nil
}

func (s *State) catanTwoEarn(player, count int) error {
	q := s.Catan.Two
	if player < 0 || player >= len(q.Tokens) {
		return errors.New("贸易筹码领取玩家无效")
	}
	if s.Catan.twoKnights() {
		if count < 0 {
			return errors.New("贸易筹码领取数量无效")
		}
		got := min(count, q.Bank)
		q.Bank -= got
		q.Tokens[player] += got
		if count > 0 {
			s.catanLog(player, "有限供应：领取 %d 枚贸易筹码（应领 %d 枚，缺额不记账）", got, count)
		}
		return nil
	}
	missing, err := q.tokenShortfall(count)
	if err != nil {
		return err
	}
	q.TokensIssued += missing
	q.Bank += missing - count
	q.Tokens[player] += count
	if count > 0 {
		s.catanLog(player, "获得 %d 枚贸易筹码", count)
	}
	return nil
}

func (s *State) catanTwoCanExchangeKnight(player int) bool {
	if !s.catanTwoTokenWindow(player) {
		return false
	}
	g, q := s.Catan, s.Catan.Two
	if g.twoKnights() {
		return !q.KnightExchanged && (len(g.twoKnightTokenVertices(player)) > 0 || len(g.twoAttackKnightTokenEdges(player)) > 0)
	}
	_, err := q.tokenShortfall(2)
	return !q.KnightExchanged && g.Players[player].Knights > 0 && err == nil
}

func (g *Catan) twoDesert() int {
	for _, tile := range g.Tiles {
		if tile.Resource == CatanDesert {
			return tile.ID
		}
	}
	return -1
}

func (g *Catan) twoRetreatTiles() []int {
	if g.Transport != nil || g.twoAttack() || g.twoAttackKnights() || g.twoKnights() && (g.CitiesKnights.Invasions == 0 || g.Robber < 0) {
		return []int{}
	}
	if g.Caravans != nil && !g.riversCaravans() || g.twoSeafarers() && g.twoDesert() < 0 {
		if g.Robber >= 0 {
			return []int{-1}
		}
		return []int{}
	}
	if g.Rivers != nil {
		return slices.DeleteFunc(slices.Clone(g.Rivers.Map.Swamps), func(id int) bool { return id == g.Robber })
	}
	if desert := g.twoDesert(); desert >= 0 && desert != g.Robber {
		return []int{desert}
	}
	return []int{}
}

func (g *Catan) twoTransportRetreatEdges() []int {
	out := []int{}
	if g.Two != nil && g.Transport != nil {
		for _, e := range g.Edges {
			if e.Owner == -1 && !slices.Contains(g.Transport.Barbarians[:], e.ID) {
				out = append(out, e.ID)
			}
		}
	}
	return out
}

func (s *State) catanTwoTokenAction(player int, a Action) error {
	if !s.catanTwoTokenWindow(player) {
		return errors.New("请在自己的掷骰前或行动阶段使用贸易筹码")
	}
	g, q := s.Catan, s.Catan.Two
	if len(a.Give)+len(a.Take)+len(a.Cards)+len(a.Tokens) != 0 {
		return errors.New("此行动不能预选资源牌")
	}
	if a.Type == "catan_two_knight" && g.twoKnights() {
		return s.catanTwoKnightForTokens(player, a)
	}
	if a.Choice != "" && !(g.twoKnights() && a.Type == "catan_two_trade" && (a.Choice == "resources" || a.Choice == "mixed")) {
		return errors.New("未知双人贸易选项")
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
	if g.twoKnights() && a.Type == "catan_two_trade" && a.Choice == "mixed" {
		cost *= 2
	}
	// 2025 transport p24 specifies one trade token for this replacement
	// action. The ordinary score-dependent cost still applies to forced trade.
	if a.Type == "catan_two_robber" && g.Transport != nil {
		cost = 1
	}
	if q.Spent || q.Tokens[player] < cost {
		return errors.New("本回合已使用贸易筹码，或筹码不足")
	}
	switch a.Type {
	case "catan_two_trade":
		hand := g.Players[1-player].Resources
		mixed := g.twoKnights() && a.Choice == "mixed"
		own := g.Players[player].Resources
		if g.twoKnights() && !mixed {
			hand = hand[:5]
			own = own[:5]
		}
		count := min(2, sum(hand))
		if count == 0 || sum(own)+count < 2 {
			return errors.New("对手必须有资源，且取牌后应能交还两张")
		}
		trade := &CatanTwoTrade{Resume: s.Phase, Drawn: make([]int, len(g.Bank)), Mixed: mixed}
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
		what := "资源"
		if mixed {
			what = "资源或商品"
		}
		s.catanLog(player, "花费 %d 枚贸易筹码，从玩家 %d 随机取 %d 张%s，待选择交还 2 张", cost, 2-player, count, what)
	case "catan_two_robber":
		if g.twoAttack() || g.twoAttackKnights() {
			if err := s.catanTwoMoveBarbarian(player, a, cost); err != nil {
				return err
			}
			break
		}
		if t := g.Transport; t != nil {
			if a.Card < 0 || a.Card >= len(t.Barbarians) || !slices.Contains(g.twoTransportRetreatEdges(), a.Edge) {
				return errors.New("请选择一名蛮族及没有道路、没有其他蛮族的边")
			}
			t.Barbarians[a.Card] = a.Edge
			s.catanLog(player, "花费1枚贸易筹码，将蛮族%d移到空路 #%d，不偷牌", a.Card+1, a.Edge+1)
			break
		}
		target, terrain := g.twoDesert(), "沙漠"
		if g.Rivers != nil {
			target, terrain = a.Tile, "沼泽"
		}
		if g.Caravans != nil && !g.riversCaravans() || g.twoSeafarers() && g.twoDesert() < 0 {
			target, terrain = -1, "棋盘外"
		}
		if !slices.Contains(g.twoRetreatTiles(), target) {
			return errors.New("请选择另一处允许强盗退回的地形")
		}
		g.Robber = target
		if target < 0 {
			s.catanLog(player, "花费 %d 枚贸易筹码，将强盗移到棋盘外，不偷牌", cost)
		} else {
			s.catanLog(player, "花费 %d 枚贸易筹码，将强盗移回%s #%d，不偷牌", cost, terrain, target+1)
		}
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
	if q == nil || q.Trade == nil || s.Phase != "catan_two_trade" || player != s.Turn || a.Type != "catan_two_return" || !g.cardBundle(a.Give) || g.twoKnights() && !q.Trade.Mixed && sum(a.Give[5:]) != 0 || sum(a.Give) != 2 || !catanHas(g.Players[player].Resources, a.Give) {
		return errors.New("请由取牌者选择两张资源交还对手")
	}
	for color, count := range a.Give {
		g.Players[player].Resources[color] -= count
		g.Players[1-player].Resources[color] += count
	}
	what := "资源"
	if q.Trade.Mixed {
		what = "资源或商品"
	}
	s.Phase = q.Trade.Resume
	q.Trade = nil
	s.catanLog(player, "向玩家 %d 交还 2 张%s，完成强制交易", 2-player, what)
	return nil
}

func (s *State) catanTwoReturnBot(player int) (Action, error) {
	g := s.Catan
	if g.Two == nil || g.Two.Trade == nil || player != s.Turn {
		return Action{}, errors.New("inactive forced trade")
	}
	hand := append([]int{}, g.Players[player].Resources...)
	give := make([]int, len(g.Bank))
	if g.twoKnights() && !g.Two.Trade.Mixed {
		hand = hand[:5]
	}
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
	if !g.twoKnights() && s.catanTwoCanExchangeKnight(player) && q.Tokens[player] < 2 && g.ArmyOwner != player {
		return Action{Type: "catan_two_knight"}, true
	}
	if g.twoAttackKnights() && s.catanTwoCanExchangeKnight(player) && q.Tokens[player] < g.twoTokenCost(player) {
		for _, edge := range g.twoAttackKnightTokenEdges(player) {
			k := g.Attack.City.Knights[g.Attack.City.at(edge)]
			if !k.Active && g.Attack.castleEdge(g, edge) {
				return Action{Type: "catan_two_knight", Edge: edge}, true
			}
		}
	}
	if g.twoKnights() && !g.twoAttackKnights() && s.catanTwoCanExchangeKnight(player) && q.Tokens[player] < g.twoTokenCost(player) {
		_, _, cities := g.pieces(player)
		strength := 0
		for _, n := range g.CitiesKnights.Knights {
			if n.Owner == player {
				strength += n.Strength
			}
		}
		for _, vertex := range g.twoKnightTokenVertices(player) {
			n := g.knightAt(vertex)
			if !n.Active && strength-n.Strength >= cities {
				return Action{Type: "catan_two_knight", Vertex: vertex}, true
			}
		}
	}
	if q.Spent || q.Tokens[player] < 1 {
		return Action{}, false
	}
	if t := g.Transport; t != nil {
		// Remove an immediate wagon obstruction before moving. Use only
		// public piece positions; do not inspect cargo/development stacks.
		position := t.Wagons[player].Position
		for piece, edge := range t.Barbarians {
			e := g.Edges[edge]
			if e.A != position && e.B != position {
				continue
			}
			for _, target := range g.twoTransportRetreatEdges() {
				d := g.Edges[target]
				if d.A != position && d.B != position {
					return Action{Type: "catan_two_robber", Card: piece, Edge: target}, true
				}
			}
		}
	}
	if q.Tokens[player] < g.twoTokenCost(player) {
		return Action{}, false
	}
	if g.twoAttack() || g.twoAttackKnights() {
		best := Action{}
		score := 0
		moves := g.twoAttackMoves()
		for _, from := range g.Attack.Map.Coast {
			for _, to := range moves[from] {
				value := g.attackTileInterest(player, from)*g.Attack.Barbarians[from] - g.attackTileInterest(player, to)*(g.Attack.Barbarians[to]+1)
				if value > score {
					score = value
					best = Action{Type: "catan_two_robber", Card: from, Tile: to}
				}
			}
		}
		if score > 0 {
			return best, true
		}
	}
	if targets := g.twoRetreatTiles(); g.Robber >= 0 && len(targets) > 0 && g.Tiles[g.Robber].Resource < 5 {
		for _, vertex := range g.Tiles[g.Robber].Vertices {
			if g.Vertices[vertex].Owner == player && g.Vertices[vertex].Level > 0 {
				return Action{Type: "catan_two_robber", Tile: targets[0]}, true
			}
		}
	}
	if sum(g.Players[1-player].Resources) >= 2 {
		for _, count := range g.Players[player].Resources {
			if count >= 3 {
				if g.twoKnights() {
					if q.Tokens[player] >= 2*g.twoTokenCost(player) {
						return Action{Type: "catan_two_trade", Choice: "mixed"}, true
					}
					continue
				}
				return Action{Type: "catan_two_trade"}, true
			}
		}
	}
	return Action{}, false
}
