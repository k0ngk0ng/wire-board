package game

import (
	"errors"
	"slices"
)

// Production-controller checks; the unified State.Apply boundary additionally
// validates complete world components, progress inventory and turn metadata.
// Use a complete copy because the last city response can fail a later payout.
func (s *State) catanExplorerCityRoll(red, yellow, face int) error {
	if err := s.validateExplorerCityProduction(); err != nil {
		return err
	}
	g := s.Catan
	if s.Finished || s.Phase != "catan_roll" || g.Explorer.Economy.Turn.Phase != "roll" || g.CitiesKnights.Pending != nil || g.CitiesKnights.Event != nil || red < 1 || red > 6 || yellow < 1 || yellow > 6 || face < 0 || face > 5 {
		return errors.New("不是有效的组合掷骰阶段或骰子")
	}
	next := clone(*s)
	ng := next.Catan
	ng.Explorer.Economy.Turn.Dice = [2]int{red, yellow}
	ng.Explorer.Economy.Turn.Phase = "city"
	ng.Dice = []int{red, yellow}
	ng.RollID++
	if err := next.catanStartCityDiceEvent(red, yellow, face, 0, false); err != nil {
		return err
	}
	if err := next.validateExplorerCityProduction(); err != nil {
		return err
	}
	*s = next
	return nil
}

func (s *State) catanExplorerCityRespond(player int, a Action) error {
	if s.Phase == "catan_progress_end" {
		return s.catanExplorerCityFlow(player, a)
	}
	if err := s.validateExplorerCityProduction(); err != nil {
		return err
	}
	q := s.Catan.CitiesKnights.Pending
	if s.Finished || q == nil || len(q.Players) == 0 || q.Players[0] != player || a.Prompt < 1 || uint64(a.Prompt) != s.Catan.TurnSerial || !slices.Contains([]string{"pillage", "defender_reward", "progress_discard", "aqueduct", "metropolis", "knight_retreat", "guild_dues", "commercial_harbor", "diplomacy", "espionage", "sabotage", "wedding", "treason_remove", "treason_place"}, q.Kind) {
		return errors.New("不是当前组合城市回应玩家或序号")
	}
	next := clone(*s)
	if err := next.catanCityChoice(player, a); err != nil {
		return err
	}
	if !next.Finished && next.Catan.CitiesKnights.Pending == nil && next.Catan.Explorer.Economy.Turn.Phase == "aqueduct" {
		if err := next.catanExplorerCityFinishProduction(); err != nil {
			return err
		}
	}
	if err := next.validateExplorerCityProduction(); err != nil {
		return err
	}
	*s = next
	return nil
}

// Called only after the shared city event has applied pillage/draw/defense.
func (s *State) catanExplorerCityProduction(dice [2]int) error {
	g, x := s.Catan, s.Catan.Explorer
	result, err := x.Economy.resolveProduction(g, x.Fleet, x.Cargo, s.Turn, g.TurnSerial, dice)
	if err != nil {
		return err
	}
	received := make([]int, len(g.Players))
	for p, hand := range result.Resources {
		received[p] = sum(hand)
		if received[p] > 0 || result.Gold[p] > 0 {
			s.catanLog(p, "生产获得 %s", catanTradeText(hand, result.Gold[p]))
		}
	}
	if x.Economy.Turn.Phase == "aqueduct" {
		s.catanStartAqueduct(received)
		if g.CitiesKnights.Pending != nil {
			return nil
		}
		return s.catanExplorerCityFinishProduction()
	}
	return s.catanExplorerAfterProduction()
}
func (s *State) catanExplorerCityFinishProduction() error {
	g, x := s.Catan, s.Catan.Explorer
	if x.Economy.Turn.Phase != "aqueduct" || g.CitiesKnights.Pending != nil || g.CitiesKnights.Event != nil {
		return errors.New("城市补偿尚未完成")
	}
	if err := x.Cargo.beginAction(g, x.Fleet, s.Turn, g.TurnSerial); err != nil {
		return err
	}
	x.Economy.Turn.Phase = "ready"
	return s.catanExplorerAfterProduction()
}

