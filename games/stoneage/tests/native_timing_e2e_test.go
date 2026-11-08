package stoneage_test

import (
	"math"
	"strings"
	"testing"

	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
)

func TestStoneageNativeAttackContactTimelineE2E(t *testing.T) {
	m, c, s := combatFixture(t)
	s.Players[0].Body.Speed, s.Players[0].Ride.Speed = 1000, 1000
	m.Data = raw(s)
	a, b := guards(0), guards(1)
	a.Player = game.Command{Kind: "attack", Target: 2}
	a.Pet = game.Command{Kind: "wait", Target: 1}
	b.Pet = game.Command{Kind: "wait", Target: 3}
	end := combatRound(t, m, c, a, b)
	for _, event := range end.Events {
		if event.Kind != "attack" || event.Actor != 0 {
			continue
		}
		value := obj(t, event)
		motions, ok := value["motions"].([]any)
		if !ok || len(motions) != 1 || event.Target != 2 || event.Duration < 900 {
			t.Fatal("missing readable native approach/attack/return timeline", event)
		}
		motion := motions[0].(map[string]any)
		to := motion["to"].([]any)
		// Team-one character, first row, is (101,150), distinct from its pet.
		distance := math.Hypot(to[0].(float64)-101, to[1].(float64)-150)
		if distance > 64.01 || distance < 63.99 {
			t.Fatal("attack stopped near the front pet instead of the selected character", distance, to)
		}
		impact := value["impactAt"].(float64)
		if impact != motion["attackStart"].(float64)+motion["contactOffset"].(float64) || impact < 500 || impact >= float64(event.Duration) {
			t.Fatal("wrong native contact boundary", value)
		}
		// Native SPR timing uses the shared 1.5x server clock. A counter or
		// continuation parks at contact with only the short between-hit pause.
		minimumHold := 220.0
		if motion["stay"] == true {
			minimumHold = 65
		}
		if motion["attackDuration"].(float64) < 80 || motion["postHold"].(float64) < minimumHold {
			t.Fatal("native frames or scaled hold missing", motion)
		}
		report(t, "native-contact-timeline", end)
		return
	}
	t.Fatal("no first player attack")
}

func TestStoneageNONOTargetRedirectIsExplicitE2E(t *testing.T) {
	m, c, base := combatFixture(t, 10, 51)
	for i := range base.Pool.Pets {
		base.Pool.Pets[i].Loyalty = 25
	}
	for i := range base.Players {
		base.Players[i].Pet.Loyalty = 25
	}
	a, b := guards(0), guards(1)
	a.Pet = game.Command{Kind: "skill", ID: 51, Target: 2}
	b.Pet = game.Command{Kind: "wait", Target: 3}
	for seed := uint64(1); seed <= 100; seed++ {
		s := base
		s.Random = seed
		m.Data = raw(s)
		end := combatRound(t, m, c, a, b)
		refused := false
		for _, e := range end.Events {
			refused = refused || e.Actor == 1 && e.Kind == "status" && strings.Contains(e.Name, "NONO")
		}
		if !refused {
			continue
		}
		for _, e := range end.Events {
			if e.Actor == 1 && e.Kind == "attack" && e.Target == 3 {
				v := obj(t, e)
				if v["originalTarget"] != float64(2) || !strings.Contains(e.RedirectReason, "NONO") {
					t.Fatal("NONO target change is not explained", v)
				}
				report(t, "nono-target-redirect", end)
				return
			}
		}
	}
	t.Fatal("no random target branch")
}

func TestStoneageGuardianRedirectIsExplicitE2E(t *testing.T) {
	m, c, s := combatFixture(t, 20)
	s.Players[0].Body.Speed, s.Players[0].Ride.Speed = 1000, 1000
	m.Data = raw(s)
	a, b := guards(0), guards(1)
	a.Player = game.Command{Kind: "attack", Target: 2}
	a.Pet = game.Command{Kind: "wait", Target: 1}
	b.Pet = game.Command{Kind: "skill", ID: 20, Target: 0}
	end := combatRound(t, m, c, a, b)
	for _, e := range end.Events {
		if e.Kind == "attack" && e.Actor == 0 {
			v := obj(t, e)
			if e.Target != 3 || v["originalTarget"] != float64(2) || v["redirectReason"] != "忠犬代挡" {
				t.Fatal("redirected character attack must explain why the pet was hit", v)
			}
			report(t, "guardian-target-redirect", end)
			return
		}
	}
	t.Fatal("missing redirected attack")
}
