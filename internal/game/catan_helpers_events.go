package game

import (
	"errors"
	"slices"
)

// Event effects finish before production and its Hilda/Thorolf response. Keep
// the two queues exclusive, including when a helper is used before the draw.
func (s *State) validateEventHelpers() error {
	g := s.Catan
	if g.Explorer != nil {
		return s.validateExplorerHelpers()
	}
	if !g.Options.Helpers {
		if g.HelperPending != nil || len(g.HelperDisplay) != 0 || len(g.HelperExile) != 0 {
			return errors.New("事件牌存档中存在未启用的助手回应")
		}
		for _, p := range g.Players {
			if p.Helper != nil {
				return errors.New("事件牌存档中存在未启用的助手")
			}
		}
		return nil
	}
	if g.Two != nil || g.Rivers != nil || g.Attack != nil || g.BaseSetup != nil {
		return errors.New("该剧本的助手与事件牌三重组合尚未接入")
	}
	if _, err := NormalizeCatanOptions(g.Options); err != nil {
		return err
	}
	if len(g.Bank) != 5 {
		return errors.New("事件牌助手资源银行无效")
	}
	seen := map[int]bool{}
	add := func(id int) bool {
		if id < 1 || id > 12 || seen[id] {
			return false
		}
		seen[id] = true
		return true
	}
	for _, id := range g.HelperDisplay {
		if !add(id) {
			return errors.New("事件牌助手展示区重复或无效")
		}
	}
	missing := 0
	for _, p := range g.Players {
		h := p.Helper
		if h == nil {
			if !p.Eliminated {
				if !g.setup() {
					return errors.New("事件牌玩家缺少助手")
				}
				missing++
			}
			continue
		}
		if p.Eliminated || !add(h.ID) || h.AcquiredTurn > g.TurnSerial || h.UsedTurn > g.TurnSerial {
			return errors.New("事件牌助手归属或使用时机无效")
		}
	}
	count := 2 * len(g.Players)
	if g.Options.AllHelpers {
		count = 12
	}
	if len(seen)+missing != count {
		return errors.New("事件牌助手数量不守恒")
	}
	q := g.HelperPending
	if q == nil {
		if !s.Finished && s.Phase == "catan_helper" {
			return errors.New("事件牌助手回应缺失")
		}
		return nil
	}
	if g.setup() || s.Finished || s.Phase != "catan_helper" || g.CardEvent != nil || g.GoldPending != nil || q.Player < 0 || q.Player >= len(g.Players) || g.Players[q.Player].Eliminated {
		return errors.New("事件牌助手回应冲突或归属无效")
	}
	h := g.Players[q.Player].Helper
	if !slices.Contains([]string{"catan_turn", "catan_roll", "catan_discard", "catan_robber"}, q.Resume) {
		return errors.New("事件牌助手后续阶段无效")
	}
	if q.Resume != "catan_turn" && !(h.ID == 10 && q.Resume == "catan_roll") && !(h.ID == 5 && (q.Resume == "catan_discard" || q.Resume == "catan_robber")) {
		return errors.New("事件牌助手后续阶段与能力不符")
	}
	if q.Kind != "development" && len(q.Cards) != 0 || q.Kind != "resource" && q.Optional {
		return errors.New("事件牌助手回应数据无效")
	}
	if q.Kind == "exchange" {
		if h.UsedTurn != g.TurnSerial || h.AcquiredTurn >= g.TurnSerial {
			return errors.New("尚未使用助手不能翻面交换")
		}
		return nil
	}
	if !g.helperReady(q.Player, h.ID) {
		return errors.New("助手尚不可用或本回合已使用")
	}
	switch q.Kind {
	case "resource":
		if (h.ID != 3 && h.ID != 5) || q.Optional != (h.ID == 3) || sum(g.Bank[:5]) == 0 {
			return errors.New("事件牌助手资源回应无效")
		}
		if g.RevealedEvent == nil || !g.RevealedEvent.ProductionStarted || (g.RevealedEvent.Production == 7) != (h.ID == 5) {
			return errors.New("助手补偿与生产点数不符")
		}
	case "development":
		if h.ID != 6 || q.Player != s.Turn || len(q.Cards) < 1 || len(q.Cards) > 3 {
			return errors.New("助手发展卡候选无效")
		}
		for _, card := range q.Cards {
			if card < 0 || card > 4 {
				return errors.New("助手发展卡种类无效")
			}
		}
	case "leader":
		if h.ID != 7 || q.Player != s.Turn || q.Target < 0 || q.Target >= len(g.Players) || q.Target == q.Player || g.Players[q.Target].Eliminated || sum(g.Players[q.Target].Resources) == 0 {
			return errors.New("助手查看手牌目标无效")
		}
	default:
		return errors.New("未知事件牌助手回应")
	}
	return nil
}
