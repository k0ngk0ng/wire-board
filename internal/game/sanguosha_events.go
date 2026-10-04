package game

import "slices"

func (s *State) sgAllPassed(passed []int) bool {
	for _, i := range s.sgOrder(0) {
		if !slices.Contains(passed, i) {
			return false
		}
	}
	return true
}

func (s *State) sgRun() {
	g := s.Sanguosha
	for !s.Finished && g.Pending == nil {
		if len(g.Queue) == 0 {
			if !g.Selecting && !s.sgAlive(s.Turn) {
				s.sgPush(SGEvent{Type: "next", Actor: s.Turn})
			} else {
				break
			}
		}
		e := g.Queue[0]
		g.Queue = g.Queue[1:]
		s.sgEvent(e)
	}
	if !s.Finished && !g.Selecting && g.Pending == nil && len(g.Queue) == 0 {
		s.Phase = "sg_play"
	}
	if s.Finished {
		g.Pending = nil
		g.Queue = nil
		s.Phase = "finished"
	}
}
func (s *State) sgEvent(e SGEvent) {
	g := s.Sanguosha
	if s.sgGuhuoEvent(e) {
		return
	}
	if s.sgWindEvent(e) {
		return
	}
	if s.sgFireEvent(e) {
		return
	}
	if s.sgMountainEvent(e) {
		return
	}
	if s.sgThicketEvent(e) {
		return
	}
	switch e.Type {
	case "cleanup":
		s.sgThicketCleanup(e)
		g.Revealed = nil
		s.sgFinishCards(e.Cards)
	case "draw":
		s.sgDraw(e.Actor, e.Amount)
	case "heal":
		s.sgHeal(e.Actor, e.Amount)
	case "optional_draw":
		if !s.sgAlive(e.Actor) {
			return
		}
		if e.Flag {
			s.sgDraw(e.Actor, e.Amount)
		} else {
			s.sgOptional(e.Actor, e.Kind, e)
		}
	case "begin":
		if !s.sgAlive(e.Actor) {
			s.sgPush(SGEvent{Type: "next", Actor: e.Actor})
			return
		}
		s.Turn = e.Actor
		g.TurnSequence++
		if g.Players[e.Actor].Flipped {
			g.Players[e.Actor].Flipped = false
			s.sgLog("%s 将武将牌翻回正面，跳过本回合", s.sgName(e.Actor))
			s.sgPush(SGEvent{Type: "next", Actor: e.Actor})
			return
		}
		s.Turn = e.Actor
		g.Players[e.Actor].Used = map[string]int{}
		g.SkipPlay = false
		g.SkipDraw = false
		g.SkipJudge = false
		g.SkipDiscard = false
		g.DiscardPhase = false
		g.InPlay = false
		s.Phase = "sg_start"
		s.sgLog("%s 的回合开始", s.sgName(e.Actor))
		s.sgPush(SGEvent{Type: "huashen_select", Actor: e.Actor}, SGEvent{Type: "mountain_start", Actor: e.Actor}, SGEvent{Type: "thicket_start", Actor: e.Actor}, SGEvent{Type: "guanxing", Actor: e.Actor}, SGEvent{Type: "luoshen", Actor: e.Actor}, SGEvent{Type: "qiaobian", Actor: e.Actor, Kind: "judge"}, SGEvent{Type: "shensu_judge", Actor: e.Actor}, SGEvent{Type: "delayed", Actor: e.Actor}, SGEvent{Type: "qiaobian", Actor: e.Actor, Kind: "draw"}, SGEvent{Type: "draw_phase", Actor: e.Actor}, SGEvent{Type: "qiaobian", Actor: e.Actor, Kind: "play"}, SGEvent{Type: "shensu_play", Actor: e.Actor}, SGEvent{Type: "fangquan", Actor: e.Actor}, SGEvent{Type: "play_phase", Actor: e.Actor})
	case "guanxing":
		if !s.sgHas(e.Actor, "guanxing") {
			return
		}
		if !e.Flag {
			s.sgOptional(e.Actor, "guanxing", e)
			return
		}
		cards := s.sgDrawIDs(min(5, len(s.sgOrder(0))))
		s.sgAsk(e.Actor, "guanxing", "将牌分配到牌堆顶或底，按选择顺序排列", e)
		g.Pending.Cards = cards
	case "luoshen":
		if s.sgHas(e.Actor, "luoshen") {
			if !e.Flag {
				s.sgOptional(e.Actor, "luoshen", e)
			} else {
				s.sgPush(SGEvent{Type: "judge", Actor: e.Actor, Kind: "luoshen"})
			}
		}
	case "draw_phase":
		if g.SkipDraw || !s.sgAlive(e.Actor) {
			return
		}
		if s.sgHas(e.Actor, "tuxi") || s.sgHas(e.Actor, "luoyi") || s.sgHas(e.Actor, "yingzi") || s.sgHas(e.Actor, "shuangxiong") || s.sgHas(e.Actor, "zaiqi") || s.sgHas(e.Actor, "haoshi") {
			s.sgAsk(e.Actor, "draw_phase", "选择摸牌阶段行动", e)
		} else {
			s.sgDraw(e.Actor, 2)
		}
	case "play_phase":
		if !s.sgAlive(e.Actor) || g.SkipPlay {
			s.sgPush(s.sgEndPhase(e.Actor)...)
		} else {
			s.Phase = "sg_play"
			g.InPlay = true
		}
	case "discard_phase":
		if !s.sgAlive(e.Actor) || g.SkipDiscard {
			return
		}
		p := g.Players[e.Actor]
		if !e.Flag && s.sgHas(e.Actor, "keji") && p.Used["keji_slash"] == 0 && len(p.Hand) > s.sgHandLimit(e.Actor) {
			s.sgAsk(e.Actor, "keji", "是否发动克己，跳过弃牌？", e)
			return
		}
		if !g.DiscardPhase {
			g.DiscardedHand = nil
			g.DiscardedOther = nil
			g.DiscardPhase = true
		}
		due := len(p.Hand) - s.sgHandLimit(e.Actor)
		if due > 0 {
			e.Amount = due
			s.sgAsk(e.Actor, "discard", "弃牌至当前体力值", e)
			g.Pending.Cards = p.Hand
		}
	case "next":
		for i := range g.Players {
			g.Players[i].Drank = 0
		}
		if len(g.ExtraTurns) > 0 {
			if len(g.ResumeTurns) == 0 {
				g.ResumeTurns = append(g.ResumeTurns, e.Actor)
			}
			extra := g.ExtraTurns[0]
			g.ExtraTurns = g.ExtraTurns[1:]
			s.sgLog("%s 的额外回合", s.sgName(extra))
			s.sgPush(SGEvent{Type: "begin", Actor: extra})
			return
		}
		if len(g.ResumeTurns) > 0 {
			e.Actor = g.ResumeTurns[0]
			g.ResumeTurns = nil
		}
		next := s.sgNext(e.Actor)
		if next <= e.Actor {
			s.Round++
		}
		s.sgPush(SGEvent{Type: "begin", Actor: next})
	case "equip":
		if !s.sgAlive(e.Actor) {
			s.sgFinishCards(e.Cards)
			return
		}
		slot := SGCardTypes[e.Kind].Slot
		if old := s.sgEquip(e.Actor, slot); old != 0 {
			s.sgSpent(e.Actor, []int{old})
		}
		for _, id := range e.Cards {
			s.sgTakeTable(id)
			g.Players[e.Actor].Equip = append(g.Players[e.Actor].Equip, id)
		}
	case "grace_reveal":
		g.Grace = s.sgDrawIDs(len(s.sgOrder(0)))
		s.sgLog("五谷丰登：展示 %d 张牌", len(g.Grace))
	case "grace_cleanup":
		g.Discard = append(g.Discard, g.Grace...)
		g.Grace = nil
	case "null_window":
		if !s.sgAlive(e.Target) {
			return
		}
		if s.sgAllPassed(e.Targets) {
			if !e.Flag {
				e.Type = "effect"
				s.sgPush(e)
			} else {
				if e.Aux == -2 {
					s.sgDelayCancelled(e)
				}
				s.sgLog("「%s」对 %s 的效果被无懈可击抵消", SGCardTypes[e.Kind].Name, s.sgName(e.Target))
			}
			return
		}
		s.sgAsk(-1, "nullification", "所有人同时响应：是否使用无懈可击？", e)
	case "effect":
		s.sgEffect(e)
	case "slash_start":
		if !s.sgAlive(e.Target) {
			return
		}
		if s.sgHas(e.Target, "kongcheng") && len(g.Players[e.Target].Hand) == 0 {
			return
		}
		e.Type = "xiangle"
		s.sgPush(e)
		if s.sgHas(e.Target, "liuli") && len(g.Players[e.Target].Hand)+len(g.Players[e.Target].Equip) > 0 {
			s.sgAsk(e.Target, "liuli", "是否弃一张牌发动流离，转移此杀？", e)
		}
	case "slash_weapon":
		if !s.sgAlive(e.Target) {
			return
		}
		e.Type = "slash_tieji"
		s.sgPush(e)
		if s.sgWeapon(e.Actor) == "double_sword" && s.sgFemale(e.Actor) != s.sgFemale(e.Target) {
			s.sgAsk(e.Actor, "double_sword", "是否发动雌雄双股剑？", e)
		}
	case "slash_tieji":
		if s.sgSlashImmune(e) {
			return
		}
		if s.sgHas(e.Actor, "liegong") && g.InPlay && e.Actor == s.Turn && (len(g.Players[e.Target].Hand) >= g.Players[e.Actor].HP || len(g.Players[e.Target].Hand) <= s.sgRange(e.Actor)) {
			s.sgAsk(e.Actor, "liegong", "是否发动烈弓，令目标不能使用闪？", e)
		} else if s.sgHas(e.Actor, "tieji") {
			s.sgAsk(e.Actor, "tieji", "是否发动铁骑？", e)
		} else {
			s.sgPush(s.sgSlashResponse(e))
		}
	case "response":
		s.sgResponsePrompt(e)
	case "defended":
		s.sgLog("%s 抵消了「杀」", s.sgName(e.Target))
		next := e
		next.Type = "weapon_missed"
		mengjin := e
		mengjin.Type = "mengjin"
		s.sgPush(mengjin, next)
	case "weapon_missed":
		if s.sgAlive(e.Actor) && (s.sgWeapon(e.Actor) == "axe" || s.sgWeapon(e.Actor) == "blade") {
			s.sgAsk(e.Actor, "weapon_after_jink", "是否发动武器效果？", e)
		}
	case "hit":
		if !s.sgAlive(e.Target) {
			return
		}
		if sgIsSlash(e.Kind) && s.sgWeapon(e.Actor) == "ice_sword" && len(g.Players[e.Target].Hand)+len(g.Players[e.Target].Equip) > 0 {
			s.sgAsk(e.Actor, "ice_sword", "是否用寒冰剑防止伤害，改为弃对方至多两张牌？", e)
			return
		}
		next := e
		next.Type = "damage"
		s.sgPush(next)
		if sgIsSlash(e.Kind) && s.sgWeapon(e.Actor) == "kylin_bow" && (s.sgEquip(e.Target, "offense") != 0 || s.sgEquip(e.Target, "defense") != 0) {
			s.sgAsk(e.Actor, "kylin_bow", "是否弃置目标的一张坐骑？", e)
		}
	case "chain_damage":
		if s.sgAlive(e.Target) && g.Players[e.Target].Chained {
			s.sgDamage(e)
		}
	case "damage":
		s.sgDamage(e)
	case "lose_hp":
		if s.sgAlive(e.Target) {
			g.Players[e.Target].HP -= e.Amount
			s.sgLog("%s 失去 %d 点体力", s.sgName(e.Target), e.Amount)
			if g.Players[e.Target].HP <= 0 {
				s.sgEnterDying(SGEvent{Actor: -1, Target: e.Target, Step: s.Turn})
			}
		}
	case "hurt":
		if !s.sgAlive(e.Target) {
			return
		}
		events := []SGEvent{}
		if sgIsSlash(e.Kind) {
			next := e
			next.Type = "beige"
			next.Step = 0
			events = append(events, next)
		}
		if s.sgHas(e.Target, "xinsheng") {
			next := e
			next.Type = "xinsheng"
			next.Flag = false
			events = append(events, next)
		}
		for _, skill := range []string{"jianxiong", "fankui", "ganglie", "yiji", "jieming", "fangzhu"} {
			if s.sgHas(e.Target, skill) {
				times := 1
				if skill == "yiji" {
					times = e.Amount
				}
				for range times {
					next := e
					next.Type = "hurt_skill"
					next.Kind = skill
					events = append(events, next)
				}
			}
		}
		s.sgPush(events...)
	case "hurt_skill":
		if !s.sgAlive(e.Target) {
			return
		}
		if e.Kind == "jieming" || e.Kind == "fangzhu" {
			e.Type = e.Kind
			s.sgPush(e)
			return
		}
		if e.Kind == "jianxiong" {
			ids := []int{}
			for _, id := range e.Cards {
				if slices.Contains(g.Table, id) {
					ids = append(ids, id)
				}
			}
			e.Cards = ids
			if len(ids) == 0 {
				return
			}
		}
		if (e.Kind == "fankui" || e.Kind == "ganglie") && !s.sgAlive(e.Actor) {
			return
		}
		if !e.Flag {
			s.sgOptional(e.Target, e.Kind, e)
			return
		}
		switch e.Kind {
		case "jianxiong":
			for _, id := range e.Cards {
				s.sgTakeTable(id)
				g.Players[e.Target].Hand = append(g.Players[e.Target].Hand, id)
			}
		case "fankui":
			if len(g.Players[e.Actor].Hand)+len(g.Players[e.Actor].Equip) > 0 {
				s.sgAsk(e.Target, "steal", "选择获得伤害来源的一张牌", SGEvent{Actor: e.Target, Target: e.Actor, Kind: "fankui"})
			}
		case "ganglie":
			s.sgPush(SGEvent{Type: "judge", Actor: e.Target, Target: e.Actor, Kind: "ganglie"})
		case "yiji":
			ids := s.sgDrawIDs(2)
			if len(ids) > 0 {
				s.sgAsk(e.Target, "yiji", "分配遗计的牌（可给自己）", e)
				g.Pending.Cards = ids
			}
		}
	case "dying":
		if !s.sgAlive(e.Target) || g.Players[e.Target].HP > 0 {
			return
		}
		if e.Count >= len(g.Players) {
			if s.sgBuquSafe(e.Target) {
				s.sgLog("%s 的不屈牌没有重复点数，脱离濒死", s.sgName(e.Target))
				return
			}
			s.sgDie(e.Target, e.Actor)
			return
		}
		who := (e.Step + e.Count) % len(g.Players)
		if !s.sgAlive(who) || s.sgHas(s.Turn, "wansha") && who != s.Turn && who != e.Target {
			e.Count++
			s.sgPush(e)
			return
		}
		if who == e.Target && !e.Flag && s.sgHas(who, "niepan") && g.Players[who].Marks["niepan"] == 0 {
			e.Flag = true
			s.sgAsk(who, "niepan", "是否发动涅槃？整局限一次，弃置所有牌、回复至3体力并摸三张", e)
			return
		}
		s.sgAsk(who, "peach", "濒死求桃：救助 "+s.sgName(e.Target), e)
	case "delayed":
		if g.SkipJudge {
			return
		}
		if !s.sgAlive(e.Actor) {
			return
		}
		ds := append([]SGDelayed{}, g.Players[e.Actor].Judgment...)
		es := []SGEvent{}
		for j := len(ds) - 1; j >= 0; j-- {
			d := ds[j]
			es = append(es, SGEvent{Type: "delay_take", Actor: e.Actor, Target: e.Actor, Kind: d.Kind, Cards: []int{d.Card}, Aux: -2, Step: s.Turn})
		}
		s.sgPush(es...)
	case "delay_take":
		p := &g.Players[e.Actor]
		found := false
		for j, d := range p.Judgment {
			if d.Card == e.Cards[0] {
				p.Judgment = append(p.Judgment[:j], p.Judgment[j+1:]...)
				found = true
				break
			}
		}
		if !found {
			return
		}
		s.sgPlaceTable(e.Actor, e.Cards)
		e.Type = "null_window"
		s.sgPush(e)
	case "judge":
		ids := s.sgDrawIDs(1)
		if len(ids) == 0 {
			return
		}
		e.Aux = ids[0]
		s.sgPlaceTable(e.Actor, []int{e.Aux})
		e.Type = "judge_replace"
		e.Step = 0
		s.sgPush(e)
	case "judge_replace":
		order := s.sgOrder(s.Turn)
		for e.Step < len(order) {
			i := order[e.Step]
			e.Step++
			if s.sgHas(i, "guidao") && len(g.Players[i].Hand)+len(g.Players[i].Equip) > 0 {
				s.sgAsk(i, "guidao", "鬼道：可用黑色牌替换并获得判定牌", e)
				g.Pending.Cards = []int{e.Aux}
				return
			}
			if s.sgHas(i, "guicai") && len(g.Players[i].Hand) > 0 {
				s.sgAsk(i, "guicai", "是否用手牌替换此次判定？", e)
				g.Pending.Cards = []int{e.Aux}
				return
			}
		}
		e.Type = "judge_result"
		s.sgPush(e)
	case "judge_result":
		s.sgJudgeResult(e)
	case "tiandu":
		if s.sgHas(e.Actor, "tiandu") && slices.Contains(g.Table, e.Aux) {
			s.sgAsk(e.Actor, "tiandu", "是否获得你的判定牌？", e)
		} else {
			s.sgFinishCards([]int{e.Aux})
		}
	case "ice_continue":
		if s.sgAlive(e.Target) && len(g.Players[e.Target].Hand)+len(g.Players[e.Target].Equip) > 0 {
			s.sgAsk(e.Actor, "steal", "寒冰剑：弃置第二张牌", e)
		}
	case "support":
		s.sgSupport(e)
	}
}
func (s *State) sgSlashResponse(e SGEvent) SGEvent {
	e.Type = "response"
	e.Count = 1
	if s.sgHas(e.Actor, "wushuang") || s.sgHas(e.Actor, "roulin") && s.sgFemale(e.Target) || s.sgHas(e.Target, "roulin") && s.sgFemale(e.Actor) {
		e.Count = 2
	}
	e.Step = 0
	return e
}
func (s *State) sgResponsePrompt(e SGEvent) {
	if !s.sgAlive(e.Target) {
		return
	}
	if s.sgSlashImmune(e) {
		return
	}
	s.sgAsk(e.Target, "card", "请响应「"+SGCardTypes[e.Kind].Name+"」", e)
}
func sgWanted(e SGEvent) string {
	if sgIsSlash(e.Kind) || e.Kind == "archery_attack" {
		return "jink"
	}
	return "slash"
}
func (s *State) sgResponseSuccess(e SGEvent) {
	if e.Count > 1 {
		e.Count--
		e.Step = 0
		s.sgPush(e)
		return
	}
	switch e.Kind {
	case "slash", "fire_slash", "thunder_slash":
		e.Type = "defended"
		s.sgPush(e)
	case "duel":
		e.Actor, e.Target = e.Target, e.Actor
		e.Count = 1
		if s.sgHas(e.Actor, "wushuang") {
			e.Count = 2
		}
		e.Step = 0
		s.sgPush(e)
	}
}
func (s *State) sgResponseFail(e SGEvent) {
	if e.Kind == "collateral" {
		id := s.sgEquip(e.Target, "weapon")
		if id != 0 {
			s.sgLose(e.Target, []int{id})
			s.Sanguosha.Players[e.Actor].Hand = append(s.Sanguosha.Players[e.Actor].Hand, id)
		}
		return
	}
	e.Type = "hit"
	s.sgPush(e)
}
func (s *State) sgEffect(e SGEvent) {
	g := s.Sanguosha
	if !s.sgAlive(e.Target) {
		return
	}
	if e.Aux == -2 {
		s.sgPush(SGEvent{Type: "judge", Actor: e.Target, Target: e.Actor, Kind: e.Kind, Cards: e.Cards})
		return
	}
	if e.Kind == "savage_assault" && (s.sgHas(e.Target, "huoshou") || s.sgHas(e.Target, "juxiang")) {
		s.sgLog("%s 的技能令南蛮入侵无效", s.sgName(e.Target))
		return
	}
	if (e.Kind == "savage_assault" || e.Kind == "archery_attack") && sgCard(s.sgEquip(e.Target, "armor")).Kind == "vine" {
		s.sgLog("%s 的藤甲令「%s」无效", s.sgName(e.Target), SGCardTypes[e.Kind].Name)
		return
	}
	switch e.Kind {
	case "analeptic":
		g.Players[e.Target].Drank++
	case "iron_chain":
		g.Players[e.Target].Chained = !g.Players[e.Target].Chained
		word := "重置"
		if g.Players[e.Target].Chained {
			word = "横置"
		}
		s.sgLog("%s 被%s", s.sgName(e.Target), word)
	case "fire_attack":
		if len(g.Players[e.Target].Hand) > 0 {
			s.sgAsk(e.Target, "fire_reveal", "火攻：展示一张手牌", e)
		}
	case "peach":
		s.sgHeal(e.Target, 1)
	case "ex_nihilo":
		s.sgDraw(e.Target, 2)
	case "god_salvation":
		s.sgHeal(e.Target, 1)
	case "amazing_grace":
		if len(g.Grace) > 0 {
			s.sgAsk(e.Target, "grace", "选择一张五谷丰登的牌", e)
			g.Pending.Cards = g.Grace
		}
	case "snatch", "dismantlement":
		if len(g.Players[e.Target].Hand)+len(g.Players[e.Target].Equip)+len(g.Players[e.Target].Judgment) > 0 {
			s.sgAsk(e.Actor, "steal", "选择目标区域的一张牌", e)
		}
	case "indulgence", "lightning", "supply_shortage":
		if len(e.Cards) > 0 {
			for _, id := range e.Cards {
				s.sgTakeTable(id)
			}
			g.Players[e.Target].Judgment = append(g.Players[e.Target].Judgment, SGDelayed{Card: e.Cards[0], Kind: e.Kind})
		}
	case "duel", "savage_assault", "archery_attack", "collateral":
		e.Type = "response"
		e.Count = 1
		if e.Kind == "duel" && s.sgHas(e.Actor, "wushuang") {
			e.Count = 2
		}
		e.Step = 0
		s.sgPush(e)
	}
}
func (s *State) sgPassLightning(e SGEvent) {
	g := s.Sanguosha
	for _, i := range s.sgOrder(s.sgNext(e.Actor)) {
		if i == e.Actor {
			continue
		}
		exists := false
		for _, d := range g.Players[i].Judgment {
			exists = exists || d.Kind == "lightning"
		}
		if !exists && !s.sgWeimu(e.Actor, i, "lightning", e.Cards) {
			s.sgTakeTable(e.Cards[0])
			g.Players[i].Judgment = append(g.Players[i].Judgment, SGDelayed{Card: e.Cards[0], Kind: "lightning"})
			return
		}
	}
	s.sgFinishCards(e.Cards)
}
func (s *State) sgDelayCancelled(e SGEvent) {
	if e.Kind == "lightning" {
		s.sgPassLightning(e)
	} else {
		s.sgFinishCards(e.Cards)
	}
}
func (s *State) sgJudgeResult(e SGEvent) {
	g := s.Sanguosha
	c := s.sgCardFor(e.Actor, e.Aux)
	red := c.Suit == 1 || c.Suit == 3
	s.sgLog("%s 的判定结果：%s %d「%s」", s.sgName(e.Actor), []string{"♠", "♥", "♣", "♦"}[c.Suit], c.Rank, SGCardTypes[c.Kind].Name)
	after := SGEvent{Type: "tiandu", Actor: e.Actor, Aux: e.Aux}
	switch e.Kind {
	case "tuntian":
		if c.Suit != 1 && slices.Contains(g.Table, c.ID) && s.sgHas(e.Actor, "tuntian") {
			s.sgTakeTable(c.ID)
			g.Players[e.Actor].Fields = append(g.Players[e.Actor].Fields, c.ID)
			s.sgLog("%s 屯田成功，现有 %d 张田", s.sgName(e.Actor), len(g.Players[e.Actor].Fields))
		}
	case "beige":
		switch c.Suit {
		case 1:
			s.sgHeal(e.Actor, 1)
		case 3:
			s.sgDraw(e.Actor, 2)
		case 2:
			s.sgPush(SGEvent{Type: "beige_discard", Actor: e.Target})
		case 0:
			if s.sgAlive(e.Target) {
				g.Players[e.Target].Flipped = !g.Players[e.Target].Flipped
				s.sgLog("%s 因悲歌翻面", s.sgName(e.Target))
			}
		}
	case "baonue":
		if c.Suit == 0 {
			s.sgHeal(e.Target, 1)
		}
	case "shuangxiong":
		g.Players[e.Actor].Used["shuangxiong"] = map[bool]int{true: 1, false: 2}[red]
		s.sgTakeTable(c.ID)
		g.Players[e.Actor].Hand = append(g.Players[e.Actor].Hand, c.ID)
	case "luoshen":
		if !red {
			s.sgTakeTable(c.ID)
			g.Players[e.Actor].Hand = append(g.Players[e.Actor].Hand, c.ID)
			s.sgPush(SGEvent{Type: "luoshen", Actor: e.Actor})
		}
	case "support_eight":
		if red {
			s.sgResponseSuccess(*e.Next.Next)
			s.sgJinkPlayed(e.Actor)
		} else {
			next := *e.Next
			next.Step |= 1
			next.Count--
			s.sgPush(next)
		}
	case "eight_diagram":
		if red {
			s.sgResponseSuccess(*e.Next)
			s.sgJinkPlayed(e.Actor)
		} else {
			next := *e.Next
			next.Step |= 1
			s.sgPush(next)
		}
	case "tieji":
		next := *e.Next
		if red {
			next.Type = "hit"
			s.sgPush(next)
		} else {
			s.sgPush(s.sgSlashResponse(next))
		}
	case "leiji":
		if c.Suit == 0 {
			s.sgPush(SGEvent{Type: "damage", Actor: e.Target, Target: e.Actor, Kind: "leiji", Nature: "thunder", Amount: 2})
		}
	case "ganglie":
		if c.Suit != 1 && s.sgAlive(e.Target) {
			s.sgAsk(e.Target, "ganglie", "弃两张手牌，否则受到刚烈的1点伤害", e)
		}
	case "supply_shortage":
		if c.Suit != 2 {
			g.SkipDraw = true
			s.sgLog("%s 因兵粮寸断跳过摸牌阶段", s.sgName(e.Actor))
		}
		s.sgFinishCards(e.Cards)
	case "indulgence":
		if c.Suit != 1 {
			g.SkipPlay = true
			s.sgLog("%s 因乐不思蜀跳过出牌阶段", s.sgName(e.Actor))
		}
		s.sgFinishCards(e.Cards)
	case "lightning":
		if c.Suit == 0 && c.Rank >= 2 && c.Rank <= 9 {
			s.sgPush(SGEvent{Type: "damage", Actor: -1, Target: e.Actor, Kind: "lightning", Cards: e.Cards, Amount: 3, Nature: "thunder"}, SGEvent{Type: "cleanup", Cards: e.Cards})
		} else {
			s.sgPassLightning(e)
		}
	}
	if !red && e.Actor != g.Lord && s.sgAlive(e.Actor) && s.sgKingdom(e.Actor) == "wei" && s.sgHas(g.Lord, "songwei") {
		s.sgPush(SGEvent{Type: "songwei", Actor: e.Actor, Target: g.Lord})
	}
	s.sgPush(after)
}
func (s *State) sgDie(i, killer int) {
	g := s.Sanguosha
	p := &g.Players[i]
	duanchang := s.sgHas(i, "duanchang")
	p.Dead = true
	s.sgLog("%s 阵亡，身份为%s", s.sgName(i), map[string]string{"lord": "主公", "loyalist": "忠臣", "rebel": "反贼", "renegade": "内奸"}[p.Role])
	alive := s.sgOrder(0)
	winners := []int{}
	if p.Role == "lord" {
		if len(alive) == 1 && g.Players[alive[0]].Role == "renegade" {
			winners = alive
		} else {
			for j, x := range g.Players {
				if x.Role == "rebel" {
					winners = append(winners, j)
				}
			}
		}
	} else {
		enemies := false
		for _, j := range alive {
			r := g.Players[j].Role
			enemies = enemies || r == "rebel" || r == "renegade"
		}
		if !enemies {
			for j, x := range g.Players {
				if x.Role == "lord" || x.Role == "loyalist" {
					winners = append(winners, j)
				}
			}
		}
	}
	if len(winners) > 0 {
		s.sgDeathClear(i)
		s.Finished = true
		s.Winners = winners
		s.sgLog("本局结束，获胜阵营的所有成员共同获胜")
		return
	}
	s.sgPush(SGEvent{Type: "death_loot", Actor: killer, Target: i})
	if duanchang {
		s.sgLoseSkills(killer)
	}
}
func (s *State) sgSupport(e SGEvent) {
	g := s.Sanguosha
	for e.Count < len(g.Players) {
		who := (e.Actor + 1 + e.Count) % len(g.Players)
		e.Count++
		if who == e.Actor {
			continue
		}
		kingdom := "wei"
		if e.Kind == "jijiang" {
			kingdom = "shu"
		}
		if s.sgAlive(who) && s.sgKingdom(who) == kingdom {
			e.Target = who
			s.sgAsk(who, "support", s.sgName(e.Actor)+" 请求「"+SGSkills[e.Kind].Name+"」", e)
			return
		}
	}
	if e.Next != nil {
		next := *e.Next
		next.Step |= 2
		s.sgPush(next)
	} else {
		g.Players[e.Actor].Used["jijiang_failed"] = 1
		s.sgLog("无人响应激将")
	}
}

func (s *State) sgDeathClear(i int) {
	g := s.Sanguosha
	p := &g.Players[i]
	g.Discard = append(g.Discard, p.Hand...)
	g.Discard = append(g.Discard, p.Equip...)
	g.Discard = append(g.Discard, p.Buqu...)
	g.Discard = append(g.Discard, p.Fields...)
	p.Fields = nil
	p.Buqu = nil
	p.BuquActive = false
	for _, d := range p.Judgment {
		g.Discard = append(g.Discard, d.Card)
	}
	p.Hand = []int{}
	p.Equip = []int{}
	p.Judgment = []SGDelayed{}

}
func (s *State) sgDeathReward(i, killer int) {
	g := s.Sanguosha
	p := &g.Players[i]
	if s.sgAlive(killer) {
		if p.Role == "rebel" {
			s.sgDraw(killer, 3)
		} else if p.Role == "loyalist" && g.Players[killer].Role == "lord" {
			ids := append(append([]int{}, g.Players[killer].Hand...), g.Players[killer].Equip...)
			s.sgDiscard(killer, ids)
			s.sgLog("主公误杀忠臣，弃置全部手牌和装备")
		}
	}
}
