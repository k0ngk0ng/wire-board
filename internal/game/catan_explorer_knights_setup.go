package game

import "errors"

// A combined setup can restore shuffled progress decks, but no city effects,
// cards or upgrades may have happened before the first production roll.
func (g *Catan) validateExplorerCityOpening() error {
	k := g.CitiesKnights
	if k == nil {
		return nil
	}
	if k.Layout != "variable" || k.ActionSerial != 1 || k.EventDie != -1 || k.BarbarianPosition != 0 || k.Invasions != 0 || k.RobberStart != -1 || k.Pending != nil || k.Event != nil || k.Merchant != nil || k.TradePowers != nil || k.Chase != "" || k.ProgressEventID != 0 || len(k.ProgressEvents)+len(k.FallenCities)+len(k.Knights)+len(k.Walls) != 0 || k.Metropolises != [3]int{-1, -1, -1} {
		return errors.New("组合开局尚不能包含城市事件、骑士、城墙或进步效果")
	}
	counts := make([]int, len(catanProgressRules))
	for track, deck := range k.ProgressDecks {
		for _, card := range deck {
			if card < 0 || card >= len(catanProgressRules) || catanProgressRules[card].Track != track {
				return errors.New("组合起始进步牌轨道或牌号无效")
			}
			counts[card]++
		}
	}
	for id, rule := range catanProgressRules {
		if counts[id] != rule.Count {
			return errors.New("组合起始进步牌库存不守恒")
		}
	}
	for _, p := range k.Players {
		if len(p.Progress)+len(p.PublicProgress) != 0 || p.Improvements != [3]int{} || p.DefenderPoints != 0 || p.ProgressPoints != 0 {
			return errors.New("组合开局不能已有城市建设或进步牌奖励")
		}
	}
	return nil
}
