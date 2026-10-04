package game

import (
	"errors"
	"slices"
)

var sgMountainGenerals = []SGGeneral{
	{"zhanghe", "张郃", "wei", 4, false, []string{"qiaobian"}},
	{"dengai", "邓艾", "wei", 4, false, []string{"tuntian", "zaoxian"}},
	{"jiangwei", "姜维", "shu", 4, false, []string{"tiaoxin", "zhiji"}},
	{"liushan", "刘禅", "shu", 3, false, []string{"xiangle", "fangquan", "ruoyu"}},
	{"sunce", "孙策", "wu", 4, false, []string{"jiang", "hunzi", "zhiba"}},
	{"erzhang", "张昭张纮", "wu", 3, false, []string{"zhijian", "guzheng"}},
	{"caiwenji", "蔡文姬", "qun", 3, true, []string{"beige", "duanchang"}},
	{"zuoci", "左慈", "qun", 3, false, []string{"huashen", "xinsheng"}},
}

func init() {
	for k, v := range map[string]SGSkill{
		"qiaobian":      {"巧变", "判定、摸牌、出牌或弃牌阶段前，可弃一张手牌跳过该阶段。跳过未被跳过的摸牌阶段后可获得其他一至两人各一张手牌；跳过出牌阶段后可移动场上一张装备或判定牌。"},
		"tuntian":       {"屯田", "回合外失去手牌或装备后，可以判定；若不为红桃，将判定牌置为田。计算你到其他角色的距离减去田的数量。"},
		"zaoxian":       {"凿险", "觉醒技：准备阶段，若田至少三张，减1点体力上限并获得急袭。"},
		"jixi":          {"急袭", "出牌阶段，可以将一张田当顺手牵羊使用。按消耗此田后的距离选择目标。"},
		"tiaoxin":       {"挑衅", "出牌阶段限一次：选择攻击范围包含你的另一角色，其须对你使用杀，否则由你弃置其一张手牌或装备。"},
		"zhiji":         {"志继", "觉醒技：准备阶段没有手牌时，减1点体力上限，回复1点体力或摸两张牌，然后获得观星。"},
		"xiangle":       {"享乐", "锁定技：成为杀的目标时，来源须额外弃置一张基本牌，否则此杀对你无效。"},
		"fangquan":      {"放权", "可以跳过出牌阶段；若如此做，回合结束时可以弃一张手牌，令另一名角色获得一个额外回合。"},
		"ruoyu":         {"若愚", "主公技，觉醒技：准备阶段，若你的体力值为全场最少（可并列），增加1点体力上限、回复1点体力并获得激将。"},
		"jiang":         {"激昂", "使用决斗或红色杀，或成为这些牌的目标时，可以摸一张牌。"},
		"hunzi":         {"魂姿", "觉醒技：准备阶段体力为1时，减1点体力上限并获得英姿和英魂。"},
		"zhiba":         {"制霸", "主公技：其他吴势力角色各自出牌阶段限一次，可与你拼点。若其没赢，你可获得两张拼点牌。魂姿觉醒后，你可拒绝拼点。"},
		"zhiba_pindian": {"制霸·拼点", "出牌阶段限一次，与拥有制霸的主公拼点；若你没有赢，主公可获得双方拼点牌。"},
		"zhijian":       {"直谏", "出牌阶段，可以将一张装备手牌放入另一名角色空置的对应装备栏，然后摸一张牌。"},
		"guzheng":       {"固政", "其他角色弃牌阶段结束时，可以返还其在该阶段弃置的一张手牌，然后获得此阶段其他仍在弃牌堆的弃置牌。"},
		"beige":         {"悲歌", "任意角色受到杀的伤害后，可以弃一张手牌或装备，令其判定：红桃回复1；方片摸2；梅花令伤害来源弃2；黑桃令来源翻面。"},
		"duanchang":     {"断肠", "锁定技：你死亡时，令伤害来源失去所有武将技能。"},
		"huashen":       {"化身", "开局秘密获得两张化身，选择其中一名武将的非主公、非限定、非觉醒技能，势力与性别随之改变。回合开始或结束时可以重新选择。其他人仅能看到当前化身及技能。"},
		"xinsheng":      {"新生", "受到伤害后，可以获得与伤害点数相同数量的新化身。"},
	} {
		SGSkills[k] = v
	}
}

