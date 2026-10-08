package game

import (
	"errors"
	"slices"
)

func (s *State) validateExplorerCityInventory() error {
	g, k := s.Catan, s.Catan.CitiesKnights
	if k.Layout != "variable" || k.Rules != catanCitiesKnightsRules(len(g.Players)) || k.Invasions < 0 || k.BarbarianPosition < 0 || k.BarbarianPosition > catanBarbarianDistance || k.RobberStart != -1 || k.Chase != "" || k.EventDie < -1 || k.EventDie > 5 || g.RollID == 0 && k.EventDie != -1 || g.RollID > 0 && k.EventDie < 0 {
		return errors.New("组合城市规则、事件骰或蛮族记录无效")
	}
	return s.validateCityProgressInventory()
}

// Shared stock and privacy invariants; map/turn rules remain recipe-specific.
func (s *State) validateCityProgressInventory() error {
	g, k := s.Catan, s.Catan.CitiesKnights
	counts := make([]int, len(catanProgressRules))
	totalCards := 0
	for _, rule := range catanProgressRules {
		totalCards += rule.Count
	}
	for track, deck := range k.ProgressDecks {
		for _, id := range deck {
			if id < 0 || id >= len(counts) || catanProgressRules[id].Track != track {
				return errors.New("组合进步牌堆的卡牌或轨道无效")
			}
			counts[id]++
		}
	}
	for player, p := range k.Players {
		if p.DefenderPoints < 0 || p.ProgressPoints != len(p.PublicProgress) {
			return errors.New("组合防御分或公开进步牌分数无效")
		}
		for _, id := range p.Progress {
			if id < 0 || id >= len(counts) || catanProgressRules[id].Victory {
				return errors.New("组合私有进步手牌包含无效或公开胜利牌")
			}
			counts[id]++
		}
		for _, id := range p.PublicProgress {
			if id < 0 || id >= len(counts) || !catanProgressRules[id].Victory {
				return errors.New("组合公开进步牌不是胜利牌")
			}
			counts[id]++
		}
		if player != s.Turn && len(p.Progress) > 4 && (len(p.Progress) != 5 || k.Pending == nil || k.Pending.Kind != "progress_discard" || !slices.Equal(k.Pending.Players, []int{player}) || k.Event == nil) {
			return errors.New("非行动玩家进步牌超限必须立即回应")
		}
	}
	if g.twoSeafarersKnights() && g.tribe() != nil {
		if err := g.validateTribeProgress(); err != nil {
			return err
		}
		for _, reward := range g.tribe().Development {
			counts[reward.Card]++
		}
	}
	for id, rule := range catanProgressRules {
		if counts[id] != rule.Count {
			return errors.New("组合进步牌总库存不守恒")
		}
	}
	if len(k.ProgressEvents) > 18 || uint64(len(k.ProgressEvents)) > k.ProgressEventID || k.ProgressEventID > 0 && len(k.ProgressEvents) == 0 {
		return errors.New("组合进步牌动画序列缺失或超限")
	}
	for i, event := range k.ProgressEvents {
		if event.ID != k.ProgressEventID-uint64(len(k.ProgressEvents)-1-i) || event.Player < 0 || event.Player >= len(g.Players) || event.Count < 1 {
			return errors.New("组合进步牌动画玩家、序号或数量无效")
		}
		switch event.Kind {
		case "draw", "play":
			if event.Other != -1 || event.Count != 1 || event.Track < 0 || event.Track > 2 || event.Kind == "play" && event.Card == nil {
				return errors.New("组合抽牌/用牌动画字段无效")
			}
			if event.Card != nil {
				id := *event.Card
				if id < 0 || id >= len(counts) || catanProgressRules[id].Track != event.Track || (event.Kind == "draw") != catanProgressRules[id].Victory {
					return errors.New("组合动画泄露非公开牌或公开牌标记错误")
				}
			}
		case "return":
			if event.Other != -1 || event.Track != -1 || event.Card != nil || event.Count > totalCards {
				return errors.New("组合弃牌动画不能公开卡牌种类")
			}
		case "transfer":
			if event.Other < 0 || event.Other >= len(g.Players) || event.Other == event.Player || event.Track != -1 || event.Count != 1 || event.Card != nil {
				return errors.New("组合转牌动画不能公开私有牌种类")
			}
		default:
			return errors.New("组合进步牌动画类型无效")
		}
	}
	return nil
}

func (s *State) validateExplorerCityEventQueue() error {
	g, k := s.Catan, s.Catan.CitiesKnights
	e, q := k.Event, k.Pending
	if e == nil {
		if k.BarbarianPosition == catanBarbarianDistance {
			return errors.New("蛮族到达却没有攻击事件")
		}
		return nil
	}
	if q != nil && q.Source != "" {
		return errors.New("组合生产响应不能夹带行动阶段来源")
	}
	if e.Attack != (k.BarbarianPosition == catanBarbarianDistance) || e.Attack && e.Face < 3 {
		return errors.New("组合蛮族攻击与事件骰不一致")
	}
	previous := -1
	for _, task := range e.Tasks {
		offset := (task.Player - s.Turn + len(g.Players)) % len(g.Players)
		if offset <= previous || g.Players[task.Player].Eliminated {
			return errors.New("组合事件队列顺序、重复玩家或离场者无效")
		}
		previous = offset
		if e.Attack {
			if task.Kind != e.Tasks[0].Kind || task.Kind != "pillage" && task.Kind != "defender_reward" || task.Kind == "pillage" && len(g.pillageSites(task.Player)) == 0 {
				return errors.New("蛮族事件包含无效城市响应")
			}
		} else if task.Kind != "draw" || e.Face > 2 || task.Track != e.Face || k.Players[task.Player].Improvements[task.Track] < 1 || e.Red > k.Players[task.Player].Improvements[task.Track]+1 {
			return errors.New("组合进步抽牌队列缺少对应改良")
		}
	}
	if q == nil {
		if !s.Finished {
			return errors.New("组合事件暂停但没有待回应玩家")
		}
	} else if q.Kind == "pillage" || q.Kind == "defender_reward" {
		if len(e.Tasks) == 0 || len(q.Players) != 1 || q.Kind != e.Tasks[0].Kind || q.Players[0] != e.Tasks[0].Player {
			return errors.New("组合待回应玩家与事件队首不符")
		}
	} else if q.Kind == "progress_discard" {
		if len(q.Players) != 1 || q.Players[0] == s.Turn || len(k.Players[q.Players[0]].Progress) != 5 {
			return errors.New("组合事件弃牌没有实际超限玩家")
		}
	} else {
		return errors.New("组合生产事件夹带无关响应")
	}
	return nil
}
