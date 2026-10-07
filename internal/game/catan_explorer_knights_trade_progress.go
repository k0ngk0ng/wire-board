package game

import (
	"errors"
	"slices"
)

// Mandatory card effects are not negotiated player trades, including during
// the second paired action. Fleets expire with that action segment; Merchant
// ownership persists until a later card transfers it.
func (s *State) validateExplorerTradeProgress() error {
	g, k := s.Catan, s.Catan.CitiesKnights
	if merchant := k.Merchant; merchant != nil {
		if merchant.Owner < 0 || merchant.Owner >= len(g.Players) || g.Players[merchant.Owner].Eliminated || !slices.Contains(g.merchantTiles(merchant.Owner), merchant.Tile) {
			return errors.New("组合商人缺少有效玩家或本人建筑相邻地块")
		}
	}
	powers := k.TradePowers
	if powers != nil {
		if powers.Player != s.Turn || len(powers.Fleets)+len(powers.Harbors) < 1 || len(powers.Fleets) > 2 || len(powers.Harbors) > 2 || g.Explorer.Economy.Turn.Phase != "ready" || g.Explorer.Cargo.Turn == nil {
			return errors.New("组合贸易进步牌额度或所属行动段无效")
		}
		for i, color := range powers.Fleets {
			if color < 0 || color >= 8 || slices.Contains(powers.Fleets[:i], color) {
				return errors.New("组合商船队种类无效或重复")
			}
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
