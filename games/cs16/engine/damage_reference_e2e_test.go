package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type originalDamageCase struct {
	Weapon         string `json:"weapon"`
	BaseDamage     int    `json:"baseDamage"`
	ArmorRatio     int    `json:"armorRatio"`
	Hitgroup       int    `json:"hitgroup"`
	InitialArmor   int    `json:"initialArmor"`
	Helmet         bool   `json:"helmet"`
	FriendlyFire   bool   `json:"friendlyFire"`
	HealthDamage   []int  `json:"healthDamage"`
	Armor          []int  `json:"armor"`
	ArmorRemainder []int  `json:"armorRemainder"`
}

func TestOriginalDamageMatrixE2E(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "damage-reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		SourceSHA256 string               `json:"sourceSHA256"`
		Rows         []originalDamageCase `json:"rows"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SourceSHA256 != "608904a0357af349466c7b34e69f224268090e3b553b5d46bf13b180001c9d8c" || len(fixture.Rows) != 2424 {
		t.Fatal("reference identity or matrix coverage changed", fixture.SourceSHA256, len(fixture.Rows))
	}
	results := make([]map[string]any, 0, len(fixture.Rows))
	failed := 0
	for index, row := range fixture.Rows {
		if len(row.HealthDamage) != 3 || len(row.Armor) != 3 || len(row.ArmorRemainder) != 3 {
			t.Fatal("reference case must retain all three consecutive hits", index)
		}
		s := combatArena("t")
		s.Players[1].Health = 10000
		s.Players[1].Armor = row.InitialArmor
		s.Players[1].Helmet = row.Helmet
		if row.FriendlyFire {
			s.Options.FriendlyFire = true
			s.Players[1].Team = "t"
		}
		damages, armors, remainders := []int{}, []int{}, []int{}
		passed := true
		for step := range row.HealthDamage {
			before := s.Players[1].Health
			attacker := 0
			if row.Weapon == "c4" {
				attacker = -1
			}
			DamagePlayer(s, attacker, 1, row.BaseDamage, row.ArmorRatio, row.Hitgroup, row.Weapon)
			damages = append(damages, before-s.Players[1].Health)
			armors = append(armors, s.Players[1].Armor)
			remainders = append(remainders, s.Players[1].ArmorRemainder)
			passed = passed && damages[step] == row.HealthDamage[step] && armors[step] == row.Armor[step] && remainders[step] == row.ArmorRemainder[step]
		}
		results = append(results, map[string]any{"index": index, "weapon": row.Weapon, "group": row.Hitgroup, "armor": row.InitialArmor, "helmet": row.Helmet, "friendlyFire": row.FriendlyFire, "expectedDamage": row.HealthDamage, "actualDamage": damages, "expectedArmor": row.Armor, "actualArmor": armors, "expectedArmorRemainder": row.ArmorRemainder, "actualArmorRemainder": remainders, "passed": passed})
		if !passed {
			failed++
			if failed <= 16 {
				t.Error(fmt.Sprintf("%s group %d armor %d helmet %t friend %t: damage %v want %v armor %v want %v", row.Weapon, row.Hitgroup, row.InitialArmor, row.Helmet, row.FriendlyFire, damages, row.HealthDamage, armors, row.Armor))
			}
		}
	}
	out := filepath.Join("..", "..", "..", ".local", "cs16", "verification", "damage-reference")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, err := json.MarshalIndent(map[string]any{"passed": failed == 0, "referenceSHA256": fixture.SourceSHA256, "cases": len(results), "failed": failed, "results": results}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(out, "matrix.json"), append(report, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	if failed > 0 {
		t.Fatalf("%d of %d original damage cases differ; full result retained", failed, len(results))
	}
}

func TestOriginalRangeDamageBoundariesE2E(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "range-reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		SourceSHA256    string `json:"sourceSHA256"`
		MaximumDistance int    `json:"maximumDistance"`
		Transitions     int    `json:"transitions"`
		Rows            []struct {
			Damage     int   `json:"damage"`
			Modifier   int   `json:"modifier"`
			Boundaries []int `json:"boundaries"`
		} `json:"rows"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SourceSHA256 != "608904a0357af349466c7b34e69f224268090e3b553b5d46bf13b180001c9d8c" || len(fixture.Rows) != 1725 || fixture.Transitions != 100050 || fixture.MaximumDistance != 8192*Unit {
		t.Fatal("range reference coverage changed")
	}
	checked, failed := 0, 0
	failures := []map[string]int{}
	for _, row := range fixture.Rows {
		if row.Damage < 1 || row.Damage > 115 || len(row.Boundaries) != row.Damage {
			t.Fatal("incomplete original range transitions", row.Damage, row.Modifier)
		}
		for index, boundary := range row.Boundaries {
			for _, adjacent := range []int{-1, 0, 1} {
				distance := boundary + adjacent
				if distance < 0 || distance > fixture.MaximumDistance {
					continue
				}
				checked++
				want := index + 1
				if adjacent > 0 {
					want--
				}
				actual := rangeDamage(row.Damage, row.Modifier, distance)
				if actual != want {
					failed++
					if len(failures) < 32 {
						failures = append(failures, map[string]int{"damage": row.Damage, "modifier": row.Modifier, "distance": distance, "expected": want, "actual": actual})
					}
				}
			}
		}
	}
	if checked != 217854 {
		t.Fatal("range boundary coverage changed", checked)
	}
	out := filepath.Join("..", "..", "..", ".local", "cs16", "verification", "damage-reference")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, err := json.MarshalIndent(map[string]any{"passed": failed == 0, "checked": checked, "failed": failed, "failureSamples": failures, "referenceSHA256": fixture.SourceSHA256}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(out, "range.json"), append(report, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	if failed > 0 {
		t.Fatalf("%d of %d exact range boundaries differ: %v", failed, checked, failures)
	}
}
