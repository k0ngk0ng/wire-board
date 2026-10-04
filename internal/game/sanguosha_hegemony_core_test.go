package game

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func sgHegState(n int) *State {
	s := &State{Kind: "sanguosha", Round: 1}
	s.initSGHegemony(n)
	g := s.Sanguosha
	g.Selecting, g.Selected, g.Pending = false, n, nil
	g.InPlay, g.ActivePhase, g.TurnSequence = true, "play", 1
	s.Turn, s.Phase = 0, "sg_play"
	pairs := [][2]string{
		{"heg_caocao", "heg_xuchu"}, {"heg_zhouyu", "heg_xiaoqiao"},
		{"heg_liubei", "heg_zhangfei"}, {"heg_lvbu", "heg_diaochan"},
		{"heg_guojia", "heg_zhenji"}, {"heg_sunquan", "heg_zhoutai"},
		{"heg_zhaoyun", "heg_liushan"}, {"heg_yuanshao", "heg_yanliangwenchou"},
	}
	for i := range g.Players {
		sgHegSetPair(s, i, pairs[i])
	}
	return s
}

func sgHegSetPair(s *State, i int, pair [2]string) {
	p := &s.Sanguosha.Players[i]
	p.General, p.Hegemony.Deputy = pair[0], pair[1]
	p.MaxHP = (sgGeneral(pair[0]).HP + sgGeneral(pair[1]).HP) / 2
	p.HP, p.Choices = p.MaxHP, nil
}

func TestSanguoshaHegemonyCatalogAndSelection(t *testing.T) {
	if len(sgHegemonyGenerals) != 60 || len(sgHegemonyCards) != 108 || len(sgHegemonyCompanions) != 19 {
		t.Fatal("base roster/deck mismatch")
	}
	kingdoms := map[string]int{}
	for _, general := range sgHegemonyGenerals {
		kingdoms[general.Kingdom]++
		if sgGeneral(general.ID).ID == "" || general.HP < 3 {
			t.Fatal("invalid general", general)
		}
		for _, skill := range general.Skills {
			if sgLordSkill(skill) {
				t.Fatal("identity lord skill in national-war roster", general.ID, skill)
			}
		}
	}
	for _, k := range []string{"wei", "shu", "wu", "qun"} {
		if kingdoms[k] != 15 {
			t.Fatal("unbalanced roster", kingdoms)
		}
	}
	types := map[string]int{}
	for _, c := range sgHegemonyCards {
		if sgCard(c.ID) != c || c.ID <= 160 || c.Rank < 1 || c.Rank > 13 {
			t.Fatal("physical card identity changed", c)
		}
		types[c.Kind]++
	}
	for kind, want := range map[string]int{"slash": 21, "fire_slash": 3, "thunder_slash": 5, "jink": 14, "peach": 8, "analeptic": 3, "heg_nullification": 2, "await_exhausted": 2, "known_both": 2, "befriend_attacking": 1, "six_swords": 1, "triblade": 1} {
		if types[kind] != want {
			t.Fatal(kind, types[kind], want)
		}
	}
	// This internal foundation must not expose a partially implemented mode.
	if _, err := NormalizeSGOptions(SGOptions{Mode: "hegemony"}); err == nil {
		t.Fatal("unfinished national-war mode enabled")
	}
	for n := 4; n <= 8; n++ {
		s := &State{Kind: "sanguosha", Round: 1}
		s.initSGHegemony(n)
		seen := map[string]bool{}
		for _, p := range s.Sanguosha.Players {
			if len(p.Choices) != 7 || p.Role != "" {
				t.Fatal("wrong initial candidates or identity role")
			}
			for _, id := range p.Choices {
				if seen[id] {
					t.Fatal("shared private candidate", id)
				}
				seen[id] = true
			}
		}
		first := s.Sanguosha.Hegemony.First
		for count := 0; count < n; count++ {
			if s.Phase != "sg_select" {
				t.Fatal("general selection used ordinary response timing")
			}
			i := s.SanguoshaActor()
			p := s.Sanguosha.Players[i]
			sgGodIllegal(t, s, i, Action{Choice: p.Choices[0] + "+" + p.Choices[0]})
			pair := ""
			for _, a := range p.Choices {
				for _, b := range p.Choices {
					if a != b && sgGeneral(a).Kingdom == sgGeneral(b).Kingdom {
						pair = a + "+" + b
					}
				}
			}
			if pair == "" {
				t.Fatal("no legal same-kingdom pair")
			}
			sgDo(t, s, i, Action{Choice: pair})
			sgRestoreMountain(t, s)
		}
		if s.Sanguosha.Selecting || s.Turn != first {
			t.Fatal("selection didn't begin random first seat")
		}
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_reveal_turn" || q.Player != first {
			t.Fatal("first seat must be able to reveal before start skills and draw", q)
		}
		sgDo(t, s, first, Action{Choice: "pass"})
		sgPassAll(t, s) // Hidden optional start/draw skills may still be offered.
		for i, p := range s.Sanguosha.Players {
			want := 4
			if i == first {
				want = 6 // first draw phase, while all generals remain hidden
			}
			if len(p.Hand) != want || p.HP != p.MaxHP {
				t.Fatal("initial deal or average HP", i, p)
			}
		}
	}
}

