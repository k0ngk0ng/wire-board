package game

import (
	"errors"
	"slices"
)

func (s *State) sgHegLuoshenEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "luoshen", "heg_luoshen":
		if !s.sgHegMayInvoke(e.Actor, "heg_luoshen") || len(g.Deck)+len(g.Discard) == 0 {
			if e.Type == "heg_luoshen" {
				s.sgHegLuoshenCollect(e.Actor)
			}
			return e.Type == "heg_luoshen"
		}
		s.sgAsk(e.Actor, "heg_luoshen", "洛神：是否继续判定？结束后统一获得仍在处理区的黑牌", e)
		g.Pending.Choices = []string{"yes", "pass"}
	case "heg_luoshen_tiandu":
		if s.sgHegMayInvoke(e.Actor, "tiandu") && slices.Contains(g.Table, e.Aux) {
			s.sgAsk(e.Actor, "heg_luoshen_tiandu", "天妒：是否现在获得这张判定牌？放弃则留待洛神结束获得", e)
			g.Pending.Choices = []string{"yes", "pass"}
			g.Pending.Cards = []int{e.Aux}
		}
	case "heg_luoshen_collect":
		s.sgHegLuoshenCollect(e.Actor)
	default:
		return false
	}
	return true
}

func (s *State) sgHegLuoshenCollect(i int) {
	g := s.Sanguosha
	h := g.Players[i].Hegemony
	ids := []int{}
	for _, id := range h.Luoshen {
		if slices.Contains(g.Table, id) {
			s.sgTakeTable(id)
			ids = append(ids, id)
		}
	}
	h.Luoshen = nil
	s.sgGain(i, ids)
}

func (s *State) sgHegLuoshenRespond(i int, a Action, q SGPrompt) (bool, error) {
	if q.Kind != "heg_luoshen" && q.Kind != "heg_luoshen_tiandu" {
		return false, nil
	}
	if a.Choice != "yes" && a.Choice != "pass" {
		return true, errors.New("请选择发动或放弃")
	}
	if q.Kind == "heg_luoshen_tiandu" {
		if a.Choice == "yes" && slices.Contains(s.Sanguosha.Table, q.Event.Aux) {
			s.sgTakeTable(q.Event.Aux)
			s.sgGain(i, []int{q.Event.Aux})
		}
	} else if a.Choice == "yes" {
		s.sgPush(SGEvent{Type: "judge", Actor: i, Kind: "heg_luoshen"})
	} else {
		s.sgHegLuoshenCollect(i)
	}
	return true, nil
}
