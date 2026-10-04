package game

import (
	"errors"
	"slices"
)

// Classic Wind: the nostalgia variants of Caoren, Zhoutai, Zhangjiao and Yuji
// are intentionally different from the later revised Wind package.
var sgWindGenerals = []SGGeneral{
	{"caoren", "曹仁", "wei", 4, false, []string{"jushou"}},
	{"xiahouyuan", "夏侯渊", "wei", 4, false, []string{"shensu"}},
	{"huangzhong", "黄忠", "shu", 4, false, []string{"liegong"}},
	{"weiyan", "魏延", "shu", 4, false, []string{"kuanggu"}},
	{"xiaoqiao", "小乔", "wu", 3, true, []string{"tianxiang", "hongyan"}},
	{"zhoutai", "周泰", "wu", 4, false, []string{"buqu"}},
	{"zhangjiao", "张角", "qun", 3, false, []string{"leiji", "guidao", "huangtian"}},
	{"yuji", "于吉", "qun", 3, false, []string{"guhuo"}},
}

func init() {
	for k, v := range map[string]SGSkill{
		"jushou":         {"据守", "结束阶段可以摸三张牌，然后将武将牌翻面。背面朝上时跳过下个回合并翻回正面。"},
		"shensu":         {"神速", "可以选择：跳过判定和摸牌阶段；或跳过出牌阶段并弃置一张装备牌。每选择一项，视为使用一张无距离限制的杀。"},
		"liegong":        {"烈弓", "出牌阶段使用杀指定目标时，若其手牌数不少于你的体力值，或不大于你的攻击范围，可以令其不能使用闪。"},
		"kuanggu":        {"狂骨", "锁定技：每对距离1以内的角色造成1点伤害，回复1点体力。距离在伤害发生时计算。"},
		"tianxiang":      {"天香", "受到伤害时，可以弃置一张红桃手牌，将伤害转移给另一名角色；该角色结算伤害后，摸其已损失体力值张牌。"},
		"hongyan":        {"红颜", "锁定技：你的黑桃牌均视为红桃牌，包括你的判定牌。"},
		"buqu":           {"不屈", "体力扣减到0或更低时，可以将牌堆顶的牌置为不屈牌，使其数量等于1减当前体力。若点数均不相同，你不会因此濒死；重复则继续求桃。回复体力后移去相应数量的不屈牌。"},
		"leiji":          {"雷击", "每当你使用或打出闪后，可以令一名角色判定；若为黑桃，对其造成2点雷电伤害。"},
		"guidao":         {"鬼道", "判定牌生效前，可以用一张黑色手牌或装备牌替换它，并获得原判定牌。"},
		"huangtian":      {"黄天", "主公技：其他群势力角色的出牌阶段限一次，可将一张闪或闪电交给你。"},
		"huangtian_give": {"黄天·给牌", "出牌阶段限一次，将一张闪或闪电手牌交给拥有黄天的主公。"},
		"guhuo":          {"蛊惑", "声明一张基本牌或非延时锦囊，将一张手牌扣下。其他体力大于0的角色可以质疑；无人质疑则生效，有人质疑时仅真实的红桃牌生效。质疑真实牌者失去1点体力，质疑虚假牌者摸一张牌。"},
	} {
		SGSkills[k] = v
	}
}

func (g *Sanguosha) generalCatalog() []SGGeneral {
	all := append([]SGGeneral{}, SGGenerals...)
	if g.Options.StandardVersion == "breakthrough" {
		for i, general := range all {
			if revised := sgGeneral("jie_" + general.ID); revised.ID != "" {
				all[i] = revised
			}
		}
	}
	if slices.Contains(g.Options.Packs, "wind") {
		all = append(all, sgWindGenerals...)
	}
	if slices.Contains(g.Options.Packs, "fire") {
		all = append(all, sgFireGenerals...)
	}
	if slices.Contains(g.Options.Packs, "thicket") {
		all = append(all, sgThicketGenerals...)
	}
	if slices.Contains(g.Options.Packs, "mountain") {
		all = append(all, sgMountainGenerals...)
	}
	if slices.Contains(g.Options.Packs, "god") {
		all = append(all, sgGodGenerals...)
	}
	return all
}
func (s *State) sgLordChoices() {
	g := s.Sanguosha
	lords := []string{"caocao", "liubei", "sunquan"}
	if g.Options.StandardVersion == "breakthrough" {
		lords = []string{"jie_caocao", "jie_liubei", "sunquan"}
	}
	if slices.Contains(g.Options.Packs, "wind") {
		lords = append(lords, "zhangjiao")
	}
	if slices.Contains(g.Options.Packs, "fire") {
		lords = append(lords, "yuanshao")
	}
	if slices.Contains(g.Options.Packs, "thicket") {
		lords = append(lords, "caopi", "dongzhuo")
	}
	if slices.Contains(g.Options.Packs, "mountain") {
		lords = append(lords, "liushan", "sunce")
	}
	ids := []string{}
	for _, general := range g.generalCatalog() {
		if !slices.Contains(lords, general.ID) {
			ids = append(ids, general.ID)
		}
	}
	shuffle(ids)
	g.Players[g.Lord].Choices = append(lords, ids[:min(2, len(ids))]...)
}

