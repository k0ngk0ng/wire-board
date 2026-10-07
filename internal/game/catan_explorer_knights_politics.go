package game

import (
	"errors"
	"slices"
)

func (s *State) validateExplorerPolitics() error {
	g, k := s.Catan, s.Catan.CitiesKnights
	q := k.Pending
	if q == nil || !slices.Contains([]string{"diplomacy", "espionage", "sabotage", "wedding", "treason_remove", "treason_place"}, q.Kind) {
		return nil
	}
	if len(q.Players) == 0 || q.Ship || q.Source != "" || g.Trade != nil {
		return errors.New("组合政治进步牌回应记录无效")
	}
	for _, player := range q.Players {
		if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated {
			return errors.New("组合政治进步牌回应玩家无效")
		}
	}
	actor := q.Players[0]
	switch q.Kind {
	case "diplomacy":
		if len(q.Players) != 1 || actor != s.Turn || len(g.diplomacyPlacements(actor)) == 0 {
			return errors.New("外交缺少有效免费道路重建位置")
		}
	case "espionage":
		if len(q.Players) != 1 || actor != s.Turn || !g.politicsOpponent(actor, q.Target) || len(k.Players[q.Target].Progress) == 0 {
			return errors.New("间谍缺少有效对手进步手牌")
		}
		for _, card := range k.Players[q.Target].Progress {
			if card < 0 || card >= len(catanProgressRules) || catanProgressRules[card].Victory {
				return errors.New("间谍目标进步手牌损坏")
			}
		}
	case "wedding", "sabotage":
		if q.Target != s.Turn {
			return errors.New("婚礼或破坏的使用者不是当前玩家")
		}
		last := 0
		own := g.Players[s.Turn].Score - g.hiddenVictoryPoints(s.Turn)
		for _, player := range q.Players {
			distance := (player - s.Turn + len(g.Players)) % len(g.Players)
			score := g.Players[player].Score - g.hiddenVictoryPoints(player)
			count := sum(g.Players[player].Resources)
			if distance <= last || q.Kind == "wedding" && (score <= own || count == 0) || q.Kind == "sabotage" && (score < own || count < 2) {
				return errors.New("婚礼或破坏队列次序、分数或手牌不符")
			}
			last = distance
		}
	case "treason_remove":
		if len(q.Players) != 1 || q.Target != s.Turn || !g.politicsOpponent(s.Turn, actor) || !slices.ContainsFunc(k.Knights, func(n CatanKnight) bool { return n.Owner == actor }) {
			return errors.New("叛变缺少对手可移除的骑士")
		}
	case "treason_place":
		n := q.Knight
		if len(q.Players) != 1 || actor != s.Turn || n == nil || !g.politicsOpponent(actor, n.Owner) || n.Strength < 1 || n.Strength > 3 || !g.explorerKnightSite(n.Vertex) || g.Vertices[n.Vertex].Level != 0 || g.knightAt(n.Vertex) != nil || n.ActivatedAt > k.ActionSerial || n.PromotedAt > k.ActionSerial || len(g.treasonPlacements(actor, n.Strength)) == 0 {
			return errors.New("叛变被移除棋子记录或免费放置位置无效")
		}
	}
	return nil
}
