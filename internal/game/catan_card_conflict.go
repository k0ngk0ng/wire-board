package game

import (
	"errors"
	"slices"
)

func (g *Catan) beginCardConflict(start int) {
	q := g.CardEvent
	if g.CitiesKnights == nil && g.ArmyOwner >= 0 && g.ArmyOwner < len(g.Players) && !g.Players[g.ArmyOwner].Eliminated {
		// The holder acts even if this is somebody else's production turn or
		// the holder's face-up count is tied with another player's.
		q.Players = []int{g.ArmyOwner}
		return
	}
	leaders := g.cardEventLeaders("tournament", start)
	if len(leaders) == 1 {
		q.Players = leaders
		// The base fallback explicitly says "may"; C&K says "steals".
		q.Optional = g.CitiesKnights == nil
	}
}

// Conflict is a card effect, not a robber activation: neither adjacency nor
// Friendly Robber protection applies. In C&K, commodities are explicitly
// included, so eligible hand size is already public information.
func (g *Catan) cardTheftTargets(player int) []int {
	targets := []int{}
	for p := range g.Players {
		if p != player && !g.Players[p].Eliminated && sum(g.Players[p].Resources) > 0 {
			targets = append(targets, p)
		}
	}
	return targets
}

func (s *State) catanCardTheftChoice(player int, a Action) error {
	g := s.Catan
	if len(a.Take)+len(a.Give)+len(a.Cards) != 0 {
		return errors.New("事件偷牌只能选择对手，不能指定偷取的牌")
	}
	if a.Type == "catan_event_skip" && g.CardEvent.Kind == "conflict" && g.CardEvent.Optional {
		s.catanLog(player, "冲突：放弃本次可选偷牌")
		return nil
	}
	if a.Type != "catan_event_steal" || !slices.Contains(g.cardTheftTargets(player), a.Target) {
		return errors.New("请选择一位有手牌的在场对手")
	}
	hand := g.Players[a.Target].Resources
	n := catanRandom(sum(hand))
	for color, count := range hand {
		if n < count {
			hand[color]--
			g.Players[player].Resources[color]++
			break
		}
		n -= count
	}
	// Color is visible only through the participants' own resulting hands.
	s.catanLog(player, "%s：从玩家%d随机偷取1张牌", catanCardEventNames[g.CardEvent.Kind], a.Target+1)
	return nil
}
