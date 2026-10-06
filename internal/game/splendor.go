package game

import (
	_ "embed"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

//go:embed splendor_cards.csv
var cardCSV string

//go:embed splendor_nobles.csv
var nobleCSV string

type Card struct {
	Orient         string `json:"orient,omitempty"`
	BonusCount     int    `json:"bonusCount,omitempty"`
	SacrificeColor int    `json:"sacrificeColor,omitempty"`
	ID             int    `json:"id"`
	Tier           int    `json:"tier"`
	Points         int    `json:"points"`
	Color          int    `json:"color"`
	Cost           []int  `json:"cost"`
}
type Noble struct {
	ID   int   `json:"id"`
	Cost []int `json:"cost"`
}
type GemPlayer struct {
	TradingPosts []int   `json:"tradingPosts,omitempty"`
	Eliminated   bool    `json:"eliminated,omitempty"`
	Tokens       []int   `json:"tokens"`
	Bonus        []int   `json:"bonus"`
	Reserved     []Card  `json:"reserved"`
	Cards        []Card  `json:"cards"`
	Nobles       []Noble `json:"nobles"`
	Score        int     `json:"score"`
}
type Splendor struct {
	Catalog       string                `json:"catalog,omitempty"`
	Cities        []GemCity             `json:"cities,omitempty"`
	Exiled        []Card                `json:"exiled,omitempty"`
	Options       SplendorOptions       `json:"options"`
	Effects       []GemEffect           `json:"effects,omitempty"`
	Refills       []GemRefill           `json:"refills,omitempty"`
	ReserveChoice []Card                `json:"reserveChoice,omitempty"`
	Strongholds   map[int]GemStronghold `json:"strongholds,omitempty"`
	ConquestUsed  bool                  `json:"conquestUsed,omitempty"`
	StartPlayer   int                   `json:"startPlayer"`
	CardEventID   uint64                `json:"cardEventId,omitempty"`
	CardEvents    []SplendorCardEvent   `json:"cardEvents,omitempty"`
	TokenEventID  uint64                `json:"tokenEventId,omitempty"`
	TokenEvents   []SplendorTokenEvent  `json:"tokenEvents,omitempty"`
	Bank          []int                 `json:"bank"`
	Decks         [][]Card              `json:"decks"`
	Market        [][]Card              `json:"market"`
	Nobles        []Noble               `json:"nobles"`
	Players       []GemPlayer           `json:"players"`
	LastRound     bool                  `json:"lastRound"`
}

// Only public card information belongs in the shared animation stream.
// Blind reservations carry a tier and a card back, never a secret card ID.
type SplendorCardEvent struct {
	Orient bool   `json:"orient,omitempty"`
	ID     uint64 `json:"id"`
	Player int    `json:"player"`
	Action string `json:"action"`
	Source string `json:"source"`
	Tier   int    `json:"tier"`
	Slot   int    `json:"slot"`
	Card   *Card  `json:"card,omitempty"`
	Noble  *Noble `json:"noble,omitempty"`
}

func (s *State) recordSplendorCard(event SplendorCardEvent) {
	g := s.Splendor
	g.CardEventID++
	event.ID, event.Player = g.CardEventID, s.Turn
	g.CardEvents = append(g.CardEvents, event)
	if len(g.CardEvents) > 12 {
		g.CardEvents = g.CardEvents[len(g.CardEvents)-12:]
	}
}

func (s *State) recordSplendorNoble(noble Noble) {
	noble.Cost = append([]int{}, noble.Cost...)
	s.recordSplendorCard(SplendorCardEvent{Action: "noble", Source: "nobles", Noble: &noble})
}

type SplendorTokenEvent struct {
	ID     uint64 `json:"id"`
	Player int    `json:"player"`
	Action string `json:"action"`
	Tokens []int  `json:"tokens"`
}

func (s *State) recordSplendorTokens(action string, tokens []int) {
	if sum(tokens) == 0 {
		return
	}
	g := s.Splendor
	g.TokenEventID++
	g.TokenEvents = append(g.TokenEvents, SplendorTokenEvent{ID: g.TokenEventID, Player: s.Turn, Action: action, Tokens: append([]int{}, tokens...)})
	if len(g.TokenEvents) > 12 {
		g.TokenEvents = g.TokenEvents[len(g.TokenEvents)-12:]
	}
}

func integers(v []string) []int {
	r := []int{}
	for _, x := range v {
		n, e := strconv.Atoi(x)
		if e != nil {
			panic(e)
		}
		r = append(r, n)
	}
	return r
}
func Cards() []Card {
	rows, e := csv.NewReader(strings.NewReader(cardCSV)).ReadAll()
	if e != nil {
		panic(e)
	}
	r := []Card{}
	for i, row := range rows[1:] {
		v := integers(row)
		r = append(r, Card{ID: i + 1, Tier: v[0], Points: v[1], Color: v[2] - 1, Cost: v[3:]})
	}
	return r
}
func Nobles() []Noble {
	rows, e := csv.NewReader(strings.NewReader(nobleCSV)).ReadAll()
	if e != nil {
		panic(e)
	}
	r := []Noble{}
	for i, row := range rows[1:] {
		r = append(r, Noble{i + 1, integers(row)})
	}
	return r
}
func (s *State) initSplendor(n int) {
	g := &Splendor{Decks: make([][]Card, 3), Market: make([][]Card, 3), Bank: []int{7, 7, 7, 7, 7, 5}, Nobles: Nobles(), Players: make([]GemPlayer, n)}
	if n < 4 {
		for i := 0; i < 5; i++ {
			g.Bank[i] = n + 2
		}
	}
	for _, c := range Cards() {
		g.Decks[c.Tier-1] = append(g.Decks[c.Tier-1], c)
	}
	for i := 0; i < 3; i++ {
		shuffle(g.Decks[i])
		g.Market[i] = append([]Card{}, g.Decks[i][:4]...)
		g.Decks[i] = g.Decks[i][4:]
	}
	shuffle(g.Nobles)
	g.Nobles = g.Nobles[:n+1]
	for i := range g.Players {
		g.Players[i] = GemPlayer{Tokens: make([]int, 6), Bonus: make([]int, 5), Reserved: []Card{}, Cards: []Card{}, Nobles: []Noble{}}
	}
	s.Splendor = g
	seats := make([]int, n)
	for i := range seats {
		seats[i] = i
	}
	shuffle(seats)
	g.StartPlayer, s.Turn = seats[0], seats[0]
}
func (s *State) applySplendor(a Action) error {
	// Expansion effects can span several requests. Reject an invalid request
	// without altering a deck, pending choice, animation, or resource count.
	if s.Splendor.Options.expanded() {
		next := clone(*s)
		if err := next.applySplendorStep(a); err != nil {
			return err
		}
		*s = next
		return nil
	}
	return s.applySplendorStep(a)
}
func (s *State) applySplendorStep(a Action) error {
	g := s.Splendor
	p := &g.Players[s.Turn]
	if strings.HasPrefix(s.Phase, "gem_") {
		return s.gemExpansionAction(a)
	}
	if s.Phase == "discard" {
		if a.Type != "discard" || len(a.Tokens) != 6 || sum(a.Tokens) != sum(p.Tokens)-10 {
			return errors.New("请选择多出的宝石归还，使总数为 10")
		}
		for i, n := range a.Tokens {
			if n < 0 || n > p.Tokens[i] {
				return errors.New("归还数量不正确")
			}
		}
		for i, n := range a.Tokens {
			p.Tokens[i] -= n
			g.Bank[i] += n
		}
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 归还了 %s（持有 %d / 10 枚）", s.Turn+1, splendorGemSummary(a.Tokens), sum(p.Tokens)))
		s.recordSplendorTokens("return", a.Tokens)
		s.gemAfter()
		return nil
	}
	if s.Phase == "noble" {
		if a.Type != "noble" {
			return errors.New("请先选择一位贵族")
		}
		for i, n := range g.Nobles {
			if n.ID == a.Noble && eligible(p, n) {
				p.Nobles = append(p.Nobles, n)
				p.Score += 3
				s.logSplendorNoble(n)
				s.recordSplendorNoble(n)
				g.Nobles = append(g.Nobles[:i], g.Nobles[i+1:]...)
				s.gemTradingPostCheck()
				return nil
			}
		}
		return errors.New("这位贵族的条件尚未满足")
	}
	switch a.Type {
	case "take":
		if len(a.Tokens) != 6 || a.Tokens[5] != 0 {
			return errors.New("不能直接拿取黄金")
		}
		different, total, twos, available := 0, 0, 0, 0
		for i := 0; i < 5; i++ {
			if g.Bank[i] > 0 {
				available++
			}
			v := a.Tokens[i]
			if v < 0 || v > 2 || v > g.Bank[i] {
				return errors.New("宝石数量不正确")
			}
			if v > 0 {
				different++
			}
			if v == 2 {
				twos++
				if g.Bank[i] < 4 {
					return errors.New("拿取两枚同色宝石前，该堆至少需要四枚")
				}
			}
			total += v
		}
		if !((twos == 1 && different == 1 && total == 2) || (twos == 0 && different == min(3, available) && total > 0)) {
			return errors.New("拿取三种不同颜色（供应不足时尽量拿取），或两枚同色宝石")
		}
		for i, n := range a.Tokens {
			p.Tokens[i] += n
			g.Bank[i] -= n
		}
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 拿取了 %s（共 %d 枚）", s.Turn+1, splendorGemSummary(a.Tokens), total))
		s.recordSplendorTokens("take", a.Tokens)
		if twos == 1 && p.hasPost(GemPostExtraToken) {
			for color, n := range a.Tokens {
				if n == 2 {
					g.Effects = append(g.Effects, GemEffect{Kind: "token", Exclude: color})
				}
			}
		}
	case "reserve", "buy":
		if a.Type == "reserve" && len(p.Reserved) >= 3 {
			return errors.New("最多预留三张卡牌")
		}
		var c Card
		loc, idx := -2, -1
		if a.Card == 0 && a.Type == "reserve" {
			loc = a.Tier - 1
			if a.Choice == "orient" {
				loc += 3
			} else if a.Choice != "" {
				return errors.New("未知牌堆")
			}
			if a.Tier < 1 || a.Tier > 3 || loc >= len(g.Decks) || len(g.Decks[loc]) == 0 {
				return errors.New("牌堆已空")
			}
			if p.hasPost(GemPostBlindReserve) {
				g.ReserveChoice = append([]Card{}, g.Decks[loc][:min(2, len(g.Decks[loc]))]...)
				g.Decks[loc] = g.Decks[loc][len(g.ReserveChoice):]
				s.Phase = "gem_reserve"
				return nil
			}
			c = g.Decks[loc][0]
			idx = -1
		} else {
			for tier, cards := range g.Market {
				for j, x := range cards {
					if x.ID > 0 && x.ID == a.Card {
						c = x
						loc = tier
						idx = j
					}
				}
			}
			if a.Type == "buy" {
				for j, x := range p.Reserved {
					if x.ID == a.Card {
						c = x
						loc = -1
						idx = j
					}
				}
			}
			if loc == -2 {
				return errors.New("卡牌已不在此处")
			}
		}
		if !g.gemCardAccessible(c.ID, s.Turn) {
			return errors.New("该卡牌由对手的要塞占据")
		}
		var detail string
		if a.Type == "buy" {
			pay, discard, err := gemOrientPayment(*p, c, a)
			if err != nil {
				return err
			}
			s.gemDiscardCards(discard)
			for i, n := range pay {
				p.Tokens[i] -= n
				g.Bank[i] += n
			}
			g.Effects = append(g.Effects, s.gemAcquireCard(c)...)
			s.recordSplendorTokens("pay", pay)
			source := "市场"
			if loc == -1 {
				source = "预留区"
			}
			payment := "无需支付宝石"
			if sum(pay) > 0 {
				payment = "支付 " + splendorGemSummary(pay)
			}
			bonus := ""
			if c.gemBonus() > 0 {
				bonus = fmt.Sprintf("；永久%s +%d", splendorGemNames[c.Color], c.gemBonus())
			}
			if len(discard) > 0 {
				payment += fmt.Sprintf("；弃置发展卡×%d", len(discard))
			}
			detail = fmt.Sprintf("从%s购买了 %s；%s%s，当前 %d 分", source, splendorCardSummary(c), payment, bonus, p.Score)
		} else {
			// A blind reservation must never include the drawn card's identity,
			// color, points or cost in the shared log.
			if idx == -1 {
				detail = fmt.Sprintf("从 %d 级牌堆盲预留了 1 张发展卡", c.Tier)
			} else {
				detail = "从市场预留了 " + splendorCardSummary(c)
			}
			p.Reserved = append(p.Reserved, c)
			if g.Bank[5] > 0 {
				g.Bank[5]--
				p.Tokens[5]++
				s.recordSplendorTokens("gold", []int{0, 0, 0, 0, 0, 1})
				detail += "；获得黄金×1"
			} else {
				detail += "；黄金已空，未获得黄金"
			}
		}
		event := SplendorCardEvent{Action: a.Type, Source: "market", Tier: c.Tier, Orient: c.Orient != "", Slot: idx, Card: &c}
		if loc == -1 {
			event.Source = "reserved"
		} else if idx == -1 {
			event.Source, event.Card = "deck", nil
		}
		s.recordSplendorCard(event)
		delete(g.Strongholds, c.ID)
		if loc == -1 {
			p.Reserved = append(p.Reserved[:idx], p.Reserved[idx+1:]...)
		} else if idx == -1 {
			g.Decks[loc] = g.Decks[loc][1:]
		} else if g.Options.expanded() {
			g.Market[loc][idx] = Card{Tier: c.Tier, Orient: c.Orient}
			g.Refills = append(g.Refills, GemRefill{Tier: loc, Slot: idx})
		} else if len(g.Decks[loc]) > 0 {
			g.Market[loc][idx] = g.Decks[loc][0]
			g.Decks[loc] = g.Decks[loc][1:]
		} else {
			g.Market[loc] = append(g.Market[loc][:idx], g.Market[loc][idx+1:]...)
		}
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d %s", s.Turn+1, detail))
		if a.Type == "buy" {
			if p.hasPost(GemPostPurchaseToken) {
				g.Effects = append(g.Effects, GemEffect{Kind: "token", Exclude: -1})
			}
			if g.Options.Strongholds {
				g.Effects = append(g.Effects, GemEffect{Kind: "stronghold"})
			}
		}
	case "pass":
		if s.gemHasMove() {
			return errors.New("仍有合法行动，不能跳过")
		}
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 无可用行动，跳过", s.Turn+1))
	default:
		return errors.New("未知操作")
	}
	s.gemContinueEffects()
	return nil
}
func eligible(p *GemPlayer, n Noble) bool {
	counts := p.Bonus
	for _, c := range p.Cards {
		if c.Orient != "" {
			physical := p.gemCardCounts()
			counts = physical[:]
			break
		}
	}
	for i, c := range n.Cost {
		if counts[i] < c {
			return false
		}
	}
	return true
}
func (s *State) gemAfter() {
	g := s.Splendor
	p := &g.Players[s.Turn]
	if g.Options.Cities {
		s.gemTradingPostCheck()
		return
	}
	options := []int{}
	for i, n := range g.Nobles {
		if eligible(p, n) {
			options = append(options, i)
		}
	}
	if len(options) > 1 {
		s.Phase = "noble"
		return
	}
	if len(options) == 1 {
		i := options[0]
		p.Nobles = append(p.Nobles, g.Nobles[i])
		p.Score += 3
		s.logSplendorNoble(g.Nobles[i])
		s.recordSplendorNoble(g.Nobles[i])
		g.Nobles = append(g.Nobles[:i], g.Nobles[i+1:]...)
	}
	s.gemTradingPostCheck()
}
func (s *State) gemNext() {
	g := s.Splendor
	trigger := !g.Players[s.Turn].Eliminated && g.Players[s.Turn].Score >= 15
	if g.Options.Cities {
		trigger = s.gemCityTrigger()
	}
	if trigger {
		g.LastRound = true
	}
	s.Phase = "turn"
	g.ConquestUsed = false
	wrapped := false
	for range g.Players {
		s.Turn = (s.Turn + 1) % len(g.Players)
		wrapped = wrapped || s.Turn == g.StartPlayer
		if !g.Players[s.Turn].Eliminated {
			break
		}
	}
	if wrapped {
		s.Round++
		if g.LastRound && g.Options.Cities {
			eligible := false
			for _, p := range g.Players {
				eligible = eligible || len(g.gemCitiesFor(p)) > 0
			}
			g.LastRound = eligible
		}
		if g.LastRound {
			s.Finished = true
			s.Phase = "finished"
			best, cards := -1, 999
			for i, p := range g.Players {
				if p.Eliminated || (g.Options.Cities && len(g.gemCitiesFor(p)) == 0) {
					continue
				}
				if p.Score > best || (p.Score == best && len(p.Cards) < cards) {
					best = p.Score
					cards = len(p.Cards)
					s.Winners = []int{i}
				} else if p.Score == best && len(p.Cards) == cards {
					s.Winners = append(s.Winners, i)
				}
			}
		}
	}
}

// EliminateSplendor removes the current seat without changing player indices.
// The server is responsible for authorizing the timeout and the requesting user.
func (s *State) EliminateSplendor(player int) error {
	if s.Kind != "splendor" || s.Finished || player != s.Turn || player < 0 || player >= len(s.Splendor.Players) || s.Splendor.Players[player].Eliminated {
		return errors.New("无法移除此玩家")
	}
	g := s.Splendor
	p := &g.Players[player]
	s.gemCancelEffects()
	p.Eliminated = true
	for i, n := range p.Tokens {
		g.Bank[i] += n
		p.Tokens[i] = 0
	}
	// Return secret reservations to their decks without revealing their identities.
	touched := make([]bool, len(g.Decks))
	for _, card := range p.Reserved {
		g.Decks[card.gemDeck()] = append(g.Decks[card.gemDeck()], card)
		touched[card.gemDeck()] = true
	}
	p.Reserved = []Card{}
	for tier, changed := range touched {
		if changed {
			shuffle(g.Decks[tier])
		}
	}
	s.Log = append(s.Log, fmt.Sprintf("玩家 %d 超时被移出，筹码归还供应区，预留卡洗回牌堆", player+1))
	active := []int{}
	for i, p := range g.Players {
		if !p.Eliminated {
			active = append(active, i)
		}
	}
	if len(active) == 1 {
		s.Finished, s.Phase, s.Winners, s.Turn = true, "finished", active, active[0]
		s.Log = append(s.Log, fmt.Sprintf("仅剩玩家 %d，本局获胜", active[0]+1))
		return nil
	}
	s.gemNext()
	return nil
}
func (s *State) gemHasMove() bool {
	g := s.Splendor
	p := g.Players[s.Turn]
	if sum(g.Bank[:5]) > 0 {
		return true
	}
	if len(p.Reserved) < 3 {
		for i := range g.Decks {
			if len(g.Decks[i]) > 0 {
				return true
			}
			for _, c := range g.Market[i] {
				if c.ID > 0 && g.gemCardAccessible(c.ID, s.Turn) {
					return true
				}
			}
		}
	}
	cards := append([]Card{}, p.Reserved...)
	for _, m := range g.Market {
		cards = append(cards, m...)
	}
	for _, c := range cards {
		if c.ID == 0 || !g.gemCardAccessible(c.ID, s.Turn) {
			continue
		}
		if _, ok := gemBestPurchase(p, c); ok {
			return true
		}
	}
	return false
}

// Match the gem order used by the rules and the board: green, white, blue,
// black, red, gold. Explicit color names also make plain-text logs unambiguous.
var splendorGemNames = [...]string{"祖母绿（绿）", "钻石（白）", "蓝宝石（蓝）", "缟玛瑙（黑）", "红宝石（红）", "黄金"}

func splendorGemSummary(tokens []int) string {
	parts := []string{}
	for i, n := range tokens {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%s×%d", splendorGemNames[i], n))
		}
	}
	return strings.Join(parts, "、")
}

func splendorCardSummary(c Card) string {
	color := "无色"
	if c.Color >= 0 && c.Color < 5 {
		color = splendorGemNames[c.Color]
	}
	if c.Orient == GemOrientGold {
		color = "东方黄金"
	} else if c.gemCopy() && c.Color < 0 {
		color = "东方复制"
	}
	return fmt.Sprintf("%s发展卡（%d 级，%d 分，#%d）", color, c.Tier, c.Points, c.ID)
}

func (s *State) logSplendorNoble(n Noble) {
	s.Log = append(s.Log, fmt.Sprintf("玩家 %d 获得贵族 #%d 的来访（+3 分），当前 %d 分", s.Turn+1, n.ID, s.Splendor.Players[s.Turn].Score))
}
