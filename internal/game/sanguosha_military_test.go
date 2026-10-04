package game

import (
	"encoding/json"
	"slices"
	"testing"
)

func sgMilitaryTestState() *State {
	s := sgTestState()
	s.Sanguosha.Options = SGOptions{Mode: "identity", Deck: "military", Packs: []string{"standard"}}
	for _, c := range sgMilitaryCards {
		s.Sanguosha.Deck = append(s.Sanguosha.Deck, c.ID)
	}
	return s
}
func sgWear(t *testing.T, s *State, i int, kind string) int {
	t.Helper()
	id := sgGive(t, s, i, kind)
	p := &s.Sanguosha.Players[i]
	p.Hand = sgRemove(p.Hand, id)
	p.Equip = append(p.Equip, id)
	return id
}
func sgMilitaryConservation(t *testing.T, s *State) {
	t.Helper()
	g := s.Sanguosha
	ids := append(append(append(append([]int{}, g.Deck...), g.Discard...), g.Table...), g.Grace...)
	for _, p := range g.Players {
		ids = append(ids, p.Hand...)
		ids = append(ids, p.Equip...)
		for _, d := range p.Judgment {
			ids = append(ids, d.Card)
		}
	}
	if q := g.Pending; q != nil && (q.Kind == "guanxing" || q.Kind == "yiji") {
		ids = append(ids, q.Cards...)
	}
	slices.Sort(ids)
	if len(ids) != 160 {
		t.Fatalf("card count=%d, pending=%+v", len(ids), g.Pending)
	}
	for j, id := range ids {
		if id != j+1 {
			t.Fatalf("duplicate/missing: index %d = %d", j, id)
		}
	}
}
func sgPassNull(t *testing.T, s *State) {
	t.Helper()
	for s.Sanguosha.Pending != nil && s.Sanguosha.Pending.Kind == "nullification" {
		sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
	}
}
func TestSanguoshaMilitaryCatalogAndLegacy(t *testing.T) {
	counts := map[string]int{}
	for j, c := range sgMilitaryCards {
		if c.ID != 109+j || c.Rank != j%13+1 || c.Suit != j/13 || SGCardTypes[c.Kind].Name == "" {
			t.Fatal(c)
		}
		counts[c.Kind]++
	}
	for kind, want := range map[string]int{"fire_slash": 5, "thunder_slash": 9, "analeptic": 5, "iron_chain": 6, "fire_attack": 3, "supply_shortage": 2, "jink": 9, "peach": 4, "nullification": 3, "vine": 2, "silver_lion": 1, "fan": 1, "guding_blade": 1, "hualiu": 1} {
		if counts[kind] != want {
			t.Fatal(kind, counts[kind], want)
		}
	}
	for _, o := range []SGOptions{{}, {Deck: "standard"}, {Deck: "military"}} {
		s, err := NewSanguosha(8, o)
		if err != nil {
			t.Fatal(err)
		}
		want := 108
		if o.Deck == "military" {
			want = 160
		}
		if len(s.Sanguosha.Deck) != want || len(s.Sanguosha.cardCatalog()) != want {
			t.Fatal(o)
		}
	}
	legacy := sgTestState()
	raw, _ := json.Marshal(legacy)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Sanguosha.cardCatalog()) != 108 {
		t.Fatal("legacy catalog expanded")
	}
	if _, err := NewSanguosha(4, SGOptions{Deck: "missing"}); err == nil {
		t.Fatal("unknown deck")
	}
}
func TestSanguoshaMilitaryWineSlashAndChain(t *testing.T) {
	s := sgMilitaryTestState()
	g := s.Sanguosha
	for i := range g.Players {
		g.Players[i].HP = 8
		g.Players[i].MaxHP = 8
	}
	wine := sgGive(t, s, 0, "analeptic")
	fire := sgGive(t, s, 0, "fire_slash")
	sgWear(t, s, 0, "guding_blade")
	sgWear(t, s, 1, "vine")
	sgWear(t, s, 2, "silver_lion")
	for i := 1; i < 4; i++ {
		g.Players[i].Chained = true
	}
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{wine}})
	if s.Sanguosha.Pending != nil || s.Sanguosha.Players[0].Drank != 1 {
		t.Fatal("wine must not open nullification")
	}
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{fire}, Targets: []int{1}})
	sgPassAll(t, s)
	g = s.Sanguosha
	// 1 base + 1 wine + 1 empty-hand blade + 1 vine = 4. Lion caps to 1;
	// propagation into the final target keeps 4, no repeated attacker bonuses.
	for i, want := range []int{8, 4, 7, 4} {
		if g.Players[i].HP != want {
			t.Fatalf("seat %d HP %d want %d", i, g.Players[i].HP, want)
		}
	}
	for _, p := range g.Players {
		if p.Chained || p.Drank != 0 {
			t.Fatal("unresolved chain/drink")
		}
	}
	sgMilitaryConservation(t, s)
}
func TestSanguoshaMilitaryArmorAndNature(t *testing.T) {
	for _, tc := range []struct {
		kind, armor, weapon string
		hp                  int
	}{
		{"slash", "vine", "", 4}, {"fire_slash", "vine", "", 2}, {"thunder_slash", "vine", "", 3},
		{"slash", "vine", "qinggang_sword", 3}, {"fire_slash", "vine", "qinggang_sword", 3},
		{"savage_assault", "vine", "", 4}, {"archery_attack", "vine", "", 4},
	} {
		t.Run(tc.kind+tc.armor+tc.weapon, func(t *testing.T) {
			s := sgMilitaryTestState()
			sgWear(t, s, 1, tc.armor)
			if tc.weapon != "" {
				sgWear(t, s, 0, tc.weapon)
			}
			id := sgGive(t, s, 0, tc.kind)
			targets := []int{1}
			if !sgIsSlash(tc.kind) {
				targets = nil
			}
			sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: targets})
			sgPassAll(t, s)
			if s.Sanguosha.Players[1].HP != tc.hp {
				t.Fatal(s.Sanguosha.Players[1].HP, tc.hp)
			}
			sgMilitaryConservation(t, s)
		})
	}
	t.Run("armor on chain recipients is not ignored", func(t *testing.T) {
		s := sgMilitaryTestState()
		sgWear(t, s, 0, "qinggang_sword")
		sgWear(t, s, 1, "vine")
		sgWear(t, s, 2, "vine")
		s.Sanguosha.Players[1].Chained = true
		s.Sanguosha.Players[2].Chained = true
		id := sgGive(t, s, 0, "fire_slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 3 || s.Sanguosha.Players[2].HP != 2 {
			t.Fatal("armor ignore propagated")
		}
	})
	t.Run("ordinary damage does not unchain", func(t *testing.T) {
		s := sgMilitaryTestState()
		s.Sanguosha.Players[1].Chained = true
		s.Sanguosha.Players[2].Chained = true
		id := sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		sgPassAll(t, s)
		if !s.Sanguosha.Players[1].Chained || s.Sanguosha.Players[2].HP != 4 {
			t.Fatal("normal damage propagated")
		}
	})
}
func TestSanguoshaMilitaryFireAttackPrivacyAndRestore(t *testing.T) {
	s := sgMilitaryTestState()
	attack := sgGive(t, s, 0, "fire_attack")
	shown := sgGive(t, s, 1, "fire_slash")
	cost := 0
	for _, id := range s.Sanguosha.Deck {
		if sgCard(id).Suit == sgCard(shown).Suit {
			cost = id
			break
		}
	}
	s.Sanguosha.Deck = sgRemove(s.Sanguosha.Deck, cost)
	s.Sanguosha.Players[0].Hand = append(s.Sanguosha.Players[0].Hand, cost)
	hidden := sgGive(t, s, 1, "thunder_slash")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{attack}, Targets: []int{1}})
	sgPassNull(t, s)
	if s.Sanguosha.Pending.Kind != "fire_reveal" {
		t.Fatal(s.Sanguosha.Pending)
	}
	sgDo(t, s, 1, Action{Cards: []int{shown}})
	if s.Sanguosha.Pending.Kind != "fire_discard" {
		t.Fatal("no discard prompt")
	}
	view := s.View(2)["sanguosha"].(map[string]any)
	if !slices.Equal(view["revealed"].([]int), []int{shown}) {
		t.Fatal("not publicly revealed")
	}
	if _, ok := view["players"].([]map[string]any)[1]["hand"]; ok {
		t.Fatal("private hand leaked", hidden)
	}
	raw, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	sgDo(t, &restored, 0, Action{Cards: []int{cost}})
	sgPassAll(t, &restored)
	if restored.Sanguosha.Players[1].HP != 3 || len(restored.Sanguosha.Revealed) != 0 || !slices.Contains(restored.Sanguosha.Players[1].Hand, shown) {
		t.Fatal("fire attack resolution")
	}
	sgMilitaryConservation(t, &restored)
}
func TestSanguoshaMilitaryIronChainRecastAndCounterspell(t *testing.T) {
	t.Run("recast is not trick use", func(t *testing.T) {
		s := sgMilitaryTestState()
		s.Sanguosha.Players[0].General = "huangyueying"
		id := sgGive(t, s, 0, "iron_chain")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}})
		if s.Sanguosha.Pending != nil || len(s.Sanguosha.Players[0].Hand) != 1 {
			t.Fatal("recast triggered jizhi or nullification")
		}
		sgMilitaryConservation(t, s)
	})
	t.Run("nullification cancels only one target", func(t *testing.T) {
		s := sgMilitaryTestState()
		id := sgGive(t, s, 0, "iron_chain")
		null := sgGive(t, s, 2, "nullification")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{0, 1}})
		sgDo(t, s, 2, Action{Cards: []int{null}})
		sgPassAll(t, s)
		if s.Sanguosha.Players[0].Chained || !s.Sanguosha.Players[1].Chained {
			t.Fatal("wrong target countered")
		}
		sgMilitaryConservation(t, s)
	})
}
func TestSanguoshaMilitarySupplyShortage(t *testing.T) {
	for _, suit := range []int{0, 2} {
		t.Run(string(rune('0'+suit)), func(t *testing.T) {
			s := sgMilitaryTestState()
			id := sgGive(t, s, 0, "supply_shortage")
			sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
			if len(s.Sanguosha.Players[1].Judgment) != 1 {
				t.Fatal("delay not placed")
			}
			g := s.Sanguosha
			for j, id := range g.Deck {
				if sgCard(id).Suit == suit {
					g.Deck[0], g.Deck[j] = g.Deck[j], g.Deck[0]
					break
				}
			}
			sgDo(t, s, 0, Action{Type: "sg_end"})
			sgPassAll(t, s)
			want := 0
			if suit == 2 {
				want = 2
			}
			if s.Turn != 1 || len(s.Sanguosha.Players[1].Hand) != want || len(s.Sanguosha.Players[1].Judgment) != 0 {
				t.Fatal("incorrect draw skip", s.Sanguosha.Players[1])
			}
			sgMilitaryConservation(t, s)
		})
	}
	t.Run("distance and duplicate validation", func(t *testing.T) {
		s := sgMilitaryTestState()
		id := sgGive(t, s, 0, "supply_shortage")
		if s.Apply(0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{2}}) == nil {
			t.Fatal("ignored distance")
		}
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		other := sgGive(t, s, 0, "supply_shortage")
		if s.Apply(0, Action{Type: "sg_play", Cards: []int{other}, Targets: []int{1}}) == nil {
			t.Fatal("duplicate delay")
		}
	})
}
func TestSanguoshaMilitarySelfRescueAndLossHeal(t *testing.T) {
	t.Run("wine only self rescue and no drink use limit", func(t *testing.T) {
		s := sgMilitaryTestState()
		wines := []int{sgGive(t, s, 0, "analeptic"), sgGive(t, s, 1, "analeptic"), sgGive(t, s, 1, "analeptic")}
		s.Sanguosha.Players[1].HP = -1
		s.Sanguosha.Players[1].Used["analeptic"] = 1
		s.sgPush(SGEvent{Type: "dying", Actor: 0, Target: 1, Step: 0})
		s.sgRun()
		if s.Apply(0, Action{Prompt: s.Sanguosha.Pending.ID, Cards: []int{wines[0]}}) == nil {
			t.Fatal("wine rescued someone else")
		}
		sgDo(t, s, 0, Action{Choice: "pass"})
		sgDo(t, s, 1, Action{Cards: []int{wines[1]}})
		sgDo(t, s, 1, Action{Cards: []int{wines[2]}})
		if s.Sanguosha.Players[1].HP != 1 || s.Sanguosha.Players[1].Drank != 0 || s.Sanguosha.Pending != nil {
			t.Fatal("wine rescue")
		}
		sgMilitaryConservation(t, s)
	})
	t.Run("lion leaving equipment heals once", func(t *testing.T) {
		s := sgMilitaryTestState()
		lion := sgWear(t, s, 0, "silver_lion")
		s.Sanguosha.Players[0].HP = 2
		vine := sgGive(t, s, 0, "vine")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{vine}})
		if s.Sanguosha.Players[0].HP != 3 || !slices.Contains(s.Sanguosha.Discard, lion) {
			t.Fatal("replacement heal")
		}
		sgMilitaryConservation(t, s)
	})
}
func TestSanguoshaMilitarySlashConversionAndLimits(t *testing.T) {
	s := sgMilitaryTestState()
	s.Sanguosha.Players[0].General = "caocao"
	sgWear(t, s, 0, "fan")
	sgWear(t, s, 1, "vine")
	id := sgGive(t, s, 0, "slash")
	other := sgGive(t, s, 0, "thunder_slash")
	sgDo(t, s, 0, Action{Type: "sg_play", Skill: "fan", Cards: []int{id}, Targets: []int{1}})
	sgPassAll(t, s)
	if s.Sanguosha.Players[1].HP != 2 {
		t.Fatal("fan not fire")
	}
	if s.Apply(0, Action{Type: "sg_play", Cards: []int{other}, Targets: []int{1}}) == nil {
		t.Fatal("elemental slash bypassed limit")
	}
	s.Sanguosha.Players[0].General = "zhaoyun"
	if kind, err := s.sgAs(0, []int{other}, "longdan", "jink"); err != nil || kind != "jink" {
		t.Fatal("longdan elemental slash", kind, err)
	}
}
func TestSanguoshaMilitaryBotsAndTimeouts(t *testing.T) {
	for _, n := range []int{4, 5, 8} {
		s, err := NewSanguosha(n, SGOptions{Deck: "military"})
		if err != nil {
			t.Fatal(err)
		}
		for step := 0; step < 6000 && !s.Finished; step++ {
			i := s.SanguoshaActor()
			a, err := s.BotAction(i)
			if err != nil {
				t.Fatalf("n=%d step=%d prompt=%+v: %v", n, step, s.Sanguosha.Pending, err)
			}
			if err = s.Apply(i, a); err != nil {
				t.Fatal(a, err)
			}
			sgMilitaryConservation(t, s)
			if step%50 == 0 && !s.Finished {
				raw, _ := json.Marshal(s)
				var restored State
				if err = json.Unmarshal(raw, &restored); err != nil {
					t.Fatal(err)
				}
				s = &restored
			}
		}
		if !s.Finished {
			t.Fatalf("n=%d bots did not finish", n)
		}
	}
}

