package engine

import (
	"reflect"
	"testing"
)

// Failure inventory: AI decision lost before canonical world commit, binding
// a wrong auxiliary object, writing a teammate's message or status, changing
// unrelated actors/auxiliary actions, or reusing a copied human-command state.
func TestOriginalAuxiliaryCanonicalInput(t *testing.T) {
	var rows []struct {
		worldInputFixture
		Actor uint8
	}
	readPropAttackFixture(t, "testdata/ai-auxiliary-world-input.json.gz", &rows)
	if len(rows) != 16 {
		t.Fatal("missing original full input snapshots", len(rows))
	}
	for i, r := range rows {
		s := r.Before
		if e := advanceWorldComputerInput(&s, r.Actor, r.context(t)); e != nil {
			t.Fatal(i, e)
		}
		if s != r.After {
			t.Fatalf("case%d actor%d: %s", i, r.Actor, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "input"))
		}
	}
}
