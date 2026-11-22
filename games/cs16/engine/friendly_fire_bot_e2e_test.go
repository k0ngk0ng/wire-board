package engine

import (
	"encoding/json"
	"testing"
)

func friendlyBotArena(enabled bool) *State {
	state := &State{Options: Options{FriendlyFire: enabled}, Phase: "live", Frame: 100, Seed: 71, World: Map{ID: "fixture", Root: ContentsEmpty, Mode: "bomb", Bombsites: []Bounds{{Min: Vec3{-1000 * Unit, -100 * Unit, 0}, Max: Vec3{-900 * Unit, 100 * Unit, 100 * Unit}}}}, Bomb: Bomb{Carrier: -1}, Players: []Player{{Seat: 0, Team: "t", Character: "terror", Alive: true, Life: 1, Health: 100, Yaw: 0, Grounded: true, Weapon: "ak47", Ammo: 30, Inventory: []WeaponState{{ID: "knife", Ammo: 1}, {ID: "ak47", Ammo: 30, Reserve: 90}}}, {Seat: 1, Team: "ct", Character: "urban", Alive: true, Life: 1, Health: 100, X: 250 * Unit, Yaw: 32768, Weapon: "usp"}, {Seat: 2, Team: "t", Character: "leet", Alive: true, Life: 1, Health: 100, X: 125 * Unit, Weapon: "glock18"}}}
	state.Players[0].BotEnemy, state.Players[0].BotSeenAt, state.Players[0].BotAimSeed = 1, 1, 1
	return state
}

func TestFriendlyFireBotStopsAtTeammateBodyE2E(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		state := friendlyBotArena(enabled)
		before, _ := json.Marshal(state)
		input := BotInput(state, 0)
		if input.Fire {
			t.Fatalf("friendlyFire=%v bot fired through teammate", enabled)
		}
		after, _ := json.Marshal(state)
		if string(before) != string(after) {
			t.Fatal("friendly-block check mutated authority")
		}
		state.Players[2].Y = 100 * Unit
		if !BotInput(state, 0).Fire {
			t.Fatal("side teammate incorrectly blocked enemy shot")
		}
		state.Players[2].X, state.Players[2].Y = 400*Unit, 0
		if !BotInput(state, 0).Fire {
			t.Fatal("teammate behind enemy incorrectly blocked first target")
		}
		state.Players[2].X, state.Players[2].Alive = 125*Unit, false
		if !BotInput(state, 0).Fire {
			t.Fatal("dead teammate blocked enemy shot")
		}
	}
}

func TestFriendlyFireBotChecksCachedAttackE2E(t *testing.T) {
	state := friendlyBotArena(true)
	p := &state.Players[0]
	p.AppliedSeq, p.ReceivedSeq = 1, 1
	p.Input = Input{Seq: 1, Life: 1, ViewFrame: 100, Yaw: 0, Fire: true, QueuedFrame: 100}
	state.Frame = 103
	if BotInput(state, 0).Fire {
		t.Fatal("cached held attack ignored newly blocking teammate")
	}
	if !p.Input.Fire {
		t.Fatal("bot mutated previously committed input")
	}
}

func TestFriendlyFireBotKeepsEnemySelectionE2E(t *testing.T) {
	state := friendlyBotArena(true)
	state.Players[1].Alive = false
	if BotInput(state, 0).Fire {
		t.Fatal("friendlyFire enabled made teammate an enemy target")
	}
	state.Players[1].Alive = true
	state.Players[1].X = 40 * Unit
	state.Players[2].X = 20 * Unit
	state.Players[0].Weapon = "knife"
	state.Players[0].Inventory = []WeaponState{{ID: "knife", Ammo: 1}}
	if BotInput(state, 0).Fire {
		t.Fatal("knife bot attacked through teammate")
	}
}

func TestFriendlyFireOptionsStayInAuthorityE2E(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		state, err := New(Options{Map: "de_dust2", FriendlyFire: enabled}, 10, 71)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		var restored State
		if err = json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		if restored.Options.FriendlyFire != enabled {
			t.Fatal("setting lost during restore")
		}
	}
	state, err := New(Options{Map: "de_dust2"}, 10, 71)
	if err != nil || state.Options.FriendlyFire {
		t.Fatal("omitted setting did not retain disabled default")
	}
}
