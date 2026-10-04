package game

import (
	"errors"
	"fmt"
	"slices"
)

// Rule identity is persisted with a match: the 2025 split-box releases change
// several powers from Cities of Splendor (2017).
const SplendorExpansionRules = "split-box-2025"

type SplendorOptions struct {
	Orient       bool   `json:"orient,omitempty"`
	Cities       bool   `json:"cities,omitempty"`
	Rules        string `json:"rules,omitempty"`
	TradingPosts bool   `json:"tradingPosts,omitempty"`
	Strongholds  bool   `json:"strongholds,omitempty"`
}

func (o SplendorOptions) expanded() bool {
	return o.TradingPosts || o.Strongholds || o.Orient || o.Cities
}

func (o SplendorOptions) Validate() error {
	// Enable only after the complete 2025 catalogs and presentation are verified.
	if o.Orient || o.Cities {
		return errors.New("东方与城市扩展仍在核对完整卡牌数据，暂未开放")
	}
	if o.Rules != "" && o.Rules != SplendorExpansionRules {
		return errors.New("不支持的璀璨宝石扩展规则版本")
	}
	return nil
}

func NormalizeSplendorOptions(o SplendorOptions) (SplendorOptions, error) {
	if err := o.Validate(); err != nil {
		return o, err
	}
	if o.expanded() {
		o.Rules = SplendorExpansionRules
	} else {
		o.Rules = ""
	}
	return o, nil
}

func NewSplendor(n int, options SplendorOptions) (*State, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	s, err := New("splendor", n)
	if err != nil {
		return nil, err
	}
	if options.expanded() {
		options.Rules = SplendorExpansionRules
	}
	s.Splendor.Options = options
	if options.Strongholds {
		s.Splendor.Strongholds = map[int]GemStronghold{}
	}
	return s, nil
}

const (
	GemPostPurchaseToken = iota + 1
	GemPostBlindReserve
	GemPostExtraToken
	GemPostDoubleGold
	GemPostPrestige
)

type GemTradingPost struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Cost        [5]int `json:"cost"`
}

func GemTradingPosts() []GemTradingPost {
	return []GemTradingPost{
		{GemPostPurchaseToken, "购牌馈赠", "每次购买发展卡后、补牌前，拿取一枚非黄金宝石。", [5]int{0, 1, 0, 0, 3}},
		{GemPostBlindReserve, "择优预留", "盲预留时查看牌堆顶部两张，保留一张，另一张放回牌堆底部。", [5]int{0, 0, 0, 3, 0}},
		{GemPostExtraToken, "额外宝石", "拿取两枚同色宝石后，再拿一枚其他颜色的非黄金宝石。", [5]int{0, 2, 0, 0, 0}},
		{GemPostDoubleGold, "黄金增值", "每枚黄金支付同一种颜色的两枚宝石；多付不找零。", [5]int{0, 0, 3, 1, 0}},
		{GemPostPrestige, "贸易声望", "每个已获得的贸易站提供一分声望，包含本贸易站。", [5]int{5, 0, 0, 0, 0}},
	}
}

func (p GemPlayer) hasPost(id int) bool { return slices.Contains(p.TradingPosts, id) }

// Requirements count physical cards, not discount bonuses. In particular an
// Orient double-bonus card remains one card for cities, nobles and posts.
func (p GemPlayer) gemCardCounts() [5]int {
	var counts [5]int
	for _, c := range p.Cards {
		if c.Color >= 0 && c.Color < 5 {
			counts[c.Color]++
		}
	}
	return counts
}

func (s *State) gemPostOptions() []int {
	p := s.Splendor.Players[s.Turn]
	counts := p.gemCardCounts()
	result := []int{}
	if !s.Splendor.Options.TradingPosts {
		return result
	}
	for _, post := range GemTradingPosts() {
		if p.hasPost(post.ID) {
			continue
		}
		ok := true
		for color, n := range post.Cost {
			if counts[color] < n {
				ok = false
			}
		}
		if ok {
			result = append(result, post.ID)
		}
	}
	return result
}

func (s *State) gemGainPost(id int) {
	p := &s.Splendor.Players[s.Turn]
	p.TradingPosts = append(p.TradingPosts, id)
	if id == GemPostPrestige {
		p.Score += len(p.TradingPosts)
	} else if p.hasPost(GemPostPrestige) {
		p.Score++
	}
	s.Log = append(s.Log, fmt.Sprintf("玩家 %d 获得贸易站「%s」，当前 %d 分", s.Turn+1, GemTradingPosts()[id-1].Name, p.Score))
}

