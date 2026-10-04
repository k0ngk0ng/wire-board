package game

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

func (s *State) sgJieSkill(i int, a Action) (bool, error) {
	if !slices.Contains([]string{"jie_rende", "yijue", "jie_kurou", "jie_fanjian", "jie_guose", "chuli", "jie_lijian"}, a.Skill) {
		return false, nil
	}
	g := s.Sanguosha
	p := &g.Players[i]
	if p.Used[a.Skill] > 0 {
		return true, errors.New("本阶段已发动此技能")
	}
	for _, to := range a.Targets {
		if !s.sgAlive(to) {
			return true, errors.New("请选择存活角色")
		}
	}
	to := -1
	if len(a.Targets) == 1 {
		to = a.Targets[0]
	}
	switch a.Skill {
	case "jie_rende":
		if err := s.sgJieRende(i, a, clone(p.Hand)); err != nil {
			return true, err
		}
	case "yijue":
		if to < 0 || to == i || len(g.Players[to].Hand) == 0 || s.sgValidateCards(i, a.Cards, 1, true) != nil {
			return true, errors.New("义绝需要一张手牌，与另一名有手牌角色拼点")
		}
		s.sgAsk(to, "pindian", "义绝：选择一张手牌拼点", SGEvent{Actor: i, Target: to, Kind: "yijue", Cards: clone(a.Cards)})
	case "jie_kurou":
		if len(a.Targets) > 0 || s.sgValidateCards(i, a.Cards, 1, false) != nil {
			return true, errors.New("苦肉需要弃一张自己的手牌或装备")
		}
		s.sgPush(SGEvent{Type: "lose_hp", Target: i, Amount: 1})
		s.sgDiscard(i, a.Cards)
	case "jie_fanjian":
		if to < 0 || to == i || s.sgValidateCards(i, a.Cards, 1, true) != nil {
			return true, errors.New("反间需要交给另一角色一张手牌")
		}
		s.sgPush(SGEvent{Type: "jie_fanjian", Actor: i, Target: to, Color: s.sgCardFor(i, a.Cards[0]).Suit})
		s.sgLose(i, a.Cards)
		s.sgGain(to, a.Cards)
	case "jie_guose":
		if to < 0 || s.sgValidateCards(i, a.Cards, 1, false) != nil || s.sgCardFor(i, a.Cards[0]).Suit != 3 {
			return true, errors.New("国色需要一张方片手牌或装备和一名目标")
		}
		if a.Choice == "remove" {
			id := 0
			for _, d := range g.Players[to].Judgment {
				if d.Kind == "indulgence" {
					id = d.Card
				}
			}
			if id == 0 {
				return true, errors.New("目标判定区没有乐不思蜀")
			}
			s.sgPush(SGEvent{Type: "jie_guose_remove", Actor: i, Target: to, Cards: []int{id}})
			s.sgDiscard(i, a.Cards)
		} else if a.Choice == "use" {
			if !s.sgHandUsable(i, a.Cards) {
				return true, errors.New("义绝：不能使用手牌")
			}
			s.sgPush(SGEvent{Type: "draw", Actor: i, Amount: 1})
			if err := s.sgUse(i, "indulgence", a.Cards, a.Targets, false); err != nil {
				return true, err
			}
		} else {
			return true, errors.New("请选择施加或移除乐不思蜀")
		}
	case "chuli":
		if s.sgValidateCards(i, a.Cards, 1, false) != nil || len(a.Targets) == 0 || !sgSubset(a.Targets, s.sgOrder(0)) {
			return true, errors.New("除疠需要自己的牌和不同势力的其他目标")
		}
		kingdoms := map[string]bool{}
		for _, who := range a.Targets {
			k := s.sgKingdom(who)
			if who == i || kingdoms[k] || len(s.sgDiscardable(i, who, false)) == 0 {
				return true, errors.New("除疠目标须各不相同势力且有可弃置手牌或装备")
			}
			kingdoms[k] = true
		}
		e := SGEvent{Type: "chuli", Actor: i}
		for _, who := range s.sgOrder(s.Turn) {
			if slices.Contains(a.Targets, who) {
				e.Targets = append(e.Targets, who)
			}
		}
		if s.sgCardFor(i, a.Cards[0]).Suit == 0 {
			e.Cards = []int{i}
		} // seat list, not physical cards
		s.sgPush(e)
		s.sgDiscard(i, a.Cards)
	case "jie_lijian":
		if len(a.Targets) != 2 || a.Targets[0] == a.Targets[1] || s.sgValidateCards(i, a.Cards, 1, false) != nil {
			return true, errors.New("离间需要弃一张牌并指定两名其他男性角色")
		}
		for _, who := range a.Targets {
			if who == i || s.sgFemale(who) {
				return true, errors.New("离间需要其他男性角色")
			}
		}
		if !s.sgCanTarget(a.Targets[1], a.Targets[0], "duel") {
			return true, errors.New("该角色不能成为决斗目标")
		}
		s.sgPush(SGEvent{Type: "jie_virtual_duel", Actor: a.Targets[1], Target: a.Targets[0]})
		s.sgDiscard(i, a.Cards)
	}
	p.Used[a.Skill]++
	s.sgLog("%s 发动「%s」", s.sgName(i), SGSkills[a.Skill].Name)
	return true, nil
}

