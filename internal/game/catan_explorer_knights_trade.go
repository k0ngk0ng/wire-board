package game

import "errors"

// Called inside the private whole-state action transaction. Offers include
// all eight card types and E&P gold; progress cards are never tradable.
func (s *State) catanExplorerCityPlayerTrade(player int, a Action) error {
	g := s.Catan
	if g.Paired != nil && g.Paired.Second {
		return errors.New("配对第二位只能与银行交易，不能与玩家自由交易")
	}
	switch a.Type {
	case "catan_trade_offer":
		return s.catanOffer(player, a)
	case "catan_trade_accept", "catan_trade_reject":
		return s.catanRespondTrade(player, a)
	case "catan_trade_complete":
		return s.catanCompleteTrade(player, a)
	case "catan_trade_cancel":
		if g.Trade == nil || g.Trade.From != player || g.Trade.ID != a.Offer {
			return errors.New("此交易已结束或不属于你")
		}
		g.Trade = nil
		return nil
	}
	return errors.New("未知玩家交易")
}

func (s *State) validateExplorerCityTrade() error {
	g := s.Catan
	t := g.Trade
	if t == nil {
		return nil
	}
	if s.Finished || s.Phase != "catan_turn" || g.Paired != nil && g.Paired.Second || t.From != s.Turn || t.ID <= 0 || t.ID != g.TradeID || len(t.Responses) != len(g.Players) || !g.cardBundle(t.Give) || !g.cardBundle(t.Take) || !g.validTradeGold(t.GoldGive) || !g.validTradeGold(t.GoldTake) || t.GoldGive > 0 && t.GoldTake > 0 || sum(t.Give)+t.GoldGive == 0 || sum(t.Take)+t.GoldTake == 0 {
		return errors.New("组合玩家交易阶段、卡牌或报价无效")
	}
	for r := range t.Give {
		if t.Give[r] > 0 && t.Take[r] > 0 {
			return errors.New("组合交易不能同时给出和索取同种卡牌")
		}
	}
	for p, response := range t.Responses {
		if response < -1 || response > 1 || p == t.From && response != 0 || g.Players[p].Eliminated && response == 1 {
			return errors.New("组合交易回应记录无效")
		}
	}
	return nil
}
