package engine_test

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/games/hotlinemiami/engine"
)

func TestExpandedCharactersAndWeaponActions(t *testing.T) {
	characters := engine.Characters()
	if len(characters) != 8 {
		t.Fatalf("want eight playable looks, got %d", len(characters))
	}
	seen := map[string]bool{}
	for _, character := range characters {
		if seen[character] {
			t.Fatal("duplicate playable look")
		}
		seen[character] = true
		if _, err := engine.NewWithMap(engine.Options{Map: "fixture", Characters: []string{character, "jacket"}}, 2, 23, arena()); err != nil {
			t.Fatalf("look %s: %v", character, err)
		}
	}
	wanted := map[string]bool{"pistol": true, "uzi": true, "m16": true, "shotgun": true, "knife": true, "bat": true, "katana": true, "magnum": true, "mp5": true, "double-barrel": true, "axe": true, "machete": true}
	if len(engine.Weapons()) != len(wanted) {
		t.Fatalf("want twelve weapons, got %d", len(engine.Weapons()))
	}
	for _, weapon := range engine.Weapons() {
		if !wanted[weapon.ID] {
			t.Fatalf("unexpected or duplicate weapon %s", weapon.ID)
		}
		delete(wanted, weapon.ID)
		t.Run(weapon.ID, func(t *testing.T) {
			s, err := engine.NewWithMap(engine.Options{Map: "fixture", Weapons: []string{weapon.ID, "pistol"}}, 2, 37, arena())
			if err != nil {
				t.Fatal(err)
			}
			u := engine.Unit
			s.Players[0].X, s.Players[0].Y = 100*u, 200*u
			s.Players[1].X, s.Players[1].Y = 120*u, 200*u
			s.Players[0].ProtectedUntil, s.Players[1].ProtectedUntil = 0, 0
			input(t, s, 0, engine.Input{Fire: true})
			input(t, s, 0, engine.Input{})
			engine.Advance(s, 4)
			if s.Players[1].Alive || s.Players[0].Score != 1 {
				t.Fatalf("weapon %s did not settle a legal hit", weapon.ID)
			}
			remaining := weapon.Ammo
			if !weapon.Melee {
				remaining--
			}
			if s.Players[0].Ammo != remaining {
				t.Fatalf("weapon %s ammo changed incorrectly", weapon.ID)
			}
			picked := match(t, 2)
			ammo := 0
			if !weapon.Melee {
				ammo = max(1, weapon.Ammo/2)
			}
			picked.Drops = []engine.Drop{{ID: 900, X: picked.Players[0].X, Y: picked.Players[0].Y, Weapon: weapon.ID, Ammo: ammo}}
			input(t, picked, 0, engine.Input{Pickup: true})
			engine.Step(picked)
			if picked.Players[0].Weapon != weapon.ID || picked.Players[0].Ammo != ammo {
				t.Fatalf("weapon %s pickup refilled or changed identity", weapon.ID)
			}
			if weapon.Cooldown <= 0 || weapon.Range <= 0 || !weapon.Melee && (weapon.Speed <= 0 || weapon.Pellets <= 0 || weapon.Ammo <= 0) {
				t.Fatalf("weapon %s has incomplete authority parameters", weapon.ID)
			}
		})
	}
}

func TestThrowThenUnarmedActionUsesSharedTiming(t *testing.T) {
	weapon := engine.UnarmedWeapon()
	if weapon.ID != "" || !weapon.Melee || weapon.Cooldown != 22 || weapon.Range != 25*engine.Unit || weapon.Ammo != 0 {
		t.Fatal("unarmed metadata changed original authority parameters")
	}
	for _, selectable := range engine.Weapons() {
		if selectable.ID == "" {
			t.Fatal("unarmed was added to selectable weapons")
		}
	}
	s := match(t, 2)
	u := engine.Unit
	s.Players[0].X, s.Players[0].Y = 100*u, 200*u
	s.Players[1].X, s.Players[1].Y = 120*u, 200*u
	s.Players[0].ProtectedUntil, s.Players[1].ProtectedUntil = 0, 0
	input(t, s, 0, engine.Input{Throw: true, Aim: 512})
	engine.Step(s)
	if s.Players[0].Weapon != "" {
		t.Fatal("throw did not leave the player unarmed")
	}
	engine.ClearInput(s, 0)
	engine.Advance(s, 13)
	input(t, s, 0, engine.Input{Fire: true})
	input(t, s, 0, engine.Input{})
	engine.Step(s)
	if !s.Players[1].Alive || s.Players[1].DownUntil <= s.Frame || s.Players[0].Score != 0 {
		t.Fatal("unarmed action did not knock down the target without lethal damage")
	}
	if s.Players[0].NextAttackFrame != s.Frame+weapon.Cooldown || s.Players[0].Ammo != 0 || s.Players[0].Weapon != "" {
		t.Fatal("unarmed action diverged from shared timing or consumed a weapon")
	}
}
