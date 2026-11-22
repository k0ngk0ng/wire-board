package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func nightVisionFixture(team string) *State {
	s := classicArena()
	s.BuyEndsAt = 10000
	s.Players[0].Team = team
	s.Players[1].Team = "ct"
	if team == "ct" {
		s.Players[1].Team = "t"
	}
	s.Players[0].Money = 16000
	return s
}

func nightVisionArtifact(t *testing.T, name string, value any) {
	t.Helper()
	directory := filepath.Join("..", "..", "..", ".local", "cs16", "verification", "nvg")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, name+".json"), append(raw, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestNightVisionPurchaseBoundariesE2E(t *testing.T) {
	results := []map[string]any{}
	for _, team := range []string{"t", "ct"} {
		for _, money := range []int{1249, 1250, 1251} {
			t.Run(fmt.Sprintf("%s-%d", team, money), func(t *testing.T) {
				s := nightVisionFixture(team)
				p := &s.Players[0]
				p.Money = money
				weapons := len(p.Inventory)
				err := Buy(s, 0, "nvgs")
				if money < 1250 {
					if err == nil || p.NightVisionOwned || p.NightVision || p.Money != money {
						t.Fatal("insufficient funds changed night vision ownership or cash", err, p.Money)
					}
				} else {
					if err != nil || !p.NightVisionOwned || p.NightVision || p.Money != money-1250 || len(p.Inventory) != weapons {
						t.Fatal("night vision purchase differs from equipment semantics", err, p.Money)
					}
					before := p.Money
					if err = Buy(s, 0, "nvgs"); err == nil || p.Money != before {
						t.Fatal("duplicate night vision purchase spent money")
					}
				}
				results = append(results, map[string]any{"team": team, "moneyBefore": money, "moneyAfter": p.Money, "owned": p.NightVisionOwned})
			})
		}
	}
	nightVisionArtifact(t, "purchase-boundaries", map[string]any{"passed": !t.Failed(), "cases": results})
}

func TestNightVisionEdgeCooldownAndReleaseE2E(t *testing.T) {
	s := nightVisionFixture("t")
	classicDrive(t, s, 0, Input{NightVision: true}, 1)
	if s.Players[0].NightVision || InputReleased(Input{NightVision: true}) {
		t.Fatal("unowned night vision toggled or was treated as a released input")
	}
	classicDrive(t, s, 0, Input{}, 30)
	if err := Buy(s, 0, "nvgs"); err != nil {
		t.Fatal(err)
	}
	classicDrive(t, s, 0, Input{NightVision: true}, 1)
	p := &s.Players[0]
	if !p.NightVision || p.NightVisionAt != s.Frame+30 {
		t.Fatal("owned night vision did not toggle with original cooldown")
	}
	first := s.Frame
	classicDrive(t, s, 0, Input{NightVision: true}, 5)
	if !p.NightVision {
		t.Fatal("held night vision key toggled repeatedly")
	}
	classicDrive(t, s, 0, Input{}, 1)
	classicDrive(t, s, 0, Input{NightVision: true}, 1)
	if !p.NightVision {
		t.Fatal("night vision ignored its cooldown")
	}
	classicDrive(t, s, 0, Input{}, int(first+29-s.Frame))
	classicDrive(t, s, 0, Input{NightVision: true}, 1)
	if p.NightVision || s.Frame != first+30 {
		t.Fatal("night vision did not toggle at exact cooldown boundary")
	}
	changes := 0
	for _, event := range s.Events {
		if event.Kind == "nightvision" {
			changes++
			if event.Seat != 0 || event.Target != -1 || event.Weapon != "nvgs" {
				t.Fatal("night vision event identity is invalid")
			}
		}
	}
	if changes != 2 {
		t.Fatal("night vision emitted missing or duplicate toggle events", changes)
	}
	nightVisionArtifact(t, "toggle-cooldown", map[string]any{"passed": true, "firstFrame": first, "secondFrame": s.Frame, "cooldownFrames": 30, "toggleEvents": changes})
}

func TestNightVisionSurvivalDeathAndRestoreE2E(t *testing.T) {
	s := nightVisionFixture("t")
	if err := Buy(s, 0, "nvgs"); err != nil {
		t.Fatal(err)
	}
	classicDrive(t, s, 0, Input{NightVision: true}, 1)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	restored.World = s.World
	if !restored.Players[0].NightVisionOwned || !restored.Players[0].NightVision || restored.Players[0].NightVisionAt != s.Players[0].NightVisionAt {
		t.Fatal("night vision checkpoint lost equipment, display or cooldown")
	}
	InitRound(&restored)
	p := &restored.Players[0]
	if !p.NightVisionOwned || p.NightVision {
		t.Fatal("surviving player lost equipment or kept night vision display across spawn")
	}
	restored.Phase = "live"
	classicDrive(t, &restored, 0, Input{NightVision: true}, 1)
	DamagePlayer(&restored, -1, 0, 1000, 1000, 0, "fall")
	if p.Alive || p.NightVision {
		t.Fatal("dead player kept night vision display")
	}
	for _, drop := range restored.Drops {
		if drop.Weapon.ID == "nvgs" {
			t.Fatal("night vision became an invented dropped weapon")
		}
	}
	InitRound(&restored)
	if p.NightVisionOwned || p.NightVision {
		t.Fatal("dead player retained night vision into next spawn")
	}
	var legacy Player
	if err = json.Unmarshal([]byte(`{"alive":true,"health":100}`), &legacy); err != nil || legacy.NightVisionOwned || legacy.NightVision {
		t.Fatal("legacy save invented night vision equipment")
	}
	nightVisionArtifact(t, "survival-death-restore", map[string]any{"passed": true, "survivorKeepsOwned": true, "spawnDisablesDisplay": true, "deathDoesNotDropNVG": true, "legacyDefaultsOff": true})
}
