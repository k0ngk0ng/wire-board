package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

type worldClockSnapshot struct {
	State      WorldClockState
	Window     string
	Pressed    string
	AudioPause uint8 `json:"audio_pause"`
}

func encodeWorldClock(t *testing.T, state WorldClockState, before worldClockSnapshot) worldClockSnapshot {
	t.Helper()
	raw, err := hex.DecodeString(before.Window)
	if err != nil || len(raw) != 64 {
		t.Fatal("invalid original world window", err)
	}
	for _, field := range []struct {
		offset int
		value  uint16
	}{{0x34, state.Frame}, {0x36, state.FrameDelay}, {0x38, state.LogicFrame}, {0x3a, state.LogicDelay}, {2, state.ControllerCount},
		{0x16, state.ScreenShake}, {0x18, state.DurationRequest}, {0x1a, state.AmplitudeRequest}, {0x1c, state.Amplitude}, {0x1e, state.Decay},
		{0x20, state.PanAmplitude}, {0x22, state.PanOffset}} {
		binary.BigEndian.PutUint16(raw[field.offset:], field.value)
	}
	raw[0], raw[1], raw[10] = state.PauseFlags, state.PauseMask, state.SlowFlag
	return worldClockSnapshot{State: state, Window: hex.EncodeToString(raw), Pressed: hex.EncodeToString(state.Pressed[:]), AudioPause: state.AudioPause}
}

func stepOriginalClockHelper(state WorldClockState, kind string) (WorldClockState, error) {
	switch kind {
	case "video":
		return StepVideoClock(state), nil
	case "logic":
		return StepLogicClock(state), nil
	case "toggle":
		return ToggleWorldPause(state)
	case "pause":
		return ApplyWorldPause(state), nil
	case "shake":
		return StepWorldFeedback(state), nil
	default:
		panic("unknown original clock helper")
	}
}

func readWorldClock(t *testing.T, name string, target any) int {
	t.Helper()
	compressed, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass     bool
		Records  int
		Baseline bool   `json:"frames_match_original_core"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(compressed)
	if !m.Pass || !m.Baseline || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original clock evidence")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
	return m.Records
}

type worldClockCall struct {
	Kind, Name    string
	Before, After worldClockSnapshot
	External      *struct {
		Pressed   [8]uint8
		Duration  *uint16
		Amplitude *uint16
	} `json:"external_input"`
}

// Original complete helpers/fixtures and these tests precede world_clock.go.
func TestWorldClockOriginalCompleteCalls(t *testing.T) {
	for _, name := range []string{"world-clock-oracle", "world-clock-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []worldClockCall
			count := readWorldClock(t, name, &rows)
			if len(rows) != count {
				t.Fatal("missing original clock calls")
			}
			kinds := map[string]int{}
			for i, row := range rows {
				got, err := stepOriginalClockHelper(row.Before.State, row.Kind)
				if err != nil || got != row.After.State || !reflect.DeepEqual(encodeWorldClock(t, got, row.Before), row.After) {
					t.Fatalf("world clock %d %s/%s differs: %v, got=%+v want=%+v", i, row.Kind, row.Name, err, got, row.After.State)
				}
				kinds[row.Kind]++
			}
			if len(kinds) != 5 {
				t.Fatal("missing original helper coverage")
			}
			t.Logf("complete original world clock calls=%d kinds=%v", count, kinds)
		})
	}
}

func TestWorldClockSingleInitialState(t *testing.T) {
	var streams []struct {
		Initial worldClockSnapshot
		Calls   []worldClockCall
	}
	count := readWorldClock(t, "world-clock-sequences", &streams)
	if len(streams) != count || count != 4 {
		t.Fatal("missing original world-clock streams")
	}
	steps := 0
	for _, stream := range streams {
		state, encoded := stream.Initial.State, stream.Initial
		for i, call := range stream.Calls {
			if call.External != nil {
				state.Pressed = call.External.Pressed
				if call.External.Duration != nil {
					state.DurationRequest = *call.External.Duration
				}
				if call.External.Amplitude != nil {
					state.AmplitudeRequest = *call.External.Amplitude
				}
			}
			var err error
			state, err = stepOriginalClockHelper(state, call.Kind)
			encoded = encodeWorldClock(t, state, encoded)
			if err != nil || state != call.After.State || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("world clock stream step %d differs: %v", i, err)
			}
			steps++
		}
	}
	if steps != 640 {
		t.Fatal("missing continuous clock helper calls")
	}
}

func TestWorldClockInvalidInputIsAtomic(t *testing.T) {
	state := WorldClockState{PauseFlags: 0x41, ControllerCount: 9, Pressed: [8]uint8{128}, AudioPause: 1}
	got, err := ToggleWorldPause(state)
	if err == nil || got != state {
		t.Fatal("invalid channel count changed pause state")
	}
	state.PauseFlags = 0x40 // Disabled toggle bypasses the unused channel data.
	got, err = ToggleWorldPause(state)
	if err != nil || got.PauseFlags != 0 || got.ControllerCount != state.ControllerCount || got.AudioPause != 1 {
		t.Fatal("disabled pause unexpectedly inspected controller storage")
	}
}
