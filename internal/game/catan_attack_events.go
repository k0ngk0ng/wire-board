package game

import "errors"

// Only these internally verified effects are accepted so far. This does not
// enable a room option or attach the unverified event catalogue/deck.
func catanAttackEventSupported(kind string) bool {
	switch kind {
	case "beautiful_day", "conflict", "robber_attacks", "robber_flees":
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
	if s.Finished || g.Trade != nil || g.Attack.Pending != nil || g.Attack.EndPlan != nil || g.Paired != nil && g.Paired.Second || face.ProductionStarted || q.Kind != "conflict" || q.Kind != face.Kind || q.Production != face.Production || q.Red != 0 || q.Face != 0 || q.Optional || len(q.Gifts) != 0 || len(q.Targets) != 0 || len(q.Players) != 1 {
		return errors.New("蛮族进攻冲突回应内容无效")
	}
	p := q.Players[0]
	if p < 0 || p >= len(g.Players) || p != g.attackConflictLeader() || len(g.cardTheftTargets(p)) == 0 {
		return errors.New("蛮族进攻冲突回应者无效")
	}
	return nil
}
