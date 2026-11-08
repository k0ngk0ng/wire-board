package stoneage_test

import (
	"encoding/json"
	"math"
	"testing"

	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
)

func eventMotion(t *testing.T, e game.Event, actor int) map[string]any {
	t.Helper()
	for _, value := range obj(t, e)["motions"].([]any) {
		m := value.(map[string]any)
		if m["actor"] == float64(actor) {
			return m
		}
	}
	t.Fatal("missing motion", actor, e)
	return nil
}
func motionPoint(m map[string]any, key string) [2]float64 {
	p := m[key].([]any)
	return [2]float64{p[0].(float64), p[1].(float64)}
}
func TestStoneageConsecutiveHitsStayAtContactE2E(t *testing.T) {
	for _, mode := range []string{"pet-double", "pet-triple", "claw", "guardian"} {
		t.Run(mode, func(t *testing.T) {
			m, c, s := combatFixture(t, 31, 11, 20)
			if mode == "claw" {
				for attempt := 0; attempt < 40; attempt++ {
					has := false
					for _, w := range s.Pool.Weapons {
						has = has || w.WeaponType == 0
					}
					if has {
						break
					}
					m, c, s = combatFixture(t, 31, 11, 20)
				}
			}
			a, b := guards(0), guards(1)
			actor, hits := 1, 2
			a.Pet = game.Command{Kind: "skill", ID: 31, Target: 2}
			if mode == "pet-triple" {
				a.Pet.ID = 11
				hits = 3
			}
			if mode == "claw" {
				actor = 0
				a.Pet = game.Command{Kind: "wait", Target: 1}
				a.Player = game.Command{Kind: "attack", Target: 2}
				found := false
				for _, w := range s.Pool.Weapons {
					if w.WeaponType == 0 {
						s.Players[0].Loadout.Weapon = w.ID
						found = true
						break
					}
				}
				if !found {
					t.Fatal("missing claw fixture")
				}
			}
			s.Players[0].Body.Speed = 1000
			s.Players[0].Pet.Speed = 1000
			s.Players[0].Ride.Speed = 1000
			s.Players[1].Body.Status.Stone = 2
			if mode == "guardian" {
				b.Pet = game.Command{Kind: "skill", ID: 20, Target: 0}
			} else {
				s.Players[1].Pet.Status.Stone = 2
			}
			m.Data = raw(s)
			end := combatRound(t, m, c, a, b)
			events := []game.Event{}
			for _, e := range end.Events {
				if e.Actor == actor && (e.Kind == "attack" || e.Kind == "miss") {
					events = append(events, e)
				}
			}
			if len(events) != hits {
				t.Fatal("wrong number of hits", events)
			}
			for i, e := range events {
				v := eventMotion(t, e, actor)
				if i == 0 {
					if v["approach"].(float64) <= 0 {
						t.Fatal("first hit must approach")
					}
				} else {
					prev := eventMotion(t, events[i-1], actor)
					if motionPoint(v, "start") != motionPoint(prev, "to") {
						t.Fatal("continuation reset to formation", v, prev)
					}
					if e.Target == events[i-1].Target && v["approach"].(float64) != 0 {
						t.Fatal("same-target continuation approached again", v)
					}
				}
				if i < hits-1 {
					if v["stay"] != true || v["returnDuration"].(float64) != 0 {
						t.Fatal("intermediate hit returned", v)
					}
				} else if v["returnDuration"].(float64) <= 0 {
					t.Fatal("final hit did not return", v)
				}
			}
			if mode == "guardian" {
				if events[0].Target != 3 || events[1].Target != 2 || events[0].RedirectReason != "忠犬代挡" {
					t.Fatal("wrong guardian sequence", events)
				}
			} else {
				for _, e := range events {
					if e.Target != 2 {
						t.Fatal("dual attack changed target", events)
					}
				}
			}
			last := end.Events[len(end.Events)-1]
			if end.Playback.Duration-last.At-last.Duration > 120 {
				t.Fatal("unnecessary round-end wait")
			}
			report(t, "sequence-"+mode, end)
		})
	}
}
func TestStoneageRangedStationaryProjectileTimelineE2E(t *testing.T) {
	for _, kind := range []int{4, 17, 18, 19} {
		t.Run(gameWeaponName(kind), func(t *testing.T) {
			m, c, s := combatFixture(t)
			for attempt := 0; attempt < 40; attempt++ {
				has := false
				for _, w := range s.Pool.Weapons {
					has = has || w.WeaponType == kind
				}
				if has {
					break
				}
				m, c, s = combatFixture(t)
			}
			found := false
			for _, w := range s.Pool.Weapons {
				if w.WeaponType == kind {
					s.Players[0].Loadout.Weapon = w.ID
					found = true
					break
				}
			}
			if !found {
				t.Fatal("weapon absent from this random fixture")
			}
			a, b := guards(0), guards(1)
			a.Player = game.Command{Kind: "attack", Target: 2}
			a.Pet = game.Command{Kind: "wait", Target: 1}
			m.Data = raw(s)
			end := combatRound(t, m, c, a, b)
			count := 0
			for _, e := range end.Events {
				if e.Actor != 0 || (e.Kind != "attack" && e.Kind != "miss") {
					continue
				}
				count++
				v := obj(t, e)
				motion := eventMotion(t, e, 0)
				if motion["approach"] != float64(0) || motion["returnDuration"] != float64(0) || motionPoint(motion, "from") != motionPoint(motion, "to") {
					t.Fatal("ranged actor walked to target", motion)
				}
				projectiles, ok := v["projectiles"].([]any)
				if !ok || len(projectiles) != 1 {
					t.Fatal("missing ranged projectile", e)
				}
				p := projectiles[0].(map[string]any)
				if p["target"] != float64(e.Target) || p["impactAt"] != v["impactAt"] || p["launchAt"].(float64) >= p["impactAt"].(float64) {
					t.Fatal("projectile impact not bound to damage", p, v)
				}
				if p["flightDuration"].(float64) < 60 || p["flightDuration"].(float64) > 650 {
					t.Fatal("unreadable projectile flight", p)
				}
				if math.Abs(float64(e.Duration)-v["impactAt"].(float64)) < 20 {
					t.Fatal("missing contact reaction")
				}
			}
			if count == 0 {
				t.Fatal("no ranged attack")
			}
			report(t, "projectile-"+gameWeaponName(kind), end)
		})
	}
}
func gameWeaponName(kind int) string {
	return map[int]string{4: "bow", 17: "boomerang", 18: "axe", 19: "stone"}[kind]
}
func TestStoneageCounterStaysInSequenceE2E(t *testing.T) {
	m, c, base := combatFixture(t, 31)
	a, b := guards(0), guards(1)
	a.Pet = game.Command{Kind: "skill", ID: 31, Target: 2}
	for seed := uint64(1); seed < 100; seed++ {
		var s game.State
		json.Unmarshal(raw(base), &s)
		s.Random = seed
		m.Data = raw(s)
		end := combatRound(t, m, c, a, b)
		for i, e := range end.Events {
			if e.Kind != "counter" || e.Target != 1 {
				continue
			}
			counter := eventMotion(t, e, e.Actor)
			if counter["approach"] != float64(0) {
				t.Fatal("counter approached from formation", e)
			}
			parked := eventMotion(t, e, 1)
			if parked["idle"] != true || motionPoint(parked, "start") == motionPoint(parked, "from") {
				t.Fatal("attacker did not remain beside counter target", e)
			}
			if i > 0 {
				prev := eventMotion(t, end.Events[i-1], 1)
				if motionPoint(parked, "start") != motionPoint(prev, "to") {
					t.Fatal("counter exchange teleported")
				}
			}
			report(t, "sequence-counter", end)
			return
		}
	}
	t.Fatal("no counter fixture")
}
