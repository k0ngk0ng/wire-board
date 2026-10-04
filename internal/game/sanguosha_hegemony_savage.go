package game

import "slices"

func (s *State) sgHegSavageEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "heg_savage_source":
		for _, who := range s.sgOrder(s.Turn) {
			if who != e.Actor && s.sgHegMayInvoke(who, "huoshou") {
				s.sgHegGate(who, "huoshou", SGEvent{Type: "heg_savage_claim", Actor: who, TrickID: e.TrickID}, SGEvent{})
				break
			}
		}
	case "heg_savage_claim":
		for j := range g.Queue {
			next := &g.Queue[j]
			if next.TrickID == e.TrickID && next.Kind == "savage_assault" {
				next.SavageSource = e.Actor + 1
			}
		}
		s.sgLog("%s 发动祸首，成为本次南蛮入侵的伤害来源", s.sgName(e.Actor))
	case "cleanup":
		if e.Kind != "savage_assault" || len(e.Cards) != 1 || sgCard(e.Cards[0]).Kind != "savage_assault" || !slices.Contains(g.Table, e.Cards[0]) {
			return false
		}
		g.Revealed = nil
		s.sgFinishCards(e.Cards)
		s.sgPush(SGEvent{Type: "heg_juxiang", Actor: e.Actor, Cards: clone(e.Cards)})
	case "heg_juxiang":
		if len(e.Cards) != 1 || !slices.Contains(g.Discard, e.Cards[0]) {
			return true
		}
		for _, who := range s.sgOrder(s.Turn) {
			if who != e.Actor && s.sgHegMayInvoke(who, "juxiang") {
				s.sgHegGate(who, "juxiang", SGEvent{Type: "heg_juxiang_gain", Actor: who, Cards: clone(e.Cards)}, SGEvent{})
				break
			}
		}
	case "heg_juxiang_gain":
		if !s.sgAlive(e.Actor) || len(e.Cards) != 1 {
			return true
		}
		id := e.Cards[0]
		// The reveal reward can reshuffle the discard pile. It may have drawn
		// this known card already; otherwise retrieve it from its new pile.
		if slices.Contains(g.Discard, id) || slices.Contains(g.Deck, id) {
			g.Discard, g.Deck = sgRemove(g.Discard, id), sgRemove(g.Deck, id)
			s.sgGain(e.Actor, []int{id})
			s.sgLog("%s 发动巨象，获得南蛮入侵", s.sgName(e.Actor))
		}
	default:
		return false
	}
	return true
}
