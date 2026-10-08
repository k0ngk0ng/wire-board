package game

import "errors"

// E&P + T&B (2025), p1: use production values and ignore every card effect.
// The site's face catalogue is unchanged; this tag records the combination.
const CatanEventExplorerRules = "explorer-production-2025"

func (g *Catan) validateEventExplorer() error {
	d := g.EventDeck
	if g.Explorer == nil {
		if d.Explorer != "" {
			return errors.New("非探险存档包含探险事件规则")
		}
		return nil
	}
	if g.Explorer.Economy == nil || d.Explorer != CatanEventExplorerRules || g.CardEvent != nil || g.Two != nil || g.Options != (CatanOptions{}) {
		return errors.New("探险事件只使用生产点数，不执行事件文字")
	}
	if t := g.Explorer.Economy.Turn; t != nil && t.Phase != "roll" && !t.NoProduction && !(t.Phase == "abandoned" && t.Dice == [2]int{}) {
		if (t.Production == 0) != d.alchemyLatest(g.RollID) {
			return errors.New("探险生产来源与事件牌或炼金术记录不一致")
		}
	}
	return nil
}

// Called inside catanDrawEventRandom's transaction, after drawing the real
// card. E&P uses one draw even with two players; fish-shoal dice are unrelated.
func (s *State) catanExplorerEventProduction(kind string, number, red, face int) error {
	g, x := s.Catan, s.Catan.Explorer
	if x == nil || x.Economy.Turn == nil || x.Economy.Turn.Phase != "roll" || !g.validCardEventDice(red, face) {
		return errors.New("不是探险事件生产阶段")
	}
	t := x.Economy.Turn
	t.Production, t.Dice, t.Phase = number, [2]int{red, 0}, "event"
	g.Dice = []int{red, 0}
	g.RollID++
	g.RevealedEvent = &CatanRevealedEvent{Kind: kind, Production: number, Red: red, Face: face, RollID: g.RollID}
	s.catanLog(s.Turn, "抽到%s：仅使用生产点数%d，忽略事件文字", catanCardEventNames[kind], number)
	if g.CitiesKnights != nil {
		t.Phase = "city"
		return s.catanStartCityDiceEvent(red, 0, face, number, false)
	}
	result, err := x.Economy.resolveProduction(g, x.Fleet, x.Cargo, s.Turn, g.TurnSerial, t.Dice)
	if err != nil {
		return err
	}
	g.RevealedEvent.ProductionStarted = true
	for p, hand := range result.Resources {
		if sum(hand) > 0 || result.Gold[p] > 0 {
			s.catanLog(p, "生产获得 %s", catanTradeText(hand, result.Gold[p]))
		}
	}
	return s.catanExplorerAfterProduction()
}

func (t catanExplorerEconomyTurn) productionNumber() int {
	if t.Production != 0 {
		return t.Production
	}
	return t.Dice[0] + t.Dice[1]
}
