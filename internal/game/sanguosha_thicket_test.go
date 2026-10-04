package game

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestSanguoshaThicketXingshangDeathRestore(t *testing.T) {
	s := sgFireState()
	s.Sanguosha.Players[0].General = "caopi"
	s.Sanguosha.Players[1].HP = 1
	hand := sgGive(t, s, 1, "jink")
	equip := sgWear(t, s, 1, "silver_lion")
	delay := sgGive(t, s, 1, "lightning")
	p := &s.Sanguosha.Players[1]
	p.Hand = sgRemove(p.Hand, delay)
	p.Judgment = append(p.Judgment, SGDelayed{Card: delay, Kind: "lightning"})
	s.sgPush(SGEvent{Type: "damage", Actor: 2, Target: 1, Amount: 1})
	s.sgRun()
	for s.Sanguosha.Pending != nil && s.Sanguosha.Pending.Kind == "peach" {
		sgDo(t, s, s.Sanguosha.Pending.Player, Action{Choice: "pass"})
	}
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "xingshang" || q.Player != 0 {
		t.Fatal("inheritance missing", q)
	}
	if len(s.Sanguosha.Players[2].Hand) != 0 {
		t.Fatal("kill reward ran before inheritance")
	}
	raw, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	s = &restored
	sgDo(t, s, 0, Action{Choice: "yes"})
	if !slices.Contains(s.Sanguosha.Players[0].Hand, hand) || !slices.Contains(s.Sanguosha.Players[0].Hand, equip) || !slices.Contains(s.Sanguosha.Discard, delay) {
		t.Fatal("wrong inherited zones")
	}
	if len(s.Sanguosha.Players[2].Hand) != 3 {
		t.Fatal("kill reward not resumed")
	}
	sgWindConservation(t, s)
}
func TestSanguoshaThicketFangzhuAndSongwei(t *testing.T) {
	s := sgFireState()
	s.Sanguosha.Players[0].General = "caopi"
	s.Sanguosha.Players[0].MaxHP = 3
	s.Sanguosha.Players[0].HP = 3
	s.sgPush(SGEvent{Type: "damage", Actor: 1, Target: 0, Amount: 2})
	s.sgRun()
	if s.Sanguosha.Pending.Kind != "fangzhu" {
		t.Fatal("Fangzhu missing")
	}
	sgDo(t, s, 0, Action{Targets: []int{2}})
	if len(s.Sanguosha.Players[2].Hand) != 2 || !s.Sanguosha.Players[2].Flipped {
		t.Fatal("draw lost HP then flip")
	}
	s.Sanguosha.Players[1].General = "xuhuang"
	sgTop(t, s, func(c SGCard) bool { return c.Suit == 2 })
	s.sgPush(SGEvent{Type: "judge", Actor: 1, Kind: "eight_diagram", Next: &SGEvent{Type: "response", Actor: 2, Target: 1, Kind: "slash", Count: 1}})
	s.sgRun()
	if q := s.Sanguosha.Pending; q.Kind != "songwei" || q.Player != 1 {
		t.Fatal("Songwei should ask judged Wei ally", q)
	}
	sgDo(t, s, 1, Action{Choice: "yes"})
	if len(s.Sanguosha.Players[0].Hand) != 1 {
		t.Fatal("Songwei draw")
	}
	sgPassAll(t, s)
	sgWindConservation(t, s)
}
func TestSanguoshaThicketDuanliangAndWeimu(t *testing.T) {
	s := sgFireState()
	s.Sanguosha.Players[0].General = "xuhuang"
	id := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Kind == "slash" && c.Suit == 0 })
	sgDo(t, s, 0, Action{Type: "sg_play", Skill: "duanliang", Cards: []int{id}, Targets: []int{2}})
	if len(s.Sanguosha.Players[2].Judgment) != 1 {
		t.Fatal("Duanliang distance2")
	}
	sgWindConservation(t, s)
	t.Run("Weimu rejects explicit black trick, excludes area targets", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[1].General = "jiaxu"
		sgGive(t, s, 1, "jink")
		trick := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Kind == "dismantlement" && c.Suit == 0 })
		before, _ := json.Marshal(s)
		if s.Apply(0, Action{Type: "sg_play", Cards: []int{trick}, Targets: []int{1}}) == nil {
			t.Fatal("Weimu explicit target accepted")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("illegal target mutated game")
		}
		aoe := sgGive(t, s, 0, "savage_assault")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{aoe}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 4 || s.Sanguosha.Players[2].HP != 3 {
			t.Fatal("black area prohibition")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaThicketHuoshouJuxiangAndZaiqi(t *testing.T) {
	s := sgFireState()
	s.Sanguosha.Players[1].General = "menghuo"
	s.Sanguosha.Players[2].General = "zhurong"
	s.Sanguosha.Players[3].General = "simayi"
	armor := sgWear(t, s, 1, "eight_diagram")
	aoe := sgGive(t, s, 0, "savage_assault")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{aoe}})
	for s.Sanguosha.Pending != nil && s.Sanguosha.Pending.Kind != "invoke" {
		q := s.Sanguosha.Pending
		sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
		if q.Kind == "invoke" {
			break
		}
	}
	if q := s.Sanguosha.Pending; q == nil || q.Event.Kind != "fankui" || q.Event.Actor != 1 {
		t.Fatal("wrong damage source", q)
	}
	sgDo(t, s, 3, Action{Choice: "yes"})
	sgDo(t, s, 3, Action{Card: armor})
	if s.Sanguosha.Players[1].HP != 4 || s.Sanguosha.Players[2].HP != 4 || s.Sanguosha.Players[3].HP != 3 || !slices.Contains(s.Sanguosha.Players[2].Hand, aoe) {
		t.Fatal("immunity/inheritance")
	}
	sgWindConservation(t, s)
	t.Run("Zaiqi reveals rather than judges", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "menghuo"
		s.Sanguosha.Players[0].HP = 2
		s.Sanguosha.Players[1].General = "zhangjiao"
		sgGive(t, s, 1, "slash")
		black := sgTop(t, s, func(c SGCard) bool { return c.Suit == 2 })
		heart := sgTop(t, s, func(c SGCard) bool { return c.Suit == 1 })
		// Put exactly these two at the top, independent of randomized deck order.
		s.Sanguosha.Deck = sgRemove(sgRemove(s.Sanguosha.Deck, black), heart)
		s.Sanguosha.Deck = append([]int{heart, black}, s.Sanguosha.Deck...)
		s.sgPush(SGEvent{Type: "draw_phase", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Choice: "zaiqi"})
		if s.Sanguosha.Pending != nil || s.Sanguosha.Players[0].HP != 3 || !slices.Equal(s.Sanguosha.Players[0].Hand, []int{black}) || !slices.Contains(s.Sanguosha.Discard, heart) {
			t.Fatal("Zaiqi result")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaThicketLieren(t *testing.T) {
	s := sgFireState()
	s.Sanguosha.Players[0].General = "zhurong"
	slash := sgGive(t, s, 0, "slash")
	mine := sgRankGive(t, s, 0, 13)
	theirs := sgRankGive(t, s, 1, 1)
	equip := sgWear(t, s, 1, "eight_diagram")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{1}})
	sgDo(t, s, 1, Action{Choice: "pass"})
	if q := s.Sanguosha.Pending; q.Kind != "lieren" {
		t.Fatal("no Lieren", q)
	}
	sgDo(t, s, 0, Action{Cards: []int{mine}})
	sgDo(t, s, 1, Action{Cards: []int{theirs}})
	if q := s.Sanguosha.Pending; q.Kind != "steal" || q.Event.Kind != "lieren" {
		t.Fatal("wrong pindian continuation", q)
	}
	sgDo(t, s, 0, Action{Card: equip})
	if !slices.Contains(s.Sanguosha.Players[0].Hand, equip) {
		t.Fatal("Lieren prize")
	}
	sgWindConservation(t, s)
}
func TestSanguoshaThicketHaoshiDimengYinghun(t *testing.T) {
	t.Run("Haoshi forces half to a tied poorest player", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "lusu"
		s.sgDraw(0, 3)
		s.sgDraw(1, 1)
		s.sgPush(SGEvent{Type: "draw_phase", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Choice: "haoshi"})
		q := s.Sanguosha.Pending
		if q.Kind != "haoshi_give" || q.Event.Amount != 3 || !slices.Equal(q.Targets, []int{2, 3}) {
			t.Fatal("wrong half or poorest", q)
		}
		ids := clone(s.Sanguosha.Players[0].Hand[:3])
		if s.Apply(0, Action{Prompt: q.ID, Cards: ids, Targets: []int{1}}) == nil {
			t.Fatal("richer recipient")
		}
		sgDo(t, s, 0, Action{Cards: ids, Targets: []int{2}})
		if len(s.Sanguosha.Players[0].Hand) != 4 || len(s.Sanguosha.Players[2].Hand) != 3 {
			t.Fatal("Haoshi transfer")
		}
		sgWindConservation(t, s)
	})
	t.Run("Dimeng swaps without revealing IDs or false empty trigger", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "lusu"
		s.Sanguosha.Players[1].General = "luxun"
		a := sgGive(t, s, 1, "peach")
		b := sgGive(t, s, 2, "jink")
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "dimeng", Targets: []int{1, 2}})
		if !slices.Equal(s.Sanguosha.Players[1].Hand, []int{b}) || !slices.Equal(s.Sanguosha.Players[2].Hand, []int{a}) || s.Sanguosha.Pending != nil {
			t.Fatal("nonempty swap triggered Lianying")
		}
		for _, viewer := range []int{-1, 0, 3} {
			players := s.View(viewer)["sanguosha"].(map[string]any)["players"].([]map[string]any)
			if _, ok := players[1]["hand"]; ok {
				t.Fatal("swap revealed hand")
			}
		}
		sgWindConservation(t, s)
	})
	t.Run("Yinghun discards equipment and handles insufficient cards", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "sunjian"
		s.Sanguosha.Players[0].HP = 1
		equip := sgWear(t, s, 1, "silver_lion")
		s.Sanguosha.Players[1].HP = 2
		s.sgPush(SGEvent{Type: "thicket_start", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Choice: "draw_one", Targets: []int{1}})
		if q := s.Sanguosha.Pending; q.Kind != "yinghun_discard" || q.Event.Amount != 2 {
			t.Fatal("must discard all available", q)
		}
		ids := append(clone(s.Sanguosha.Players[1].Hand), equip)
		sgDo(t, s, 1, Action{Cards: ids})
		if s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("equipment loss trigger")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaThicketWanshaRoulinBenghuai(t *testing.T) {
	t.Run("Wansha leaves only active and dying rescue windows", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "jiaxu"
		s.Sanguosha.Players[2].General = "pangtong"
		s.Sanguosha.Players[2].HP = 1
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 2, Amount: 1})
		s.sgRun()
		if s.Sanguosha.Pending.Player != 0 {
			t.Fatal("wrong first rescue")
		}
		sgDo(t, s, 0, Action{Choice: "pass"})
		if q := s.Sanguosha.Pending; q.Player != 2 || q.Kind != "niepan" {
			t.Fatal("Wansha skipped dying skill or included third party", q)
		}
		sgDo(t, s, 2, Action{Choice: "yes"})
		if s.Sanguosha.Players[2].HP != 3 {
			t.Fatal("Niepan under Wansha")
		}
		sgWindConservation(t, s)
	})
	t.Run("Roulin requires two jinks in both directions", func(t *testing.T) {
		for _, reverse := range []bool{false, true} {
			s := sgFireState()
			s.Sanguosha.Players[0].General = "dongzhuo"
			s.Sanguosha.Players[1].General = "zhurong"
			actor, target := 0, 1
			if reverse {
				actor, target = 1, 0
				s.Turn = 1
			}
			slash := sgGive(t, s, actor, "slash")
			jink := sgGive(t, s, target, "jink")
			sgDo(t, s, actor, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{target}})
			if s.Sanguosha.Pending.Event.Count != 2 {
				t.Fatal("Roulin count")
			}
			sgDo(t, s, target, Action{Cards: []int{jink}})
			sgDo(t, s, target, Action{Choice: "pass"})
			sgPassAll(t, s)
			if s.Sanguosha.Players[target].HP != 3 {
				t.Fatal("one jink fully blocked")
			}
			sgWindConservation(t, s)
		}
	})
	t.Run("Benghuai maximum HP loss", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "dongzhuo"
		s.Sanguosha.Players[0].HP = 8
		s.Sanguosha.Players[0].MaxHP = 8
		s.sgPush(SGEvent{Type: "benghuai", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Choice: "maxhp"})
		if s.Sanguosha.Players[0].HP != 7 || s.Sanguosha.Players[0].MaxHP != 7 {
			t.Fatal("max HP loss")
		}
		sgWindConservation(t, s)
	})
}

