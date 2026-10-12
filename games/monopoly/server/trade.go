package monopoly

import (
	"errors"
	"slices"
	"time"
)

type Bundle struct {
	Cash       int      `json:"cash"`
	Properties []int    `json:"properties"`
	JailCards  []string `json:"jailCards"`
	Redeem     []int    `json:"redeem"` // Recipient redeems these on settlement; otherwise pays interest.
}
type TradeTerms struct {
	Target int    `json:"target"`
	Give   Bundle `json:"give"`
	Want   Bundle `json:"want"`
}
type Trade struct {
	ID        int          `json:"id"`
	From      int          `json:"from"`
	To        int          `json:"to"`
	Give      Bundle       `json:"give"`
	Want      Bundle       `json:"want"`
	Expires   int64        `json:"expires"`
	Mortgages map[int]bool `json:"mortgages"`
}

func (s *State) hasIncomingTrade(p int) bool {
	if !s.tradeAllowed() {
		return false
	}
	for _, offer := range s.Trades {
		if offer.To == p {
			return true
		}
	}
	return false
}
func (s *State) tradeAllowed() bool {
	return s.Phase != "finished" && s.Phase != "mortgage_transfer"
}

func (s *State) bundleFee(owner int, b Bundle) (int, error) {
	if b.Cash < 0 || b.Cash > s.available(owner) {
		return 0, errors.New("报价现金不足或已用于竞价")
	}
	seen := map[int]bool{}
	fee := 0
	for _, id := range b.Properties {
		if seen[id] || !s.owns(owner, id) || s.groupBuildings(id) > 0 {
			return 0, errors.New("地产已易主、重复或色组仍有建筑")
		}
		seen[id] = true
		if s.Properties[id].Mortgaged {
			fee += mortgageFee(id)
			if slices.Contains(b.Redeem, id) {
				fee += board[id].Price / 2
			}
		}
	}
	seenRedeem := map[int]bool{}
	for _, id := range b.Redeem {
		if seenRedeem[id] || !seen[id] || !s.Properties[id].Mortgaged {
			return 0, errors.New("无效的赎回选择")
		}
		seenRedeem[id] = true
	}
	cards := map[string]bool{}
	for _, card := range b.JailCards {
		if cards[card] || (card != "chance" && card != "chest") || !slices.Contains(s.Players[owner].JailCards, card) {
			return 0, errors.New("免狱卡不存在或重复")
		}
		cards[card] = true
	}
	return fee, nil
}
func (s *State) checkTrade(t Trade) (fromFee, toFee int, err error) {
	if t.From < 0 || t.To < 0 || t.From >= len(s.Players) || t.To >= len(s.Players) || t.From == t.To || s.Players[t.From].Bankrupt || s.Players[t.To].Bankrupt {
		return 0, 0, errors.New("无效交易玩家")
	}
	if len(t.Give.Properties)+len(t.Want.Properties)+len(t.Give.JailCards)+len(t.Want.JailCards) == 0 {
		return 0, 0, errors.New("交易须包含地产或免狱卡，不支持借款或空报价")
	}
	if s.buildingAuction() && s.Auction.Bidder >= 0 {
		for _, side := range []struct {
			owner  int
			bundle Bundle
		}{{t.From, t.Give}, {t.To, t.Want}} {
			for _, id := range side.bundle.Properties {
				if id >= 0 && id < 40 && side.owner == s.Auction.Bidder && board[id].Group == board[s.Auction.Property].Group {
					return 0, 0, errors.New("竞价期间不能拆散当前最高报价的建设色组")
				}
			}
		}
	}
	for id, mortgaged := range t.Mortgages {
		if id < 0 || id >= 40 || s.Properties[id].Mortgaged != mortgaged {
			return 0, 0, errors.New("抵押状态已改变，请重新报价")
		}
	}
	toFee, err = s.bundleFee(t.From, t.Give)
	if err != nil {
		return
	}
	fromFee, err = s.bundleFee(t.To, t.Want)
	if err != nil {
		return
	}
	if s.available(t.From)-t.Give.Cash+t.Want.Cash < fromFee || s.available(t.To)-t.Want.Cash+t.Give.Cash < toFee {
		err = errors.New("交易后资金不足以支付抵押费用")
	}
	return
}
func (s *State) offerTrade(p int, terms *TradeTerms, now time.Time) error {
	if !s.tradeAllowed() || terms == nil {
		return errors.New("当前不能发起交易")
	}
	t := Trade{From: p, To: terms.Target, Give: terms.Give, Want: terms.Want, Mortgages: map[int]bool{}}
	if _, _, err := s.checkTrade(t); err != nil {
		return err
	}
	for _, b := range []Bundle{t.Give, t.Want} {
		for _, id := range b.Properties {
			t.Mortgages[id] = s.Properties[id].Mortgaged
		}
	}
	s.TradeSerial++
	if len(s.TradeSent) != len(s.Players) {
		s.TradeSent = make([]int, len(s.Players))
		for i := range s.TradeSent {
			s.TradeSent[i] = -1
		}
	}
	s.TradeSent[p] = s.TurnSerial
	t.ID = s.TradeSerial
	seconds := s.Options.TradeSeconds
	if seconds <= 0 {
		seconds = 90
	}
	t.Expires = now.Add(time.Duration(seconds) * time.Second).UnixMilli()
	s.Trades = slices.DeleteFunc(s.Trades, func(old Trade) bool { return old.From == p })
	s.Trades = append(s.Trades, t)
	s.Waiting[t.To] = false
	return nil
}
func (s *State) tradeIndex(id int) int {
	for i, t := range s.Trades {
		if t.ID == id {
			return i
		}
	}
	return -1
}
func (s *State) answerTrade(p, id int, kind string, now time.Time) error {
	i := s.tradeIndex(id)
	if i < 0 {
		return errors.New("交易已撤回、修改或结束")
	}
	t := s.Trades[i]
	if now.UnixMilli() >= t.Expires {
		return errors.New("交易报价已过期")
	}
	if kind == "trade_cancel" {
		if p != t.From {
			return errors.New("只能撤回自己的报价")
		}
		s.Trades = append(s.Trades[:i], s.Trades[i+1:]...)
		return nil
	}
	if p != t.To {
		return errors.New("只能回应发给自己的交易")
	}
	if kind == "trade_reject" {
		s.Trades = append(s.Trades[:i], s.Trades[i+1:]...)
		return nil
	}
	if !s.tradeAllowed() {
		return errors.New("请先完成当前结算或建筑拍卖")
	}
	fromFee, toFee, err := s.checkTrade(t)
	if err != nil {
		return err
	}
	// All validation precedes mutation; incoming cash can fund receiving fees.
	s.pay(t.From, t.To, t.Give.Cash)
	s.pay(t.To, t.From, t.Want.Cash)
	s.pay(t.From, -1, fromFee)
	s.pay(t.To, -1, toFee)
	transfer := func(from, to int, b Bundle) {
		for _, id := range b.Properties {
			s.Properties[id].Owner = to
			if slices.Contains(b.Redeem, id) {
				s.Properties[id].Mortgaged = false
			}
		}
		for _, card := range b.JailCards {
			index := slices.Index(s.Players[from].JailCards, card)
			s.Players[from].JailCards = append(s.Players[from].JailCards[:index], s.Players[from].JailCards[index+1:]...)
			s.Players[to].JailCards = append(s.Players[to].JailCards, card)
		}
	}
	transfer(t.From, t.To, t.Give)
	transfer(t.To, t.From, t.Want)
	s.Trades = append(s.Trades[:i], s.Trades[i+1:]...)
	s.note("玩家 %d 与玩家 %d 完成交易；抵押费用分别为 %d、%d", t.From+1, t.To+1, fromFee, toFee)
	return s.settleDebt(now)
}
func (s *State) tradeViews(viewer int) []any {
	views := []any{}
	for _, t := range s.Trades {
		if viewer != t.From && viewer != t.To {
			continue
		}
		fromFee, toFee, err := s.checkTrade(t)
		reason := ""
		if err != nil {
			reason = err.Error()
		}
		views = append(views, map[string]any{"id": t.ID, "from": t.From, "to": t.To, "give": t.Give, "want": t.Want, "expires": t.Expires, "fromFee": fromFee, "toFee": toFee, "valid": err == nil, "reason": reason})
	}
	return views
}

// Bot valuation uses only public assets and the addressed proposal, never decks
// or randomness. This is a conservative baseline, not a negotiation engine.
func (s *State) bundleValue(b Bundle) int {
	value := b.Cash + 50*len(b.JailCards)
	for _, id := range b.Properties {
		price := board[id].Price
		if s.Properties[id].Mortgaged && !slices.Contains(b.Redeem, id) {
			price /= 2
		}
		value += price
	}
	return value
}
