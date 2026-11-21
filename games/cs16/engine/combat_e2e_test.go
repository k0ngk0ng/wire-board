package engine

import (
	"testing"
)

func combatArena(team string) *State {
	return &State{Phase: "live", BuyEndsAt: 9000, Seed: 41, World: Map{ID: "test", Root: ContentsEmpty, BuyZones: []Bounds{{Min: Vec3{-1000 * Unit, -1000 * Unit, -1000 * Unit}, Max: Vec3{1000 * Unit, 1000 * Unit, 1000 * Unit}}}}, Players: []Player{{Seat: 0, Team: team, Alive: true, Life: 1, Health: 100, Money: 16000, Grounded: true}, {Seat: 1, Team: "ct", X: 250 * Unit, Alive: true, Life: 1, Health: 100, Grounded: true}}}
}

func TestAllTwentyFourWeaponsPurchaseFireReloadE2E(t *testing.T) {
	weapons := Weapons()
	if len(weapons) != 24 {
		t.Fatalf("gun count %d", len(weapons))
	}
	seen := map[string]bool{}
	for _, weapon := range weapons {
		if seen[weapon.ID] || weapon.Damage < 1 || weapon.Ammo < 1 || weapon.Price < 1 || weapon.Cycle < 1 || weapon.Reload < 1 {
			t.Fatalf("invalid weapon %+v", weapon)
		}
		seen[weapon.ID] = true
		team := weapon.Team
		if team == "" {
			team = "t"
		}
		state := combatArena(team)
		state.Players[1].Team = "opponent"
		InitInventory(&state.Players[0])
		DropWeapons(state, 0)
		if err := Buy(state, 0, weapon.ID); err != nil {
			t.Fatalf("%s: %v", weapon.ID, err)
		}
		p := &state.Players[0]
		p.NextAttack = 0
		p.Input = Input{Fire: true, Yaw: 0, Life: 1}
		state.Frame = 100
		before := p.Ammo
		StepCombat(state)
		if p.Ammo != before-1 {
			t.Fatalf("%s ammo %d -> %d", weapon.ID, before, p.Ammo)
		}
		fired := false
		for _, event := range state.Events {
			if event.Kind == "shot" && event.Weapon == weapon.ID {
				fired = true
			}
		}
		if !fired {
			t.Fatalf("%s missing shot", weapon.ID)
		}
		p.Input.Fire = false
		p.Previous.Fire = false
		if err := Buy(state, 0, map[bool]string{true: "primammo", false: "secammo"}[weapon.Slot == 1]); err != nil {
			t.Fatalf("%s ammo buy %v", weapon.ID, err)
		}
		p.Input.Reload = true
		state.Frame = p.NextAttack + 1
		StepCombat(state)
		p.Input.Reload = false
		state.Frame += weapon.Reload + 1000
		for range weapon.Ammo + 2 {
			StepCombat(state)
			state.Frame += 100
		}
		if p.Ammo != weapon.Ammo {
			t.Fatalf("%s reload ammo %d want %d", weapon.ID, p.Ammo, weapon.Ammo)
		}
	}
}

func TestPurchaseRestrictionsAndArmorE2E(t *testing.T) {
	state := combatArena("t")
	InitInventory(&state.Players[0])
	if Buy(state, 0, "m4a1") == nil {
		t.Fatal("T bought m4")
	}
	if err := Buy(state, 0, "vesthelm"); err != nil {
		t.Fatal(err)
	}
	if state.Players[0].Armor != 100 || !state.Players[0].Helmet {
		t.Fatal("armor not equipped")
	}
	money := state.Players[0].Money
	if Buy(state, 0, "vesthelm") == nil || state.Players[0].Money != money {
		t.Fatal("duplicate armor charged")
	}
	if Buy(state, 0, "defuser") == nil {
		t.Fatal("T bought defuser")
	}
	state.Players[0].Money = 0
	if Buy(state, 0, "ak47") == nil {
		t.Fatal("free ak47")
	}
	state.Players[0].Money = 16000
	state.Frame = state.BuyEndsAt + 1
	if Buy(state, 0, "ak47") == nil {
		t.Fatal("late buy")
	}
	state.Frame = 0
	state.Players[0].X = 2000 * Unit
	if Buy(state, 0, "ak47") == nil {
		t.Fatal("outside buyzone")
	}
}

