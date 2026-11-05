package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func fighter4SchemaRules(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	zipped, err := os.ReadFile("../data/chu-world-rules.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var rules map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rules); err != nil {
		t.Fatal(err)
	}
	rules["schema"] = json.RawMessage(`3`)
	data, err := os.ReadFile("testdata/fighter4-projectile-rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var dragon struct{ Curve Fighter4DragonCurve }
	if err := json.Unmarshal(data, &dragon); err != nil {
		t.Fatal(err)
	}
	rules["fighter4_dragon"], err = json.Marshal(dragon)
	if err != nil {
		t.Fatal(err)
	}
	movement := fighter4CompositionMovements(t)[0x15946]
	rules["alternate_movements"], err = json.Marshal([]struct {
		Source   uint32              `json:"source"`
		Movement FighterMovementSpec `json:"movement"`
	}{{0x15946, movement}})
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

func TestFighter4RulesSchema3AndLegacy(t *testing.T) {
	rules := fighter4SchemaRules(t)
	raw, _ := json.Marshal(rules)
	library, err := DecodeChuWorldRules(raw)
	if err != nil {
		t.Fatal("valid schema3 rules rejected", err)
	}
	movements := fighter4CompositionMovements(t)
	if library.MovementSources[0x15946] != movements[0x15946] || library.MovementSources[0x12578] != movements[0x12578] || library.Fighter4Dragon.Curve.FirstIndex != -32 || len(library.Fighter4Dragon.Curve.Values) != 1344 {
		t.Fatal("schema3 lost independent source movement or padded signed-word curve")
	}
	var original struct{ Curve Fighter4DragonCurve }
	if err := json.Unmarshal(rules["fighter4_dragon"], &original); err != nil || !reflect.DeepEqual(library.Fighter4Dragon.Curve, original.Curve) || !reflect.DeepEqual(library.Fighter4Dragon.Animations, library.Animations) {
		t.Fatal("schema3 altered semantic curve or animation data", err)
	}
	for _, version := range []string{"1", "2"} {
		rules["schema"] = json.RawMessage(version)
		delete(rules, "alternate_movements")
		delete(rules, "fighter4_dragon")
		raw, _ := json.Marshal(rules)
		legacy, err := DecodeChuWorldRules(raw)
		if err != nil {
			t.Fatal("legacy schema rejected", version, err)
		}
		for _, spec := range legacy.Fighters {
			if legacy.MovementSources[spec.MovementSource] != spec.Movement {
				t.Fatal("legacy default movement source lost", version, spec.CharacterID)
			}
		}
		if _, ok := legacy.MovementSources[0x15946]; ok || len(legacy.Fighter4Dragon.Curve.Values) != 0 {
			t.Fatal("legacy schema invented unavailable source4 data")
		}
	}
}

func TestFighter4RulesSchema3RejectsMissingAndConflictingData(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(map[string]json.RawMessage)
	}{
		{"missing-dragon", func(r map[string]json.RawMessage) { delete(r, "fighter4_dragon") }},
		{"missing-movement", func(r map[string]json.RawMessage) { delete(r, "alternate_movements") }},
		{"unknown-movement", func(r map[string]json.RawMessage) {
			r["alternate_movements"] = json.RawMessage(`[{"source":88392,"movement":{}}]`)
		}},
		{"duplicate-movement", func(r map[string]json.RawMessage) {
			var v []json.RawMessage
			_ = json.Unmarshal(r["alternate_movements"], &v)
			r["alternate_movements"], _ = json.Marshal(append(v, v[0]))
		}},
		{"default-alias", func(r map[string]json.RawMessage) {
			var v []map[string]any
			_ = json.Unmarshal(r["alternate_movements"], &v)
			v[0]["source"] = float64(0x12578)
			r["alternate_movements"], _ = json.Marshal(v)
		}},
		{"missing-release-sequences", func(r map[string]json.RawMessage) { delete(r, "fighter0_projectiles") }},
		{"wrong-curve-start", func(r map[string]json.RawMessage) {
			var v struct{ Curve Fighter4DragonCurve }
			_ = json.Unmarshal(r["fighter4_dragon"], &v)
			v.Curve.FirstIndex = 0
			r["fighter4_dragon"], _ = json.Marshal(v)
		}},
		{"short-curve", func(r map[string]json.RawMessage) {
			var v struct{ Curve Fighter4DragonCurve }
			_ = json.Unmarshal(r["fighter4_dragon"], &v)
			v.Curve.Values = v.Curve.Values[:1343]
			r["fighter4_dragon"], _ = json.Marshal(v)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rules := fighter4SchemaRules(t)
			test.change(rules)
			raw, _ := json.Marshal(rules)
			library, err := DecodeChuWorldRules(raw)
			if err == nil || !reflect.DeepEqual(library, ChuWorldLibrary{}) {
				t.Fatal("invalid schema3 accepted or leaked partial library", err)
			}
		})
	}
}

func TestFighter4DragonCanonicalSave(t *testing.T) {
	s := ChuWorldState{Objects: WorldObjects{0x8500: {ObjectID: 0x8500, DragonBank: 0x600}}, DragonBankCursor: 0x600}
	for bank := range s.DragonHistory {
		for i := range s.DragonHistory[bank] {
			s.DragonHistory[bank][i] = Fighter4DragonSample{X: int16(i - 300), Y: int16(bank*100 + i), Angle: uint16(65535 - i), Depth: uint16(i * 71), Layer: uint8(bank + 1), SkipGraphics: uint8(i), Status: uint8(i % 3)}
		}
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored ChuWorldState
	if err := json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(restored, s) {
		t.Fatal("typed dragon history/bank persistence lost canonical state", err)
	}
}
