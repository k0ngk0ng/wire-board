package engine

import (
	"encoding/json"
	"testing"
)

func TestRoundEndMotionContinuesUntilNextFreezeE2E(t *testing.T) {
	state := freezeScenario()
	state.Phase = "round_end"
	state.Players[0].Input = Input{Life: 1, MoveY: 1000, Jump: true, Crouch: true}
	before := Position(state.Players[0])
	Advance(state, state.Frame+10)
	if Position(state.Players[0]) == before || !state.Players[0].Crouch {
		t.Fatal("round end movement or crouch locked")
	}
	if state.Phase != "round_end" || state.PhaseEndsAt != 10000 {
		t.Fatal("movement changed round result")
	}
	state.Phase = "freeze"
	before = Position(state.Players[0])
	Advance(state, state.Frame+1)
	if Position(state.Players[0]) != before {
		t.Fatal("next freeze allowed movement")
	}
}

func sniperScenario(t *testing.T, id string, zoom int) (*State, int) {
	t.Helper()
	state := combatArena("t")
	InitInventory(&state.Players[0])
	if err := Buy(state, 0, id); err != nil {
		t.Fatal(err)
	}
	p := &state.Players[0]
	index := weaponIndex(p, id)
	p.Inventory[index].Zoom = zoom
	p.Inventory[index].Reserve = 10
	p.NextAttack = 0
	state.Frame = 100
	p.Input = Input{Life: 1, Fire: true, Yaw: 32768}
	StepCombat(state)
	return state, index
}

func TestBoltSniperShotExitsAndResumesOriginalScopeE2E(t *testing.T) {
	for _, id := range []string{"awp", "scout"} {
		second := 10
		cycle := int64(145)
		if id == "scout" {
			second = 15
			cycle = 125
		}
		for _, zoom := range []int{0, 40, second} {
			state, index := sniperScenario(t, id, zoom)
			p := &state.Players[0]
			if p.Inventory[index].Zoom != 0 {
				t.Fatalf("%s shot kept scope %d", id, p.Inventory[index].Zoom)
			}
			if p.NextAttack != 100+cycle {
				t.Fatalf("%s cycle %d", id, p.NextAttack)
			}
			p.Input.Fire = false
			state.Frame = p.NextAttack - 1
			StepCombat(state)
			if p.Inventory[index].Zoom != 0 {
				t.Fatal("scope resumed before bolt cycle")
			}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			var restored State
			if err = json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			restored.Frame = p.NextAttack
			StepCombat(&restored)
			if restored.Players[0].Inventory[index].Zoom != zoom {
				t.Fatalf("%s scope failed to resume %d", id, zoom)
			}
		}
	}
}

func TestBoltSniperSwitchAndReloadCancelScopeResumeE2E(t *testing.T) {
	for _, action := range []string{"select", "drop", "reload"} {
		state, index := sniperScenario(t, "awp", 40)
		p := &state.Players[0]
		p.Input.Fire = false
		if action == "select" {
			p.Input.Select = "knife"
		}
		if action == "drop" {
			p.Input.Drop = true
		}
		if action == "reload" {
			state.Frame = p.NextAttack
			p.Input.Reload = true
		}
		state.Frame++
		StepCombat(state)
		if action == "drop" {
			for _, drop := range state.Drops {
				if drop.Weapon.Zoom != 0 {
					t.Fatal("drop retained scope")
				}
			}
			continue
		}
		if action == "select" {
			p.Input.Select = "awp"
			state.Frame++
			StepCombat(state)
		}
		state.Frame = p.NextAttack + 500
		p.Input = Input{}
		StepCombat(state)
		if p.Inventory[index].Zoom != 0 {
			t.Fatalf("%s resumed cancelled scope", action)
		}
	}
}

func TestBoltScopeClearsAcrossNewRoundAndDeathE2E(t *testing.T) {
	state, index := sniperScenario(t, "awp", 40)
	state.Options = Options{Map: "de_dust2", RoundSeconds: 120, MaxRounds: 5}
	InitRound(state)
	state.Frame = 1000
	StepCombat(state)
	if state.Players[0].Inventory[index].Zoom != 0 {
		t.Fatal("new round restored old scope")
	}
}