func TestModesAndMagazineSemanticsE2E(t *testing.T) {
	for _, id := range []string{"usp", "m4a1", "glock18", "famas", "aug", "sg552", "awp", "scout", "sg550", "g3sg1"} {
		w, _ := WeaponByID(id)
		team := w.Team
		if team == "" {
			team = "t"
		}
		s := combatArena(team)
		InitInventory(&s.Players[0])
		DropWeapons(s, 0)
		if err := Buy(s, 0, id); err != nil {
			t.Fatal(err)
		}
		p := &s.Players[0]
		p.NextAttack = 0
		p.Input.Secondary = true
		StepCombat(s)
		var state WeaponState
		for _, item := range p.Inventory {
			if item.ID == id {
				state = item
			}
		}
		if !(state.Silenced || state.Burst || state.Zoom != 0) {
			t.Fatalf("%s secondary no state", id)
		}
	}
}

func TestCombatDeathAndNoDuplicateKillE2E(t *testing.T) {
	s := combatArena("t")
	InitInventory(&s.Players[0])
	if err := Buy(s, 0, "awp"); err != nil {
		t.Fatal(err)
	}
	p := &s.Players[0]
	p.NextAttack = 0
	p.Input = Input{Fire: true, Pitch: 0}
	p.Inventory[weaponIndex(p, "awp")].Zoom = 40
	s.Players[1].Z = 0
	s.Frame = 100
	StepCombat(s)
	if s.Players[1].Alive || p.Kills != 1 || s.Players[1].Deaths != 1 {
		t.Fatalf("death not settled: %+v %+v", *p, s.Players[1])
	}
	s.Frame += 200
	p.Previous.Fire = false
	StepCombat(s)
	if p.Kills != 1 || s.Players[1].Deaths != 1 {
		t.Fatal("kill repeated")
	}
}

func TestGrenadeInventoryFuseAndSmokeExpiryE2E(t *testing.T) {
	s := combatArena("t")
	InitInventory(&s.Players[0])
	for _, id := range []string{"hegrenade", "flashbang", "flashbang", "smokegrenade"} {
		if err := Buy(s, 0, id); err != nil {
			t.Fatal(err)
		}
	}
	if Buy(s, 0, "flashbang") == nil || Buy(s, 0, "hegrenade") == nil {
		t.Fatal("grenade capacity bypass")
	}
	p := &s.Players[0]
	p.Weapon = "smokegrenade"
	SyncWeapon(p)
	p.NextAttack = 0
	p.Input.Fire = true
	StepCombat(s)
	p.Previous = p.Input
	p.Input.Fire = false
	s.Frame = 50
	StepCombat(s)
	if len(s.Grenades) != 1 {
		t.Fatalf("grenade count %d", len(s.Grenades))
	}
	s.Frame = s.Grenades[0].ExplodesAt
	StepCombat(s)
	if !s.Grenades[0].Active {
		t.Fatal("smoke inactive")
	}
	s.Frame = s.Grenades[0].ExpiresAt + 1
	StepCombat(s)
	if len(s.Grenades) != 0 {
		t.Fatal("expired smoke retained")
	}
}