func (s *State) gemTradingPostCheck() {
	options := s.gemPostOptions()
	if len(options) > 1 {
		s.Phase = "gem_post"
		return
	}
	if len(options) == 1 {
		s.gemGainPost(options[0])
	}
	s.gemNext()
}

// Each gold unit must be assigned to a single color. Thus deficits [1,1]
// require two gold even with the double-gold power, while [3,0] require two.
func gemPayment(p GemPlayer, c Card, selected []int) ([]int, error) {
	value := 1
	if p.hasPost(GemPostDoubleGold) {
		value = 2
	}
	pay := make([]int, 6)
	if len(selected) > 0 {
		if len(selected) != 6 {
			return nil, errors.New("请选择六种宝石的支付数量")
		}
		copy(pay, selected)
	}
	gold := 0
	for color, cost := range c.Cost {
		need := max(0, cost-p.Bonus[color])
		if len(selected) == 0 {
			pay[color] = min(need, p.Tokens[color])
		}
		if pay[color] < 0 || pay[color] > need || pay[color] > p.Tokens[color] {
			return nil, errors.New("支付数量不正确，不能多付或使用未持有的宝石")
		}
		gold += (need - pay[color] + value - 1) / value
	}
	if len(selected) == 0 {
		pay[5] = gold
	}
	if pay[5] != gold {
		return nil, errors.New("黄金必须恰好替代未支付的颜色宝石（增值时每枚用于同一颜色）")
	}
	if gold > p.Tokens[5] {
		return nil, errors.New("宝石不足以购买这张卡牌")
	}
	return pay, nil
}

// Effects and empty market slots are saved explicitly, so reconnect/restart
// cannot skip a choice or reveal replacement cards before a required choice.
type GemEffect struct {
	Card    int    `json:"card,omitempty"`
	Tier    int    `json:"tier,omitempty"`
	Kind    string `json:"kind"`
	Exclude int    `json:"exclude"`
}

type GemRefill struct {
	Tier int `json:"tier"`
	Slot int `json:"slot"`
}

type GemStronghold struct {
	Player int `json:"player"`
	Count  int `json:"count"`
}

func (g *Splendor) gemCardAccessible(card, player int) bool {
	h, exists := g.Strongholds[card]
	return !exists || h.Player == player
}

func (g *Splendor) gemMarketCard(id int) (Card, bool) {
	if id <= 0 {
		return Card{}, false
	}
	for _, row := range g.Market {
		for _, c := range row {
			if c.ID == id {
				return c, true
			}
		}
	}
	return Card{}, false
}

func (g *Splendor) gemPlacedStrongholds(player int) int {
	n := 0
	for _, h := range g.Strongholds {
		if h.Player == player {
			n += h.Count
		}
	}
	return n
}

func (s *State) gemConquestCard() int {
	g := s.Splendor
	if !g.Options.Strongholds || g.ConquestUsed {
		return 0
	}
	for id, h := range g.Strongholds {
		if h.Player != s.Turn || h.Count != 3 {
			continue
		}
		if c, ok := g.gemMarketCard(id); ok {
			if _, ok := gemBestPurchase(g.Players[s.Turn], c); ok {
				return id
			}
		}
	}
	return 0
}

func (s *State) gemRefillMarket() {
	g := s.Splendor
	for _, refill := range g.Refills {
		if len(g.Decks[refill.Tier]) > 0 {
			g.Market[refill.Tier][refill.Slot] = g.Decks[refill.Tier][0]
			g.Decks[refill.Tier] = g.Decks[refill.Tier][1:]
		}
	}
	if !g.Options.Orient {
		for i, row := range g.Market {
			g.Market[i] = slices.DeleteFunc(row, func(c Card) bool { return c.ID == 0 })
		}
	}
	g.Refills = nil
}

func (s *State) gemContinueEffects() {
	g := s.Splendor
	for len(g.Effects) > 0 {
		e := g.Effects[0]
		switch e.Kind {
		case "copy":
			s.Phase = "gem_copy"
			return
		case "free_card":
			if len(s.gemFreeCards(e.Tier)) > 0 {
				s.Phase = "gem_free_card"
				return
			}
		case "token":
			for i := 0; i < 5; i++ {
				if i != e.Exclude && g.Bank[i] > 0 {
					s.Phase = "gem_token"
					return
				}
			}
		case "stronghold":
			if len(s.gemStrongholdActions()) > 0 {
				s.Phase = "gem_stronghold"
				return
			}
		}
		g.Effects = g.Effects[1:]
	}
	if s.gemConquestCard() > 0 {
		s.Phase = "gem_conquest"
		return
	}
	s.gemRefillMarket()
	if sum(g.Players[s.Turn].Tokens) > 10 {
		s.Phase = "discard"
	} else {
		s.gemAfter()
	}
}

