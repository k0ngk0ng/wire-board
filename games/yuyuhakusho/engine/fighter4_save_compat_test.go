package engine

import (
	"encoding/json"
	"testing"
)

// Adding unused dragon owners must not change existing characters' saved
// state shape or their deterministic state hashes. Nonzero owners are covered
// by the canonical and actual-callback save/restore tests.
func TestFighter4ZeroStateKeepsLegacySaveShape(t *testing.T) {
	s := ChuWorldState{Objects: WorldObjects{0x8400: {ObjectID: 0x8400}}}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"DragonHistory", "DragonBankCursor"} {
		if _, ok := fields[name]; ok {
			t.Fatal("unused source4 owner changed legacy save shape", name)
		}
	}
	var objects map[string]map[string]json.RawMessage
	if err := json.Unmarshal(fields["Objects"], &objects); err != nil {
		t.Fatal(err)
	}
	if _, ok := objects["33792"]["DragonBank"]; ok {
		t.Fatal("unused FE changed legacy object save shape")
	}
}
