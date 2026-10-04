package game

import (
	"errors"
	"slices"
)

// Eight original god generals, pinned to the same source as the elemental packs.
var sgGodGenerals = []SGGeneral{
	{"shenguanyu", "神关羽", "god", 5, false, []string{"wushen", "wuhun"}},
	{"shenlvmeng", "神吕蒙", "god", 3, false, []string{"shelie", "gongxin"}},
	{"shenzhouyu", "神周瑜", "god", 4, false, []string{"qinyin", "yeyan"}},
	{"shenzhugeliang", "神诸葛亮", "god", 3, false, []string{"qixing", "kuangfeng", "dawu"}},
	{"shencaocao", "神曹操", "god", 3, false, []string{"guixin", "feiying"}},
	{"shenlvbu", "神吕布", "god", 5, false, []string{"kuangbao", "wumou", "wuqian", "shenfen"}},
	{"shenzhaoyun", "神赵云", "god", 2, false, []string{"juejing", "longhun"}},
	{"shensimayi", "神司马懿", "god", 4, false, []string{"renjie", "baiyin", "lianpo"}},
}

func init() {
	for k, v := range map[string]SGSkill{
		"wushen":    {"武神", "锁定技：你的红桃手牌视为普通杀，不能再按原牌使用；使用红桃杀无距离限制，仍受出杀次数限制。"},
		"wuhun":     {"武魂", "锁定技：受到伤害扣血前，其他伤害来源获得等量梦魇。死亡且本局尚未结束时，选择存活梦魇最多且非零的一人判定；若不是桃或桃园结义，其直接死亡，不能求桃。之后清除梦魇。"},
		"shelie":    {"涉猎", "摸牌阶段可以改为亮出五张牌，每种花色获得一张，其余置入弃牌堆。"},
		"gongxin":   {"攻心", "出牌阶段限一次：观看另一角色的手牌，可选择其中一张红桃弃置或放到牌堆顶。只有你能查看其全手牌。"},
		"qinyin":    {"琴音", "弃牌阶段结束时，若此阶段弃置至少两张自己的牌，可令所有角色各回复1点体力或各失去1点体力。"},
		"yeyan":     {"业炎", "限定技：对一至三人各造成1点火伤；或弃四种花色手牌各一张、失去3点体力，再对一人造成3火伤，或对两人分别造成2、1火伤。整局限一次。"},
		"qixing":    {"七星", "起手额外七张，从十一张牌中选七张秘密置为星。摸牌阶段结束可以交换等量手牌与星；其他人仅见星的数量。"},
		"kuangfeng": {"狂风", "结束阶段，可以弃一张星选择一人；直到你的下次回合开始阶段或死亡，其受到的火焰伤害+1。"},
		"dawu":      {"大雾", "结束阶段，可以弃任意数量的星，选择等量不同角色；直到你的下次回合开始阶段或死亡，防止其受到的非雷电伤害。不能防止失去体力。"},
		"guixin":    {"归心", "每受到1点伤害后，可依次从每名其他角色手牌、装备或判定区获得一张牌，然后将自己翻面。多点伤害逐次选择。"},
		"feiying":   {"飞影", "锁定技：其他角色到你的距离+1。"},
		"kuangbao":  {"狂暴", "锁定技：开局获得2暴怒，每造成或受到1点伤害后获得1暴怒。"},
		"wumou":     {"无谋", "锁定技：使用非延时锦囊时，须弃1暴怒或失去1点体力。"},
		"wuqian":    {"无前", "出牌阶段可弃2暴怒，选择另一角色：本回合你获得无双，且该角色的防具失效。"},
		"shenfen":   {"神愤", "出牌阶段限一次，弃6暴怒：所有其他角色先各受1伤害，再各弃全部装备，再各弃四张手牌（不足全弃），最后你翻面。"},
		"juejing":   {"绝境", "锁定技：正常摸牌阶段额外摸已损失体力值张牌，手牌上限+2。采用经典两血版本。"},
		"longhun":   {"龙魂", "将与你当前体力相同数量（至少一张）的同花色手牌或装备转化：红桃桃、方片火杀、黑桃无懈可击、梅花闪。"},
		"renjie":    {"忍戒", "锁定技：每受到1点伤害后，或弃牌阶段每真正弃置一张自己的牌后，获得1忍。"},
		"baiyin":    {"拜印", "觉醒技：准备阶段忍至少4枚，减1体力上限并获得极略。"},
		"jilve":     {"极略", "每次弃1忍：判定时鬼才（手牌或装备换判）；受伤后放逐；使用任何锦囊时集智（亮顶牌，非基本直接获得，基本可用一张手牌置顶交换）；出牌阶段制衡或完杀各一次。完杀仅本回合。"},
		"lianpo":    {"连破", "任何人的回合结束时，若你于该回合击杀至少一人，可以获得一个额外回合。"},
	} {
		SGSkills[k] = v
	}
}

