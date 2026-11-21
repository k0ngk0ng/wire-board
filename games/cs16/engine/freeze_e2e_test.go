package engine

import (
	"encoding/json"
	"testing"
)

func freezeScenario() *State {
	s := combatArena("t")
	s.Phase = "freeze"
	s.PhaseEndsAt = 10000
	s.Options = Options{Map: "de_dust2", RoundSeconds: 120, MaxRounds: 2}
	s.Round = 1
	InitInventory(&s.Players[0])
	return s
}

func freezeCommand(t *testing.T, state *State, input Input) {
	t.Helper()
	p := state.Players[0]
	input.Seq = p.ReceivedSeq + 1
	input.Life = p.Life
	input.ViewFrame = state.Frame
	if err := QueueInput(state, 0, input); err != nil {
		t.Fatal(err)
	}
	Advance(state, state.Frame+1)
	if state.Players[0].AppliedSeq != input.Seq {
		t.Fatal("freeze input was not applied")
	}
}

func TestFreezeInventoryCommandsE2E(t *testing.T) {
	s := freezeScenario()
	if err := Buy(s, 0, "ak47"); err != nil {
		t.Fatal(err)
	}
	freezeCommand(t, s, Input{Select: "knife"})
	p := &s.Players[0]
	if p.Weapon != "knife" {
		t.Fatalf("freeze selected %s", p.Weapon)
	}
	freezeCommand(t, s, Input{Select: "ak47"})
	index := weaponIndex(p, "ak47")
	p.Inventory[index].Ammo = 25
	p.Inventory[index].Reserve = 10
	SyncWeapon(p)
	Advance(s, p.NextAttack)
	freezeCommand(t, s, Input{Reload: true})
	deadline := p.Inventory[index].ReloadUntil
	if deadline <= s.Frame {
		t.Fatal("freeze reload did not start")
	}
	Advance(s, deadline)
	if p.Ammo != 30 || p.Reserve != 5 {
		t.Fatalf("freeze reload ammo %d reserve %d", p.Ammo, p.Reserve)
	}
	freezeCommand(t, s, Input{Select: "glock18"})
	Advance(s, p.NextAttack)
	freezeCommand(t, s, Input{Secondary: true})
	if !p.Inventory[weaponIndex(p, "glock18")].Burst {
		t.Fatal("freeze secondary was consumed")
	}
	freezeCommand(t, s, Input{Select: "ak47"})
	freezeCommand(t, s, Input{Drop: true})
	if p.Weapon != "glock18" || len(s.Drops) != 1 {
		t.Fatal("freeze drop failed")
	}
	freezeCommand(t, s, Input{Use: true})
	if p.Weapon != "ak47" || len(s.Drops) != 0 {
		t.Fatal("freeze pickup failed")
	}
}

func TestFreezePreventsAttackAndGrenadesE2E(t *testing.T) {
	s := freezeScenario()
	p := &s.Players[0]
	index := weaponIndex(p, "glock18")
	p.Inventory[index].Burst = true
	p.Inventory[index].BurstLeft = 2
	p.Inventory[index].BurstAt = 0
	p.LandingSpeed = 1100 * Unit
	s.Grenades = []Grenade{{ID: 1, Kind: "hegrenade", Owner: 1, ExplodesAt: 0}}
	freezeCommand(t, s, Input{Fire: true})
	if p.Health != 100 || p.Ammo != 20 || s.Grenades[0].Active {
		t.Fatal("freeze applied damage or fire")
	}
	if err := Buy(s, 0, "hegrenade"); err != nil {
		t.Fatal(err)
	}
	freezeCommand(t, s, Input{Select: "hegrenade"})
	Advance(s, p.NextAttack)
	freezeCommand(t, s, Input{Fire: true})
	freezeCommand(t, s, Input{})
	if p.Action != "" || len(s.Grenades) != 1 {
		t.Fatal("freeze pulled or threw grenade")
	}
	for _, event := range s.Events {
		switch event.Kind {
		case "shot", "explode", "grenade_pin", "throw", "hit", "kill":
			t.Fatalf("freeze emitted %s", event.Kind)
		}
	}
}

func TestRoundEndInventoryRemainsClosedE2E(t *testing.T) {
	s := freezeScenario()
	s.Phase = "round_end"
	p := &s.Players[0]
	p.Input = Input{Select: "knife", Reload: true, Secondary: true, Fire: true, Drop: true, Use: true}
	p.LandingSpeed = 1100 * Unit
	s.Grenades = []Grenade{{ID: 1, Kind: "hegrenade", ExplodesAt: 0}}
	before, _ := json.Marshal(s)
	StepCombat(s)
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("round end processed combat or inventory")
	}
}

func TestRotatingDoorMaterialDirectionE2E(t *testing.T) {
	s := combatArena("t")
	s.World.Planes = []Plane{{X: NormalUnit}}
	s.World.Brushes = []Brush{{ID: 9, Kind: "func_door_rotating", Angle: 16384}}
	s.World.Surfaces = []Surface{{Plane: 0, Material: "W", Brush: 9, Vertices: []Vec3{{0, 0, 0}, {0, 10 * Unit, 0}, {0, 10 * Unit, 10 * Unit}, {0, 0, 10 * Unit}}}}
	s.Brushes = []BrushState{{ID: 9, Progress: 1000, Direction: -1}}
	if got := combatMaterial(s, TraceResult{Brush: 0, Plane: 0, End: Vec3{5 * Unit, 0, 5 * Unit}}); got != "W" {
		t.Fatalf("reverse door material %s", got)
	}
	s.Brushes[0].Direction = 0
	if got := combatMaterial(s, TraceResult{Brush: 0, Plane: 0, End: Vec3{-5 * Unit, 0, 5 * Unit}}); got != "W" {
		t.Fatalf("default door material %s", got)
	}
}
