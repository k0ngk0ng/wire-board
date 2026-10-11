package game

import (
	"errors"
	"slices"
)

func (s *State) sgRespond(i int, a Action) error {
	g := s.Sanguosha
	prompt := *g.Pending
	e := prompt.Event
	g.Pending = nil
	p := &g.Players[i]
	pass := a.Choice == "pass"
	if handled, err := s.sgThreeRespond(i, a, prompt); handled {
		return err
	}
	if handled, err := s.sgHegRespond(i, a, prompt); handled {
		return err
	}
	if handled, err := s.sgJieRespond(i, a, prompt); handled {
		return err
	}
	if handled, err := s.sgGuhuoRespond(i, a, prompt); handled {
		return err
	}
	if handled, err := s.sgWindRespond(i, a, prompt); handled {
		return err
	}
	if handled, err := s.sgFireRespond(i, a, prompt); handled {
		return err
	}
	if handled, err := s.sgMountainRespond(i, a, prompt); handled {
		return err
	}
	if handled, err := s.sgThicketRespond(i, a, prompt); handled {
		return err
	}
	if handled, err := s.sgGodRespond(i, a, prompt); handled {
		return err
	}
	switch prompt.Kind {
	case "general":
		if !slices.Contains(p.Choices, a.Choice) {
			return errors.New("请选择候选武将")
		}
		p.General = a.Choice
		p.MaxHP = sgGeneral(p.General).HP
		if p.Role == "lord" && len(g.Players) >= 5 {
			p.MaxHP++
		}
		p.HP = p.MaxHP
		p.Choices = nil
		g.Selected++
		if i == g.Lord {
			ids := []string{}
			for _, x := range g.generalCatalog() {
				if x.ID != p.General {
					ids = append(ids, x.ID)
				}
			}
			shuffle(ids)
			for _, j := range s.sgOrder(s.sgNext(i)) {
				if j == i {
					continue
				}
				g.Players[j].Choices = append([]string{}, ids[:3]...)
				ids = ids[3:]
			}
		}
		if g.Selected < len(g.Players) {
			s.sgAsk(s.sgNext(i), "general", "选择你的武将", SGEvent{})
		} else {
			g.Selecting = false
			s.sgGodGameStart()
		}
	case "invoke":
		if !pass {
			if a.Choice != "yes" {
				return errors.New("请选择发动或放弃")
			}
			e.Flag = true
			s.sgLog("%s 发动「%s」", s.sgName(i), SGSkills[e.Kind].Name)
			s.sgPush(e)
		}
	case "guanxing":
		all := append(append([]int{}, a.Cards...), a.Take...)
		if !sgSameIDs(all, prompt.Cards) {
			return errors.New("请将全部观星牌放回牌堆顶或底")
		}
		g.Deck = append(append(append([]int{}, a.Cards...), g.Deck...), a.Take...)
	case "draw_phase":
		switch a.Choice {
		case "jie_tuxi":
			n := s.sgGodDrawCount(i, 2)
			if !s.sgHas(i, "jie_tuxi") || len(a.Targets) < 1 || len(a.Targets) > n || !sgSubset(a.Targets, s.sgOrder(0)) {
				return errors.New("突袭目标数量不合法")
			}
			for _, t := range a.Targets {
				if t == i || len(g.Players[t].Hand) == 0 || len(g.Players[t].Hand) < len(p.Hand) {
					return errors.New("突袭目标手牌数须不少于自己")
				}
			}
			es := []SGEvent{{Type: "draw", Actor: i, Amount: n - len(a.Targets)}}
			for _, t := range s.sgOrder(s.Turn) {
				if slices.Contains(a.Targets, t) {
					es = append(es, SGEvent{Type: "jie_tuxi_take", Actor: i, Target: t})
				}
			}
			s.sgPush(es...)
		case "shelie":
			if !s.sgHas(i, "shelie") {
				return errors.New("没有涉猎")
			}
			s.sgGodShelie(i)
		case "tuxi":
			if !s.sgHas(i, "tuxi") || len(a.Targets) < 1 || len(a.Targets) > 2 {
				return errors.New("突袭选择一至两名角色")
			}
			seen := map[int]bool{}
			for _, t := range a.Targets {
				if t == i || !s.sgAlive(t) || len(g.Players[t].Hand) == 0 || seen[t] {
					return errors.New("突袭目标不合法")
				}
				seen[t] = true
			}
			for _, t := range a.Targets {
				ids := clone(g.Players[t].Hand)
				shuffle(ids)
				s.sgLose(t, ids[:1])
				s.sgGain(i, ids[:1])
			}
			s.sgLog("%s 发动突袭，获得 %d 张手牌", s.sgName(i), len(a.Targets))
		case "zaiqi", "haoshi":
			if !s.sgHas(i, a.Choice) {
				return errors.New("没有此摸牌技能")
			}
			return s.sgThicketDraw(i, a.Choice)
		case "shuangxiong":
			if !s.sgHas(i, "shuangxiong") {
				return errors.New("没有双雄")
			}
			s.sgPush(SGEvent{Type: "judge", Actor: i, Kind: "shuangxiong"})
		case "luoyi":
			if !s.sgHas(i, "luoyi") {
				return errors.New("没有裸衣")
			}
			p.Used["luoyi"] = 1
			s.sgDraw(i, s.sgGodDrawCount(i, 1))
		case "yingzi":
			if !s.sgHas(i, "yingzi") {
				return errors.New("没有英姿")
			}
			s.sgDraw(i, s.sgGodDrawCount(i, 3))
		case "normal", "pass":
			s.sgDraw(i, s.sgGodDrawCount(i, 2))
		default:
			return errors.New("未知摸牌方式")
		}
	case "keji":
		if pass {
			e.Flag = true
			s.sgPush(e)
		} else if a.Choice != "yes" {
			return errors.New("请选择跳过或正常弃牌")
		}
	case "discard":
		if err := s.sgValidateCards(i, a.Cards, e.Amount, true); err != nil {
			return err
		}
		s.sgDiscard(i, a.Cards)
		s.sgLog("%s 弃置 %d 张手牌", s.sgName(i), len(a.Cards))
	case "nullification":
		if pass {
			if slices.Contains(e.Targets, i) {
				return errors.New("你已经放弃本次无懈响应")
			}
			e.Targets = append(e.Targets, i)
			if !s.sgAllPassed(e.Targets) {
				prompt.Event = e
				g.Pending = &prompt
				return nil
			}
		} else {
			kind, err := s.sgCommittedAs(i, a.Cards, a.Skill, "nullification")
			if err != nil {
				return err
			}
			if err := s.sgHegCounterScope(i, kind, a.Choice, &e); err != nil {
				return err
			}
			e.CounterDepth++
			e.Flag = !e.Flag
			e.Targets = nil
			s.sgPush(e)
			s.sgJieCardUsed(i, "nullification", a.Cards)
			s.sgSpent(i, a.Cards)
			s.sgLog("%s 使用「无懈可击」", s.sgName(i))
			s.sgPush(SGEvent{Type: "optional_draw", Actor: i, Kind: "jizhi", Amount: 1})
			s.sgGodTrick(i, "nullification")
			s.sgHegJizhi(i, kind, a.Cards)
			return nil
		}
		s.sgPush(e)
	case "fire_reveal":
		if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
			return err
		}
		id := a.Cards[0]
		e.Aux = s.sgCardFor(i, id).Suit
		g.Revealed = []int{id}
		s.sgLog("火攻：%s 展示 %s %d「%s」", s.sgName(i), []string{"♠", "♥", "♣", "♦"}[e.Aux], sgCard(id).Rank, SGCardTypes[sgCard(id).Kind].Name)
		if s.sgAlive(e.Actor) {
			s.sgAsk(e.Actor, "fire_discard", "火攻：弃置相同花色的手牌造成火焰伤害，或放弃", e)
			g.Pending.Cards = []int{id}
		}
	case "fire_discard":
		if !pass {
			if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
				return err
			}
			if s.sgCardFor(i, a.Cards[0]).Suit != e.Aux {
				return errors.New("火攻需要弃置与展示牌花色相同的手牌")
			}
			e.Type = "damage"
			e.Amount = 1
			e.Nature = "fire"
			s.sgPush(e)
			s.sgDiscard(i, a.Cards)
		}
	case "peach":
		if pass {
			e.Count++
			s.sgPush(e)
			break
		}
		kind, err := s.sgCommittedAs(i, a.Cards, a.Skill, "")
		if err != nil {
			return err
		}
		if kind != "peach" && !(kind == "analeptic" && i == e.Target) {
			return errors.New("救人需要桃；酒只能自救")
		}
		s.sgJieCardUsed(i, kind, a.Cards)
		s.sgSpent(i, a.Cards)
		amount := 1
		if i != e.Target && s.sgHas(e.Target, "jiuyuan") && s.sgKingdom(i) == "wu" {
			amount++
		}
		s.sgPush(e)
		s.sgHeal(e.Target, amount)
		s.sgLog("%s 使用%s救助 %s", s.sgName(i), SGCardTypes[kind].Name, s.sgName(e.Target))
	case "card":
		if pass {
			s.sgResponseFail(e)
			break
		}
		wanted := sgWanted(e)
		if a.Skill == "fan" && e.Kind != "collateral" {
			return errors.New("朱雀羽扇只能在使用杀时发动，不能用于打出杀")
		}
		if a.Choice == "eight_diagram" {
			if s.sgHegemony() && wanted == "jink" && e.Step&1 == 0 && !s.sgIgnoreArmor(e) && s.sgEquip(i, "armor") == 0 && s.sgHegMayInvoke(i, "bazhen") {
				if err := s.sgHegRevealSkill(i, "bazhen"); err != nil {
					return err
				}
			}
			if wanted != "jink" || e.Step&1 != 0 || s.sgArmor(i) != "eight_diagram" || s.sgIgnoreArmor(e) {
				return errors.New("此时不能发动八卦阵")
			}
			s.sgPush(SGEvent{Type: "judge", Actor: i, Kind: "eight_diagram", Next: &e})
			break
		}
		if a.Choice == "hujia" || a.Choice == "jijiang" {
			skill := a.Choice
			if !s.sgHas(i, skill) || e.Step&2 != 0 || (skill == "hujia" && wanted != "jink") || (skill == "jijiang" && wanted != "slash") {
				return errors.New("此时不能发动主公技")
			}
			s.sgPush(SGEvent{Type: "support", Actor: i, Kind: skill, Next: &e})
			break
		}
		kind, err := s.sgCommittedAs(i, a.Cards, a.Skill, wanted)
		if err != nil {
			return err
		}
		if e.Kind == "collateral" {
			return s.sgUse(i, kind, a.Cards, []int{e.Aux}, true)
		}
		s.sgLog("%s 打出「%s」", s.sgName(i), SGCardTypes[wanted].Name)
		if wanted == "slash" && i == s.Turn && g.InPlay {
			p.Used["keji_slash"]++
		}
		s.sgResponseSuccess(e)
		s.sgJieCardUsed(i, wanted, a.Cards)
		s.sgSpent(i, a.Cards)
		if wanted == "jink" {
			s.sgJinkPlayed(i)
		}
	case "support":
		if a.Choice == "eight_diagram" {
			if e.Kind != "hujia" || e.Step&1 != 0 || s.sgArmor(i) != "eight_diagram" {
				return errors.New("此时不能发动八卦阵")
			}
			s.sgPush(SGEvent{Type: "judge", Actor: i, Kind: "support_eight", Next: &e})
			break
		}
		if pass {
			s.sgPush(e)
			break
		}
		wanted := "jink"
		if e.Kind == "jijiang" {
			wanted = "slash"
		}
		if a.Skill == "fan" && e.Next != nil && e.Next.Kind != "collateral" && e.Next.Kind != "luanwu" && e.Next.Kind != "tiaoxin" {
			return errors.New("此时不能发动朱雀羽扇")
		}
		kind, err := s.sgCommittedAs(i, a.Cards, a.Skill, wanted)
		if err != nil {
			return err
		}
		s.sgLog("%s 响应 %s 的%s", s.sgName(i), s.sgName(e.Actor), SGSkills[e.Kind].Name)
		s.sgJieCardUsed(i, wanted, a.Cards)
		if e.Next == nil {
			s.sgLose(i, a.Cards)
			return s.sgUse(e.Actor, kind, a.Cards, e.Targets, false)
		}
		if e.Next.Kind == "collateral" || e.Next.Kind == "luanwu" || e.Next.Kind == "tiaoxin" {
			s.sgLose(i, a.Cards)
			return s.sgUse(e.Actor, kind, a.Cards, []int{e.Next.Aux}, true)
		}
		s.sgResponseSuccess(*e.Next)
		s.sgSpent(i, a.Cards)
		if wanted == "jink" {
			s.sgJinkPlayed(i)
		}
	case "liuli":
		if pass {
			break
		}
		if err := s.sgValidateCards(i, a.Cards, 1, false); err != nil {
			return err
		}
		if len(a.Targets) != 1 || a.Targets[0] == e.Actor || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) {
			return errors.New("流离目标不合法")
		}
		s.sgDiscard(i, a.Cards)
		t := a.Targets[0]
		if s.sgDistance(i, t) > s.sgRange(i) || !s.sgHegemony() && s.sgHas(t, "kongcheng") && len(g.Players[t].Hand) == 0 {
			return errors.New("流离目标不在弃牌后的攻击范围内")
		}
		// Replace this slash's continuation, not loss-trigger events prepended by payment.
		for j := range g.Queue {
			if (g.Queue[j].Type == "slash_weapon" || g.Queue[j].Type == "xiangle") && g.Queue[j].Actor == e.Actor && g.Queue[j].Target == i {
				g.Queue[j].Target = t
				break
			}
		}
		s.sgLog("%s 发动流离，杀转移给 %s", s.sgName(i), s.sgName(t))
	case "double_sword":
		if !pass {
			s.sgAsk(e.Target, "sword_discard", "弃一张手牌，否则对方摸一张牌", e)
		}
	case "sword_discard":
		if pass {
			s.sgDraw(e.Actor, 1)
		} else {
			if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
				return err
			}
			s.sgDiscard(i, a.Cards)
		}
	case "tieji":
		if pass {
			s.sgPush(s.sgSlashResponse(e))
		} else {
			s.sgPush(SGEvent{Type: "judge", Actor: i, Kind: "tieji", Next: &e})
		}
	case "weapon_after_jink":
		if pass {
			break
		}
		if s.sgWeapon(i) == "axe" {
			if err := s.sgValidateCards(i, a.Cards, 2, false); err != nil {
				return err
			}
			if slices.Contains(a.Cards, s.sgEquip(i, "weapon")) {
				return errors.New("贯石斧不能弃置自身")
			}
			e.Type = "hit"
			s.sgPush(e)
			s.sgDiscard(i, a.Cards)
		} else {
			kind, err := s.sgCommittedAs(i, a.Cards, a.Skill, "slash")
			if err != nil {
				return err
			}
			return s.sgUse(i, kind, a.Cards, []int{e.Target}, true)
		}
	case "ice_sword":
		if pass {
			e.Type = "damage"
			s.sgPush(e)
		} else {
			e.Kind = "ice_sword"
			e.Count = 2
			s.sgAsk(i, "steal", "寒冰剑：弃置目标的一张牌", e)
		}
	case "kylin_bow":
		if !pass {
			if len(a.Cards) != 1 || (!slices.Contains(g.Players[e.Target].Equip, a.Cards[0])) {
				return errors.New("请选择目标坐骑")
			}
			slot := SGCardTypes[sgCard(a.Cards[0]).Kind].Slot
			if slot != "offense" && slot != "defense" {
				return errors.New("请选择目标坐骑")
			}
			s.sgDiscard(e.Target, a.Cards)
		}
	case "steal":
		target := &g.Players[e.Target]
		id := a.Card
		if a.Choice == "hand" {
			if len(target.Hand) == 0 {
				return errors.New("目标没有手牌")
			}
			ids := clone(target.Hand)
			shuffle(ids)
			id = ids[0]
		} else {
			valid := slices.Contains(target.Equip, id)
			if e.Kind == "snatch" || e.Kind == "dismantlement" || e.Kind == "guixin" {
				for _, d := range target.Judgment {
					valid = valid || d.Card == id
				}
			}
			if !valid {
				return errors.New("请选择装备、判定牌或随机手牌")
			}
		}
		if !sgStealDiscards(e.Kind) && i == e.Target && slices.Contains(p.Hand, id) {
			return nil
		}
		if sgStealDiscards(e.Kind) && !s.sgCanDiscard(i, e.Target, id) {
			return errors.New("奇才：不能弃置对方的非坐骑装备")
		}
		if e.Kind == "chuli" {
			if s.sgCardFor(e.Target, id).Suit == 0 {
				e.Cards = append(e.Cards, e.Target)
			}
			s.sgPush(e)
		}
		if e.Kind == "ice_sword" && e.Count > 1 {
			next := e
			next.Count--
			next.Type = "ice_continue"
			s.sgPush(next)
		}
		if sgStealDiscards(e.Kind) {
			s.sgRecordDiscard(e.Target, []int{id})
		}
		s.sgLose(e.Target, []int{id})
		if sgStealDiscards(e.Kind) {
			g.Discard = append(g.Discard, id)
			s.sgLog("%s 弃置 %s 的「%s」", s.sgName(i), s.sgName(e.Target), SGCardTypes[sgCard(id).Kind].Name)
		} else {
			s.sgGain(i, []int{id})
			s.sgLog("%s 获得 %s 的一张牌", s.sgName(i), s.sgName(e.Target))
		}

	case "grace":
		if !slices.Contains(g.Grace, a.Card) {
			return errors.New("请选择五谷丰登中的牌")
		}
		s.sgGain(i, []int{a.Card})
		g.Grace = sgRemove(g.Grace, a.Card)
		s.sgLog("%s 从五谷丰登获得「%s」", s.sgName(i), SGCardTypes[sgCard(a.Card).Kind].Name)
	case "guicai":
		if !pass {
			if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
				return err
			}
			s.sgFinishCards([]int{e.Aux})
			s.sgPay(i, a.Cards)
			e.Aux = a.Cards[0]
			s.sgLog("%s 发动鬼才，更换判定牌", s.sgName(i))
		}
		s.sgPush(e)
	case "tiandu":
		if !pass {
			s.sgTakeTable(e.Aux)
			s.sgGain(i, []int{e.Aux})
		} else {
			s.sgFinishCards([]int{e.Aux})
		}
	case "ganglie":
		if pass {
			s.sgPush(SGEvent{Type: "damage", Actor: e.Actor, Target: i, Kind: "ganglie", Amount: 1})
		} else {
			if err := s.sgValidateCards(i, a.Cards, 2, true); err != nil {
				return err
			}
			s.sgDiscard(i, a.Cards)
		}
	case "yiji":
		if len(a.Cards) == 0 || len(a.Targets) != 1 || !s.sgAlive(a.Targets[0]) {
			return errors.New("选择遗计牌和一名获得者")
		}
		if !sgSubset(a.Cards, prompt.Cards) {
			return errors.New("请选择遗计中的牌")
		}
		t := a.Targets[0]
		s.sgGain(t, a.Cards)
		remaining := clone(prompt.Cards)
		for _, id := range a.Cards {
			remaining = sgRemove(remaining, id)
		}
		if len(remaining) > 0 {
			s.sgAsk(i, "yiji", "继续分配遗计的牌", e)
			g.Pending.Cards = remaining
		}
	case "fanjian":
		suit := slices.Index([]string{"spade", "heart", "club", "diamond"}, a.Choice)
		if suit < 0 {
			return errors.New("请选择花色")
		}
		if len(g.Players[e.Actor].Hand) == 0 {
			break
		}
		ids := clone(g.Players[e.Actor].Hand)
		shuffle(ids)
		id := ids[0]
		s.sgLose(e.Actor, []int{id})
		s.sgGain(i, []int{id})
		s.sgLog("反间：%s 获得 %s %d「%s」", s.sgName(i), []string{"♠", "♥", "♣", "♦"}[sgCard(id).Suit], sgCard(id).Rank, SGCardTypes[sgCard(id).Kind].Name)
		if sgCard(id).Suit != suit {
			s.sgPush(SGEvent{Type: "damage", Actor: e.Actor, Target: i, Kind: "fanjian", Amount: 1})
		}
	default:
		return errors.New("未知响应阶段")
	}
	return nil
}
func sgSubset(ids, available []int) bool {
	seen := map[int]bool{}
	for _, id := range ids {
		if seen[id] || !slices.Contains(available, id) {
			return false
		}
		seen[id] = true
	}
	return true
}
func sgSameIDs(a, b []int) bool { return len(a) == len(b) && sgSubset(a, b) }

