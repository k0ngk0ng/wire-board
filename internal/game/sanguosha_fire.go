package game

import (
	"errors"
	"slices"
)

var sgFireGenerals = []SGGeneral{
	{"dianwei", "典韦", "wei", 4, false, []string{"qiangxi"}},
	{"xunyu", "荀彧", "wei", 3, false, []string{"quhu", "jieming"}},
	{"wolong", "卧龙诸葛亮", "shu", 3, false, []string{"huoji", "bazhen", "kanpo"}},
	{"pangtong", "庞统", "shu", 3, false, []string{"lianhuan", "niepan"}},
	{"taishici", "太史慈", "wu", 4, false, []string{"tianyi"}},
	{"yuanshao", "袁绍", "qun", 4, false, []string{"luanji", "xueyi"}},
	{"yanliangwenchou", "颜良文丑", "qun", 4, false, []string{"shuangxiong"}},
	{"pangde", "庞德", "qun", 4, false, []string{"mashu", "mengjin"}},
}

func init() {
	for k, v := range map[string]SGSkill{
		"qiangxi":     {"强袭", "出牌阶段限一次：失去1点体力或弃置一张武器牌，对攻击范围内另一名角色造成1点伤害。弃装备区武器时，按失去该武器后的范围计算。"},
		"quhu":        {"驱虎", "出牌阶段限一次：与体力值大于你的角色拼点。若赢，你令其对其攻击范围内由你选择的另一名角色造成1点伤害；否则其对你造成1点伤害。拼点同点视为没赢。"},
		"jieming":     {"节命", "每受到1点伤害后，可以令一名角色将手牌补至体力上限（最多5张）；多点伤害逐次选择。"},
		"huoji":       {"火计", "可以将一张红色手牌当火攻使用。"},
		"bazhen":      {"八阵", "锁定技：若没有装备防具，视为装备八卦阵。需要闪时仍可自行选择是否判定；青釭剑可无视八阵。"},
		"kanpo":       {"看破", "可以将一张黑色手牌当无懈可击使用。"},
		"lianhuan":    {"连环", "可以将一张梅花手牌当铁索连环使用或重铸。"},
		"niepan":      {"涅槃", "限定技：濒死求桃轮到你时，可以弃置所有手牌、装备和判定区的牌，将体力回复至3点（不超过上限），摸三张牌，重置武将牌并翻至正面。整局限一次。"},
		"tianyi":      {"天义", "出牌阶段限一次：与另一名角色拼点。若赢，本回合使用杀无距离限制、可多使用一张杀且每张杀可多选一名目标；否则本回合不能使用杀。拼点同点视为没赢。"},
		"luanji":      {"乱击", "可以将两张花色相同的手牌当万箭齐发使用。"},
		"xueyi":       {"血裔", "主公技，锁定技：手牌上限增加其他存活群势力角色数的两倍。"},
		"shuangxiong": {"双雄", "摸牌阶段可以改为判定并获得判定牌。本回合可将与判定牌颜色不同的一张手牌当决斗使用。"},
		"mengjin":     {"猛进", "你的杀被闪抵消后，可以弃置目标一张手牌或装备牌。"},
	} {
		SGSkills[k] = v
	}
}

