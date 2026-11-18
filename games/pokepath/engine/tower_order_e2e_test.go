package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

type towerImpactInput struct {
	Power     float64 `json:"power"`
	Critical  float64 `json:"critical"`
	MaxHealth float64 `json:"maxHealth"`
	Ordinary  bool    `json:"ordinary"`
}

type towerOrderInput struct {
	Name    string            `json:"name"`
	Species []string          `json:"species"`
	Y       []float64         `json:"y"`
	Order   []int             `json:"order"`
	Health  float64           `json:"health"`
	Speed   float64           `json:"speed"`
	LinkAge float64           `json:"linkAge,omitempty"`
	Frames  int               `json:"frames,omitempty"`
	OffsetX float64           `json:"offsetX,omitempty"`
	Impact  *towerImpactInput `json:"impact,omitempty"`
}

type towerOrderEvent struct {
	Frame    int     `json:"frame"`
	Kind     string  `json:"kind"`
	Tower    int     `json:"tower"`
	Enemy    int     `json:"enemy"`
	Amount   float64 `json:"amount"`
	Resolved float64 `json:"resolved"`
}

func TestCoreTowerOrderingReferenceFixtureE2E(t *testing.T) {
	inputs := []towerOrderInput{
		{"descending-deployment", []string{"charmander", "treecko"}, []float64{156, 132}, []int{0, 1}, 1000, 1, 0, 0, 0, nil},
		{"equal-height-stable", []string{"charmander", "treecko"}, []float64{132, 132}, []int{0, 1}, 1000, 1, 0, 0, 0, nil},
		{"equal-height-reversed", []string{"charmander", "treecko"}, []float64{132, 132}, []int{1, 0}, 1000, 1, 0, 0, 0, nil},
		{"spectrier-above", []string{"spectrier", "charmander"}, []float64{108, 132}, []int{0, 1}, 1000, 1, 0, 0, 0, nil},
		{"spectrier-above-reversed", []string{"spectrier", "charmander"}, []float64{108, 132}, []int{1, 0}, 1000, 1, 0, 0, 0, nil},
		{"lethal-before-later-target", []string{"charmander", "treecko"}, []float64{132, 156}, []int{0, 1}, 6, 1, 0, 0, 0, nil},
		{"lethal-with-reversed-deploy", []string{"charmander", "treecko"}, []float64{132, 156}, []int{1, 0}, 6, 1, 0, 0, 0, nil},
		{"three-towers-spectral-last", []string{"spectrier", "charmander", "treecko"}, []float64{108, 156, 132}, []int{0, 1, 2}, 1000, 1, 0, 0, 0, nil},
		{"ordinary-before-beam", []string{"charmander", "bulbasaur"}, []float64{132, 156}, []int{1, 0}, 1000, 1, 0, 0, 0, nil},
		{"beam-before-ordinary", []string{"bulbasaur", "charmander"}, []float64{132, 156}, []int{1, 0}, 1000, 1, 0, 0, 0, nil},
		{"maw-after-earlier-damage", []string{"charmander", "megaMawile"}, []float64{132, 132}, []int{0, 1}, 8, 1, 0, 0, 0, nil},
		{"maw-before-later-damage", []string{"megaMawile", "charmander"}, []float64{132, 132}, []int{0, 1}, 8, 1, 0, 0, 0, nil},
		{"maw-reversed-stable-deploy", []string{"charmander", "megaMawile"}, []float64{132, 132}, []int{1, 0}, 8, 1, 0, 0, 0, nil},
		{"maw-after-lethal-damage", []string{"charmander", "megaMawile"}, []float64{132, 132}, []int{0, 1}, 6, 1, 0, 0, 0, nil},
	}
	for _, speed := range []float64{.4, .8, 1, 1.7, 5} {
		inputs = append(inputs, towerOrderInput{Name: fmt.Sprintf("moving-order-%g", speed), Species: []string{"skarmory", "charmander"}, Y: []float64{132, 156}, Order: []int{0, 1}, Health: 1000, Speed: speed})
	}
	inputs = append(inputs,
		towerOrderInput{Name: "link-before-ordinary", Species: []string{"plusle", "minun", "charmander"}, Y: []float64{132, 156, 180}, Order: []int{0, 1, 2}, Health: 1000, Speed: 1, LinkAge: 990},
		towerOrderInput{Name: "link-after-first-owner-shot", Species: []string{"plusle", "minun", "charmander"}, Y: []float64{132, 156, 180}, Order: []int{2, 0, 1}, Health: 1000, Speed: 1, LinkAge: 990},
		towerOrderInput{Name: "link-minus-first-lethal", Species: []string{"plusle", "minun"}, Y: []float64{132, 156}, Order: []int{0, 1}, Health: 6, Speed: 1, LinkAge: 990},
		towerOrderInput{Name: "link-outside-width", Species: []string{"plusle", "minun"}, Y: []float64{132, 156}, Order: []int{0, 1}, Health: 1000, Speed: 1, LinkAge: 990, OffsetX: 18.5},
		towerOrderInput{Name: "link-natural-1000ms", Species: []string{"plusle", "minun"}, Y: []float64{132, 156}, Order: []int{0, 1}, Health: 1000, Speed: 1, Frames: 120},
	)
	for _, species := range []string{"mawile", "megaMawile"} {
		for _, power := range []float64{6, 7, 15} {
			for _, critical := range []float64{0, 100} {
				for _, health := range []float64{490, 500, 510} {
					inputs = append(inputs, towerOrderInput{Name: fmt.Sprintf("impact-%s-%g-%g-%g", species, power, critical, health), Species: []string{species}, Y: []float64{132}, Order: []int{0}, Health: health, Speed: 1, Impact: &towerImpactInput{Power: power, Critical: critical, MaxHealth: 1000, Ordinary: true}})
				}
				if species == "megaMawile" {
					inputs = append(inputs, towerOrderInput{Name: fmt.Sprintf("impact-extra-maw-%g-%g", power, critical), Species: []string{species}, Y: []float64{132}, Order: []int{0}, Health: 490, Speed: 1, Impact: &towerImpactInput{Power: power, Critical: critical, MaxHealth: 1000}})
				}
			}
		}
	}
	for _, power := range []float64{5, 831, 833, 834, 836} {
		inputs = append(inputs, towerOrderInput{Name: fmt.Sprintf("impact-field-technician-%g", power), Species: []string{"marshadow", "tapuBulu"}, Y: []float64{132, 156}, Order: []int{0, 1}, Health: 10000, Speed: 1, Impact: &towerImpactInput{Power: power, MaxHealth: 10000, Ordinary: true}})
	}
	results := []map[string]any{}
	for _, input := range inputs {
		t.Run(input.Name, func(t *testing.T) {
			state := coreNew(t, 2)
			state.Speed = input.Speed
			state.Team = nil
			for _, species := range input.Species {
				key := species
				if key == "megaMawile" {
					key = "mawile"
				}
				pokemon, err := newPokemon(state, key, 1)
				if err != nil {
					t.Fatal(err)
				}
				state.Team = append(state.Team, pokemon)
			}
			for index, species := range input.Species {
				if species == "megaMawile" {
					state.Inventory = append(state.Inventory, OwnedItem{ID: "mawilite"})
					coreApply(t, state, 0, Action{Type: "equip", PokemonID: state.Team[index].ID, ItemID: "mawilite"})
				}
				if state.Team[index].Species != species {
					t.Fatal("controlled deployment lost its required form")
				}
			}
			for index, slot := range input.Order {
				coreApply(t, state, index%2, Action{Type: "deploy", PokemonID: state.Team[slot].ID, TileID: []int{1, 2, 14}[index]})
			}
			coreApply(t, state, 0, Action{Type: "wave"})
			owners := map[int]int{}
			for index := range state.Team {
				state.Team[index].Stats = Stats{Power: 6, Range: 600, AttackMS: 10000, CriticalPct: 0}
				if input.Impact != nil {
					state.Team[index].Stats.Power = input.Impact.Power
					state.Team[index].Stats.CriticalPct = input.Impact.Critical
				}
				state.Team[index].Target = "first"
				owners[state.Team[index].ID] = index
			}
			towers := map[int]int{}
			for index := range state.Towers {
				tower := &state.Towers[index]
				slot := owners[tower.PokemonID]
				tower.X, tower.Y, tower.CooldownMS, tower.StatsRefreshMS = 252, input.Y[slot], 0, 100
				if state.Team[slot].Species == "megaMawile" && (input.Impact == nil || !input.Impact.Ordinary) || input.Impact != nil && slot > 0 {
					tower.CooldownMS = 10000
				}
				if input.Name == "link-minus-first-lethal" {
					tower.CooldownMS = 10000
				}
				if state.Team[slot].Species == "skarmory" {
					value := worldTower(state, tower)
					value.Origin, value.Angle = Point{X: tower.X, Y: tower.Y}, math.Pi/2
				}
				towers[tower.ID] = slot
			}
			if input.Species[0] == "plusle" {
				var first, second int
				for _, tower := range state.Towers {
					if owners[tower.PokemonID] == 0 {
						first = tower.ID
					}
					if owners[tower.PokemonID] == 1 {
						second = tower.ID
					}
				}
				raw, _ := json.Marshal(map[string]any{"first": first, "second": second, "ageMS": input.LinkAge, "hitAt": map[int]float64{}})
				var link WorldLink
				if err := json.Unmarshal(raw, &link); err != nil {
					t.Fatal(err)
				}
				state.World.Links = []WorldLink{link}
			}
			state.Enemies = nil
			enemies := map[int]int{}
			for index := 0; index < 3; index++ {
				id, err := SpawnEnemyAt(state, "rattata", Point{X: 252 + float64(index*20) + input.OffsetX, Y: 132}, 0, 0)
				if err != nil {
					t.Fatal(err)
				}
				enemy := enemyFor(state, id)
				enemy.HP, enemy.MaxHP, enemy.BaseSpeed, enemy.Entered, enemy.Distance = input.Health, input.Health, 0, true, float64(100-index*10)
				if input.Impact != nil {
					enemy.MaxHP = input.Impact.MaxHealth
					if input.Impact.Ordinary {
						for towerIndex := range state.Towers {
							worldTower(state, &state.Towers[towerIndex]).Flags[fmt.Sprintf("maw:%d", id)] = true
						}
					}
				}
				enemies[id] = index
			}
			snapshot, err := Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := Unmarshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			deployed := append([]Tower(nil), state.Towers...)
			trace := []towerOrderEvent{}
			frames := []map[string]any{}
			cursor := state.NextEventID - 1
			elapsed := int64(0)
			frameCount := input.Frames
			if frameCount == 0 {
				frameCount = 3
			}
			for frame := 0; frame < frameCount; frame++ {
				next := int64(((frame+1)*50 + 2) / 3)
				duration := next - elapsed
				elapsed = next
				if err := Advance(state, duration); err != nil {
					t.Fatal(err)
				}
				for _, event := range state.Events {
					if event.ID <= cursor || event.Type != "fire" && (event.Type != "hit" || event.SourceKind != "physical") {
						continue
					}
					slot := towers[event.Source]
					if event.Type == "hit" {
						slot = owners[event.Source]
					}
					trace = append(trace, towerOrderEvent{Frame: frame + 1, Kind: event.Type, Tower: slot, Enemy: enemies[event.Target], Amount: event.HPDamage, Resolved: event.ResolvedDamage})
				}
				cursor = state.NextEventID - 1
				data, err := Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				frames = append(frames, map[string]any{"duration": duration, "state": json.RawMessage(data), "view": ProjectTransient(state)})
			}
			if err := Advance(restored, elapsed); err != nil {
				t.Fatal(err)
			}
			left, _ := Marshal(state)
			right, _ := Marshal(restored)
			if !bytes.Equal(left, right) {
				t.Fatal("save/restore or millisecond splitting changed tower updates")
			}
			for index, tower := range deployed {
				if tower.ID != state.Towers[index].ID {
					t.Fatal("the update reordered the persistent deployment list")
				}
			}
			health := make([]float64, 3)
			for _, enemy := range state.Enemies {
				health[enemies[enemy.ID]] = max(0, enemy.HP)
			}
			positions := []map[string]any{}
			for _, tower := range state.Towers {
				positions = append(positions, map[string]any{"tower": towers[tower.ID], "x": tower.X - 12, "y": tower.Y - 12})
			}
			results = append(results, map[string]any{"input": input, "events": trace, "health": health, "positions": positions, "initial": json.RawMessage(snapshot), "frames": frames})
		})
	}
	output := os.Getenv("WIRE_BOARD_POKEPATH_TOWER_ORDER_OUT")
	if output == "" {
		output = filepath.Join("..", "..", "..", ".local", "pokepath", "verification", "tower-order", "native.json")
	}
	absolute, err := filepath.Abs(output)
	if err != nil {
		t.Fatal(err)
	}
	allowed, _ := filepath.Abs(filepath.Join("..", "..", "..", ".local", "pokepath", "verification", "tower-order"))
	local, err := filepath.Rel(allowed, absolute)
	if err != nil || filepath.IsAbs(local) || local == ".." || len(local) > 3 && local[:3] == ".."+string(filepath.Separator) {
		t.Fatal("tower-order output must stay inside its verification directory")
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(map[string]any{"rulesVersion": RulesVersion(), "cases": results}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCoreProjectileImpactMultiplierSnapshotValidationE2E(t *testing.T) {
	state := coreNew(t, 1)
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: coreTile(t, state)})
	coreApply(t, state, 0, Action{Type: "wave"})
	for index := 0; index < 3000 && len(state.Projectiles) == 0; index++ {
		if err := Advance(state, 20); err != nil {
			t.Fatal(err)
		}
	}
	if len(state.Projectiles) == 0 {
		t.Fatal("the deployed unit never fired a real projectile")
	}
	snapshot, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name  string
		value json.RawMessage
		valid bool
	}{
		{"zero", json.RawMessage("0"), true},
		{"half", json.RawMessage("0.5"), true},
		{"identity", json.RawMessage("1"), true},
		{"double", json.RawMessage("2"), true},
		{"negative", json.RawMessage("-0.5"), false},
		{"text", json.RawMessage(`"half"`), false},
		{"overflow", json.RawMessage("1e309"), false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var document map[string]json.RawMessage
			if err := json.Unmarshal(snapshot, &document); err != nil {
				t.Fatal(err)
			}
			var projectiles []map[string]json.RawMessage
			if err := json.Unmarshal(document["projectiles"], &projectiles); err != nil {
				t.Fatal(err)
			}
			projectiles[0]["extraPowerMultiplier"] = scenario.value
			document["projectiles"], err = json.Marshal(projectiles)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Unmarshal(encoded)
			if (err == nil) != scenario.valid {
				t.Fatal("snapshot accepted an invalid impact multiplier or rejected a valid one", err)
			}
		})
	}
}