func (s *State) sgJieWuEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "zhaxiang":
		if e.Amount <= 0 || !s.sgHas(e.Actor, "zhaxiang") {
			return true
		}
		e.Amount--
		s.sgPush(SGEvent{Type: "zhaxiang_bonus", Actor: e.Actor}, e)
		s.sgDraw(e.Actor, 3)
	case "zhaxiang_bonus":
		if s.sgAlive(e.Actor) && s.Turn == e.Actor && g.ActivePhase == "play" {
			g.Players[e.Actor].Used["zhaxiang"]++
		}

	case "jie_virtual_duel":
		if s.sgCanTarget(e.Actor, e.Target, "duel") {
			_ = s.sgUse(e.Actor, "duel", nil, []int{e.Target}, true)
		}
	case "jie_fanjian":
		if !s.sgAlive(e.Target) {
			return true
		}
		if len(g.Players[e.Target].Hand)+len(g.Players[e.Target].Equip) == 0 {
			s.sgPush(SGEvent{Type: "lose_hp", Target: e.Target, Amount: 1})
			return true
		}
		s.sgAsk(e.Target, "jie_fanjian", "反间：亮出全部手牌并弃掉指定花色的手牌/装备，或失去1体力", e)
		g.Pending.Choices = []string{"discard", "lose_hp"}
	case "jie_guose_remove":
		if !s.sgAlive(e.Actor) || !s.sgAlive(e.Target) {
			return true
		}
		for _, d := range g.Players[e.Target].Judgment {
			if d.Card == e.Cards[0] && d.Kind == "indulgence" {
				s.sgPush(SGEvent{Type: "draw", Actor: e.Actor, Amount: 1})
				s.sgDiscard(e.Target, e.Cards)
				break
			}
		}
	case "chuli":
		if !s.sgAlive(e.Actor) {
			return true
		}
		for e.Step < len(e.Targets) {
			e.Target = e.Targets[e.Step]
			e.Step++
			if len(s.sgDiscardable(e.Actor, e.Target, false)) > 0 {
				e.Kind = "chuli"
				s.sgAsk(e.Actor, "steal", "除疠：弃置当前目标一张手牌或装备", e)
				return true
			}
		}
		es := []SGEvent{}
		for _, who := range e.Cards {
			es = append(es, SGEvent{Type: "draw", Actor: who, Amount: 1})
		}
		s.sgPush(es...)
	case "liyu":
		if !s.sgHas(e.Actor, "liyu") || !s.sgAlive(e.Target) || e.Actor == e.Target || len(g.Players[e.Target].Hand)+len(g.Players[e.Target].Equip) == 0 {
			return true
		}
		targets := []int{}
		for _, who := range s.sgOrder(s.Turn) {
			if who != e.Target && s.sgCanTarget(e.Actor, who, "duel") {
				targets = append(targets, who)
			}
		}
		if len(targets) > 0 {
			s.sgAsk(e.Target, "liyu", "利驭：可选择第三者，令吕布获得你一张牌后与其决斗，或放弃", e)
			g.Pending.Targets = targets
		}
	case "fenwei":
		for e.Step < len(g.Players) {
			who := (s.Turn + e.Step) % len(g.Players)
			e.Step++
			if s.sgHas(who, "fenwei") && g.Players[who].Marks["fenwei"] == 0 {
				s.sgAsk(who, "fenwei", "奋威：整局限一次，可令这张锦囊对所选目标无效", e)
				g.Pending.Targets = clone(e.Targets)
				return true
			}
		}
	default:
		return false
	}
	return true
}

