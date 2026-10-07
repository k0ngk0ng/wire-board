package game

import (
	"errors"
	"slices"
)

// The drawn card is public and is held here only while resolving its mandatory
// effect. It never enters a player's development hand or ordinary discard.
type catanAttackCardPending struct {
	ID     int    `json:"id"`
	Player int    `json:"player"`
	Card   string `json:"card"`
}

var catanAttackCardNames = map[string]string{
	"capture": "俘获", "knighthood": "授勋", "swift_knight": "迅捷骑士", "treason": "叛变",
}

func (a catanAttack) captureTargets() []int {
	out := []int{}
	for _, id := range a.Map.Coast {
		if a.Barbarians[id] > 0 {
			out = append(out, id)
		}
	}
	return out
}

func (a catanAttack) recruitEdges(g *Catan, player int, card string) []int {
	out := []int{}
	used := map[int]bool{}
	own := 0
	for _, knight := range a.Knights {
		used[knight.Edge] = true
		if knight.Player == player {
			own++
		}
	}
	if own >= 6 {
		return out
	}
	for _, edge := range g.Edges {
		if !used[edge.ID] && (card == "swift_knight" || card == "knighthood" && a.castleEdge(g, edge.ID)) {
			out = append(out, edge.ID)
		}
	}
	return out
}

type catanAttackTreasonPlan struct {
	Sources      []int `json:"sources"`
	Destinations []int `json:"destinations"`
}

// Use the largest feasible effect, preserving distinct board sources before
// taking missing pieces from supply. Every advertised source set can finish.
func (a catanAttack) treasonPlans() []catanAttackTreasonPlan {
	board := a.captureTargets()
	for count := 2; count >= 1; count-- {
		fromBoard := min(count, len(board))
		if count-fromBoard > a.supply() {
			continue
		}
		sources := [][]int{}
		switch fromBoard {
		case 0:
			sources = append(sources, []int{})
		case 1:
			for _, id := range board {
				sources = append(sources, []int{id})
			}
		case 2:
			for i, one := range board {
				for _, two := range board[i+1:] {
					sources = append(sources, []int{one, two})
				}
			}
		}
		plans := []catanAttackTreasonPlan{}
		for _, from := range sources {
			targets := a.treasonDestinations(from)
			if len(targets) < count {
				continue
			}
			for len(from) < count {
				from = append(from, -1)
			}
			plans = append(plans, catanAttackTreasonPlan{from, targets})
		}
		if len(plans) > 0 {
			return plans
		}
	}
	return []catanAttackTreasonPlan{{Sources: []int{}, Destinations: []int{}}}
}

func (a catanAttack) treasonDestinations(from []int) []int {
	out := []int{}
	for _, id := range a.Map.Coast {
		if !a.conquered(id) && !slices.Contains(from, id) {
			out = append(out, id)
		}
	}
	return out
}

// A real coin shortage is resolved by the site's ledger rule. Only corrupt
// integer bounds can prevent payment; never inspect the next hidden card.
func (a catanAttack) cardSupplyReady() bool {
	_, err := catanGoldShortfall(a.GoldBank, a.GoldIssued, 2)
	return err == nil
}

