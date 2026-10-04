package game

import (
	"errors"
	"slices"
)

func init() {
	for id, info := range map[string]SGSkill{
		"heg_rende":     {"仁德·国", "出牌阶段可将手牌交给其他角色；本阶段首次累计交出至少三张牌时，回复一点体力。"},
		"heg_jizhi":     {"集智·国", "使用未经过技能转化的非延时锦囊牌时，可以摸一张牌。"},
		"heg_zhiheng":   {"制衡·国", "出牌阶段限一次：弃置一至体力上限张手牌或装备，再摸等量的牌。"},
		"heg_lijian":    {"离间·国", "出牌阶段限一次：弃一张牌，令另一名男性对你指定的第三名男性使用虚拟决斗；此决斗可被无懈可击抵消。"},
		"heg_duanchang": {"断肠·国", "锁定技：阵亡时，选择杀死你的角色的一张武将牌，令其失去该武将牌的技能。"},
		"heg_xiaoguo":   {"骁果", "其他角色的结束阶段开始时，可弃一张基本手牌；其须弃一张装备牌，否则受到你造成的一点伤害。"},
		"heg_shushen":   {"淑慎", "每回复一点体力后，可让另一名已明置的同势力角色摸一张牌。"},
		"heg_shenzhi":   {"神智", "准备阶段可弃全部手牌；若弃牌数量不少于当前体力，回复一点体力。"},
		"heg_duoshi":    {"度势", "出牌阶段至多四次：将一张红色手牌当以逸待劳使用。"},
		"heg_duanbing":  {"短兵", "使用杀时，可多选择一名与你距离为一的角色。"},
		"heg_fenxun":    {"奋迅", "出牌阶段限一次：弃一张手牌或装备，令你到另一名角色的距离在本回合内视为一。"},
		"heg_xiongyi":   {"雄异", "限定技：出牌阶段令自己和已明置的同势力角色各摸三张牌；若本势力存活人数为最少（可并列），自己回复一点体力。"},
		"heg_mingshi":   {"名士", "锁定技：受到伤害时，若来源仍有暗置武将，将本次伤害减一。"},
		"heg_lirang":    {"礼让", "自己被弃置的牌进入弃牌堆后，可将其中的牌交给其他角色。"},
		"heg_shuangren": {"双刃", "出牌阶段开始时可与另一角色拼点；赢则对其或其同势力角色使用无距离限制的虚拟杀，未赢则跳过出牌阶段。"},
		"heg_sijian":    {"死谏", "失去最后的手牌后，可弃另一名角色的一张手牌、装备或判定牌。"},
		"heg_suishi":    {"随势", "锁定技：其他角色因同势力角色造成的伤害进入濒死时，摸一张牌；其他同势力角色阵亡时，失去一点体力。"},
		"heg_kuangfu":   {"狂斧", "使用杀对目标造成伤害后，可弃其一张装备，或将其移入自己空着的对应装备栏。"},
		"heg_huoshui":   {"祸水", "出牌阶段可明置本武将；你的回合内，其他角色不能明置武将牌。"},
		"heg_qingcheng": {"倾城", "出牌阶段弃一张装备牌，将另一名双将都已明置的角色的一张武将牌暗置。"},
	} {
		SGSkills[id] = info
	}
	for id, info := range map[string]SGCardInfo{
		"heg_nullification":  {"无懈可击·国", "响应锦囊时可抵消当前目标，或同时抵消本牌对该目标同势力后续目标的效果；也可抵消其他无懈可击。", "", 0},
		"await_exhausted":    {"以逸待劳", "自己与已明置的同势力角色各摸两张牌，再各弃两张手牌或装备。", "", 0},
		"known_both":         {"知己知彼", "私下查看另一名角色的手牌或一张暗置的武将牌；也可不选目标重铸，弃此牌并摸一张。", "", 0},
		"befriend_attacking": {"远交近攻", "自己已明置时，选择一名已明置、与你势力不同的角色；其摸一张牌，你摸三张牌。", "", 0},
		"six_swords":         {"吴六剑", "其他已明置的同势力角色攻击范围增加一。", "weapon", 2},
		"triblade":           {"三尖两刃刀", "使用杀造成伤害后，可弃一张手牌，对与受伤角色距离为一的另一角色造成一点伤害。", "weapon", 3},
	} {
		SGCardTypes[id] = info
	}
}

