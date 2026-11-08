package stoneage_test

import (
	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"testing"
)

// Status/defense regression: incapacitated targets cannot evade or retain a
// chosen guard while executing a confused attack. Covered by failure case 9.
func TestStoneageIncapacitatedTargetCannotEvadeE2E(t *testing.T) {
	for _, kind := range []string{"sleep", "stone"} {
		t.Run(kind, func(t *testing.T) {
			m, c, s := combatFixture(t)
			s.Players[0].Body.Speed = 1
			s.Players[1].Body.Speed = 10000
			s.Players[1].Riding = false
			if kind == "sleep" {
				s.Players[1].Body.Status.Sleep = 2
			} else {
				s.Players[1].Body.Status.Stone = 2
			}
			a, b := guards(0), guards(1)
			a.Player = game.Command{Kind: "attack", Target: 2}
			for seed := uint64(1); seed <= 20; seed++ {
				s.Random = seed
				m.Data = raw(s)
				out := combatRound(t, m, c, a, b)
				found := false
				for _, e := range out.Events {
					if e.Actor == 0 && e.Target == 2 {
						if e.Kind == "miss" {
							t.Fatal("incapacitated target evaded", kind, seed)
						}
						if e.Kind == "attack" {
							found = true
						}
					}
				}
				if !found {
					t.Fatal("missing attack")
				}
			}
			report(t, "incapacitation-"+kind, map[string]any{"seeds": 20, "scope": "controlled speed/status fixture through production round execution"})
		})
	}
}
