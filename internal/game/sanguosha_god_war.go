package game

import (
	"errors"
	"slices"
)

func (s *State) sgGodWarSkill(i int, a Action) (bool, error) {
	g := s.Sanguosha
	p := &g.Players[i]
	switch a.Skill {
	case "yeyan":
		if p.Marks["yeyan"] > 0 {
			return true, errors.New("业炎整局只能发动一次")
		}
		seen := map[int]bool{}
		for _, to := range a.Targets {
			if !s.sgAlive(to) || seen[to] {
				return true, errors.New("业炎需要不同的存活目标")
			}
			seen[to] = true
		}
		if len(a.Cards) == 0 {
			if len(a.Targets) < 1 || len(a.Targets) > 3 {
				return true, errors.New("小业炎选择一至三名角色")
			}
		} else {
			if len(a.Targets) < 1 || len(a.Targets) > 2 || s.sgValidateCards(i, a.Cards, 4, true) != nil {
				return true, errors.New("大业炎需要四张手牌，以及一或两名角色")
			}
			suits := map[int]bool{}
			for _, id := range a.Cards {
				suits[s.sgCardFor(i, id).Suit] = true
			}
			if len(suits) != 4 {
				return true, errors.New("大业炎需要四种花色各一张")
			}
		}
		s.sgGodAddMark(i, "yeyan", 1)
		es := []SGEvent{}
		if len(a.Cards) > 0 {
			es = append(es, SGEvent{Type: "lose_hp", Target: i, Amount: 3})
		}
		// In the two-target version the first selected target receives two damage.
		damage := map[int]int{}
		for j, to := range a.Targets {
			n := 1
			if len(a.Cards) > 0 && j == 0 {
				n = 4 - len(a.Targets)
			}
			damage[to] = n
		}
		for _, to := range s.sgOrder(s.Turn) {
			if n := damage[to]; n > 0 {
				es = append(es, SGEvent{Type: "damage", Actor: i, Target: to, Kind: "yeyan", Nature: "fire", Amount: n})
			}
		}
		s.sgPush(es...)
		s.sgDiscard(i, a.Cards)
		s.sgLog("%s 发动业炎", s.sgName(i))
	case "wuqian":
		if p.Marks["wrath"] < 2 || len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) {
			return true, errors.New("无前需要2暴怒，并指定另一名角色")
		}
		s.sgGodAddMark(i, "wrath", -2)
		if !slices.Contains(p.Temporary, "wushuang") {
			p.Temporary = append(p.Temporary, "wushuang")
		}
		if !slices.Contains(p.ArmorOff, a.Targets[0]) {
			p.ArmorOff = append(p.ArmorOff, a.Targets[0])
		}
		s.sgLog("%s 发动无前，获得本回合无双，%s 的防具本回合失效", s.sgName(i), s.sgName(a.Targets[0]))
	case "shenfen":
		if p.Marks["wrath"] < 6 || p.Used["shenfen"] > 0 {
			return true, errors.New("神愤每阶段限一次，需要6暴怒")
		}
		s.sgGodAddMark(i, "wrath", -6)
		p.Used["shenfen"]++
		es := []SGEvent{}
		targets := sgRemove(s.sgOrder(s.Turn), i)
		for _, to := range targets {
			es = append(es, SGEvent{Type: "damage", Actor: i, Target: to, Kind: "shenfen", Amount: 1})
		}
		for _, to := range targets {
			es = append(es, SGEvent{Type: "shenfen_equip", Actor: i, Target: to})
		}
		for _, to := range targets {
			es = append(es, SGEvent{Type: "shenfen_hand", Actor: i, Target: to})
		}
		es = append(es, SGEvent{Type: "god_flip", Actor: i})
		s.sgPush(es...)
		s.sgLog("%s 发动神愤", s.sgName(i))
	default:
		return false, nil
	}
	return true, nil
}
func (s *State) sgGodWarEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "qinyin":
		if g.DiscardPhase && g.DiscardedOwn >= 2 && s.sgHas(e.Actor, "qinyin") {
			s.sgAsk(e.Actor, "qinyin", "琴音：令所有角色回复1体力或失去1体力，也可放弃", e)
		}
	case "wumou":
		if !s.sgHas(e.Actor, "wumou") {
			return true
		}
		if g.Players[e.Actor].Marks["wrath"] > 0 {
			s.sgAsk(e.Actor, "wumou", "无谋：弃1暴怒或失去1体力", e)
		} else {
			s.sgPush(SGEvent{Type: "lose_hp", Target: e.Actor, Amount: 1})
		}
	case "shenfen_equip":
		if s.sgAlive(e.Target) {
			s.sgDiscard(e.Target, clone(g.Players[e.Target].Equip))
		}
	case "shenfen_hand":
		if !s.sgAlive(e.Target) {
			return true
		}
		p := g.Players[e.Target]
		if len(p.Hand) <= 4 {
			s.sgDiscard(e.Target, clone(p.Hand))
		} else {
			e.Amount = 4
			s.sgAsk(e.Target, "shenfen_hand", "神愤：弃置四张手牌", e)
		}
	case "god_flip":
		if s.sgAlive(e.Actor) {
			g.Players[e.Actor].Flipped = !g.Players[e.Actor].Flipped
			s.sgLog("%s 将武将牌翻面", s.sgName(e.Actor))
		}
	default:
		return false
	}
	return true
}
func (s *State) sgGodWarRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	switch q.Kind {
	case "wumou":
		if a.Choice == "wrath" && g.Players[i].Marks["wrath"] > 0 {
			s.sgGodAddMark(i, "wrath", -1)
			s.sgLog("%s 无谋弃1暴怒", s.sgName(i))
		} else if a.Choice == "hp" {
			s.sgPush(SGEvent{Type: "lose_hp", Target: i, Amount: 1})
		} else {
			return true, errors.New("请选择弃暴怒或失去体力")
		}
	case "qinyin":
		if a.Choice == "pass" {
			return true, nil
		}
		if a.Choice != "heal" && a.Choice != "lose" {
			return true, errors.New("请选择全体回复或失去体力")
		}
		es := []SGEvent{}
		for _, to := range s.sgOrder(s.Turn) {
			kind := "heal"
			if a.Choice == "lose" {
				kind = "lose_hp"
			}
			es = append(es, SGEvent{Type: kind, Actor: to, Target: to, Amount: 1})
		}
		s.sgPush(es...)
		s.sgLog("%s 发动琴音：所有角色%s1体力", s.sgName(i), map[string]string{"heal": "回复", "lose": "失去"}[a.Choice])
	case "shenfen_hand":
		if err := s.sgValidateCards(i, a.Cards, 4, true); err != nil {
			return true, err
		}
		s.sgDiscard(i, a.Cards)
	default:
		return false, nil
	}
	return true, nil
}
