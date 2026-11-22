package engine

import (
	"encoding/json"
	"fmt"
	"testing"
)

func hitEffectValue(t *testing.T, event Event) int {
	t.Helper()
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]json.RawMessage
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"armor", "helmet", "inventory", "armorRemainder"} {
		if value[key] != nil {
			t.Fatal("impact exposed private equipment", key)
		}
	}
	fx := 0
	if field := value["fx"]; field != nil {
		if err = json.Unmarshal(field, &fx); err != nil {
			t.Fatal(err)
		}
		if fx == 0 {
			t.Fatal("empty impact effect was not omitted")
		}
	}
	return fx
}

func TestHitEffectOriginalArmorTypesE2E(t *testing.T) {
	weapons := append(Weapons(), Weapon{ID: "knife"})
	for _, weapon := range weapons {
		for _, row := range []struct {
			group, armor, remainder, want int
			helmet                        bool
		}{
			{0, 100, 0, 1, true}, {1, 0, 0, 1, false}, {1, 100, 0, 1, false},
			{1, 100, 0, 2, true}, {1, 0, 0, 2, true},
			{2, 100, 0, 0, false}, {3, 100, 0, 0, true},
			{4, 100, 0, 0, false}, {5, 100, 0, 0, true},
			{2, 0, 1, 0, false}, {2, 0, 0, 0, true},
			{2, 0, 0, 1, false}, {6, 100, 0, 1, true}, {7, 100, 0, 1, true},
		} {
			t.Run(fmt.Sprintf("%s/%d/%d/%d/%t", weapon.ID, row.group, row.armor, row.remainder, row.helmet), func(t *testing.T) {
				s := combatArena("t")
				s.Players[1].Health = 10000
				s.Players[1].Armor, s.Players[1].ArmorRemainder, s.Players[1].Helmet = row.armor, row.remainder, row.helmet
				DamagePlayer(s, 0, 1, 10, 500, row.group, weapon.ID)
				if len(s.Events) != 1 || s.Events[0].Kind != "hit" || hitEffectValue(t, s.Events[0]) != row.want {
					t.Fatal("impact differs from original TraceAttack type branch", s.Events)
				}
			})
		}
	}
}

func TestHitEffectArmorExhaustionUsesPreHitStateE2E(t *testing.T) {
	for _, group := range []int{1, 2} {
		s := combatArena("t")
		s.Players[1].Health, s.Players[1].Armor, s.Players[1].Helmet = 10000, 1, true
		DamagePlayer(s, 0, 1, 10, 500, group, "ak47")
		want := 0
		if group == 1 {
			want = 2
		}
		if s.Players[1].Armor != 0 || s.Players[1].Helmet || hitEffectValue(t, s.Events[0]) != want {
			t.Fatal("armor exhaustion changed the current impact", s.Events, s.Players[1])
		}
		DamagePlayer(s, 0, 1, 10, 500, group, "ak47")
		if hitEffectValue(t, s.Events[1]) != 1 {
			t.Fatal("next unarmored impact did not bleed", s.Events)
		}
	}
}

func TestHitEffectNonTraceDamageAndRejectedHitsE2E(t *testing.T) {
	for _, weapon := range []string{"hegrenade", "c4", "fall", "unknown"} {
		s := combatArena("t")
		s.Players[1].Armor, s.Players[1].Helmet = 100, true
		DamagePlayer(s, 0, 1, 1, 500, 1, weapon)
		if len(s.Events) != 1 || hitEffectValue(t, s.Events[0]) != 0 {
			t.Fatal("non-TraceAttack invented a bullet effect", weapon, s.Events)
		}
	}
	s := combatArena("t")
	s.Players[1].Team = s.Players[0].Team
	DamagePlayer(s, 0, 1, 10, 500, 2, "ak47")
	if len(s.Events) != 0 {
		t.Fatal("rejected friendly hit emitted an effect", s.Events)
	}
}
