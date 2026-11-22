package engine

import (
	"encoding/json"
	"testing"
)

func TestHEOriginalLaunchVelocityE2E(t *testing.T) {
	for _, moving := range []bool{false, true} {
		s := combatArena("t")
		p := &s.Players[0]
		p.Inventory = []WeaponState{{ID: "hegrenade", Ammo: 1}}
		p.Weapon, p.Action, p.ActionSince = "hegrenade", "grenade:hegrenade", 1
		p.Pitch, p.Yaw = 0, 0
		if moving {
			p.VX, p.VY, p.VZ = 101*Unit, -27*Unit, 50*Unit
		}
		s.Frame = 51
		StepCombat(s)
		if len(s.Grenades) != 1 {
			t.Fatal("released HE did not spawn")
		}
		g := s.Grenades[0]
		direction := Direction(0, -65536/36)
		want := Vec3{int(int64(direction.X)*600*Unit/NormalUnit) + p.VX, int(int64(direction.Y)*600*Unit/NormalUnit) + p.VY, int(int64(direction.Z)*600*Unit/NormalUnit) + p.VZ}
		if g.VX != want.X || g.VY != want.Y || g.VZ != want.Z {
			t.Errorf("moving %t launch %v original quantized %v", moving, Vec3{g.VX, g.VY, g.VZ}, want)
		}
	}
}

func TestHEOriginalGravityDistinctFromFlashE2E(t *testing.T) {
	for _, scenario := range []struct {
		kind    string
		gravity int
	}{{"hegrenade", 440}, {"flashbang", 400}, {"smokegrenade", 400}} {
		s := combatArena("t")
		s.Grenades = []Grenade{{ID: 1, Kind: scenario.kind, Z: 1000 * Unit, ExplodesAt: 150}}
		s.Frame = 1
		StepCombat(s)
		if got, want := s.Grenades[0].VZ, -scenario.gravity*Unit/FPS; got != want {
			t.Errorf("%s vertical velocity %d original quantized %d", scenario.kind, got, want)
		}
	}
}

func TestHEHeldPinDoesNotCookFuseE2E(t *testing.T) {
	s := combatArena("t")
	p := &s.Players[0]
	p.Inventory = []WeaponState{{ID: "hegrenade", Ammo: 1}}
	p.Weapon, p.Input.Fire = "hegrenade", true
	s.Frame = 1
	StepCombat(s)
	for s.Frame < 250 {
		p.Previous = p.Input
		s.Frame++
		StepCombat(s)
	}
	if len(s.Grenades) != 0 || p.Inventory[0].Ammo != 1 {
		t.Fatal("held HE cooked or consumed before release")
	}
	p.Previous, p.Input.Fire = p.Input, false
	s.Frame++
	StepCombat(s)
	if len(s.Grenades) != 1 || s.Grenades[0].ExplodesAt != s.Frame+150 {
		t.Fatal("HE fuse did not start at throw", s.Grenades)
	}
}

func TestHEQueuedThrowExplodesAndDamagesOnceE2E(t *testing.T) {
	s := primedGrenadeState(t)
	p := &s.Players[0]
	p.Health = 1000
	s.Players[1].Health = 1000
	s.Players[1].X, s.Players[1].Y, s.Players[1].Z = p.X+100*Unit, p.Y, p.Z
	if err := QueueInput(s, 0, Input{Seq: 3, Life: p.Life, ViewFrame: s.Frame, Yaw: p.Yaw, Pitch: 16000}); err != nil {
		t.Fatal(err)
	}
	Advance(s, s.Frame+50)
	if len(s.Grenades) != 1 || weaponIndex(&s.Players[0], "hegrenade") >= 0 {
		t.Fatal("normal queued release did not consume and throw HE")
	}
	g := s.Grenades[0]
	throwFrame := g.ExplodesAt - 150
	Advance(s, g.ExplodesAt-1)
	if s.Players[1].Health != 1000 || len(s.Grenades) != 1 {
		t.Fatal("HE dealt damage or disappeared before fuse")
	}
	if s.Grenades[0].Z < p.Z-36*Unit {
		t.Fatal("HE passed through the original floor", s.Grenades[0])
	}
	Advance(s, g.ExplodesAt)
	health := s.Players[1].Health
	if health >= 1000 || health <= 900 || len(s.Grenades) != 0 {
		t.Fatal("HE did not damage nearby opponent at fuse", health, len(s.Grenades))
	}
	explosions, hits := 0, 0
	var explosion Event
	for _, event := range s.Events {
		if event.Kind == "explode" && event.Weapon == "hegrenade" {
			explosions++
			explosion = event
		}
		if event.Kind == "hit" && event.Weapon == "hegrenade" && event.Target == 1 {
			hits++
		}
	}
	Advance(s, s.Frame+100)
	if explosions != 1 || hits != 1 || explosion.Frame != g.ExplodesAt || s.Players[1].Health != health {
		t.Fatal("HE explosion event or health was missing or duplicated", explosions, hits, explosion.Frame, s.Players[1].Health)
	}
	report, err := json.Marshal(map[string]any{"throwFrame": throwFrame, "explodeFrame": explosion.Frame, "explosion": Vec3{explosion.X, explosion.Y, explosion.Z}, "opponentDamage": 1000 - health, "oneExplosion": explosions == 1, "oneHit": hits == 1, "remainingGrenades": len(s.Grenades), "map": s.MapID})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(report))
}