func TestSanguoshaHegemonySelectionTimeoutAndPrivateBot(t *testing.T) {
	s := &State{Kind: "sanguosha", Round: 1}
	s.initSGHegemony(8)
	for s.Sanguosha.Selecting {
		i := s.SanguoshaActor()
		before := clone(*s)
		a, err := s.SanguoshaTimeoutAction()
		if err != nil {
			t.Fatal("selection timeout cannot choose a legal pair", err)
		}
		// Alter only another player's secrets. The current bot's choice must
		// remain identical; it can inspect its own seven candidates only.
		for j := range s.Sanguosha.Players {
			if j != i {
				s.Sanguosha.Players[j].Choices = []string{"heg_liubei", "heg_zhangfei"}
			}
		}
		b, err := s.SanguoshaTimeoutAction()
		if err != nil || a.Choice != b.Choice {
			t.Fatal("bot used another seat's private candidates", a, b, err)
		}
		*s = before
		if err := s.Apply(i, a); err != nil {
			t.Fatal(err)
		}
		sgHegConservation(t, s)
		sgRestoreMountain(t, s)
	}
}

func TestSanguoshaHegemonyHiddenViewAndSkills(t *testing.T) {
	s := sgHegState(4)
	p := &s.Sanguosha.Players[1]
	id := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 0 })
	if s.sgHas(1, "hongyan") || s.sgKingdom(1) != "" || s.sgFemale(1) || strings.Contains(s.sgName(1), "小乔") {
		t.Fatal("hidden general changes public rules")
	}
	for _, viewer := range []int{-1, 0, 1, 2, 3} {
		v := s.sgView(viewer)["sanguosha"].(map[string]any)
		seat := v["players"].([]map[string]any)[1]
		_, ownSkills := seat["ownSkills"]
		_, hand := seat["hand"]
		if ownSkills != (viewer == 1) || hand != (viewer == 1) || seat["deputy"] != "" && viewer != 1 || seat["general"] != "" && viewer != 1 {
			t.Fatal("hidden generals/hand leaked", viewer, seat)
		}
		if _, ok := seat["role"]; ok || v["lord"] != -1 {
			t.Fatal("identity or hidden faction leaked", viewer)
		}
	}
	s.sgHegShow(1, []int{1}, false)
	if !s.sgHas(1, "hongyan") || s.sgHas(1, "yingzi") || !s.sgFemale(1) || s.sgKingdom(1) != "wu" {
		t.Fatal("deputy reveal did not activate only its skills")
	}
	for _, viewer := range []int{-1, 0, 1} {
		cards := s.sgVisibleCatalog(viewer)
		c := cards[slices.IndexFunc(cards, func(c SGCard) bool { return c.ID == id })]
		want := 0
		if viewer == 1 {
			want = 1
		}
		if c.Suit != want {
			t.Fatal("sparse-ID filtered card visibility", viewer, c)
		}
	}
	s.sgPlaceTable(1, []int{id})
	s.sgLose(1, []int{id})
	cards := s.sgVisibleCatalog(-1)
	if cards[slices.IndexFunc(cards, func(c SGCard) bool { return c.ID == id })].Suit != 1 {
		t.Fatal("processing card suit not visible for sparse IDs")
	}
	s.sgHegShow(1, []int{0}, false)
	if !s.sgHas(1, "yingzi") || s.sgFemale(1) {
		t.Fatal("main general did not determine gender after reveal")
	}
	p.Hegemony.Lost[1] = true
	if s.sgHas(1, "hongyan") || !s.sgHas(1, "yingzi") {
		t.Fatal("slot-specific skill loss affected other general")
	}
	sgRestoreMountain(t, s)
}