func (s *State) sgAwaken(i int, skill string, hp int, acquired ...string) {
	s.sgMark(i, skill)
	p := &s.Sanguosha.Players[i]
	p.MaxHP += hp
	p.HP = min(p.HP, p.MaxHP)
	s.sgLog("%s 觉醒「%s」，体力上限变为 %d", s.sgName(i), SGSkills[skill].Name, p.MaxHP)
	if p.MaxHP <= 0 {
		s.sgDie(i, -1)
		return
	}
	s.sgAcquire(i, acquired...)
}
func (s *State) sgMountainStart(i int) {
	p := &s.Sanguosha.Players[i]
	if s.sgHas(i, "zaoxian") && p.Marks["zaoxian"] == 0 && len(p.Fields) >= 3 {
		s.sgAwaken(i, "zaoxian", -1, "jixi")
	}
	if s.sgHas(i, "hunzi") && p.Marks["hunzi"] == 0 && p.HP == 1 {
		s.sgAwaken(i, "hunzi", -1, "yingzi", "yinghun")
	}
	if s.sgHas(i, "ruoyu") && p.Marks["ruoyu"] == 0 {
		lowest := true
		for _, who := range s.sgOrder(0) {
			if s.Sanguosha.Players[who].HP < p.HP {
				lowest = false
			}
		}
		if lowest {
			s.sgAwaken(i, "ruoyu", 1, "jijiang")
			s.sgHeal(i, 1)
		}
	}
	if s.sgHas(i, "zhiji") && p.Marks["zhiji"] == 0 && len(p.Hand) == 0 {
		s.sgAwaken(i, "zhiji", -1, "guanxing")
		if s.sgAlive(i) {
			s.sgAsk(i, "zhiji", "志继觉醒：选择回复1点体力或摸两张牌", SGEvent{Actor: i})
			s.Sanguosha.Pending.Choices = []string{"draw"}
			if p.HP < p.MaxHP {
				s.Sanguosha.Pending.Choices = append(s.Sanguosha.Pending.Choices, "heal")
			}
		}
	}
}
func (s *State) sgTuntianLoss(i int, lost bool) {
	if lost && i != s.Turn && s.sgHas(i, "tuntian") {
		s.sgPush(SGEvent{Type: "tuntian", Actor: i})
	}
}
func (s *State) sgRecordDiscard(i int, ids []int) {
	g := s.Sanguosha
	if !g.DiscardPhase {
		return
	}
	for _, id := range ids {
		if i == s.Turn && slices.Contains(g.Players[i].Hand, id) {
			if !slices.Contains(g.DiscardedHand, id) {
				g.DiscardedHand = append(g.DiscardedHand, id)
			}
		} else if !slices.Contains(g.DiscardedOther, id) {
			g.DiscardedOther = append(g.DiscardedOther, id)
		}
	}
}
func (s *State) sgMountainEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "mountain_start":
		s.sgMountainStart(e.Actor)
	case "tuntian":
		if !s.sgHas(e.Actor, "tuntian") {
			return true
		}
		if !e.Flag {
			s.sgOptional(e.Actor, "tuntian", e)
		} else {
			s.sgPush(SGEvent{Type: "judge", Actor: e.Actor, Kind: "tuntian"})
		}
	case "qiaobian":
		if s.sgHas(e.Actor, "qiaobian") && len(g.Players[e.Actor].Hand) > 0 {
			s.sgAsk(e.Actor, "qiaobian", "巧变：弃一张手牌跳过"+map[string]string{"judge": "判定", "draw": "摸牌", "play": "出牌", "discard": "弃牌"}[e.Kind]+"阶段，或放弃", e)
		}
	case "qiaobian_benefit":
		if !s.sgAlive(e.Actor) {
			return true
		}
		if e.Kind == "draw" {
			for _, who := range s.sgOrder(s.Turn) {
				if who != e.Actor && len(g.Players[who].Hand) > 0 {
					e.Targets = append(e.Targets, who)
				}
			}
			if len(e.Targets) > 0 {
				s.sgAsk(e.Actor, "qiaobian_draw", "巧变：选择其他一至两名有手牌角色，各获得一张手牌", e)
				g.Pending.Targets = e.Targets
			}
		} else if e.Kind == "play" && len(s.sgQiaobianMoves(e.Actor)) > 0 {
			s.sgAsk(e.Actor, "qiaobian_move", "巧变：选择场上的装备或判定牌，移动到合法角色的对应区域", e)
		}
	case "fangquan":
		if !g.SkipPlay && s.sgHas(e.Actor, "fangquan") {
			s.sgAsk(e.Actor, "fangquan", "是否发动放权，跳过出牌阶段？", e)
		}
	case "fangquan_finish":
		if s.sgHas(e.Actor, "fangquan") && g.Players[e.Actor].Used["fangquan"] > 0 && len(g.Players[e.Actor].Hand) > 0 {
			s.sgAsk(e.Actor, "fangquan_give", "放权：弃一张手牌，选择获得额外回合的另一角色", e)
		}
	case "xiangle":
		if !s.sgAlive(e.Target) {
			return true
		}
		if s.sgHas(e.Target, "xiangle") {
			if s.sgAlive(e.Actor) {
				s.sgAsk(e.Actor, "xiangle", "享乐：额外弃一张基本手牌，否则此杀对目标无效", e)
			}
		} else {
			e.Type = "slash_weapon"
			s.sgPush(e)
			if e.Color == 1 {
				s.sgPush(SGEvent{Type: "optional_draw", Actor: e.Target, Kind: "jiang", Amount: 1})
			}
		}
	case "tiaoxin_response":
		if !s.sgAlive(e.Actor) || !s.sgAlive(e.Target) {
			return true
		}
		e.Kind = "tiaoxin"
		s.sgAsk(e.Target, "tiaoxin", "挑衅：对发起者使用杀，或令其弃置你的一张手牌或装备", e)
		g.Pending.Targets = []int{e.Actor}
	case "tiaoxin_discard":
		if s.sgAlive(e.Actor) && s.sgAlive(e.Target) && len(g.Players[e.Target].Hand)+len(g.Players[e.Target].Equip) > 0 {
			e.Kind = "tiaoxin"
			s.sgAsk(e.Actor, "steal", "挑衅：弃置对方一张手牌或装备", e)
		}
	case "guzheng":
		g.DiscardPhase = false
		if !s.sgAlive(e.Actor) {
			g.DiscardedHand = nil
			g.DiscardedOther = nil
			return true
		}
		e.Cards = nil
		for _, id := range g.DiscardedHand {
			if slices.Contains(g.Discard, id) {
				e.Cards = append(e.Cards, id)
			}
		}
		order := s.sgOrder(s.Turn)
		for e.Step < len(order) && len(e.Cards) > 0 {
			who := order[e.Step]
			e.Step++
			if who != e.Actor && s.sgHas(who, "guzheng") {
				s.sgAsk(who, "guzheng", "固政：选择返还的一张弃置手牌，然后获得其余弃牌", e)
				g.Pending.Cards = e.Cards
				return true
			}
		}
		g.DiscardedHand = nil
		g.DiscardedOther = nil
	case "beige":
		if !s.sgAlive(e.Target) {
			return true
		}
		order := s.sgOrder(s.Turn)
		for e.Step < len(order) {
			who := order[e.Step]
			e.Step++
			if s.sgHas(who, "beige") && len(g.Players[who].Hand)+len(g.Players[who].Equip) > 0 {
				s.sgAsk(who, "beige", "悲歌：弃一张手牌或装备，令受伤角色判定", e)
				return true
			}
		}
	case "beige_discard":
		if !s.sgAlive(e.Actor) {
			return true
		}
		e.Amount = min(2, len(g.Players[e.Actor].Hand)+len(g.Players[e.Actor].Equip))
		if e.Amount > 0 {
			s.sgAsk(e.Actor, "beige_discard", "悲歌梅花：弃置两张手牌或装备（不足则全部弃置）", e)
		}
	case "zhiba_start":
		if !s.sgAlive(e.Actor) || !s.sgHas(e.Target, "zhiba") {
			return true
		}
		if g.Players[e.Target].Marks["hunzi"] > 0 && !e.Flag {
			s.sgAsk(e.Target, "zhiba_accept", "制霸：接受或拒绝此次拼点", e)
		} else {
			s.sgAsk(e.Target, "pindian", "制霸：选择一张手牌拼点", e)
		}
	case "zhiba_obtain":
		if s.sgAlive(e.Target) {
			s.sgAsk(e.Target, "zhiba_obtain", "制霸：是否获得双方仍在处理区的拼点牌？", e)
			g.Pending.Cards = e.Cards
		}
	case "huashen_init":
		if s.sgHas(e.Actor, "huashen") {
			s.sgAcquireAvatars(e.Actor, 2)
			s.sgAskHuashen(e.Actor, true)
		}
	case "huashen_select":
		if s.sgHas(e.Actor, "huashen") {
			s.sgAskHuashen(e.Actor, false)
		}
	case "xinsheng":
		if s.sgHas(e.Target, "xinsheng") {
			if !e.Flag {
				s.sgOptional(e.Target, "xinsheng", e)
			} else {
				s.sgAcquireAvatars(e.Target, e.Amount)
			}
		}
	default:
		return false
	}
	return true
}

