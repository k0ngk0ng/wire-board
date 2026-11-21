package engine

import (
	"encoding/json"
	"testing"
)

func TestTenSeatConfigurationE2E(t *testing.T) {
	for _, count := range []int{2, 4, 6, 8, 10} {
		state, err := New(Options{Map: "de_dust2", RoundSeconds: 120, MaxRounds: 2}, count, 7)
		if err != nil || len(state.Players) != count {
			t.Fatalf("%d seats: %v", count, err)
		}
		teams := map[string]int{}
		for _, player := range state.Players {
			teams[player.Team]++
			if !player.Alive || player.Health != 100 || player.Life != 1 {
				t.Fatalf("invalid spawn: %+v", player)
			}
		}
		if teams["t"] > 5 || teams["ct"] > 5 {
			t.Fatalf("unbalanced teams: %v", teams)
		}
	}
	for _, count := range []int{0, 1, 11, 32} {
		if _, err := New(Options{Map: "de_dust2"}, count, 7); err == nil {
			t.Fatalf("accepted %d seats", count)
		}
	}
}

func TestClassicMapsAndDeterministicRestoreE2E(t *testing.T) {
	for _, id := range []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault"} {
		world := MapByID(id)
		if world.ID != id || len(world.Planes) == 0 || len(world.Hulls) < 2 || len(world.SpawnsT) < 5 || len(world.SpawnsCT) < 5 {
			t.Fatalf("incomplete original map %s", id)
		}
		state, err := New(Options{Map: id, RoundSeconds: 120, MaxRounds: 2}, 10, 42)
		if err != nil {
			t.Fatal(err)
		}
		Advance(state, state.PhaseEndsAt+1)
		for seat := range state.Players {
			player := state.Players[seat]
			input := Input{Seq: 1, Life: player.Life, ViewFrame: state.Frame, MoveY: 1000, Yaw: player.Yaw}
			if err := QueueInput(state, seat, input); err != nil {
				t.Fatal(err)
			}
		}
		raw, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		var restored State
		if err := json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		restored.World = world
		Advance(state, state.Frame+80)
		Advance(&restored, restored.Frame+80)
		a, _ := json.Marshal(state)
		b, _ := json.Marshal(&restored)
		if string(a) != string(b) {
			t.Fatalf("%s restore diverged", id)
		}
		for _, player := range state.Players {
			if PointContents(world.Hulls[0], Vec3{player.X, player.Y, player.Z}) == ContentsSolid {
				t.Fatalf("%s seat %d entered solid", id, player.Seat)
			}
		}
	}
}

func TestInputOrderingAndCleanupE2E(t *testing.T) {
	state, err := New(Options{Map: "de_dust2", MaxRounds: 2}, 2, 9)
	if err != nil {
		t.Fatal(err)
	}
	p := state.Players[0]
	in := Input{Seq: 1, Life: p.Life, ViewFrame: 0, Fire: true, Jump: true}
	if err := QueueInput(state, 0, in); err != nil {
		t.Fatal(err)
	}
	if err := QueueInput(state, 0, in); err == nil {
		t.Fatal("duplicate accepted")
	}
	in.Seq, in.Life = 2, p.Life+1
	if err := QueueInput(state, 0, in); err == nil {
		t.Fatal("wrong life accepted")
	}
	in.Seq, in.Life, in.ViewFrame = 2, p.Life, state.Frame+1
	if err := QueueInput(state, 0, in); err == nil {
		t.Fatal("future accepted")
	}
	ClearInput(state, 0)
	if state.Players[0].Input.Fire || len(state.Players[0].Pending) != 0 {
		t.Fatal("held input retained")
	}
	frame := state.Frame
	Advance(state, frame)
	if state.Frame != frame {
		t.Fatal("same-frame advance changed clock")
	}
}