func (s *State) catanAttackBuyCard(player int, action Action) error {
	g, a := s.Catan, s.Catan.Attack
	if a == nil || s.Phase != "catan_turn" || player != s.Turn || a.Pending != nil || action.Skill != "" || action.Choice != "" {
		return errors.New("只能在自己的行动阶段购买蛮族进攻发展卡")
	}
	cost := catanPrices["catan_buy_dev"]
	if !catanHas(g.Players[player].Resources, cost) {
		return errors.New("资源不足：发展卡需要羊毛、粮食、矿石各1张")
	}
	if !a.cardSupplyReady() {
		return errors.New("金币记账超出安全范围，不能购买发展卡")
	}
	catanMove(g.Players[player].Resources, g.Bank, cost)
	g.Trade = nil
	a.CardSequence++
	s.catanLog(player, "支付 羊毛×1、粮食×1、矿石×1，购买并立即使用发展卡")
	for {
		if len(a.Deck) == 0 {
			a.Deck, a.Discard = a.Discard, []string{}
			shuffle(a.Deck)
			s.Log = append(s.Log, "蛮族进攻发展卡用完，洗混弃牌形成新牌堆")
		}
		card := a.Deck[len(a.Deck)-1]
		a.Deck = a.Deck[:len(a.Deck)-1]
		a.Pending = &catanAttackCardPending{ID: a.CardSequence, Player: player, Card: card}
		s.Phase = "catan_attack_card"
		s.catanLog(player, "翻开发展卡：%s", catanAttackCardNames[card])
		if card == "capture" && len(a.captureTargets()) == 0 {
			a.Discard = append(a.Discard, card)
			a.Pending = nil
			s.catanLog(player, "沿海没有蛮族，弃置俘获并免费重抽")
			continue
		}
		if card == "treason" && len(a.treasonPlans()[0].Sources) == 0 {
			if err := s.catanAttackTreason(player, nil, nil); err != nil {
				return err
			}
			s.catanAttackFinishCard()
			return nil
		}
		if (card == "knighthood" || card == "swift_knight") && len(a.recruitEdges(g, player, card)) == 0 {
			s.catanLog(player, "%s：没有可放置的骑士或空位，不能增加棋子，弃置此牌", catanAttackCardNames[card])
			s.catanAttackFinishCard()
		}
		return nil
	}
}

func (s *State) catanAttackFinishCard() {
	a := s.Catan.Attack
	a.Discard = append(a.Discard, a.Pending.Card)
	a.Pending = nil
	s.Phase = "catan_turn"
	s.catanScores()
	s.catanVictory()
}

func (s *State) catanAttackCardChoice(player int, action Action) error {
	g, a := s.Catan, s.Catan.Attack
	if a == nil || a.Pending == nil || s.Phase != "catan_attack_card" || player != s.Turn || player != a.Pending.Player || action.Type != "catan_attack_card" || action.Prompt != a.Pending.ID || action.Choice != a.Pending.Card || action.Skill != "" {
		return errors.New("请由当前玩家完成这张蛮族进攻发展卡")
	}
	switch a.Pending.Card {
	case "capture":
		if !slices.Contains(a.captureTargets(), action.Tile) {
			return errors.New("请选择有蛮族的沿海地块")
		}
		liberates := a.conquered(action.Tile)
		a.Barbarians[action.Tile]--
		a.Prisoners[player]++
		s.catanLog(player, "俘获地块 #%d 的1个蛮族，现有俘虏%d个（%d分）", action.Tile+1, a.Prisoners[player], a.Prisoners[player]/2)
		if liberates {
			s.catanLog(player, "解放地块 #%d，恢复生产及相邻被征服建筑", action.Tile+1)
		}
	case "knighthood", "swift_knight":
		if !slices.Contains(a.recruitEdges(g, player, a.Pending.Card), action.Edge) {
			return errors.New("请选择可放置骑士的空边，授勋只能放在城堡边")
		}
		a.Knights = append(a.Knights, catanAttackKnight{Player: player, Edge: action.Edge})
		s.catanLog(player, "%s：在路线 #%d 放置骑士，行动阶段结束后才能移动", catanAttackCardNames[a.Pending.Card], action.Edge+1)
	case "treason":
		if err := s.catanAttackTreason(player, action.Give, action.Take); err != nil {
			return err
		}
	default:
		return errors.New("未知蛮族进攻发展卡")
	}
	s.catanAttackFinishCard()
	return nil
}

