package engine

import (
	"encoding/json"
	"testing"
)

func botInventoryScenario(ammo, reserve int) *State {
	p := Player{Seat: 0, Team: "t", Character: "terror", X: 0, Y: 0, Z: 36 * Unit, Alive: true, Life: 1, Health: 100, Grounded: true, Weapon: "ak47", Ammo: ammo, Reserve: reserve, Inventory: []WeaponState{{ID: "knife", Ammo: 1}, {ID: "glock18", Ammo: 20, Reserve: 40}, {ID: "ak47", Ammo: ammo, Reserve: reserve}}}
	return &State{Options: Options{RoundSeconds: 120, MaxRounds: 2}, Phase: "live", PhaseEndsAt: 12000, Round: 1, Seed: 71, World: Map{ID: "fixture", Mode: "elimination", Root: ContentsEmpty}, Bomb: Bomb{Carrier: -1}, Players: []Player{p, {Seat: 1, Team: "ct", Character: "urban", X: 250 * Unit, Z: 36 * Unit, Alive: true, Life: 1, Health: 100, Grounded: true, Weapon: "usp"}}}
}

func botInventorySubmit(t *testing.T, state *State, input Input) {
	t.Helper()
	if err := QueueInput(state, 0, input); err != nil {
		t.Fatal(err)
	}
	Advance(state, state.Frame+3)
}

func TestBotInventoryReloadAfterRejectedDeployInputE2E(t *testing.T) {
	s := botInventoryScenario(0, 60)
	s.Players[0].NextAttack = 75
	first := Input{Seq: 1, Life: 1, ViewFrame: 0, Reload: true}
	botInventorySubmit(t, s, first)
	if s.Players[0].Inventory[2].ReloadUntil != 0 {
		t.Fatal("deployment illegally accepted first reload")
	}
	reloadStarted, reloadEnded, fired := false, false, false
	for s.Frame < 600 && !fired {
		input := BotInput(s, 0)
		if s.Frame < s.Players[0].NextAttack && input.Reload {
			t.Fatal("reload held before legal cooldown")
		}
		botInventorySubmit(t, s, input)
		for _, event := range s.Events {
			if event.Seat != 0 {
				continue
			}
			switch event.Kind {
			case "reload":
				reloadStarted = true
			case "reload_end":
				reloadEnded = true
			case "shot":
				if event.Weapon == "ak47" {
					fired = true
				}
			}
		}
	}
	if !reloadStarted || !reloadEnded || !fired {
		t.Fatalf("reload recovery start %t end %t shot %t frame %d", reloadStarted, reloadEnded, fired, s.Frame)
	}
	if s.Players[0].Reserve != 30 || s.Players[0].Ammo != 29 {
		t.Fatalf("rules ammo changed incorrectly %d/%d", s.Players[0].Ammo, s.Players[0].Reserve)
	}
}

func TestBotInventoryEmptyPrimarySwitchesToLoadedPistolE2E(t *testing.T) {
	s := botInventoryScenario(0, 0)
	input := BotInput(s, 0)
	if input.Select != "glock18" || input.Fire || input.Reload {
		t.Fatalf("empty primary fallback input %+v", input)
	}
	botInventorySubmit(t, s, input)
	if s.Players[0].Weapon != "glock18" {
		t.Fatal("fallback did not execute real Select")
	}
	fired := false
	for s.Frame < 300 && !fired {
		botInventorySubmit(t, s, BotInput(s, 0))
		for _, event := range s.Events {
			if event.Kind == "shot" && event.Seat == 0 && event.Weapon == "glock18" {
				fired = true
			}
		}
	}
	if !fired {
		t.Fatal("loaded pistol never fired")
	}
	if s.Players[0].Inventory[2].Ammo != 0 || s.Players[0].Inventory[2].Reserve != 0 {
		t.Fatal("empty primary was refilled by strategy")
	}
	if s.Players[0].Ammo >= 20 {
		t.Fatal("pistol shot did not consume real ammo")
	}
}

func TestBotInventoryReloadPulseAndPurityE2E(t *testing.T) {
	s := botInventoryScenario(0, 60)
	s.Frame = 100
	p := &s.Players[0]
	input := Input{Select: "ak47", Fire: true, Secondary: true, Reload: true, MoveX: 333, Seq: 17, Life: 1, ViewFrame: 100}
	before, _ := json.Marshal(s)
	BotWeapon(s, 0, &input)
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("inventory helper mutated authority")
	}
	if !input.Reload || input.Fire || input.Secondary || input.MoveX != 333 || input.Seq != 17 {
		t.Fatalf("legal reload pulse %+v", input)
	}
	p.Input.Reload = true
	input.Reload = true
	BotWeapon(s, 0, &input)
	if input.Reload {
		t.Fatal("held committed reload not released")
	}
	p.Input.Reload = false
	p.Inventory[2].ReloadUntil = 200
	input.Reload, input.Fire = true, true
	BotWeapon(s, 0, &input)
	if input.Reload || input.Fire {
		t.Fatal("active reload retained attack input")
	}
	p.Inventory[2].ReloadUntil = 0
	p.NextAttack = 101
	input.Reload = true
	BotWeapon(s, 0, &input)
	if input.Reload {
		t.Fatal("cooldown emitted premature reload edge")
	}
}

func TestBotInventoryCachedFallbackAndKnifeE2E(t *testing.T) {
	s := botInventoryScenario(0, 0)
	p := &s.Players[0]
	p.ReceivedSeq, p.AppliedSeq = 1, 1
	p.Input = Input{Seq: 1, Life: 1, ViewFrame: 0, Select: "ak47", Fire: true, Reload: true, QueuedFrame: 100}
	s.Frame = 103
	input := BotInput(s, 0)
	if input.Select != "glock18" || input.Fire || input.Reload {
		t.Fatalf("cached empty primary input %+v", input)
	}
	p.Inventory[1].Ammo, p.Inventory[1].Reserve = 0, 0
	if got := botBestWeapon(p); got != "knife" {
		t.Fatalf("no ammo best weapon %s", got)
	}
	input = Input{Select: "ak47", Fire: true, Reload: true}
	BotWeapon(s, 0, &input)
	if input.Select != "knife" || input.Fire || input.Reload {
		t.Fatalf("knife fallback input %+v", input)
	}
}

func TestBotInventoryPreservesObjectiveAndUtilityIntentE2E(t *testing.T) {
	s := botInventoryScenario(0, 0)
	p := &s.Players[0]
	p.Inventory = append(p.Inventory, WeaponState{ID: "c4", Ammo: 1}, WeaponState{ID: "hegrenade", Ammo: 1})
	s.World.Mode = "bomb"
	s.Bomb.Carrier = 0
	for _, id := range []string{"c4", "hegrenade", "knife"} {
		input := Input{Select: id, Fire: true, MoveY: 500}
		BotWeapon(s, 0, &input)
		if input.Select != id || !input.Fire || input.MoveY != 500 {
			t.Fatalf("utility %s was replaced %+v", id, input)
		}
	}
}