func (s *State) sgJieWuRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	e := q.Event
	switch q.Kind {
	case "jie_fanjian":
		if a.Choice == "lose_hp" {
			s.sgPush(SGEvent{Type: "lose_hp", Target: i, Amount: 1})
			return true, nil
		}
		if a.Choice != "discard" {
			return true, errors.New("请选择亮牌弃置或失去体力")
		}
		g.Revealed = clone(g.Players[i].Hand)
		names := []string{}
		for _, id := range g.Revealed {
			c := s.sgCardFor(i, id)
			names = append(names, fmt.Sprintf("%s%d「%s」", []string{"♠", "♥", "♣", "♦"}[c.Suit], c.Rank, SGCardTypes[c.Kind].Name))
		}
		s.sgLog("%s 因反间展示全部手牌：%s", s.sgName(i), strings.Join(names, "、"))
		ids := append(clone(g.Players[i].Hand), g.Players[i].Equip...)
		ids = slices.DeleteFunc(ids, func(id int) bool { return s.sgCardFor(i, id).Suit != e.Color })
		s.sgDiscard(i, ids)
	case "liyu":
		if a.Choice == "pass" {
			return true, nil
		}
		if len(a.Targets) != 1 || !slices.Contains(q.Targets, a.Targets[0]) {
			return true, errors.New("请选择利驭的合法决斗目标")
		}
		s.sgPush(SGEvent{Type: "jie_virtual_duel", Actor: e.Actor, Target: a.Targets[0]})
		s.sgAsk(e.Actor, "steal", "利驭：获得受伤者一张手牌或装备，再对其指定角色决斗", SGEvent{Actor: e.Actor, Target: i, Kind: "liyu"})
	case "fenwei":
		if a.Choice != "pass" {
			if len(a.Targets) == 0 || !sgSubset(a.Targets, q.Targets) {
				return true, errors.New("奋威需要选取这张锦囊的至少一名目标")
			}
			s.sgMark(i, "fenwei")
			g.Queue = slices.DeleteFunc(g.Queue, func(x SGEvent) bool {
				return x.TrickID == e.TrickID && (x.Type == "null_window" || x.Type == "effect") && slices.Contains(a.Targets, x.Target)
			})
			s.sgLog("%s 发动奋威，锦囊对 %d 名所选目标无效", s.sgName(i), len(a.Targets))
		}
		s.sgPush(e)
	case "jie_qianxun":
		if a.Choice != "yes" && a.Choice != "pass" {
			return true, errors.New("请选择发动或放弃谦逊")
		}
		s.sgPush(e)
		if a.Choice == "yes" {
			ids := clone(g.Players[i].Hand)
			s.sgLose(i, ids)
			g.Players[i].Qianxun = append(g.Players[i].Qianxun, ids...)
			s.sgLog("%s 暂时扣置 %d 张谦逊牌，本回合结束取回", s.sgName(i), len(ids))
		}
	default:
		return false, nil
	}
	return true, nil
}
