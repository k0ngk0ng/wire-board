package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func originalRangeExpected(t *testing.T, damage, modifier, distance int) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "range-reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Rows []struct {
			Damage     int   `json:"damage"`
			Modifier   int   `json:"modifier"`
			Boundaries []int `json:"boundaries"`
		} `json:"rows"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, row := range fixture.Rows {
		if row.Damage != damage || row.Modifier != modifier {
			continue
		}
		result := 0
		for _, boundary := range row.Boundaries {
			if distance <= boundary {
				result++
			}
		}
		return result
	}
	t.Fatal("missing original range fixture", damage, modifier)
	return 0
}

func originalBallisticFixture(t *testing.T, weaponID string, distance int) (*State, combatShot, int) {
	t.Helper()
	s := combatArena("t")
	s.Players[1].Health, s.Players[1].Character = 10000, "urban"
	s.Players[1].X, s.Players[1].Z, s.Players[1].Yaw = distance*Unit, 36*Unit, 32768
	point := botChestPoint(s.Players[1])
	weapon, ok := WeaponByID(weaponID)
	if !ok {
		t.Fatal("missing gun", weaponID)
	}
	shot := combatShot{Seat: 0, Life: 1, Start: Vec3{Y: point.Y, Z: point.Z}, Weapon: weapon}
	target := HistoryPlayer{X: s.Players[1].X, Y: s.Players[1].Y, Z: s.Players[1].Z, Yaw: s.Players[1].Yaw, Alive: true, Life: 1}
	fraction, group, hit := modelHitFraction(target, "urban", shot.Start, combatAddRay(shot.Start, Direction(0, 0), weapon.Range))
	if !hit || group != 2 {
		t.Fatal("reference ray must hit the original chest model", group, hit)
	}
	return s, shot, fraction
}

func TestOriginalShotgunDamageAndMultiDamageE2E(t *testing.T) {
	for _, weapon := range []string{"m3", "xm1014"} {
		for _, armored := range []bool{false, true} {
			s, shot, fraction := originalBallisticFixture(t, weapon, 1000)
			if armored {
				s.Players[1].Armor = 100
			}
			perPellet := 20 * (NormalUnit - fraction) / NormalUnit
			want := perPellet * shot.Weapon.Pellets
			if armored {
				want /= 2
			}
			resolveGunshot(s, shot)
			if damage := 10000 - s.Players[1].Health; damage != want {
				t.Errorf("%s armor %t: actual %d original %d (%d pellets x %d)", weapon, armored, damage, want, shot.Weapon.Pellets, perPellet)
			}
		}
	}
}

func TestOriginalBurstRemainingDamageE2E(t *testing.T) {
	for _, scenario := range []struct {
		weapon    string
		secondary bool
		damage    int
		modifier  int
	}{{"glock18", false, 25, 750}, {"glock18", true, 18, 900}, {"famas", false, 34, 960}, {"famas", true, 30, 960}} {
		s, shot, fraction := originalBallisticFixture(t, scenario.weapon, 1000)
		shot.Burst, shot.Secondary = true, scenario.secondary
		distance := int(int64(shot.Weapon.Range) * int64(fraction) / NormalUnit)
		want := originalRangeExpected(t, scenario.damage, scenario.modifier, distance)
		resolveGunshot(s, shot)
		if damage := 10000 - s.Players[1].Health; damage != want {
			t.Errorf("%s remaining %t: actual %d original %d", scenario.weapon, scenario.secondary, damage, want)
		}
	}
}

func TestOriginalPlayerPenetrationDamageAndRangeE2E(t *testing.T) {
	for _, secondDistance := range []int{2000, 7400} {
		s, shot, fraction := originalBallisticFixture(t, "ak47", 1000)
		second := s.Players[1]
		second.Seat, second.X = 2, secondDistance*Unit
		s.Players = append(s.Players, second)
		firstDistance := int(int64(shot.Weapon.Range) * int64(fraction) / NormalUnit)
		firstDamage := originalRangeExpected(t, 36, 980, firstDistance)
		start := combatAddRay(shot.Start, Direction(0, 0), firstDistance+42*Unit)
		remaining := (shot.Weapon.Range - firstDistance) * 3 / 4
		target := HistoryPlayer{X: second.X, Y: second.Y, Z: second.Z, Yaw: second.Yaw, Alive: true, Life: 1}
		secondFraction, group, hit := modelHitFraction(target, "urban", start, combatAddRay(start, Direction(0, 0), remaining))
		want := 0
		if hit {
			if group != 2 {
				t.Fatal("second reference target is not chest", group)
			}
			want = originalRangeExpected(t, firstDamage*3/4, 980, int(int64(remaining)*int64(secondFraction)/NormalUnit))
		}
		resolveGunshot(s, shot)
		if damage := 10000 - s.Players[2].Health; damage != want {
			t.Errorf("second target at %d: actual %d original %d", secondDistance, damage, want)
		}
	}
}

func TestOriginalMaterialPowerQuantizationE2E(t *testing.T) {
	weapon, _ := WeaponByID("awp")
	for _, scenario := range []struct {
		material string
		power    int
		damage   int
	}{{"M", 6, 200}, {"C", 11, 500}, {"G", 22, 400}, {"V", 22, 450}, {"T", 29, 300}, {"P", 18, 450}, {"W", 45, 600}} {
		power, damage := materialPenetration(weapon, scenario.material)
		if power != scenario.power*Unit || damage != scenario.damage {
			t.Errorf("%s power %d damage %d original %d %d", scenario.material, power, damage, scenario.power*Unit, scenario.damage)
		}
	}
}

func TestOriginalVentPenetrationDamageE2E(t *testing.T) {
	s, shot, _ := originalBallisticFixture(t, "awp", 7100)
	s.World.Planes = []Plane{{X: NormalUnit, Dist: 6910 * Unit}, {X: NormalUnit, Dist: 6920 * Unit}}
	s.World.Nodes = []Node{{Plane: 0, Front: 1, Back: ContentsEmpty}, {Plane: 1, Front: ContentsEmpty, Back: ContentsSolid}}
	s.World.Root = 0
	s.World.Surfaces = []Surface{{Plane: 0, Material: "V", Brush: -1, Vertices: []Vec3{{6910 * Unit, -512 * Unit, -512 * Unit}, {6910 * Unit, 512 * Unit, -512 * Unit}, {6910 * Unit, 512 * Unit, 512 * Unit}, {6910 * Unit, -512 * Unit, 512 * Unit}}}}
	resolveGunshot(s, shot)
	if damage := 10000 - s.Players[1].Health; damage != 43 {
		t.Fatalf("vent damage %d original 43; original f32 .45 converts 100 to 44 before the next trace", damage)
	}
}

func TestOriginalShotgunBreakableDamageWaitsForMultiDamageE2E(t *testing.T) {
	s, shot, _ := originalBallisticFixture(t, "m3", 300)
	s.World.Brushes = []Brush{{ID: 1, Kind: "func_breakable", Health: 30, Bounds: Bounds{Min: Vec3{100 * Unit, -256 * Unit, 0}, Max: Vec3{110 * Unit, 256 * Unit, 100 * Unit}}}}
	s.Brushes = []BrushState{{ID: 1, Health: 30}}
	resolveGunshot(s, shot)
	if !s.Brushes[0].Open || s.Players[1].Health != 10000 {
		t.Fatalf("one shotgun discharge must finish tracing all pellets before breaking the same box: open %t health %d", s.Brushes[0].Open, s.Players[1].Health)
	}
	resolveGunshot(s, shot)
	if s.Players[1].Health != 9838 {
		t.Fatalf("next shotgun discharge should pass through the broken box: health %d want 9838", s.Players[1].Health)
	}
}