func (s *State) gemStrongholdActions() []Action {
	g := s.Splendor
	result := []Action{}
	for _, row := range g.Market {
		for _, c := range row {
			if c.ID == 0 {
				continue
			}
			h, exists := g.Strongholds[c.ID]
			if exists && h.Player != s.Turn {
				if h.Count == 1 {
					result = append(result, Action{Type: "gem_stronghold", Choice: "remove", Card: c.ID})
				}
				continue
			}
			if h.Count >= 3 {
				continue
			}
			if g.gemPlacedStrongholds(s.Turn) < 3 {
				result = append(result, Action{Type: "gem_stronghold", Choice: "place", Card: c.ID})
			}
			for _, sourceRow := range g.Market {
				for _, source := range sourceRow {
					if from, ok := g.Strongholds[source.ID]; ok && from.Player == s.Turn && source.ID != c.ID {
						result = append(result, Action{Type: "gem_stronghold", Choice: "place", Card: c.ID, Target: source.ID})
					}
				}
			}
		}
	}
	return result
}

func (s *State) gemExpansionAction(a Action) error {
	g := s.Splendor
	p := &g.Players[s.Turn]
	switch s.Phase {
	case "gem_copy", "gem_free_card":
		return s.gemOrientAction(a)
	case "gem_post":
		if a.Type != "gem_post" || !slices.Contains(s.gemPostOptions(), a.Card) {
			return errors.New("请选择一个符合条件且未拥有的贸易站")
		}
		s.gemGainPost(a.Card)
		s.gemNext()
		return nil
	case "gem_token":
		if a.Type != "gem_token" || len(g.Effects) == 0 || a.Color < 0 || a.Color >= 5 || a.Color == g.Effects[0].Exclude || g.Bank[a.Color] < 1 {
			return errors.New("请选择一枚符合贸易站条件的宝石")
		}
		p.Tokens[a.Color]++
		g.Bank[a.Color]--
		take := make([]int, 6)
		take[a.Color] = 1
		s.recordSplendorTokens("take", take)
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 通过贸易站拿取 %s", s.Turn+1, splendorGemSummary(take)))
		g.Effects = g.Effects[1:]
	case "gem_reserve":
		if a.Type != "gem_reserve" {
			return errors.New("请先选择要预留的发展卡")
		}
		index := slices.IndexFunc(g.ReserveChoice, func(c Card) bool { return c.ID == a.Card })
		if index < 0 || len(p.Reserved) >= 3 {
			return errors.New("无法预留这张卡牌")
		}
		c := g.ReserveChoice[index]
		p.Reserved = append(p.Reserved, c)
		for i, other := range g.ReserveChoice {
			if i != index {
				g.Decks[other.gemDeck()] = append(g.Decks[other.gemDeck()], other)
			}
		}
		g.ReserveChoice = nil
		detail := fmt.Sprintf("玩家 %d 从 %d 级牌堆择优盲预留了 1 张发展卡", s.Turn+1, c.Tier)
		if g.Bank[5] > 0 {
			g.Bank[5]--
			p.Tokens[5]++
			s.recordSplendorTokens("gold", []int{0, 0, 0, 0, 0, 1})
			detail += "；获得黄金×1"
		}
		s.Log = append(s.Log, detail)
		s.recordSplendorCard(SplendorCardEvent{Action: "reserve", Source: "deck", Tier: c.Tier, Orient: c.Orient != "", Slot: -1})
	case "gem_stronghold":
		valid := false
		for _, candidate := range s.gemStrongholdActions() {
			if a.Type == candidate.Type && a.Choice == candidate.Choice && a.Card == candidate.Card && a.Target == candidate.Target {
				valid = true
				break
			}
		}
		if !valid {
			return errors.New("请选择合法的要塞放置、移动或移除操作")
		}
		if a.Choice == "remove" {
			delete(g.Strongholds, a.Card)
			s.Log = append(s.Log, fmt.Sprintf("玩家 %d 移除了发展卡 #%d 上对手的单座要塞", s.Turn+1, a.Card))
		} else {
			if a.Target > 0 {
				h := g.Strongholds[a.Target]
				h.Count--
				if h.Count == 0 {
					delete(g.Strongholds, a.Target)
				} else {
					g.Strongholds[a.Target] = h
				}
			}
			h := g.Strongholds[a.Card]
			h.Player = s.Turn
			h.Count++
			if g.Strongholds == nil {
				g.Strongholds = map[int]GemStronghold{}
			}
			g.Strongholds[a.Card] = h
			s.Log = append(s.Log, fmt.Sprintf("玩家 %d 在发展卡 #%d 放置要塞（共 %d 座）", s.Turn+1, a.Card, h.Count))
		}
		g.Effects = g.Effects[1:]
	case "gem_conquest":
		if a.Type == "gem_conquest_skip" {
			g.ConquestUsed = true
		} else {
			if a.Type != "gem_conquest" || a.Card != s.gemConquestCard() || a.Card == 0 {
				return errors.New("请选择三座己方要塞所在的发展卡")
			}
			g.ConquestUsed = true
			s.Phase = "turn"
			a.Type = "buy"
			return s.applySplendorStep(a)
		}
	default:
		return errors.New("未知的扩展操作阶段")
	}
	s.gemContinueEffects()
	return nil
}

