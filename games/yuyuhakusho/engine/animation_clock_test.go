package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"testing"
)

// Written before animation_clock.go. Expectations are complete original-ROM
// calls; the observer also verifies every RAM write and the coldboot baseline.
func TestAnimationAndDriftAgainstOriginalCalls(t *testing.T) {
	raw, err := os.ReadFile("testdata/animation-oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Kind        string         `json:"kind"`
		Frame       int            `json:"frame"`
		Actor       string         `json:"actor"`
		ClockBefore AnimationClock `json:"clock_before"`
		ClockAfter  AnimationClock `json:"clock_after"`
		DriftBefore ResidualMotion `json:"drift_before"`
		DriftAfter  ResidualMotion `json:"drift_after"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	manifestRaw, err := os.ReadFile("testdata/animation-oracle-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pass          bool   `json:"pass"`
		CompleteCalls int    `json:"complete_calls"`
		FixtureSHA256 string `json:"fixture_sha256"`
		BaselineMatch bool   `json:"frames_match_original_core"`
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if !manifest.Pass || !manifest.BaselineMatch || len(rows) == 0 || manifest.CompleteCalls != len(rows) || manifest.FixtureSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("original trace has no verified provenance or complete calls")
	}
	clocks, drifts := 0, 0
	for _, r := range rows {
		switch r.Kind {
		case "clock":
			clocks++
			if got := StepAnimationClock(r.ClockBefore); got != r.ClockAfter {
				t.Fatalf("frame %d actor %s clock: got %+v want %+v", r.Frame, r.Actor, got, r.ClockAfter)
			}
		case "drift":
			drifts++
			if got := StepResidualMotion(r.DriftBefore); got != r.DriftAfter {
				t.Fatalf("frame %d actor %s drift: got %+v want %+v", r.Frame, r.Actor, got, r.DriftAfter)
			}
		default:
			t.Fatalf("unknown record kind %q", r.Kind)
		}
	}
	if clocks == 0 || drifts == 0 {
		t.Fatal("missing one of the original functions")
	}
	t.Logf("original calls matched: animation clock=%d residual motion=%d", clocks, drifts)
}

// Static instruction boundary expectations, explicitly separate from natural
// coverage. A completion byte is not a boolean and stays untouched unless the
// original loop-completion path writes 1.
func TestAnimationClockBoundaries(t *testing.T) {
	cases := []struct {
		name string
		in   AnimationClock
		want AnimationClock
	}{
		{"before_duration", AnimationClock{4, 5, 3, 0, 0}, AnimationClock{4, 5, 4, 0, 0}},
		{"reaches_duration", AnimationClock{4, 5, 4, 0, 0}, AnimationClock{5, 5, 0, 0, 0}},
		{"loop_completion", AnimationClock{4, 5, 4, 128, 0}, AnimationClock{0, 5, 0, 128, 1}},
		{"preserve_completion_on_advance", AnimationClock{4, 5, 4, 0, 7}, AnimationClock{5, 5, 0, 0, 7}},
		{"preserve_completion_before_duration", AnimationClock{4, 5, 3, 128, 1}, AnimationClock{4, 5, 4, 128, 1}},
		{"frame_index_wrap", AnimationClock{255, 1, 0, 0, 0}, AnimationClock{0, 1, 0, 0, 0}},
		{"zero_duration_after_tick_wrap", AnimationClock{9, 0, 255, 0, 0}, AnimationClock{10, 0, 0, 0, 0}},
		{"tick_wrap_stays_before_duration", AnimationClock{9, 1, 255, 0, 0}, AnimationClock{9, 1, 0, 0, 0}},
		{"unsigned_duration", AnimationClock{9, 255, 127, 0, 0}, AnimationClock{9, 255, 128, 0, 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StepAnimationClock(c.in); got != c.want {
				t.Fatalf("got %+v want %+v", got, c.want)
			}
		})
	}
}

func TestResidualMotionBoundaries(t *testing.T) {
	cases := []struct {
		name string
		in   ResidualMotion
		want ResidualMotion
	}{
		{"zero_does_not_move", ResidualMotion{1000, 0, 0}, ResidualMotion{1000, 0, 0}},
		{"negative_does_not_move", ResidualMotion{1000, -1, 1}, ResidualMotion{1000, -1, 1}},
		{"reaches_zero", ResidualMotion{1000, 0x4800, 1}, ResidualMotion{1000, 0, 1}},
		{"crosses_zero_facing_one", ResidualMotion{1000, 1, 1}, ResidualMotion{-17431, -18431, 1}},
		{"crosses_zero_facing_zero", ResidualMotion{1000, 1, 0}, ResidualMotion{19431, -18431, 0}},
		{"decay_before_displacement", ResidualMotion{1000, 0x9000, 0}, ResidualMotion{-17432, 0x4800, 0}},
		{"facing_is_exactly_one", ResidualMotion{1000, 0x9000, 2}, ResidualMotion{-17432, 0x4800, 2}},
		{"positive_position_overflow", ResidualMotion{math.MaxInt32, 0x4801, 1}, ResidualMotion{math.MinInt32, 1, 1}},
		{"negative_position_overflow", ResidualMotion{math.MinInt32, 0x4801, 0}, ResidualMotion{math.MaxInt32, 1, 0}},
		{"maximum_positive_residual", ResidualMotion{math.MaxInt32, math.MaxInt32, 1}, ResidualMotion{-18434, 2147465215, 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StepResidualMotion(c.in); got != c.want {
				t.Fatalf("got %+v want %+v", got, c.want)
			}
		})
	}
}