func (s *State) sgMountainRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	p := &g.Players[i]
	e := q.Event
	pass := a.Choice == "pass"
	switch q.Kind {
	case "zhiji":
		if a.Choice == "draw" {
			s.sgDraw(i, 2)
		} else if a.Choice == "heal" && p.HP < p.MaxHP {
			s.sgHeal(i, 1)
		} else {
			return true, errors.New("请选择觉醒后的摸牌或回复")
		}
	case "qiaobian":
		if pass {
			return true, nil
		}
		if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
			return true, err
		}
		skipped := false
		switch e.Kind {
		case "judge":
			skipped = g.SkipJudge
			g.SkipJudge = true
		case "draw":
			skipped = g.SkipDraw
			g.SkipDraw = true
		case "play":
			skipped = g.SkipPlay
			g.SkipPlay = true
		case "discard":
			skipped = g.SkipDiscard
			g.SkipDiscard = true
		}
		if !skipped {
			e.Type = "qiaobian_benefit"
			s.sgPush(e)
		}
		s.sgDiscard(i, a.Cards)
		s.sgLog("%s 发动巧变，跳过%s阶段", s.sgName(i), map[string]string{"judge": "判定", "draw": "摸牌", "play": "出牌", "discard": "弃牌"}[e.Kind])
	case "qiaobian_draw":
		if pass {
			return true, nil
		}
		if len(a.Targets) < 1 || len(a.Targets) > 2 || len(a.Targets) == 2 && a.Targets[0] == a.Targets[1] {
			return true, errors.New("选择一至两名不同角色")
		}
		for _, who := range a.Targets {
			if !slices.Contains(q.Targets, who) || len(g.Players[who].Hand) == 0 {
				return true, errors.New("目标没有可获得的手牌")
			}
		}
		for _, who := range a.Targets {
			ids := clone(g.Players[who].Hand)
			shuffle(ids)
			s.sgLose(who, ids[:1])
			p.Hand = append(p.Hand, ids[0])
		}
		s.sgLog("%s 发动巧变，获得 %d 张手牌", s.sgName(i), len(a.Targets))
	case "qiaobian_move":
		if pass {
			return true, nil
		}
		for _, move := range s.sgQiaobianMoves(i) {
			if a.Card != move.Card || !slices.Equal(a.Targets, move.Targets) {
				continue
			}
			from, to := a.Targets[0], a.Targets[1]
			if slices.Contains(g.Players[from].Equip, a.Card) {
				s.sgLose(from, []int{a.Card})
				g.Players[to].Equip = append(g.Players[to].Equip, a.Card)
			} else {
				for _, d := range g.Players[from].Judgment {
					if d.Card == a.Card {
						s.sgLose(from, []int{a.Card})
						g.Players[to].Judgment = append(g.Players[to].Judgment, d)
						break
					}
				}
			}
			s.sgLog("%s 巧变：将 %s 的「%s」移至 %s", s.sgName(i), s.sgName(from), SGCardTypes[sgCard(a.Card).Kind].Name, s.sgName(to))
			return true, nil
		}
		return true, errors.New("请选择合法的移动来源、卡牌及空置目标区域")
	case "fangquan":
		if pass {
			return true, nil
		}
		if a.Choice != "yes" {
			return true, errors.New("请选择发动或放弃")
		}
		g.SkipPlay = true
		p.Used["fangquan"] = 1
	case "fangquan_give":
		if pass {
			return true, nil
		}
		if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
			return true, err
		}
		if len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) {
			return true, errors.New("选择另一名存活角色")
		}
		g.ExtraTurns = append([]int{a.Targets[0]}, g.ExtraTurns...)
		s.sgDiscard(i, a.Cards)
		s.sgLog("%s 放权：%s 将获得额外回合", s.sgName(i), s.sgName(a.Targets[0]))
	case "xiangle":
		if pass {
			s.sgLog("%s 未支付享乐，此杀对 %s 无效", s.sgName(i), s.sgName(e.Target))
			return true, nil
		}
		if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
			return true, err
		}
		if !sgBasic(sgCard(a.Cards[0]).Kind) {
			return true, errors.New("享乐需要额外弃置基本牌")
		}
		e.Type = "slash_weapon"
		s.sgPush(e)
		s.sgDiscard(i, a.Cards)
	case "tiaoxin":
		if pass {
			e.Type = "tiaoxin_discard"
			s.sgPush(e)
			return true, nil
		}
		if a.Choice == "jijiang" {
			if !s.sgHas(i, "jijiang") || e.Step&2 != 0 || !s.sgCanTarget(i, e.Actor, "slash") || s.sgTianyi(i) == -1 {
				return true, errors.New("此时不能发动激将")
			}
			e.Aux = e.Actor
			s.sgPush(SGEvent{Type: "support", Actor: i, Kind: "jijiang", Next: &e})
			return true, nil
		}
		kind, err := s.sgAs(i, a.Cards, a.Skill, "slash")
		if err != nil {
			return true, err
		}
		return true, s.sgUse(i, kind, a.Cards, []int{e.Actor}, true)
	case "guzheng":
		s.sgPush(e)
		if pass {
			return true, nil
		}
		if !slices.Contains(q.Cards, a.Card) || !slices.Contains(g.Discard, a.Card) {
			return true, errors.New("请选择该弃牌阶段弃置的手牌")
		}
		g.Discard = sgRemove(g.Discard, a.Card)
		g.Players[e.Actor].Hand = append(g.Players[e.Actor].Hand, a.Card)
		count := 0
		for _, id := range append(clone(g.DiscardedHand), g.DiscardedOther...) {
			if slices.Contains(g.Discard, id) {
				g.Discard = sgRemove(g.Discard, id)
				p.Hand = append(p.Hand, id)
				count++
			}
		}
		s.sgLog("%s 固政：返还 %s 一张牌，获得其余 %d 张弃牌", s.sgName(i), s.sgName(e.Actor), count)
	case "beige":
		s.sgPush(e)
		if pass {
			return true, nil
		}
		if err := s.sgValidateCards(i, a.Cards, 1, false); err != nil {
			return true, err
		}
		s.sgPush(SGEvent{Type: "judge", Kind: "beige", Actor: e.Target, Target: e.Actor})
		s.sgDiscard(i, a.Cards)
	case "beige_discard":
		if err := s.sgValidateCards(i, a.Cards, e.Amount, false); err != nil {
			return true, err
		}
		s.sgDiscard(i, a.Cards)
	case "zhiba_accept":
		if pass {
			s.sgLog("%s 拒绝制霸拼点", s.sgName(i))
			return true, nil
		}
		if a.Choice != "yes" {
			return true, errors.New("请选择接受或拒绝")
		}
		e.Flag = true
		s.sgPush(e)
	case "zhiba_obtain":
		if pass {
			return true, nil
		}
		if a.Choice != "yes" {
			return true, errors.New("请选择获得或放弃")
		}
		for _, id := range e.Cards {
			if slices.Contains(g.Table, id) {
				s.sgTakeTable(id)
				p.Hand = append(p.Hand, id)
			}
		}
	case "huashen":
		return true, s.sgSelectAvatar(i, a, q)
	default:
		return false, nil
	}
	return true, nil
}