func TestDoorStatePredictionParityE2E(t *testing.T) {
	world := Map{ID: "fixture", Root: ContentsEmpty, Brushes: []Brush{{ID: 1, Kind: "func_door", TargetName: "locked", Bounds: Bounds{Min: Vec3{50 * Unit, -100 * Unit, -100 * Unit}, Max: Vec3{60 * Unit, 100 * Unit, 200 * Unit}}}}}
	for _, open := range []bool{false, true} {
		brushes := []BrushState{{ID: 1, Open: open}}
		player := Player{Seat: 0, Team: "t", Alive: true, Health: 100, Life: 1, X: 20 * Unit, Z: 36 * Unit, VX: 250 * Unit, Weapon: "knife"}
		input := Input{Seq: 1, Life: 1, MoveY: 1000}
		predicted := PredictMotionWithBrushes(world, player, input, 20, brushes)
		state := &State{World: world, Players: []Player{player, {Seat: 1, Team: "ct", Alive: true, Health: 100, Life: 1, X: 1000 * Unit}}, Phase: "live", PhaseEndsAt: 120 * FPS, Brushes: brushes}
		if err := QueueInput(state, 0, input); err != nil {
			t.Fatal(err)
		}
		Advance(state, 20)
		authoritative := state.Players[0]
		if predicted.X != authoritative.X || predicted.Y != authoritative.Y || predicted.Z != authoritative.Z || predicted.VX != authoritative.VX || predicted.VY != authoritative.VY || predicted.VZ != authoritative.VZ {
			t.Fatalf("door=%v prediction diverged", open)
		}
		if open && predicted.X <= 50*Unit {
			t.Fatal("open door blocked prediction")
		}
		if !open && predicted.X >= 50*Unit {
			t.Fatal("closed door allowed prediction")
		}
	}
}

func TestPlayerCollisionPredictionE2E(t *testing.T) {
	world := Map{ID: "fixture", Root: ContentsEmpty}
	for _, alive := range []bool{true, false} {
		player := Player{Seat: 0, Team: "t", Alive: true, Health: 100, Life: 1, Z: 36 * Unit, VX: 250 * Unit, Weapon: "knife"}
		peer := Player{Seat: 1, Team: "ct", Alive: alive, Health: 100, Life: 1, X: 48 * Unit, Z: 36 * Unit}
		input := Input{Seq: 1, Life: 1, MoveY: 1000}
		predicted := PredictMotionWithDynamics(world, player, input, 20, nil, []Player{peer})
		if alive && predicted.X >= 17*Unit {
			t.Fatal("living player was traversed")
		}
		if !alive && predicted.X <= 48*Unit {
			t.Fatal("dead player blocked movement")
		}
		state := &State{World: world, Players: []Player{player, peer, {Seat: 2, Team: "ct", Alive: true, Health: 100, Life: 1, X: 2000 * Unit}}, Phase: "live", PhaseEndsAt: 120 * FPS, Bomb: Bomb{Planted: true, ExplodesAt: 120 * FPS}}
		if err := QueueInput(state, 0, input); err != nil {
			t.Fatal(err)
		}
		Advance(state, 20)
		if predicted.X != state.Players[0].X || predicted.Y != state.Players[0].Y || predicted.Z != state.Players[0].Z {
			t.Fatal("player collision prediction diverged")
		}
	}
}

func TestLadderClimbAndExitE2E(t *testing.T) {
	world := Map{Root: ContentsEmpty, Ladders: []Bounds{{Min: Vec3{-Unit, -Unit, 0}, Max: Vec3{Unit, Unit, 72 * Unit}}}}
	player := Player{Seat: 0, Alive: true, Health: 100, Life: 1, Z: 36 * Unit, Weapon: "knife"}
	up := PredictMotion(world, player, Input{Life: 1, MoveY: 1000}, 20)
	if up.Z <= player.Z || up.X <= player.X {
		t.Fatal("ladder climb lost horizontal exit motion")
	}
	down := PredictMotion(world, player, Input{Life: 1, MoveY: -1000}, 20)
	if down.Z >= player.Z {
		t.Fatal("ladder descent did not move down")
	}
}

