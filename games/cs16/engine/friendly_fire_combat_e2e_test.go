package engine

import "testing"

func friendlyFireArena(enabled bool) *State {
	s := combatArena("t")
	s.Options.FriendlyFire = enabled
	s.Players[0].Character = "terror"
	s.Players[1].Team = "t"
	s.Players[1].Character = "terror"
	InitInventory(&s.Players[0])
	return s
}

func friendlyFireGun(t *testing.T, state *State, id string) {
	t.Helper()
	if err := Buy(state, 0, id); err != nil {
		t.Fatal(err)
	}
	p := &state.Players[0]
	p.NextAttack = 0
	p.Input = Input{Fire: true, Life: p.Life, ViewFrame: state.Frame}
	if id == "awp" {
		p.Inventory[weaponIndex(p, id)].Zoom = 40
	}
	state.Frame = 100
	StepCombat(state)
}

func TestFriendlyFireDefaultAndGunE2E(t *testing.T) {
	if (Options{}).FriendlyFire {
		t.Fatal("friendly fire default enabled")
	}
	for _, enabled := range []bool{false, true} {
		s := friendlyFireArena(enabled)
		friendlyFireGun(t, s, "ak47")
		if !enabled && s.Players[1].Health != 100 {
			t.Fatal("disabled gun team damage")
		}
		if enabled && (s.Players[1].Health >= 100 || s.Players[1].Health <= 0) {
			t.Fatalf("enabled reduced gun damage health %d", s.Players[1].Health)
		}
		if s.Players[0].Kills != 0 {
			t.Fatal("nonlethal team damage rewarded")
		}
	}
}

func TestFriendlyFireKnifeAndHEE2E(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s := friendlyFireArena(enabled)
		s.Players[1].X = 36 * Unit
		s.Players[1].Yaw = 32768
		p := &s.Players[0]
		p.Weapon = "knife"
		SyncWeapon(p)
		p.Input.Fire = true
		s.Frame = 100
		StepCombat(s)
		want := 100
		if enabled {
			want = 93
		}
		if s.Players[1].Health != want {
			t.Fatalf("knife friendlyFire %t health %d want %d", enabled, s.Players[1].Health, want)
		}
		s = friendlyFireArena(enabled)
		s.Players[0].X = 1000 * Unit
		s.Players[1].X = 100 * Unit
		s.Grenades = []Grenade{{ID: 1, Kind: "hegrenade", Owner: 0, ExplodesAt: 100}}
		s.Frame = 100
		StepCombat(s)
		want = 100
		if enabled {
			want = 29
		}
		if s.Players[1].Health != want {
			t.Fatalf("HE friendlyFire %t health %d want %d", enabled, s.Players[1].Health, want)
		}
	}
}

func TestFriendlyFireArmorAndTeamKillSettlementE2E(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s := friendlyFireArena(enabled)
		s.Players[1].Armor = 100
		DamagePlayer(s, 0, 1, 100, 775, 2, "ak47")
		wantHealth, wantArmor := 100, 100
		if enabled {
			wantHealth, wantArmor = 73, 96
		}
		if s.Players[1].Health != wantHealth || s.Players[1].Armor != wantArmor {
			t.Fatalf("armor friendlyFire %t health %d armor %d", enabled, s.Players[1].Health, s.Players[1].Armor)
		}
	}
	for _, money := range []int{1000, 5000} {
		s := friendlyFireArena(true)
		s.Players[0].Money = money
		s.Players[0].Kills = 4
		DamagePlayer(s, 0, 1, 400, 1000, 2, "ak47")
		p := s.Players[0]
		if s.Players[1].Alive || s.Players[1].Deaths != 1 || p.Kills != 3 || p.Money != max(0, money-3300) {
			t.Fatalf("team kill settlement %+v %+v", p, s.Players[1])
		}
		beforeDrops, beforeEvents := len(s.Drops), len(s.Events)
		DamagePlayer(s, 0, 1, 400, 1000, 2, "ak47")
		if s.Players[0].Kills != 3 || s.Players[0].Money != p.Money || s.Players[1].Deaths != 1 || len(s.Drops) != beforeDrops || len(s.Events) != beforeEvents {
			t.Fatal("team kill replay settled twice")
		}
	}
	s := friendlyFireArena(true)
	s.Players[1].Team = "ct"
	s.Players[0].Money = 15900
	DamagePlayer(s, 0, 1, 100, 1000, 2, "ak47")
	if s.Players[0].Kills != 1 || s.Players[0].Money != 16000 {
		t.Fatal("enemy kill reward changed")
	}
}

func TestFriendlyFireSelfWorldAndC4E2E(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s := friendlyFireArena(enabled)
		DamagePlayer(s, 0, 0, 20, 500, 0, "hegrenade")
		if s.Players[0].Health != 80 {
			t.Fatal("friendly fire setting changed self damage")
		}
		DamagePlayer(s, -1, 1, 20, 1000, 0, "fall")
		if s.Players[1].Health != 80 {
			t.Fatal("friendly fire setting changed world fall damage")
		}
		DamagePlayer(s, -1, 0, 20, 500, 0, "c4")
		DamagePlayer(s, -1, 1, 20, 500, 0, "c4")
		if s.Players[0].Health != 60 || s.Players[1].Health != 60 {
			t.Fatal("friendly fire setting changed C4")
		}
		s = friendlyFireArena(enabled)
		s.Players[0].Money = 1000
		DamagePlayer(s, 0, 0, 100, 500, 0, "hegrenade")
		if s.Players[0].Alive || s.Players[0].Kills != -1 || s.Players[0].Money != 700 {
			t.Fatal("self kill settlement changed")
		}
	}
}

func TestFriendlyFireBodyBlockingAndPenetrationE2E(t *testing.T) {
	for _, gun := range []string{"mp5navy", "ak47"} {
		base := friendlyFireArena(false)
		base.Players[1].Team = "ct"
		base.Players[1].Character = "urban"
		friendlyFireGun(t, base, gun)
		baselineHealth := base.Players[1].Health
		if baselineHealth >= 100 {
			t.Fatalf("%s baseline missed", gun)
		}
		behindHealth := -1
		for _, enabled := range []bool{false, true} {
			s := friendlyFireArena(enabled)
			s.Players[1].X = 100 * Unit
			s.Players = append(s.Players, Player{Seat: 2, Team: "ct", Character: "urban", X: 250 * Unit, Alive: true, Life: 1, Health: 100, Grounded: true})
			friendlyFireGun(t, s, gun)
			if !enabled && s.Players[1].Health != 100 {
				t.Fatal("blocking teammate was hurt with friendly fire off")
			}
			if gun == "mp5navy" && s.Players[2].Health != 100 {
				t.Fatal("low penetration bullet passed teammate")
			}
			if gun == "ak47" && (s.Players[2].Health <= baselineHealth || s.Players[2].Health >= 100) {
				t.Fatalf("penetrating AK health %d baseline %d", s.Players[2].Health, baselineHealth)
			}
			if behindHealth >= 0 && s.Players[2].Health != behindHealth {
				t.Fatal("friendly fire setting changed physical penetration")
			}
			behindHealth = s.Players[2].Health
		}
	}
}
