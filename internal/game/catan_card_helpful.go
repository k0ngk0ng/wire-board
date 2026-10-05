package game

import (
	"errors"
	"slices"
)

func (g *Catan) beginHelpfulNeighbor(start int) {
	q := g.CardEvent
	best := -1
	for p := range g.Players {
		if !g.Players[p].Eliminated {
			best = max(best, g.Players[p].Score-g.hiddenVictoryPoints(p))
		}
	}
	for offset := range len(g.Players) {
		p := (start + offset) % len(g.Players)
		if g.Players[p].Eliminated {
			continue
		}
		score := g.Players[p].Score - g.hiddenVictoryPoints(p)
		if score < best {
			q.Targets = append(q.Targets, p)
		} else if sum(g.Players[p].Resources) > 0 {
			q.Players = append(q.Players, p)
		}
	}
	if len(q.Targets) == 0 {
		q.Players = []int{} // Everyone tied: there is no lower-score recipient.
	}
}

func (s *State) catanHelpfulNeighborChoice(player int, a Action) error {
	g := s.Catan
	q := g.CardEvent
	if a.Type != "catan_event_gift" || !slices.Contains(q.Targets, a.Target) || a.Target < 0 || a.Target >= len(g.Players) || g.Players[a.Target].Eliminated ||
		!g.cardBundle(a.Give) || sum(a.Give) != 1 || !catanHas(g.Players[player].Resources, a.Give) {
		return errors.New("请选择一位分数较低的玩家，并交给他自己的一张资源或商品")
	}
	// Only highest-score players give; recipients cannot themselves become
	// givers, and resource transfers do not change VP. Apply one gift now.
	catanMove(g.Players[player].Resources, g.Players[a.Target].Resources, a.Give)
	s.catanLog(player, "援助邻居：向玩家%d交出1张牌", a.Target+1)
	return nil
}