func TestWaterSwimWithoutLocalDamageE2E(t *testing.T) {
	world := Map{Planes: []Plane{{}}, Root: ContentsWater, Hulls: []Hull{{Root: ContentsEmpty}, {Root: ContentsEmpty}}}
	player := Player{Seat: 0, Alive: true, Health: 100, Life: 1, Weapon: "knife"}
	one := PredictMotion(world, player, Input{Life: 1, Jump: true}, 1)
	if one.VZ != 96*Unit {
		t.Fatalf("source swim impulse %d want %d", one.VZ, 96*Unit)
	}
	up := PredictMotion(world, player, Input{Life: 1, Jump: true}, 20)
	if up.WaterLevel != 3 || up.Z <= 0 || up.Health != 100 || !up.Alive {
		t.Fatal("swim prediction could not rise safely")
	}
	idle := PredictMotion(world, player, Input{Life: 1}, 20)
	if idle.Z >= 0 || idle.VZ < -100*Unit {
		t.Fatal("idle water sink used air gravity")
	}
}

func TestOwnedWeaponSelectionPredictionE2E(t *testing.T) {
	player := Player{Seat: 0, Alive: true, Health: 100, Life: 1, Weapon: "knife", Inventory: []WeaponState{{ID: "knife", Ammo: 1}, {ID: "awp", Ammo: 10, Reserve: 30}}}
	world := Map{Root: ContentsEmpty}
	selected := PredictMotion(world, player, Input{Life: 1, Select: "awp"}, 1)
	if selected.Weapon != "awp" || selected.Ammo != 10 || selected.Reserve != 30 {
		t.Fatal("owned switch did not predict inventory display")
	}
	invalid := PredictMotion(world, player, Input{Life: 1, Select: "ak47"}, 1)
	if invalid.Weapon != "knife" {
		t.Fatal("unowned switch predicted authority")
	}
}

func TestGroundJumpUsesQuantizedOriginalImpulseE2E(t *testing.T) {
	plane := Plane{Z: NormalUnit, Dist: 36 * Unit}
	hull := Hull{Root: 0, Nodes: []Node{{Plane: 0, Front: ContentsEmpty, Back: ContentsSolid}}}
	world := Map{Planes: []Plane{plane}, Root: ContentsEmpty, Hulls: []Hull{hull, hull}}
	player := Player{Seat: 0, Alive: true, Health: 100, Life: 1, Grounded: true, Z: 36 * Unit, Weapon: "knife"}
	next := PredictMotion(world, player, Input{Life: 1, Jump: true}, 1)
	want := Sqrt(int64(2*800*45)*Unit*Unit) - 800*Unit/FPS
	if next.VZ != want || next.Z <= player.Z || next.Grounded {
		t.Fatalf("jump impulse got %d want %d", next.VZ, want)
	}
}

func TestAirDuckKeepsOriginalCenterE2E(t *testing.T) {
	world := Map{Root: ContentsEmpty}
	player := Player{Seat: 0, Alive: true, Health: 100, Life: 1, Z: 100 * Unit, Weapon: "knife"}
	duck := PredictMotion(world, player, Input{Life: 1, Crouch: true}, 1)
	if !duck.Crouch || player.Z-duck.Z >= Unit {
		t.Fatalf("air duck lowered center by %d", player.Z-duck.Z)
	}
	player.Crouch = true
	stand := PredictMotion(world, player, Input{Life: 1}, 1)
	if stand.Crouch || Abs(stand.Z-player.Z) >= Unit {
		t.Fatalf("air unduck moved center by %d", stand.Z-player.Z)
	}
}

