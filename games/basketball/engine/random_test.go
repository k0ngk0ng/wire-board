package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalRandomStream(t *testing.T) {
	b, err := os.ReadFile("testdata/random.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Scenario      string
		Frame         int
		Before, After [4]byte
		Value         byte
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) < 100 {
		t.Fatalf("too few original random calls: %d", len(rows))
	}
	last := ""
	var state RandomState
	for _, r := range rows {
		if r.Scenario != last {
			state = RandomState(binary.BigEndian.Uint32(r.Before[:]))
			last = r.Scenario
		}
		if uint32(state) != binary.BigEndian.Uint32(r.Before[:]) {
			t.Fatalf("%s frame %d: random state discontinuity", r.Scenario, r.Frame)
		}
		next, value := NextRandom(state)
		if uint32(next) != binary.BigEndian.Uint32(r.After[:]) || value != r.Value {
			t.Fatalf("%s frame %d: got %08x/%d want %v/%d", r.Scenario, r.Frame, next, value, r.After, r.Value)
		}
		state = next
	}
	t.Logf("matched %d continuous original random calls", len(rows))
}

func TestOriginalRandomDoesNotReseedZero(t *testing.T) {
	state, value := NextRandom(0)
	if state != 0 || value != 0 {
		t.Fatal(state, value)
	}
}
