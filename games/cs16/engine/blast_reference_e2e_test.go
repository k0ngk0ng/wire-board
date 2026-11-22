package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func originalBlastFixture() *State {
	s := objectiveFixture("bomb")
	s.World.Root, s.World.Planes, s.World.Nodes = ContentsEmpty, nil, nil
	s.Players[0].X = 10000 * Unit
	s.Players[1].Health, s.Players[1].Z = 10000, 36*Unit
	return s
}

func TestOriginalBlastDistanceAndArmorE2E(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "blast-reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Rows []struct {
			Base, Radius, Distance, InitialArmor, HealthDamage, Armor int
		} `json:"rows"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil || len(fixture.Rows) != 40 {
		t.Fatal("invalid blast reference", err, len(fixture.Rows))
	}
	for _, row := range fixture.Rows {
		s := originalBlastFixture()
		s.Players[1].X, s.Players[1].Armor = row.Distance, row.InitialArmor
		if row.Base == 100 {
			explodeHE(s, Grenade{Owner: 0, Kind: "hegrenade", Z: 35 * Unit})
		} else {
			s.Frame = 100
			s.Bomb = Bomb{Carrier: -1, Defuser: -1, Planted: true, ExplodesAt: 100, Z: 35 * Unit}
			StepObjectives(s)
		}
		if damage := 10000 - s.Players[1].Health; damage != row.HealthDamage || s.Players[1].Armor != row.Armor {
			t.Errorf("base %d distance %d armor %d: damage %d want %d armor %d want %d", row.Base, row.Distance, row.InitialArmor, damage, row.HealthDamage, s.Players[1].Armor, row.Armor)
		}
	}
}

func TestOriginalHEWallsWaterAndFractionE2E(t *testing.T) {
	for _, scenario := range []struct {
		name             string
		sourceX, targetX int
		waterLevel       int
		wall, water      bool
		damage           int
	}{
		{"through-wall", 0, 100, 0, true, false, 71},
		{"water-to-dry", 0, 100, 0, false, true, 0},
		{"water-to-partial-water", 0, 25, 1, false, true, 92},
		{"dry-to-submerged", 100, 0, 3, false, true, 0},
	} {
		s := originalBlastFixture()
		s.Players[1].X, s.Players[1].WaterLevel = scenario.targetX*Unit, scenario.waterLevel
		if scenario.wall {
			s.World.Planes = []Plane{{X: NormalUnit, Dist: 50 * Unit}, {X: NormalUnit, Dist: 60 * Unit}}
			s.World.Nodes = []Node{{Plane: 0, Front: 1, Back: ContentsEmpty}, {Plane: 1, Front: ContentsEmpty, Back: ContentsSolid}}
			s.World.Root = 0
		}
		if scenario.water {
			s.World.Planes = []Plane{{X: NormalUnit, Dist: 50 * Unit}}
			s.World.Nodes = []Node{{Plane: 0, Front: ContentsEmpty, Back: ContentsWater}}
			s.World.Root = 0
		}
		explodeHE(s, Grenade{Owner: 0, Kind: "hegrenade", X: scenario.sourceX * Unit, Z: 35 * Unit})
		if damage := 10000 - s.Players[1].Health; damage != scenario.damage {
			t.Errorf("%s damage %d original %d", scenario.name, damage, scenario.damage)
		}
	}
	s := originalBlastFixture()
	s.Players[1].X, s.Players[1].Armor = 100*Unit, 100
	for range 3 {
		explodeHE(s, Grenade{Owner: 0, Kind: "hegrenade", Z: 35 * Unit})
	}
	if s.Players[1].Health != 9895 || s.Players[1].Armor != 46 || s.Players[1].ArmorRemainder != 342858 {
		t.Errorf("repeated blast lost pre-armor fractional damage: health %d armor %d remainder %d", s.Players[1].Health, s.Players[1].Armor, s.Players[1].ArmorRemainder)
	}
}

func TestOriginalC4BlastGroundLiftE2E(t *testing.T) {
	s := objectiveFixture("bomb")
	s.Frame = 100
	s.Players[0].X = 10000 * Unit
	s.Players[1].X, s.Players[1].Health = 0, 10000
	s.Bomb = Bomb{Carrier: -1, Defuser: -1, Planted: true, ExplodesAt: 100}
	StepObjectives(s)
	if damage := 10000 - s.Players[1].Health; damage != 496 {
		t.Errorf("grounded C4 damage %d original 496", damage)
	}
}

func TestOriginalHEExplosionAndDamageShareOriginE2E(t *testing.T) {
	s := objectiveFixture("bomb")
	s.Frame = 100
	s.Players[0].X = 10000 * Unit
	s.Players[1].X, s.Players[1].Health = 0, 10000
	s.Grenades = []Grenade{{ID: 1, Owner: 0, Kind: "hegrenade", ExplodesAt: 100}}
	trace := TraceWorld(s, Vec3{Z: 8 * Unit}, Vec3{Z: -32 * Unit}, -1)
	wantZ := trace.End.Z + 76*Unit*3/5
	StepCombat(s)
	for _, event := range s.Events {
		if event.Kind == "explode" && event.Weapon == "hegrenade" {
			if event.Z != wantZ || s.Players[1].Health != 9904 {
				t.Fatalf("HE effect at %d want %d and health %d want 9904", event.Z, wantZ, s.Players[1].Health)
			}
			return
		}
	}
	t.Fatal("missing HE explosion")
}