func (s *State) sgSkill(i int, a Action) error {
	g := s.Sanguosha
	p := &g.Players[i]
	skill := a.Skill
	if skill == "huangtian_give" {
		return s.sgHuangtianGive(i, a)
	}
	if skill == "zhiba_pindian" {
		return s.sgZhiba(i, a)
	}
	if !s.sgHas(i, skill) {
		return errors.New("没有此技能")
	}
	if handled, err := s.sgHegSkill(i, a); handled {
		return err
	}
	if handled, err := s.sgJieSkill(i, a); handled {
		return err
	}
	if handled, err := s.sgGodSkill(i, a); handled {
		return err
	}
	if handled, err := s.sgMountainSkill(i, a); handled {
		return err
	}
	if handled, err := s.sgThicketSkill(i, a); handled {
		return err
	}
	if handled, err := s.sgFireSkill(i, a); handled {
		return err
	}
	for _, t := range a.Targets {
		if !s.sgAlive(t) {
			return errors.New("目标已阵亡或不存在")
		}
	}
	once := slices.Contains([]string{"zhiheng", "fanjian", "jieyin", "qingnang", "lijian"}, skill)
	if once && p.Used[skill] > 0 {
		return errors.New("本阶段已经发动此技能")
	}
	target := func() int {
		if len(a.Targets) != 1 {
			return -1
		}
		return a.Targets[0]
	}()
	switch skill {
	case "rende":
		if target < 0 || target == i || len(a.Cards) == 0 {
			return errors.New("选择手牌交给另一名角色")
		}
		if err := s.sgValidateCards(i, a.Cards, -1, true); err != nil {
			return err
		}
		before := p.Used[skill]
		s.sgLose(i, a.Cards)
		s.sgGain(target, a.Cards)
		p.Used[skill] += len(a.Cards)
		if before < 2 && p.Used[skill] >= 2 {
			s.sgHeal(i, 1)
		}
	case "zhiheng":
		if len(a.Cards) == 0 {
			return errors.New("请选择要制衡的牌")
		}
		if err := s.sgValidateCards(i, a.Cards, -1, false); err != nil {
			return err
		}
		s.sgPush(SGEvent{Type: "draw", Actor: i, Amount: len(a.Cards)})
		s.sgDiscard(i, a.Cards)
	case "kurou":
		s.sgPush(SGEvent{Type: "lose_hp", Target: i, Amount: 1}, SGEvent{Type: "draw", Actor: i, Amount: 2})
	case "qingnang", "jieyin":
		n := 1
		if skill == "jieyin" {
			n = 2
		}
		if target < 0 || g.Players[target].HP >= g.Players[target].MaxHP {
			return errors.New("选择已受伤角色")
		}
		if skill == "jieyin" && (target == i || !s.sgMale(target)) {
			return errors.New("结姻需要另一名已受伤男性角色")
		}
		if err := s.sgValidateCards(i, a.Cards, n, true); err != nil {
			return err
		}
		s.sgDiscard(i, a.Cards)
		s.sgHeal(target, 1)
		if skill == "jieyin" {
			s.sgHeal(i, 1)
		}
	case "fanjian":
		if target < 0 || target == i || len(p.Hand) == 0 {
			return errors.New("反间需要有手牌并指定另一名角色")
		}
		s.sgAsk(target, "fanjian", "反间：猜测将获得的牌的花色", SGEvent{Actor: i, Target: target})
	case "lijian":
		if len(a.Targets) != 2 || a.Targets[0] == a.Targets[1] {
			return errors.New("离间需要两名不同男性角色")
		}
		for _, t := range a.Targets {
			if t == i || !s.sgMale(t) {
				return errors.New("离间需要其他男性角色")
			}
		}
		if !s.sgCanTarget(a.Targets[1], a.Targets[0], "duel") {
			return errors.New("该角色不能成为决斗目标")
		}
		if err := s.sgValidateCards(i, a.Cards, 1, false); err != nil {
			return err
		}
		s.sgPush(SGEvent{Type: "effect", Actor: a.Targets[1], Target: a.Targets[0], Kind: "duel", Amount: 1, Aux: -1})
		s.sgPush(SGEvent{Type: "optional_draw", Actor: a.Targets[1], Kind: "jiang", Amount: 1}, SGEvent{Type: "optional_draw", Actor: a.Targets[0], Kind: "jiang", Amount: 1})
		s.sgDiscard(i, a.Cards)
	case "jijiang":
		if target < 0 || !s.sgCanTarget(i, target, "slash") {
			return errors.New("选择杀的合法目标")
		}
		if p.Used["slash"] > 0 && s.sgWeapon(i) != "crossbow" {
			return errors.New("已经使用过杀")
		}
		if p.Used["jijiang_failed"] > 0 {
			return errors.New("本阶段激将已无人响应")
		}
		s.sgPush(SGEvent{Type: "support", Actor: i, Kind: "jijiang", Targets: a.Targets})
	default:
		return errors.New("此技能应在对应时机发动")
	}
	if once {
		p.Used[skill]++
	}
	s.sgLog("%s 发动「%s」", s.sgName(i), SGSkills[skill].Name)
	return nil
}