func TestOriginalRampPrecisionRecoveryE2E(t *testing.T) {
	world := MapByID("cs_assault")
	player := Player{Seat: 0, Alive: true, Health: 100, Life: 1, Crouch: true, X: 78578, Y: 769917, Z: 88905, Weapon: "knife"}
	state := &State{World: world}
	if !TraceWorld(state, Position(player), Position(player), 1).StartSolid {
		t.Fatal("original regression position no longer exercises solid recovery")
	}
	next := PredictMotion(world, player, Input{Life: 1, Crouch: true}, 1)
	if TraceWorld(state, Position(next), Position(next), 1).StartSolid || Distance(Position(player), Position(next)) > 7*Unit {
		t.Fatal("original bounded precision recovery failed")
	}
	plane := Plane{Z: NormalUnit, Dist: 36 * Unit}
	hull := Hull{Root: 0, Nodes: []Node{{Plane: 0, Front: ContentsEmpty, Back: ContentsSolid}}}
	sealed := Map{Planes: []Plane{plane}, Root: ContentsEmpty, Hulls: []Hull{hull, hull}}
	deep := Player{Seat: 0, Alive: true, Health: 100, Life: 1, Z: 20 * Unit, Weapon: "knife"}
	stuck := PredictMotion(sealed, deep, Input{Life: 1}, 1)
	if stuck.Z != deep.Z {
		t.Fatal("recovery escaped a deep solid")
	}
}

func TestVisibleTracksSmokeOcclusionE2E(t *testing.T) {
	state := &State{Frame: 100, World: Map{Root: ContentsEmpty}, Players: []Player{{Seat: 0, Team: "t", Alive: true, X: -400 * Unit, Z: 53 * Unit}, {Seat: 1, Team: "ct", Alive: true, X: 400 * Unit, Z: 53 * Unit}}, Grenades: []Grenade{{Kind: "smokegrenade", Active: true, ExpiresAt: 1800}}}
	if Visible(state, 0, 1) {
		t.Fatal("enemy behind smoke remained visible")
	}
	state.Players[1].X = 0
	if Visible(state, 0, 1) {
		t.Fatal("enemy inside smoke remained visible")
	}
	state.Frame = 1800
	if !Visible(state, 0, 1) {
		t.Fatal("expired smoke hid enemy")
	}
}

func primedGrenadeState(t *testing.T) *State {
	t.Helper()
	state, err := New(Options{Map: "de_dust2", MaxRounds: 2}, 2, 77)
	if err != nil {
		t.Fatal(err)
	}
	Advance(state, state.PhaseEndsAt+1)
	if err := Buy(state, 0, "hegrenade"); err != nil {
		t.Fatal(err)
	}
	p := &state.Players[0]
	if err := QueueInput(state, 0, Input{Seq: 1, Life: p.Life, ViewFrame: state.Frame, Select: "hegrenade", Yaw: p.Yaw}); err != nil {
		t.Fatal(err)
	}
	Advance(state, state.Frame+76)
	p = &state.Players[0]
	if err := QueueInput(state, 0, Input{Seq: 2, Life: p.Life, ViewFrame: state.Frame, Fire: true, Yaw: p.Yaw}); err != nil {
		t.Fatal(err)
	}
	Advance(state, state.Frame+1)
	if state.Players[0].Action != "grenade:hegrenade" {
		t.Fatal("fixture did not prime grenade")
	}
	return state
}

func TestLostInputCancelsPrimedGrenadeE2E(t *testing.T) {
	state := primedGrenadeState(t)
	Advance(state, state.Frame+60)
	p := &state.Players[0]
	if p.Action != "" || len(state.Grenades) != 0 {
		t.Fatal("lost input threw a primed grenade")
	}
	for _, item := range p.Inventory {
		if item.ID == "hegrenade" && item.Ammo == 1 {
			return
		}
	}
	t.Fatal("cancelled grenade was consumed")
}

func TestReleasedGrenadeCompletesAfterInputTimeoutE2E(t *testing.T) {
	state := primedGrenadeState(t)
	p := &state.Players[0]
	if err := QueueInput(state, 0, Input{Seq: 3, Life: p.Life, ViewFrame: state.Frame, Yaw: p.Yaw}); err != nil {
		t.Fatal(err)
	}
	Advance(state, state.Frame+60)
	if len(state.Grenades) != 1 || state.Grenades[0].Kind != "hegrenade" || state.Players[0].Action != "" {
		t.Fatal("normal release was cancelled by later input timeout")
	}
	for _, item := range state.Players[0].Inventory {
		if item.ID == "hegrenade" {
			t.Fatal("normal throw did not consume grenade")
		}
	}
}
