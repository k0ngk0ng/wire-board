package game

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestSanguoshaBreakthroughCatalogLegacyAndMixed(t *testing.T) {
	classic, _ := NewSanguosha(8, SGOptions{})
	s, err := NewSanguosha(8, SGOptions{StandardVersion: "breakthrough", Deck: "military", Packs: []string{"wind", "fire", "thicket", "mountain", "god"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(classic.Sanguosha.generalCatalog()) != 25 || len(s.Sanguosha.generalCatalog()) != 65 {
		t.Fatal("version replacement changed pool size")
	}
	for _, general := range s.Sanguosha.generalCatalog() {
		if sgGeneral("jie_"+general.ID).ID != "" {
			t.Fatal("classic duplicate in revised pool", general.ID)
		}
	}
	if !slices.Contains(s.Sanguosha.Players[s.Sanguosha.Lord].Choices, "jie_caocao") || slices.Contains(s.Sanguosha.Players[s.Sanguosha.Lord].Choices, "caocao") {
		t.Fatal("wrong lord variants")
	}
	if _, err := NormalizeSGOptions(SGOptions{StandardVersion: "modern"}); err == nil {
		t.Fatal("unknown version accepted")
	}
	for _, version := range []int{0, 1} {
		old := sgJieState("sunce")
		old.Sanguosha.RulesVersion = version
		old.Sanguosha.Players[0].HP = 1
		sgRestoreMountain(t, old)
		old.sgMountainStart(0)
		want := "yingzi"
		if version == 1 {
			want = "jie_yingzi"
		}
		if !old.sgHas(0, want) {
			t.Fatal("associated skill changed saved version", version)
		}
	}
	groups := [][]string{
		{"jie_liubei", "jie_xiahoudun", "lusu", "shenlvbu", "jie_luxun", "jie_guojia", "caiwenji", "shenzhugeliang"},
		{"jie_caocao", "jie_machao", "xiaoqiao", "jie_huangyueying", "shenlvmeng", "yuji", "zuoci", "jie_huanggai"},
		{"sunce", "jie_ganning", "jie_lvbu", "jie_huatuo", "zhanghe", "jie_zhouyu", "shensimayi", "zhangjiao"},
	}
	for group, generals := range groups {
		s, _ := NewSanguosha(8, SGOptions{StandardVersion: "breakthrough", Deck: "military", Packs: []string{"wind", "fire", "thicket", "mountain", "god"}})
		g := s.Sanguosha
		g.Selecting = false
		g.Selected = 8
		g.Pending = nil
		g.Lord = 0
		s.Turn = 0
		for i, id := range generals {
			sgGodGeneral(s, i, id)
			g.Players[i].Role = []string{"lord", "loyalist", "rebel", "rebel", "renegade", "rebel", "rebel", "loyalist"}[i]
		}
		s.sgGodGameStart()
		s.sgRun()
		for step := 0; step < 10000 && !s.Finished; step++ {
			i := s.SanguoshaActor()
			a, err := s.BotAction(i)
			if err != nil {
				t.Fatalf("mixed group%d step%d prompt %+v: %v", group, step, s.Sanguosha.Pending, err)
			}
			if err := s.Apply(i, a); err != nil {
				t.Fatal(group, step, a, err)
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
			t.Fatal("mixed game stalled", group, s.Log)
		}
	}
}

func TestSanguoshaBreakthroughMovementAndLifecycle(t *testing.T) {
	t.Run("atomic exchanges have batch gains but no false empty hand", func(t *testing.T) {
		s := sgJieState("lusu", "jie_xiahoudun", "jie_luxun")
		s.sgAcquire(1, "jie_lianying")
		sgGive(t, s, 1, "jink")
		sgGive(t, s, 2, "slash")
		sgGive(t, s, 2, "peach")
		s.sgPush(SGEvent{Type: "dimeng_swap", Actor: 0, Targets: []int{1, 2}})
		s.sgRun()
		sgJiePrompt(t, s, 1, "qingjian")
		if len(s.Sanguosha.Pending.Cards) != 2 {
			t.Fatal("exchange split into gains")
		}
		sgDo(t, s, 1, Action{Choice: "pass"})
		if s.Sanguosha.Pending != nil {
			t.Fatal("exchange falsely emptied hand")
		}
		sgWindConservation(t, s)
	})
	t.Run("active death clears seals and private piles are discarded", func(t *testing.T) {
		s := sgJieState()
		s.Turn = 1
		s.Sanguosha.Players[2].Silenced = true
		s.Sanguosha.Players[2].HandSealed = true
		s.Sanguosha.Players[1].Yiji = s.sgDrawIDs(2)
		s.Sanguosha.Players[1].Qianxun = s.sgDrawIDs(2)
		s.sgDie(1, 0)
		s.sgRun()
		if s.Sanguosha.Players[2].Silenced || s.Sanguosha.Players[2].HandSealed || len(s.Sanguosha.Players[1].Yiji)+len(s.Sanguosha.Players[1].Qianxun) != 0 {
			t.Fatal("death cleanup")
		}
		sgWindConservation(t, s)
	})
	t.Run("Zhaxiang separate gains outside play and clears bonuses", func(t *testing.T) {
		s := sgJieState("jie_huanggai")
		s.sgAcquire(0, "qingjian")
		s.Sanguosha.ActivePhase = "finish"
		s.sgPush(SGEvent{Type: "lose_hp", Target: 0, Amount: 2})
		s.sgRun()
		for range 2 {
			sgJiePrompt(t, s, 0, "qingjian")
			if len(s.Sanguosha.Pending.Cards) != 3 {
				t.Fatal("Zhaxiang gain batches")
			}
			sgDo(t, s, 0, Action{Choice: "pass"})
		}
		if s.Sanguosha.Players[0].Used["zhaxiang"] != 0 || len(s.Sanguosha.Players[0].Hand) != 6 {
			t.Fatal("bonus outside play")
		}
		s.Sanguosha.Players[0].Used["zhaxiang"] = 1
		s.sgJieTurnEnd(0)
		if s.Sanguosha.Players[0].Used["zhaxiang"] != 0 {
			t.Fatal("bonus survived turn")
		}
		sgWindConservation(t, s)
	})
}

func sgJieState(generals ...string) *State {
	s := sgGodState()
	s.Sanguosha.ActivePhase = "play"
	s.Sanguosha.TurnSequence = 1
	for i, id := range generals {
		sgGodGeneral(s, i, id)
		s.Sanguosha.Players[i].BaseKingdom = ""
	}
	return s
}
func sgJiePrompt(t *testing.T, s *State, who int, kind string) {
	t.Helper()
	q := s.Sanguosha.Pending
	if q == nil || q.Kind != kind || q.Player != who {
		t.Fatalf("want %d/%s, got %+v; log %v", who, kind, q, s.Log)
	}
	sgWindConservation(t, s)
	sgRestoreMountain(t, s)
}
func sgJieNullPass(t *testing.T, s *State) {
	t.Helper()
	for n := 0; n < 16 && s.Sanguosha.Pending != nil && s.Sanguosha.Pending.Kind == "nullification"; n++ {
		sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
	}
}

func TestSanguoshaBreakthroughWei(t *testing.T) {
	t.Run("Jianxiong exclusive gain or draw", func(t *testing.T) {
		for _, choice := range []string{"take", "draw"} {
			s := sgJieState("zhangfei", "jie_caocao")
			id := sgGive(t, s, 0, "slash")
			sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
			sgDo(t, s, 1, Action{Choice: "pass"})
			sgJiePrompt(t, s, 1, "jie_jianxiong")
			sgDo(t, s, 1, Action{Choice: choice})
			p := s.Sanguosha.Players[1]
			if len(p.Hand) != 1 || slices.Contains(p.Hand, id) != (choice == "take") {
				t.Fatal("not exclusive", choice, p.Hand)
			}
			sgWindConservation(t, s)
		}
	})
	t.Run("Fankui per point stops on decline", func(t *testing.T) {
		s := sgJieState("zhangfei", "jie_simayi")
		sgGive(t, s, 0, "slash")
		sgGive(t, s, 0, "jink")
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 1, Amount: 2})
		s.sgRun()
		sgJiePrompt(t, s, 1, "invoke")
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgJiePrompt(t, s, 1, "steal")
		sgDo(t, s, 1, Action{Choice: "hand"})
		sgJiePrompt(t, s, 1, "invoke")
		sgDo(t, s, 1, Action{Choice: "pass"})
		if len(s.Sanguosha.Players[1].Hand) != 1 || s.Sanguosha.Pending != nil {
			t.Fatal("feedback continued after decline")
		}
	})
	t.Run("Guicai pays equipment and heals Silver Lion", func(t *testing.T) {
		s := sgJieState("zhangfei", "jie_simayi")
		s.Sanguosha.Players[1].HP = 2
		id := sgWear(t, s, 1, "silver_lion")
		s.sgPush(SGEvent{Type: "judge", Actor: 2, Kind: "tieji", Next: &SGEvent{Actor: 2, Target: 3, Kind: "slash", Amount: 1}})
		s.sgRun()
		sgJiePrompt(t, s, 1, "jie_guicai")
		sgDo(t, s, 1, Action{Cards: []int{id}})
		if s.Sanguosha.Players[1].HP != 3 || len(s.Sanguosha.Players[1].Equip) != 0 {
			t.Fatal("equipment loss hook")
		}
		sgPassAll(t, s)
		sgWindConservation(t, s)
	})
	t.Run("Ganglie red damage black discard and Qingjian batches", func(t *testing.T) {
		s := sgJieState("zhangfei", "jie_xiahoudun")
		id := sgGive(t, s, 0, "jink")
		sgSetTop(t, s, 1, 0)
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 1, Amount: 2})
		s.sgRun()
		sgDo(t, s, 1, Action{Choice: "yes"})
		if s.Sanguosha.Players[0].HP != 3 {
			t.Fatal("red judgment")
		}
		sgSetTop(t, s, 0, 0)
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgJiePrompt(t, s, 1, "steal")
		sgDo(t, s, 1, Action{Choice: "hand"})
		if !slices.Contains(s.Sanguosha.Discard, id) {
			t.Fatal("black judgment")
		}
		s.sgDraw(1, 2)
		s.sgRun()
		sgJiePrompt(t, s, 1, "qingjian")
		batch := clone(s.Sanguosha.Pending.Cards)
		late := sgGive(t, s, 1, "peach")
		sgGodIllegal(t, s, 1, Action{Cards: []int{late}, Targets: []int{2}})
		sgDo(t, s, 1, Action{Cards: batch[:1], Targets: []int{2}})
		sgJiePrompt(t, s, 1, "qingjian")
		sgDo(t, s, 1, Action{Cards: batch[1:], Targets: []int{3}})
		if !slices.Equal(s.Sanguosha.Players[1].Hand, []int{late}) {
			t.Fatal("wrong gain batch")
		}
		sgWindConservation(t, s)
		s.Turn = 1
		s.Sanguosha.ActivePhase = "draw"
		s.sgDraw(1, 2)
		s.sgRun()
		if s.Sanguosha.Pending != nil {
			t.Fatal("Qingjian during own draw phase")
		}
	})
	t.Run("Tuxi reduced normal draw eligibility", func(t *testing.T) {
		s := sgJieState("jie_zhangliao")
		sgGive(t, s, 0, "jink")
		stolen := sgGive(t, s, 1, "slash")
		next := s.Sanguosha.Deck[0]
		s.sgEvent(SGEvent{Type: "draw_phase", Actor: 0})
		sgGodIllegal(t, s, 0, Action{Choice: "jie_tuxi", Targets: []int{2}})
		sgDo(t, s, 0, Action{Choice: "jie_tuxi", Targets: []int{1}})
		if len(s.Sanguosha.Players[0].Hand) != 3 || !slices.Contains(s.Sanguosha.Players[0].Hand, stolen) || !slices.Contains(s.Sanguosha.Players[0].Hand, next) {
			t.Fatal("tuxi normal remainder")
		}
		sgWindConservation(t, s)
	})
	t.Run("Luoyi skips private-pile return and lasts through other turns", func(t *testing.T) {
		s := sgJieState("jie_xuchu")
		pile := s.sgDrawIDs(1)
		s.Sanguosha.Players[0].Yiji = pile
		s.sgPush(SGEvent{Type: "jie_luoyi", Actor: 0}, SGEvent{Type: "draw_phase", Actor: 0})
		s.sgRun()
		top := clone(s.Sanguosha.Deck[:3])
		want := 0
		for _, id := range top {
			c := sgCard(id)
			if sgBasic(c.Kind) || c.Kind == "duel" || SGCardTypes[c.Kind].Slot == "weapon" {
				want++
			}
		}
		sgDo(t, s, 0, Action{Choice: "yes"})
		if len(s.Sanguosha.Players[0].Hand) != want || len(s.Sanguosha.Players[0].Yiji) != 1 {
			t.Fatal("Luoyi replacement is a skipped draw")
		}
		s.Turn = 1
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 2, Kind: "duel", Amount: 1})
		s.sgRun()
		if s.Sanguosha.Players[2].HP != 2 {
			t.Fatal("out of turn bonus")
		}
		s.Sanguosha.Players[0].Flipped = true
		s.sgEvent(SGEvent{Type: "begin", Actor: 0})
		if !s.Sanguosha.Players[0].JieLuoyi {
			t.Fatal("flipped skip expired bonus")
		}
		s.Sanguosha.Queue = nil
		s.sgEvent(SGEvent{Type: "begin", Actor: 0})
		if s.Sanguosha.Players[0].JieLuoyi {
			t.Fatal("actual round start did not expire bonus")
		}
		sgWindConservation(t, s)
	})
	t.Run("Yiji may give four current-hand cards privately", func(t *testing.T) {
		s := sgJieState("zhangfei", "jie_guojia")
		sgGive(t, s, 1, "slash")
		sgGive(t, s, 1, "jink")
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 1, Amount: 1})
		s.sgRun()
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgJiePrompt(t, s, 1, "jie_yiji_targets")
		sgDo(t, s, 1, Action{Targets: []int{2, 3}})
		first := clone(s.Sanguosha.Players[1].Hand[:2])
		sgDo(t, s, 1, Action{Cards: first})
		second := clone(s.Sanguosha.Players[1].Hand)
		sgDo(t, s, 1, Action{Cards: second})
		for _, viewer := range []int{-1, 0, 1, 2, 3} {
			ps := s.sgView(viewer)["sanguosha"].(map[string]any)["players"].([]map[string]any)
			if ps[2]["yijiCount"] != 2 {
				t.Fatal("missing private-pile count")
			}
			_, visible := ps[2]["yiji"]
			if visible != (viewer == 2) {
				t.Fatal("private pile leaked", viewer)
			}
		}
		sgRestoreMountain(t, s)
		s.Turn = 2
		s.sgPush(SGEvent{Type: "draw_phase", Actor: 2})
		s.sgRun()
		if len(s.Sanguosha.Players[2].Yiji) != 0 || len(s.Sanguosha.Players[2].Hand) != 4 {
			t.Fatal("pile did not return at draw phase start")
		}
		sgWindConservation(t, s)
	})
}

