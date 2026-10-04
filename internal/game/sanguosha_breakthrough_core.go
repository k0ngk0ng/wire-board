package game

import "slices"

// Frequencies follow the pinned C++ skills, including the compulsory defaults
// of Filter/Distance/Prohibit/TargetMod/MaxCardsSkill. Paoxiao explicitly opts
// out in that source. Awakening and limited skills are not Compulsory.
func sgCompulsory(skill string) bool {
	return slices.Contains([]string{
		"kongcheng", "mashu", "qicai", "jiuyuan", "qianxun", "wushuang",
		"kuanggu", "hongyan", "bazhen", "xueyi", "huoshou", "juxiang",
		"weimu", "wansha", "roulin", "benghuai", "xiangle", "duanchang",
		"wushen", "wuhun", "feiying", "kuangbao", "wumou", "juejing", "renjie",
		"jie_qicai", "jie_yingzi", "zhaxiang",
		"heg_duanchang", "heg_mingshi", "heg_suishi",
	}, skill)
}

func (s *State) sgGain(i int, ids []int) {
	if len(ids) == 0 {
		return
	}
	if !s.sgAlive(i) {
		s.Sanguosha.Discard = append(s.Sanguosha.Discard, ids...)
		return
	}
	s.Sanguosha.Players[i].Hand = append(s.Sanguosha.Players[i].Hand, ids...)
	s.sgHandGained(i, ids)
}

// Atomic exchanges install both hands first, then call this same gain hook.
// The triggering IDs are saved separately from any later gains/losses.
func (s *State) sgHandGained(i int, ids []int) {
	g := s.Sanguosha
	if len(ids) > 0 && g.TurnSequence > 0 && !(s.Turn == i && g.ActivePhase == "draw") && s.sgHas(i, "qingjian") {
		s.sgPush(SGEvent{Type: "qingjian", Actor: i, Cards: clone(ids)})
	}
}

func (s *State) sgEmptyHand(i, lost int) {
	if lost <= 0 || len(s.Sanguosha.Players[i].Hand) != 0 {
		return
	}
	if s.sgHas(i, "lianying") {
		s.sgPush(SGEvent{Type: "optional_draw", Actor: i, Kind: "lianying", Amount: 1})
	}
	if s.sgHas(i, "jie_lianying") {
		s.sgPush(SGEvent{Type: "jie_lianying", Actor: i, Amount: lost})
	}
}

func (s *State) sgHandUsable(i int, ids []int) bool {
	if i < 0 || i >= len(s.Sanguosha.Players) {
		return false
	}
	p := s.Sanguosha.Players[i]
	if !p.HandSealed {
		return true
	}
	for _, id := range ids {
		v := s.Sanguosha.Virtual
		if slices.Contains(p.Hand, id) || v != nil && v.Player == i && v.Card == id && v.Paid {
			return false
		}
	}
	return true
}

func (s *State) sgClearSilence() {
	for i := range s.Sanguosha.Players {
		s.Sanguosha.Players[i].Silenced = false
		s.Sanguosha.Players[i].HandSealed = false
	}
}

func (s *State) sgCanDiscard(actor, owner, id int) bool {
	if !s.sgAlive(owner) {
		return false
	}
	p := s.Sanguosha.Players[owner]
	if actor != owner && s.sgHas(owner, "jie_qicai") && slices.Contains(p.Equip, id) {
		slot := SGCardTypes[sgCard(id).Kind].Slot
		if slot != "offense" && slot != "defense" {
			return false
		}
	}
	return true
}

func (s *State) sgDiscardable(actor, owner int, judgment bool) []int {
	if !s.sgAlive(owner) {
		return nil
	}
	p := s.Sanguosha.Players[owner]
	ids := append(clone(p.Hand), p.Equip...)
	if judgment {
		for _, d := range p.Judgment {
			ids = append(ids, d.Card)
		}
	}
	return slices.DeleteFunc(ids, func(id int) bool { return !s.sgCanDiscard(actor, owner, id) })
}

func sgStealDiscards(kind string) bool {
	return slices.Contains([]string{"dismantlement", "ice_sword", "mengjin", "tiaoxin", "jie_ganglie", "chuli"}, kind)
}

func (s *State) sgJieClearPiles(i int) {
	p := &s.Sanguosha.Players[i]
	s.Sanguosha.Discard = append(s.Sanguosha.Discard, p.Yiji...)
	s.Sanguosha.Discard = append(s.Sanguosha.Discard, p.Qianxun...)
	p.Yiji, p.Qianxun = nil, nil
}

func (s *State) sgJieTurnEnd(i int) {
	g := s.Sanguosha
	if s.sgAlive(i) && g.ActivePhase != "inactive" {
		p := &g.Players[i]
		p.PreviousHP = p.HP
		p.PreviousHPSet = true
	}
	g.ActivePhase = "inactive"
	delete(g.Players[i].Used, "zhaxiang")
	s.sgClearSilence()
	for _, who := range s.sgOrder(s.Turn) {
		p := &g.Players[who]
		ids := p.Qianxun
		p.Qianxun = nil
		s.sgGain(who, ids)
	}
}
