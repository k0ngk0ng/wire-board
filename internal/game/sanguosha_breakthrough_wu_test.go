package game

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestSanguoshaBreakthroughWuAndQun(t *testing.T) {
	t.Run("Jizhi includes delayed tricks and shares basic exchange", func(t *testing.T) {
		s := sgJieState("jie_huangyueying")
		use := sgGive(t, s, 0, "indulgence")
		cost := sgGive(t, s, 0, "slash")
		top := sgTop(t, s, func(c SGCard) bool { return c.Kind == "peach" })
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{use}, Targets: []int{1}})
		sgJiePrompt(t, s, 0, "invoke")
		sgDo(t, s, 0, Action{Choice: "yes"})
		sgJiePrompt(t, s, 0, "jilve_jizhi_exchange")
		sgDo(t, s, 0, Action{Cards: []int{cost}})
		if s.Sanguosha.Deck[0] != cost || !slices.Equal(s.Sanguosha.Players[0].Hand, []int{top}) {
			t.Fatal("basic exchange")
		}
		if len(s.Sanguosha.Players[1].Judgment) != 1 {
			t.Fatal("delayed trick did not finish")
		}
		sgWindConservation(t, s)
	})
	t.Run("Qicai protects equipment from discard but not acquisition", func(t *testing.T) {
		s := sgJieState("zhangfei", "jie_huangyueying")
		armor := sgWear(t, s, 1, "eight_diagram")
		weapon := sgWear(t, s, 1, "crossbow")
		remove := sgGive(t, s, 0, "dismantlement")
		sgGodIllegal(t, s, 0, Action{Type: "sg_play", Cards: []int{remove}, Targets: []int{1}})
		horse := sgWear(t, s, 1, "jueying")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{remove}, Targets: []int{1}})
		sgJieNullPass(t, s)
		sgJiePrompt(t, s, 0, "steal")
		sgGodIllegal(t, s, 0, Action{Card: armor})
		sgGodIllegal(t, s, 0, Action{Card: weapon})
		sgDo(t, s, 0, Action{Card: horse})
		grab := sgGive(t, s, 0, "snatch")
		// The discarded defensive horse restores distance one.
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{grab}, Targets: []int{1}})
		sgJieNullPass(t, s)
		sgDo(t, s, 0, Action{Card: armor})
		if !slices.Contains(s.Sanguosha.Players[0].Hand, armor) {
			t.Fatal("Qicai prevented obtaining")
		}
		sgWindConservation(t, s)
	})
	t.Run("Fenwei cancels selected original targets before nullification", func(t *testing.T) {
		s := sgJieState("zhangfei", "jie_ganning", "jie_luxun")
		sgGive(t, s, 2, "jink")
		id := sgGive(t, s, 0, "savage_assault")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}})
		sgJiePrompt(t, s, 1, "fenwei")
		sgGodIllegal(t, s, 1, Action{Targets: []int{0}})
		sgDo(t, s, 1, Action{Targets: []int{1, 3}})
		sgJieNullPass(t, s)
		// Removing all but one effective target must not make this a sole-target trick.
		sgJiePrompt(t, s, 2, "card")
		sgDo(t, s, 2, Action{Choice: "pass"})
		if s.Sanguosha.Players[1].HP != 4 || s.Sanguosha.Players[2].HP != 2 || s.Sanguosha.Players[3].HP != 4 {
			t.Fatal("wrong canceled subset")
		}
		if s.Sanguosha.Players[1].Marks["fenwei"] != 1 {
			t.Fatal("limited mark")
		}
		sgWindConservation(t, s)
	})
	t.Run("Qinxue seven-player threshold and Gongxin acquisition", func(t *testing.T) {
		s := sgJieState("jie_lvmeng")
		s.Sanguosha.Players[0].HP = 2
		s.sgDraw(0, 4)
		s.sgEvent(SGEvent{Type: "jie_start", Actor: 0})
		if s.sgHas(0, "gongxin") {
			t.Fatal("awakened below four-player threshold")
		}
		s.sgDraw(0, 1)
		s.sgEvent(SGEvent{Type: "jie_start", Actor: 0})
		if !s.sgHas(0, "gongxin") || s.Sanguosha.Players[0].MaxHP != 3 {
			t.Fatal("did not awaken")
		}
		s.sgEvent(SGEvent{Type: "jie_start", Actor: 0})
		if s.Sanguosha.Players[0].MaxHP != 3 {
			t.Fatal("awakened twice")
		}
		sgWindConservation(t, s)
	})
	t.Run("Kurou costs then Zhaxiang draws and makes red slash unavoidable", func(t *testing.T) {
		s := sgJieState("jie_huanggai")
		cost := sgGive(t, s, 0, "jink")
		red := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Kind == "slash" && c.Suit == 1 })
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "jie_kurou", Cards: []int{cost}})
		if len(s.Sanguosha.Players[0].Hand) != 4 || s.Sanguosha.Players[0].HP != 3 || s.Sanguosha.Players[0].Used["zhaxiang"] != 1 {
			t.Fatal("kurou/zhaxiang")
		}
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{red}, Targets: []int{2}})
		if s.Sanguosha.Pending != nil || s.Sanguosha.Players[2].HP != 3 {
			t.Fatal("red slash range/jink restriction")
		}
		second := sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{second}, Targets: []int{1}})
		sgPassAll(t, s)
		third := sgGive(t, s, 0, "slash")
		sgGodIllegal(t, s, 0, Action{Type: "sg_play", Cards: []int{third}, Targets: []int{1}})
		sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "jie_kurou", Cards: []int{third}})
		sgWindConservation(t, s)
	})
	t.Run("Yingzi fixed limit and Fanjian suit or HP", func(t *testing.T) {
		for _, choice := range []string{"discard", "lose_hp"} {
			s := sgJieState("jie_zhouyu")
			s.Sanguosha.Players[0].HP = 1
			if s.sgHandLimit(0) != 3 || s.sgGodDrawCount(0, 2) != 3 {
				t.Fatal("Yingzi is compulsory")
			}
			give := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 1 })
			other := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 1 })
			keep := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 2 })
			sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "jie_fanjian", Cards: []int{give}, Targets: []int{1}})
			sgJiePrompt(t, s, 1, "jie_fanjian")
			sgDo(t, s, 1, Action{Choice: choice})
			if choice == "discard" {
				if !slices.Equal(s.Sanguosha.Players[1].Hand, []int{keep}) || !slices.Contains(s.Sanguosha.Discard, other) || !slices.Contains(s.Sanguosha.Discard, give) {
					t.Fatal("wrong suit discarded")
				}
			} else if s.Sanguosha.Players[1].HP != 3 || len(s.Sanguosha.Players[1].Hand) != 3 {
				t.Fatal("wrong HP choice")
			}
			sgWindConservation(t, s)
		}
	})
	t.Run("Guose diamond can place or remove self indulgence", func(t *testing.T) {
		for _, choice := range []string{"use", "remove"} {
			s := sgJieState("jie_daqiao")
			cost := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 3 })
			to := 1
			if choice == "remove" {
				to = 0
				id := sgGive(t, s, 0, "indulgence")
				s.Sanguosha.Players[0].Hand = sgRemove(s.Sanguosha.Players[0].Hand, id)
				s.Sanguosha.Players[0].Judgment = append(s.Sanguosha.Players[0].Judgment, SGDelayed{Card: id, Kind: "indulgence"})
			}
			sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "jie_guose", Choice: choice, Cards: []int{cost}, Targets: []int{to}})
			if len(s.Sanguosha.Players[0].Hand) != 1 || (len(s.Sanguosha.Players[to].Judgment) == 1) != (choice == "use") {
				t.Fatal("guose branch", choice)
			}
			sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "jie_guose", Choice: choice, Cards: clone(s.Sanguosha.Players[0].Hand), Targets: []int{to}})
			sgWindConservation(t, s)
		}
	})
	t.Run("Qianxun private storage Lianying and return despite skill loss", func(t *testing.T) {
		s := sgJieState("zhangfei", "jie_luxun")
		stored := []int{sgGive(t, s, 1, "slash"), sgGive(t, s, 1, "jink")}
		id := sgGive(t, s, 0, "dismantlement")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		sgJieNullPass(t, s)
		sgJiePrompt(t, s, 1, "jie_qianxun")
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgJiePrompt(t, s, 1, "jie_lianying")
		sgGodIllegal(t, s, 1, Action{Targets: []int{1, 1}})
		sgDo(t, s, 1, Action{Targets: []int{2, 3}})
		if !slices.Equal(s.Sanguosha.Players[1].Qianxun, stored) || len(s.Sanguosha.Players[2].Hand) != 1 {
			t.Fatal("Qianxun/Lianying")
		}
		for _, viewer := range []int{-1, 0, 1, 2} {
			ps := s.sgView(viewer)["sanguosha"].(map[string]any)["players"].([]map[string]any)
			_, visible := ps[1]["qianxun"]
			if visible != (viewer == 1) {
				t.Fatal("private Qianxun leak")
			}
		}
		sgRestoreMountain(t, s)
		s.sgLoseSkills(1)
		s.sgJieTurnEnd(0)
		s.sgRun()
		if !slices.Equal(s.Sanguosha.Players[1].Hand, stored) || len(s.Sanguosha.Players[1].Qianxun) != 0 {
			t.Fatal("piles stranded after skill loss")
		}
		sgWindConservation(t, s)
	})
	t.Run("Chuli kingdom checks and all discards before spade draws", func(t *testing.T) {
		s := sgJieState("jie_huatuo", "jie_simayi", "zhangfei", "jie_zhouyu")
		cost := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 0 })
		for i := 1; i < 4; i++ {
			sgFindGive(t, s, i, func(c SGCard) bool { return c.Suit == 0 })
		}
		sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "chuli", Cards: []int{cost}, Targets: []int{1, 1}})
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "chuli", Cards: []int{cost}, Targets: []int{1, 2, 3}})
		for i := 1; i < 4; i++ {
			sgJiePrompt(t, s, 0, "steal")
			if len(s.Sanguosha.Players[0].Hand) != 0 {
				t.Fatal("spade reward before finishing discards")
			}
			sgDo(t, s, 0, Action{Choice: "hand"})
		}
		for _, p := range s.Sanguosha.Players {
			if len(p.Hand) != 1 {
				t.Fatal("spade rewards", p)
			}
		}
		sgWindConservation(t, s)
	})
	t.Run("Liyu victim chooses third party and Lijian is nullifiable", func(t *testing.T) {
		s := sgJieState("jie_lvbu")
		a := sgGive(t, s, 0, "slash")
		sgGive(t, s, 1, "peach")
		if s.Sanguosha.Players[0].MaxHP != 5 {
			t.Fatal("pinned Lvbu HP")
		}
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{a}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Choice: "pass"})
		sgJiePrompt(t, s, 1, "liyu")
		sgGodIllegal(t, s, 1, Action{Targets: []int{0}})
		sgDo(t, s, 1, Action{Targets: []int{2}})
		sgJiePrompt(t, s, 0, "steal")
		sgDo(t, s, 0, Action{Choice: "hand"})
		sgJiePrompt(t, s, -1, "nullification")
		sgPassAll(t, s)
		sgWindConservation(t, s)
		s = sgJieState("jie_diaochan")
		cost := sgGive(t, s, 0, "jink")
		counter := sgGive(t, s, 3, "nullification")
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "jie_lijian", Cards: []int{cost}, Targets: []int{1, 2}})
		sgJiePrompt(t, s, -1, "nullification")
		sgDo(t, s, 3, Action{Cards: []int{counter}})
		sgJieNullPass(t, s)
		if s.Sanguosha.Players[1].HP != 4 || s.Sanguosha.Players[2].HP != 4 {
			t.Fatal("revised Lijian was not nullified")
		}
		sgWindConservation(t, s)
	})
}