func (s *State) sgHegSkill(i int, a Action) (bool, error) {
	if !s.sgHegemony() {
		return false, nil
	}
	g := s.Sanguosha
	p := &g.Players[i]
	switch a.Skill {
	case "heg_rende":
		if len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) || len(a.Cards) == 0 {
			return true, errors.New("仁德需要至少一张手牌和另一名存活角色")
		}
		if err := s.sgValidateCards(i, a.Cards, -1, true); err != nil {
			return true, err
		}
		before := p.Used[a.Skill]
		s.sgLose(i, a.Cards)
		s.sgGain(a.Targets[0], a.Cards)
		p.Used[a.Skill] += len(a.Cards)
		if before < 3 && p.Used[a.Skill] >= 3 {
			s.sgHeal(i, 1)
		}
		s.sgLog("%s 发动仁德，交给 %s %d 张手牌", s.sgName(i), s.sgName(a.Targets[0]), len(a.Cards))
	case "heg_zhiheng":
		if p.Used[a.Skill] > 0 || len(a.Cards) < 1 || len(a.Cards) > p.MaxHP {
			return true, errors.New("制衡每阶段一次，弃牌数量不得超过体力上限")
		}
		if err := s.sgValidateCards(i, a.Cards, -1, false); err != nil {
			return true, err
		}
		p.Used[a.Skill]++
		s.sgPush(SGEvent{Type: "draw", Actor: i, Amount: len(a.Cards)})
		s.sgDiscard(i, a.Cards)
	case "heg_lijian":
		if p.Used[a.Skill] > 0 || len(a.Targets) != 2 || a.Targets[0] == a.Targets[1] {
			return true, errors.New("离间每阶段一次，需要两名其他男性角色")
		}
		for _, target := range a.Targets {
			if target == i || !s.sgAlive(target) || !s.sgMale(target) {
				return true, errors.New("离间需要两名性别已确定的其他男性角色")
			}
		}
		if !s.sgCanTarget(a.Targets[1], a.Targets[0], "duel") {
			return true, errors.New("该角色不能成为决斗目标")
		}
		if err := s.sgValidateCards(i, a.Cards, 1, false); err != nil {
			return true, err
		}
		p.Used[a.Skill]++
		g.TrickSequence++
		s.sgPush(SGEvent{Type: "null_window", Actor: a.Targets[1], Target: a.Targets[0], Kind: "duel", Amount: 1, Aux: -1, Step: a.Targets[1], TrickID: g.TrickSequence, OriginalTargets: []int{a.Targets[0]}})
		s.sgDiscard(i, a.Cards)
	case "heg_fenxun":
		if p.Used[a.Skill] > 0 || len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) {
			return true, errors.New("奋迅每阶段一次，需要另一名存活角色")
		}
		if err := s.sgValidateCards(i, a.Cards, 1, false); err != nil {
			return true, err
		}
		p.Used[a.Skill] = a.Targets[0] + 1
		s.sgDiscard(i, a.Cards)
		s.sgLog("%s 对 %s 发动奋迅，本回合距离视为一", s.sgName(i), s.sgName(a.Targets[0]))
	case "heg_xiongyi":
		if p.Marks["heg_xiongyi"] > 0 {
			return true, errors.New("雄异已使用")
		}
		s.sgMark(i, "heg_xiongyi")
		es := []SGEvent{}
		for _, who := range s.sgOrder(s.Turn) {
			if s.sgHegFriend(i, who) {
				es = append(es, SGEvent{Type: "draw", Actor: who, Amount: 3})
			}
		}
		es = append(es, SGEvent{Type: "heg_xiongyi_heal", Actor: i})
		s.sgPush(es...)
	case "heg_huoshui":
		// Revealing is handled before skill application. Already face-up Huoshui
		// is continuous and doesn't need another action.
		return true, errors.New("祸水在武将明置后自动生效")
	default:
		return false, nil
	}
	return true, nil
}

func (s *State) sgMale(i int) bool {
	return i >= 0 && i < len(s.Sanguosha.Players) && (!s.sgHegemony() || s.sgHegShown(i)) && !s.sgFemale(i)
}

func (s *State) sgHegFactionSize(i int) int {
	n := 0
	for _, who := range s.sgOrder(0) {
		if s.sgHegFriend(i, who) {
			n++
		}
	}
	return n
}

func (s *State) sgHegSmallestFaction(i int) bool {
	n := s.sgHegFactionSize(i)
	for _, who := range s.sgOrder(0) {
		if s.sgHegShown(who) && s.sgHegFactionSize(who) < n {
			return false
		}
	}
	return true
}

func (s *State) sgHegSkillSlot(i int, skill string) int {
	p := s.Sanguosha.Players[i]
	for slot, id := range s.sgHegGeneralIDs(i) {
		if !p.Hegemony.Lost[slot] && slices.Contains(sgGeneral(id).Skills, skill) {
			return slot
		}
	}
	return -1
}
