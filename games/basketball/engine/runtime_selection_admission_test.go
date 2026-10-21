package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// Failure inventory before admission: historical server gates conceal original
// teams/slots/court7; another team selects unrestored AI/attack/ball branches;
// teammate slot2/3 changes grades incorrectly; newly selectable court never
// finishes; control/JSON restoration changes the source trajectory. No source
// menu legality claim follows from these cold integration matches. Original
// legality is established separately by retained controller-only menu routes.
func TestRuntimeOriginalSelectionAdmissionMatrix(t *testing.T) {
	if os.Getenv("BASKETBALL_SELECTION_ADMISSION") != "1" {
		t.Skip("explicit full basketball admission audit")
	}
	// Every distinct ordered team pair, with deterministic court/roster coverage.
	// This is 56 representative configurations, not all Cartesian combinations.
	for left := uint8(0); left < 8; left++ {
		for right := uint8(0); right < 8; right++ {
			if left == right {
				continue
			}
			config := DefaultRuntimeConfig()
			config.Match.Mode = 128
			config.Match.Seed = 0x12345678
			config.Match.Teams = [2]uint8{left, right}
			config.Match.Court = (left + right) & 7
			config.Match.Characters = [4]uint8{left & 3, (left + 1) & 3, right & 3, (right + 3) & 3}
			config.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
			t.Run(fmt.Sprintf("teams%d-%d-court%d", left, right, config.Match.Court), func(t *testing.T) { t.Parallel(); verifyColdVideoRuntime(t, config, true) })
		}
	}
	// Court7 plus previously hidden character slots2/3, each team in the first
	// position, and all four platform control arrangements. The script supplies
	// only the existing intermission Start; no invented player/AI strategy.
	for team := uint8(0); team < 8; team++ {
		for _, mode := range []struct {
			name        string
			controllers [4]int8
		}{
			{"solo", [4]int8{0, -1, -1, -1}},
			{"cooperative", [4]int8{0, 1, -1, -1}},
			{"versus", [4]int8{0, -1, 2, -1}},
			{"all-computer", [4]int8{-1, -1, -1, -1}},
		} {
			config := DefaultRuntimeConfig()
			config.Match.Mode = 128
			config.Match.Seed = 0x12345678
			config.Match.Court = 7
			config.Match.Teams = [2]uint8{team, (team + 1) & 7}
			config.Match.Characters = [4]uint8{2, 3, 3, 2}
			config.Match.ControllerAssignments = mode.controllers
			t.Run(fmt.Sprintf("court7-slots23-team%d-%s", team, mode.name), func(t *testing.T) {
				t.Parallel()
				if mode.name == "versus" && (team == 2 || team == 3) {
					verifyOriginalOvertimeTakeover(t, config)
				} else {
					verifyColdVideoRuntime(t, config, true)
				}
			})
		}
	}
}

func TestRuntimeAllOriginalCharactersAdmission(t *testing.T) {
	if os.Getenv("BASKETBALL_SELECTION_ADMISSION") != "1" {
		t.Skip("explicit all-character admission audit")
	}
	for team := uint8(0); team < 8; team++ {
		config := DefaultRuntimeConfig()
		config.Match.Mode = 128
		config.Match.Seed = 0x12345678
		config.Match.Teams = [2]uint8{team, (team + 1) & 7}
		config.Match.Court = (team + 3) & 7
		config.Match.Characters = [4]uint8{0, 1, 1, 0}
		config.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
		t.Run(fmt.Sprintf("slots01-team%d", team), func(t *testing.T) { t.Parallel(); verifyColdVideoRuntime(t, config, true) })
	}
}

// The first fixed-input audit reached genuine 54:54 untimed overtime for these
// two versus configurations. Human seats deliberately receive no match input.
// Retain the exact 48000-frame diagnostic prefix, then exercise the platform's
// existing seat automation API. Only original AI supplies subsequent gameplay.
func verifyOriginalOvertimeTakeover(t *testing.T, config RuntimeConfig) {
	t.Helper()
	var first string
	for repeat := 0; repeat < 2; repeat++ {
		s, e := NewRuntime(config)
		if e != nil {
			t.Fatal(e)
		}
		digest := sha256.New()
		restores := 0
		takeover := false
		var overtimeAt uint64
		for frame := 0; frame < 60000 && !RuntimeFinished(s); frame++ {
			if frame == 48000 {
				if s.Playing == nil || s.Playing.Match.OvertimeFlags&128 == 0 || s.Playing.Match.Clock.Minutes|s.Playing.Match.Clock.Tens|s.Playing.Match.Clock.Ones != 0 || s.Playing.World.Preparation.Scores[0] != s.Playing.World.Preparation.Scores[1] {
					t.Fatal("retained untimed overtime condition changed")
				}
				immutable := s.Config.Match
				s, e = SetRuntimeAutomation(s, [4]bool{true, true, true, true})
				if e != nil {
					t.Fatal(e)
				}
				immutable.ControllerAssignments = [4]int8{-1, -1, -1, -1}
				if s.Config.Match != immutable {
					t.Fatal("takeover rewrote match selection/seed")
				}
				takeover = true
				overtimeAt = s.VideoFrames
			}
			held := [4]Buttons{}
			if s.AwaitingNextPeriod && s.WaitVideoFrames == 0 {
				held[0] = ButtonStart
			}
			before, _ := json.Marshal(s)
			next, e := StepVideo(s, held)
			if e != nil {
				t.Fatalf("frame%d: %v before=%s", frame, e, before)
			}
			if frame%300 == 0 || s.AwaitingNextPeriod {
				var restored RuntimeState
				if e = json.Unmarshal(before, &restored); e != nil {
					t.Fatal(e)
				}
				again, e := StepVideo(restored, held)
				if e != nil || !reflect.DeepEqual(next, again) {
					t.Fatal("overtime restore diverged", frame, e)
				}
				restores++
			}
			if next.LogicAdvanced {
				raw, _ := json.Marshal(next)
				digest.Write(raw)
			}
			s = next.State
		}
		if !RuntimeFinished(s) || !takeover || restores < 160 || s.Outcome.Scores[0] == s.Outcome.Scores[1] {
			t.Fatal("original-AI takeover did not finish", s.VideoFrames)
		}
		sum := hex.EncodeToString(digest.Sum(nil))
		if repeat == 0 {
			first = sum
		} else if sum != first {
			t.Fatal("overtime takeover nondeterministic")
		}
		t.Logf("repeat=%d original_idle_prefix=%d videos=%d scores=%v restores=%d trajectory_sha256=%s; genuine untimed overtime then explicit original-AI seat takeover", repeat, overtimeAt, s.VideoFrames, s.Outcome.Scores, restores, sum)
	}
}