func (s *State) sgGodLonghun(i int, ids []int, desired string) (string, error) {
	if !s.sgHas(i, "longhun") || len(ids) != max(1, s.Sanguosha.Players[i].HP) {
		return "", errors.New("龙魂需要等于当前体力值的牌数（至少一张）")
	}
	suit := s.sgCardFor(i, ids[0]).Suit
	for _, id := range ids {
		if s.sgCardFor(i, id).Suit != suit {
			return "", errors.New("龙魂需要相同花色的牌")
		}
	}
	kind := []string{"nullification", "peach", "jink", "fire_slash"}[suit]
	if desired != "" && desired != kind && !(desired == "slash" && sgIsSlash(kind)) {
		return "", errors.New("龙魂花色不符合当前响应")
	}
	return kind, nil
}

func (s *State) sgGodWushenRange(i int, kind string, ids []int) bool {
	if !sgIsSlash(kind) || !s.sgHas(i, "wushen") || len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		if s.sgCardFor(i, id).Suit != 1 {
			return false
		}
	}
	return true
}

func (s *State) sgGodSkill(i int, a Action) (bool, error) {
	if handled, err := s.sgGodWarSkill(i, a); handled {
		return true, err
	}
	if handled, err := s.sgGodSimaSkill(i, a); handled {
		return true, err
	}
	if a.Skill != "gongxin" {
		return false, nil
	}
	g := s.Sanguosha
	if g.Players[i].Used["gongxin"] > 0 {
		return true, errors.New("本阶段已发动攻心")
	}
	if len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) || len(g.Players[a.Targets[0]].Hand) == 0 {
		return true, errors.New("攻心需要另一名有手牌的存活角色")
	}
	to := a.Targets[0]
	g.Players[i].Used["gongxin"]++
	s.sgLog("%s 对 %s 发动攻心，私下查看手牌", s.sgName(i), s.sgName(to))
	if len(g.Players[to].Hand) > 0 {
		s.sgAsk(i, "gongxin", "攻心：查看目标手牌，可选择红桃弃置或放回牌堆顶", SGEvent{Actor: i, Target: to})
		g.Pending.Cards = clone(g.Players[to].Hand)
	}
	return true, nil
}
func (s *State) sgGodShelie(i int) {
	ids := s.sgDrawIDs(5)
	s.sgPlaceTable(-1, ids)
	if len(ids) > 0 {
		s.sgAsk(i, "shelie", "涉猎：选择每种花色的一张牌，其余弃置", SGEvent{Actor: i, Cards: ids})
		s.Sanguosha.Pending.Cards = ids
	}
}
func (s *State) sgGodRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	e := q.Event
	if handled, err := s.sgGodStarsRespond(i, a, q); handled {
		return true, err
	}
	if handled, err := s.sgGodWarRespond(i, a, q); handled {
		return true, err
	}
	if handled, err := s.sgGodSimaRespond(i, a, q); handled {
		return true, err
	}
	switch q.Kind {
	case "wuhun_target":
		if len(a.Targets) != 1 || !slices.Contains(q.Targets, a.Targets[0]) {
			return true, errors.New("请选择梦魇最多的角色")
		}
		s.sgPush(SGEvent{Type: "judge", Actor: a.Targets[0], Target: i, Kind: "wuhun"})
	case "shelie":
		if !sgSubset(a.Cards, q.Cards) {
			return true, errors.New("请选择涉猎亮出的牌")
		}
		suits := map[int]bool{}
		chosen := map[int]bool{}
		for _, id := range q.Cards {
			suits[sgCard(id).Suit] = true
		}
		for _, id := range a.Cards {
			suit := sgCard(id).Suit
			if chosen[suit] {
				return true, errors.New("涉猎每种花色只能取一张")
			}
			chosen[suit] = true
		}
		if len(chosen) != len(suits) {
			return true, errors.New("涉猎必须选择每种花色的一张牌")
		}
		for _, id := range a.Cards {
			s.sgTakeTable(id)
		}
		s.sgGain(i, a.Cards)
		s.sgFinishCards(q.Cards)
		s.sgLog("%s 涉猎获得 %d 张不同花色的牌", s.sgName(i), len(a.Cards))
	case "gongxin":
		if a.Choice == "pass" {
			return true, nil
		}
		if !slices.Contains(q.Cards, a.Card) || !s.sgOwn(e.Target, a.Card, true) || s.sgCardFor(e.Target, a.Card).Suit != 1 {
			return true, errors.New("攻心只能选择目标的一张红桃手牌")
		}
		if a.Choice == "discard" {
			s.sgDiscard(e.Target, []int{a.Card})
		} else if a.Choice == "top" {
			s.sgLose(e.Target, []int{a.Card})
			g.Deck = append([]int{a.Card}, g.Deck...)
		} else {
			return true, errors.New("请选择弃置或放回牌堆顶")
		}
		s.sgLog("%s 攻心：将 %s 的红桃「%s」%s", s.sgName(i), s.sgName(e.Target), SGCardTypes[sgCard(a.Card).Kind].Name, map[string]string{"discard": "弃置", "top": "置于牌堆顶"}[a.Choice])
	default:
		return false, nil
	}
	return true, nil
}
