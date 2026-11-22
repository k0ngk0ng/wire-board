package engine

import (
	"bytes"
	"encoding/json"
	"testing"
)

func knifeReferenceArena() *State {
	s := classicArena()
	s.BuyEndsAt = 9000
	p := &s.Players[0]
	p.Weapon, p.Inventory = "knife", []WeaponState{{ID: "knife", Ammo: 1}}
	p.X, p.Y, p.Z = 0, 0, 36*Unit
	s.Players[1].X, s.Players[1].Yaw, s.Players[1].Character = 25*Unit, 32768, "urban"
	return s
}

func knifeReferenceRemainder(t *testing.T, player Player) int {
	t.Helper()
	raw, err := json.Marshal(player)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	value := 0
	if field := fields["armorRemainder"]; field != nil {
		if err = json.Unmarshal(field, &value); err != nil {
			t.Fatal(err)
		}
	}
	return value
}

func knifeReferenceSetRemainder(t *testing.T, player *Player, value int) {
	t.Helper()
	raw, err := json.Marshal(player)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	fields["armorRemainder"], err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(fields)
	if err != nil || json.Unmarshal(raw, player) != nil {
		t.Fatal("invalid test armor remainder", err)
	}
}

func knifeReferenceHit(t *testing.T, state *State, secondary bool) Event {
	t.Helper()
	before := len(state.Events)
	state.Players[0].Input = Input{Fire: !secondary, Secondary: secondary}
	StepCombat(state)
	for _, event := range state.Events[before:] {
		if event.Kind == "hit" && event.Weapon == "knife" {
			return event
		}
	}
	t.Fatal("knife did not reach actual model or hull")
	return Event{}
}

func TestKnifeReferenceStabDamageAndArmorE2E(t *testing.T) {
	for _, scenario := range []struct {
		name                                string
		z, pitch, targetY, yaw, armor       int
		helmet, friendly                    bool
		group, damage, remaining, remainder int
	}{
		{"front-head", 36, 0, 0, 32768, 0, false, false, 1, 260, 0, 0},
		{"front-head-vest", 36, 0, 0, 32768, 100, false, false, 1, 260, 100, 0},
		{"front-head-helmet", 36, 0, 0, 32768, 100, true, false, 1, 221, 80, 400000},
		{"front-chest", 36, 4000, 0, 32768, 0, false, false, 2, 65, 0, 0},
		{"front-chest-vest", 36, 4000, 0, 32768, 100, false, false, 2, 55, 95, 100000},
		{"front-abdomen", 20, 0, 0, 32768, 0, false, false, 3, 81, 0, 0},
		{"front-abdomen-vest", 20, 0, 0, 32768, 100, false, false, 3, 69, 93, 725000},
		{"front-leg-vest", 4, 0, 0, 32768, 100, false, false, 6, 48, 100, 0},
		{"back-chest", 36, 0, 0, 0, 0, false, false, 2, 195, 0, 0},
		{"back-chest-vest", 36, 0, 0, 0, 100, false, false, 2, 165, 85, 300000},
		{"back-abdomen", 36, 4000, 0, 0, 0, false, false, 3, 243, 0, 0},
		{"back-abdomen-vest", 36, 4000, 0, 0, 100, false, false, 3, 207, 81, 575000},
		{"generic-hull-vest", 36, 0, 25, 32768, 100, false, false, 0, 55, 95, 100000},
		{"generic-hull-thin-vest", 36, 0, 25, 32768, 4, false, false, 0, 57, 0, 0},
		{"friendly-chest-vest", 36, 4000, 0, 32768, 100, false, true, 2, 19, 98, 235000},
		{"friendly-leg-vest", 4, 0, 0, 32768, 100, false, true, 6, 17, 100, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s := knifeReferenceArena()
			s.Players[0].Z, s.Players[0].Pitch = scenario.z*Unit, scenario.pitch
			target := &s.Players[1]
			target.Y, target.Yaw, target.Armor, target.Helmet = scenario.targetY*Unit, scenario.yaw, scenario.armor, scenario.helmet
			if scenario.friendly {
				s.Options.FriendlyFire, target.Team = true, "t"
			}
			hit := knifeReferenceHit(t, s, true)
			if hit.Hitgroup != scenario.group || hit.Value != scenario.damage || target.Health != max(0, 100-scenario.damage) || target.Armor != scenario.remaining || knifeReferenceRemainder(t, *target) != scenario.remainder || target.Alive != (scenario.damage < 100) {
				t.Fatalf("group=%d damage=%d health=%d armor=%d remainder=%d alive=%t", hit.Hitgroup, hit.Value, target.Health, target.Armor, knifeReferenceRemainder(t, *target), target.Alive)
			}
			t.Logf("group=%d damage=%d health=%d armor=%d remainder=%d alive=%t", hit.Hitgroup, hit.Value, target.Health, target.Armor, knifeReferenceRemainder(t, *target), target.Alive)
		})
	}
}