func TestSanguoshaHegemonyRevealRewards(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_liubei", "heg_ganfuren"})
	s.Sanguosha.Players[0].HP = 2
	s.sgHegShow(0, []int{1}, true)
	s.sgRun()
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_reward" || q.Event.Kind != "first" {
		t.Fatal("first reveal reward", q)
	}
	sgGodIllegal(t, s, 0, Action{Choice: "heal"})
	sgDo(t, s, 0, Action{Choice: "draw"})
	if len(s.Sanguosha.Players[0].Hand) != 2 {
		t.Fatal("first reveal amount")
	}
	s.sgHegShow(0, []int{0}, true)
	s.sgRun()
	sgRestoreMountain(t, s)
	if q := s.Sanguosha.Pending; q == nil || q.Event.Kind != "companion" {
		t.Fatal("companion reward", q)
	}
	sgDo(t, s, 0, Action{Choice: "heal"})
	if s.Sanguosha.Players[0].HP != 3 || s.Sanguosha.Pending.Event.Kind != "half" {
		t.Fatal("companion followed by half-HP reward")
	}
	sgDo(t, s, 0, Action{Choice: "draw"})
	if len(s.Sanguosha.Players[0].Hand) != 3 {
		t.Fatal("half HP draw")
	}
	s.sgHegShow(0, []int{0, 1}, true)
	s.sgRun()
	if s.Sanguosha.Pending != nil {
		t.Fatal("repeated reward")
	}
}

func TestSanguoshaHegemonyCareeristsAndSharedVictory(t *testing.T) {
	t.Run("dead members count toward reveal threshold", func(t *testing.T) {
		s := sgHegState(4)
		for i, pair := range [][2]string{{"heg_caocao", "heg_xuchu"}, {"heg_guojia", "heg_zhenji"}, {"heg_simayi", "heg_xiahoudun"}, {"heg_zhangliao", "heg_xiahouyuan"}} {
			sgHegSetPair(s, i, pair)
			s.sgHegShow(i, []int{0}, false)
			if i == 0 {
				s.Sanguosha.Players[i].Dead = true
			}
		}
		if s.Sanguosha.Players[2].Role != "careerist" || s.Sanguosha.Players[3].Role != "careerist" || s.sgHegFriend(2, 3) || s.sgHegFriend(1, 2) || !s.sgHegFriend(0, 1) {
			t.Fatal("careerist factions")
		}
		if s.sgHegCheckVictory() {
			t.Fatal("different careerists ended the game")
		}
	})
	t.Run("dead ally also wins, hidden survivors safely reveal", func(t *testing.T) {
		s := sgHegState(6)
		sgHegSetPair(s, 1, [2]string{"heg_simayi", "heg_xiahoudun"})
		s.sgHegShow(0, []int{0}, false)
		s.sgHegShow(1, []int{0}, false)
		for _, i := range []int{0, 2, 3, 5} {
			s.Sanguosha.Players[i].Dead = true
		}
		if !s.sgHegCheckVictory() || !slices.Equal(s.Winners, []int{0, 1, 4}) || s.Sanguosha.Players[4].Hegemony.Shown != [2]bool{true, true} {
			t.Fatal("shared winning faction", s.Winners)
		}
	})
	t.Run("potential careerist blocks automatic reveal and victory", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 1, [2]string{"heg_guojia", "heg_zhenji"})
		sgHegSetPair(s, 2, [2]string{"heg_simayi", "heg_xiahoudun"})
		s.sgHegShow(0, []int{0}, false)
		s.Sanguosha.Players[0].Dead = true
		s.Sanguosha.Players[3].Dead = true
		s.sgHegShow(1, []int{0}, false)
		before, _ := json.Marshal(s)
		if s.sgHegCheckVictory() {
			t.Fatal("revealed would-be careerist to force game over")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("victory probe leaked hidden general")
		}
	})
	t.Run("last hidden survivor reveals without reward", func(t *testing.T) {
		s := sgHegState(4)
		for i := 1; i < 4; i++ {
			s.Sanguosha.Players[i].Dead = true
		}
		if !s.sgHegCheckVictory() || !slices.Equal(s.Winners, []int{0}) || len(s.Sanguosha.Queue) != 0 {
			t.Fatal("last survivor", s.Winners)
		}
	})
}

func TestSanguoshaHegemonyKillRewards(t *testing.T) {
	for _, hiddenKiller := range []bool{false, true} {
		s := sgHegState(6)
		for i := range s.Sanguosha.Players {
			if i != 0 || !hiddenKiller {
				s.sgHegShow(i, []int{0}, false)
			}
		}
		s.sgDie(1, 0) // victim Wu, one remaining living Wu ally
		s.sgRun()
		want := 2
		if hiddenKiller {
			want = 0
		}
		if len(s.Sanguosha.Players[0].Hand) != want {
			t.Fatal("kill reward depended on a secret faction or wrong survivor count")
		}
	}
	s := sgHegState(6)
	s.sgHegShow(0, []int{0}, false)
	s.sgHegShow(4, []int{0}, false)
	sgGive(t, s, 0, "slash")
	sgWear(t, s, 0, "crossbow")
	s.sgDie(4, 0)
	s.sgRun()
	if len(s.Sanguosha.Players[0].Hand)+len(s.Sanguosha.Players[0].Equip) != 0 {
		t.Fatal("friendly fire did not discard hand and equipment")
	}
}
