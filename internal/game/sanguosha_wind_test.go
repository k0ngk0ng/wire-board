package game

import (
	"encoding/json"
	"slices"
	"testing"
)

func sgWindState() *State {
	s := sgMilitaryTestState()
	s.Sanguosha.Options.Packs = []string{"standard", "wind"}
	s.Sanguosha.InPlay = true
	return s
}
func sgFindGive(t *testing.T, s *State, i int, match func(SGCard) bool) int {
	t.Helper()
	g := s.Sanguosha
	for _, id := range g.Deck {
		if match(sgCard(id)) {
			g.Deck = sgRemove(g.Deck, id)
			g.Players[i].Hand = append(g.Players[i].Hand, id)
			return id
		}
	}
	t.Fatal("no matching card")
	return 0
}
func sgTop(t *testing.T, s *State, match func(SGCard) bool) int {
	t.Helper()
	g := s.Sanguosha
	for j, id := range g.Deck {
		if match(sgCard(id)) {
			g.Deck[0], g.Deck[j] = g.Deck[j], g.Deck[0]
			return id
		}
	}
	t.Fatal("no matching top card")
	return 0
}
func sgWindConservation(t *testing.T, s *State) {
	t.Helper()
	trial := clone(*s)
	for i, p := range trial.Sanguosha.Players {
		trial.Sanguosha.Discard = append(trial.Sanguosha.Discard, p.Buqu...)
		trial.Sanguosha.Discard = append(trial.Sanguosha.Discard, p.Fields...)
		trial.Sanguosha.Players[i].Fields = nil
		trial.Sanguosha.Players[i].Buqu = nil
	}
	// The mandatory removal prompt references cards already in the public pile.
	sgMilitaryConservation(t, &trial)
}

