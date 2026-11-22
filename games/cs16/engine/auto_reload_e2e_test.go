package engine

import (
	"encoding/json"
	"testing"
)

func autoReloadArena(id string, ammo, reserve int) *State {
	s := classicArena()
	s.World.Mode, s.PhaseEndsAt = "elimination", 0
	p := &s.Players[0]
	p.Inventory = []WeaponState{{ID: "knife", Ammo: 1}, {ID: id, Ammo: ammo, Reserve: reserve}}
	if id == "knife" {
		p.Inventory = p.Inventory[1:]
	}
	p.Weapon, p.NextAttack = id, s.Frame
	s.Players[1].Y = 2000 * Unit
	SyncWeapon(p)
	return s
}

func autoReloadCount(s *State, id string) int {
	count := 0
	for _, event := range s.Events {
		if event.Kind == "reload" && event.Weapon == id {
			count++
		}
	}
	return count
}

func TestAutoReloadAllTwentyFourWeaponsE2E(t *testing.T) {
	for _, weapon := range Weapons() {
		t.Run(weapon.ID, func(t *testing.T) {
			s := autoReloadArena(weapon.ID, 0, weapon.Ammo)
			classicDrive(t, s, 0, Input{}, 1)
			p := &s.Players[0]
			index := weaponIndex(p, weapon.ID)
			until := p.Inventory[index].ReloadUntil
			if until <= s.Frame || p.Inventory[index].Zoom != 0 || autoReloadCount(s, weapon.ID) != 1 {
				t.Fatal("idle empty weapon did not start one automatic reload", p.Inventory[index])
			}
			classicDrive(t, s, 0, Input{}, int(until-s.Frame-1))
			if p.Ammo != 0 || autoReloadCount(s, weapon.ID) != 1 {
				t.Fatal("automatic reload duplicated or granted ammunition early", p.Ammo)
			}
			classicDrive(t, s, 0, Input{}, 1)
			if weapon.Pellets > 1 {
				classicDrive(t, s, 0, Input{}, int(weapon.Reload)*weapon.Ammo)
			}
			if p.Ammo != weapon.Ammo || p.Reserve != 0 || p.Inventory[index].ReloadUntil != 0 {
				t.Fatal("automatic reload did not complete within available ammunition", p.Inventory[index])
			}
		})
	}
}

func TestAutoReloadOrdinaryWeaponsRequireReleasedAttacksE2E(t *testing.T) {
	for _, weapon := range Weapons() {
		if weapon.Pellets > 1 {
			continue
		}
		t.Run(weapon.ID, func(t *testing.T) {
			s := autoReloadArena(weapon.ID, 1, weapon.Ammo)
			classicDrive(t, s, 0, Input{Fire: true}, 101)
			if s.Players[0].Ammo != 0 || autoReloadCount(s, weapon.ID) != 0 {
				t.Fatal("ordinary weapon reloaded while primary attack was held")
			}
			classicDrive(t, s, 0, Input{}, 400)
			if autoReloadCount(s, weapon.ID) != 1 {
				t.Fatal("releasing primary did not start automatic reload")
			}
		})
	}
	s := autoReloadArena("ak47", 0, 30)
	classicDrive(t, s, 0, Input{Secondary: true}, 50)
	if autoReloadCount(s, "ak47") != 0 {
		t.Fatal("held secondary attack allowed automatic reload")
	}
	classicDrive(t, s, 0, Input{}, 1)
	if autoReloadCount(s, "ak47") != 1 {
		t.Fatal("releasing both attacks did not start automatic reload")
	}
}

func TestAutoReloadCooldownAndInventoryGuardsE2E(t *testing.T) {
	s := autoReloadArena("ak47", 0, 30)
	s.Players[0].NextAttack = 102
	for _, expected := range []int{0, 0, 1} {
		classicDrive(t, s, 0, Input{}, 1)
		if autoReloadCount(s, "ak47") != expected {
			t.Fatal("automatic reload violated strict cooldown boundary", s.Frame)
		}
	}
	for _, sample := range []struct {
		id      string
		ammo    int
		reserve int
	}{{"ak47", 0, 0}, {"ak47", 1, 30}, {"knife", 0, 1}, {"hegrenade", 0, 1}, {"c4", 0, 1}} {
		t.Run(sample.id, func(t *testing.T) {
			s := autoReloadArena(sample.id, sample.ammo, sample.reserve)
			classicDrive(t, s, 0, Input{}, 10)
			if autoReloadCount(s, sample.id) != 0 || s.Players[0].Ammo != sample.ammo || s.Players[0].Reserve != sample.reserve {
				t.Fatal("invalid automatic reload changed inventory", s.Players[0].Inventory)
			}
		})
	}
}

