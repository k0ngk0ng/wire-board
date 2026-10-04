package game

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Classic Thicket rules, pinned to the same release as the Wind and Fire packs.
var sgThicketGenerals = []SGGeneral{
	{"caopi", "曹丕", "wei", 3, false, []string{"xingshang", "fangzhu", "songwei"}},
	{"xuhuang", "徐晃", "wei", 4, false, []string{"duanliang"}},
	{"menghuo", "孟获", "shu", 4, false, []string{"huoshou", "zaiqi"}},
	{"zhurong", "祝融", "shu", 4, true, []string{"juxiang", "lieren"}},
	{"sunjian", "孙坚", "wu", 4, false, []string{"yinghun"}},
	{"lusu", "鲁肃", "wu", 3, false, []string{"haoshi", "dimeng"}},
	{"dongzhuo", "董卓", "qun", 8, false, []string{"jiuchi", "roulin", "benghuai", "baonue"}},
	{"jiaxu", "贾诩", "qun", 3, false, []string{"wansha", "luanwu", "weimu"}},
}

func init() {
	for k, v := range map[string]SGSkill{
		"xingshang": {"行殇", "其他角色阵亡时，可以获得其所有手牌和装备牌，不含判定区的牌。"},
		"fangzhu":   {"放逐", "受到伤害后，可以令另一名角色摸你已损失体力值张牌，然后将其武将牌翻面。"},
		"songwei":   {"颂威", "主公技：其他魏势力角色的黑色判定牌生效后，可以令你摸一张牌。"},
		"duanliang": {"断粮", "可以将黑色基本牌或装备牌当兵粮寸断使用；你使用兵粮寸断的距离限制为2。"},
		"huoshou":   {"祸首", "锁定技：南蛮入侵对你无效；其他人使用南蛮入侵造成伤害时，你视为伤害来源。"},
		"zaiqi":     {"再起", "摸牌阶段受伤时，可以改为亮出已损失体力值张牌：每张红桃回复1点体力并弃置，获得其他牌。这不是判定。"},
		"juxiang":   {"巨象", "锁定技：南蛮入侵对你无效。其他角色的南蛮入侵结算结束后，获得仍在处理区的此牌。"},
		"lieren":    {"烈刃", "你的杀直接造成伤害后，可与受伤角色拼点；若赢，获得其一张手牌或装备牌。"},
		"yinghun":   {"英魂", "准备阶段受伤时，可以令另一角色摸X张弃1张，或摸1张弃X张，X为你已损失体力值。弃牌可含装备。"},
		"haoshi":    {"好施", "摸牌阶段可以多摸两张牌。摸牌后若手牌多于5张，须将一半手牌（向下取整）交给其他手牌最少的角色之一。"},
		"dimeng":    {"缔盟", "出牌阶段限一次：选择两名其他角色，弃置与其手牌数之差相等数量的牌，然后交换他们的手牌。"},
		"jiuchi":    {"酒池", "可以将一张黑桃手牌当酒使用。"},
		"roulin":    {"肉林", "锁定技：你对女性使用杀，或女性对你使用杀时，目标需连续使用两张闪才能抵消。与无双不叠加。"},
		"benghuai":  {"崩坏", "锁定技：结束阶段，若有其他角色体力值低于你，须选择失去1点体力或1点体力上限。"},
		"baonue":    {"暴虐", "主公技：其他群势力角色造成伤害后，可以进行判定；若为黑桃，你回复1点体力。"},
		"wansha":    {"完杀", "锁定技：你的回合内，只有你和濒死角色可使用桃救助该角色。"},
		"luanwu":    {"乱武", "限定技：出牌阶段，令其他角色依次对距离最近的一名角色使用杀，否则失去1点体力。整局限一次。"},
		"weimu":     {"帷幕", "锁定技：不能成为黑色锦囊的目标。经典蛊惑声明的牌按其专门规则处理。"},
	} {
		SGSkills[k] = v
	}
}
func (s *State) sgThicketStart(e SGEvent) {
	p := s.Sanguosha.Players[e.Actor]
	if s.sgHas(e.Actor, "yinghun") && p.HP < p.MaxHP {
		e.Amount = p.MaxHP - p.HP
		s.sgAsk(e.Actor, "yinghun", "英魂：选择另一角色与摸弃方式，或放弃", e)
	}
}

func (s *State) sgWeimu(actor, target int, kind string, ids []int) bool {
	if !s.sgHas(target, "weimu") || !sgIsTrick(kind) || s.sgCardColor(actor, ids) != 2 {
		return false
	}
	// Classic NosGuhuo declares a colorless trick for target prohibition.
	return s.Sanguosha.Virtual == nil || s.Sanguosha.Virtual.Player != actor
}