func TestKnifeReferenceBackstabAngleBoundaryE2E(t *testing.T) {
	for _, scenario := range []struct{ yaw, damage int }{
		{6710, 195}, {6711, 195}, {6712, 65},
		{65536 - 6710, 195}, {65536 - 6711, 195}, {65536 - 6712, 65},
	} {
		s := knifeReferenceArena()
		s.Players[1].Yaw = scenario.yaw
		hit := knifeReferenceHit(t, s, true)
		if hit.Hitgroup != 2 || hit.Value != scenario.damage {
			t.Fatalf("yaw=%d group=%d damage=%d want=%d", scenario.yaw, hit.Hitgroup, hit.Value, scenario.damage)
		}
	}
}

func TestKnifeReferenceBackstabUsesAttackForwardE2E(t *testing.T) {
	for _, scenario := range []struct {
		y, yaw, damage int
	}{
		{31, 0, 195}, {-31, 0, 195}, {31, 8192, 65}, {-31, 65536 - 8192, 65},
	} {
		s := knifeReferenceArena()
		s.Players[1].X, s.Players[1].Y, s.Players[1].Yaw = 33*Unit, scenario.y*Unit, scenario.yaw
		hit := knifeReferenceHit(t, s, true)
		if hit.Hitgroup != 0 || hit.Value != scenario.damage {
			t.Fatalf("side hull y=%d targetYaw=%d group=%d damage=%d want=%d", scenario.y, scenario.yaw, hit.Hitgroup, hit.Value, scenario.damage)
		}
		t.Logf("nonoverlapping side hull x=33 y=%d attackYaw=0 targetYaw=%d damage=%d", scenario.y, scenario.yaw, hit.Value)
	}
}

func TestKnifeReferenceArmorThinBoundaryE2E(t *testing.T) {
	for _, scenario := range []struct{ armor, fraction, damage, remaining, remainder int }{
		{0, 0, 65, 0, 0}, {0, 1, 64, 0, 0}, {1, 0, 63, 0, 0},
		{4, 0, 57, 0, 0}, {4, 699999, 55, 0, 0}, {4, 700000, 55, 0, 0}, {4, 700001, 55, 0, 1}, {5, 0, 55, 0, 100000},
	} {
		s := knifeReferenceArena()
		s.Players[0].Pitch = 4000
		target := &s.Players[1]
		target.Armor = scenario.armor
		knifeReferenceSetRemainder(t, target, scenario.fraction)
		hit := knifeReferenceHit(t, s, true)
		if hit.Hitgroup != 2 || hit.Value != scenario.damage || target.Armor != scenario.remaining || knifeReferenceRemainder(t, *target) != scenario.remainder {
			t.Fatalf("armor=%d fraction=%d damage=%d remaining=%d remainder=%d", scenario.armor, scenario.fraction, hit.Value, target.Armor, knifeReferenceRemainder(t, *target))
		}
	}
}

