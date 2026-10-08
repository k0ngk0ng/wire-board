package game

import "errors"

func (d *catanEventSession) alchemyLatest(roll int) bool {
	return d.AlchemyRolls > 0 && d.LastAlchemyRoll == roll
}

func (g *Catan) validateEventKnights() error {
	d := g.EventDeck
	if g.CitiesKnights == nil {
		if d.Knights != "" || d.AlchemyRolls != 0 || d.LastAlchemyRoll != 0 {
			return errors.New("非城市骑士存档包含炼金术或组合规则")
		}
		return nil
	}
	if d.Knights != CatanEventKnightsRules || !g.citySeaSupported() || g.Two != nil || g.Options.Helpers && g.Explorer == nil {
		return errors.New("事件牌与城市骑士组合配置无效")
	}
	if d.AlchemyRolls < 0 || d.AlchemyRolls > g.RollID || d.LastAlchemyRoll < d.AlchemyRolls || d.LastAlchemyRoll > g.RollID || (d.AlchemyRolls == 0) != (d.LastAlchemyRoll == 0) {
		return errors.New("炼金术替代抽牌次数与回合不一致")
	}
	return nil
}

func (g *Catan) validCardEventDice(red, face int) bool {
	if g.CitiesKnights == nil {
		return red == 0 && face == 0
	}
	return red >= 1 && red <= 6 && face >= 0 && face <= 5
}

func (g *Catan) validateEventAlchemy(phase string) error {
	if x := g.Explorer; x != nil && x.Economy.Turn != nil && (x.Economy.Turn.NoProduction || x.Economy.Turn.Phase == "roll" && g.Paired != nil && len(g.Dice) == 2 && g.Dice[0] == 0 && g.Dice[1] == 0) {
		if g.CitiesKnights != nil && g.CitiesKnights.Event == nil && g.RevealedEvent == nil && g.CardEvent == nil && len(g.Dice) == 2 && g.Dice[0] == 0 && g.Dice[1] == 0 {
			return nil
		}
		return errors.New("配对第二位炼金术记录无效")
	}
	if g.setup() || g.CitiesKnights == nil || g.CardEvent != nil || g.RevealedEvent != nil || phase == "catan_card_event" || len(g.Dice) != 2 || g.Dice[0] < 1 || g.Dice[0] > 6 || g.Dice[1] < 1 || g.Dice[1] > 6 {
		return errors.New("炼金术替代抽牌状态无效")
	}
	if e := g.CitiesKnights.Event; e != nil && (e.Production != 0 || e.Epidemic || e.Red != g.Dice[0] || e.Yellow != g.Dice[1] || e.Face < 0 || e.Face > 5) {
		return errors.New("炼金术城市事件与所选骰子不一致")
	}
	return nil
}