func TestAutoReloadShotgunsWhilePrimaryHeldE2E(t *testing.T) {
	for _, id := range []string{"m3", "xm1014"} {
		t.Run(id, func(t *testing.T) {
			s := autoReloadArena(id, 0, 4)
			classicDrive(t, s, 0, Input{Fire: true}, 1)
			p := &s.Players[0]
			until := p.Inventory[weaponIndex(p, id)].ReloadUntil
			if until <= s.Frame || autoReloadCount(s, id) != 1 {
				t.Fatal("empty shotgun primary attack did not begin reload")
			}
			classicDrive(t, s, 0, Input{Fire: true}, int(until-s.Frame))
			if p.Reserve != 3 || p.Ammo != 0 {
				t.Fatal("held primary did not fire the first inserted shell", p.Inventory)
			}
			shots := 0
			for _, event := range s.Events {
				if event.Kind == "shot" && event.Weapon == id {
					shots++
				}
			}
			if shots != 1 {
				t.Fatal("shotgun did not interrupt loading to shoot exactly once", shots)
			}
		})
	}
}

func TestAutoReloadShotgunsUnderwaterE2E(t *testing.T) {
	for _, id := range []string{"m3", "xm1014"} {
		t.Run(id, func(t *testing.T) {
			s := autoReloadArena(id, 0, 4)
			s.World.Root = ContentsWater
			classicDrive(t, s, 0, Input{Fire: true}, 1)
			if s.Players[0].WaterLevel != 3 || autoReloadCount(s, id) != 0 {
				t.Fatal("underwater forbidden primary attack began reload")
			}
			classicDrive(t, s, 0, Input{}, 100)
			if autoReloadCount(s, id) != 1 {
				t.Fatal("released underwater weapon did not reload")
			}
		})
	}
}

func TestAutoReloadBurstSwitchDeathFreezeAndRecoveryE2E(t *testing.T) {
	t.Run("burst", func(t *testing.T) {
		s := autoReloadArena("famas", 0, 25)
		item := &s.Players[0].Inventory[1]
		item.Burst, item.BurstLeft, item.BurstAt = true, 2, 110
		classicDrive(t, s, 0, Input{}, 9)
		if item.ReloadUntil != 0 {
			t.Fatal("pending burst was interrupted by reload")
		}
		classicDrive(t, s, 0, Input{}, 12)
		if item.BurstLeft != 0 || item.ReloadUntil <= s.Frame {
			t.Fatal("empty weapon did not reload after pending burst drained", *item)
		}
	})
	t.Run("switch", func(t *testing.T) {
		s := autoReloadArena("ak47", 0, 30)
		s.Players[0].Inventory = append(s.Players[0].Inventory, WeaponState{ID: "deagle", Reserve: 7})
		classicDrive(t, s, 0, Input{}, 1)
		classicDrive(t, s, 0, Input{Select: "deagle"}, 1)
		p := &s.Players[0]
		if p.Inventory[1].ReloadUntil != 0 || p.Inventory[1].Ammo != 0 || p.Inventory[2].ReloadUntil != 0 {
			t.Fatal("switch retained old reload or skipped deployment cooldown", p.Inventory)
		}
		classicDrive(t, s, 0, Input{}, int(p.NextAttack-s.Frame))
		if p.Inventory[2].ReloadUntil != 0 {
			t.Fatal("new weapon reloaded at deployment boundary")
		}
		classicDrive(t, s, 0, Input{}, 1)
		if p.Inventory[2].ReloadUntil <= s.Frame {
			t.Fatal("new empty weapon did not reload after deployment")
		}
	})
	t.Run("death", func(t *testing.T) {
		s := autoReloadArena("ak47", 0, 30)
		classicDrive(t, s, 0, Input{}, 1)
		s.Players[0].Alive = false
		classicDrive(t, s, 0, Input{}, 300)
		if s.Players[0].Ammo != 0 || s.Players[0].Reserve != 30 {
			t.Fatal("dead player completed automatic reload")
		}
		s = autoReloadArena("ak47", 0, 30)
		s.Players[0].Alive = false
		classicDrive(t, s, 0, Input{}, 1)
		if autoReloadCount(s, "ak47") != 0 {
			t.Fatal("dead player began automatic reload")
		}
	})
	t.Run("freeze", func(t *testing.T) {
		s := autoReloadArena("ak47", 0, 30)
		s.Phase, s.PhaseEndsAt = "freeze", 1000
		classicDrive(t, s, 0, Input{}, 1)
		if autoReloadCount(s, "ak47") != 1 {
			t.Fatal("freeze blocked ordinary idle reload")
		}
		s = autoReloadArena("m3", 0, 4)
		s.Phase, s.PhaseEndsAt = "freeze", 1000
		classicDrive(t, s, 0, Input{Fire: true}, 1)
		if autoReloadCount(s, "m3") != 0 {
			t.Fatal("freeze executed empty shotgun primary attack")
		}
	})
	t.Run("recovery", func(t *testing.T) {
		s := autoReloadArena("m4a1", 0, 30)
		classicDrive(t, s, 0, Input{}, 100)
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var restored State
		if err = json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		restored.World = s.World
		classicDrive(t, s, 0, Input{}, 300)
		classicDrive(t, &restored, 0, Input{}, 300)
		a, _ := json.Marshal(s)
		b, _ := json.Marshal(restored)
		if string(a) != string(b) || s.Players[0].Ammo != 30 || s.Players[0].Reserve != 0 {
			t.Fatal("restored automatic reload diverged or did not complete")
		}
	})
}
