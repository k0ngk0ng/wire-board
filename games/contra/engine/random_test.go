package contra

// Failure cases recorded before implementation: incoming Carry is discarded;
// sums clamp instead of wrapping; a new frame counter replaces the value already
// loaded before NMI; enemy-side additions update only a copy; Carry is guessed
// from the last game tick; or captured after-state is fed back as implementation.
// This verifies the isolated arithmetic against original CPU writes. It does
// not prescribe idle-loop counts, virtual CPU scheduling, or World integration.

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func randomOriginalFixture(t *testing.T, name string, value any) {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "..", "docs", "research", "nes-contra", "evidence", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if err := json.NewDecoder(z).Decode(value); err != nil {
		t.Fatal(err)
	}
}

func TestRandomOriginalIdleArithmetic(t *testing.T) {
	var fixture struct {
		Scenarios []struct {
			Controlled bool
			Adds       []struct{ LoadedFrame, CounterAtADC, Before, CarryIn, After, CarryOut uint8 }
		}
	}
	randomOriginalFixture(t, "random-controlled-v2.json.gz", &fixture)
	count := 0
	for _, scenario := range fixture.Scenarios {
		if !scenario.Controlled {
			t.Fatal("missing explicit controlled scope")
		}
		for _, sample := range scenario.Adds {
			state := RandomState{Value: sample.Before}
			carry := state.Accumulate(sample.LoadedFrame, sample.CarryIn != 0)
			if state.Value != sample.After || carry != (sample.CarryOut != 0) {
				t.Fatalf("sample %d: %+v gave %d/%t", count, sample, state.Value, carry)
			}
			count++
		}
	}
	if count != 3584 {
		t.Fatalf("expected 3584 original ADC cases, got %d", count)
	}
}

func TestRandomOriginalNaturalLoadedOperand(t *testing.T) {
	var fixture struct {
		Scenarios []struct {
			Adds []struct{ LoadedFrame, CounterAtADC, Before, CarryIn, After, CarryOut uint8 }
		}
	}
	randomOriginalFixture(t, "random-natural-v4.json.gz", &fixture)
	stale := 0
	for _, scenario := range fixture.Scenarios {
		for _, sample := range scenario.Adds {
			state := RandomState{Value: sample.Before}
			carry := state.Accumulate(sample.LoadedFrame, sample.CarryIn != 0)
			if state.Value != sample.After || carry != (sample.CarryOut != 0) {
				t.Fatalf("natural ADC differs: %+v -> %+v/%t", sample, state, carry)
			}
			if sample.LoadedFrame != sample.CounterAtADC {
				stale++
			}
		}
	}
	if stale == 0 {
		t.Fatal("no interrupted LDA/ADC boundary evidence")
	}
}

func TestRandomOriginalMutatingEnemyBranch(t *testing.T) {
	var fixture struct {
		Passed  bool
		Records []struct{ Frame, Value, CarryIn, Strength, After, CarryOut uint8 }
	}
	randomOriginalFixture(t, "random-mutator-v2.json.gz", &fixture)
	if !fixture.Passed || len(fixture.Records) != 180 {
		t.Fatal("missing original enemy dispatch cases")
	}
	for _, sample := range fixture.Records {
		state := RandomState{Value: sample.Value}
		carry := state.Accumulate(sample.Frame, sample.CarryIn != 0)
		if state.Value != sample.After || carry != (sample.CarryOut != 0) {
			t.Fatalf("enemy ADC differs: %+v -> %+v/%t", sample, state, carry)
		}
	}
}
