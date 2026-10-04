package game

import (
	"errors"
	"fmt"
	"slices"
	"sort"
)

const (
	GemOrientGold        = "gold"
	GemOrientCopy        = "copy"
	GemOrientCopyCascade = "copy_cascade"
	GemOrientDouble      = "double"
	GemOrientCascade     = "cascade"
	GemOrientSacrifice   = "sacrifice"
)

func (c Card) gemDeck() int {
	if c.Orient != "" {
		return c.Tier + 2
	}
	return c.Tier - 1
}
func (c Card) gemBonus() int {
	if c.Color < 0 || c.Color >= 5 {
		return 0
	}
	if c.Orient == GemOrientDouble {
		return 2
	}
	if c.BonusCount > 0 {
		return c.BonusCount
	}
	return 1
}
func (c Card) gemCopy() bool { return c.Orient == GemOrientCopy || c.Orient == GemOrientCopyCascade }
func gemCanAcquire(p GemPlayer, c Card) bool {
	if !c.gemCopy() {
		return true
	}
	for _, owned := range p.Cards {
		if owned.ID != c.ID && owned.gemBonus() > 0 {
			return true
		}
	}
	return false
}

// Cards lists either colorless virtual-gold cards, or the two physical cards
// paid for a sacrifice card. Tokens always contains physical tokens only.
func gemOrientPayment(p GemPlayer, c Card, a Action) ([]int, []int, error) {
	if !gemCanAcquire(p, c) {
		return nil, nil, errors.New("必须先拥有一张带永久奖励的发展卡，才能取得复制卡")
	}
	if c.Orient == GemOrientSacrifice {
		if slices.ContainsFunc(a.Tokens, func(n int) bool { return n != 0 }) {
			return nil, nil, errors.New("弃置购牌不能支付筹码")
		}
		if c.SacrificeColor < 0 || c.SacrificeColor >= 5 {
			return nil, nil, errors.New("这张东方卡需要弃置两张指定颜色的发展卡，不支付筹码")
		}
		ids := append([]int{}, a.Cards...)
		candidates := []Card{}
		for _, owned := range p.Cards {
			if owned.Color == c.SacrificeColor {
				candidates = append(candidates, owned)
			}
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].gemCopy() != candidates[j].gemCopy() {
				return candidates[i].gemCopy()
			}
			if candidates[i].Points != candidates[j].Points {
				return candidates[i].Points < candidates[j].Points
			}
			return candidates[i].gemBonus() < candidates[j].gemBonus()
		})
		if len(ids) == 0 && len(candidates) >= 2 {
			ids = []int{candidates[0].ID, candidates[1].ID}
		}
		if len(ids) != 2 || ids[0] == ids[1] {
			return nil, nil, errors.New("请选择两张不同的指定颜色发展卡")
		}
		copies, chosenCopies := 0, 0
		for _, owned := range candidates {
			if owned.gemCopy() {
				copies++
			}
		}
		for _, id := range ids {
			found := false
			for _, owned := range candidates {
				if owned.ID == id {
					found = true
					if owned.gemCopy() {
						chosenCopies++
					}
				}
			}
			if !found {
				return nil, nil, errors.New("弃置的发展卡颜色或所有权不正确")
			}
		}
		if chosenCopies != min(2, copies) {
			return nil, nil, errors.New("必须优先弃置视为该颜色的复制卡")
		}
		return make([]int, 6), ids, nil
	}
	if len(a.Cards) == 0 {
		pay, err := gemPayment(p, c, a.Tokens)
		return pay, nil, err
	}
	seen := map[int]bool{}
	for _, id := range a.Cards {
		if seen[id] {
			return nil, nil, errors.New("不能重复使用同一张黄金卡")
		}
		seen[id] = true
		index := slices.IndexFunc(p.Cards, func(owned Card) bool { return owned.ID == id && owned.Orient == GemOrientGold })
		if index < 0 {
			return nil, nil, errors.New("只能弃置自己持有的东方黄金卡")
		}
	}
	value := 1
	if p.hasPost(GemPostDoubleGold) {
		value = 2
	}
	pay := make([]int, 6)
	if len(a.Tokens) > 0 {
		if len(a.Tokens) != 6 {
			return nil, nil, errors.New("支付数量必须包含六种筹码")
		}
		copy(pay, a.Tokens)
	}
	required := 0
	for color, cost := range c.Cost {
		need := max(0, cost-p.Bonus[color])
		if len(a.Tokens) == 0 {
			pay[color] = min(need, p.Tokens[color])
		}
		if pay[color] < 0 || pay[color] > need || pay[color] > p.Tokens[color] {
			return nil, nil, errors.New("所选筹码支付数量不正确")
		}
		required += (need - pay[color] + value - 1) / value
	}
	if len(a.Tokens) == 0 {
		pay[5] = max(0, required-2*len(a.Cards))
	}
	virtualUsed := required - pay[5]
	if pay[5] < 0 || pay[5] > p.Tokens[5] || virtualUsed < len(a.Cards) || virtualUsed > 2*len(a.Cards) {
		return nil, nil, errors.New("每张黄金卡提供两枚虚拟黄金，至少使用一枚；每枚只能抵同一种颜色")
	}
	return pay, append([]int{}, a.Cards...), nil
}