func TestOriginalModelHitboxesAndRotationE2E(t *testing.T) {
	loadCombatModels()
	for _, id := range []string{"terror", "leet", "arctic", "guerilla", "urban", "gsg9", "sas", "gign"} {
		model, ok := combatModels[id]
		if !ok || len(model.Hitboxes) != 21 || len(model.Poses["stand"]) < 50 || len(model.Poses["crouch"]) < 50 {
			t.Fatalf("%s original hitboxes missing", id)
		}
		for _, crouch := range []bool{false, true} {
			for _, yaw := range []int{0, 16384, 32768, 49152} {
				target := HistoryPlayer{Seat: 1, X: 100 * Unit, Z: 36 * Unit, Alive: true, Life: 1, Crouch: crouch, Yaw: yaw}
				matrix := model.Poses["stand"]
				if crouch {
					matrix = model.Poses["crouch"]
				}
				for _, box := range model.Hitboxes {
					if box.Group != 1 {
						continue
					}
					bone := matrix[box.Bone]
					local := Vec3{(box.Min.X + box.Max.X) / 2, (box.Min.Y + box.Max.Y) / 2, (box.Min.Z + box.Max.Z) / 2}
					posed := Vec3{bone.X + int((int64(bone.XX)*int64(local.X)+int64(bone.XY)*int64(local.Y)+int64(bone.XZ)*int64(local.Z))/NormalUnit), bone.Y + int((int64(bone.YX)*int64(local.X)+int64(bone.YY)*int64(local.Y)+int64(bone.YZ)*int64(local.Z))/NormalUnit), bone.Z + int((int64(bone.ZX)*int64(local.X)+int64(bone.ZY)*int64(local.Y)+int64(bone.ZZ)*int64(local.Z))/NormalUnit)}
					direction := Direction(yaw, 0)
					center := Vec3{target.X + int((int64(direction.X)*int64(posed.X)-int64(direction.Y)*int64(posed.Y))/NormalUnit), target.Y + int((int64(direction.Y)*int64(posed.X)+int64(direction.X)*int64(posed.Y))/NormalUnit), target.Z + posed.Z}
					start, end := center, center
					start.Z += 20 * Unit
					end.Z -= 20 * Unit
					_, group, hit := modelHitFraction(target, id, start, end)
					if !hit || group != 1 {
						t.Fatalf("%s crouch %t yaw %d head not hit: %d %t", id, crouch, yaw, group, hit)
					}
					break
				}
			}
		}
	}
}

func TestCounterStrikeFallDamageBoundariesE2E(t *testing.T) {
	for _, speed := range []int{500*Unit - 1, 500 * Unit, 500*Unit + 1, 506 * Unit, 1100 * Unit} {
		s := combatArena("t")
		p := &s.Players[0]
		p.Armor = 100
		p.LandingSpeed = speed
		StepCombat(s)
		want := max(0, (speed-500*Unit)*125/(600*Unit))
		if p.Health != max(0, 100-want) || p.Armor != 100 || p.LandingSpeed != 0 {
			t.Fatalf("fall speed %d health %d armor %d landing %d want damage %d", speed, p.Health, p.Armor, p.LandingSpeed, want)
		}
		health := p.Health
		StepCombat(s)
		if p.Health != health {
			t.Fatal("duplicate landing damage")
		}
	}
	s := combatArena("t")
	s.Players[0].LandingSpeed = 1100 * Unit
	s.Players[0].WaterLevel = 1
	StepCombat(s)
	if s.Players[0].Health != 100 {
		t.Fatal("water landing damaged")
	}
}

func TestMaterialPenetrationE2E(t *testing.T) {
	for _, material := range []string{"W", "M", "C"} {
		s := combatArena("t")
		s.Players[1].Z = 28 * Unit
		s.World.Planes = []Plane{{X: NormalUnit, Dist: 100 * Unit}, {X: NormalUnit, Dist: 120 * Unit}}
		s.World.Nodes = []Node{{Plane: 0, Front: 1, Back: ContentsEmpty}, {Plane: 1, Front: ContentsEmpty, Back: ContentsSolid}}
		s.World.Root = 0
		s.World.Surfaces = []Surface{{Plane: 0, Material: material, Brush: -1, Vertices: []Vec3{{100 * Unit, -512 * Unit, -512 * Unit}, {100 * Unit, 512 * Unit, -512 * Unit}, {100 * Unit, 512 * Unit, 512 * Unit}, {100 * Unit, -512 * Unit, 512 * Unit}}}}
		InitInventory(&s.Players[0])
		if err := Buy(s, 0, "awp"); err != nil {
			t.Fatal(err)
		}
		p := &s.Players[0]
		p.NextAttack = 0
		p.Inventory[weaponIndex(p, "awp")].Zoom = 40
		p.Input.Fire = true
		s.Frame = 100
		StepCombat(s)
		if material == "W" && s.Players[1].Health == 100 {
			t.Fatal("wood stopped AWP")
		}
		if material != "W" && s.Players[1].Health != 100 {
			t.Fatalf("%s 20-unit wall penetrated", material)
		}
	}
}

