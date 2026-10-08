package game

import "errors"

// Both ordinary rolls and Alchemy enter the same combined event/production
// controller. Clone before consuming a progress card so a failed downstream
// payout cannot consume Alchemy or leave an event half applied.
func (s *State) catanExplorerCityRollAction(player int, a Action, randN func(int) int) error {
	if err := s.validateExplorerCityProduction(); err != nil {
		return err
	}
	g := s.Catan
	if s.Finished || s.Phase != "catan_roll" || player != s.Turn || a.Prompt < 1 || uint64(a.Prompt) != g.TurnSerial || a.Skill != "" || randN == nil {
		return errors.New("不是当前组合掷骰玩家、阶段或序号")
	}
	next := clone(*s)
	switch a.Type {
	case "catan_roll":
		if len(a.Tokens) != 0 || a.Choice != "" {
			return errors.New("普通掷骰不能指定点数")
		}
		if g.EventDeck != nil {
			if err := next.catanDrawEventRandom(randN); err != nil {
				return err
			}
			break
		}
		red, yellow, face := randN(6)+1, randN(6)+1, randN(6)
		if err := next.catanExplorerCityRoll(red, yellow, face); err != nil {
			return err
		}
	case "catan_progress":
		if a.Card != 0 {
			return errors.New("掷骰前只能使用炼金术")
		}
		if err := next.catanPlayProgressRandom(player, a, randN); err != nil {
			return err
		}
	default:
		return errors.New("请掷骰或使用炼金术")
	}
	if err := next.validateExplorerCityProduction(); err != nil {
		return err
	}
	*s = next
	return nil
}