func (s *State) sgCardFor(i, id int) SGCard {
	c := sgCard(id)
	if c.Suit == 0 && s.sgHas(i, "hongyan") {
		c.Suit = 1
	}
	if c.Suit == 1 && s.sgHas(i, "wushen") && slices.Contains(s.Sanguosha.Players[i].Hand, id) {
		c.Kind = "slash"
	}
	return c
}
func (s *State) sgCardColor(i int, ids []int) int {
	color := 0
	for _, id := range ids {
		c := s.sgCardFor(i, id)
		next := 2
		if c.Suit == 1 || c.Suit == 3 {
			next = 1
		}
		if color != 0 && color != next {
			return 3
		}
		color = next
	}
	if color == 0 {
		return 3
	}
	return color
}

func (s *State) sgEndPhase(i int) []SGEvent {
	return []SGEvent{{Type: "qiaobian", Actor: i, Kind: "discard"}, {Type: "discard_phase", Actor: i}, {Type: "qinyin", Actor: i}, {Type: "guzheng", Actor: i}, {Type: "stage", Kind: "finish"}, {Type: "optional_draw", Actor: i, Kind: "biyue", Amount: 1}, {Type: "benghuai", Actor: i}, {Type: "god_finish", Actor: i}, {Type: "wind_finish", Actor: i}, {Type: "fangquan_finish", Actor: i}, {Type: "huashen_select", Actor: i}, {Type: "next", Actor: i}}
}

// A separate event stage permits transfer before recipient armor, and avoids
// applying attacker bonuses again when Tianxiang changes the recipient.
func (s *State) sgWindDamage(e SGEvent) bool {
	if e.DamageStage >= 2 || !s.sgHas(e.Target, "tianxiang") {
		return false
	}
	for _, id := range s.Sanguosha.Players[e.Target].Hand {
		if s.sgCardFor(e.Target, id).Suit == 1 {
			s.sgAsk(e.Target, "tianxiang", "天香：弃一张红桃手牌并选择转移目标，或放弃", e)
			return true
		}
	}
	return false
}

func (s *State) sgWindEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "wind_finish":
		if !s.sgHas(e.Actor, "jushou") {
			return true
		}
		if !e.Flag {
			s.sgOptional(e.Actor, "jushou", e)
		} else {
			s.sgDraw(e.Actor, 3)
			g.Players[e.Actor].Flipped = !g.Players[e.Actor].Flipped
			s.sgLog("%s 发动据守，将武将牌翻面", s.sgName(e.Actor))
		}
	case "shensu_judge", "shensu_play":
		if !s.sgHas(e.Actor, "shensu") || (e.Type == "shensu_play" && g.SkipPlay) {
			return true
		}
		if e.Type == "shensu_play" {
			found := false
			for _, id := range append(append([]int{}, g.Players[e.Actor].Hand...), g.Players[e.Actor].Equip...) {
				found = found || SGCardTypes[sgCard(id).Kind].Slot != ""
			}
			if !found {
				return true
			}
		}
		s.sgAsk(e.Actor, e.Type, "神速：选择杀的目标，或放弃；跳过出牌阶段还需弃一张装备牌", e)
	case "leiji":
		if s.sgHas(e.Actor, "leiji") {
			s.sgAsk(e.Actor, "leiji", "雷击：选择一名角色判定，黑桃则造成2点雷电伤害", e)
		}
	case "damage_dealt":
		s.sgGodDamageDealt(e)
		s.sgThicketDamageDealt(e)
		if e.Near && s.sgHas(e.Actor, "kuanggu") {
			s.sgHeal(e.Actor, e.Amount)
		}
	case "tianxiang_draw":
		if s.sgAlive(e.Target) {
			p := g.Players[e.Target]
			s.sgDraw(e.Target, p.MaxHP-p.HP)
		}
	case "buqu_enter":
		if !s.sgAlive(e.Target) || g.Players[e.Target].HP > 0 {
			return true
		}
		if s.sgHas(e.Target, "buqu") {
			s.sgAsk(e.Target, "buqu", "是否发动不屈，补充不屈牌并检查重复点数？", e)
		} else {
			e.Type = "dying"
			s.sgPush(e)
		}
	case "buqu_trim":
		if !s.sgAlive(e.Actor) {
			return true
		}
		p := &g.Players[e.Actor]
		need := max(0, 1-p.HP)
		if need == 0 {
			g.Discard = append(g.Discard, p.Buqu...)
			p.Buqu = nil
			p.BuquActive = false
			return true
		}
		if len(p.Buqu) > need {
			e.Amount = len(p.Buqu) - need
			s.sgAsk(e.Actor, "buqu_remove", "回复体力：选择移去多余的不屈牌", e)
			g.Pending.Cards = clone(p.Buqu)
		}
	default:
		return false
	}
	return true
}

