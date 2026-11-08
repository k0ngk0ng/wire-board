package stoneage_test

// Controlled starting snapshots exercise hard-to-repeat branches through the
// same public Apply/OnTimer pipeline as full games. Scope is explicitly a
// combat fixture, not a substitute for the natural full-match HTTP scenarios.
// Failure inventory predates implementation: FAILURE_SCENARIOS.md, cases 7–13.
import (
	"encoding/json"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func combatFixture(t *testing.T, required ...int) (gameapi.Match, gameapi.Context, game.State) {
	t.Helper()
	for attempt := 0; attempt < 80; attempt++ {
		m, c := start(t, 2)
		found := true
		var s game.State
		json.Unmarshal(m.Data, &s)
		for _, id := range required {
			has := false
			for _, skill := range s.Pool.Skills {
				if skill.ID == id {
					has = true
				}
			}
			found = found && has
		}
		if !found {
			continue
		}
		for step := 0; step < 8 && view(t, m, 0)["phase"] != "planning"; step++ {
			m = botsStep(t, m, &c)
		}
		json.Unmarshal(m.Data, &s)
		s.Random = 77
		for i := range s.Players {
			p := &s.Players[i]
			for _, weapon := range s.Pool.Weapons {
				if weapon.WeaponType >= 1 && weapon.WeaponType <= 3 {
					p.Loadout.Weapon = weapon.ID
					break
				}
			}
			p.Loadout.Skills = []int{1, 2, 3, 210}
			for _, id := range required {
				if id != 1 && id != 2 && id != 3 && id != 210 {
					p.Loadout.Skills = append(p.Loadout.Skills, id)
				}
			}
			for _, u := range []*game.Unit{&p.Body, &p.Pet, &p.Ride} {
				u.HP = 10000
				u.MaxHP = 10000
				u.Attack = 200
				u.Defense = 80
				u.Speed = 100
				u.Status = game.Status{}
				u.Guard = false
			}
			p.Body.MP = p.Body.MaxMP
			p.Damage = 0
			p.Healing = 0
			p.Kills = 0
			p.Submitted = false
			p.Riding = true
		}
		m.Data = raw(s)
		return m, c, s
	}
	t.Fatal("could not draw required production skill")
	return gameapi.Match{}, gameapi.Context{}, game.State{}
}
func guards(seat int) game.Plan {
	return game.Plan{Player: game.Command{Kind: "guard", Target: seat * 2}, Pet: game.Command{Kind: "guard", Target: seat*2 + 1}}
}
func combatRound(t *testing.T, m gameapi.Match, c gameapi.Context, a, b game.Plan) game.State {
	t.Helper()
	for i, plan := range []game.Plan{a, b} {
		c.Actor = i
		m = action(t, m, c, map[string]any{"type": "submit", "plan": plan})
	}
	var s game.State
	json.Unmarshal(m.Data, &s)
	if s.Phase != "resolving" || len(s.Events) == 0 {
		t.Fatal("no battle timeline")
	}
	return s
}
func finishFixture(t *testing.T, c gameapi.Context, s game.State) gameapi.Match {
	t.Helper()
	m := gameapi.Match{Schema: 1, Data: raw(s), SystemDeadline: s.ResolveAt}
	c.Actor = -1
	c.Now = time.UnixMilli(s.ResolveAt)
	m, e := game.Module().OnTimer(c, m)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Validate(2); e != nil {
		t.Fatal(e)
	}
	return m
}
func damageFrom(s game.State, actor int) int {
	total := 0
	for _, e := range s.Events {
		if e.Actor == actor && e.Kind == "attack" {
			total += e.Amount + e.RideAmount
		}
	}
	return total
}

func TestStoneageCombatGuardBreakAndRideDamageE2E(t *testing.T) {
	m, c, s := combatFixture(t)
	a, b := guards(0), guards(1)
	a.Player = game.Command{Kind: "attack", Target: 2}
	protected := combatRound(t, m, c, a, b)
	b.Player = game.Command{Kind: "attack", Target: 0}
	plain := combatRound(t, m, c, a, b)
	if damageFrom(protected, 0) >= damageFrom(plain, 0) {
		t.Fatal("guard did not reduce damage")
	}
	a, b = guards(0), guards(1)
	a.Pet = game.Command{Kind: "skill", ID: 3, Target: 2}
	broken := combatRound(t, m, c, a, b)
	b.Player = game.Command{Kind: "attack", Target: 0}
	unprotected := combatRound(t, m, c, a, b)
	if damageFrom(broken, 1) <= damageFrom(unprotected, 1) {
		t.Fatal("guard break did not distinguish defending target")
	}
	rideDamage := s.Players[1].Ride.HP - plain.Players[1].Ride.HP
	bodyDamage := s.Players[1].Body.HP - plain.Players[1].Body.HP
	if rideDamage <= 0 || bodyDamage <= 0 || plain.Players[0].Damage != rideDamage+bodyDamage {
		t.Fatal("ride split not reflected in actual HP", rideDamage, bodyDamage)
	}
	s.Players[1].Ride.HP = 1
	m.Data = raw(s)
	b = guards(1)
	fall := combatRound(t, m, c, a, b)
	if fall.Players[1].Riding || fall.Players[1].Ride.HP != 0 {
		t.Fatal("dead mount still riding")
	}
	report(t, "combat-guard-break-ride", map[string]any{"scope": "controlled initial combat states through production module", "guard": protected.Events, "plain": plain.Events, "break": broken.Events, "mountDeath": fall.Events})
}
func TestStoneageCombatDismountAndNoReplacementE2E(t *testing.T) {
	m, c, s := combatFixture(t)
	a, b := guards(0), guards(1)
	a.Pet = game.Command{Kind: "skill", ID: 210, Target: 2}
	var result game.State
	found := false
	for seed := uint64(1); seed <= 30; seed++ {
		s.Random = seed
		m.Data = raw(s)
		result = combatRound(t, m, c, a, b)
		if !result.Players[1].Riding && result.Players[1].Ride.HP > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("dismount never triggered")
	}
	next := finishFixture(t, c, result)
	v := view(t, next, 1)
	for _, kind := range []string{"switch_pet", "ride", "reserve", "revive"} {
		a := raw(map[string]any{"type": kind, "session": v["session"], "window": v["window"]})
		c.Actor = 1
		if _, e := game.Module().Apply(c, next, a); e == nil {
			t.Fatal("accepted replacement/re-ride", kind)
		}
	}
	report(t, "combat-dismount-no-replacement", result.Events)
}
func TestStoneageCombatSuppliesMagicAndControlE2E(t *testing.T) {
	m, c, s := combatFixture(t)
	p := &s.Players[0]
	p.Body.HP = 300
	p.Body.MaxHP = 1000
	hpID := 0
	oldCount := 0
	for _, row := range p.Inventory {
		if row.Item.Kind == "hp" {
			hpID = row.Item.ID
			oldCount = row.Count
		}
	}
	a, b := guards(0), guards(1)
	a.Player = game.Command{Kind: "item", ID: hpID, Target: 0}
	m.Data = raw(s)
	healed := combatRound(t, m, c, a, b)
	if healed.Players[0].Body.HP <= 300 || healed.Players[0].Body.HP > 1000 {
		t.Fatal("healing invalid")
	}
	for _, row := range healed.Players[0].Inventory {
		if row.Item.ID == hpID && row.Count != oldCount-1 {
			t.Fatal("supply consumption wrong")
		}
	}
	s.Players[0].Body.Status.Sleep = 2
	m.Data = raw(s)
	sleeping := combatRound(t, m, c, a, b)
	if sleeping.Players[0].Body.HP != 300 {
		t.Fatal("sleeping actor used potion")
	}
	s.Players[0].Body.Status = game.Status{Stone: 2, Poison: 3, PoisonSource: 2}
	m.Data = raw(s)
	poisoned := combatRound(t, m, c, a, b)
	if poisoned.Players[0].Body.HP >= 300 || poisoned.Players[0].Body.Status.Stone != 1 {
		t.Fatal("poison/control duration incorrect")
	}
	cure := 0
	for _, row := range s.Players[1].Inventory {
		if row.Item.Kind == "cure" {
			cure = row.Item.ID
		}
	}
	// An allied actor may cure an incapacitated ally in team matches; here
	// the target belongs to its own side and is a pet, preserving target checks.
	s.Players[1].Pet.Status = game.Status{Stone: 2, Poison: 3, Sleep: 2, Confusion: 2, Drunk: 2}
	b.Player = game.Command{Kind: "item", ID: cure, Target: 3}
	m.Data = raw(s)
	cured := combatRound(t, m, c, guards(0), b)
	status := cured.Players[1].Pet.Status
	if status.Stone+status.Poison+status.Sleep+status.Confusion+status.Drunk != 0 {
		t.Fatal("cure incomplete", status)
	}
	// Equipped magic only; insufficient MP is rejected before joint resolution.
	for _, magic := range s.Players[0].Magic {
		s.Players[0].Body.Status = game.Status{}
		s.Players[0].Body.MP = 0
		m.Data = raw(s)
		c.Actor = 0
		plan := guards(0)
		target := 0
		if magic.Kind == "damage" {
			target = 2
		}
		plan.Player = game.Command{Kind: "magic", ID: magic.ID, Target: target}
		if _, e := game.Module().Apply(c, m, raw(game.Action{Type: "submit", Session: s.Session, Window: s.Window, Plan: &plan})); e == nil {
			t.Fatal("accepted magic without MP")
		}
		break
	}
	report(t, "combat-supplies-control", map[string]any{"healed": healed.Events, "sleeping": sleeping.Events, "poisoned": poisoned.Events, "cured": cured.Events})
}
func TestStoneageCombatEliminationDrawAndFinalPlaybackE2E(t *testing.T) {
	m, c, s := combatFixture(t)
	s.Players[1].Body.HP = 1
	s.Players[1].Riding = false
	s.Players[1].Ride.HP = 0
	a, b := guards(0), guards(1)
	a.Player = game.Command{Kind: "attack", Target: 2}
	m.Data = raw(s)
	end := combatRound(t, m, c, a, b)
	if end.Result != nil || end.PendingResult == nil {
		t.Fatal("result published before movie completed")
	}
	result := finishFixture(t, c, end).Result
	if result == nil || result.Draw || len(result.Winners) != 1 || result.Winners[0] != 0 {
		t.Fatal("living pet blocked defeated team elimination", result)
	}
	m, c, s = combatFixture(t)
	s.Round = s.Options.MaxRounds
	m.Data = raw(s)
	draw := combatRound(t, m, c, guards(0), guards(1))
	result = finishFixture(t, c, draw).Result
	if result == nil || !result.Draw || len(result.Winners) != 0 || result.Ranks[0] != 1 || result.Ranks[1] != 1 {
		t.Fatal("round limit is not explicit draw", result)
	}
	report(t, "combat-elimination-draw", map[string]any{"elimination": end.PendingResult, "draw": result})
}