func TestHistoricalLifeIsolationAndMutualFireE2E(t *testing.T) {
	s := combatArena("t")
	s.Frame = 100
	s.Players[1].Life = 2
	s.Players[1].Z = 28 * Unit
	s.History = []History{{Frame: 90, Players: []HistoryPlayer{{Seat: 0, Alive: true, Life: 1}, {Seat: 1, X: s.Players[1].X, Z: 28 * Unit, Alive: true, Life: 1}}}}
	InitInventory(&s.Players[0])
	if err := Buy(s, 0, "awp"); err != nil {
		t.Fatal(err)
	}
	p := &s.Players[0]
	p.NextAttack = 0
	p.Inventory[weaponIndex(p, "awp")].Zoom = 40
	p.Input = Input{Fire: true, ViewFrame: 90}
	StepCombat(s)
	if s.Players[1].Health != 100 {
		t.Fatal("old life attack damaged new life")
	}
	s = combatArena("t")
	s.Players[1].Team = "ct"
	for seat := range s.Players {
		InitInventory(&s.Players[seat])
		s.Players[seat].Money = 16000
		if err := Buy(s, seat, "awp"); err != nil {
			t.Fatal(err)
		}
		player := &s.Players[seat]
		player.NextAttack = 0
		player.Inventory[weaponIndex(player, "awp")].Zoom = 40
		player.Input.Fire = true
	}
	s.Players[1].Yaw = 32768
	s.Frame = 100
	StepCombat(s)
	if s.Players[0].Alive || s.Players[1].Alive || s.Players[0].Kills != 1 || s.Players[1].Kills != 1 {
		t.Fatal("same-frame mutual fire lost")
	}
}

func TestReferenceWeaponParametersE2E(t *testing.T) {
	expected := []struct {
		id                                          string
		damage, clip, reserve, price, reload, armor int
		cycle                                       int64
	}{
		{"glock18", 25, 20, 120, 400, 220, 525, 150000}, {"usp", 34, 12, 100, 500, 270, 500, 150000}, {"p228", 32, 13, 52, 600, 270, 625, 150000}, {"deagle", 54, 7, 35, 650, 220, 750, 225000}, {"elite", 36, 30, 120, 800, 450, 525, 75000}, {"fiveseven", 20, 20, 100, 750, 270, 750, 150000},
		{"m3", 20, 8, 32, 1700, 45, 500, 875000}, {"xm1014", 20, 7, 32, 3000, 30, 500, 250000}, {"mp5navy", 26, 30, 120, 1500, 263, 500, 75000}, {"tmp", 20, 30, 120, 1250, 212, 500, 70000}, {"p90", 21, 50, 100, 2350, 340, 750, 66000}, {"mac10", 29, 30, 100, 1400, 315, 475, 70000}, {"ump45", 30, 25, 100, 1700, 350, 500, 100000},
		{"ak47", 36, 30, 90, 2500, 245, 775, 95500}, {"m4a1", 32, 30, 90, 3100, 305, 700, 87500}, {"galil", 30, 35, 90, 2000, 245, 775, 87500}, {"famas", 30, 25, 90, 2250, 330, 700, 82500}, {"aug", 32, 30, 90, 3500, 330, 700, 82500}, {"sg552", 33, 30, 90, 3500, 300, 700, 82500},
		{"scout", 75, 10, 90, 2750, 200, 850, 1250000}, {"awp", 115, 10, 30, 4750, 250, 975, 1450000}, {"g3sg1", 80, 20, 90, 5000, 350, 825, 250000}, {"sg550", 70, 30, 90, 4200, 335, 725, 250000}, {"m249", 32, 100, 200, 5750, 470, 750, 100000},
	}
	for _, want := range expected {
		got, ok := WeaponByID(want.id)
		if !ok || got.Damage != want.damage || got.Ammo != want.clip || got.Reserve != want.reserve || got.Price != want.price || got.Reload != int64(want.reload) || got.ArmorRatio != want.armor || got.CycleUS != want.cycle {
			t.Fatalf("%s reference mismatch %+v expected %+v", want.id, got, want)
		}
	}
}