func gemBestPurchase(p GemPlayer, c Card) (Action, bool) {
	a := Action{Type: "buy", Card: c.ID}
	if _, _, err := gemOrientPayment(p, c, a); err == nil {
		return a, true
	}
	if c.Orient == GemOrientSacrifice || !gemCanAcquire(p, c) {
		return a, false
	}
	for _, owned := range p.Cards {
		if owned.Orient != GemOrientGold {
			continue
		}
		a.Cards = append(a.Cards, owned.ID)
		if _, _, err := gemOrientPayment(p, c, a); err == nil {
			return a, true
		}
	}
	return a, false
}

func (s *State) gemDiscardCards(ids []int) {
	g := s.Splendor
	p := &g.Players[s.Turn]
	for _, id := range ids {
		index := slices.IndexFunc(p.Cards, func(c Card) bool { return c.ID == id })
		c := p.Cards[index]
		if c.gemBonus() > 0 {
			p.Bonus[c.Color] -= c.gemBonus()
		}
		p.Score -= c.Points
		g.Exiled = append(g.Exiled, c)
		p.Cards = append(p.Cards[:index], p.Cards[index+1:]...)
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 弃置 %s，移出本局", s.Turn+1, splendorCardSummary(c)))
	}
}

func (s *State) gemAcquireCard(c Card) []GemEffect {
	p := &s.Splendor.Players[s.Turn]
	p.Cards = append(p.Cards, c)
	p.Score += c.Points
	if c.gemBonus() > 0 {
		p.Bonus[c.Color] += c.gemBonus()
	}
	switch c.Orient {
	case GemOrientCopy:
		return []GemEffect{{Kind: "copy", Card: c.ID}}
	case GemOrientCopyCascade:
		return []GemEffect{{Kind: "copy", Card: c.ID}, {Kind: "free_card", Tier: 1}}
	case GemOrientCascade:
		return []GemEffect{{Kind: "free_card", Tier: 2}}
	}
	return nil
}

func (s *State) gemFreeCards(tier int) []Card {
	g := s.Splendor
	p := g.Players[s.Turn]
	result := []Card{}
	for _, row := range g.Market {
		for _, c := range row {
			if c.ID > 0 && c.Tier == tier && g.gemCardAccessible(c.ID, s.Turn) && gemCanAcquire(p, c) {
				result = append(result, c)
			}
		}
	}
	return result
}

func (s *State) gemOrientAction(a Action) error {
	g := s.Splendor
	p := &g.Players[s.Turn]
	if len(g.Effects) == 0 {
		return errors.New("没有待结算的东方效果")
	}
	e := g.Effects[0]
	if e.Kind == "copy" && a.Type == "gem_copy" {
		index := slices.IndexFunc(p.Cards, func(c Card) bool { return c.ID == e.Card })
		source := slices.IndexFunc(p.Cards, func(c Card) bool { return c.ID == a.Card && c.ID != e.Card && c.gemBonus() > 0 })
		if index < 0 || source < 0 {
			return errors.New("请选择自己已拥有的一张永久奖励卡")
		}
		card := &p.Cards[index]
		card.Color = p.Cards[source].Color
		card.BonusCount = p.Cards[source].gemBonus()
		p.Bonus[card.Color] += card.BonusCount
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 将复制卡 #%d 与发展卡 #%d 配对，永久%s +%d", s.Turn+1, e.Card, a.Card, splendorGemNames[card.Color], card.BonusCount))
		g.Effects = g.Effects[1:]
	} else if e.Kind == "free_card" && a.Type == "gem_free_card" {
		candidates := s.gemFreeCards(e.Tier)
		index := slices.IndexFunc(candidates, func(c Card) bool { return c.ID == a.Card })
		if index < 0 {
			return errors.New("请选择对应等级的可取得公开卡，对手的要塞会阻止取得")
		}
		card := candidates[index]
		for row, cards := range g.Market {
			for slot, c := range cards {
				if c.ID == card.ID {
					g.Market[row][slot] = Card{Tier: c.Tier, Orient: c.Orient}
					g.Refills = append(g.Refills, GemRefill{Tier: row, Slot: slot})
					s.recordSplendorCard(SplendorCardEvent{Action: "free", Source: "market", Tier: c.Tier, Orient: c.Orient != "", Slot: slot, Card: &card})
				}
			}
		}
		delete(g.Strongholds, card.ID)
		effects := s.gemAcquireCard(card)
		g.Effects = append(effects, g.Effects[1:]...)
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 通过东方效果免费取得 %s（不触发购牌能力）", s.Turn+1, splendorCardSummary(card)))
	} else {
		return errors.New("请先完成当前东方卡的选择")
	}
	s.gemContinueEffects()
	return nil
}
