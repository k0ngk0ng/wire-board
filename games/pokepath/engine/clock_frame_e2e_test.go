package engine

import (
	"bytes"
	"math"
	"testing"
)

func coreFrameState(t *testing.T, speed float64) *State {
	t.Helper()
	state, err := New(Options{Starter: "froaki", Speed: speed}, 2, 99127, 1730000000000)
	if err != nil {
		t.Fatal(err)
	}
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: 1})
	coreApply(t, state, 0, Action{Type: "wave"})
	return state
}

func TestCoreSixtyHertzFramesAdvanceWithoutMillisecondRoundingDrift(t *testing.T) {
	state := coreFrameState(t, 1)
	seedSequence := state.Sequence
	for index, checkpoint := range []struct {
		advance, elapsed, simulated int64
		frame                       uint64
	}{{16, 16, 0, 0}, {1, 17, 16, 1}, {16, 33, 16, 1}, {1, 34, 33, 2}, {16, 50, 50, 3}, {950, 1000, 1000, 60}} {
		if err := Advance(state, checkpoint.advance); err != nil {
			t.Fatal(err)
		}
		if state.Clock != checkpoint.elapsed || state.Tick != checkpoint.simulated || state.Sequence-seedSequence != checkpoint.frame {
			t.Fatalf("checkpoint %d: elapsed %d ms, simulated %d ms, frames %d; want %d, %d, %d", index, state.Clock, state.Tick, state.Sequence-seedSequence, checkpoint.elapsed, checkpoint.simulated, checkpoint.frame)
		}
	}
}

func TestCoreSixtyHertzKeepsTheSameFractionalPhaseAcrossResumeAndSpeedChange(t *testing.T) {
	state := coreFrameState(t, 1.7)
	if err := Advance(state, 23); err != nil {
		t.Fatal(err)
	}
	due := float64(state.RemainderMS) + state.SpeedRemainder
	if state.Tick != 28 || math.Abs(due-(23-50.0/3)) > 0.00000001 {
		t.Fatalf("unexpected physics phase after 23 ms at 1.7x: tick=%v remainder=%v", state.Tick, due)
	}
	checkpoint, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := Unmarshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	for _, world := range []*State{state, resumed} {
		coreApply(t, world, 1, Action{Type: "speed", Value: 1})
	}
	if err := Advance(state, 1977); err != nil {
		t.Fatal(err)
	}
	for remaining := int64(1977); remaining > 0; {
		slice := min(29, remaining)
		if err := Advance(resumed, slice); err != nil {
			t.Fatal(err)
		}
		remaining -= slice
	}
	first, _ := Marshal(state)
	second, _ := Marshal(resumed)
	if !bytes.Equal(first, second) || state.Tick != 2011 || state.Sequence != 124 {
		t.Fatalf("checkpoint or request partition altered the simulation: tick=%d sequence=%d", state.Tick, state.Sequence)
	}
}

func TestCoreSixtyHertzProcessesEveryScaledFrameAtAllGameSpeeds(t *testing.T) {
	for _, speed := range []float64{.4, .8, 1, 1.2, 1.7, 2, 2.5, 5} {
		state := coreFrameState(t, speed)
		sequence := state.Sequence
		if err := Advance(state, 1000); err != nil {
			t.Fatal(err)
		}
		if state.Clock != 1000 || state.Tick != int64(speed*1000) || state.Sequence-sequence != uint64(math.Ceil(speed)*60) || state.RemainderMS != 0 || state.SpeedRemainder != 0 {
			t.Fatalf("%vx: expected scaled time and perfect phase after a second, got tick=%d frames=%d remainder=(%d,%v)", speed, state.Tick, state.Sequence-sequence, state.RemainderMS, state.SpeedRemainder)
		}
	}
}

func TestCoreSixtyHertzSlowSpeedUpdatesAtTheOriginalRealFrameBoundary(t *testing.T) {
	for _, speed := range []float64{.4, .8, 1.2, 1.7, 2.5} {
		state := coreFrameState(t, speed)
		sequence := state.Sequence
		if err := Advance(state, 16); err != nil {
			t.Fatal(err)
		}
		if state.Sequence != sequence {
			t.Fatal("rules ran before the first real frame", speed)
		}
		if err := Advance(state, 1); err != nil {
			t.Fatal(err)
		}
		if state.Sequence-sequence != uint64(math.Ceil(speed)) || state.Tick != int64(speed*1000/60) {
			t.Fatalf("%vx: did not divide one real frame into equal original substeps: tick=%v steps=%v", speed, state.Tick, state.Sequence-sequence)
		}
	}
}