func TestReferenceSpreadBoundariesE2E(t *testing.T) {
	p := Player{Grounded: true}
	usp, _ := WeaponByID("usp")
	item := WeaponState{ID: "usp"}
	if got := spreadFor(&p, &item, usp, 100); got != 80 {
		t.Fatalf("stationary USP spread %d", got)
	}
	p.VX = 1
	item = WeaponState{ID: "usp"}
	if got := spreadFor(&p, &item, usp, 100); got != 180 {
		t.Fatalf("moving USP spread %d", got)
	}
	awp, _ := WeaponByID("awp")
	p.VX = 11 * Unit
	item = WeaponState{ID: "awp", Zoom: 40}
	if got := spreadFor(&p, &item, awp, 100); got != 1000 {
		t.Fatalf("slow AWP spread %d", got)
	}
	p.VX = 140 * Unit
	if got := spreadFor(&p, &item, awp, 100); got != 1000 {
		t.Fatalf("boundary AWP spread %d", got)
	}
	p.VX = 140*Unit + 1
	if got := spreadFor(&p, &item, awp, 100); got != 2500 {
		t.Fatalf("fast AWP spread %d", got)
	}
	p.VX = 0
	famas, _ := WeaponByID("famas")
	item = WeaponState{ID: "famas"}
	if got := spreadFor(&p, &item, famas, 100); got != 140 {
		t.Fatalf("FAMAS auto first spread %d", got)
	}
	item = WeaponState{ID: "famas", Burst: true}
	if got := spreadFor(&p, &item, famas, 100); got != 40 {
		t.Fatalf("FAMAS burst first spread %d", got)
	}
	sg550, _ := WeaponByID("sg550")
	item = WeaponState{ID: "sg550", Zoom: 40}
	if got := spreadFor(&p, &item, sg550, 100); got != 50 {
		t.Fatalf("SG550 source initial accuracy spread %d", got)
	}
	item = WeaponState{ID: "famas", AccuracyInitialized: true, Accuracy: 0}
	if got := spreadFor(&p, &item, famas, 100); got != 100 {
		t.Fatalf("FAMAS zero reload accuracy spread %d", got)
	}
}

func TestBurstShotTimingE2E(t *testing.T) {
	s := combatArena("ct")
	InitInventory(&s.Players[0])
	if err := Buy(s, 0, "famas"); err != nil {
		t.Fatal(err)
	}
	p := &s.Players[0]
	index := weaponIndex(p, "famas")
	p.Inventory[index].Burst = true
	p.Input.Fire = true
	p.NextAttack = 0
	s.Frame = 100
	StepCombat(s)
	if p.Ammo != 24 {
		t.Fatalf("initial FAMAS burst ammo %d", p.Ammo)
	}
	p.Input.Fire = false
	s.Frame = 105
	StepCombat(s)
	if p.Ammo != 23 {
		t.Fatalf("second FAMAS burst ammo %d", p.Ammo)
	}
	s.Frame = 110
	StepCombat(s)
	if p.Ammo != 23 {
		t.Fatalf("FAMAS third shot too early %d", p.Ammo)
	}
	s.Frame = 115
	StepCombat(s)
	if p.Ammo != 22 {
		t.Fatalf("third FAMAS burst ammo %d", p.Ammo)
	}
}

func TestReferenceKickProfilesE2E(t *testing.T) {
	p := Player{Grounded: true}
	if got := recoilProfile(&p, "ak47"); got != ([7]int{10000, 3750, 1750, 375, 57500, 17500, 8}) {
		t.Fatalf("AK standing recoil %v", got)
	}
	p.VX = 1
	if got := recoilProfile(&p, "ak47"); got != ([7]int{15000, 4500, 2250, 500, 65000, 25000, 7}) {
		t.Fatalf("AK moving recoil %v", got)
	}
	p.VX = 0
	p.Grounded = false
	if got := recoilProfile(&p, "ump45"); got != ([7]int{1250, 6500, 5500, 475, 55000, 40000, 10}) {
		t.Fatalf("UMP airborne recoil %v", got)
	}
	p.Grounded = true
	p.Crouch = true
	if got := recoilProfile(&p, "famas"); got != ([7]int{5750, 3250, 2000, 110, 32500, 20000, 8}) {
		t.Fatalf("FAMAS crouch recoil %v", got)
	}
}

