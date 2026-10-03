package game

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestDotaPythonReference(t *testing.T) {
	var cases []struct {
		Name          string
		Round         int
		Before, After Dota
		Orders        []DotaOrder
		Finished      bool
		Winner        int
	}
	raw, err := os.ReadFile("testdata/dota_reference.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			s := &State{Kind: "dota", Round: c.Round, Phase: "dota_plan", Dota: &c.Before}
			for p, a := range c.Orders {
				if !slices.Contains(s.Dota.legal(p, true), a) {
					t.Fatalf("reference action rejected: %d %+v", p, a)
				}
			}
			s.dotaResolve(c.Orders, false)
			g := s.Dota
			want := c.After
			if !reflect.DeepEqual(g.Players, want.Players) || !reflect.DeepEqual(g.Towers, want.Towers) || g.Core != want.Core || !slices.Equal(g.Track, want.Track) || g.Kills != want.Kills || s.Finished != c.Finished {
				t.Fatalf("Go diverged from Python\ngot: %+v\nwant: %+v", g, want)
			}
			if s.Finished && !slices.Equal(s.Winners, g.seats(c.Winner)) {
				t.Fatal("wrong winning team")
			}
		})
	}
}
func dotaTestStart(t *testing.T, n int) *State {
	t.Helper()
	s, err := New("dota", n*2)
	if err != nil {
		t.Fatal(err)
	}
	for s.Phase != "dota_plan" {
		p := s.DotaActors()[0]
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Apply(p, a); err != nil {
			t.Fatal(err)
		}
	}
	return s
}
func TestDotaSetupAndPersistence(t *testing.T) {
	for _, n := range []int{1, 3, 5, 7, 8} {
		if _, err := New("dota", n); err == nil {
			t.Fatalf("accepted %d seats", n)
		}
	}
	s, _ := New("dota", 4)
	g := s.Dota
	before := clone(*s)
	if err := s.Apply(0, Action{Type: "dota_teams", Prompt: 1, Targets: []int{0, 0, 0, 1}}); err == nil {
		t.Fatal("unbalanced teams accepted")
	}
	if !reflect.DeepEqual(before, *s) {
		t.Fatal("failed action mutated state")
	}
	if err := s.Apply(0, Action{Type: "dota_teams", Prompt: 1, Targets: []int{0, 1, 0, 1}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(0, Action{Type: "dota_confirm", Prompt: 1}); err == nil {
		t.Fatal("accepted stale confirmation")
	}
	if err := s.Apply(0, Action{Type: "dota_confirm", Prompt: g.Sequence}); err != nil {
		t.Fatal(err)
	}
	restored := clone(*s)
	if !slices.Equal(s.DotaActors(), restored.DotaActors()) {
		t.Fatal("confirmation not persisted")
	}
	for _, n := range []int{1, 2, 3} {
		s = dotaTestStart(t, n)
		if s.Round != 1 || len(s.Dota.Players) != n*2 {
			t.Fatal("bad initial board")
		}
		heroes := map[string]bool{}
		for _, p := range s.Dota.Players {
			if heroes[p.Hero] {
				t.Fatal("duplicate hero")
			}
			heroes[p.Hero] = true
		}
	}
}
func TestDotaSecretOrdersAndAtomicActions(t *testing.T) {
	s := dotaTestStart(t, 3)
	g := s.Dota
	p := 0
	teammate := g.seats(g.Players[p].Team)[1]
	if teammate == p {
		teammate = g.seats(g.Players[p].Team)[2]
	}
	enemy := g.seats(1 - g.Players[p].Team)[0]
	a := g.legal(p, true)[0]
	if err := s.Apply(p, Action{Type: "dota_plan", Prompt: g.Sequence, Dota: &a}); err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []int{-1, enemy, p, teammate} {
		v := s.View(viewer)["dota"].(map[string]any)
		if _, ok := v["pending"]; ok {
			t.Fatal("raw pending leaked")
		}
		plans := v["plans"].(map[int]DotaOrder)
		_, seen := plans[p]
		if seen != (viewer == p || viewer == teammate) {
			t.Fatalf("wrong order visibility for %d", viewer)
		}
	}
	restored := clone(*s)
	if !reflect.DeepEqual(restored.Dota.Pending, g.Pending) {
		t.Fatal("hidden plan not persisted")
	}
	before := clone(*s)
	if err := s.Apply(enemy, Action{Type: "dota_plan", Prompt: g.Sequence, Dota: &DotaOrder{Card: 99}}); err == nil {
		t.Fatal("invalid plan accepted")
	}
	if !reflect.DeepEqual(before, *s) {
		t.Fatal("invalid plan mutated state")
	}
	if err := s.Apply(p, Action{Type: "dota_unlock", Prompt: g.Sequence}); err != nil {
		t.Fatal(err)
	}
	if g.Pending[p] != nil {
		t.Fatal("unlock failed")
	}
	first, err := s.BotAction(p)
	if err != nil {
		t.Fatal(err)
	}
	secret := g.legal(enemy, true)[0]
	g.Pending[enemy] = &secret
	second, err := s.BotAction(p)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("bot read enemy pending order")
	}
}
func TestDotaBotsFinishAllModesByAncient(t *testing.T) {
	for n := 1; n <= 3; n++ {
		for sample := 0; sample < 10; sample++ {
			s := dotaTestStart(t, n)
			for !s.Finished {
				if s.Round > 60 {
					t.Fatalf("stalled %dv%d: %+v", n, n, s.Dota)
				}
				p := s.DotaActors()[0]
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.Apply(p, a); err != nil {
					t.Fatal(err)
				}
				for _, h := range s.Dota.Players {
					if h.HP <= 0 || h.HP > h.maximum() || h.Gold < 0 || h.Gold > 12 || h.Charge < 0 || h.Charge > 3 {
						t.Fatalf("invalid resources %+v", h)
					}
				}
			}
			if min(s.Dota.Core[0], s.Dota.Core[1]) != 0 || len(s.Winners) != n {
				t.Fatal("match ended without Ancient/team victory")
			}
		}
	}
}
