package game

import (
	"encoding/json"
	"slices"
	"testing"
)

func sgTestState() *State {
	s, _ := New("sanguosha", 4)
	g := s.Sanguosha
	// Tests pick cards by suit or kind, so a shuffled deck would let a helper
	// take the single copy of a piece another step still needs. Sorted order
	// keeps "the first spade" and similar lookups reproducible.
	slices.Sort(g.Deck)
	g.Selecting = false
	g.Selected = 4
	g.Pending = nil
	g.Queue = nil
	g.Lord = 0
	s.Turn = 0
	s.Phase = "sg_play"
	for i := range g.Players {
		g.Players[i] = SGPlayer{General: "zhangfei", Role: []string{"lord", "rebel", "renegade", "loyalist"}[i], HP: 4, MaxHP: 4, Hand: []int{}, Equip: []int{}, Judgment: []SGDelayed{}, Used: map[string]int{}}
	}
	return s
}
func sgGive(t *testing.T, s *State, i int, kind string) int {
	t.Helper()
	g := s.Sanguosha
	for _, id := range g.Deck {
		if sgCard(id).Kind == kind {
			g.Deck = sgRemove(g.Deck, id)
			g.Players[i].Hand = append(g.Players[i].Hand, id)
			return id
		}
	}
	t.Fatal("missing card", kind)
	return 0
}
func sgDo(t *testing.T, s *State, i int, a Action) {
	t.Helper()
	if s.Sanguosha.Pending != nil {
		a.Prompt = s.Sanguosha.Pending.ID
	}
	if err := s.Apply(i, a); err != nil {
		t.Fatal(a, err)
	}
}
func sgPassAll(t *testing.T, s *State) {
	t.Helper()
	for j := 0; j < 100 && s.Sanguosha.Pending != nil && !s.Finished; j++ {
		i := s.SanguoshaActor()
		if err := s.SanguoshaTimeout(); err != nil {
			t.Fatal(i, s.Sanguosha.Pending, err)
		}
	}
}
func sgConservation(t *testing.T, s *State) {
	t.Helper()
	g := s.Sanguosha
	all := append(append(append(append([]int{}, g.Deck...), g.Discard...), g.Table...), g.Grace...)
	for _, p := range g.Players {
		all = append(all, p.Hand...)
		all = append(all, p.Equip...)
		for _, d := range p.Judgment {
			all = append(all, d.Card)
		}
	}
	if q := g.Pending; q != nil && (q.Kind == "guanxing" || q.Kind == "yiji") {
		all = append(all, q.Cards...)
	}
	seen := map[int]bool{}
	for _, id := range all {
		if seen[id] || id < 1 || id > 108 {
			t.Fatalf("duplicate or invalid card %d, phase %s prompt %+v", id, s.Phase, g.Pending)
		}
		seen[id] = true
	}
	if len(all) != 108 {
		t.Fatalf("card conservation: %d, prompt %+v", len(all), g.Pending)
	}
}
func TestSanguoshaClassicCatalog(t *testing.T) {
	if len(SGGenerals) != 25 || len(sgCards) != 108 {
		t.Fatal("catalog size")
	}
	counts := map[string]int{}
	for _, c := range sgCards {
		counts[c.Kind]++
		if c.Rank < 1 || c.Rank > 13 || SGCardTypes[c.Kind].Name == "" {
			t.Fatal(c)
		}
	}
	for k, n := range map[string]int{"slash": 30, "jink": 15, "peach": 8, "nullification": 4, "lightning": 2, "crossbow": 2} {
		if counts[k] != n {
			t.Fatal(k, counts[k])
		}
	}
}
func TestSanguoshaSlashResponseAtomicAndResume(t *testing.T) {
	s := sgTestState()
	kill := sgGive(t, s, 0, "slash")
	jink := sgGive(t, s, 1, "jink")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{kill}, Targets: []int{1}})
	if s.Sanguosha.Pending.Kind != "card" {
		t.Fatal(s.Sanguosha.Pending)
	}
	before, _ := json.Marshal(s)
	err := s.Apply(1, Action{Prompt: s.Sanguosha.Pending.ID, Cards: []int{kill}})
	after, _ := json.Marshal(s)
	if err == nil || string(before) != string(after) {
		t.Fatal("invalid action mutated state")
	}
	var resumed State
	json.Unmarshal(before, &resumed)
	sgDo(t, &resumed, 1, Action{Cards: []int{jink}})
	if resumed.Sanguosha.Players[1].HP != 4 || resumed.Phase != "sg_play" {
		t.Fatal("resume failed")
	}
	sgConservation(t, &resumed)
}
func TestSanguoshaWushuangAndDying(t *testing.T) {
	s := sgTestState()
	s.Sanguosha.Players[0].General = "lvbu"
	kill := sgGive(t, s, 0, "slash")
	jink := sgGive(t, s, 1, "jink")
	s.Sanguosha.Players[1].HP = 1
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{kill}, Targets: []int{1}})
	sgDo(t, s, 1, Action{Cards: []int{jink}})
	if s.Sanguosha.Pending.Event.Count != 1 {
		t.Fatal("must still require another jink")
	}
	sgDo(t, s, 1, Action{Choice: "pass"})
	if s.Sanguosha.Pending.Kind != "peach" {
		t.Fatal("missing rescue")
	}
	sgPassAll(t, s)
	if !s.Sanguosha.Players[1].Dead || len(s.Sanguosha.Players[0].Hand) != 3 {
		t.Fatal("death reward", s.Sanguosha.Players)
	}
	sgConservation(t, s)
}
func TestSanguoshaNegativeHealthNeedsMultiplePeaches(t *testing.T) {
	s := sgTestState()
	p1 := sgGive(t, s, 0, "peach")
	p2 := sgGive(t, s, 0, "peach")
	s.Sanguosha.Players[1].HP = 1
	s.sgPush(SGEvent{Type: "damage", Actor: 2, Target: 1, Kind: "slash", Amount: 2})
	s.sgRun()
	sgDo(t, s, 0, Action{Cards: []int{p1}})
	if s.Sanguosha.Players[1].HP != 0 || s.Sanguosha.Pending == nil {
		t.Fatal("rescued too early")
	}
	sgDo(t, s, 0, Action{Cards: []int{p2}})
	if s.Sanguosha.Players[1].HP != 1 || s.Sanguosha.Players[1].Dead {
		t.Fatal("rescue failed")
	}
	sgConservation(t, s)
}
func TestSanguoshaIdentityTeamWinners(t *testing.T) {
	s := sgTestState()
	s.sgDie(1, 0)
	s.sgDie(2, 0)
	if !s.Finished || !slices.Equal(s.Winners, []int{0, 3}) {
		t.Fatal(s.Winners)
	}
	s = sgTestState()
	s.sgDie(3, 1)
	s.sgDie(0, 1)
	if !slices.Equal(s.Winners, []int{1}) {
		t.Fatal(s.Winners)
	}
	s = sgTestState()
	s.sgDie(1, 2)
	s.sgDie(3, 2)
	s.sgDie(0, 2)
	if !slices.Equal(s.Winners, []int{2}) {
		t.Fatal(s.Winners)
	}
}
func TestSanguoshaCounterspellChain(t *testing.T) {
	s := sgTestState()
	duel := sgGive(t, s, 0, "duel")
	n1 := sgGive(t, s, 1, "nullification")
	n2 := sgGive(t, s, 2, "nullification")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{duel}, Targets: []int{1}})
	sgDo(t, s, 0, Action{Choice: "pass"})
	sgDo(t, s, 1, Action{Cards: []int{n1}})
	sgDo(t, s, 2, Action{Cards: []int{n2}})
	sgPassAll(t, s)
	if s.Sanguosha.Players[1].HP != 3 {
		t.Fatal("double nullification should restore duel")
	}
	sgConservation(t, s)
}
func TestSanguoshaViewHidesPrivateState(t *testing.T) {
	s, _ := New("sanguosha", 4)
	v := s.View(-1)["sanguosha"].(map[string]any)
	for _, key := range []string{"deck", "discard", "queue"} {
		if _, ok := v[key]; ok {
			t.Fatal("secret", key)
		}
	}
	for _, p := range v["players"].([]map[string]any) {
		if _, ok := p["hand"]; ok {
			t.Fatal("hand leak")
		}
		if role, ok := p["role"]; ok && role != "lord" {
			t.Fatal("role leak")
		}
		if _, ok := p["choices"]; ok {
			t.Fatal("choice leak")
		}
	}
	s = sgTestState()
	ids := s.sgDrawIDs(2)
	s.sgAsk(0, "yiji", "private choice", SGEvent{})
	s.Sanguosha.Pending.Cards = ids
	other := s.View(1)["sanguosha"].(map[string]any)["pending"].(map[string]any)
	if _, ok := other["cards"]; ok {
		t.Fatal("private prompt leak")
	}
}
func TestSanguoshaBotsFinishAndConserveCards(t *testing.T) {
	for _, n := range []int{4, 5, 8} {
		s, _ := New("sanguosha", n)
		for step := 0; !s.Finished && step < 6000; step++ {
			i := s.SanguoshaActor()
			a, err := s.BotAction(i)
			if err != nil {
				t.Fatalf("n=%d step=%d phase=%s prompt=%+v: %v", n, step, s.Phase, s.Sanguosha.Pending, err)
			}
			if err = s.Apply(i, a); err != nil {
				t.Fatal(a, err)
			}
			sgConservation(t, s)
		}
		if !s.Finished {
			t.Fatalf("%d-seat game did not finish", n)
		}
	}
}