func (s *State) sgArmor(i int) string {
	if s.sgGodArmorOff(i) {
		return ""
	}
	id := s.sgEquip(i, "armor")
	if id != 0 {
		return sgCard(id).Kind
	}
	if s.sgHas(i, "bazhen") {
		return "eight_diagram"
	}
	return ""
}
func (s *State) sgTianyi(i int) int {
	if i != s.Turn || !s.sgAlive(i) {
		return 0
	}
	return s.Sanguosha.Players[i].Used["tianyi_result"]
}
func (s *State) sgHandLimit(i int) int {
	n := max(0, s.Sanguosha.Players[i].HP)
	if s.sgHas(i, "juejing") {
		n += 2
	}
	if s.sgHas(i, "xueyi") {
		for _, j := range s.sgOrder(0) {
			if j != i && s.sgKingdom(j) == "qun" {
				n += 2
			}
		}
	}
	return n
}
func (s *State) sgFireSkill(i int, a Action) (bool, error) {
	if !slices.Contains([]string{"qiangxi", "quhu", "tianyi"}, a.Skill) {
		return false, nil
	}
	g := s.Sanguosha
	p := &g.Players[i]
	if p.Used[a.Skill] > 0 {
		return true, errors.New("本阶段已经发动此技能")
	}
	if len(a.Targets) != 1 || !s.sgAlive(a.Targets[0]) || a.Targets[0] == i {
		return true, errors.New("请选择另一名存活角色")
	}
	target := a.Targets[0]
	if a.Skill == "qiangxi" {
		if len(a.Cards) > 1 {
			return true, errors.New("强袭只能弃一张武器，或不选牌失去体力")
		}
		if err := s.sgValidateCards(i, a.Cards, -1, false); err != nil {
			return true, err
		}
		if len(a.Cards) == 1 && SGCardTypes[sgCard(a.Cards[0]).Kind].Slot != "weapon" {
			return true, errors.New("强袭需要武器牌")
		}
		trial := clone(*s)
		trial.sgLose(i, a.Cards)
		if trial.sgDistance(i, target) > trial.sgRange(i) {
			return true, errors.New("强袭目标不在支付后的攻击范围内")
		}
		p.Used[a.Skill]++
		es := []SGEvent{}
		if len(a.Cards) == 0 {
			es = append(es, SGEvent{Type: "lose_hp", Target: i, Amount: 1})
		}
		es = append(es, SGEvent{Type: "damage", Actor: i, Target: target, Kind: "qiangxi", Amount: 1})
		s.sgPush(es...)
		s.sgDiscard(i, a.Cards)
		s.sgLog("%s 发动强袭 → %s", s.sgName(i), s.sgName(target))
		return true, nil
	}
	if len(g.Players[target].Hand) == 0 {
		return true, errors.New("拼点目标必须有手牌")
	}
	if a.Skill == "quhu" && g.Players[target].HP <= p.HP {
		return true, errors.New("驱虎需要体力值大于你的角色")
	}
	if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
		return true, err
	}
	p.Used[a.Skill]++
	// The initiator's choice is committed privately. Both cards stay in hand
	// until the target commits, so no loss trigger can reveal/replace the choice.
	s.sgAsk(target, "pindian", s.sgName(i)+" 发动「"+SGSkills[a.Skill].Name+"」：选择一张手牌拼点", SGEvent{Type: "pindian_result", Actor: i, Target: target, Kind: a.Skill, Cards: clone(a.Cards)})
	s.sgLog("%s 对 %s 发动「%s」拼点，双方选牌后同时揭示", s.sgName(i), s.sgName(target), SGSkills[a.Skill].Name)
	return true, nil
}
func (s *State) sgFireEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "pindian_result":
		a, b := sgCard(e.Cards[0]), sgCard(e.Cards[1])
		win := a.Rank > b.Rank
		s.sgLog("%s 拼点：%s %s%d「%s」，%s %s%d「%s」；%s%s", SGSkills[e.Kind].Name, s.sgName(e.Actor), []string{"♠", "♥", "♣", "♦"}[s.sgCardFor(e.Actor, a.ID).Suit], a.Rank, SGCardTypes[a.Kind].Name, s.sgName(e.Target), []string{"♠", "♥", "♣", "♦"}[s.sgCardFor(e.Target, b.ID).Suit], b.Rank, SGCardTypes[b.Kind].Name, s.sgName(e.Actor), map[bool]string{true: "胜出", false: "未胜出"}[win])
		s.sgPush(SGEvent{Type: "cleanup", Cards: e.Cards})
		if e.Kind == "zhiba" {
			if !win {
				e.Type = "zhiba_obtain"
				s.sgPush(e)
			}
			return true
		}
		if e.Kind == "tianyi" {
			g.Players[e.Actor].Used["tianyi_result"] = map[bool]int{true: 1, false: -1}[win]
			return true
		}
		if e.Kind == "lieren" {
			if win && s.sgAlive(e.Actor) && s.sgAlive(e.Target) && len(g.Players[e.Target].Hand)+len(g.Players[e.Target].Equip) > 0 {
				s.sgAsk(e.Actor, "steal", "烈刃胜出：获得目标一张手牌或装备", e)
			}
			return true
		}
		if !win {
			s.sgPush(SGEvent{Type: "damage", Actor: e.Target, Target: e.Actor, Kind: "quhu", Amount: 1})
			return true
		}
		targets := []int{}
		for _, i := range s.sgOrder(s.Turn) {
			if i != e.Target && s.sgDistance(e.Target, i) <= s.sgRange(e.Target) {
				targets = append(targets, i)
			}
		}
		if s.sgAlive(e.Actor) && s.sgAlive(e.Target) && len(targets) > 0 {
			s.sgAsk(e.Actor, "quhu_target", "驱虎胜出：选择对方攻击范围内的一名角色受到1点伤害", e)
			g.Pending.Targets = targets
		}
	case "jieming":
		if s.sgHas(e.Target, "jieming") && e.Amount > 0 {
			s.sgAsk(e.Target, "jieming", "节命：选择一名角色补充手牌，或放弃剩余次数", e)
		}
	case "mengjin":
		if s.sgHas(e.Actor, "mengjin") && s.sgAlive(e.Target) && len(g.Players[e.Target].Hand)+len(g.Players[e.Target].Equip) > 0 {
			s.sgAsk(e.Actor, "mengjin", "是否发动猛进，弃置目标一张手牌或装备？", e)
		}
	case "niepan_rebirth":
		if s.sgAlive(e.Actor) {
			p := &g.Players[e.Actor]
			s.sgHeal(e.Actor, max(0, min(3, p.MaxHP)-p.HP))
			s.sgDraw(e.Actor, 3)
			p.Chained = false
			p.Flipped = false
			s.sgLog("%s 发动涅槃，回复体力并重置武将牌", s.sgName(e.Actor))
		}
	default:
		return false
	}
	return true
}
func (s *State) sgFireRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	e := q.Event
	p := &g.Players[i]
	pass := a.Choice == "pass"
	switch q.Kind {
	case "pindian":
		if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
			return true, err
		}
		if !s.sgOwn(e.Actor, e.Cards[0], true) {
			return true, errors.New("拼点发起者的牌已不可用")
		}
		e.Type = "pindian_result"
		e.Cards = append(e.Cards, a.Cards[0])
		s.sgPush(e)
		s.sgPay(e.Actor, e.Cards[:1])
		s.sgPay(i, a.Cards)
	case "quhu_target":
		if len(a.Targets) != 1 || !slices.Contains(q.Targets, a.Targets[0]) || !s.sgAlive(a.Targets[0]) {
			return true, errors.New("请选择驱虎范围内的角色")
		}
		s.sgPush(SGEvent{Type: "damage", Actor: e.Target, Target: a.Targets[0], Kind: "quhu", Amount: 1})
	case "jieming":
		if pass {
			return true, nil
		}
		if len(a.Targets) != 1 || !s.sgAlive(a.Targets[0]) {
			return true, errors.New("节命需要一名存活角色")
		}
		to := a.Targets[0]
		other := g.Players[to]
		s.sgDraw(to, max(0, min(5, other.MaxHP)-len(other.Hand)))
		s.sgLog("%s 发动节命 → %s", s.sgName(i), s.sgName(to))
		e.Amount--
		s.sgPush(e)
	case "mengjin":
		if pass {
			return true, nil
		}
		if a.Choice != "yes" {
			return true, errors.New("请选择发动或放弃")
		}
		e.Kind = "mengjin"
		s.sgAsk(i, "steal", "猛进：弃置目标一张手牌或装备（不能选择判定牌）", e)
	case "niepan":
		s.sgPush(e)
		if pass {
			return true, nil
		}
		if a.Choice != "yes" || p.Marks["niepan"] > 0 || !s.sgHas(i, "niepan") {
			return true, errors.New("涅槃只能发动一次")
		}
		if p.Marks == nil {
			p.Marks = map[string]int{}
		}
		p.Marks["niepan"] = 1
		ids := append(append([]int{}, p.Hand...), p.Equip...)
		for _, d := range p.Judgment {
			ids = append(ids, d.Card)
		}
		s.sgPush(SGEvent{Type: "niepan_rebirth", Actor: i})
		s.sgDiscard(i, ids)
	default:
		return false, nil
	}
	return true, nil
}