func TestSanguoshaBreakthroughShu(t *testing.T) {
	t.Run("Rende uses a frozen hand and one session", func(t *testing.T) {
		s := sgJieState("jie_liubei")
		s.Sanguosha.Players[0].HP = 2
		a := sgGive(t, s, 0, "slash")
		b := sgGive(t, s, 0, "jink")
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "jie_rende", Cards: []int{a}, Targets: []int{1}})
		sgJiePrompt(t, s, 0, "jie_rende")
		late := sgGive(t, s, 0, "peach")
		sgGodIllegal(t, s, 0, Action{Cards: []int{late}, Targets: []int{2}})
		sgDo(t, s, 0, Action{Cards: []int{b}, Targets: []int{2}})
		if s.Sanguosha.Players[0].HP != 3 {
			t.Fatal("two-card recovery")
		}
		sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "jie_rende", Cards: []int{late}, Targets: []int{1}})
		sgWindConservation(t, s)
	})
	t.Run("Yijue suppresses optional skills and all hand conversions", func(t *testing.T) {
		s := sgJieState("jie_guanyu", "yuji")
		s.sgAcquire(1, "hongyan", "paoxiao", "longhun", "wusheng")
		high := sgRankGive(t, s, 0, 13)
		low := sgRankGive(t, s, 1, 2)
		card := sgGive(t, s, 1, "jink")
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "yijue", Cards: []int{high}, Targets: []int{1}})
		sgJiePrompt(t, s, 1, "pindian")
		sgDo(t, s, 1, Action{Cards: []int{low}})
		if !s.sgHas(1, "hongyan") || s.sgHas(1, "paoxiao") || s.sgHas(1, "guhuo") {
			t.Fatal("wrong compulsory metadata")
		}
		if _, err := s.sgAs(1, []int{card}, "", "jink"); err == nil {
			t.Fatal("sealed hand used")
		}
		if _, err := s.sgAs(1, []int{card}, "longhun", "jink"); err == nil {
			t.Fatal("sealed conversion used")
		}
		if s.sgValidateCards(1, []int{card}, 1, true) != nil {
			t.Fatal("hand seal incorrectly prevents discard/pindian")
		}
		sgRestoreMountain(t, s)
		s.sgJieTurnEnd(0)
		if !s.sgHas(1, "paoxiao") || !s.sgHas(1, "guhuo") || s.Sanguosha.Players[1].HandSealed {
			t.Fatal("turn end did not restore")
		}
		sgWindConservation(t, s)
	})
	t.Run("Yijue tie can heal", func(t *testing.T) {
		s := sgJieState("jie_guanyu")
		a := sgRankGive(t, s, 0, 7)
		b := sgRankGive(t, s, 1, 7)
		s.Sanguosha.Players[1].HP = 2
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "yijue", Cards: []int{a}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Cards: []int{b}})
		sgJiePrompt(t, s, 0, "yijue_heal")
		sgDo(t, s, 0, Action{Choice: "yes"})
		if s.Sanguosha.Players[1].HP != 3 || s.Sanguosha.Players[1].Silenced {
			t.Fatal("tie handling")
		}
	})
	t.Run("Tishen no first-round baseline then limited actual healing", func(t *testing.T) {
		s := sgJieState("jie_zhangfei")
		s.Sanguosha.Players[0].HP = 1
		s.sgEvent(SGEvent{Type: "jie_start", Actor: 0})
		if s.Sanguosha.Pending != nil {
			t.Fatal("invented prior turn HP")
		}
		s.Sanguosha.Players[0].PreviousHPSet = true
		s.Sanguosha.Players[0].PreviousHP = 4
		s.Sanguosha.Players[0].MaxHP = 3
		s.sgEvent(SGEvent{Type: "jie_start", Actor: 0})
		sgDo(t, s, 0, Action{Choice: "yes"})
		if s.Sanguosha.Players[0].HP != 3 || len(s.Sanguosha.Players[0].Hand) != 2 {
			t.Fatal("healed count")
		}
		s.Sanguosha.Players[0].HP = 1
		s.sgEvent(SGEvent{Type: "jie_start", Actor: 0})
		if s.Sanguosha.Pending != nil {
			t.Fatal("limited reused")
		}
	})
	t.Run("Yajiao response category and discard does not trigger", func(t *testing.T) {
		s := sgJieState("zhangfei", "jie_zhaoyun")
		a := sgGive(t, s, 0, "slash")
		b := sgGive(t, s, 1, "jink")
		top := sgTop(t, s, func(c SGCard) bool { return sgBasic(c.Kind) })
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{a}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Cards: []int{b}})
		sgJiePrompt(t, s, 1, "invoke")
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgJiePrompt(t, s, 1, "yajiao")
		sgDo(t, s, 1, Action{Choice: "give", Targets: []int{1}})
		if !slices.Contains(s.Sanguosha.Players[1].Hand, top) {
			t.Fatal("yajiao did not obtain")
		}
		s.sgDiscard(1, []int{top})
		s.sgRun()
		if s.Sanguosha.Pending != nil {
			t.Fatal("discard incorrectly triggered Yajiao")
		}
		sgWindConservation(t, s)
	})
	t.Run("Tieji optional silence retains Hongyan for discard suit", func(t *testing.T) {
		s := sgJieState("jie_machao", "xiaoqiao")
		a := sgGive(t, s, 0, "slash")
		b := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 0 })
		jink := sgGive(t, s, 1, "jink")
		sgSetTop(t, s, 1, 0)
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{a}, Targets: []int{1}})
		sgJiePrompt(t, s, 0, "jie_tieji")
		sgDo(t, s, 0, Action{Choice: "yes"})
		sgJiePrompt(t, s, 1, "jie_tieji_discard")
		if s.sgHas(1, "tianxiang") || !s.sgHas(1, "hongyan") {
			t.Fatal("skill frequency")
		}
		sgDo(t, s, 1, Action{Cards: []int{b}})
		sgJiePrompt(t, s, 1, "card")
		sgDo(t, s, 1, Action{Cards: []int{jink}})
		if s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("tieji did not permit jink after cost")
		}
	})
}
