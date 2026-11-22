package engine

import (
	"encoding/json"
	"testing"
)

func originalHostageDamageFixture(t *testing.T, group int) (*State, combatShot) {
	t.Helper()
	s := combatArena("t")
	s.Players[1].X = 10000 * Unit
	s.World.Hostages = []Spawn{{X: 450 * Unit}}
	s.Hostages = []Hostage{{ID: 1, X: 450 * Unit, Health: 100, Following: -1}}
	loadCombatModels()
	model := combatModels["hostage"]
	for _, box := range model.Hitboxes {
		if box.Group != group {
			continue
		}
		bone := model.Poses["stand"][box.Bone]
		local := Vec3{(box.Min.X + box.Max.X) / 2, (box.Min.Y + box.Max.Y) / 2, (box.Min.Z + box.Max.Z) / 2}
		point := Vec3{450*Unit + bone.X + int((int64(bone.XX)*int64(local.X)+int64(bone.XY)*int64(local.Y)+int64(bone.XZ)*int64(local.Z))/NormalUnit), bone.Y + int((int64(bone.YX)*int64(local.X)+int64(bone.YY)*int64(local.Y)+int64(bone.YZ)*int64(local.Z))/NormalUnit), bone.Z + int((int64(bone.ZX)*int64(local.X)+int64(bone.ZY)*int64(local.Y)+int64(bone.ZZ)*int64(local.Z))/NormalUnit)}
		start := point
		start.X -= 450 * Unit
		weapon, _ := WeaponByID("glock18")
		shot := combatShot{Seat: 0, Life: 1, Start: start, Weapon: weapon}
		index, fraction, actualGroup := nearestHostageHit(s, start, combatAddRay(start, Direction(0, 0), weapon.Range), map[int]bool{})
		if index != 0 || actualGroup != group {
			continue
		}
		if damage := originalRangeExpected(t, 25, 750, int(int64(weapon.Range)*int64(fraction)/NormalUnit)); damage != 19 {
			t.Fatal("hostage reference ray must receive an attenuated 19-point bullet", damage)
		}
		return s, shot
	}
	t.Fatal("original hostage hitbox group is unreachable", group)
	return nil, combatShot{}
}

func TestOriginalHostageHeadAndLegDamageE2E(t *testing.T) {
	s, shot := originalHostageDamageFixture(t, 1)
	resolveGunshot(s, shot)
	if s.Hostages[0].Health != 0 || s.Players[0].Money != 13500 {
		t.Errorf("hostage head must apply TraceAttack x3 then TakeDamage x2.5 and clamp: health %d money %d", s.Hostages[0].Health, s.Players[0].Money)
	}
	s, shot = originalHostageDamageFixture(t, 6)
	for index, health := range []int{92, 83, 75, 66, 58, 49, 41, 32, 24, 15, 6, 0} {
		resolveGunshot(s, shot)
		if s.Hostages[0].Health != health {
			t.Errorf("hostage leg shot %d health %d want %d; original per-shot damage 19 x .75 x .6", index+1, s.Hostages[0].Health, health)
		}
		if index == 3 {
			raw, err := json.Marshal(s)
			if err != nil || json.Unmarshal(raw, s) != nil {
				t.Fatal("hostage damage state did not restore", err)
			}
		}
	}
	if s.Players[0].Money != 13640 {
		t.Errorf("hostage fractional clamping penalty %d want 13640", s.Players[0].Money)
	}
	InitRound(s)
	if len(s.Hostages) != 1 || s.Hostages[0].Health != 100 {
		t.Fatal("new round retained hostage health debt")
	}
}

func TestOriginalHostageBlastUsesLastGroupWithoutTraceMultiplierE2E(t *testing.T) {
	s, shot := originalHostageDamageFixture(t, 6)
	resolveGunshot(s, shot)
	explodeHE(s, Grenade{Owner: 0, Kind: "hegrenade", X: 275 * Unit, Z: -Unit})
	if s.Hostages[0].Health != 62 || s.Players[0].Money != 15260 {
		t.Fatal("HE must retain leg TakeDamage modifier without applying another TraceAttack multiplier", s.Hostages[0].Health, s.Players[0].Money)
	}
}

func TestOriginalHostageShotgunAccumulatesBeforeFinalModifierE2E(t *testing.T) {
	for _, scenario := range []struct {
		weapon string
		health int
		money  int
	}{{"m3", 32, 14640}, {"xm1014", 55, 15100}} {
		s, shot := originalHostageDamageFixture(t, 6)
		shot.Weapon, _ = WeaponByID(scenario.weapon)
		resolveGunshot(s, shot)
		if s.Hostages[0].Health != scenario.health || s.Players[0].Money != scenario.money {
			t.Errorf("%s hostage health %d money %d original %d %d", scenario.weapon, s.Hostages[0].Health, s.Players[0].Money, scenario.health, scenario.money)
		}
	}
}
