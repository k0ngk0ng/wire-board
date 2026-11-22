package engine

import (
	"encoding/json"
	"testing"
)

func classicArena() *State {
	s := objectiveFixture("bomb")
	s.Frame = 100
	s.Bomb = Bomb{Carrier: -1, Defuser: -1}
	for seat := range s.Players {
		InitInventory(&s.Players[seat])
	}
	return s
}

func classicDrive(t *testing.T, s *State, seat int, input Input, frames int) {
	t.Helper()
	for range frames {
		p := &s.Players[seat]
		input.Seq, input.Life, input.ViewFrame = p.ReceivedSeq+1, p.Life, s.Frame
		if err := QueueInput(s, seat, input); err != nil {
			t.Fatal(err)
		}
		Advance(s, s.Frame+1)
	}
}

func TestClassicZoneGeometryIndependentOfPurchaseDeadlineE2E(t *testing.T) {
	s := classicArena()
	s.World.BuyZones = []Bounds{{Min: Vec3{-50 * Unit, -50 * Unit, 0}, Max: Vec3{50 * Unit, 50 * Unit, 80 * Unit}, Team: "t"}}
	s.BuyEndsAt = s.Frame - 1
	if !InBuyZone(s, 0) || CanBuy(s, 0) || !InBombZone(s, 0) || InBuyZone(s, 1) {
		t.Fatal("geographic zone status incorrectly depends on purchase permission")
	}
	s.Players[0].Alive = false
	if InBuyZone(s, 0) || InBombZone(s, 0) {
		t.Fatal("dead player retains zone status")
	}
}

func TestClassicWalkingPicksUpOrdinaryDroppedWeaponE2E(t *testing.T) {
	s := classicArena()
	s.World.Mode = "elimination"
	s.Players[0].X = -100 * Unit
	s.Drops = []Drop{{ID: 1, X: 0, Z: 0, Weapon: WeaponState{ID: "ak47", Ammo: 7, Reserve: 13}}}
	classicDrive(t, s, 0, Input{MoveY: 1000}, 70)
	if weaponIndex(&s.Players[0], "ak47") < 0 || len(s.Drops) != 0 {
		t.Fatal("walking without Use did not collect the empty-slot dropped firearm")
	}
	p := &s.Players[0]
	s.Players[1].X = 2000 * Unit
	p.Input, p.Previous = Input{Drop: true}, Input{}
	StepCombat(s)
	if weaponIndex(p, "ak47") >= 0 || len(s.Drops) != 1 || !s.Drops[0].Moving {
		t.Fatal("G did not produce an airborne drop")
	}
	classicDrive(t, s, 0, Input{}, 50)
	if weaponIndex(p, "ak47") >= 0 || len(s.Drops) != 1 || s.Drops[0].Moving {
		t.Fatalf("drop was immediately reacquired or never settled: player %d %d %d drops %+v", p.X, p.Y, p.Z, s.Drops)
	}
}

func TestClassicDroppedAmmoAndGrenadeCapacityE2E(t *testing.T) {
	s := classicArena()
	p := &s.Players[0]
	p.Inventory = append(p.Inventory, WeaponState{ID: "ak47", Ammo: 30})
	p.Weapon = "ak47"
	s.Drops = []Drop{{ID: 1, X: p.X, Y: p.Y, Z: 0, Weapon: WeaponState{ID: "ak47", Ammo: 7, Reserve: 13}}}
	PickupDrop(s, 0)
	if p.Inventory[weaponIndex(p, "ak47")].Reserve != 20 || len(s.Drops) != 0 {
		t.Fatal("same firearm did not transfer useful matching ammunition")
	}
	s.Drops = []Drop{{ID: 2, Z: 0, Weapon: WeaponState{ID: "galil", Ammo: 30}}}
	PickupDrop(s, 0)
	if weaponIndex(p, "galil") >= 0 || len(s.Drops) != 1 {
		t.Fatal("occupied primary slot silently replaced its firearm")
	}
	p.Inventory = append(p.Inventory, WeaponState{ID: "flashbang", Ammo: 1})
	s.Drops = []Drop{{ID: 3, Z: 0, Weapon: WeaponState{ID: "flashbang", Ammo: 1}}, {ID: 4, Z: 0, Weapon: WeaponState{ID: "flashbang", Ammo: 1}}}
	PickupDrop(s, 0)
	PickupDrop(s, 0)
	count := 0
	for _, item := range p.Inventory {
		if item.ID == "flashbang" {
			count++
		}
	}
	if count != 2 || len(s.Drops) != 1 {
		t.Fatal("flashbang pickup does not respect original two-item capacity")
	}
}

