package game

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func sgFireState() *State {
	s := sgWindState()
	s.Sanguosha.Options.Packs = append(s.Sanguosha.Options.Packs, "fire")
	return s
}
func sgRankGive(t *testing.T, s *State, i, rank int) int {
	return sgFindGive(t, s, i, func(c SGCard) bool { return c.Rank == rank })
}
func TestSanguoshaFireSelection(t *testing.T) {
	s, err := NewSanguosha(8, SGOptions{Packs: []string{"fire", "wind", "fire"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sanguosha.generalCatalog()) != 41 || !slices.Contains(s.Sanguosha.Players[s.Sanguosha.Lord].Choices, "yuanshao") {
		t.Fatal("fire catalog or lord")
	}
	if !slices.Equal(s.Sanguosha.Options.Packs, []string{"standard", "wind", "fire"}) {
		t.Fatal(s.Sanguosha.Options)
	}
}
func TestSanguoshaFireQiangxi(t *testing.T) {
	t.Run("equipped weapon range is excluded from payment", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "dianwei"
		weapon := sgWear(t, s, 0, "kylin_bow")
		before, _ := json.Marshal(s)
		if s.Apply(0, Action{Type: "sg_skill", Skill: "qiangxi", Cards: []int{weapon}, Targets: []int{2}}) == nil {
			t.Fatal("counted discarded weapon range")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("invalid range mutated state")
		}
		handWeapon := sgGive(t, s, 0, "axe")
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "qiangxi", Cards: []int{handWeapon}, Targets: []int{2}})
		if s.Sanguosha.Players[2].HP != 3 || s.sgEquip(0, "weapon") != weapon {
			t.Fatal("hand weapon cost")
		}
		if s.Apply(0, Action{Type: "sg_skill", Skill: "qiangxi", Targets: []int{1}}) == nil {
			t.Fatal("repeated skill")
		}
		sgWindConservation(t, s)
	})
	t.Run("HP cost resolves dying before damage and source can die", func(t *testing.T) {
		s := sgFireState()
		s.Turn = 1
		s.Sanguosha.Players[1].General = "dianwei"
		s.Sanguosha.Players[1].HP = 1
		sgDo(t, s, 1, Action{Type: "sg_skill", Skill: "qiangxi", Targets: []int{0}})
		if s.Sanguosha.Pending.Kind != "peach" || s.Sanguosha.Players[0].HP != 4 {
			t.Fatal("damage before cost rescue")
		}
		sgPassAll(t, s)
		if !s.Sanguosha.Players[1].Dead || s.Sanguosha.Players[0].HP != 3 {
			t.Fatal("damage lost when source died")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaFirePindianPrivacyTianyiAndRestore(t *testing.T) {
	for _, win := range []bool{true, false} {
		t.Run(map[bool]string{true: "win", false: "tie loses"}[win], func(t *testing.T) {
			s := sgFireState()
			s.Sanguosha.Players[0].General = "taishici"
			mine := sgRankGive(t, s, 0, 13)
			rank := 12
			if !win {
				rank = 13
			}
			theirs := sgRankGive(t, s, 2, rank)
			sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "tianyi", Cards: []int{mine}, Targets: []int{2}})
			if len(s.Sanguosha.Table) != 0 || !slices.Contains(s.Sanguosha.Players[0].Hand, mine) {
				t.Fatal("early reveal/payment")
			}
			for _, viewer := range []int{-1, 2} {
				v := s.View(viewer)["sanguosha"].(map[string]any)
				q := v["pending"].(map[string]any)
				if ids, ok := q["cards"].([]int); ok && len(ids) > 0 {
					t.Fatal("pindian hidden choice leaked")
				}
			}
			raw, _ := json.Marshal(s)
			var restored State
			if err := json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			s = &restored
			sgDo(t, s, 2, Action{Cards: []int{theirs}})
			if s.sgTianyi(0) != map[bool]int{true: 1, false: -1}[win] {
				t.Fatal("pindian result")
			}
			if win {
				first := sgGive(t, s, 0, "slash")
				sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{first}, Targets: []int{1, 2}})
				sgPassAll(t, s)
				second := sgGive(t, s, 0, "slash")
				sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{second}, Targets: []int{2}})
				sgPassAll(t, s)
				if s.Sanguosha.Players[1].HP != 3 || s.Sanguosha.Players[2].HP != 2 {
					t.Fatal("extra targets/range/count")
				}
				third := sgGive(t, s, 0, "slash")
				if s.Apply(0, Action{Type: "sg_play", Cards: []int{third}, Targets: []int{1}}) == nil {
					t.Fatal("third slash allowed")
				}
			} else {
				slash := sgGive(t, s, 0, "slash")
				sgWear(t, s, 0, "crossbow")
				if s.Apply(0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{1}}) == nil {
					t.Fatal("crossbow ignored Tianyi prohibition")
				}
				// A failed Tianyi forbids use, but a duel may still request a played slash.
				s.sgAsk(0, "card", "", SGEvent{Type: "response", Actor: 1, Target: 0, Kind: "duel", Count: 1})
				sgDo(t, s, 0, Action{Cards: []int{slash}})
				sgPassAll(t, s)
			}
			s.Turn = 1
			if s.sgTianyi(0) != 0 {
				t.Fatal("turn-scoped Tianyi leaked")
			}
			sgWindConservation(t, s)
		})
	}
}
func TestSanguoshaFireQuhuAndJieming(t *testing.T) {
	s := sgFireState()
	s.Sanguosha.Players[0].General = "xunyu"
	s.Sanguosha.Players[0].HP = 3
	s.Sanguosha.Players[0].MaxHP = 3
	mine := sgRankGive(t, s, 0, 13)
	theirs := sgRankGive(t, s, 1, 1)
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "quhu", Cards: []int{mine}, Targets: []int{1}})
	sgDo(t, s, 1, Action{Cards: []int{theirs}})
	if q := s.Sanguosha.Pending; q.Kind != "quhu_target" || !slices.Contains(q.Targets, 2) || slices.Contains(q.Targets, 1) {
		t.Fatal("wrong wolf candidates", q)
	}
	if s.Apply(0, Action{Prompt: s.Sanguosha.Pending.ID, Targets: []int{3}}) == nil {
		t.Fatal("out of range wolf")
	}
	sgDo(t, s, 0, Action{Targets: []int{2}})
	if s.Sanguosha.Players[2].HP != 3 {
		t.Fatal("quhu damage")
	}
	// Per damage point, choose independently, cap refill at maxHP and five.
	s.Sanguosha.Players[3].MaxHP = 7
	s.sgPush(SGEvent{Type: "damage", Actor: 1, Target: 0, Amount: 2})
	s.sgRun()
	if s.Sanguosha.Pending.Kind != "jieming" {
		t.Fatal("jieming missing")
	}
	sgDo(t, s, 0, Action{Targets: []int{3}})
	sgDo(t, s, 0, Action{Targets: []int{0}})
	if len(s.Sanguosha.Players[3].Hand) != 5 || len(s.Sanguosha.Players[0].Hand) != 3 || s.Sanguosha.Pending != nil {
		t.Fatal("jieming refills", s.Sanguosha.Pending)
	}
	sgWindConservation(t, s)
	t.Run("tie deals damage from tiger and invokes jieming", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "xunyu"
		s.Sanguosha.Players[0].HP = 3
		a, b := sgRankGive(t, s, 0, 7), sgRankGive(t, s, 1, 7)
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "quhu", Cards: []int{a}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Cards: []int{b}})
		if s.Sanguosha.Players[0].HP != 2 || s.Sanguosha.Pending.Kind != "jieming" {
			t.Fatal("lost quhu")
		}
		sgDo(t, s, 0, Action{Choice: "pass"})
		sgWindConservation(t, s)
	})
}
func TestSanguoshaFireWolong(t *testing.T) {
	t.Run("red Huoji, black Kanpo, only hand cards", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "wolong"
		red := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 1 && c.Kind == "slash" })
		black := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Kind == "crossbow" && c.Suit == 2 })
		equip := sgWear(t, s, 0, "kylin_bow")
		if kind, err := s.sgAs(0, []int{red}, "huoji", ""); err != nil || kind != "fire_attack" {
			t.Fatal(kind, err)
		}
		if kind, err := s.sgAs(0, []int{black}, "kanpo", "nullification"); err != nil || kind != "nullification" {
			t.Fatal(kind, err)
		}
		if _, err := s.sgAs(0, []int{equip}, "huoji", ""); err == nil {
			t.Fatal("equipped Huoji")
		}
		sgGive(t, s, 1, "jink")
		sgDo(t, s, 0, Action{Type: "sg_play", Skill: "huoji", Cards: []int{red}, Targets: []int{1}})
		sgPassNull(t, s)
		if s.Sanguosha.Pending.Kind != "fire_reveal" {
			t.Fatal("Huoji bypassed reveal")
		}
		sgWindConservation(t, s)
	})
	for _, armor := range []string{"", "silver_lion", "qinggang_sword"} {
		t.Run("bazhen "+armor, func(t *testing.T) {
			s := sgFireState()
			s.Sanguosha.Players[1].General = "wolong"
			if armor == "silver_lion" {
				sgWear(t, s, 1, armor)
			} else if armor != "" {
				sgWear(t, s, 0, armor)
			}
			slash := sgGive(t, s, 0, "slash")
			sgTop(t, s, func(c SGCard) bool { return c.Suit == 1 })
			sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{1}})
			q := s.Sanguosha.Pending
			err := s.Apply(1, Action{Prompt: q.ID, Choice: "eight_diagram"})
			if armor != "" {
				if err == nil {
					t.Fatal("Bazhen with armor/Qinggang")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			sgPassAll(t, s)
			if s.Sanguosha.Players[1].HP != 4 {
				t.Fatal("Bazhen red judgment failed")
			}
			sgWindConservation(t, s)
		})
	}
}
func TestSanguoshaFirePangtong(t *testing.T) {
	s := sgFireState()
	p := &s.Sanguosha.Players[1]
	p.General = "pangtong"
	p.MaxHP = 4
	p.HP = 1
	p.Chained = true
	p.Flipped = true
	sgWear(t, s, 1, "silver_lion")
	id := sgGive(t, s, 1, "indulgence")
	p.Hand = sgRemove(p.Hand, id)
	p.Judgment = append(p.Judgment, SGDelayed{Card: id, Kind: "indulgence"})
	sgGive(t, s, 1, "jink")
	s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 1, Kind: "slash", Amount: 1})
	s.sgRun()
	for s.Sanguosha.Pending.Kind == "peach" && s.Sanguosha.Pending.Player != 1 {
		sgDo(t, s, s.Sanguosha.Pending.Player, Action{Choice: "pass"})
	}
	if s.Sanguosha.Pending.Kind != "niepan" {
		t.Fatal("no limited rescue")
	}
	sgDo(t, s, 1, Action{Choice: "yes"})
	p = &s.Sanguosha.Players[1]
	if p.HP != 3 || len(p.Hand) != 3 || len(p.Equip)+len(p.Judgment) != 0 || p.Flipped || p.Chained || p.Marks["niepan"] != 1 {
		t.Fatal("rebirth", p)
	}
	raw, _ := json.Marshal(s)
	var next State
	json.Unmarshal(raw, &next)
	s = &next
	s.sgPush(SGEvent{Type: "lose_hp", Target: 1, Amount: 3})
	s.sgRun()
	for s.Sanguosha.Pending != nil && s.Sanguosha.Pending.Kind == "peach" {
		sgDo(t, s, s.Sanguosha.Pending.Player, Action{Choice: "pass"})
	}
	if s.Sanguosha.Pending != nil && s.Sanguosha.Pending.Kind == "niepan" {
		t.Fatal("limited skill refreshed after load")
	}
	if !s.Sanguosha.Players[1].Dead {
		t.Fatal("second rescue did not die")
	}
	sgWindConservation(t, s)
	t.Run("club recast even in classic deck", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "pangtong"
		id := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 2 })
		sgDo(t, s, 0, Action{Type: "sg_play", Skill: "lianhuan", Cards: []int{id}})
		if len(s.Sanguosha.Players[0].Hand) != 1 || !slices.Contains(s.Sanguosha.Discard, id) {
			t.Fatal("Lianhuan recast")
		}
	})
}
func TestSanguoshaFireYuanShao(t *testing.T) {
	s := sgFireState()
	s.Sanguosha.Players[0].General = "yuanshao"
	s.Sanguosha.Players[1].General = "pangde"
	s.Sanguosha.Players[2].General = "yuji"
	if s.sgHandLimit(0) != 8 {
		t.Fatal("Xueyi count")
	}
	s.Sanguosha.Players[2].Dead = true
	if s.sgHandLimit(0) != 6 {
		t.Fatal("dead counted")
	}
	s.Sanguosha.Players[2].Dead = false
	a := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 3 })
	b := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 3 })
	c := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 2 })
	if _, err := s.sgAs(0, []int{a, c}, "luanji", ""); err == nil {
		t.Fatal("different suits")
	}
	sgDo(t, s, 0, Action{Type: "sg_play", Skill: "luanji", Cards: []int{a, b}})
	sgPassAll(t, s)
	if s.Sanguosha.Players[1].HP != 3 || s.Sanguosha.Players[2].HP != 3 || s.Sanguosha.Players[3].HP != 3 {
		t.Fatal("Luanji targets")
	}
	sgWindConservation(t, s)
}
func TestSanguoshaFireShuangxiongAndMengjin(t *testing.T) {
	t.Run("judgment replaces draw and enables opposite color duel", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "yanliangwenchou"
		black := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 2 })
		top := sgTop(t, s, func(c SGCard) bool { return c.Suit == 1 })
		s.sgPush(SGEvent{Type: "draw_phase", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Choice: "shuangxiong"})
		if len(s.Sanguosha.Players[0].Hand) != 2 || !slices.Contains(s.Sanguosha.Players[0].Hand, top) || s.Sanguosha.Players[0].Used["shuangxiong"] != 1 {
			t.Fatal("judge card not received")
		}
		if _, err := s.sgAs(0, []int{top}, "shuangxiong", ""); err == nil {
			t.Fatal("same color accepted")
		}
		sgDo(t, s, 0, Action{Type: "sg_play", Skill: "shuangxiong", Cards: []int{black}, Targets: []int{1}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("transformed duel")
		}
		s.Turn = 1
		if _, err := s.sgAs(0, []int{top}, "shuangxiong", ""); err == nil {
			t.Fatal("offturn transform")
		}
		sgWindConservation(t, s)
	})
	t.Run("miss discards equipment but cannot target judgment", func(t *testing.T) {
		s := sgFireState()
		s.Sanguosha.Players[0].General = "pangde"
		slash := sgGive(t, s, 0, "slash")
		jink := sgGive(t, s, 2, "jink")
		armor := sgWear(t, s, 2, "silver_lion")
		s.Sanguosha.Players[2].HP = 2
		delay := sgGive(t, s, 2, "indulgence")
		p := &s.Sanguosha.Players[2]
		p.Hand = sgRemove(p.Hand, delay)
		p.Judgment = append(p.Judgment, SGDelayed{Card: delay, Kind: "indulgence"})
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{2}})
		sgDo(t, s, 2, Action{Cards: []int{jink}})
		if s.Sanguosha.Pending.Kind != "mengjin" {
			t.Fatal("no Mengjin")
		}
		sgDo(t, s, 0, Action{Choice: "yes"})
		if s.Apply(0, Action{Prompt: s.Sanguosha.Pending.ID, Card: delay}) == nil {
			t.Fatal("Mengjin judgment removal")
		}
		sgDo(t, s, 0, Action{Card: armor})
		if s.Sanguosha.Players[2].HP != 3 || !slices.Contains(s.Sanguosha.Discard, armor) {
			t.Fatal("armor loss")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaFireBotsAndSaveRestore(t *testing.T) {
	for offset := range 8 {
		s := sgFireState()
		for i := range s.Sanguosha.Players {
			g := sgFireGenerals[(offset+i)%8]
			p := &s.Sanguosha.Players[i]
			p.General = g.ID
			p.HP = g.HP
			p.MaxHP = g.HP
			s.sgDraw(i, 4)
		}
		s.sgPush(SGEvent{Type: "begin", Actor: 0})
		s.sgRun()
		for step := 0; step < 8000 && !s.Finished; step++ {
			i := s.SanguoshaActor()
			a, err := s.BotAction(i)
			if err != nil {
				t.Fatalf("offset %d step %d pending %+v: %v", offset, step, s.Sanguosha.Pending, err)
			}
			if err = s.Apply(i, a); err != nil {
				t.Fatal(a, err)
			}
			sgWindConservation(t, s)
			if step%101 == 0 {
				raw, _ := json.Marshal(s)
				var next State
				if err := json.Unmarshal(raw, &next); err != nil {
					t.Fatal(err)
				}
				s = &next
			}
		}
		if !s.Finished {
			t.Fatalf("fire bots stalled %d %s", offset, strings.Join(s.Log, "\n"))
		}
	}
}
