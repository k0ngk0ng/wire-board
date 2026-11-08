package stoneage_test

import (
	"encoding/json"
	"reflect"
	"testing"

	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func comboActors(t *testing.T, event game.Event) []int {
	t.Helper()
	var actors []int
	json.Unmarshal(raw(obj(t, event)["actors"]), &actors)
	return actors
}
func TestStoneageClassicCombinedAttackE2E(t *testing.T) {
	m, c, base := combatFixture(t)
	base.Players[0].Body.Speed, base.Players[0].Ride.Speed, base.Players[0].Pet.Speed = 500, 500, 500
	base.Players[1].Body.Speed, base.Players[1].Ride.Speed, base.Players[1].Pet.Speed = 10000, 10000, 10000
	a, b := guards(0), guards(1)
	a.Player = game.Command{Kind: "attack", Target: 2}
	a.Pet = game.Command{Kind: "skill", ID: 1, Target: 2}
	b.Pet = game.Command{Kind: "wait", Target: 3}
	formed, separate := 0, 0
	for seed := uint64(1); seed <= 160; seed++ {
		s := base
		s.Random = seed
		m.Data = raw(s)
		end := combatRound(t, m, c, a, b)
		var combo *game.Event
		for i, event := range end.Events {
			if event.Kind == "combo" {
				combo = &end.Events[i]
			}
		}
		stream := seed
		for i := 0; i < 8; i++ {
			referenceRandom(&stream, 100)
		}
		if (combo != nil) != (referenceRandom(&stream, 100) < 50) {
			t.Fatal("combined opening must use the original 50/100 boundary", seed)
		}
		if combo == nil {
			separate++
			continue
		}
		formed++
		actors := comboActors(t, *combo)
		if len(actors) != 2 || actors[0] == actors[1] || actors[0] > 1 || actors[1] > 1 || combo.Target != 2 {
			t.Fatal("invalid automatic combined attack", *combo)
		}
		if combo.Amount+combo.RideAmount <= 0 {
			t.Fatal("combined attack missed a much faster defender", *combo)
		}
		for _, event := range end.Events {
			if event.Kind == "counter" || event.Kind == "miss" || event.Kind == "attack" && event.Actor <= 1 {
				t.Fatal("combined participants attacked twice or allowed ordinary evasion/counter", event)
			}
		}
		if end.Players[0].Damage != combo.Amount+combo.RideAmount || base.Players[1].Body.HP-end.Players[1].Body.HP != combo.Amount || base.Players[1].Ride.HP-end.Players[1].Ride.HP != combo.RideAmount {
			t.Fatal("combined damage/ride partition/attribution differs", *combo)
		}
		restored, err := game.Module().Restore(c, gameapi.Match{Schema: 1, Data: raw(end), SystemDeadline: end.ResolveAt})
		if err != nil {
			t.Fatal(err)
		}
		var replay game.State
		json.Unmarshal(restored.Data, &replay)
		if !reflect.DeepEqual(end, replay) {
			t.Fatal("restore changed combined attack timeline")
		}
		if formed == 1 {
			report(t, "classic-combined-attack", end)
		}
	}
	if formed < 55 || formed > 105 || separate < 55 {
		t.Fatal("automatic 50% start gate missing", formed, separate)
	}
	report(t, "classic-combined-attack-rate", map[string]any{"seeds": 160, "formed": formed, "separate": separate})
}

func TestStoneageCombinedAttackBreakConditionsE2E(t *testing.T) {
	m, c, base := combatFixture(t, 10)
	a, b := guards(0), guards(1)
	a.Player = game.Command{Kind: "attack", Target: 2}
	a.Pet = game.Command{Kind: "skill", ID: 1, Target: 2}
	b.Pet = game.Command{Kind: "wait", Target: 3}
	for _, name := range []string{"different target", "ranged", "special skill", "sleep", "enemy interrupts", "loyalty refusal"} {
		t.Run(name, func(t *testing.T) {
			for seed := uint64(1); seed <= 50; seed++ {
				var s game.State
				json.Unmarshal(raw(base), &s)
				s.Random = seed
				first := a
				s.Players[0].Body.Speed, s.Players[0].Ride.Speed, s.Players[0].Pet.Speed = 1000, 1000, 1000
				s.Players[1].Body.Speed, s.Players[1].Ride.Speed, s.Players[1].Pet.Speed = 1, 1, 1
				switch name {
				case "different target":
					first.Pet.Target = 3
				case "ranged":
					// Use an existing drawn weapon, with a controlled family.
					for i := range s.Pool.Weapons {
						if s.Pool.Weapons[i].ID == s.Players[0].Loadout.Weapon {
							s.Pool.Weapons[i].WeaponType = 4
						}
					}
				case "special skill":
					first.Pet.ID = 10
				case "sleep":
					s.Players[0].Pet.Status.Sleep = 2
				case "enemy interrupts":
					s.Players[0].Pet.Speed = 1
					s.Players[1].Body.Speed, s.Players[1].Ride.Speed = 500, 500
				case "loyalty refusal":
					for i := range s.Pool.Pets {
						s.Pool.Pets[i].Loyalty = 25
					}
					for i := range s.Players {
						s.Players[i].Pet.Loyalty = 25
					}
				}
				m.Data = raw(s)
				end := combatRound(t, m, c, first, b)
				refused, combined := false, false
				for _, e := range end.Events {
					refused = refused || e.Actor == 1 && e.Kind == "status"
					combined = combined || e.Kind == "combo"
				}
				if combined && (name != "loyalty refusal" || refused) {
					t.Fatal("invalid combined attack", name, seed)
				}
			}
		})
	}
}

func TestStoneageCombinedAttackTenMembersAndLethalE2E(t *testing.T) {
	m, c := start(t, 10)
	for step := 0; step < 50 && view(t, m, 0)["phase"] != "planning"; step++ {
		m = botsStep(t, m, &c)
	}
	var base game.State
	json.Unmarshal(m.Data, &base)
	if base.Phase != "planning" {
		t.Fatal("5v5 selection did not finish")
	}
	for i := range base.Pool.Pets {
		base.Pool.Pets[i].Loyalty = 100
	}
	plans := make([]game.Plan, 10)
	for i := range base.Players {
		p := &base.Players[i]
		p.Pet.Loyalty = 100
		p.Riding = false
		for _, w := range base.Pool.Weapons {
			if w.WeaponType >= 1 && w.WeaponType <= 3 {
				p.Loadout.Weapon = w.ID
				break
			}
		}
		p.Loadout.Skills = []int{1, 2}
		for _, u := range []*game.Unit{&p.Body, &p.Pet} {
			u.HP, u.MaxHP, u.Attack, u.Defense, u.Speed = 10000, 10000, 200, 80, 500
			if p.Team == 1 {
				u.Speed = 10000
			}
		}
		plans[i] = guards(i)
		if p.Team == 0 {
			plans[i] = game.Plan{Player: game.Command{Kind: "attack", Target: 2}, Pet: game.Command{Kind: "skill", ID: 1, Target: 2}}
		}
	}
	base.Players[1].Body.HP = 1
	for seed := uint64(1); seed <= 60; seed++ {
		s := base
		s.Random = seed
		m.Data = raw(s)
		for i, plan := range plans {
			c.Actor = i
			m = action(t, m, c, map[string]any{"type": "submit", "plan": plan})
		}
		var end game.State
		json.Unmarshal(m.Data, &end)
		for _, e := range end.Events {
			if e.Kind != "combo" || len(comboActors(t, e)) != 10 {
				continue
			}
			kills, damage := 0, 0
			for _, p := range end.Players {
				kills += p.Kills
				damage += p.Damage
			}
			if e.Target != 2 || e.Amount != 1 || kills != 1 || damage != 1 {
				t.Fatal("lethal combined attack repeated a kill, lost members, or retargeted", e, kills, damage)
			}
			report(t, "combined-ten-members-lethal", end)
			return
		}
	}
	t.Fatal("no ten-member combined attack")
}