func (s *State) sgThicketDraw(i int, skill string) error {
	g := s.Sanguosha
	p := &g.Players[i]
	if skill == "haoshi" {
		s.sgDraw(i, 4)
		s.sgPush(SGEvent{Type: "haoshi_give", Actor: i})
		return nil
	}
	if p.HP >= p.MaxHP {
		return errors.New("没有损失体力，不能发动再起")
	}
	ids := s.sgDrawIDs(p.MaxHP - p.HP)
	s.sgPlaceTable(-1, ids)
	names := []string{}
	hearts := []int{}
	got := []int{}
	for _, id := range ids {
		c := sgCard(id)
		names = append(names, fmt.Sprintf("%s%d「%s」", []string{"♠", "♥", "♣", "♦"}[c.Suit], c.Rank, SGCardTypes[c.Kind].Name))
		if c.Suit == 1 {
			hearts = append(hearts, id)
		} else {
			s.sgTakeTable(id)
			got = append(got, id)
		}
	}
	s.sgLog("%s 再起亮出：%s", s.sgName(i), strings.Join(names, "、"))
	s.sgHeal(i, len(hearts))
	s.sgFinishCards(hearts)
	s.sgGain(i, got)
	return nil
}
func (s *State) sgThicketDamageDealt(e SGEvent) {
	es := []SGEvent{}
	if sgIsSlash(e.Kind) {
		next := e
		next.Type = "liyu"
		es = append(es, next)
	}
	if s.sgHas(e.Actor, "lieren") && sgIsSlash(e.Kind) && !e.Transfer && !e.Chain && s.sgAlive(e.Target) && len(s.Sanguosha.Players[e.Actor].Hand) > 0 && len(s.Sanguosha.Players[e.Target].Hand) > 0 {
		next := e
		next.Type = "lieren"
		es = append(es, next)
	}
	g := s.Sanguosha
	if s.sgAlive(e.Actor) && e.Actor != g.Lord && s.sgKingdom(e.Actor) == "qun" && s.sgHas(g.Lord, "baonue") {
		es = append(es, SGEvent{Type: "baonue", Actor: e.Actor, Target: g.Lord})
	}
	s.sgPush(es...)
}
func (s *State) sgThicketCleanup(e SGEvent) {
	g := s.Sanguosha
	if e.Kind != "savage_assault" || len(e.Cards) != 1 || !slices.Contains(g.Table, e.Cards[0]) {
		return
	}
	for _, who := range s.sgOrder(s.Turn) {
		if who != e.Actor && s.sgHas(who, "juxiang") {
			s.sgTakeTable(e.Cards[0])
			s.sgGain(who, e.Cards[:1])
			s.sgLog("%s 发动巨象，获得结算后的南蛮入侵", s.sgName(who))
			return
		}
	}
}
func (s *State) sgThicketSkill(i int, a Action) (bool, error) {
	g := s.Sanguosha
	p := &g.Players[i]
	switch a.Skill {
	case "dimeng":
		if p.Used[a.Skill] > 0 {
			return true, errors.New("本阶段已发动缔盟")
		}
		if len(a.Targets) != 2 || a.Targets[0] == a.Targets[1] {
			return true, errors.New("缔盟需要两名不同的其他角色")
		}
		for _, t := range a.Targets {
			if t == i || !s.sgAlive(t) {
				return true, errors.New("缔盟需要两名存活的其他角色")
			}
		}
		pa, pb := g.Players[a.Targets[0]], g.Players[a.Targets[1]]
		due := len(pa.Hand) - len(pb.Hand)
		if due < 0 {
			due = -due
		}
		if err := s.sgValidateCards(i, a.Cards, due, false); err != nil {
			return true, err
		}
		p.Used[a.Skill]++
		s.sgPush(SGEvent{Type: "dimeng_swap", Actor: i, Targets: clone(a.Targets)})
		s.sgDiscard(i, a.Cards)
	case "luanwu":
		if p.Marks["luanwu"] > 0 || len(a.Cards)+len(a.Targets) != 0 {
			return true, errors.New("乱武整局限一次，无需选牌或目标")
		}
		if p.Marks == nil {
			p.Marks = map[string]int{}
		}
		p.Marks["luanwu"] = 1
		events := []SGEvent{}
		for _, who := range s.sgOrder(s.sgNext(i)) {
			if who != i {
				events = append(events, SGEvent{Type: "luanwu", Actor: i, Target: who})
			}
		}
		s.sgPush(events...)
		s.sgLog("%s 发动乱武，其他角色依次响应", s.sgName(i))
	default:
		return false, nil
	}
	return true, nil
}
func (s *State) sgThicketEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "death_loot":
		dead := g.Players[e.Target]
		for e.Step < len(g.Players) && len(dead.Hand)+len(dead.Equip) > 0 {
			who := (s.Turn + e.Step) % len(g.Players)
			e.Step++
			if who != e.Target && s.sgHas(who, "xingshang") {
				s.sgAsk(who, "xingshang", "行殇：是否获得阵亡角色的所有手牌和装备？", e)
				return true
			}
		}
		s.sgDeathClear(e.Target)
		s.sgDeathReward(e.Target, e.Actor)
	case "thicket_start":
		s.sgThicketStart(e)
	case "fangzhu":
		if s.sgHas(e.Target, "fangzhu") {
			s.sgAsk(e.Target, "fangzhu", "放逐：选择另一角色摸牌并翻面，或放弃", e)
		}
	case "songwei":
		if s.sgAlive(e.Actor) && s.sgHas(e.Target, "songwei") {
			s.sgAsk(e.Actor, "songwei", "是否发动颂威，让主公摸一张牌？", e)
		}
	case "baonue":
		if s.sgAlive(e.Actor) && s.sgHas(e.Target, "baonue") {
			s.sgAsk(e.Actor, "baonue", "是否发动暴虐，判定黑桃让主公回复1点体力？", e)
		}
	case "lieren":
		if s.sgHas(e.Actor, "lieren") && s.sgAlive(e.Target) && len(g.Players[e.Actor].Hand) > 0 && len(g.Players[e.Target].Hand) > 0 {
			s.sgAsk(e.Actor, "lieren", "烈刃：选一张手牌与伤害目标拼点，或放弃", e)
		}
	case "haoshi_give":
		if !s.sgAlive(e.Actor) || len(g.Players[e.Actor].Hand) <= 5 {
			return true
		}
		least := 10000
		targets := []int{}
		for _, who := range s.sgOrder(s.Turn) {
			if who == e.Actor {
				continue
			}
			n := len(g.Players[who].Hand)
			if n < least {
				least = n
				targets = nil
			}
			if n == least {
				targets = append(targets, who)
			}
		}
		if len(targets) > 0 {
			e.Amount = len(g.Players[e.Actor].Hand) / 2
			s.sgAsk(e.Actor, "haoshi_give", "好施：将一半手牌交给其他手牌最少的角色之一", e)
			g.Pending.Targets = targets
		}
	case "yinghun_discard":
		if !s.sgAlive(e.Target) {
			return true
		}
		p := g.Players[e.Target]
		e.Amount = min(e.Amount, len(p.Hand)+len(p.Equip))
		if e.Amount > 0 {
			s.sgAsk(e.Target, "yinghun_discard", "英魂：选择需要弃置的手牌或装备", e)
		}
	case "dimeng_swap":
		a, b := e.Targets[0], e.Targets[1]
		if !s.sgAlive(a) || !s.sgAlive(b) {
			return true
		}
		ah, bh := clone(g.Players[a].Hand), clone(g.Players[b].Hand)
		// Install both destinations before evaluating any empty-hand trigger.
		g.Players[a].Hand = bh
		g.Players[b].Hand = ah
		for _, who := range s.sgOrder(s.Turn) {
			before, after := 0, 0
			if who == a {
				before, after = len(ah), len(bh)
			} else if who == b {
				before, after = len(bh), len(ah)
			}
			s.sgTuntianLoss(who, before > 0)
			if before > 0 && after == 0 {
				s.sgEmptyHand(who, before)
			}
			if who == a || who == b {
				s.sgHandGained(who, g.Players[who].Hand)
			}
		}
		s.sgLog("%s 发动缔盟，%s 与 %s 交换手牌（%d ↔ %d张）", s.sgName(e.Actor), s.sgName(a), s.sgName(b), len(ah), len(bh))
	case "benghuai":
		if !s.sgHas(e.Actor, "benghuai") {
			return true
		}
		for _, who := range s.sgOrder(s.Turn) {
			if who != e.Actor && g.Players[who].HP < g.Players[e.Actor].HP {
				s.sgAsk(e.Actor, "benghuai", "崩坏：选择失去1点体力或1点体力上限", e)
				g.Pending.Choices = []string{"hp", "maxhp"}
				break
			}
		}
	case "luanwu":
		if !s.sgAlive(e.Target) {
			return true
		}
		nearest := 10000
		targets := []int{}
		for _, who := range s.sgOrder(s.Turn) {
			if who == e.Target {
				continue
			}
			d := s.sgDistance(e.Target, who)
			if d < nearest {
				nearest = d
				targets = nil
			}
			if d == nearest {
				targets = append(targets, who)
			}
		}
		e.Kind = "luanwu"
		s.sgAsk(e.Target, "luanwu", "乱武：对距离最近的角色使用杀，或放弃并失去1点体力", e)
		g.Pending.Targets = targets
	default:
		return false
	}
	return true
}
func (s *State) sgThicketRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	e := q.Event
	p := &g.Players[i]
	pass := a.Choice == "pass"
	switch q.Kind {
	case "xingshang":
		if !pass {
			if a.Choice != "yes" {
				return true, errors.New("请选择获得或放弃")
			}
			dead := &g.Players[e.Target]
			s.sgGain(i, append(clone(dead.Hand), dead.Equip...))
			dead.Hand = []int{}
			dead.Equip = []int{}
			s.sgLog("%s 发动行殇，获得 %s 的手牌与装备", s.sgName(i), s.sgName(e.Target))
		}
		s.sgPush(e)
	case "fangzhu":
		if pass {
			return true, nil
		}
		if len(a.Targets) != 1 || !s.sgAlive(a.Targets[0]) || a.Targets[0] == i {
			return true, errors.New("放逐需要另一名存活角色")
		}
		to := a.Targets[0]
		s.sgDraw(to, max(0, p.MaxHP-p.HP))
		g.Players[to].Flipped = !g.Players[to].Flipped
		s.sgLog("%s 发动放逐，%s 摸牌并翻面", s.sgName(i), s.sgName(to))
	case "songwei", "baonue":
		if pass {
			return true, nil
		}
		if a.Choice != "yes" {
			return true, errors.New("请选择发动或放弃")
		}
		if q.Kind == "songwei" {
			s.sgDraw(e.Target, 1)
		} else {
			s.sgPush(SGEvent{Type: "judge", Actor: i, Target: e.Target, Kind: "baonue"})
		}
	case "lieren":
		if pass {
			return true, nil
		}
		if err := s.sgValidateCards(i, a.Cards, 1, true); err != nil {
			return true, err
		}
		s.sgAsk(e.Target, "pindian", s.sgName(i)+" 发动烈刃：选择一张手牌拼点", SGEvent{Type: "pindian_result", Actor: i, Target: e.Target, Kind: "lieren", Cards: clone(a.Cards)})
	case "yinghun":
		if pass {
			return true, nil
		}
		if len(a.Targets) != 1 || !s.sgAlive(a.Targets[0]) || a.Targets[0] == i {
			return true, errors.New("英魂需要另一名存活角色")
		}
		draw, discard := e.Amount, 1
		if a.Choice == "draw_one" {
			draw, discard = 1, e.Amount
		} else if a.Choice != "draw_many" {
			return true, errors.New("请选择英魂的摸弃方式")
		}
		s.sgDraw(a.Targets[0], draw)
		s.sgPush(SGEvent{Type: "yinghun_discard", Actor: i, Target: a.Targets[0], Amount: discard})
	case "yinghun_discard":
		if err := s.sgValidateCards(i, a.Cards, e.Amount, false); err != nil {
			return true, err
		}
		s.sgDiscard(i, a.Cards)
	case "haoshi_give":
		if err := s.sgValidateCards(i, a.Cards, e.Amount, true); err != nil {
			return true, err
		}
		if len(a.Targets) != 1 || !slices.Contains(q.Targets, a.Targets[0]) {
			return true, errors.New("好施必须交给其他手牌最少的角色")
		}
		s.sgLose(i, a.Cards)
		s.sgGain(a.Targets[0], a.Cards)
		s.sgLog("%s 发动好施，交给 %s %d 张手牌", s.sgName(i), s.sgName(a.Targets[0]), len(a.Cards))
	case "benghuai":
		if a.Choice == "hp" {
			s.sgPush(SGEvent{Type: "lose_hp", Target: i, Amount: 1})
		} else if a.Choice == "maxhp" {
			p.MaxHP--
			p.HP = min(p.HP, p.MaxHP)
			s.sgLog("%s 因崩坏失去1点体力上限", s.sgName(i))
			if p.MaxHP <= 0 {
				s.sgDie(i, -1)
			}
		} else {
			return true, errors.New("崩坏必须选择体力或上限")
		}
	case "luanwu":
		if pass {
			s.sgPush(SGEvent{Type: "lose_hp", Target: i, Amount: 1})
			return true, nil
		}
		if len(a.Targets) != 1 || !slices.Contains(q.Targets, a.Targets[0]) {
			return true, errors.New("乱武只能选择距离最近的角色")
		}
		if a.Choice == "jijiang" {
			if !s.sgHas(i, "jijiang") || e.Step&2 != 0 || !s.sgCanTarget(i, a.Targets[0], "slash") || s.sgTianyi(i) == -1 {
				return true, errors.New("此时不能发动激将")
			}
			e.Aux = a.Targets[0]
			s.sgPush(SGEvent{Type: "support", Actor: i, Kind: "jijiang", Next: &e})
			return true, nil
		}
		kind, err := s.sgAs(i, a.Cards, a.Skill, "slash")
		if err != nil {
			return true, err
		}
		return true, s.sgUse(i, kind, a.Cards, a.Targets, true)
	default:
		return false, nil
	}
	return true, nil
}