func TestKnifeReferenceRepeatedSwingsAndMixedGunArmorE2E(t *testing.T) {
	s := knifeReferenceArena()
	s.Players[0].Pitch = 4000
	s.Players[1].Armor = 100
	for index, want := range []struct{ health, armor, fraction int }{{88, 98, 700000}, {76, 97, 600000}, {64, 96, 500000}, {52, 95, 400000}, {40, 94, 300000}, {28, 93, 200000}, {16, 92, 100000}, {4, 91, 0}, {0, 89, 700000}} {
		hit := knifeReferenceHit(t, s, false)
		if hit.Hitgroup != 2 || s.Players[1].Health != want.health || s.Players[1].Armor != want.armor || knifeReferenceRemainder(t, s.Players[1]) != want.fraction {
			t.Fatalf("swing=%d hit=%+v health=%d armor=%d fraction=%d", index+1, hit, s.Players[1].Health, s.Players[1].Armor, knifeReferenceRemainder(t, s.Players[1]))
		}
		s.Frame += 40
	}
	s = knifeReferenceArena()
	s.Players[0].Pitch, s.Players[1].Armor = 4000, 100
	knifeReferenceHit(t, s, false)
	DamagePlayer(s, 0, 1, 34, 500, 2, "usp")
	if s.Players[1].Health != 71 || s.Players[1].Armor != 90 || knifeReferenceRemainder(t, s.Players[1]) != 300000 {
		t.Fatal("ordinary gun damage discarded knife armor fraction")
	}
	s.Frame += 40
	knifeReferenceHit(t, s, false)
	if s.Players[1].Health != 59 || s.Players[1].Armor != 89 || knifeReferenceRemainder(t, s.Players[1]) != 200000 {
		t.Fatal("knife after firearm lost fractional armor")
	}
	s = knifeReferenceArena()
	s.Players[1].Armor = 0
	knifeReferenceSetRemainder(t, &s.Players[1], 400000)
	DamagePlayer(s, 0, 1, 34, 500, 2, "usp")
	if s.Players[1].Health != 67 || s.Players[1].Armor != 0 || knifeReferenceRemainder(t, s.Players[1]) != 0 {
		t.Fatal("firearm ignored the final fractional armor point")
	}
}

func TestKnifeReferenceArmorRestorePurchaseAndRoundE2E(t *testing.T) {
	s := knifeReferenceArena()
	s.Players[0].Pitch, s.Players[1].Armor = 4000, 100
	knifeReferenceHit(t, s, false)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	restored.Frame += 40
	knifeReferenceHit(t, &restored, false)
	if restored.Players[1].Health != 76 || restored.Players[1].Armor != 97 || knifeReferenceRemainder(t, restored.Players[1]) != 600000 {
		t.Fatal("saved armor fraction was not restored")
	}
	for _, item := range []string{"vest", "vesthelm"} {
		s = knifeReferenceArena()
		s.Players[1].Armor, s.Players[1].Money = 98, 16000
		knifeReferenceSetRemainder(t, &s.Players[1], 400000)
		if err = Buy(s, 1, item); err != nil {
			t.Fatal(err)
		}
		if s.Players[1].Armor != 100 || knifeReferenceRemainder(t, s.Players[1]) != 0 {
			t.Fatal("armor purchase retained old fraction", item)
		}
	}
	s = knifeReferenceArena()
	s.Players[1].Armor = 98
	knifeReferenceSetRemainder(t, &s.Players[1], 400000)
	InitRound(s)
	if s.Players[1].Armor != 98 || knifeReferenceRemainder(t, s.Players[1]) != 400000 {
		t.Fatal("surviving player lost carried armor fraction")
	}
	s.Players[1].Alive, s.Players[1].Health = false, 0
	InitRound(s)
	if s.Players[1].Armor != 0 || knifeReferenceRemainder(t, s.Players[1]) != 0 {
		t.Fatal("respawn retained previous life armor fraction")
	}
	var legacy Player
	if err = json.Unmarshal([]byte("{\"armor\":100,\"alive\":true,\"health\":100}"), &legacy); err != nil || knifeReferenceRemainder(t, legacy) != 0 {
		t.Fatal("legacy armor did not default to zero fraction", err)
	}
}

