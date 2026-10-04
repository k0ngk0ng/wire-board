package game

import (
	"errors"
	"slices"
)

func (s *State) sgHegEffect(e SGEvent) bool {
	if !s.sgHegemony() {
		return false
	}
	g := s.Sanguosha
	switch e.Kind {
	case "await_exhausted":
		if g.Hegemony.AwaitEffects == nil {
			g.Hegemony.AwaitEffects = map[int][]int{}
		}
		g.Hegemony.AwaitEffects[e.TrickID] = append(g.Hegemony.AwaitEffects[e.TrickID], e.Target)
		s.sgDraw(e.Target, 2)
	case "known_both":
		if !s.sgAlive(e.Actor) {
			return true
		}
		p := g.Players[e.Target]
		choices := []string{}
		if len(p.Hand) > 0 {
			choices = append(choices, "hand")
		}
		if !p.Hegemony.Shown[0] {
			choices = append(choices, "head")
		}
		if !p.Hegemony.Shown[1] {
			choices = append(choices, "deputy")
		}
		if len(choices) > 0 {
			s.sgAsk(e.Actor, "heg_known_both", "知己知彼：选择要私下查看的区域", e)
			g.Pending.Choices = choices
		}
	case "befriend_attacking":
		s.sgPush(SGEvent{Type: "draw", Actor: e.Target, Amount: 1}, SGEvent{Type: "draw", Actor: e.Actor, Amount: 3})
	default:
		return false
	}
	return true
}

func (s *State) sgHegCardEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "heg_await_discard":
		if s.sgAlive(e.Target) && slices.Contains(g.Hegemony.AwaitEffects[e.TrickID], e.Target) {
			e.Amount = min(2, len(g.Players[e.Target].Hand)+len(g.Players[e.Target].Equip))
			if e.Amount > 0 {
				s.sgAsk(e.Target, "heg_await_discard", "以逸待劳：弃置两张手牌或装备（不足时全部弃置）", e)
			}
		}
	case "heg_await_cleanup":
		delete(g.Hegemony.AwaitEffects, e.TrickID)
	case "heg_triblade":
		if !s.sgAlive(e.Actor) || !s.sgAlive(e.Target) || s.sgWeapon(e.Actor) != "triblade" || len(g.Players[e.Actor].Hand) == 0 {
			return true
		}
		targets := []int{}
		for _, who := range s.sgOrder(s.Turn) {
			if who != e.Actor && who != e.Target && s.sgDistance(e.Target, who) == 1 {
				targets = append(targets, who)
			}
		}
		if len(targets) > 0 {
			s.sgAsk(e.Actor, "heg_triblade", "三尖两刃刀：可弃一张手牌，对受伤角色距离一以内的另一角色造成一点伤害", e)
			g.Pending.Targets = targets
		}
	default:
		return false
	}
	return true
}

func (s *State) sgHegCardRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	e := q.Event
	switch q.Kind {
	case "heg_await_discard":
		if err := s.sgValidateCards(i, a.Cards, e.Amount, false); err != nil {
			return true, err
		}
		s.sgDiscard(i, a.Cards)
	case "heg_known_both":
		if !slices.Contains(q.Choices, a.Choice) {
			return true, errors.New("请选择知己知彼的可查看区域")
		}
		s.sgAsk(i, "heg_intel", "知己知彼：这些信息仅自己可见，确认后继续", e)
		g.Pending.Choices = []string{"ok"}
		if a.Choice == "hand" {
			g.Pending.Cards = clone(g.Players[e.Target].Hand)
			g.Pending.Event.Kind = "hand"
		} else {
			slot := 0
			if a.Choice == "deputy" {
				slot = 1
			}
			g.Pending.Event.Kind = a.Choice
			h := g.Players[i].Hegemony
			if h.KnownGenerals == nil {
				h.KnownGenerals = map[int][2]string{}
			}
			known := h.KnownGenerals[e.Target]
			known[slot] = s.sgHegGeneralIDs(e.Target)[slot]
			h.KnownGenerals[e.Target] = known
		}
		s.sgLog("%s 私下查看了 %s 的%s", s.sgName(i), s.sgName(e.Target), map[string]string{"hand": "手牌", "head": "主将", "deputy": "副将"}[a.Choice])
	case "heg_intel":
		if a.Choice != "ok" && a.Choice != "pass" {
			return true, errors.New("请确认已查看")
		}
	case "heg_triblade":
		if a.Choice == "pass" {
			return true, nil
		}
		if len(a.Targets) != 1 || !slices.Contains(q.Targets, a.Targets[0]) || !s.sgAlive(a.Targets[0]) {
			return true, errors.New("请选择三尖两刃刀的合法目标")
		}
		if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
			return true, err
		}
		s.sgPush(SGEvent{Type: "damage", Actor: i, Target: a.Targets[0], Amount: 1, Kind: "triblade"})
		s.sgDiscard(i, a.Cards)
	default:
		return false, nil
	}
	return true, nil
}

func (s *State) sgHegJizhi(i int, kind string, ids []int) {
	if !s.sgHegemony() || !sgIsTrick(kind) || sgIsDelayed(kind) {
		return
	}
	// A physical card (or one-card virtual copy retaining its kind) qualifies.
	// Qixi/Huoji/Duoshi/Luanji conversions do not acquire Jizhi draws.
	if len(ids) == 0 || len(ids) == 1 && sgCard(ids[0]).Kind == kind {
		s.sgPush(SGEvent{Type: "optional_draw", Actor: i, Kind: "heg_jizhi", Amount: 1})
	}
}

func (s *State) sgHegCounterScope(i int, kind, choice string, e *SGEvent) error {
	if !s.sgHegemony() {
		return nil
	}
	if choice != "" && choice != "single" && choice != "faction" {
		return errors.New("请选择无懈可击的范围")
	}
	if choice != "faction" {
		return nil
	}
	if kind != "heg_nullification" || e.CounterDepth > 0 || e.Aux == -2 || sgIsDelayed(e.Kind) {
		return errors.New("此响应不能使用国无懈的势力范围")
	}
	e.HegFactionCounter = true
	return nil
}

func (s *State) sgHegCancelFaction(e SGEvent) {
	if !s.sgHegemony() || !e.HegFactionCounter || e.TrickID == 0 {
		return
	}
	g := s.Sanguosha
	g.Queue = slices.DeleteFunc(g.Queue, func(next SGEvent) bool {
		return next.Type == "null_window" && next.TrickID == e.TrickID && s.sgHegFriend(e.Target, next.Target)
	})
	s.sgLog("国无懈同时抵消这张锦囊对同势力后续目标的效果")
}
