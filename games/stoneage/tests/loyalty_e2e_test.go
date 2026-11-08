package stoneage_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestStoneageRandomLoyaltyHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 4, game.Module(), "stoneage")
	h.Start(map[string]any{})
	v := httpView(t, h.People[0])
	low := 0
	for _, pet := range v.Pool.Pets {
		weight := 0
		for _, n := range pet.Weights {
			weight += n
		}
		if pet.Loyalty < 80 {
			low++
			if weight < 100 || pet.Loyalty < 20 || pet.Loyalty > 30 {
				t.Fatal("only stronger species may have NONO loyalty 20..30", pet)
			}
		}
	}
	if low > 3 {
		t.Fatal("at least 17 candidates must have loyalty >=80", low)
	}
	for _, ride := range v.Pool.Rides {
		if ride.Loyalty != 100 {
			t.Fatal("ride loyalty must stay at 100")
		}
	}
	seen := map[int]bool{}
	for _, p := range append(v.Pool.Pets, v.Pool.Rides...) {
		if p.Loyalty < 0 || p.Loyalty > 100 {
			t.Fatal("loyalty out of range")
		}
		seen[p.Loyalty] = true
	}
	if len(seen) < 2 {
		t.Fatal("loyalty was uniform")
	}
	for _, c := range append(h.People, h.Spectator) {
		if !reflect.DeepEqual(v.Pool, httpView(t, c).Pool) {
			t.Fatal("loyalty differs by viewer")
		}
	}
	h.Restart()
	if !reflect.DeepEqual(v.Pool, httpView(t, h.People[0]).Pool) {
		t.Fatal("restart rerolled loyalty")
	}
	httpSelection(t, h)
	for _, p := range httpView(t, h.People[0]).Players {
		for _, pet := range v.Pool.Pets {
			if pet.ID == p.Loadout.Pet && pet.Loyalty != p.Pet.Loyalty {
				t.Fatal("combat pet lost candidate loyalty")
			}
		}
		for _, pet := range v.Pool.Rides {
			if pet.ID == p.Loadout.Ride && pet.Loyalty != p.Ride.Loyalty {
				t.Fatal("ride lost candidate loyalty")
			}
		}
	}
	report(t, "random-loyalty-http", httpView(t, h.People[0]))
}

func TestStoneageLoyaltyCombatAndEscapeE2E(t *testing.T) {
	m, c, base := combatFixture(t)
	for i := range base.Pool.Pets {
		base.Pool.Pets[i].Loyalty = 0
	}
	for i := range base.Players {
		base.Players[i].Pet.Loyalty = 0
	}
	a, b := guards(0), guards(1)
	a.Pet = game.Command{Kind: "skill", ID: 1, Target: 2}
	b.Pet = game.Command{Kind: "wait", Target: 3}
	owner, fled := false, false
	for seed := 1; seed <= 150 && (!owner || !fled); seed++ {
		s := base
		s.Random = uint64(seed)
		m.Data = raw(s)
		end := combatRound(t, m, c, a, b)
		for _, e := range end.Events {
			if e.Actor == 1 && strings.Contains(e.Name, "攻击主人") {
				owner = true
			}
			if e.Actor == 1 && e.Kind == "flee" {
				fled = true
				if !end.Players[0].Pet.Fled || end.Players[0].Pet.HP <= 0 {
					t.Fatal("escape became death")
				}
				restored := finishFixture(t, c, end)
				c.Actor = 0
				v := view(t, restored, 0)
				if v["phase"] != "planning" {
					t.Fatal("escape ended a living player's game")
				}
				before := string(restored.Data)
				if _, err := game.Module().Apply(c, restored, raw(map[string]any{"type": "submit", "session": end.Session, "window": v["window"], "plan": a})); err == nil {
					t.Fatal("escaped pet can attack")
				}
				if string(restored.Data) != before {
					t.Fatal("rejected escaped command mutated state")
				}
				report(t, "loyalty-escape", end)
			}
		}
	}
	if !owner || !fled {
		t.Fatal("loyalty branches not exercised", owner, fled)
	}
}

func TestStoneageLegacyLoyaltyRestore(t *testing.T) {
	m, c, s := combatFixture(t)
	s.Rules = "stoneage-arena-v1"
	m.Data = raw(s)
	var legacy map[string]any
	json.Unmarshal(m.Data, &legacy)
	pool := legacy["pool"].(map[string]any)
	for _, key := range []string{"pets", "rides"} {
		for _, v := range pool[key].([]any) {
			delete(v.(map[string]any), "loyalty")
		}
	}
	for _, p := range legacy["players"].([]any) {
		for _, key := range []string{"pet", "ride"} {
			delete(p.(map[string]any)[key].(map[string]any), "loyalty")
		}
	}
	m.Data = raw(legacy)
	restored, err := game.Module().Restore(c, m)
	if err != nil {
		t.Fatal(err)
	}
	var after game.State
	json.Unmarshal(restored.Data, &after)
	if after.Random != s.Random || after.Window != s.Window {
		t.Fatal("migration reset RNG/window")
	}
	for _, p := range after.Players {
		if p.Pet.Loyalty != 100 || p.Ride.Loyalty != 100 {
			t.Fatal("legacy loyalty did not default to 100")
		}
	}
	report(t, "legacy-loyalty-restore", after)
}
