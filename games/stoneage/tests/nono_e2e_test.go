package stoneage_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestStoneageNONOPreparedBalanceE2E(t *testing.T) {
	m, c, base := combatFixture(t, 10, 40, 51)
	for i := range base.Pool.Pets {
		base.Pool.Pets[i].Loyalty = 25
	}
	for i := range base.Players {
		base.Players[i].Pet.Loyalty = 25
	}
	a, b := guards(0), guards(1)
	a.Pet = game.Command{Kind: "skill", ID: 51, Target: 2}
	b.Pet = game.Command{Kind: "wait", Target: 3}
	seen := map[string]bool{}
	for seed := uint64(1); seed <= 300 && len(seen) < 2; seed++ {
		s := base
		s.Random = seed
		m.Data = raw(s)
		end := combatRound(t, m, c, a, b)
		refused, name, damage := false, "", 0
		for _, e := range end.Events {
			if e.Actor == 1 && strings.Contains(e.Name, "擅自使用技能") {
				refused = true
			}
			if e.Actor == 1 && e.Kind == "attack" {
				name = e.Name
				damage += e.Amount + e.RideAmount
			}
		}
		if !refused || name != "连续攻击" && name != "一击必杀" || seen[name] {
			continue
		}
		plain := a
		plain.Pet = game.Command{Kind: "skill", ID: 1, Target: 2}
		m.Data = raw(s)
		comparison := combatRound(t, m, c, plain, b)
		plainDamage := 0
		for _, e := range comparison.Events {
			if e.Actor == 1 && e.Kind == "attack" {
				if e.Name != name {
					t.Fatal("same seed chose different NONO skill", e.Name, name)
				}
				plainDamage += e.Amount + e.RideAmount
			}
		}
		// Mighty can miss or be fully guarded. Verify the damage modifier
		// using a nonzero impact with the same replacement/target/RNG.
		if plainDamage == 0 {
			continue
		}
		if damage <= plainDamage {
			t.Fatal("NONO lost the prepared backwater attack modifier", name, seed, damage, plainDamage)
		}
		pet := end.Players[0].Pet
		if pet.Attack != base.Players[0].Pet.Attack || pet.Defense != base.Players[0].Pet.Defense || obj(t, pet)["turnAttack"] != float64(145) || pet.TurnDefense != 45 {
			t.Fatal("NONO must keep round modifiers and the defense cost, without changing base stats", pet)
		}
		restored, err := game.Module().Restore(c, gameapi.Match{Schema: 1, Data: raw(end), SystemDeadline: end.ResolveAt})
		if err != nil {
			t.Fatal(err)
		}
		var replay game.State
		json.Unmarshal(restored.Data, &replay)
		if !reflect.DeepEqual(end, replay) {
			t.Fatal("restoring NONO timeline recomputed the result")
		}
		seen[name] = true
		report(t, "nono-"+name, map[string]any{"scope": "controlled production Apply round", "seed": seed, "boostedDamage": damage, "plainDamage": plainDamage, "end": end})
		// A new round must clear the old prepared modifier even if the new
		// pet instruction is wait; restoring the timeline must not reroll it.
		next := finishFixture(t, c, end)
		var before game.State
		json.Unmarshal(next.Data, &before)
		m.Data = next.Data
		after := combatRound(t, m, c, guards(0), b)
		if obj(t, after.Players[0].Pet)["turnAttack"] != float64(100) || after.Players[0].Pet.TurnDefense != 100 || before.Random == 0 {
			t.Fatal("NONO modifiers leaked into next round")
		}
	}
	if len(seen) != 2 {
		t.Fatal("both NONO combination branches must be exercised", seen)
	}
}