func TestCounterStrikeEyeAndScopedMovementE2E(t *testing.T) {
	p := Player{Z: 36 * Unit}
	if got := combatEye(&p); got.Z != 53*Unit {
		t.Fatalf("CS eye height %d", got.Z)
	}
	p.Crouch = true
	if got := combatEye(&p); got.Z != 48*Unit {
		t.Fatalf("CS crouch eye height %d", got.Z)
	}
	for _, item := range []struct {
		id    string
		speed int
	}{{"awp", 150}, {"scout", 220}, {"g3sg1", 150}, {"sg550", 150}, {"sg552", 200}, {"aug", 240}} {
		p.Weapon = item.id
		p.Inventory = []WeaponState{{ID: item.id, Zoom: 40}}
		if got := WeaponMoveSpeed(&p); got != item.speed*Unit {
			t.Fatalf("%s scope speed %d", item.id, got)
		}
		p.Inventory[0].Zoom = 0
		w, _ := WeaponByID(item.id)
		if got := WeaponMoveSpeed(&p); got != w.Speed {
			t.Fatalf("%s normal speed %d", item.id, got)
		}
	}
	for _, id := range []string{"awp", "scout", "g3sg1", "sg550"} {
		item := WeaponState{ID: id}
		secondaryMode(&item)
		secondaryMode(&item)
		want := 15
		if id == "awp" {
			want = 10
		}
		if item.Zoom != want {
			t.Fatalf("%s second FOV %d", id, item.Zoom)
		}
	}
}

func TestDeploymentAndAutomaticSelectionE2E(t *testing.T) {
	for _, item := range []struct {
		id    string
		delay int64
	}{{"ak47", 75}, {"awp", 145}, {"scout", 125}} {
		s := combatArena("t")
		InitInventory(&s.Players[0])
		s.Frame = 100
		if err := Buy(s, 0, item.id); err != nil {
			t.Fatal(err)
		}
		if s.Players[0].NextAttack != s.Frame+item.delay {
			t.Fatalf("%s draw deadline %d", item.id, s.Players[0].NextAttack)
		}
	}
	s := combatArena("t")
	InitInventory(&s.Players[0])
	if err := Buy(s, 0, "ak47"); err != nil {
		t.Fatal(err)
	}
	if err := Buy(s, 0, "hegrenade"); err != nil {
		t.Fatal(err)
	}
	p := &s.Players[0]
	p.Weapon = "hegrenade"
	p.NextAttack = 0
	p.Input.Fire = true
	StepCombat(s)
	p.Previous = p.Input
	p.Input.Fire = false
	s.Frame = 50
	StepCombat(s)
	if p.Weapon != "ak47" {
		t.Fatalf("empty grenade selected %s", p.Weapon)
	}
}

func TestHostageGunDamageAndZoomResetE2E(t *testing.T) {
	s := combatArena("t")
	s.Players[1].Y = 500 * Unit
	s.Hostages = []Hostage{{ID: 1, X: 250 * Unit, Z: -36 * Unit, Health: 100, Following: -1}}
	InitInventory(&s.Players[0])
	if err := Buy(s, 0, "awp"); err != nil {
		t.Fatal(err)
	}
	p := &s.Players[0]
	p.NextAttack = 0
	p.Inventory[weaponIndex(p, "awp")].Zoom = 40
	p.Input.Fire = true
	s.Frame = 100
	money := p.Money
	StepCombat(s)
	if s.Hostages[0].Health != 0 || p.Money != max(0, money-2500) {
		t.Fatalf("hostage shot result health %d money %d", s.Hostages[0].Health, p.Money)
	}
	p.Input = Input{Select: "glock18"}
	s.Frame = 300
	StepCombat(s)
	p.Input = Input{Select: "awp"}
	s.Frame = 400
	StepCombat(s)
	if p.Inventory[weaponIndex(p, "awp")].Zoom != 0 {
		t.Fatal("deploy retained stale scope")
	}
}
