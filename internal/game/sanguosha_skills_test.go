package game

import (
	"slices"
	"testing"
)

func sgSetTop(t *testing.T, s *State, suit, rank int) int {
	t.Helper()
	g := s.Sanguosha
	for _, id := range g.Deck {
		c := sgCard(id)
		if c.Suit == suit && (rank == 0 || c.Rank == rank) {
			g.Deck = append([]int{id}, sgRemove(g.Deck, id)...)
			return id
		}
	}
	t.Fatal("no judgment card")
	return 0
}
func TestSanguoshaTransformations(t *testing.T) {
	for _, tc := range []struct {
		general, skill, want string
		suit                 int
		hand                 bool
	}{{"guanyu", "wusheng", "slash", 1, false}, {"zhenji", "qingguo", "jink", 0, true}, {"ganning", "qixi", "dismantlement", 2, false}, {"daqiao", "guose", "indulgence", 3, false}, {"huatuo", "jijiu", "peach", 1, false}} {
		s := sgTestState()
		p := &s.Sanguosha.Players[1]
		p.General = tc.general
		id := sgSetTop(t, s, tc.suit, 0)
		s.Sanguosha.Deck = s.Sanguosha.Deck[1:]
		p.Hand = append(p.Hand, id)
		kind, err := s.sgAs(1, []int{id}, tc.skill, tc.want)
		if err != nil || kind != tc.want {
			t.Fatal(tc, err)
		}
		if _, err = s.sgAs(2, []int{id}, tc.skill, tc.want); err == nil {
			t.Fatal("used other player's card")
		}
	}
}
func TestSanguoshaActiveSkills(t *testing.T) {
	t.Run("rende", func(t *testing.T) {
		s := sgTestState()
		p := &s.Sanguosha.Players[0]
		p.General = "liubei"
		p.HP = 2
		ids := []int{sgGive(t, s, 0, "slash"), sgGive(t, s, 0, "jink")}
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "rende", Cards: ids, Targets: []int{1}})
		if s.Sanguosha.Players[0].HP != 3 || len(s.Sanguosha.Players[1].Hand) != 2 {
			t.Fatal("rende")
		}
		sgConservation(t, s)
	})
	t.Run("zhiheng", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "sunquan"
		id := sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "zhiheng", Cards: []int{id}})
		if len(s.Sanguosha.Players[0].Hand) != 1 || slices.Contains(s.Sanguosha.Players[0].Hand, id) {
			t.Fatal("zhiheng")
		}
		if s.Apply(0, Action{Type: "sg_skill", Skill: "zhiheng", Cards: s.Sanguosha.Players[0].Hand}) == nil {
			t.Fatal("repeated once per phase skill")
		}
		sgConservation(t, s)
	})
	t.Run("kurou", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "huanggai"
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "kurou"})
		if s.Sanguosha.Players[0].HP != 3 || len(s.Sanguosha.Players[0].Hand) != 2 {
			t.Fatal("kurou")
		}
		sgConservation(t, s)
	})
	for _, general := range []string{"huatuo", "sunshangxiang"} {
		t.Run(general, func(t *testing.T) {
			s := sgTestState()
			s.Sanguosha.Players[0].General = general
			s.Sanguosha.Players[0].HP = 2
			s.Sanguosha.Players[1].HP = 2
			ids := []int{sgGive(t, s, 0, "slash")}
			skill := "qingnang"
			if general == "sunshangxiang" {
				skill = "jieyin"
				ids = append(ids, sgGive(t, s, 0, "jink"))
			}
			sgDo(t, s, 0, Action{Type: "sg_skill", Skill: skill, Cards: ids, Targets: []int{1}})
			if s.Sanguosha.Players[1].HP != 3 {
				t.Fatal("healing")
			}
			sgConservation(t, s)
		})
	}
	t.Run("lijian", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "diaochan"
		id := sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "lijian", Cards: []int{id}, Targets: []int{1, 2}})
		if q := s.Sanguosha.Pending; q.Kind != "card" || q.Player != 1 || q.Event.Actor != 2 {
			t.Fatal("lijian must directly duel", q)
		}
		sgPassAll(t, s)
		sgConservation(t, s)
	})
}
func TestSanguoshaJudgmentGuicaiTianduAndArmor(t *testing.T) {
	s := sgTestState()
	s.Sanguosha.Players[1].General = "guojia"
	s.Sanguosha.Players[2].General = "simayi"
	armor := sgGive(t, s, 1, "eight_diagram")
	s.Sanguosha.Players[1].Hand = sgRemove(s.Sanguosha.Players[1].Hand, armor)
	s.Sanguosha.Players[1].Equip = []int{armor}
	slash := sgGive(t, s, 0, "slash")
	red := sgSetTop(t, s, 1, 4)
	s.Sanguosha.Deck = s.Sanguosha.Deck[1:]
	s.Sanguosha.Players[2].Hand = []int{red}
	sgSetTop(t, s, 0, 7)
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{1}})
	sgDo(t, s, 1, Action{Choice: "eight_diagram"})
	if s.Sanguosha.Pending.Kind != "guicai" {
		t.Fatal("guicai missing")
	}
	sgDo(t, s, 2, Action{Cards: []int{red}})
	if s.Sanguosha.Pending.Kind != "tiandu" {
		t.Fatal("tiandu missing")
	}
	sgDo(t, s, 1, Action{Choice: "yes"})
	if s.Sanguosha.Players[1].HP != 4 || !slices.Contains(s.Sanguosha.Players[1].Hand, red) {
		t.Fatal("armor/red replacement/tiandu")
	}
	sgConservation(t, s)
}
func TestSanguoshaSupportUsesAllyCard(t *testing.T) {
	s := sgTestState()
	s.Sanguosha.Players[0].General = "liubei"
	s.Sanguosha.Players[1].General = "guanyu"
	id := sgGive(t, s, 1, "slash")
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "jijiang", Targets: []int{1}})
	if s.Sanguosha.Pending.Kind != "support" {
		t.Fatal("jijiang missing")
	}
	sgDo(t, s, 1, Action{Cards: []int{id}})
	sgPassAll(t, s)
	if slices.Contains(s.Sanguosha.Players[1].Hand, id) {
		t.Fatal("ally card not spent")
	}
	sgConservation(t, s)
}
func TestSanguoshaLianyingBeforeNextDuelResponse(t *testing.T) {
	s := sgTestState()
	s.Sanguosha.Players[1].General = "luxun"
	duel := sgGive(t, s, 0, "duel")
	slash := sgGive(t, s, 1, "slash")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{duel}, Targets: []int{1}})
	for s.Sanguosha.Pending.Kind == "nullification" {
		sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
	}
	sgDo(t, s, 1, Action{Cards: []int{slash}})
	if s.Sanguosha.Pending.Kind != "invoke" || s.Sanguosha.Pending.Player != 1 {
		t.Fatal("lianying timing", s.Sanguosha.Pending)
	}
	sgDo(t, s, 1, Action{Choice: "yes"})
	if len(s.Sanguosha.Players[1].Hand) != 1 {
		t.Fatal("lianying draw")
	}
	sgPassAll(t, s)
	sgConservation(t, s)
}
func TestSanguoshaConcurrentNullWindow(t *testing.T) {
	s := sgTestState()
	id := sgGive(t, s, 0, "ex_nihilo")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}})
	q := s.Sanguosha.Pending.ID
	sgDo(t, s, 3, Action{Choice: "pass"})
	if s.Sanguosha.Pending.ID != q {
		t.Fatal("passing must not invalidate other responses")
	}
	if s.Apply(3, Action{Prompt: q, Choice: "pass"}) == nil {
		t.Fatal("duplicate pass accepted")
	}
	sgDo(t, s, 2, Action{Choice: "pass"})
	sgDo(t, s, 0, Action{Choice: "pass"})
	sgDo(t, s, 1, Action{Choice: "pass"})
	if len(s.Sanguosha.Players[0].Hand) != 2 {
		t.Fatal("all pass must resolve effect")
	}
	sgConservation(t, s)
}
func TestSanguoshaRemainingClassicSkills(t *testing.T) {
	t.Run("jianxiong", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[1].General = "caocao"
		id := sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Choice: "pass"})
		sgDo(t, s, 1, Action{Choice: "yes"})
		if !slices.Contains(s.Sanguosha.Players[1].Hand, id) {
			t.Fatal("jianxiong")
		}
		sgConservation(t, s)
	})
	t.Run("fankui", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[1].General = "simayi"
		id := sgGive(t, s, 0, "slash")
		other := sgGive(t, s, 0, "jink")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Choice: "pass"})
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgDo(t, s, 1, Action{Choice: "hand"})
		if !slices.Contains(s.Sanguosha.Players[1].Hand, other) {
			t.Fatal("fankui")
		}
		sgConservation(t, s)
	})
	t.Run("ganglie", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[1].General = "xiahoudun"
		id := sgGive(t, s, 0, "slash")
		sgSetTop(t, s, 2, 0)
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Choice: "pass"})
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgDo(t, s, 0, Action{Choice: "pass"})
		if s.Sanguosha.Players[0].HP != 3 {
			t.Fatal("ganglie")
		}
		sgConservation(t, s)
	})
	t.Run("tuxi", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "zhangliao"
		a := sgGive(t, s, 1, "jink")
		b := sgGive(t, s, 2, "peach")
		s.sgPush(SGEvent{Type: "draw_phase", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Choice: "tuxi", Targets: []int{1, 2}})
		if !slices.Contains(s.Sanguosha.Players[0].Hand, a) || !slices.Contains(s.Sanguosha.Players[0].Hand, b) {
			t.Fatal("tuxi")
		}
		sgConservation(t, s)
	})
	t.Run("luoyi", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "xuchu"
		s.sgPush(SGEvent{Type: "draw_phase", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Choice: "luoyi"})
		id := sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 2 {
			t.Fatal("luoyi")
		}
		sgConservation(t, s)
	})
	t.Run("yiji", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[1].General = "guojia"
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 1, Kind: "slash", Amount: 1})
		s.sgRun()
		sgDo(t, s, 1, Action{Choice: "yes"})
		ids := clone(s.Sanguosha.Pending.Cards)
		sgDo(t, s, 1, Action{Cards: ids[:1], Targets: []int{2}})
		sgDo(t, s, 1, Action{Cards: ids[1:], Targets: []int{1}})
		sgConservation(t, s)
	})
	t.Run("luoshen", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "zhenji"
		id := sgSetTop(t, s, 2, 0)
		s.sgPush(SGEvent{Type: "luoshen", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Choice: "yes"})
		if !slices.Contains(s.Sanguosha.Players[0].Hand, id) || s.Sanguosha.Pending.Kind != "invoke" {
			t.Fatal("luoshen")
		}
		sgDo(t, s, 0, Action{Choice: "pass"})
		sgConservation(t, s)
	})
	t.Run("guanxing_kongcheng", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "zhugeliang"
		if s.sgCanTarget(1, 0, "slash") || s.sgCanTarget(1, 0, "duel") {
			t.Fatal("kongcheng")
		}
		s.sgPush(SGEvent{Type: "guanxing", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Choice: "yes"})
		ids := clone(s.Sanguosha.Pending.Cards)
		sgDo(t, s, 0, Action{Cards: []int{ids[2], ids[0]}, Take: []int{ids[3], ids[1]}})
		if s.Sanguosha.Deck[0] != ids[2] {
			t.Fatal("guanxing order")
		}
		sgConservation(t, s)
	})
	t.Run("longdan", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "zhaoyun"
		id := sgGive(t, s, 0, "jink")
		sgDo(t, s, 0, Action{Type: "sg_play", Skill: "longdan", Cards: []int{id}, Targets: []int{1}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("longdan")
		}
		sgConservation(t, s)
	})
	t.Run("tieji_mashu", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "machao"
		if s.sgDistance(0, 2) != 1 {
			t.Fatal("mashu")
		}
		id := sgGive(t, s, 0, "slash")
		sgSetTop(t, s, 1, 0)
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{2}})
		sgDo(t, s, 0, Action{Choice: "yes"})
		if s.Sanguosha.Players[2].HP != 3 || s.Sanguosha.Pending != nil {
			t.Fatal("red tieji must bypass jink")
		}
		sgConservation(t, s)
	})
	t.Run("jizhi_qicai", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "huangyueying"
		id := sgGive(t, s, 0, "snatch")
		sgGive(t, s, 2, "jink")
		if !s.sgCanTarget(0, 2, "snatch") {
			t.Fatal("qicai")
		}
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{2}})
		if s.Sanguosha.Pending.Kind != "invoke" {
			t.Fatal("jizhi")
		}
		sgDo(t, s, 0, Action{Choice: "yes"})
		for s.Sanguosha.Pending.Kind == "nullification" {
			sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
		}
		sgDo(t, s, 0, Action{Choice: "hand"})
		if len(s.Sanguosha.Players[0].Hand) != 2 {
			t.Fatal("jizhi draw")
		}
		sgConservation(t, s)
	})
	t.Run("jiuyuan", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "sunquan"
		s.Sanguosha.Players[1].General = "zhouyu"
		id := sgGive(t, s, 1, "peach")
		s.Sanguosha.Players[0].HP = 0
		s.sgPush(SGEvent{Type: "dying", Actor: 2, Target: 0, Step: 1})
		s.sgRun()
		sgDo(t, s, 1, Action{Cards: []int{id}})
		if s.Sanguosha.Players[0].HP != 2 {
			t.Fatal("jiuyuan")
		}
		sgConservation(t, s)
	})
	t.Run("keji", func(t *testing.T) {
		s := sgTestState()
		p := &s.Sanguosha.Players[0]
		p.General = "lvmeng"
		p.HP = 1
		sgGive(t, s, 0, "slash")
		sgGive(t, s, 0, "jink")
		s.sgPush(SGEvent{Type: "discard_phase", Actor: 0})
		s.sgRun()
		if s.Sanguosha.Pending.Kind != "keji" {
			t.Fatal("keji prompt")
		}
		sgDo(t, s, 0, Action{Choice: "yes"})
		p = &s.Sanguosha.Players[0]
		p.Used["keji_slash"] = 1
		s.sgPush(SGEvent{Type: "discard_phase", Actor: 0})
		s.sgRun()
		if s.Sanguosha.Pending.Kind != "discard" {
			t.Fatal("keji must not skip after slash")
		}
	})
	t.Run("yingzi_fanjian", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "zhouyu"
		s.sgPush(SGEvent{Type: "draw_phase", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Choice: "yingzi"})
		if len(s.Sanguosha.Players[0].Hand) != 3 {
			t.Fatal("yingzi")
		}
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "fanjian", Targets: []int{1}})
		sgDo(t, s, 1, Action{Choice: "spade"})
		if len(s.Sanguosha.Players[1].Hand) != 1 {
			t.Fatal("fanjian")
		}
		sgConservation(t, s)
	})
	t.Run("liuli", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[1].General = "daqiao"
		kill := sgGive(t, s, 0, "slash")
		cost := sgGive(t, s, 1, "jink")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{kill}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Cards: []int{cost}, Targets: []int{2}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 4 || s.Sanguosha.Players[2].HP != 3 {
			t.Fatal("liuli")
		}
		sgConservation(t, s)
	})
	t.Run("xiaoji", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "sunshangxiang"
		id := sgGive(t, s, 0, "crossbow")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}})
		other := sgGive(t, s, 0, "blade")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{other}})
		if s.Sanguosha.Pending.Kind != "invoke" {
			t.Fatal("xiaoji")
		}
		sgDo(t, s, 0, Action{Choice: "yes"})
		if len(s.Sanguosha.Players[0].Hand) != 2 {
			t.Fatal("xiaoji draw")
		}
		sgConservation(t, s)
	})
	t.Run("biyue", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "diaochan"
		sgDo(t, s, 0, Action{Type: "sg_end"})
		sgDo(t, s, 0, Action{Choice: "yes"})
		if len(s.Sanguosha.Players[0].Hand) != 1 {
			t.Fatal("biyue")
		}
		sgConservation(t, s)
	})
}
func TestSanguoshaPayingWeaponAndHorseChangesLegality(t *testing.T) {
	s := sgTestState()
	s.Sanguosha.Players[0].General = "guanyu"
	// Diamond crossbow is red. After it is turned into a slash, its unlimited
	// slash allowance is gone; it cannot pay for a second slash this phase.
	var id int
	for _, c := range sgCards {
		if c.Kind == "crossbow" && c.Suit == 3 {
			id = c.ID
		}
	}
	s.Sanguosha.Deck = sgRemove(s.Sanguosha.Deck, id)
	s.Sanguosha.Players[0].Equip = []int{id}
	s.Sanguosha.Players[0].Used["slash"] = 1
	if s.Apply(0, Action{Type: "sg_play", Skill: "wusheng", Cards: []int{id}, Targets: []int{1}}) == nil {
		t.Fatal("sacrificed crossbow retained allowance")
	}
	s.Sanguosha.Players[0].Used["slash"] = 0
	horse := sgGive(t, s, 0, "chitu")
	s.Sanguosha.Players[0].Hand = sgRemove(s.Sanguosha.Players[0].Hand, horse)
	s.Sanguosha.Players[0].Equip = append(s.Sanguosha.Players[0].Equip, horse)
	if s.Apply(0, Action{Type: "sg_play", Skill: "wusheng", Cards: []int{horse}, Targets: []int{2}}) == nil {
		t.Fatal("sacrificed horse retained distance")
	}
	sgConservation(t, s)
}
func TestSanguoshaHujiaCanUseAllyArmor(t *testing.T) {
	s := sgTestState()
	s.Sanguosha.Players[0].General = "caocao"
	s.Sanguosha.Players[1].General = "guojia"
	id := sgGive(t, s, 1, "eight_diagram")
	s.Sanguosha.Players[1].Hand = nil
	s.Sanguosha.Players[1].Equip = []int{id}
	slash := sgGive(t, s, 3, "slash")
	s.Turn = 3
	sgSetTop(t, s, 1, 0)
	sgDo(t, s, 3, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{0}})
	sgDo(t, s, 0, Action{Choice: "hujia"})
	sgDo(t, s, 1, Action{Choice: "eight_diagram"})
	if s.Sanguosha.Pending.Kind != "tiandu" {
		t.Fatal("ally judgment must trigger tiandu")
	}
	sgDo(t, s, 1, Action{Choice: "yes"})
	if s.Sanguosha.Players[0].HP != 4 {
		t.Fatal("hujia failed")
	}
	sgConservation(t, s)
}
func TestSanguoshaCollateralAndIceSword(t *testing.T) {
	for _, respond := range []bool{false, true} {
		s := sgTestState()
		weapon := sgGive(t, s, 1, "crossbow")
		s.Sanguosha.Players[1].Hand = nil
		s.Sanguosha.Players[1].Equip = []int{weapon}
		kill := sgGive(t, s, 1, "slash")
		trick := sgGive(t, s, 0, "collateral")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{trick}, Targets: []int{1, 2}})
		for s.Sanguosha.Pending.Kind == "nullification" {
			sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
		}
		if respond {
			sgDo(t, s, 1, Action{Cards: []int{kill}})
			sgPassAll(t, s)
			if s.Sanguosha.Players[2].HP != 3 {
				t.Fatal("collateral slash")
			}
		} else {
			sgDo(t, s, 1, Action{Choice: "pass"})
			if !slices.Contains(s.Sanguosha.Players[0].Hand, weapon) {
				t.Fatal("collateral weapon transfer")
			}
		}
		sgConservation(t, s)
	}
	s := sgTestState()
	s.Sanguosha.Players[1].General = "luxun"
	weapon := sgGive(t, s, 0, "ice_sword")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{weapon}})
	kill := sgGive(t, s, 0, "slash")
	sgGive(t, s, 1, "peach")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{kill}, Targets: []int{1}})
	sgDo(t, s, 1, Action{Choice: "pass"})
	sgDo(t, s, 0, Action{Choice: "yes"})
	sgDo(t, s, 0, Action{Choice: "hand"})
	if s.Sanguosha.Pending.Kind != "invoke" {
		t.Fatal("first ice discard must allow lianying")
	}
	sgDo(t, s, 1, Action{Choice: "yes"})
	if s.Sanguosha.Pending.Kind != "steal" {
		t.Fatal("second ice discard after lianying")
	}
	sgDo(t, s, 0, Action{Choice: "hand"})
	sgDo(t, s, 1, Action{Choice: "yes"})
	if s.Sanguosha.Players[1].HP != 4 {
		t.Fatal("ice sword must prevent damage")
	}
	sgConservation(t, s)
}
func TestSanguoshaLightningCanBeAcquiredByJianxiong(t *testing.T) {
	s := sgTestState()
	s.Sanguosha.Players[0].General = "caocao"
	id := sgGive(t, s, 0, "lightning")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}})
	sgSetTop(t, s, 0, 5)
	s.sgPush(SGEvent{Type: "delayed", Actor: 0})
	s.sgRun()
	for s.Sanguosha.Pending.Kind == "nullification" {
		sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
	}
	if s.Sanguosha.Pending.Kind != "invoke" {
		t.Fatal("lightning must trigger jianxiong")
	}
	sgDo(t, s, 0, Action{Choice: "yes"})
	if !slices.Contains(s.Sanguosha.Players[0].Hand, id) || s.Sanguosha.Players[0].HP != 1 {
		t.Fatal("lightning card should remain until damage ends")
	}
	sgConservation(t, s)
}