func (s *State) sgBuquSafe(i int) bool {
	p := s.Sanguosha.Players[i]
	if !p.BuquActive || !s.sgHas(i, "buqu") || len(p.Buqu) != max(0, 1-p.HP) {
		return false
	}
	seen := map[int]bool{}
	for _, id := range p.Buqu {
		rank := sgCard(id).Rank
		if seen[rank] {
			return false
		}
		seen[rank] = true
	}
	return true
}

func (s *State) sgEnterDying(e SGEvent) {
	e.Type = "dying"
	if s.sgHas(e.Target, "buqu") {
		e.Type = "buqu_enter"
	}
	s.sgPush(e)
}
func (s *State) sgJinkPlayed(i int) {
	if s.sgHas(i, "leiji") {
		s.sgPush(SGEvent{Type: "leiji", Actor: i})
	}
}

func (s *State) sgWindRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	e := q.Event
	p := &g.Players[i]
	pass := a.Choice == "pass"
	switch q.Kind {
	case "shensu_judge", "shensu_play":
		if pass {
			return true, nil
		}
		if len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) {
			return true, errors.New("神速需要另一名存活角色")
		}
		t := a.Targets[0]
		if s.sgHas(t, "kongcheng") && len(g.Players[t].Hand) == 0 {
			return true, errors.New("空城角色不能成为杀的目标")
		}
		if q.Kind == "shensu_play" {
			if err := s.sgValidateCards(i, a.Cards, 1, false); err != nil {
				return true, err
			}
			if SGCardTypes[sgCard(a.Cards[0]).Kind].Slot == "" {
				return true, errors.New("神速需要弃一张装备牌")
			}
			g.SkipPlay = true
		} else {
			if len(a.Cards) != 0 {
				return true, errors.New("神速跳过判定与摸牌无需弃牌")
			}
			g.SkipJudge = true
			g.SkipDraw = true
		}
		amount := 1 + p.Drank
		p.Drank = 0
		s.sgPush(SGEvent{Type: "slash_start", Actor: i, Target: t, Kind: "slash", Amount: amount, Color: 3})
		if q.Kind == "shensu_play" {
			s.sgDiscard(i, a.Cards)
		}
		s.sgLog("%s 发动神速，对 %s 使用杀", s.sgName(i), s.sgName(t))
	case "liegong":
		if pass {
			s.sgPush(s.sgSlashResponse(e))
		} else if a.Choice == "yes" {
			e.Type = "hit"
			s.sgPush(e)
			s.sgLog("%s 发动烈弓，目标不能使用闪", s.sgName(i))
		} else {
			return true, errors.New("请选择发动或放弃")
		}
	case "tianxiang":
		if pass {
			e.DamageStage = 2
			e.Type = "damage"
			s.sgPush(e)
			return true, nil
		}
		if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
			return true, err
		}
		if s.sgCardFor(i, a.Cards[0]).Suit != 1 || len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) {
			return true, errors.New("天香需弃红桃手牌并选择另一名角色")
		}
		e.Target = a.Targets[0]
		e.Type = "damage"
		e.Transfer = true
		e.Foreseen = false
		e.DamageStage = 1
		s.sgPush(e)
		s.sgDiscard(i, a.Cards)
		s.sgLog("%s 发动天香，将伤害转移给 %s", s.sgName(i), s.sgName(e.Target))
	case "leiji":
		if pass {
			return true, nil
		}
		if len(a.Targets) != 1 || !s.sgAlive(a.Targets[0]) {
			return true, errors.New("请选择一名雷击判定角色")
		}
		s.sgPush(SGEvent{Type: "judge", Actor: a.Targets[0], Target: i, Kind: "leiji"})
		s.sgLog("%s 发动雷击，令 %s 判定", s.sgName(i), s.sgName(a.Targets[0]))
	case "guidao":
		if !pass {
			if err := s.sgValidateCards(i, a.Cards, 1, false); err != nil {
				return true, err
			}
			c := s.sgCardFor(i, a.Cards[0])
			if c.Suit != 0 && c.Suit != 2 {
				return true, errors.New("鬼道需要黑色手牌或装备牌")
			}
			old := e.Aux
			s.sgTakeTable(old)
			s.sgPay(i, a.Cards)
			s.sgGain(i, []int{old})
			e.Aux = a.Cards[0]
			s.sgLog("%s 发动鬼道，替换并获得原判定牌", s.sgName(i))
		}
		s.sgPush(e)
	case "buqu":
		p.BuquActive = !pass
		if !pass {
			if a.Choice != "yes" {
				return true, errors.New("请选择发动不屈或放弃")
			}
			need := max(0, 1-p.HP-len(p.Buqu))
			p.Buqu = append(p.Buqu, s.sgDrawIDs(need)...)
			s.sgLog("%s 发动不屈，当前 %d 张不屈牌", s.sgName(i), len(p.Buqu))
			if s.sgBuquSafe(i) {
				return true, nil
			}
		}
		e.Type = "dying"
		s.sgPush(e)
	case "buqu_remove":
		if len(a.Cards) != e.Amount || !sgSubset(a.Cards, p.Buqu) {
			return true, errors.New("请选择需要移去的不屈牌")
		}
		for _, id := range a.Cards {
			p.Buqu = sgRemove(p.Buqu, id)
			g.Discard = append(g.Discard, id)
		}
	default:
		return false, nil
	}
	return true, nil
}

