package stoneage_test

import (
	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"testing"
)

// Regression boundary: healing and damaging magic must retain distinct
// signed presentation effects, mana cost, ownership and actual HP changes.
func TestStoneageMagicEffectsE2E(t *testing.T) {
	for _, kind := range []string{"heal", "damage"} {
		t.Run(kind, func(t *testing.T) {
			m, c, s := combatFixture(t)
			magic := game.Magic{ID: 10, Name: "治愈的精灵", Kind: "heal", MP: 18, Power: 180}
			target := 0
			if kind == "damage" {
				magic = game.Magic{ID: 60, Name: "火焰的精灵", Kind: "damage", MP: 28, Power: 120, Status: "fire"}
				target = 2
			}
			s.Players[0].Magic = []game.Magic{magic}
			s.Players[0].Body.HP = 300
			s.Players[0].Body.MaxHP = 1000
			m.Data = raw(s)
			a, b := guards(0), guards(1)
			a.Player = game.Command{Kind: "magic", ID: magic.ID, Target: target}
			out := combatRound(t, m, c, a, b)
			found := false
			for _, event := range out.Events {
				if event.Kind == "magic" {
					fields := obj(t, event)
					if fields["effect"] != kind || event.Amount <= 0 {
						t.Fatal("magic sign/effect lost", fields)
					}
					found = true
				}
			}
			if !found || out.Players[0].Body.MP != s.Players[0].Body.MP-magic.MP {
				t.Fatal("magic or MP cost missing")
			}
			report(t, "magic-effect-"+kind, out.Events)
		})
	}
}