func (s *State) validateExplorerCityProduction() error {
	g := s.Catan
	if g == nil || g.CitiesKnights == nil || g.Explorer == nil {
		return errors.New("组合生产组件缺失")
	}
	x, k := g.Explorer, g.CitiesKnights
	if x.Board == nil || !x.Board.CitiesKnights || x.Economy == nil || x.Cargo == nil || x.Fleet == nil || x.Setup != nil || x.Economy.Turn == nil || s.Turn < 0 || s.Turn >= len(g.Players) || x.Economy.Turn.Player != s.Turn || x.Economy.Turn.Sequence != g.TurnSerial {
		return errors.New("组合生产回合或开局不符")
	}
	if err := x.Board.validate(g); err != nil {
		return err
	}
	if err := x.Economy.validate(g, x.Fleet, x.Cargo); err != nil {
		return err
	}
	if x.Pirate == nil {
		return errors.New("组合海盗组件缺失")
	}
	if err := x.Pirate.validate(g, x.Board, x.Fleet, x.Cargo, x.Economy); err != nil {
		return err
	}
	if x.Pirate.Pending != nil && (k.Pending != nil || k.Event != nil || g.FreeRoads != 0 || g.Trade != nil) {
		return errors.New("海盗与其他城市回应不能同时进行")
	}
	if err := s.validateExplorerCityDevelopment(); err != nil {
		return err
	}
	if err := s.validateExplorerKnights(); err != nil {
		return err
	}
	if err := s.validateExplorerTradeProgress(); err != nil {
		return err
	}
	if err := s.validateExplorerPolitics(); err != nil {
		return err
	}
	turn := x.Economy.Turn
	freeRoads := s.Phase == "catan_roads"
	if g.FreeRoads < 0 || g.FreeRoads > 2 || freeRoads != (g.FreeRoads > 0) || freeRoads && (s.Finished || turn.Phase != "ready" || x.Cargo.Turn == nil || x.Cargo.Turn.Phase != "action" || g.ResumePhase != "catan_turn" || k.Pending != nil || k.Event != nil) {
		return errors.New("组合免费道路阶段或剩余次数无效")
	}
	if len(g.Dice) != 2 || turn.Phase != "roll" && turn.Dice != [2]int{g.Dice[0], g.Dice[1]} {
		return errors.New("组合事件与生产骰子不一致")
	}
	if k.Event != nil {
		event := k.Event
		if turn.Phase != "city" || turn.Dice != [2]int{event.Red, event.Yellow} || event.Face != k.EventDie || event.Face < 0 || event.Face > 5 || event.Production != 0 || event.Epidemic {
			return errors.New("组合城市事件骰子或后续生产无效")
		}
		for _, task := range event.Tasks {
			if task.Player < 0 || task.Player >= len(g.Players) || !slices.Contains([]string{"draw", "pillage", "defender_reward"}, task.Kind) || task.Kind == "draw" && (task.Track < 0 || task.Track > 2) {
				return errors.New("组合城市事件回应队列无效")
			}
		}
	}
	if q := k.Pending; q != nil {
		ending := s.Phase == "catan_progress_end" && q.Kind == "progress_discard" && q.Source == "explorer_movement"
		if len(q.Players) == 0 || !slices.Contains([]string{"pillage", "defender_reward", "progress_discard", "aqueduct", "metropolis", "knight_retreat", "guild_dues", "commercial_harbor", "diplomacy", "espionage", "sabotage", "wedding", "treason_remove", "treason_place"}, q.Kind) || !ending && s.Phase != "catan_"+q.Kind {
			return errors.New("组合城市回应与生产阶段不一致")
		}
		for _, p := range q.Players {
			if p < 0 || p >= len(g.Players) || g.Players[p].Eliminated {
				return errors.New("组合城市回应玩家无效")
			}
		}
		switch q.Kind {
		case "guild_dues", "commercial_harbor", "diplomacy", "espionage", "sabotage", "wedding", "treason_remove", "treason_place":
			if turn.Phase != "ready" || k.Event != nil || x.Cargo.Turn == nil || x.Cargo.Turn.Phase != "action" {
				return errors.New("组合进步牌回应不在行动阶段")
			}
		case "knight_retreat":
			if turn.Phase != "ready" || k.Event != nil || x.Cargo.Turn == nil || x.Cargo.Turn.Phase != "action" {
				return errors.New("组合骑士撤退不在行动阶段")
			}
		case "metropolis":
			if turn.Phase != "ready" || k.Event != nil || x.Cargo.Turn == nil || x.Cargo.Turn.Phase != "action" || len(q.Players) != 1 || q.Players[0] != s.Turn || q.Track < 0 || q.Track > 2 || k.Players[s.Turn].Improvements[q.Track] < 4 || len(g.cityMetropolisSites(s.Turn)) == 0 {
				return errors.New("组合大都会选择缺少有效建设或可用城市")
			}
			owner := g.cityMetropolisOwner(q.Track)
			if owner == s.Turn || owner >= 0 && k.Players[owner].Improvements[q.Track] >= k.Players[s.Turn].Improvements[q.Track] {
				return errors.New("组合大都会选择没有取得控制权")
			}
		case "aqueduct":
			if turn.Phase != "aqueduct" || k.Event != nil {
				return errors.New("组合引水渠与生产阶段不一致")
			}
		case "progress_discard":
			if ending {
				if len(q.Players) != 1 || q.Players[0] != s.Turn || len(k.Players[s.Turn].Progress) <= 4 || turn.Phase != "ready" || x.Cargo.Turn == nil || x.Cargo.Turn.Phase != "action" || k.Event != nil || g.Trade != nil {
					return errors.New("组合航行前弃进步牌缺少有效行动来源")
				}
			} else if q.Source != "" || turn.Phase != "city" || k.Event == nil {
				return errors.New("组合生产弃进步牌缺少有效事件来源")
			}
		default:
			if turn.Phase != "city" || k.Event == nil {
				return errors.New("组合城市事件回应缺少事件")
			}
		}
	} else if !s.Finished && !freeRoads && (turn.Phase == "city" || turn.Phase == "aqueduct" || s.Phase != s.catanExplorerPhase()) {
		return errors.New("组合生产存在未完成或错误接续")
	}
	return s.validateExplorerCityTrade()
}
