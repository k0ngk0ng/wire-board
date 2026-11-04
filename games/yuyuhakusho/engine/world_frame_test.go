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

type worldFrameSnapshot struct {
	Clock            worldClockSnapshot
	Controllers      [8]ControllerFrame
	Seed             uint32
	SlowGate         uint8 `json:"slow_gate"`
	ResetEnabled     uint8 `json:"reset_enabled"`
	Palette          ArenaPaletteState
	PaletteRegisters string `json:"palette_registers"`
	PaletteBytes     string `json:"palette_bytes"`
}

func frameState(raw worldFrameSnapshot) WorldFrameState {
	return WorldFrameState{Clock: raw.Clock.State, Controllers: raw.Controllers, RandomSeed: raw.Seed, SlowGate: raw.SlowGate, ResetEnabled: raw.ResetEnabled}
}

func encodeWorldFrame(t *testing.T, state WorldFrameState, palette ArenaPaletteState, old worldFrameSnapshot) worldFrameSnapshot {
	t.Helper()
	raw := old
	raw.Clock = encodeWorldClock(t, state.Clock, old.Clock)
	raw.Controllers, raw.Seed, raw.SlowGate, raw.ResetEnabled = state.Controllers, state.RandomSeed, state.SlowGate, state.ResetEnabled
	raw.Palette = palette
	regs := make([]byte, 8)
	regs[0], regs[1] = palette.Flags, palette.Status
	binary.BigEndian.PutUint16(regs[2:], palette.Mode)
	binary.BigEndian.PutUint16(regs[4:], palette.Step)
	binary.BigEndian.PutUint16(regs[6:], palette.Target)
	raw.PaletteRegisters = hex.EncodeToString(regs)
	colors := make([]byte, 384)
	for bank, words := range [3][64]uint16{palette.Current, palette.Primary, palette.Secondary} {
		for i, word := range words {
			binary.BigEndian.PutUint16(colors[bank*128+i*2:], word)
		}
	}
	raw.PaletteBytes = hex.EncodeToString(colors)
	return raw
}

func frameComparable(t *testing.T, raw worldFrameSnapshot) worldFrameSnapshot {
	t.Helper()
	b, err := hex.DecodeString(raw.Clock.Window)
	if err != nil || len(b) != 64 {
		t.Fatal("invalid frame clock window")
	}
	// F554 is the hardware normalizer's scratch index, explicitly outside the
	// trusted normalized-sample API. No clock, RNG or palette output is masked.
	b[4], b[5] = 0, 0
	raw.Clock.Window = hex.EncodeToString(b)
	return raw
}

type worldFrameCall struct {
	Kind, Name    string
	Before, After worldFrameSnapshot
	Sampled       *[8]ControllerFrame
	Reset         bool
}

func readWorldFrame(t *testing.T, name string, target any) int {
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
		t.Fatal("unverified frame boundary evidence")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
	return m.Records
}

func frameLibrary(t *testing.T) ArenaRuntimeLibrary {
	t.Helper()
	raw, err := os.ReadFile("../data/arena-runtime-rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct{ Library ArenaRuntimeLibrary }
	if err := json.Unmarshal(raw, &data); err != nil || len(data.Library.FadeAdjust) != 8 {
		t.Fatal("missing verified palette tables", err)
	}
	return data.Library
}

func applyFrameCall(state WorldFrameState, palette ArenaPaletteState, row worldFrameCall, library ArenaRuntimeLibrary) (WorldFrameState, ArenaPaletteState, WorldFrameOutcome, error) {
	if row.Kind == "begin" {
		var samples [8]ControllerFrame
		if row.Sampled != nil {
			samples = *row.Sampled
		}
		updated, outcome, err := BeginWorldFrame(state, samples)
		return updated, palette, outcome, err
	}
	if row.Kind == "finish" {
		updated, err := FinishWorldPalette(palette, library)
		return state, updated, WorldFrameOutcome{}, err
	}
	panic("unknown original frame stage")
}

// Complete original boundary fixtures and these tests precede world_frame.go.
func TestWorldFrameOriginalBoundaries(t *testing.T) {
	library := frameLibrary(t)
	for _, name := range []string{"world-frame-oracle", "world-frame-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []worldFrameCall
			count := readWorldFrame(t, name, &rows)
			if len(rows) != count {
				t.Fatal("missing original frame boundaries")
			}
			kinds, resets := map[string]int{}, 0
			for i, row := range rows {
				state, palette, outcome, err := applyFrameCall(frameState(row.Before), row.Before.Palette, row, library)
				encoded := encodeWorldFrame(t, state, palette, row.Before)
				if err != nil || state != frameState(row.After) || palette != row.After.Palette || !reflect.DeepEqual(frameComparable(t, encoded), frameComparable(t, row.After)) ||
					row.Kind == "begin" && (outcome.InputConsumed != (row.Sampled != nil) || outcome.ResetRequested != row.Reset) {
					t.Fatalf("frame boundary %d %s/%s differs: %v got=%+v want=%+v", i, row.Kind, row.Name, err, state, frameState(row.After))
				}
				kinds[row.Kind]++
				if outcome.ResetRequested {
					resets++
				}
			}
			if len(kinds) != 2 || name == "world-frame-boundaries" && resets == 0 {
				t.Fatal("missing prefix/palette/reset coverage")
			}
			t.Logf("original frame boundaries=%d stages=%v reset requests=%d", count, kinds, resets)
		})
	}
}

func TestWorldFrameSingleInitialState(t *testing.T) {
	var streams []struct {
		Initial worldFrameSnapshot
		Calls   []worldFrameCall
	}
	count := readWorldFrame(t, "world-frame-sequences", &streams)
	if len(streams) != count || count != 4 {
		t.Fatal("missing frame boundary streams")
	}
	library := frameLibrary(t)
	steps := 0
	for _, stream := range streams {
		state, palette, encoded := frameState(stream.Initial), stream.Initial.Palette, stream.Initial
		for i, call := range stream.Calls {
			var err error
			state, palette, _, err = applyFrameCall(state, palette, call, library)
			encoded = encodeWorldFrame(t, state, palette, encoded)
			if err != nil || state != frameState(call.After) || palette != call.After.Palette || !reflect.DeepEqual(frameComparable(t, encoded), frameComparable(t, call.After)) {
				t.Fatalf("frame stream step %d differs: %v", i, err)
			}
			steps++
		}
	}
	if steps != 320 {
		t.Fatal("missing continuous boundary calls")
	}
}

func TestWorldFrameErrorsAreAtomic(t *testing.T) {
	state := WorldFrameState{Clock: WorldClockState{PauseFlags: 1, ControllerCount: 9, Frame: 17}, RandomSeed: 3}
	updated, outcome, err := BeginWorldFrame(state, [8]ControllerFrame{})
	if err == nil || updated != state || outcome != (WorldFrameOutcome{}) {
		t.Fatal("bad pause configuration leaked prefix state")
	}
	palette := ArenaPaletteState{Flags: 1, Mode: 0x100, Step: 0x100, Target: 0xfff}
	got, err := FinishWorldPalette(palette, ArenaRuntimeLibrary{})
	if err == nil || got != palette {
		t.Fatal("missing late fade data leaked progression")
	}
}