func (s *State) gemCancelEffects() {
	g := s.Splendor
	// Pending blind cards are neither owned nor public. Return them privately.
	for _, c := range g.ReserveChoice {
		g.Decks[c.gemDeck()] = append(g.Decks[c.gemDeck()], c)
	}
	g.ReserveChoice, g.Effects = nil, nil
	for id, h := range g.Strongholds {
		if h.Player == s.Turn {
			delete(g.Strongholds, id)
		}
	}
	s.gemRefillMarket()
}

func (s *State) gemExpansionBot(player int) (Action, bool, error) {
	g := s.Splendor
	p := g.Players[player]
	choices := []botChoice{}
	switch s.Phase {
	case "gem_copy":
		for _, c := range p.Cards {
			if c.gemBonus() > 0 && c.ID != g.Effects[0].Card {
				choices = append(choices, botChoice{Action{Type: "gem_copy", Card: c.ID}, c.gemBonus()*20 - p.Bonus[c.Color]})
			}
		}
	case "gem_free_card":
		for _, c := range s.gemFreeCards(g.Effects[0].Tier) {
			choices = append(choices, botChoice{Action{Type: "gem_free_card", Card: c.ID}, c.Points*20 + c.gemBonus()*10 + sum(c.Cost)})
		}
	case "gem_post":
		for _, id := range s.gemPostOptions() {
			score := 0
			if id == GemPostPrestige {
				score = len(p.TradingPosts) * 100
			}
			if id == GemPostPurchaseToken {
				score += 60
			}
			choices = append(choices, botChoice{Action{Type: "gem_post", Card: id}, score})
		}
	case "gem_token":
		for i := 0; i < 5; i++ {
			if i != g.Effects[0].Exclude && g.Bank[i] > 0 {
				choices = append(choices, botChoice{Action{Type: "gem_token", Color: i}, -p.Tokens[i]*10 - p.Bonus[i]})
			}
		}
	case "gem_reserve":
		for _, c := range g.ReserveChoice {
			choices = append(choices, botChoice{Action{Type: "gem_reserve", Card: c.ID}, c.Points*3 - gemMissing(p, c, p.Tokens)*5})
		}
	case "gem_stronghold":
		for _, a := range s.gemStrongholdActions() {
			score := 0
			if a.Choice == "place" {
				c, _ := g.gemMarketCard(a.Card)
				score = 20 + c.Points*3 - gemMissing(p, c, p.Tokens)*5 + g.Strongholds[a.Card].Count*10
				if a.Target == a.Card {
					score -= 40
				}
			}
			choices = append(choices, botChoice{a, score})
		}
	case "gem_conquest":
		if c, ok := g.gemMarketCard(s.gemConquestCard()); ok {
			if a, ok := gemBestPurchase(p, c); ok {
				a.Type = "gem_conquest"
				choices = append(choices, botChoice{a, 100})
			}
		}
		choices = append(choices, botChoice{Action{Type: "gem_conquest_skip"}, 0})
	default:
		return Action{}, false, nil
	}
	a, err := s.botLegal(player, choices)
	return a, true, err
}
