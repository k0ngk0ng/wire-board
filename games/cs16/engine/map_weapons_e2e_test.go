package engine

import (
	"encoding/json"
	"testing"
)

func mapWeaponScenario(item int, weapon string, count int) *State {
	s := combatArena("t")
	s.Players[0].Z = 36 * Unit
	s.Players[1].X = 500 * Unit
	s.World.WorldWeapons = []MapWeaponSpawn{{ID: 42, Item: item, Weapon: weapon, Count: count, Yaw: 16384}}
	InitInventory(&s.Players[0])
	InitInventory(&s.Players[1])
	return s
}

func TestMapArmouryTwentyFourEntitiesE2E(t *testing.T) {
	s := mapWeaponScenario(4, "ak47", 1)
	s.World.WorldWeapons = nil
	items := []int{0, 4, 5, 7, 6, 2, 0, 4, 5, 7, 6, 2, 1, 11, 12, 13, 10, 3, 1, 11, 12, 13, 10, 3}
	weapons := map[int]string{0: "mp5navy", 1: "tmp", 2: "p90", 3: "mac10", 4: "ak47", 5: "sg552", 6: "m4a1", 7: "aug", 10: "awp", 11: "m3", 12: "xm1014", 13: "m249"}
	for index, item := range items {
		s.World.WorldWeapons = append(s.World.WorldWeapons, MapWeaponSpawn{ID: 39 + index, Item: item, Weapon: weapons[item], X: index * 128 * Unit, Count: 1, Yaw: 16384})
	}
	InitMapWeapons(s)
	if len(s.Drops) != 24 {
		t.Fatalf("world gun count %d", len(s.Drops))
	}
	for index, drop := range s.Drops {
		weapon, _ := WeaponByID(weapons[items[index]])
		reserve := 60
		switch weapon.ID {
		case "p90":
			reserve = 50
		case "awp":
			reserve = 20
		case "m3", "xm1014":
			reserve = 24
		}
		if drop.SpawnID != 39+index || drop.Remaining != 1 || drop.Weapon.ID != weapon.ID || drop.Weapon.Ammo != weapon.Ammo || drop.Weapon.Reserve != reserve || drop.Yaw != 16384 {
			t.Fatalf("original entity mismatch %+v", drop)
		}
	}
	InitMapWeapons(s)
	if len(s.Drops) != 24 {
		t.Fatal("round initialization duplicated armoury")
	}
}

func TestMapArmouryTouchBoxBoundariesE2E(t *testing.T) {
	for _, item := range []struct {
		name        string
		point       Vec3
		crouch, hit bool
	}{
		{"center", Vec3{0, 0, 36 * Unit}, false, true},
		{"x-edge", Vec3{32 * Unit, 0, 36 * Unit}, false, true},
		{"x-outside", Vec3{32*Unit + 1, 0, 36 * Unit}, false, false},
		{"corner", Vec3{32 * Unit, 32 * Unit, 36 * Unit}, false, true},
		{"corner-outside", Vec3{32 * Unit, 32*Unit + 1, 36 * Unit}, false, false},
		{"standing-upper-edge", Vec3{0, 0, 52 * Unit}, false, true},
		{"standing-above", Vec3{0, 0, 52*Unit + 1}, false, false},
		{"standing-lower-edge", Vec3{0, 0, -36 * Unit}, false, true},
		{"standing-below", Vec3{0, 0, -36*Unit - 1}, false, false},
		{"duck-upper-edge", Vec3{0, 0, 34 * Unit}, true, true},
		{"duck-above", Vec3{0, 0, 34*Unit + 1}, true, false},
		{"duck-lower-edge", Vec3{0, 0, -32 * Unit}, true, true},
		{"duck-below", Vec3{0, 0, -32*Unit - 1}, true, false},
	} {
		s := mapWeaponScenario(4, "ak47", 1)
		InitMapWeapons(s)
		p := &s.Players[0]
		p.X, p.Y, p.Z, p.Crouch = item.point.X, item.point.Y, item.point.Z, item.crouch
		if got := TouchMapWeapons(s, 0); got != item.hit {
			t.Fatalf("%s pickup %t want %t", item.name, got, item.hit)
		}
		if item.hit && (p.Weapon != "ak47" || len(s.Drops) != 0) {
			t.Fatalf("%s failed inventory/source consumption", item.name)
		}
	}
}

func TestMapArmourySharedAmmoAndSlotsE2E(t *testing.T) {
	for _, item := range []struct {
		raw          int
		weapon, team string
		reserve      int
	}{{0, "mp5navy", "t", 100}, {3, "mac10", "ct", 84}, {4, "ak47", "ct", 60}} {
		s := mapWeaponScenario(item.raw, item.weapon, 1)
		s.Players[0].Team = item.team
		InitInventory(&s.Players[0])
		money := s.Players[0].Money
		InitMapWeapons(s)
		if !TouchMapWeapons(s, 0) {
			t.Fatalf("%s was not touched", item.weapon)
		}
		p := s.Players[0]
		definition, _ := WeaponByID(item.weapon)
		if p.Weapon != item.weapon || p.Ammo != definition.Ammo || p.Reserve != item.reserve || p.Money != money {
			t.Fatalf("%s pickup %+v", item.weapon, p)
		}
		for _, owned := range p.Inventory {
			other, ok := WeaponByID(owned.ID)
			if ok && other.AmmoType == definition.AmmoType && owned.Reserve != item.reserve {
				t.Fatal("shared ammo not synchronized")
			}
		}
		InitMapWeapons(s)
		if TouchMapWeapons(s, 0) || s.Drops[0].Remaining != 1 {
			t.Fatal("existing primary consumed map gun")
		}
	}
}