func TestClassicHitGroupDeathRecordAndRoundResetE2E(t *testing.T) {
	s := classicArena()
	p := &s.Players[1]
	p.Health, p.VX, p.VY, p.VZ = 20, 100*Unit, -25*Unit, 0
	DamagePlayer(s, 0, 1, 34, 500, 2, "usp")
	if p.Alive || p.DeathAt != s.Frame || p.DeathHitgroup != 2 || p.DeathVX != 100*Unit || p.DeathVY != -25*Unit {
		t.Fatal("authoritative body death metadata was lost")
	}
	hit, kill := false, false
	for _, event := range s.Events {
		if event.Kind == "hit" {
			hit = event.Hitgroup == 2 && event.Value == 34
		}
		if event.Kind == "kill" {
			kill = event.Hitgroup == 2 && event.Value == 2
		}
	}
	if !hit || !kill {
		t.Fatal("damage or kill lost hit group")
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(raw, &restored); err != nil || restored.Players[1].DeathAt != s.Frame {
		t.Fatal("death record did not survive save/restore", err)
	}
	InitRound(&restored)
	if restored.Players[1].DeathAt != 0 || restored.Players[1].DeathHitgroup != 0 || restored.Players[1].DeathVX != 0 {
		t.Fatal("previous life corpse survived round reset")
	}
	s = classicArena()
	DamagePlayer(s, 0, 1, 34, 500, 2, "usp")
	if !s.Players[1].Alive || s.Players[1].Health != 66 {
		t.Fatal("ordinary body shot damage was altered to hide aim problems")
	}
}

func TestClassicKnifeAirWallAndFleshOutcomesE2E(t *testing.T) {
	for _, secondary := range []bool{false, true} {
		for _, kind := range []string{"air", "wall", "hit"} {
			s := classicArena()
			p := &s.Players[0]
			p.Weapon, p.Inventory = "knife", []WeaponState{{ID: "knife", Ammo: 1}}
			s.Players[1].X = 200 * Unit
			if kind == "hit" {
				s.Players[1].X = 35 * Unit
			}
			if kind == "wall" {
				s.World.Planes = append(s.World.Planes, Plane{X: NormalUnit, Dist: 20 * Unit})
				s.World.Nodes = append(s.World.Nodes, Node{Plane: 2, Front: ContentsSolid, Back: 0})
				s.World.Root = len(s.World.Nodes) - 1
			}
			p.Input = Input{Fire: !secondary, Secondary: secondary}
			StepCombat(s)
			outcomes := 0
			for _, event := range s.Events {
				if event.Kind == "knife_"+kind {
					outcomes++
					if (event.Value == 1) != secondary || event.Weapon != "knife" {
						t.Fatal("knife outcome lost attack identity")
					}
				}
			}
			if outcomes != 1 {
				t.Fatalf("secondary %t %s missing unique knife outcome", secondary, kind)
			}
			delay := int64(35)
			if secondary {
				delay = 100
			}
			if kind != "air" {
				delay += 5
				if secondary {
					delay = 110
				}
			}
			if p.NextAttack != s.Frame+delay {
				t.Fatal("knife outcome has the wrong source attack cooldown", kind, secondary, p.NextAttack)
			}
		}
	}
}

func TestClassicFootstepsLandingAndJumpStaminaE2E(t *testing.T) {
	s := classicArena()
	s.World.Mode = "elimination"
	classicDrive(t, s, 0, Input{MoveY: 1000}, 100)
	steps := 0
	for _, event := range s.Events {
		if event.Kind == "step" && event.Seat == 0 {
			steps++
		}
	}
	if steps < 2 || steps > 5 {
		t.Fatalf("running step clock is missing or unbounded: %d", steps)
	}
	s = classicArena()
	classicDrive(t, s, 0, Input{MoveY: 1000, Walk: true}, 100)
	for _, event := range s.Events {
		if event.Kind == "step" && event.Seat == 0 {
			t.Fatal("silent walking emitted a running step")
		}
	}
	s = classicArena()
	classicDrive(t, s, 0, Input{Jump: true, MoveY: 1000}, 1)
	if s.Players[0].Stamina <= 0 {
		t.Fatal("jump omitted source stamina slowdown")
	}
	s = classicArena()
	s.Players[0].Z, s.Players[0].Grounded = 360*Unit, false
	classicDrive(t, s, 0, Input{}, 150)
	land := 0
	for _, event := range s.Events {
		if event.Kind == "land" && event.Seat == 0 && event.Value > 500 {
			land++
		}
	}
	if s.Players[0].Health >= 100 || land != 1 {
		t.Fatal("actual falling motion did not produce one damaging landing", s.Players[0].Health, land)
	}
}

func TestClassicFlashlightEdgesAndBatteryE2E(t *testing.T) {
	s := classicArena()
	classicDrive(t, s, 0, Input{Flashlight: true}, 1)
	if !s.Players[0].Flashlight || s.Players[0].FlashlightCharge != 100 {
		t.Fatal("legacy/default flashlight did not initialize and turn on")
	}
	classicDrive(t, s, 0, Input{Flashlight: true}, 120)
	if !s.Players[0].Flashlight || s.Players[0].FlashlightCharge != 99 {
		t.Fatal("held flashlight repeated or used the wrong drain interval")
	}
	classicDrive(t, s, 0, Input{}, 1)
	classicDrive(t, s, 0, Input{Flashlight: true}, 1)
	if s.Players[0].Flashlight {
		t.Fatal("second flashlight edge did not turn off")
	}
	classicDrive(t, s, 0, Input{}, 20)
	if s.Players[0].FlashlightCharge != 100 {
		t.Fatal("flashlight did not recharge")
	}
}

func TestClassicBotReactionAimAndPureDecisionE2E(t *testing.T) {
	s := classicArena()
	p := &s.Players[0]
	p.Inventory, p.Weapon, p.Ammo = []WeaponState{{ID: "ak47", Ammo: 30, Reserve: 90}}, "ak47", 30
	s.Players[1].X = 300 * Unit
	StepBotNavigation(s)
	if BotInput(s, 0).Fire {
		t.Fatal("bot fired at an enemy in its first perception frame")
	}
	for range 29 {
		s.Frame++
		StepBotNavigation(s)
	}
	if BotInput(s, 0).Fire {
		t.Fatal("bot bypassed its 300 ms reaction delay")
	}
	s.Frame++
	StepBotNavigation(s)
	raw, _ := json.Marshal(s)
	input := BotInput(s, 0)
	after, _ := json.Marshal(s)
	if !input.Fire || string(raw) != string(after) {
		t.Fatal("focused bot failed to shoot or decision mutated authoritative state")
	}
	looks := map[[2]int]bool{}
	for range 10 {
		s.Frame += 10
		input = BotInput(s, 0)
		looks[[2]int{input.Yaw, input.Pitch}] = true
	}
	if len(looks) < 3 {
		t.Fatal("bot aim has no deterministic time-varying error")
	}
}

func TestClassicCTHearsBombAndKeepsDefuseTaskThroughCombatE2E(t *testing.T) {
	for _, kit := range []bool{false, true} {
		s := classicArena()
		s.Players[0].X, s.Players[0].Health = -150*Unit, 10000
		s.Players[1].X, s.Players[1].Yaw, s.Players[1].Defuser = -400*Unit, 32768, kit
		s.Bomb = Bomb{Carrier: -1, Defuser: -1, Planted: true, X: 400 * Unit, Z: Unit, ExplodesAt: s.Frame + 45*FPS}
		for s.Frame < 3000 && s.Phase == "live" {
			input := BotInput(s, 1)
			classicDrive(t, s, 1, input, 1)
		}
		if s.LastWinner != "ct" || s.Bomb.Defuser != 1 {
			t.Fatalf("kit %t CT did not navigate past a visible camper and naturally defuse: %s frame %d x %d", kit, s.LastWinner, s.Frame, s.Players[1].X)
		}
	}
}

func TestClassicDeathDropsAndSharedAmmoPoolE2E(t *testing.T) {
	s := classicArena()
	p := &s.Players[0]
	p.Inventory = append(p.Inventory, WeaponState{ID: "tmp", Ammo: 20, Reserve: 40})
	p.Weapon, p.VX, p.Alive = "tmp", 100*Unit, false
	DropWeapons(s, 0)
	reserve := 0
	for _, drop := range s.Drops {
		reserve += drop.Weapon.Reserve
		if drop.VX != 75*Unit || drop.X != p.X {
			t.Fatal("death drop invented a forward throw instead of source inherited velocity")
		}
	}
	if reserve != 40 {
		t.Fatal("shared 9mm pool was duplicated across death drops", reserve)
	}
	s = classicArena()
	p = &s.Players[0]
	s.Drops = []Drop{{ID: 1, Z: 0, Weapon: WeaponState{ID: "tmp", Ammo: 20, Reserve: 10}}}
	PickupDrop(s, 0)
	for _, item := range p.Inventory {
		weapon, valid := WeaponByID(item.ID)
		if valid && weapon.AmmoType == "9mm" && item.Reserve != 50 {
			t.Fatal("new firearm did not join the owned shared ammunition pool", item)
		}
	}
}

func TestClassicBombHearingAndPersistentSearchE2E(t *testing.T) {
	s := classicArena()
	s.Players[1].X = -400 * Unit
	s.Bomb = Bomb{Carrier: -1, Defuser: -1, Planted: true, X: 400 * Unit, Z: Unit, ExplodesAt: 4500}
	s.Grenades = []Grenade{{ID: 1, Kind: "smokegrenade", Active: true, X: 0, Z: 36 * Unit, ExplodesAt: 0, ExpiresAt: 1800}}
	if _, known := botBombDestination(s, 1); known {
		t.Fatal("fixture bomb should not be visible through smoke before its sound is sampled")
	}
	StepBotNavigation(s)
	if point, known := botBombDestination(s, 1); !known || point != bombPosition(s.Bomb) {
		t.Fatal("CT ignored a nearby bomb beep behind smoke")
	}
	s = classicArena()
	s.Players[1].X = -300 * Unit
	s.World.Planes = []Plane{{X: NormalUnit}, {X: NormalUnit, Dist: 100 * Unit}}
	s.World.Nodes = []Node{{Plane: 0, Front: 1, Back: ContentsEmpty}, {Plane: 1, Front: ContentsEmpty, Back: ContentsSolid}}
	s.World.Root = 0
	s.World.Bombsites = []Bounds{{Min: Vec3{-1100 * Unit, -50 * Unit, 0}, Max: Vec3{-900 * Unit, 50 * Unit, 80 * Unit}}, {Min: Vec3{1900 * Unit, -50 * Unit, 0}, Max: Vec3{2100 * Unit, 50 * Unit, 80 * Unit}}}
	s.Bomb = Bomb{Carrier: -1, Defuser: -1, Planted: true, X: 3000 * Unit, Z: Unit, ExplodesAt: 4500}
	first, _ := botBombDestination(s, 1)
	s.Frame += 500
	StepBotNavigation(s)
	if next, _ := botBombDestination(s, 1); next != first {
		t.Fatal("CT abandoned an unreached search site after five seconds")
	}
	s.Players[1].X = -1000 * Unit
	s.Frame += 10
	StepBotNavigation(s)
	if next, _ := botBombDestination(s, 1); next == first {
		t.Fatal("CT did not advance after reaching and clearing its search site")
	}
}

func TestClassicOriginalDustCTNavigatesAndDefusesE2E(t *testing.T) {
	s, err := New(Options{Map: "de_dust2", MaxRounds: 1}, 2, 71)
	if err != nil {
		t.Fatal(err)
	}
	s.Phase, s.Frame, s.PhaseEndsAt = "live", 300, 12000
	s.Players[0].Alive = false
	s.Players[1].Defuser = true
	found := false
	for _, point := range s.World.Nav {
		if objectiveInBounds(point, s.World.Bombsites[0]) {
			s.Bomb = Bomb{Planted: true, Carrier: -1, Defuser: -1, X: point.X, Y: point.Y, Z: point.Z - 35*Unit, ExplodesAt: 4800}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("original bombsite lacks a reachable nav point")
	}
	for s.Frame < 4800 && s.Phase == "live" {
		before := Position(s.Players[1])
		classicDrive(t, s, 1, BotInput(s, 1), 1)
		if Distance(before, Position(s.Players[1])) > 64*Unit {
			t.Fatal("CT was teleported toward the planted bomb")
		}
	}
	if s.LastWinner != "ct" || s.Bomb.Defuser != 1 {
		t.Fatalf("original Dust CT did not reach and defuse, frame %d goal %+v pos %+v", s.Frame, s.Players[1].BotGoal, Position(s.Players[1]))
	}
}

func TestClassicDefuserDeathDropAndCTOnlyPickupE2E(t *testing.T) {
	s := classicArena()
	dead := &s.Players[1]
	dead.X, dead.Defuser, dead.Alive, dead.Inventory = 0, true, false, nil
	DropWeapons(s, 1)
	if dead.Defuser || len(s.Drops) != 1 || s.Drops[0].Weapon.ID != "defuser" {
		t.Fatal("dead CT did not drop its original thigh-pack equipment")
	}
	for range 50 {
		s.Frame++
		stepDroppedWeapons(s)
	}
	p := &s.Players[0]
	p.X = 0
	PickupDrop(s, 0)
	if p.Defuser || len(s.Drops) != 1 {
		t.Fatal("T could collect CT-only defuse equipment")
	}
	p.Team = "ct"
	PickupDrop(s, 0)
	if !p.Defuser || len(s.Drops) != 0 {
		t.Fatal("empty CT did not collect the dropped equipment")
	}
	s.Drops = []Drop{{ID: 900, Z: 0, Weapon: WeaponState{ID: "defuser", Ammo: 1}}}
	PickupDrop(s, 0)
	if len(s.Drops) != 1 {
		t.Fatal("already equipped CT consumed another kit")
	}
}

func TestClassicBombMovementRotationStaysInsideInputContractE2E(t *testing.T) {
	s := classicArena()
	p := &s.Players[1]
	p.X, p.Y, p.Yaw = 0, 0, 0
	p.BotBombKnown, p.BotBombPoint = true, Vec3{500 * Unit, 0, Unit}
	s.Bomb = Bomb{Planted: true, Carrier: -1, Defuser: -1, X: 500 * Unit, Z: Unit, ExplodesAt: 4500}
	s.World.Brushes = []Brush{{ID: 1, Kind: "func_door", Bounds: Bounds{Min: Vec3{20 * Unit, -100 * Unit, 0}, Max: Vec3{30 * Unit, 100 * Unit, 100 * Unit}}}}
	s.Brushes = []BrushState{{ID: 1}}
	for _, yaw := range []int{8192, 0, 1, 4096, 12288, 16384, 24576, 32768, 40960, 49152, 57344, 65535} {
		input := botKeepBombMovement(s, 1, Input{Seq: p.ReceivedSeq + 1, Life: p.Life, ViewFrame: s.Frame, Yaw: yaw, Pitch: -1024, Fire: true, Select: "usp"})
		if err := QueueInput(s, 1, input); err != nil {
			t.Fatalf("rotated diagonal yaw=%d rejected input %+v: %v", yaw, input, err)
		}
		if Length2D(input.MoveX, input.MoveY) > 1000 || input.Yaw != yaw || input.Pitch != -1024 || !input.Fire || !input.Use || input.Select != "usp" {
			t.Fatalf("rotation changed direction magnitude or nonmovement intent: %+v", input)
		}
		if yaw == 8192 && (input.MoveX != 0 || input.MoveY != 1000) {
			t.Fatalf("45 degree diagonal did not normalize proportionally: %+v", input)
		}
	}
}

func TestClassicPickupEventsCarryActualCoordinatesE2E(t *testing.T) {
	for _, id := range []string{"ak47", "defuser"} {
		t.Run(id, func(t *testing.T) {
			s := classicArena()
			p := &s.Players[0]
			p.Team, p.X, p.Y = "ct", 450*Unit, -120*Unit
			s.Drops = []Drop{{ID: 1, X: p.X, Y: p.Y, Z: 0, Weapon: WeaponState{ID: id, Ammo: 1}}}
			PickupDrop(s, 0)
			if len(s.Events) == 0 {
				t.Fatal("pickup did not emit an event")
			}
			event := s.Events[len(s.Events)-1]
			if event.Kind != "pickup" || event.Weapon != id || event.X != p.X || event.Y != p.Y || event.Z != p.Z {
				t.Fatalf("pickup spatial event lost the actual player position: %+v", event)
			}
		})
	}
}

func TestClassicDeathImpulseSourceCasesAndNoUniversalBodyLaunchE2E(t *testing.T) {
	for _, group := range []int{2, 3, 4, 6, 7} {
		s := classicArena()
		p := &s.Players[1]
		p.Health, p.VX, p.VY, p.VZ = 1, 100*Unit, -25*Unit, -40*Unit
		DamagePlayer(s, 0, 1, 100, 1000, group, "ak47")
		if p.DeathVX != 100*Unit || p.DeathVY != -25*Unit || p.DeathVZ != -40*Unit || p.VX != 100*Unit || p.VY != -25*Unit || p.VZ != -40*Unit {
			t.Fatal("ordinary body death acquired an invented launch", group, p.DeathVX, p.DeathVY, p.DeathVZ)
		}
	}
	variants := map[string]bool{}
	for seed := uint64(1); seed <= 64; seed++ {
		s := classicArena()
		s.Seed = seed
		s.Players[0].Yaw = 16384
		p := &s.Players[1]
		DamagePlayer(s, 0, 1, 34, 1000, 1, "usp")
		if p.Alive || p.DeathHitgroup != 1 || s.Seed != seed {
			t.Fatal("head impulse changed damage, attribution or weapon randomness")
		}
		switch {
		case p.DeathVX == 0 && p.DeathVY == 0 && p.DeathVZ == 0:
			variants["none"] = true
		case p.DeathVX >= 100*Unit && p.DeathVX <= 200*Unit && p.DeathVY == 0 && p.DeathVZ >= 50*Unit && p.DeathVZ <= 100*Unit:
			variants["forward"] = true
		case p.DeathVX >= -200*Unit && p.DeathVX <= -100*Unit && p.DeathVY == 0 && p.DeathVZ >= 50*Unit && p.DeathVZ <= 100*Unit:
			variants["backward"] = true
		case p.DeathVX == 0 && p.DeathVY >= 200*Unit && p.DeathVY <= 300*Unit && p.DeathVZ >= 200*Unit && p.DeathVZ <= 300*Unit:
			variants["hitvel"] = true
		default:
			t.Fatal("head death escaped the source throw cases", p.DeathVX, p.DeathVY, p.DeathVZ)
		}
	}
	if len(variants) != 4 {
		t.Fatal("head deaths omit original varied launch/no-launch outcomes", variants)
	}
	armVariants := map[bool]bool{}
	for seed := uint64(1); seed <= 16; seed++ {
		s := classicArena()
		s.Seed, s.Players[0].Yaw = seed, 16384
		p := &s.Players[1]
		p.Health = 1
		DamagePlayer(s, 0, 1, 34, 1000, 5, "usp")
		lift := p.DeathVZ >= 200*Unit && p.DeathVZ <= 300*Unit
		if p.DeathVX != 0 || p.DeathVY < 200*Unit || p.DeathVY > 300*Unit || !lift && p.DeathVZ != 0 {
			t.Fatal("right arm death disagrees with source lift/no-lift cases", p.DeathVX, p.DeathVY, p.DeathVZ)
		}
		armVariants[lift] = true
	}
	if len(armVariants) != 2 {
		t.Fatal("right arm deaths omitted a source variant")
	}
	for _, weapon := range []string{"ak47", "c4", "hegrenade"} {
		s := classicArena()
		p := &s.Players[1]
		p.Health, p.Crouch = 1, true
		DamagePlayer(s, 0, 1, 100, 1000, 1, weapon)
		if p.DeathVX < -200*Unit || p.DeathVX > -100*Unit || p.DeathVY != 0 || p.DeathVZ < 50*Unit || p.DeathVZ > 100*Unit {
			t.Fatal("crouch did not override other death throws", weapon, p.DeathVX, p.DeathVY, p.DeathVZ)
		}
	}
	s := classicArena()
	p := &s.Players[1]
	p.Health, p.VX = 1000, 100*Unit
	DamagePlayer(s, 0, 1, 34, 1000, 1, "usp")
	if !p.Alive || p.DeathAt != 0 || p.DeathVX != 0 || p.VX != 100*Unit {
		t.Fatal("nonlethal hit acquired corpse impulse")
	}
}

func TestClassicExplosionDeathImpulseUsesActualInflictorE2E(t *testing.T) {
	s := classicArena()
	s.Players[0].X = 1000 * Unit
	p := &s.Players[1]
	p.X, p.Health = 100*Unit, 1
	explodeHE(s, Grenade{Owner: 0, Kind: "hegrenade", X: 0, Y: 0, Z: 36 * Unit})
	if p.Alive || p.DeathVX != 400*Unit || p.DeathVY != 0 || p.DeathVZ != 375*Unit {
		t.Fatal("HE corpse used the thrower instead of real grenade origin", p.DeathVX, p.DeathVY, p.DeathVZ)
	}
	s = classicArena()
	p = &s.Players[1]
	p.X, p.Health = 100*Unit, 1
	s.Bomb.X, s.Bomb.Y, s.Bomb.Z = 0, 0, 36*Unit
	DamagePlayer(s, -1, 1, 200, 1000, 0, "c4")
	if p.DeathVX != 550*Unit || p.DeathVY != 0 || p.DeathVZ != 800*Unit {
		t.Fatal("C4 corpse impulse disagrees with source blast distance", p.DeathVX, p.DeathVY, p.DeathVZ)
	}
	s = classicArena()
	p = &s.Players[1]
	p.Health = 1
	s.Bomb.X, s.Bomb.Y, s.Bomb.Z = p.X, p.Y, p.Z
	DamagePlayer(s, -1, 1, 200, 1000, 0, "c4")
	if p.DeathVX != 0 || p.DeathVY != 0 || p.DeathVZ < 0 || p.DeathVZ > 837*Unit {
		t.Fatal("zero-distance blast produced an invalid corpse vector")
	}
}
