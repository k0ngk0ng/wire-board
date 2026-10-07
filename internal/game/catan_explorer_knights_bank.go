package game

import "errors"

// E&P replaces the ordinary resource rate with 3:1. Commodity trades retain
// C&K's 4:1 default; its merchant/fleet/commerce discounts still apply to cards.
// Gold has its own E&P exchanges rather than being a ninth card type.
func (g *Catan) explorerCityRates(player int) []int {
	rates := g.rates(player)
	for color := 0; color < 5; color++ {
		rates[color] = min(rates[color], 3)
	}
	return rates
}

// Called inside the combined action transaction, which validates the complete
// supported state before indexing hand/discount arrays and commits atomically.
func (s *State) catanExplorerCityBank(player int, a Action) error {
	g, x := s.Catan, s.Catan.Explorer
	if err := x.Economy.actionAllowed(g, x.Fleet, x.Cargo, player, g.TurnSerial); err != nil {
		return err
	}
	give, take := a.Color, a.Target
	if a.Choice != "" || len(a.Give)+len(a.Take) != 0 || give < -1 || give >= 8 || take < -1 || take >= 8 || give == take {
		return errors.New("请选择一次交换的两种不同资源、商品或金币")
	}
	if give == -1 && take >= 5 {
		return errors.New("不能用金币向银行购买商品")
	}
	if take == -1 && give >= 5 {
		return errors.New("普通金币兑换须给出三张同种资源；商品售金须使用金币农场能力")
	}
	cost := 2
	if give >= 0 {
		cost = g.explorerCityRates(player)[give]
		if take == -1 {
			cost = 3 // The separate E&P resource-for-gold exchange.
		}
	}
	if give == -1 && (x.Economy.Turn.Bought >= 2 || x.Economy.Gold[player] < cost) || give >= 0 && g.Players[player].Resources[give] < cost || take == -1 && x.Economy.GoldBank < 1 || take >= 0 && g.Bank[take] < 1 {
		return errors.New("支付卡牌、金币或银行库存不足，或本行动段金币购牌已达两次")
	}
	paid, received := make([]int, 8), make([]int, 8)
	goldPaid, goldReceived := 0, 0
	if give == -1 {
		x.Economy.Gold[player] -= cost
		x.Economy.GoldBank += cost
		x.Economy.Turn.Bought++
		goldPaid = cost
	} else {
		g.Players[player].Resources[give] -= cost
		g.Bank[give] += cost
		paid[give] = cost
	}
	if take == -1 {
		x.Economy.Gold[player]++
		x.Economy.GoldBank--
		goldReceived = 1
	} else {
		g.Players[player].Resources[take]++
		g.Bank[take]--
		received[take] = 1
	}
	s.catanLog(player, "向银行支付 %s，换得 %s", catanTradeText(paid, goldPaid), catanTradeText(received, goldReceived))
	return nil
}
