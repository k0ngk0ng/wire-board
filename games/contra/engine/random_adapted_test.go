package contra

// Adapter failure inventory before implementation (the user explicitly permits
// an independent online sequence): zero seed freezes; fixed idle counts create
// mod-255 degeneration; a duplicate/skipped frame consumes entropy; restoring
// JSON truncates 64-bit state; batching/network reads change draws; enemy writes
// update a stale copy or leak their CPU Carry into the saved idle Carry; a new
// adapter version or inconsistent saved generator state resumes silently.

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestAdaptedRandomKnownSplitMixSequence(t *testing.T) {
	state := NewAdaptedRandom(0)
	// Seed-zero SplitMix64 high bytes: e2, 6e, 06, f8, 1b. The explicit
	// adaptation XORs each into Work, then performs the original ADC/store.
	counters := []uint8{0, 1, 255, 0, 255}
	values := []uint8{0xe2, 0x8d, 0x8a, 0x73, 0x67}
	carries := []bool{false, false, true, false, true}
	for i := range counters {
		if err := state.AdvanceFrame(uint64(i+1), counters[i]); err != nil {
			t.Fatal(err)
		}
		if state.Work.Value != values[i] || state.IdleCarry != carries[i] {
			t.Fatalf("step %d: %+v", i+1, state)
		}
	}
}

func TestAdaptedRandomReplayRestoreAndSharedMutations(t *testing.T) {
	left, right := NewAdaptedRandom(math.MaxUint64), NewAdaptedRandom(math.MaxUint64)
	for frame := uint64(1); frame <= 4096; frame++ {
		counter := uint8(frame)
		if err := left.AdvanceFrame(frame, counter); err != nil {
			t.Fatal(err)
		}
		if err := right.AdvanceFrame(frame, counter); err != nil {
			t.Fatal(err)
		}
		if frame%13 == 0 {
			idleCarry := left.IdleCarry
			before := left.Work.Value
			carry := left.Work.Accumulate(counter, true)
			right.Work.Accumulate(counter, true)
			if left.IdleCarry != idleCarry || left.Work.Value != uint8(uint16(before)+uint16(counter)+1) {
				t.Fatal("enemy mutation did not preserve separate carry state", carry)
			}
		}
		if frame%17 == 0 {
			raw, err := json.Marshal(right)
			if err != nil {
				t.Fatal(err)
			}
			var shape map[string]any
			_ = json.Unmarshal(raw, &shape)
			for _, key := range []string{"seed", "entropy", "frames"} {
				if _, ok := shape[key].(string); !ok {
					t.Fatalf("%s loses precision in JS transport: %s", key, raw)
				}
			}
			var restored AdaptedRandom
			if err := json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			if err := restored.Validate(); err != nil {
				t.Fatal(err)
			}
			right = restored
		}
		if left != right {
			t.Fatalf("restore/replay diverged at frame %d", frame)
		}
	}
}

func TestAdaptedRandomRejectsInvalidTransitionsAtomically(t *testing.T) {
	state := NewAdaptedRandom(73)
	if err := state.AdvanceFrame(1, 1); err != nil {
		t.Fatal(err)
	}
	for _, frame := range []uint64{0, 1, 3, math.MaxUint64} {
		before := state
		if err := state.AdvanceFrame(frame, 200); err == nil || state != before {
			t.Fatal("invalid frame consumed entropy", frame, state)
		}
	}
	for _, corrupt := range []func(*AdaptedRandom){
		func(s *AdaptedRandom) { s.Version++ },
		func(s *AdaptedRandom) { s.Entropy++ },
		func(s *AdaptedRandom) { s.Seed++ },
		func(s *AdaptedRandom) { s.Frames++ },
	} {
		invalid := state
		corrupt(&invalid)
		before := invalid
		if err := invalid.Validate(); err == nil {
			t.Fatal("invalid snapshot accepted", invalid)
		}
		if err := invalid.AdvanceFrame(invalid.Frames+1, 3); err == nil || invalid != before {
			t.Fatal("invalid snapshot mutated")
		}
	}
}

func TestAdaptedRandomCounterExtremesDoNotDegenerate(t *testing.T) {
	for _, counter := range []uint8{0, 1, 127, 255} {
		state := NewAdaptedRandom(0)
		other := NewAdaptedRandom(1)
		var counts [256]int
		var lowBits [4]int
		var sequence, peer [64]uint8
		for frame := uint64(1); frame <= 65536; frame++ {
			if err := state.AdvanceFrame(frame, counter); err != nil {
				t.Fatal(err)
			}
			counts[state.Work.Value]++
			lowBits[state.Work.Value&3]++
			if frame <= 64 {
				if err := other.AdvanceFrame(frame, counter); err != nil {
					t.Fatal(err)
				}
				sequence[frame-1], peer[frame-1] = state.Work.Value, other.Work.Value
			}
		}
		if reflect.DeepEqual(sequence, peer) {
			t.Fatal("server seeds produced identical sequence")
		}
		// Broad deterministic regression bounds catch freeze/short cycles and
		// severe bias; this is not a cryptographic/statistical quality proof.
		for value, n := range counts {
			if n < 150 || n > 370 {
				t.Fatalf("counter=%d value=%d count=%d", counter, value, n)
			}
		}
		for value, n := range lowBits {
			if n < 15500 || n > 17300 {
				t.Fatalf("counter=%d bits=%d count=%d", counter, value, n)
			}
		}
	}
}