func sgBasic(kind string) bool {
	return sgIsSlash(kind) || slices.Contains([]string{"jink", "peach", "analeptic"}, kind)
}

// Movements are public choices. No hidden card identifiers are exposed.
func (s *State) sgQiaobianMoves(actor int) []Action {
	g := s.Sanguosha
	out := []Action{}
	for _, from := range s.sgOrder(s.Turn) {
		for _, to := range s.sgOrder(s.Turn) {
			if from == to {
				continue
			}
			for _, id := range g.Players[from].Equip {
				if s.sgEquip(to, SGCardTypes[sgCard(id).Kind].Slot) == 0 {
					out = append(out, Action{Card: id, Targets: []int{from, to}})
				}
			}
			for _, d := range g.Players[from].Judgment {
				valid := !s.sgWeimu(actor, to, d.Kind, []int{d.Card}) && !(d.Kind == "indulgence" && s.sgHas(to, "qianxun"))
				for _, other := range g.Players[to].Judgment {
					if d.Kind == other.Kind {
						valid = false
					}
				}
				if valid {
					out = append(out, Action{Card: d.Card, Targets: []int{from, to}})
				}
			}
		}
	}
	return out
}
func (s *State) sgMountainSkill(i int, a Action) (bool, error) {
	g := s.Sanguosha
	p := &g.Players[i]
	switch a.Skill {
	case "jixi":
		if len(a.Cards) != 1 || !slices.Contains(p.Fields, a.Cards[0]) {
			return true, errors.New("急袭需要一张田")
		}
		p.Fields = sgRemove(p.Fields, a.Cards[0])
		return true, s.sgUse(i, "snatch", a.Cards, a.Targets, false)
	case "tiaoxin":
		if p.Used["tiaoxin"] > 0 {
			return true, errors.New("本阶段已发动挑衅")
		}
		if len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) || s.sgDistance(a.Targets[0], i) > s.sgRange(a.Targets[0]) {
			return true, errors.New("挑衅需要攻击范围包含你的另一角色")
		}
		p.Used["tiaoxin"]++
		s.sgPush(SGEvent{Type: "tiaoxin_response", Actor: i, Target: a.Targets[0]})
	case "zhijian":
		if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
			return true, err
		}
		slot := SGCardTypes[sgCard(a.Cards[0]).Kind].Slot
		if slot == "" || len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) || s.sgEquip(a.Targets[0], slot) != 0 {
			return true, errors.New("直谏需要装备手牌及另一角色空置的对应栏")
		}
		s.sgPush(SGEvent{Type: "draw", Actor: i, Amount: 1})
		s.sgLose(i, a.Cards)
		g.Players[a.Targets[0]].Equip = append(g.Players[a.Targets[0]].Equip, a.Cards[0])
		s.sgLog("%s 直谏：将「%s」放入 %s 的装备区", s.sgName(i), SGCardTypes[sgCard(a.Cards[0]).Kind].Name, s.sgName(a.Targets[0]))
	default:
		return false, nil
	}
	return true, nil
}
func (s *State) sgZhiba(i int, a Action) error {
	g := s.Sanguosha
	if i == g.Lord || !s.sgHas(g.Lord, "zhiba") || s.sgKingdom(i) != "wu" || g.Players[i].Used["zhiba_pindian"] > 0 || len(g.Players[g.Lord].Hand) == 0 {
		return errors.New("此时不能向主公发动制霸拼点")
	}
	if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
		return err
	}
	g.Players[i].Used["zhiba_pindian"]++
	s.sgPush(SGEvent{Type: "zhiba_start", Kind: "zhiba", Actor: i, Target: g.Lord, Cards: clone(a.Cards)})
	return nil
}