func TestSanguoshaBreakthroughBotsRestore(t *testing.T) {
	for offset := range 21 {
		s := sgJieState()
		shuffle(s.Sanguosha.Deck)
		for i := range s.Sanguosha.Players {
			id := sgJieGenerals[(offset+i)%len(sgJieGenerals)].ID
			sgGodGeneral(s, i, id)
			s.Sanguosha.Players[i].BaseKingdom = ""
			s.sgDraw(i, 4)
		}
		s.Sanguosha.Queue = nil // fixture initial deal never triggers Qingjian
		s.sgPush(SGEvent{Type: "begin", Actor: 0})
		s.sgRun()
		for step := 0; step < 6500 && !s.Finished; step++ {
			i := s.SanguoshaActor()
			a, err := s.BotAction(i)
			if err != nil {
				t.Fatalf("offset %d step %d pending %+v: %v", offset, step, s.Sanguosha.Pending, err)
			}
			if err = s.Apply(i, a); err != nil {
				t.Fatal(offset, step, a, err)
			}
			sgWindConservation(t, s)
			if step%97 == 0 {
				raw, _ := json.Marshal(s)
				var next State
				if err := json.Unmarshal(raw, &next); err != nil {
					t.Fatal(err)
				}
				s = &next
			}
		}
		if !s.Finished {
			t.Fatal("bots stalled", offset, s.Log)
		}
	}
}