func TestSanguoshaMilitaryFanResponseAndWineExpiry(t *testing.T) {
	t.Run("fan is use not ordinary response", func(t *testing.T) {
		s := sgMilitaryTestState()
		sgWear(t, s, 1, "fan")
		slash := sgGive(t, s, 1, "slash")
		duel := sgGive(t, s, 0, "duel")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{duel}, Targets: []int{1}})
		sgPassNull(t, s)
		if s.Apply(1, Action{Prompt: s.Sanguosha.Pending.ID, Cards: []int{slash}, Skill: "fan"}) == nil {
			t.Fatal("fan converted an ordinary response")
		}
		sgDo(t, s, 1, Action{Cards: []int{slash}})
		sgPassAll(t, s)
		sgMilitaryConservation(t, s)
	})
	t.Run("wine once per turn and effect expires", func(t *testing.T) {
		s := sgMilitaryTestState()
		wine := sgGive(t, s, 0, "analeptic")
		second := sgGive(t, s, 0, "analeptic")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{wine}})
		before, _ := json.Marshal(s)
		if s.Apply(0, Action{Type: "sg_play", Cards: []int{second}}) == nil {
			t.Fatal("second wine accepted")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("invalid wine mutated state")
		}
		sgDo(t, s, 0, Action{Type: "sg_end"})
		if s.Sanguosha.Players[0].Drank != 0 {
			t.Fatal("wine survived turn end")
		}
		sgMilitaryConservation(t, s)
	})
	t.Run("fire mandatory reveal times out but discard stays optional", func(t *testing.T) {
		s := sgMilitaryTestState()
		fire := sgGive(t, s, 0, "fire_attack")
		sgGive(t, s, 1, "jink")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{fire}, Targets: []int{1}})
		sgPassNull(t, s)
		if err := s.SanguoshaTimeout(); err != nil {
			t.Fatal(err)
		}
		if s.Sanguosha.Pending.Kind != "fire_discard" {
			t.Fatal("mandatory reveal stuck")
		}
		if err := s.SanguoshaTimeout(); err != nil {
			t.Fatal(err)
		}
		if s.Sanguosha.Players[1].HP != 4 || s.Sanguosha.Pending != nil {
			t.Fatal("timeout spent fire cost")
		}
		sgMilitaryConservation(t, s)
	})
}