func (s *State) catanAttackTreason(player int, from, to []int) error {
	a := s.Catan.Attack
	plans := a.treasonPlans()
	count := len(plans[0].Sources)
	if len(from) != count || len(to) != count {
		return errors.New("请完成当前可执行的全部叛变移动")
	}
	valid := false
	for _, plan := range plans {
		// Compare sorted copies: source order only pairs the movement log.
		left, right := slices.Clone(from), slices.Clone(plan.Sources)
		slices.Sort(left)
		slices.Sort(right)
		if !slices.Equal(left, right) {
			continue
		}
		valid = true
		for i, id := range to {
			if !slices.Contains(plan.Destinations, id) || slices.Contains(to[:i], id) {
				valid = false
				break
			}
		}
		if valid {
			break
		}
	}
	if !valid {
		return errors.New("请选择能完成叛变的不同来源和未征服目的地，优先使用棋盘上的蛮族")
	}

	if err := a.ensureGold(2); err != nil {
		return err
	}
	a.GoldBank -= 2
	a.Gold[player] += 2
	s.catanLog(player, "叛变：领取 金币×2")
	if count < 2 {
		s.catanLog(player, "本站补充规则：叛变当前最多可移动%d个蛮族，完成可执行部分，不额外抽牌", count)
	}
	for _, id := range from {
		if id >= 0 {
			a.Barbarians[id]--
			if a.Barbarians[id] == 2 {
				s.catanLog(player, "叛变解放地块 #%d，恢复生产及相邻被征服建筑", id+1)
			}
		}
	}
	for i, id := range to {
		a.Barbarians[id]++
		if from[i] == -1 {
			s.catanLog(player, "叛变：从供应向地块 #%d 放置1个蛮族（%d/3）", id+1, a.Barbarians[id])
		} else {
			s.catanLog(player, "叛变：从地块 #%d 向地块 #%d 移动1个蛮族（%d/3）", from[i]+1, id+1, a.Barbarians[id])
		}
		if a.conquered(id) {
			s.catanLog(player, "地块 #%d 被征服，停止生产并禁止新建", id+1)
		}
	}
	return nil
}

// Only the public map, components and ownership enter these evaluations.
// Neither the unrevealed deck nor opponents' resource types may affect a choice.
func (g *Catan) attackTileInterest(player, tile int) int {
	value := 0
	for _, id := range g.Tiles[tile].Vertices {
		v := g.Vertices[id]
		if v.Level == 0 || v.Owner < 0 || g.Players[v.Owner].Eliminated {
			continue
		}
		weight := v.Level * max(1, g.tileNumberWeight(g.Tiles[tile]))
		if v.Owner == player {
			value += 4 * weight
		} else {
			value -= weight
		}
	}
	return value
}

func (s *State) catanAttackCardBot(player int) (Action, error) {
	g := s.Catan
	if g.Attack == nil || g.Attack.Pending == nil || g.Attack.Pending.Player != player || s.Phase != "catan_attack_card" {
		return Action{}, errors.New("inactive barbarian attack card seat")
	}
	a, q := g.Attack, g.Attack.Pending
	best := Action{Type: "catan_attack_card", Prompt: q.ID, Choice: q.Card}
	found, score := false, -1000000
	switch q.Card {
	case "capture":
		for _, id := range a.captureTargets() {
			value := g.attackTileInterest(player, id) * a.Barbarians[id]
			if !found || value > score {
				found, score, best.Tile = true, value, id
			}
		}
	case "knighthood", "swift_knight":
		for _, edge := range a.recruitEdges(g, player, q.Card) {
			value := 0
			for _, id := range g.Edges[edge].Tiles {
				value += 10*a.Barbarians[id] + g.attackTileInterest(player, id)
			}
			if !found || value > score {
				found, score, best.Edge = true, value, edge
			}
		}
	case "treason":
		for _, plan := range a.treasonPlans() {
			groups := [][]int{}
			switch len(plan.Sources) {
			case 0:
				groups = append(groups, []int{})
			case 1:
				for _, id := range plan.Destinations {
					groups = append(groups, []int{id})
				}
			case 2:
				for i, one := range plan.Destinations {
					for _, two := range plan.Destinations[i+1:] {
						groups = append(groups, []int{one, two})
					}
				}
			}
			for _, to := range groups {
				value := 0
				for _, id := range plan.Sources {
					if id >= 0 {
						value += g.attackTileInterest(player, id) * a.Barbarians[id]
					}
				}
				for _, id := range to {
					value -= g.attackTileInterest(player, id) * (a.Barbarians[id] + 1)
				}
				if !found || value > score {
					found, score = true, value
					best.Give, best.Take = slices.Clone(plan.Sources), slices.Clone(to)
				}
			}
		}
	}
	if !found {
		return Action{}, errors.New("no legal barbarian attack card choice")
	}
	return best, nil
}