func TestKnifeReferenceDepletedHelmetAndRebuyE2E(t *testing.T) {
	s := knifeReferenceArena()
	s.Players[0].Pitch = 4000
	target := &s.Players[1]
	target.Armor, target.Helmet, target.Money = 1, true, 16000
	knifeReferenceHit(t, s, true)
	if target.Armor != 0 || target.Helmet {
		t.Fatal("exhausted armor retained helmet protection")
	}
	before := target.Money
	if err := Buy(s, 1, "vesthelm"); err != nil {
		t.Fatal(err)
	}
	if before-target.Money != 1000 || target.Armor != 100 || !target.Helmet || knifeReferenceRemainder(t, *target) != 0 {
		t.Fatal("depleted armor incorrectly kept the helmet-only discount")
	}
	s = knifeReferenceArena()
	target = &s.Players[1]
	target.Helmet = true
	knifeReferenceSetRemainder(t, target, 400000)
	DamagePlayer(s, 0, 1, 1, 500, 2, "usp")
	if !target.Helmet || target.Armor != 0 || knifeReferenceRemainder(t, *target) != 200000 {
		t.Fatal("positive fractional armor lost helmet prematurely")
	}
	DamagePlayer(s, 0, 1, 1, 500, 2, "usp")
	if target.Helmet || target.Armor != 0 || knifeReferenceRemainder(t, *target) != 0 {
		t.Fatal("exact armor exhaustion retained helmet")
	}
}

func TestKnifeReferenceLegacyHelmetCreditE2E(t *testing.T) {
	for _, fraction := range []int{0, 400000} {
		for _, item := range []string{"vest", "vesthelm"} {
			s := knifeReferenceArena()
			p := &s.Players[1]
			p.Armor, p.Helmet, p.Money = 0, true, 16000
			knifeReferenceSetRemainder(t, p, fraction)
			raw, err := json.Marshal(p)
			if err != nil || json.Unmarshal(raw, p) != nil {
				t.Fatal("legacy player did not restore", err)
			}
			before := p.Money
			if err = Buy(s, 1, item); err != nil {
				t.Fatal(err)
			}
			price := 650
			if item == "vesthelm" && fraction == 0 {
				price = 1000
			}
			if before-p.Money != price || p.Armor != 100 || p.Helmet != (item == "vesthelm" || fraction > 0) || knifeReferenceRemainder(t, *p) != 0 {
				t.Fatalf("legacy item=%s fraction=%d price=%d armor=%d helmet=%t", item, fraction, before-p.Money, p.Armor, p.Helmet)
			}
		}
	}
	for _, item := range []string{"vest", "vesthelm"} {
		s := knifeReferenceArena()
		p := &s.Players[1]
		p.Armor, p.Helmet, p.Money = 0, true, 649
		if item == "vesthelm" {
			p.Money = 999
		}
		before, _ := json.Marshal(p)
		err := Buy(s, 1, item)
		after, _ := json.Marshal(p)
		if err == nil || !bytes.Equal(before, after) {
			t.Fatal("failed legacy armor purchase mutated the player", item, err)
		}
	}
	for _, fraction := range []int{0, 400000} {
		s := knifeReferenceArena()
		p := &s.Players[1]
		p.Armor, p.Helmet = 0, true
		knifeReferenceSetRemainder(t, p, fraction)
		InitRound(s)
		if p.Helmet != (fraction > 0) || p.Armor != 0 || knifeReferenceRemainder(t, *p) != fraction {
			t.Fatal("surviving legacy helmet ownership mismatches real armor", fraction)
		}
	}
}