func TestSanguoshaThicketLuanwuSupportAndNearest(t *testing.T) {
	s := sgFireState()
	s.Turn = 1
	s.Sanguosha.Players[1].General = "jiaxu"
	s.Sanguosha.Players[0].General = "liubei"
	s.Sanguosha.Players[2].General = "xuhuang"
	s.Sanguosha.Players[3].General = "zhaoyun"
	slash := sgGive(t, s, 3, "slash")
	sgDo(t, s, 1, Action{Type: "sg_skill", Skill: "luanwu"})
	if s.Sanguosha.Pending.Player != 2 {
		t.Fatal("wrong chaos order")
	}
	sgDo(t, s, 2, Action{Choice: "pass"})
	sgDo(t, s, 3, Action{Choice: "pass"})
	if q := s.Sanguosha.Pending; q.Player != 0 || q.Kind != "luanwu" {
		t.Fatal("lord not prompted", q)
	}
	if s.Apply(0, Action{Prompt: s.Sanguosha.Pending.ID, Choice: "jijiang", Targets: []int{2}}) == nil {
		t.Fatal("non-nearest support attack")
	}
	sgDo(t, s, 0, Action{Choice: "jijiang", Targets: []int{1}})
	sgDo(t, s, 3, Action{Cards: []int{slash}})
	sgPassAll(t, s)
	if s.Sanguosha.Players[1].HP != 3 || s.Sanguosha.Players[2].HP != 3 || s.Sanguosha.Players[3].HP != 3 {
		t.Fatal("chaos attack/loss outcomes")
	}
	if s.Sanguosha.Players[0].HP != 4 {
		t.Fatal("lord lost HP despite support")
	}
	sgWindConservation(t, s)
}
func TestSanguoshaThicketBaonueJudgeActorAndGuhuoWeimu(t *testing.T) {
	t.Run("damage actor judges and lord heals", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "dongzhuo"
		s.Sanguosha.Players[0].HP = 2
		s.Sanguosha.Players[1].General = "jiaxu"
		sgTop(t, s, func(c SGCard) bool { return c.Suit == 0 })
		s.sgPush(SGEvent{Type: "damage", Actor: 1, Target: 2, Amount: 1})
		s.sgRun()
		if q := s.Sanguosha.Pending; q.Kind != "baonue" || q.Player != 1 {
			t.Fatal("Baonue actor", q)
		}
		sgDo(t, s, 1, Action{Choice: "yes"})
		if s.Sanguosha.Players[0].HP != 3 {
			t.Fatal("Baonue recovery")
		}
		sgWindConservation(t, s)
	})
	t.Run("classic bluff trick bypasses Weimu", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "yuji"
		s.Sanguosha.Players[1].General = "jiaxu"
		fake := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 0 && c.Kind == "slash" })
		sgDo(t, s, 0, Action{Type: "sg_play", Skill: "guhuo", Choice: "duel", Cards: []int{fake}, Targets: []int{1}})
		sgAnswerBluff(t, s, -1)
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("NosGuhuo blocked by Weimu")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaThicketBotsAndSaveRestore(t *testing.T) {
	for offset := range 8 {
		s := sgFireState()
		for i := range s.Sanguosha.Players {
			g := sgThicketGenerals[(offset+i)%8]
			p := &s.Sanguosha.Players[i]
			p.General = g.ID
			p.HP = g.HP
			p.MaxHP = g.HP
			s.sgDraw(i, 4)
		}
		s.sgPush(SGEvent{Type: "begin", Actor: 0})
		s.sgRun()
		for step := 0; step < 10000 && !s.Finished; step++ {
			i := s.SanguoshaActor()
			a, err := s.BotAction(i)
			if err != nil {
				t.Fatalf("offset %d step %d pending %+v: %v", offset, step, s.Sanguosha.Pending, err)
			}
			if err = s.Apply(i, a); err != nil {
				t.Fatal(a, err)
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
			t.Fatalf("thicket bots stalled %d", offset)
		}
	}
}

func TestSanguoshaThicketLuanwuGuhuoAndRestore(t *testing.T) {
	s := sgFireState()
	s.Sanguosha.Players[0].General = "jiaxu"
	s.Sanguosha.Players[2].General = "yuji"
	fake := sgGive(t, s, 2, "jink")
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "luanwu"})
	sgDo(t, s, 1, Action{Choice: "pass"})
	sgDo(t, s, 2, Action{Skill: "guhuo", Choice: "slash", Cards: []int{fake}, Targets: []int{1}})
	raw, _ := json.Marshal(s)
	var next State
	if err := json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	s = &next
	sgAnswerBluff(t, s, -1)
	sgPassAll(t, s)
	if s.Sanguosha.Players[1].HP != 2 || s.Sanguosha.Players[2].HP != 4 || s.Sanguosha.Players[3].HP != 3 {
		t.Fatal("chaos bluff continuation")
	}
	sgWindConservation(t, s)
}
func TestSanguoshaThicketEightPlayerInteraction(t *testing.T) {
	s, err := NewSanguosha(8, SGOptions{Deck: "military", Packs: []string{"wind", "fire", "thicket"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sanguosha.generalCatalog()) != 49 {
		t.Fatal("combined catalog")
	}
	for _, id := range []string{"zhangjiao", "yuanshao", "caopi", "dongzhuo"} {
		if !slices.Contains(s.Sanguosha.Players[s.Sanguosha.Lord].Choices, id) {
			t.Fatal("expansion lord missing", id)
		}
	}
	for s.Sanguosha.Selecting {
		i := s.SanguoshaActor()
		a, err := s.BotAction(i)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Apply(i, a); err != nil {
			t.Fatal(err)
		}
	}
	for i := range s.Sanguosha.Players {
		general := sgThicketGenerals[i]
		p := &s.Sanguosha.Players[i]
		p.General = general.ID
		p.MaxHP = general.HP
		if p.Role == "lord" {
			p.MaxHP++
		}
		p.HP = p.MaxHP
	}
	for step := 0; step < 15000 && !s.Finished; step++ {
		i := s.SanguoshaActor()
		a, err := s.BotAction(i)
		if err != nil {
			t.Fatal(step, s.Sanguosha.Pending, err)
		}
		if err = s.Apply(i, a); err != nil {
			t.Fatal(err)
		}
		sgWindConservation(t, s)
		if step%113 == 0 {
			raw, _ := json.Marshal(s)
			var next State
			if err := json.Unmarshal(raw, &next); err != nil {
				t.Fatal(err)
			}
			s = &next
		}
	}
	if !s.Finished {
		t.Fatal("eight-player thicket stalled")
	}
}
