package game

import (
	"errors"
	"slices"
)

// Mandatory card effects are not negotiated player trades, including during
// the second paired action. Bank-related Merchant/Fleet remain gated separately.
func (s *State) validateExplorerTradeProgress() error {
	g, k := s.Catan, s.Catan.CitiesKnights
	powers := k.TradePowers
	if powers != nil {
		if powers.Player != s.Turn || len(powers.Fleets) != 0 || len(powers.Harbors) < 1 || len(powers.Harbors) > 2 || g.Explorer.Economy.Turn.Phase != "ready" || g.Explorer.Cargo.Turn == nil {
			return errors.New("组合贸易进步牌额度或所属行动段无效")
		}
		for _, remaining := range powers.Harbors {
			seen := map[int]bool{}
			for _, p := range remaining {
				if p < 0 || p >= len(g.Players) || p == powers.Player || seen[p] {
					return errors.New("商业港剩余玩家无效或重复")
				}
				seen[p] = true
			}
		}
	}
	q := k.Pending
	if q == nil || q.Kind != "guild_dues" && q.Kind != "commercial_harbor" {
		return nil
	}
	if len(q.Players) != 1 || q.Players[0] < 0 || q.Players[0] >= len(g.Players) || q.Target < 0 || q.Target >= len(g.Players) || q.Target == q.Players[0] || g.Players[q.Target].Eliminated || g.Players[q.Players[0]].Eliminated || g.Trade != nil {
		return errors.New("组合贸易进步牌回应玩家或目标无效")
	}
	if q.Kind == "guild_dues" {
		if q.Players[0] != s.Turn || !slices.Contains(g.guildDuesTargets(s.Turn), q.Target) || sum(g.Players[q.Target].Resources) == 0 {
			return errors.New("行会征费须由当前玩家查看高分对手的非空手牌")
		}
	} else if q.Target != s.Turn || q.Color < 0 || q.Color >= 5 || g.Players[q.Target].Resources[q.Color] < 1 || sum(g.Players[q.Players[0]].Resources[5:]) == 0 || powers == nil || !slices.ContainsFunc(powers.Harbors, func(remaining []int) bool { return !slices.Contains(remaining, q.Players[0]) }) {
		return errors.New("商业港回应缺少有效交换资源、商品或使用记录")
	}
	return nil
}
