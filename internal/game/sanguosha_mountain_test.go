package game

import (
	"encoding/json"
	"slices"
	"testing"
)

func sgMountainState() *State {
	s := sgFireState()
	s.Sanguosha.Options.Packs = append(s.Sanguosha.Options.Packs, "thicket", "mountain")
	return s
}

func TestSanguoshaMountainSelectionAndBots(t *testing.T) {
	s, err := NewSanguosha(8, SGOptions{Deck: "military", Packs: []string{"mountain", "fire", "wind", "thicket", "mountain"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sanguosha.generalCatalog()) != 57 || !slices.Contains(s.Sanguosha.Players[s.Sanguosha.Lord].Choices, "sunce") || !slices.Contains(s.Sanguosha.Players[s.Sanguosha.Lord].Choices, "liushan") {
		t.Fatal("mountain selection")
	}
	for offset := range 10 {
		s := sgMountainState()
		if offset >= 8 {
			s, err = NewSanguosha(8, SGOptions{Deck: "military", Packs: []string{"wind", "fire", "thicket", "mountain"}})
			if err != nil {
				t.Fatal(err)
			}
			g := s.Sanguosha
			g.Pending = nil
			g.Queue = nil
			g.Selecting = false
			g.Selected = 8
			g.Lord = 0
			s.Turn = 0
		}
		for i := range s.Sanguosha.Players {
			general := sgMountainGenerals[(offset+i)%8]
			p := &s.Sanguosha.Players[i]
			p.General = general.ID
			p.HP = general.HP
			p.MaxHP = general.HP
			if offset >= 8 {
				p.Role = []string{"lord", "rebel", "loyalist", "rebel", "renegade", "loyalist", "rebel", "rebel"}[i]
			}
			s.sgDraw(i, 4)
		}
		es := []SGEvent{}
		for _, who := range s.sgOrder(0) {
			es = append(es, SGEvent{Type: "huashen_init", Actor: who})
		}
		es = append(es, SGEvent{Type: "begin", Actor: 0})
		s.sgPush(es...)
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
				sgRestoreMountain(t, s)
			}
			if s.Finished {
				t.Logf("offset %d completed in %d actions", offset, step+1)
			}
		}
		if !s.Finished {
			t.Fatalf("mountain bots stalled offset %d pending %+v", offset, s.Sanguosha.Pending)
		}
	}
}
func sgRestoreMountain(t *testing.T, s *State) {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var out State
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	*s = out
}
func sgField(t *testing.T, s *State, i int) int {
	t.Helper()
	id := sgGive(t, s, i, "slash")
	p := &s.Sanguosha.Players[i]
	p.Hand = sgRemove(p.Hand, id)
	p.Fields = append(p.Fields, id)
	return id
}
func TestSanguoshaMountainAwakening(t *testing.T) {
	t.Run("Zaoxian once and post-payment Jixi range", func(t *testing.T) {
		s := sgMountainState()
		g := s.Sanguosha
		g.Players[0].General = "dengai"
		for range 3 {
			sgField(t, s, 0)
		}
		s.sgPush(SGEvent{Type: "mountain_start", Actor: 0})
		s.sgRun()
		if g.Players[0].MaxHP != 3 || !s.sgHas(0, "jixi") {
			t.Fatal("Zaoxian")
		}
		s.sgPush(SGEvent{Type: "mountain_start", Actor: 0})
		s.sgRun()
		if g.Players[0].MaxHP != 3 {
			t.Fatal("awakened twice")
		}
		g.Discard = append(g.Discard, g.Players[0].Fields[1:]...)
		g.Players[0].Fields = g.Players[0].Fields[:1]
		sgGive(t, s, 2, "jink")
		field := g.Players[0].Fields[0]
		before, _ := json.Marshal(s)
		if s.Apply(0, Action{Type: "sg_skill", Skill: "jixi", Cards: []int{field}, Targets: []int{2}}) == nil {
			t.Fatal("counted spent field in range")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("illegal Jixi changed state")
		}
		sgGive(t, s, 1, "jink")
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "jixi", Cards: []int{field}, Targets: []int{1}})
		for s.Sanguosha.Pending != nil && s.Sanguosha.Pending.Kind == "nullification" {
			sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
		}
		sgDo(t, s, 0, Action{Choice: "hand"})
		if len(s.Sanguosha.Players[0].Fields) != 0 {
			t.Fatal("field not consumed")
		}
		sgWindConservation(t, s)
	})
	t.Run("Zhiji can Guanxing immediately and survives restore", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[0].General = "jiangwei"
		s.Sanguosha.Players[0].HP = 2
		s.sgPush(SGEvent{Type: "begin", Actor: 0})
		s.sgRun()
		if s.Sanguosha.Pending.Kind != "zhiji" {
			t.Fatal(s.Sanguosha.Pending)
		}
		sgRestoreMountain(t, s)
		sgDo(t, s, 0, Action{Choice: "heal"})
		if s.Sanguosha.Players[0].HP != 3 || s.Sanguosha.Pending.Kind != "invoke" || s.Sanguosha.Pending.Event.Kind != "guanxing" {
			t.Fatal("same start Guanxing", s.Sanguosha.Pending)
		}
		sgDo(t, s, 0, Action{Choice: "yes"})
		sgDo(t, s, 0, Action{Cards: clone(s.Sanguosha.Pending.Cards)})
		sgWindConservation(t, s)
	})
	t.Run("Hunzi acquires Yinghun before this start", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[0].General = "sunce"
		s.Sanguosha.Players[0].HP = 1
		s.sgPush(SGEvent{Type: "begin", Actor: 0})
		s.sgRun()
		if s.Sanguosha.Players[0].MaxHP != 3 || s.Sanguosha.Pending.Kind != "yinghun" || !s.sgHas(0, "jie_yingzi") {
			t.Fatal("Hunzi timing", s.Sanguosha.Pending)
		}
	})
	t.Run("Ruoyu ties qualify but nonlord cannot awaken", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[0].General = "liushan"
		s.Sanguosha.Players[0].HP = 3
		s.Sanguosha.Players[0].MaxHP = 3
		s.Sanguosha.Players[1].General = "liushan"
		s.Sanguosha.Players[1].HP = 3
		s.Sanguosha.Players[1].MaxHP = 3
		s.sgPush(SGEvent{Type: "mountain_start", Actor: 0}, SGEvent{Type: "mountain_start", Actor: 1})
		s.sgRun()
		if !s.sgHas(0, "jijiang") || s.sgHas(1, "jijiang") || s.Sanguosha.Players[0].HP != 4 || s.Sanguosha.Players[1].MaxHP != 3 {
			t.Fatal("Ruoyu lord/tie")
		}
	})
}
func TestSanguoshaMountainTuntian(t *testing.T) {
	s := sgMountainState()
	s.Sanguosha.Players[1].General = "dengai"
	a := sgGive(t, s, 1, "jink")
	b := sgWear(t, s, 1, "crossbow")
	field := sgTop(t, s, func(c SGCard) bool { return c.Suit == 2 })
	s.sgDiscard(1, []int{a, b})
	s.sgRun()
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "invoke" || q.Event.Kind != "tuntian" {
		t.Fatal("batch loss trigger", q)
	}
	sgRestoreMountain(t, s)
	sgDo(t, s, 1, Action{Choice: "yes"})
	if len(s.Sanguosha.Players[1].Fields) != 1 || s.Sanguosha.Players[1].Fields[0] != field || s.Sanguosha.Pending != nil {
		t.Fatal("one judgment per batch")
	}
	if s.sgDistance(1, 3) != 1 {
		t.Fatal("field distance")
	}
	c := sgGive(t, s, 1, "jink")
	sgTop(t, s, func(c SGCard) bool { return c.Suit == 1 })
	s.sgDiscard(1, []int{c})
	s.sgRun()
	sgDo(t, s, 1, Action{Choice: "yes"})
	if len(s.Sanguosha.Players[1].Fields) != 1 {
		t.Fatal("heart became field")
	}
	s.Turn = 1
	c = sgGive(t, s, 1, "jink")
	s.sgDiscard(1, []int{c})
	s.sgRun()
	if s.Sanguosha.Pending != nil {
		t.Fatal("own turn Tuntian")
	}
	sgWindConservation(t, s)
	t.Run("Dimeng atomic swap still triggers loss", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[1].General = "dengai"
		sgGive(t, s, 1, "jink")
		sgGive(t, s, 2, "slash")
		s.sgPush(SGEvent{Type: "dimeng_swap", Actor: 0, Targets: []int{1, 2}})
		s.sgRun()
		if s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Event.Kind != "tuntian" {
			t.Fatal("swap lost hook")
		}
		sgPassAll(t, s)
		sgWindConservation(t, s)
	})
}
func TestSanguoshaMountainQiaobian(t *testing.T) {
	t.Run("skip draw and privately obtain two hands", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[0].General = "zhanghe"
		cost := sgGive(t, s, 0, "jink")
		a := sgGive(t, s, 1, "slash")
		b := sgGive(t, s, 2, "peach")
		s.sgPush(SGEvent{Type: "qiaobian", Actor: 0, Kind: "draw"}, SGEvent{Type: "draw_phase", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Cards: []int{cost}})
		if s.Sanguosha.Pending.Kind != "qiaobian_draw" {
			t.Fatal(s.Sanguosha.Pending)
		}
		sgRestoreMountain(t, s)
		sgDo(t, s, 0, Action{Targets: []int{1, 2}})
		if !sgSameIDs(s.Sanguosha.Players[0].Hand, []int{a, b}) {
			t.Fatal("draw not skipped")
		}
		sgWindConservation(t, s)
	})
	t.Run("already skipped draw has no bonus", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[0].General = "zhanghe"
		cost := sgGive(t, s, 0, "jink")
		sgGive(t, s, 1, "slash")
		s.Sanguosha.SkipDraw = true
		s.sgPush(SGEvent{Type: "qiaobian", Actor: 0, Kind: "draw"})
		s.sgRun()
		sgDo(t, s, 0, Action{Cards: []int{cost}})
		if s.Sanguosha.Pending != nil {
			t.Fatal("bonus from already skipped phase")
		}
		sgWindConservation(t, s)
	})
	t.Run("equipment loss and duplicate delay legality", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[0].General = "zhanghe"
		s.Sanguosha.Players[1].General = "sunshangxiang"
		cost := sgGive(t, s, 0, "jink")
		equip := sgWear(t, s, 1, "crossbow")
		s.sgPush(SGEvent{Type: "qiaobian", Actor: 0, Kind: "play"})
		s.sgRun()
		sgDo(t, s, 0, Action{Cards: []int{cost}})
		sgDo(t, s, 0, Action{Card: equip, Targets: []int{1, 2}})
		if s.sgEquip(2, "weapon") != equip || s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Event.Kind != "xiaoji" {
			t.Fatal("move equipment hook")
		}
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgWindConservation(t, s)
		for _, who := range []int{1, 2} {
			id := sgGive(t, s, who, "indulgence")
			p := &s.Sanguosha.Players[who]
			p.Hand = sgRemove(p.Hand, id)
			p.Judgment = append(p.Judgment, SGDelayed{Card: id, Kind: "indulgence"})
		}
		for _, move := range s.sgQiaobianMoves(0) {
			if move.Targets[0] == 1 && move.Targets[1] == 2 && sgCard(move.Card).Kind == "indulgence" {
				t.Fatal("duplicate judgment move")
			}
		}
	})
}
func TestSanguoshaMountainXiangleJiang(t *testing.T) {
	t.Run("Lijian duel triggers both participants Jiang", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[0].General = "diaochan"
		s.Sanguosha.Players[1].General = "sunce"
		s.Sanguosha.Players[2].General = "zuoci"
		s.Sanguosha.Players[2].Avatar = "sunce"
		s.Sanguosha.Players[2].AvatarSkill = "jiang"
		cost := sgGive(t, s, 0, "jink")
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "lijian", Cards: []int{cost}, Targets: []int{1, 2}})
		if q := s.Sanguosha.Pending; q.Player != 2 || q.Event.Kind != "jiang" {
			t.Fatal("duel source Jiang", q)
		}
		sgDo(t, s, 2, Action{Choice: "yes"})
		if q := s.Sanguosha.Pending; q.Player != 1 || q.Event.Kind != "jiang" {
			t.Fatal("duel target Jiang", q)
		}
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgPassAll(t, s)
		sgWindConservation(t, s)
	})
	t.Run("Xiangle extra basic cost per target", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[1].General = "liushan"
		kill := sgGive(t, s, 0, "slash")
		equip := sgGive(t, s, 0, "crossbow")
		cost := sgGive(t, s, 0, "jink")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{kill}, Targets: []int{1}})
		if s.Sanguosha.Pending.Kind != "xiangle" {
			t.Fatal("Xiangle missing")
		}
		if s.Apply(0, Action{Prompt: s.Sanguosha.Pending.ID, Cards: []int{equip}}) == nil {
			t.Fatal("equipment accepted as basic")
		}
		sgDo(t, s, 0, Action{Cards: []int{cost}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("paid slash failed")
		}
		kill = sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{kill}, Targets: []int{1}})
		sgDo(t, s, 0, Action{Choice: "pass"})
		if s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("unpaid slash hit")
		}
		sgWindConservation(t, s)
	})
	t.Run("Jiang one source trigger for multiple targets", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[0].General = "sunce"
		s.sgAcquire(0, "paoxiao")
		sgWear(t, s, 0, "halberd")
		kill := sgFindGive(t, s, 0, func(c SGCard) bool { return sgIsSlash(c.Kind) && c.Suit == 1 })
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{kill}, Targets: []int{1, 2}})
		if s.Sanguosha.Pending.Event.Kind != "jiang" {
			t.Fatal("source Jiang missing")
		}
		sgDo(t, s, 0, Action{Choice: "yes"})
		sgPassAll(t, s)
		if len(s.Sanguosha.Players[0].Hand) != 1 {
			t.Fatal("Jiang counted targets")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaMountainFangquanResume(t *testing.T) {
	s := sgMountainState()
	s.Sanguosha.Players[0].General = "liushan"
	cost := sgGive(t, s, 0, "jink")
	s.sgPush(SGEvent{Type: "fangquan", Actor: 0}, SGEvent{Type: "play_phase", Actor: 0})
	s.sgRun()
	sgDo(t, s, 0, Action{Choice: "yes"})
	if s.Sanguosha.Pending.Kind != "fangquan_give" {
		t.Fatal("finish prompt", s.Sanguosha.Pending)
	}
	sgDo(t, s, 0, Action{Cards: []int{cost}, Targets: []int{2}})
	if s.Turn != 2 || len(s.Sanguosha.ResumeTurns) != 1 {
		t.Fatal("extra turn")
	}
	sgRestoreMountain(t, s)
	// A nested extra turn must still return after the original normal seat.
	s.Sanguosha.ExtraTurns = []int{3}
	s.sgPush(SGEvent{Type: "next", Actor: 2})
	s.sgRun()
	if s.Turn != 3 {
		t.Fatal("nested extra")
	}
	sgRestoreMountain(t, s)
	s.sgPush(SGEvent{Type: "next", Actor: 3})
	s.sgRun()
	if s.Turn != 1 || len(s.Sanguosha.ResumeTurns) != 0 {
		t.Fatal("normal seat not resumed", s.Turn, s.Sanguosha.ResumeTurns)
	}
	sgWindConservation(t, s)
}
func TestSanguoshaMountainTiaoxin(t *testing.T) {
	t.Run("forced slash supplied through Jijiang", func(t *testing.T) {
		s := sgMountainState()
		s.Turn = 1
		s.Sanguosha.Players[1].General = "jiangwei"
		s.Sanguosha.Players[0].General = "liubei"
		s.Sanguosha.Players[2].General = "zhaoyun"
		kill := sgGive(t, s, 2, "slash")
		sgDo(t, s, 1, Action{Type: "sg_skill", Skill: "tiaoxin", Targets: []int{0}})
		sgDo(t, s, 0, Action{Choice: "jijiang"})
		sgDo(t, s, 1, Action{Choice: "pass"})
		sgDo(t, s, 2, Action{Cards: []int{kill}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("Jijiang forced use lost")
		}
		sgWindConservation(t, s)
	})
	s := sgMountainState()
	s.Sanguosha.Players[0].General = "jiangwei"
	id := sgWear(t, s, 1, "crossbow")
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "tiaoxin", Targets: []int{1}})
	sgDo(t, s, 1, Action{Choice: "pass"})
	sgDo(t, s, 0, Action{Card: id})
	if !slices.Contains(s.Sanguosha.Discard, id) {
		t.Fatal("Tiaoxin obtained instead of discarded")
	}
	if s.Apply(0, Action{Type: "sg_skill", Skill: "tiaoxin", Targets: []int{1}}) == nil {
		t.Fatal("Tiaoxin repeated")
	}
	sgWindConservation(t, s)
	t.Run("forced Slash with Guhuo survives restoration", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[0].General = "jiangwei"
		s.Sanguosha.Players[1].General = "yuji"
		id := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Kind == "slash" && c.Suit == 1 })
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "tiaoxin", Targets: []int{1}})
		sgDo(t, s, 1, Action{Skill: "guhuo", Choice: "slash", Cards: []int{id}})
		sgRestoreMountain(t, s)
		sgPassAll(t, s)
		if s.Sanguosha.Players[0].HP != 3 || len(s.Sanguosha.Bluffs) != 0 {
			t.Fatal("Tiaoxin Guhuo continuation")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaMountainZhiba(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "tie lord obtains", true: "awakened refusal"}[refuse], func(t *testing.T) {
			s := sgMountainState()
			g := s.Sanguosha
			g.Players[0].General = "sunce"
			g.Players[1].General = "zhouyu"
			s.Turn = 1
			a := sgRankGive(t, s, 1, 7)
			b := sgRankGive(t, s, 0, 7)
			if refuse {
				s.sgMark(0, "hunzi")
			}
			sgDo(t, s, 1, Action{Type: "sg_skill", Skill: "zhiba_pindian", Cards: []int{a}})
			sgRestoreMountain(t, s)
			if refuse {
				sgDo(t, s, 0, Action{Choice: "pass"})
				if !slices.Contains(s.Sanguosha.Players[1].Hand, a) {
					t.Fatal("rejected card lost")
				}
			} else {
				sgDo(t, s, 0, Action{Cards: []int{b}})
				if s.Sanguosha.Pending.Kind != "zhiba_obtain" {
					t.Fatal(s.Sanguosha.Pending)
				}
				sgDo(t, s, 0, Action{Choice: "yes"})
				if !sgSameIDs(s.Sanguosha.Players[0].Hand, []int{a, b}) {
					t.Fatal("tie does not reward lord")
				}
			}
			if s.Sanguosha.Players[1].Used["zhiba_pindian"] != 1 {
				t.Fatal("per turn mark")
			}
			sgWindConservation(t, s)
		})
	}
}
func TestSanguoshaMountainGuzhengAndZhijian(t *testing.T) {
	t.Run("response cards are not discards for Guzheng", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[1].General = "erzhang"
		s.Sanguosha.DiscardPhase = true
		returned := sgGive(t, s, 0, "slash")
		response := sgGive(t, s, 0, "jink")
		s.sgDiscard(0, []int{returned})
		s.sgPush(SGEvent{Type: "response", Actor: 2, Target: 0, Kind: "slash", Count: 1}, SGEvent{Type: "guzheng", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Cards: []int{response}})
		if q := s.Sanguosha.Pending; q.Kind != "guzheng" || !slices.Equal(q.Cards, []int{returned}) {
			t.Fatal("response counted as discarded hand", q)
		}
		sgDo(t, s, 1, Action{Card: returned})
		if !slices.Contains(s.Sanguosha.Discard, response) || len(s.Sanguosha.Players[1].Hand) != 0 {
			t.Fatal("Guzheng reclaimed Jink response")
		}
		sgWindConservation(t, s)
	})
	s := sgMountainState()
	s.Sanguosha.Players[0].General = "erzhang"
	equip := sgGive(t, s, 0, "crossbow")
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "zhijian", Cards: []int{equip}, Targets: []int{1}})
	if s.sgEquip(1, "weapon") != equip || len(s.Sanguosha.Players[0].Hand) != 1 {
		t.Fatal("Zhijian")
	}
	s.Turn = 1
	s.Sanguosha.DiscardPhase = true
	a := sgGive(t, s, 1, "jink")
	b := sgGive(t, s, 1, "slash")
	c := sgGive(t, s, 2, "peach")
	s.sgDiscard(1, []int{a})
	s.sgDiscard(2, []int{c})
	s.sgDiscard(1, []int{b})
	s.sgPush(SGEvent{Type: "guzheng", Actor: 1})
	s.sgRun()
	if q := s.Sanguosha.Pending; q.Kind != "guzheng" || !sgSameIDs(q.Cards, []int{a, b}) {
		t.Fatal("whole discard phase tracking", q)
	}
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Card: a})
	if !slices.Contains(s.Sanguosha.Players[1].Hand, a) || !slices.Contains(s.Sanguosha.Players[0].Hand, b) || !slices.Contains(s.Sanguosha.Players[0].Hand, c) {
		t.Fatal("Guzheng other discard cards")
	}
	sgWindConservation(t, s)
}
func TestSanguoshaMountainBeigeDuanchang(t *testing.T) {
	for suit := 0; suit < 4; suit++ {
		t.Run([]string{"flip", "heal", "discard", "draw"}[suit], func(t *testing.T) {
			s := sgMountainState()
			s.Sanguosha.Players[2].General = "caiwenji"
			cost := sgGive(t, s, 2, "jink")
			sgGive(t, s, 0, "slash")
			sgWear(t, s, 0, "crossbow")
			sgTop(t, s, func(c SGCard) bool { return c.Suit == suit })
			s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 1, Kind: "slash", Amount: 1})
			s.sgRun()
			if s.Sanguosha.Pending.Kind != "beige" {
				t.Fatal("Beige missing")
			}
			sgDo(t, s, 2, Action{Cards: []int{cost}})
			sgPassAll(t, s)
			switch suit {
			case 0:
				if !s.Sanguosha.Players[0].Flipped {
					t.Fatal("spade")
				}
			case 1:
				if s.Sanguosha.Players[1].HP != 4 {
					t.Fatal("heart")
				}
			case 2:
				if len(s.Sanguosha.Players[0].Hand)+len(s.Sanguosha.Players[0].Equip) != 0 {
					t.Fatal("club")
				}
			case 3:
				if len(s.Sanguosha.Players[1].Hand) != 2 {
					t.Fatal("diamond")
				}
			}
			sgWindConservation(t, s)
		})
	}
	t.Run("death removes inherited skills and avatar identity", func(t *testing.T) {
		s := sgMountainState()
		s.Sanguosha.Players[0].General = "zuoci"
		s.Sanguosha.Players[0].Avatars = []string{"daqiao"}
		s.Sanguosha.Players[0].Avatar = "daqiao"
		s.Sanguosha.Players[0].AvatarSkill = "qianxun"
		s.sgAcquire(0, "jijiang")
		s.Sanguosha.Players[1].General = "caiwenji"
		s.Sanguosha.Players[1].HP = 1
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 1, Amount: 1})
		s.sgRun()
		sgPassAll(t, s)
		if !s.Sanguosha.Players[0].SkillsLost || s.sgHas(0, "jijiang") || s.sgHas(0, "huashen") || s.sgKingdom(0) != "qun" || s.sgFemale(0) || len(s.Sanguosha.Players[0].Avatars) > 0 {
			t.Fatal("Duanchang reset")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaMountainHuashenPrivacy(t *testing.T) {
	s := sgMountainState()
	s.Sanguosha.Players[0].General = "zuoci"
	s.sgPush(SGEvent{Type: "huashen_init", Actor: 0})
	s.sgRun()
	if len(s.Sanguosha.Players[0].Avatars) != 2 {
		t.Fatal("initial avatars")
	}
	for _, id := range s.Sanguosha.Players[0].Avatars {
		if id == "zuoci" || id == "zhangfei" {
			t.Fatal("avatar included living general")
		}
	}
	for _, viewer := range []int{-1, 1} {
		view := s.sgView(viewer)["sanguosha"].(map[string]any)
		v := view["players"].([]map[string]any)[0]
		if _, ok := v["avatars"]; ok {
			t.Fatal("avatar candidates leaked")
		}
		if _, ok := view["pending"].(map[string]any)["choices"]; ok {
			t.Fatal("avatar prompt leaked")
		}
	}
	s.Sanguosha.Players[0].Avatars = []string{"xiaoqiao", "yuji", "sunce"}
	s.Sanguosha.Pending.Choices = clone(s.Sanguosha.Players[0].Avatars)
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Choice: "xiaoqiao", Skill: "hongyan"})
	if s.sgKingdom(0) != "wu" || !s.sgFemale(0) || !s.sgHas(0, "hongyan") {
		t.Fatal("effective identity")
	}
	s.sgPush(SGEvent{Type: "huashen_select", Actor: 0})
	s.sgRun()
	if s.Apply(0, Action{Prompt: s.Sanguosha.Pending.ID, Choice: "sunce", Skill: "hunzi"}) == nil {
		t.Fatal("awakening copied")
	}
	sgDo(t, s, 0, Action{Choice: "yuji", Skill: "guhuo"})
	if !s.sgHas(0, "guhuo") || s.sgHas(0, "hongyan") || s.sgFemale(0) {
		t.Fatal("Huashen switch or Guhuo interception")
	}
	s.sgPush(SGEvent{Type: "damage", Actor: 1, Target: 0, Amount: 2})
	s.sgRun()
	if s.Sanguosha.Pending.Event.Kind != "xinsheng" {
		t.Fatal("Xinsheng missing")
	}
	sgDo(t, s, 0, Action{Choice: "yes"})
	if len(s.Sanguosha.Players[0].Avatars) != 5 {
		t.Fatal("damage point count")
	}
	sgWindConservation(t, s)
}