func TestMapArmouryCountRestoreAndRoundRefreshE2E(t *testing.T) {
	s := mapWeaponScenario(4, "ak47", 2)
	InitMapWeapons(s)
	if !TouchMapWeapons(s, 0) || s.Drops[0].Remaining != 1 {
		t.Fatal("first count not consumed once")
	}
	if TouchMapWeapons(s, 0) || s.Drops[0].Remaining != 1 {
		t.Fatal("duplicate touch consumed remaining")
	}
	raw, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	restored.World = s.World
	restored.Players[1].X = 0
	restored.Players[1].Z = 36 * Unit
	if !TouchMapWeapons(&restored, 1) || len(restored.Drops) != 0 {
		t.Fatal("remaining count failed after restore")
	}
	InitMapWeapons(&restored)
	if len(restored.Drops) != 1 || restored.Drops[0].Remaining != 2 || restored.Drops[0].SpawnID != 42 {
		t.Fatal("round refresh did not restore source count")
	}
	for _, count := range []int{0, -1} {
		s := mapWeaponScenario(4, "ak47", count)
		InitMapWeapons(s)
		if len(s.Drops) != 1 || s.Drops[0].Remaining != 1 {
			t.Fatal("source nonpositive count normalization missing")
		}
	}
}

func TestMapArmouryKeepsOrdinaryDropSemanticsAndCapacityE2E(t *testing.T) {
	s := mapWeaponScenario(0, "mp5navy", 1)
	InitMapWeapons(s)
	s.Players[0].Z = 36 * Unit
	PickupDrop(s, 0)
	if weaponIndex(&s.Players[0], "mp5navy") >= 0 || s.Drops[0].Remaining != 1 {
		t.Fatal("E bypassed original touch flow")
	}
	s.Drops = append(s.Drops, Drop{ID: 900, X: 0, Y: 0, Z: 36 * Unit, Weapon: WeaponState{ID: "ak47", Ammo: 7, Reserve: 13}})
	PickupDrop(s, 0)
	if s.Players[0].Weapon != "ak47" || s.Players[0].Ammo != 7 || s.Players[0].Reserve != 13 || len(s.Drops) != 1 {
		t.Fatal("ordinary E pickup changed")
	}
	for index := 0; index < 71; index++ {
		p := &s.Players[0]
		p.Inventory = append(p.Inventory, WeaponState{ID: "m4a1", Ammo: 30})
		dropWeapon(s, p, len(p.Inventory)-1)
	}
	ordinary, static := 0, 0
	for _, drop := range s.Drops {
		if drop.SpawnID > 0 {
			static++
			if drop.SpawnID != 42 || drop.Remaining != 1 {
				t.Fatal("static drop damaged by capacity")
			}
		} else {
			ordinary++
		}
	}
	if ordinary != 64 || static != 1 {
		t.Fatalf("drop counts ordinary %d static %d", ordinary, static)
	}
	s.World.WorldWeapons = nil
	InitMapWeapons(s)
	if len(s.Drops) != 64 {
		t.Fatal("map without world guns altered ordinary drops")
	}
}

func TestMapArmouryFloorGoalAndInvalidMetadataE2E(t *testing.T) {
	s := mapWeaponScenario(10, "awp", 1)
	s.World.Planes = []Plane{{Z: NormalUnit}}
	s.World.Nodes = []Node{{Plane: 0, Front: ContentsEmpty, Back: ContentsSolid}}
	s.World.Root = 0
	s.World.WorldWeapons[0].Z = 64 * Unit
	InitMapWeapons(s)
	if s.Drops[0].Z < 0 || s.Drops[0].Z > Unit/16 {
		t.Fatalf("source toss did not settle floor %d", s.Drops[0].Z)
	}
	goal, ok := MapWeaponGoal(s, 0)
	if !ok || goal.Z != s.Drops[0].Z+36*Unit {
		t.Fatalf("map weapon goal %+v %t", goal, ok)
	}
	s.Players[0].Alive = false
	if TouchMapWeapons(s, 0) {
		t.Fatal("dead player picked map gun")
	}
	s.Players[0].Alive = true
	s.Phase = "round_end"
	if TouchMapWeapons(s, 0) {
		t.Fatal("round end picked map gun")
	}
	if TouchMapWeapons(s, -1) || TouchMapWeapons(s, 100) {
		t.Fatal("invalid seat picked map gun")
	}
	s = mapWeaponScenario(999, "ak47", 1)
	s.World.WorldWeapons = append(s.World.WorldWeapons, MapWeaponSpawn{ID: 43, Item: 4, Weapon: "m4a1", Count: 1})
	InitMapWeapons(s)
	if len(s.Drops) != 0 {
		t.Fatal("invalid source metadata created weapons")
	}
}
