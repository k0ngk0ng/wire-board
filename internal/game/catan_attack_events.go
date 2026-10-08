package game

import (
	"errors"
	"slices"
)

// Effect support shared by isolated resolution and the internal reference
// deck. New Year belongs to the deck lifecycle, never a selectable face effect.
func catanAttackEventSupported(kind string) bool {
	switch kind {
	case "beautiful_day", "conflict", "robber_attacks", "robber_flees",
		"earthquake", "epidemic", "plentiful_year", "calm_seas", "tournament",
		"good_neighbors", "helpful_neighbor", "trade_advantage":
		return true
	}
	return false
}

func (g *Catan) attackConflictLeader() int {
	counts := make([]int, len(g.Players))
	for _, k := range g.Attack.Knights {
		if k.Player >= 0 && k.Player < len(counts) && !g.Players[k.Player].Eliminated {
			counts[k.Player]++
		}
	}
	leader, most := -1, 0
	for p, count := range counts {
		if g.Players[p].Eliminated {
			continue
		}
		if count > most {
			leader, most = p, count
		} else if count == most {
			leader = -1
		}
	}
	if g.twoAttack() && most == 1 {
		for _, k := range g.Attack.Knights {
			if k.Player == catanAttackNeutral {
				return -1
			}
		}
	}
	return leader
}

func (s *State) validateCatanAttackEvent() error {
	g := s.Catan
	q, face := g.CardEvent, g.RevealedEvent
	if (q != nil) != (s.Phase == "catan_card_event") {
		return errors.New("蛮族进攻事件回应阶段无效")
	}
	if q == nil && face == nil {
		return nil
	}
	if g.setup() || face == nil || !catanAttackEventSupported(face.Kind) || face.Production < 2 || face.Production > 12 || face.Red != 0 || face.Face != 0 || face.RollID < 1 || face.RollID != g.RollID || face.Kind == "robber_attacks" && face.Production != 7 {
		return errors.New("蛮族进攻已揭示事件无效")
	}
	if q == nil {
		if !face.ProductionStarted {
			return errors.New("蛮族进攻事件缺少生产接续")
		}
		return nil
	}
	if s.Finished || g.Trade != nil || g.Attack.Pending != nil || g.Attack.EndPlan != nil || g.Paired != nil && g.Paired.Second || face.ProductionStarted || q.Kind != face.Kind || q.Production != face.Production || q.Red != 0 || q.Face != 0 || q.Optional || len(q.Players) == 0 || len(q.Players) > len(g.Players) {
		return errors.New("蛮族进攻事件回应内容无效")
	}
	if q.Kind != "good_neighbors" && len(q.Gifts) != 0 || q.Kind != "helpful_neighbor" && len(q.Targets) != 0 {
		return errors.New("事件包含不适用的交牌内容")
	}
	// A remaining response queue follows active-player order, without repeats.
	// Choices cannot change any remaining player's ranking or road eligibility.
	last := -1
	for _, p := range q.Players {
		if p < 0 || p >= len(g.Players) || g.Players[p].Eliminated {
			return errors.New("事件回应者无效")
		}
		offset := (p - s.Turn + len(g.Players)) % len(g.Players)
		if offset <= last {
			return errors.New("事件回应顺序无效")
		}
		last = offset
	}
	switch q.Kind {
	case "conflict", "trade_advantage":
		leader := g.LongestOwner
		if q.Kind == "conflict" {
			leader = g.attackConflictLeader()
		}
		if len(q.Players) != 1 || q.Players[0] != leader || len(g.cardTheftTargets(leader)) == 0 {
			return errors.New("蛮族进攻偷牌回应者无效")
		}
	case "earthquake":
		for _, p := range q.Players {
			if len(g.earthquakeRoads(p)) == 0 {
				return errors.New("地震回应者没有可损坏的道路")
			}
		}
	case "plentiful_year", "calm_seas", "tournament":
		if sum(g.Bank) == 0 {
			return errors.New("资源奖励没有可领取的资源")
		}
		if q.Kind != "plentiful_year" {
			leaders := g.cardEventLeaders(q.Kind, s.Turn)
			for _, p := range q.Players {
				if !slices.Contains(leaders, p) {
					return errors.New("资源奖励回应者不符合条件")
				}
			}
		}
	case "helpful_neighbor":
		copy := *g
		copy.CardEvent = &CatanCardEvent{}
		copy.beginHelpfulNeighbor(s.Turn)
		if len(q.Targets) == 0 || !slices.Equal(q.Targets, copy.CardEvent.Targets) {
			return errors.New("援助邻居接收者无效")
		}
		for _, p := range q.Players {
			if !slices.Contains(copy.CardEvent.Players, p) {
				return errors.New("援助邻居回应者无效")
			}
		}
	case "good_neighbors":
		return g.validateAttackNeighborGifts(s.Turn)
	default:
		return errors.New("该事件不能保留待选回应")
	}
	return nil
}

func (g *Catan) validateAttackNeighborGifts(start int) error {
	// Hands remain unchanged while choices are pending, so the original gift
	// participants/left neighbors can be verified without storing secret copies.
	copy := *g
	copy.CardEvent = &CatanCardEvent{}
	copy.beginNeighborGifts(start)
	q := g.CardEvent
	if len(q.Gifts) != len(copy.CardEvent.Gifts) {
		return errors.New("好邻居交牌人数无效")
	}
	remaining := []int{}
	for i, gift := range q.Gifts {
		want := copy.CardEvent.Gifts[i]
		if gift.From != want.From || gift.To != want.To || gift.Color < -1 || gift.Color >= 5 {
			return errors.New("好邻居交牌座位或资源无效")
		}
		if gift.Color == -1 {
			remaining = append(remaining, gift.From)
		} else if len(remaining) > 0 || g.Players[gift.From].Resources[gift.Color] == 0 {
			return errors.New("好邻居选择顺序或原有手牌无效")
		}
	}
	if !slices.Equal(remaining, q.Players) {
		return errors.New("好邻居待选回应不一致")
	}
	return nil
}