func TestSanguoshaWindSelectionAndFiltering(t *testing.T) {
	base, _ := NewSanguosha(8, SGOptions{})
	wind, err := NewSanguosha(8, SGOptions{Packs: []string{"wind", "wind"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(base.Sanguosha.generalCatalog()) != 25 || len(wind.Sanguosha.generalCatalog()) != 33 {
		t.Fatal("pack catalog")
	}
	if !slices.Contains(wind.Sanguosha.Players[wind.Sanguosha.Lord].Choices, "zhangjiao") {
		t.Fatal("wind lord missing")
	}
	if !slices.Equal(wind.Sanguosha.Options.Packs, []string{"standard", "wind"}) {
		t.Fatal("not normalized")
	}
	sgDo(t, wind, wind.Sanguosha.Lord, Action{Choice: "zhangjiao"})
	seen := map[string]bool{"zhangjiao": true}
	for i, p := range wind.Sanguosha.Players {
		if i == wind.Sanguosha.Lord {
			continue
		}
		for _, id := range p.Choices {
			if seen[id] {
				t.Fatal("duplicate candidate", id)
			}
			seen[id] = true
		}
	}
}

func TestSanguoshaWindHongyanPublicUseAndRestore(t *testing.T) {
	s := sgWindState()
	s.Sanguosha.Players[0].General = "xiaoqiao"
	id := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Kind == "slash" && c.Suit == 0 })
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
	raw, _ := json.Marshal(s)
	if err := json.Unmarshal(raw, s); err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []int{-1, 0, 1} {
		cards := s.View(viewer)["sanguosha"].(map[string]any)["cards"].([]SGCard)
		if cards[id-1].Suit != 1 {
			t.Fatal("public Hongyan slash displayed as spade")
		}
	}
	sgPassAll(t, s)
	if len(s.Sanguosha.TableSuits) != 0 || s.sgVisibleCatalog(-1)[id-1].Suit != 0 {
		t.Fatal("effective suit leaked into discard/deck catalog")
	}
	sgWindConservation(t, s)
}
func TestSanguoshaWindShensu(t *testing.T) {
	t.Run("skip judgment and draw, attack at any range", func(t *testing.T) {
		s := sgWindState()
		g := s.Sanguosha
		g.Players[0].General = "xiahouyuan"
		id := sgGive(t, s, 0, "indulgence")
		g.Players[0].Hand = sgRemove(g.Players[0].Hand, id)
		g.Players[0].Judgment = []SGDelayed{{Card: id, Kind: "indulgence"}}
		s.sgPush(SGEvent{Type: "begin", Actor: 0})
		s.sgRun()
		if s.Sanguosha.Pending.Kind != "shensu_judge" {
			t.Fatal(s.Sanguosha.Pending)
		}
		sgDo(t, s, 0, Action{Targets: []int{2}})
		sgPassAll(t, s)
		g = s.Sanguosha
		if len(g.Players[0].Hand) != 0 || len(g.Players[0].Judgment) != 1 || g.Players[2].HP != 3 || !g.InPlay {
			t.Fatal("shensu phase skip failed")
		}
		// The extra slash does not consume the regular play-phase allowance.
		slash := sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{1}})
		sgPassAll(t, s)
		sgWindConservation(t, s)
	})
	t.Run("discard equipment, skip play, silver lion loss trigger", func(t *testing.T) {
		s := sgWindState()
		s.Sanguosha.Players[0].General = "xiahouyuan"
		s.Sanguosha.Players[0].HP = 2
		id := sgWear(t, s, 0, "silver_lion")
		s.sgPush(SGEvent{Type: "shensu_play", Actor: 0}, SGEvent{Type: "play_phase", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Cards: []int{id}, Targets: []int{2}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[0].HP != 3 || s.Sanguosha.Players[2].HP != 3 || s.Turn != 1 {
			t.Fatal("shensu cost or phase")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaWindJushouAndLiegong(t *testing.T) {
	t.Run("jushou three draws, turnover skips full next turn", func(t *testing.T) {
		s := sgWindState()
		s.Sanguosha.Players[0].General = "caoren"
		sgDo(t, s, 0, Action{Type: "sg_end"})
		if q := s.Sanguosha.Pending; q == nil || q.Event.Kind != "jushou" {
			t.Fatal(q)
		}
		sgDo(t, s, 0, Action{Choice: "yes"})
		if len(s.Sanguosha.Players[0].Hand) != 3 || !s.Sanguosha.Players[0].Flipped {
			t.Fatal("jushou")
		}
		s.sgPush(SGEvent{Type: "begin", Actor: 0})
		s.sgRun()
		if s.Turn != 1 || s.Sanguosha.Players[0].Flipped || len(s.Sanguosha.Players[0].Hand) != 3 {
			t.Fatal("turned-over turn not skipped")
		}
		sgWindConservation(t, s)
	})
	t.Run("liegong prevents jink but not armor immunity", func(t *testing.T) {
		s := sgWindState()
		s.Sanguosha.Players[0].General = "huangzhong"
		slash := sgGive(t, s, 0, "slash")
		jink := sgGive(t, s, 1, "jink")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{1}})
		if s.Sanguosha.Pending.Kind != "liegong" {
			t.Fatal(s.Sanguosha.Pending)
		}
		sgDo(t, s, 0, Action{Choice: "yes"})
		if s.Sanguosha.Players[1].HP != 3 || !slices.Contains(s.Sanguosha.Players[1].Hand, jink) {
			t.Fatal("liegong response")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaWindKuangguDistanceBeforeDeath(t *testing.T) {
	for _, target := range []int{1, 2} {
		s := sgWindState()
		s.Sanguosha.Players[0].General = "weiyan"
		s.Sanguosha.Players[0].HP = 2
		s.Sanguosha.Players[target].HP = 1
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: target, Amount: 2, Kind: "duel"})
		s.sgRun()
		sgPassAll(t, s)
		want := 2
		if target == 1 {
			want = 4
		}
		if s.Sanguosha.Players[0].HP != want {
			t.Fatal(target, s.Sanguosha.Players[0].HP, want)
		}
		if !s.Sanguosha.Players[target].Dead {
			t.Fatal("test did not kill target")
		}
		sgWindConservation(t, s)
	}
}
func TestSanguoshaWindTianxiangAndHongyan(t *testing.T) {
	t.Run("transfer before recipient armor, retain source, draw lost hp", func(t *testing.T) {
		s := sgWindState()
		g := s.Sanguosha
		g.Players[1].General = "xiaoqiao"
		g.Players[1].HP = 3
		g.Players[1].MaxHP = 3
		cost := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 0 })
		sgWear(t, s, 0, "qinggang_sword")
		sgWear(t, s, 1, "vine")
		sgWear(t, s, 2, "vine")
		fire := sgGive(t, s, 0, "fire_slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{fire}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Choice: "pass"}) // decline the slash response
		if s.Sanguosha.Pending.Kind != "tianxiang" {
			t.Fatal(s.Sanguosha.Pending)
		}
		raw, _ := json.Marshal(s)
		var restored State
		if err := json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		sgDo(t, &restored, 1, Action{Cards: []int{cost}, Targets: []int{2}})
		sgPassAll(t, &restored)
		g = restored.Sanguosha
		if g.Players[1].HP != 3 || g.Players[2].HP != 2 || len(g.Players[2].Hand) != 2 {
			t.Fatal("transfer armor/draw", g.Players[2])
		}
		sgWindConservation(t, &restored)
	})
	t.Run("hongyan red slash and private catalog", func(t *testing.T) {
		s := sgWindState()
		s.Sanguosha.Players[0].General = "xiaoqiao"
		id := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Kind == "slash" && c.Suit == 0 })
		own := s.sgVisibleCatalog(0)
		other := s.sgVisibleCatalog(1)
		if own[id-1].Suit != 1 || other[id-1].Suit != 0 || sgCard(id).Suit != 0 {
			t.Fatal("hongyan leaked or mutated catalog")
		}
		sgWear(t, s, 1, "renwang_shield")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("red slash blocked by renwang")
		}
		sgWindConservation(t, s)
	})
	t.Run("hongyan judgment cannot be a spade lightning hit", func(t *testing.T) {
		s := sgWindState()
		s.Sanguosha.Players[0].General = "xiaoqiao"
		id := sgGive(t, s, 0, "lightning")
		s.Sanguosha.Players[0].Hand = sgRemove(s.Sanguosha.Players[0].Hand, id)
		s.Sanguosha.Table = append(s.Sanguosha.Table, id)
		sgTop(t, s, func(c SGCard) bool { return c.Suit == 0 && c.Rank >= 2 && c.Rank <= 9 })
		s.sgPush(SGEvent{Type: "judge", Actor: 0, Kind: "lightning", Cards: []int{id}})
		s.sgRun()
		sgPassAll(t, s)
		if s.Sanguosha.Players[0].HP != 4 || len(s.Sanguosha.Players[1].Judgment) != 1 {
			t.Fatal("hongyan lightning")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaWindLeijiGuidaoHuangtian(t *testing.T) {
	t.Run("jink launches leiji with equipment retrial", func(t *testing.T) {
		s := sgWindState()
		s.Sanguosha.Players[1].General = "zhangjiao"
		jink := sgGive(t, s, 1, "jink")
		slash := sgGive(t, s, 0, "slash")
		black := sgWear(t, s, 1, "qinggang_sword")
		sgTop(t, s, func(c SGCard) bool { return c.Suit == 1 })
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Cards: []int{jink}})
		if s.Sanguosha.Pending.Kind != "leiji" {
			t.Fatal(s.Sanguosha.Pending)
		}
		sgDo(t, s, 1, Action{Targets: []int{0}})
		if s.Sanguosha.Pending.Kind != "guidao" {
			t.Fatal(s.Sanguosha.Pending)
		}
		original := s.Sanguosha.Pending.Event.Aux
		sgDo(t, s, 1, Action{Cards: []int{black}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[0].HP != 2 || !slices.Contains(s.Sanguosha.Players[1].Hand, original) || len(s.Sanguosha.Players[1].Equip) != 0 {
			t.Fatal("leiji/retrial")
		}
		sgWindConservation(t, s)
	})
	t.Run("huangtian is a group donation to the lord once", func(t *testing.T) {
		s := sgWindState()
		s.Sanguosha.Players[0].General = "zhangjiao"
		s.Sanguosha.Players[1].General = "huatuo"
		s.Turn = 1
		id := sgGive(t, s, 1, "jink")
		next := sgGive(t, s, 1, "lightning")
		sgDo(t, s, 1, Action{Type: "sg_skill", Skill: "huangtian_give", Cards: []int{id}, Targets: []int{0}})
		if !slices.Contains(s.Sanguosha.Players[0].Hand, id) {
			t.Fatal("not given")
		}
		if s.Apply(1, Action{Type: "sg_skill", Skill: "huangtian_give", Cards: []int{next}, Targets: []int{0}}) == nil {
			t.Fatal("second donation accepted")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaWindBuquRecoveryAndPersistence(t *testing.T) {
	s := sgWindState()
	g := s.Sanguosha
	g.Players[1].General = "zhoutai"
	g.Players[1].HP = 1
	peach := sgGive(t, s, 0, "peach")
	// Two equal points force the classic Buqu user to still ask for rescue.
	first := sgTop(t, s, func(c SGCard) bool { return c.Rank == 7 })
	g.Deck = g.Deck[1:]
	second := sgTop(t, s, func(c SGCard) bool { return c.Rank == 7 })
	g.Deck = append([]int{first}, g.Deck...)
	s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 1, Kind: "duel", Amount: 2})
	s.sgRun()
	if s.Sanguosha.Pending.Kind != "buqu" {
		t.Fatal(s.Sanguosha.Pending)
	}
	sgDo(t, s, 1, Action{Choice: "yes"})
	if s.Sanguosha.Pending.Kind != "peach" {
		t.Fatal("duplicate ranks skipped rescue")
	}
	sgDo(t, s, 0, Action{Cards: []int{peach}})
	if q := s.Sanguosha.Pending; q.Kind != "buqu_remove" || q.Event.Amount != 1 {
		t.Fatal(q)
	}
	raw, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	sgDo(t, &restored, 1, Action{Cards: []int{second}})
	sgPassAll(t, &restored)
	g = restored.Sanguosha
	if g.Players[1].HP != 0 || g.Players[1].Dead || !slices.Equal(g.Players[1].Buqu, []int{first}) {
		t.Fatal("failed to survive via buqu", g.Players[1])
	}
	restored.sgHeal(1, 1)
	restored.sgRun()
	if len(restored.Sanguosha.Players[1].Buqu) != 0 {
		t.Fatal("healed buqu not cleared")
	}
	sgWindConservation(t, &restored)
}

func TestSanguoshaWindBotsAndSaveRestore(t *testing.T) {
	// Force every classic Wind general through whole matches, including Yuji.
	for offset := 0; offset < 8; offset++ {
		s := sgWindState()
		for i := range s.Sanguosha.Players {
			general := sgWindGenerals[(offset+i)%8]
			s.Sanguosha.Players[i].General = general.ID
			s.Sanguosha.Players[i].HP = general.HP
			s.Sanguosha.Players[i].MaxHP = general.HP
			s.sgDraw(i, 4)
		}
		s.sgPush(SGEvent{Type: "begin", Actor: 0})
		s.sgRun()
		for step := 0; step < 8000 && !s.Finished; step++ {
			i := s.SanguoshaActor()
			a, err := s.BotAction(i)
			if err != nil {
				t.Fatalf("offset=%d step=%d pending=%+v: %v", offset, step, s.Sanguosha.Pending, err)
			}
			if err = s.Apply(i, a); err != nil {
				t.Fatal(a, err)
			}
			sgWindConservation(t, s)
			if step%101 == 0 {
				raw, _ := json.Marshal(s)
				var next State
				if err = json.Unmarshal(raw, &next); err != nil {
					t.Fatal(err)
				}
				s = &next
			}
		}
		if !s.Finished {
			t.Fatalf("offset=%d stalled", offset)
		}
	}
}
