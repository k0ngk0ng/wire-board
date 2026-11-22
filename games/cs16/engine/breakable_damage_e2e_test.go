package engine

import (
	"encoding/json"
	"testing"
)

func breakableDamageFixture(health, material, flags int) *State {
	s := originalBlastFixture()
	s.Players[0].X, s.Players[1].X = 10000*Unit, 20000*Unit
	brush := Brush{ID: 91, Kind: "func_breakable", Material: material, Flags: flags, Health: health, Bounds: Bounds{Min: Vec3{92 * Unit, -16 * Unit, 28 * Unit}, Max: Vec3{108 * Unit, 16 * Unit, 44 * Unit}}}
	s.World.Brushes = []Brush{brush}
	s.Brushes = []BrushState{{ID: brush.ID, Health: health}}
	return s
}

func TestOriginalBreakableBlastAndBodyTargetE2E(t *testing.T) {
	for _, weapon := range []string{"hegrenade", "c4"} {
		s := breakableDamageFixture(600, 1, 0)
		if weapon == "hegrenade" {
			explodeHE(s, Grenade{Owner: 0, Z: 35 * Unit})
		} else {
			s.Frame = 100
			s.Bomb = Bomb{Carrier: -1, Defuser: -1, Planted: true, ExplodesAt: 100, Z: 35 * Unit}
			StepObjectives(s)
		}
		want := 529
		if weapon == "c4" {
			want = 129
		}
		if s.Brushes[0].Health != want || s.Brushes[0].Open || len(s.BrushDamageStates) != 1 {
			t.Fatalf("%s breakable damage health %d want %d", weapon, s.Brushes[0].Health, want)
		}
	}
	s := breakableDamageFixture(100, 0, 0)
	s.World.Brushes[0].Bounds.Min.X, s.World.Brushes[0].Bounds.Max.X = 10*Unit, 1000*Unit
	explodeHE(s, Grenade{Owner: 0, Z: 35 * Unit})
	if s.Brushes[0].Health != 100 || len(s.BrushDamageStates) != 0 {
		t.Fatal("blast measured brush origin or nearest surface instead of BodyTarget center")
	}
	s = breakableDamageFixture(100, 2, 0)
	s.World.Planes = []Plane{{X: NormalUnit, Dist: 40 * Unit}, {X: NormalUnit, Dist: 50 * Unit}}
	s.World.Nodes = []Node{{Plane: 0, Front: 1, Back: ContentsEmpty}, {Plane: 1, Front: ContentsEmpty, Back: ContentsSolid}}
	s.World.Root = 0
	explodeHEAt(s, 0, Vec3{Z: 35 * Unit})
	if s.Brushes[0].Health != 29 {
		t.Fatal("breakable blast added non-original wall LOS")
	}
}

func TestOriginalBreakableMaterialsAndKnifeFlagsE2E(t *testing.T) {
	for material := 0; material <= 9; material++ {
		for _, weapon := range []string{"knife", "ak47", "hegrenade", "c4"} {
			s := breakableDamageFixture(100, material, 256)
			damageBreakableUnits(s, 0, 0, 20*damageUnit, weapon)
			want := 80
			if material == 7 {
				want = 100
			}
			if s.Brushes[0].Health != want || s.Brushes[0].Open {
				t.Fatalf("material %d weapon %s health %d want %d", material, weapon, s.Brushes[0].Health, want)
			}
		}
	}
	s := breakableDamageFixture(10, 1, 1)
	if brushReceivesDamage(s, 0) || damageBreakableUnits(s, 0, 0, 100*damageUnit, "c4") || s.Brushes[0].Health != 10 {
		t.Fatal("trigger-only brush accepted damage")
	}
	s = breakableDamageFixture(10, 7, 0)
	if !brushReceivesDamage(s, 0) || damageBreakableUnits(s, 0, 0, 100*damageUnit, "ak47") || len(s.BrushDamageStates) != 0 {
		t.Fatal("unbreakable glass lost original TraceAttack slot or took damage")
	}
}

func TestOriginalBreakableFractionRestoreAndDeathE2E(t *testing.T) {
	s := breakableDamageFixture(1, 0, 0)
	s.World.Brushes[0].Target = "blast-door"
	s.World.Brushes = append(s.World.Brushes, Brush{ID: 92, Kind: "func_door", TargetName: "blast-door", Bounds: Bounds{Min: Vec3{300 * Unit, -16 * Unit, 0}, Max: Vec3{310 * Unit, 16 * Unit, 72 * Unit}}})
	s.Brushes = append(s.Brushes, BrushState{ID: 92, Health: 1})
	for range 2 {
		if damageBreakableUnits(s, 0, 0, damageUnit/4, "hegrenade") {
			t.Fatal("fractional damage killed breakable too early")
		}
	}
	if s.Brushes[0].Health != 1 || s.BrushDamageStates[0].HealthRemainder != 400000 {
		t.Fatal("breakable fractional health was truncated")
	}
	raw, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	restored.World = s.World
	if damageBreakableUnits(&restored, 0, 0, damageUnit/4, "hegrenade") || !damageBreakableUnits(&restored, 0, 0, damageUnit/4, "hegrenade") {
		t.Fatal("restored fractional health did not reach exact death boundary")
	}
	if !restored.Brushes[0].Open || restored.Brushes[0].Health != 0 || !restored.Brushes[1].Opening {
		t.Fatal("breakable death failed to open geometry or trigger existing target")
	}
	trace := TraceWorld(&restored, Vec3{Z: 36 * Unit}, Vec3{200 * Unit, 0, 36 * Unit}, -1)
	if trace.Fraction != NormalUnit {
		t.Fatal("dead breakable still blocked original collision")
	}
	before := len(restored.Events)
	if damageBreakableUnits(&restored, 0, 0, 100*damageUnit, "c4") || len(restored.Events) != before {
		t.Fatal("dead breakable emitted repeated death or target activation")
	}
	InitRound(&restored)
	if len(restored.BrushDamageStates) != 0 || restored.Brushes[0].Open || restored.Brushes[0].Health != 1 {
		t.Fatal("next round retained breakable damage residue or missing geometry")
	}
}

func TestOriginalKnifeDoesNotUseCrowbarBreakableMultiplierE2E(t *testing.T) {
	for _, material := range []int{1, 7} {
		s := breakableDamageFixture(1000, material, 256)
		s.World.Brushes[0].Bounds = Bounds{Min: Vec3{20 * Unit, -16 * Unit, 40 * Unit}, Max: Vec3{28 * Unit, 16 * Unit, 72 * Unit}}
		p := &s.Players[0]
		p.X, p.Weapon, p.Inventory = 0, "knife", []WeaponState{{ID: "knife", Ammo: 1}}
		classicDrive(t, s, 0, Input{Secondary: true}, 1)
		want := 935
		if material == 7 {
			want = 1000
		}
		if s.Brushes[0].Health != want || s.Brushes[0].Open {
			t.Fatal("original knife BULLET damage was treated as CLUB or bypassed material immunity", material, s.Brushes[0].Health, want)
		}
	}
}

func TestOriginalAssaultUnbreakableMaterialExportE2E(t *testing.T) {
	world := MapByID("cs_assault")
	found := 0
	for _, brush := range world.Brushes {
		if brush.ID == 120 || brush.ID == 158 || brush.ID == 159 {
			found++
			if brush.Material != 7 {
				t.Fatal("original assault bulletproof glass lost BSP material", brush.ID, brush.Material)
			}
		}
	}
	if found != 3 {
		t.Fatal("original assault bulletproof entities missing", found)
	}
}