func (s *State) sgHuangtianGive(i int, a Action) error {
	g := s.Sanguosha
	p := &g.Players[i]
	if s.sgKingdom(i) != "qun" || i == g.Lord || !s.sgHas(g.Lord, "huangtian") || p.Used["huangtian_give"] > 0 {
		return errors.New("此时不能向主公发动黄天")
	}
	if len(a.Targets) != 1 || a.Targets[0] != g.Lord {
		return errors.New("黄天的目标必须是拥有此技能的主公")
	}
	if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
		return err
	}
	kind := sgCard(a.Cards[0]).Kind
	if kind != "jink" && kind != "lightning" {
		return errors.New("黄天只能交出闪或闪电")
	}
	s.sgLose(i, a.Cards)
	s.sgGain(g.Lord, a.Cards)
	p.Used["huangtian_give"]++
	s.sgLog("%s 发动黄天，将「%s」交给主公", s.sgName(i), SGCardTypes[kind].Name)
	return nil
}

// Apply Hongyan only to cards whose ownership is already visible. Filtering
// another player's hidden hand in the shared catalog would reveal its IDs.
func (s *State) sgVisibleCatalog(viewer int) []SGCard {
	g := s.Sanguosha
	cards := append([]SGCard{}, g.cardCatalog()...)
	for _, id := range g.Table {
		if suit, ok := g.TableSuits[id]; ok && id > 0 && id <= len(cards) {
			cards[id-1].Suit = suit
		}
	}
	for id, kind := range g.TableKinds {
		if id > 0 && id <= len(cards) {
			cards[id-1].Kind = kind
		}
	}
	filter := func(owner, id int) {
		if id > 0 && id <= len(cards) {
			cards[id-1] = s.sgCardFor(owner, id)
		}
	}
	for i, p := range g.Players {
		for _, id := range p.Equip {
			filter(i, id)
		}
		for _, d := range p.Judgment {
			filter(i, d.Card)
		}
		for _, id := range p.Hand {
			if i == viewer || slices.Contains(g.Revealed, id) || g.Pending != nil && g.Pending.Kind == "gongxin" && g.Pending.Player == viewer && g.Pending.Event.Target == i {
				filter(i, id)
			}
		}
	}
	if q := g.Pending; q != nil && (q.Kind == "guicai" || q.Kind == "jie_guicai" || q.Kind == "jilve_guicai" || q.Kind == "guidao" || q.Kind == "tiandu") {
		filter(q.Event.Actor, q.Event.Aux)
	}
	return cards
}
