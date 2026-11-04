package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

type commandOriginalCall struct {
	Kind          string
	Slot          uint8
	Program       uint32
	Data          *CommandProgram
	Before, After CommandState
	ActorBefore   string `json:"actor_before"`
	ActorAfter    string `json:"actor_after"`
	RawInput      uint8  `json:"raw_input"`
}
type commandOriginalFixture struct {
	Library  CommandLibrary
	Bindings []struct {
		Slot    uint8
		Kind    string
		Program uint32
	}
	Calls json.RawMessage
}

// These tests and original natural/isolated fixtures precede commands.go.
func readCommandFixture(t *testing.T, name string) (commandOriginalFixture, int) {
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
	mraw, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass     bool
		Records  int
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
		Baseline bool   `json:"frames_match_original_core"`
	}
	if err = json.Unmarshal(mraw, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(compressed)
	if !m.Pass || !m.Baseline || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original command fixture")
	}
	var f commandOriginalFixture
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f, m.Records
}

func encodeCommandState(state CommandState, previous []byte) []byte {
	b := append([]byte(nil), previous...)
	b[0x61], b[0x6b], b[0x74] = state.Facing, state.Request, state.RawHeld
	copy(b[0xa8:0xb0], state.Timeouts[:])
	copy(b[0xb0:0xb8], state.Indices[:])
	return b
}

func TestCommandsOriginalCalls(t *testing.T) {
	for _, name := range []string{"commands-oracle", "commands-boundaries"} {
		t.Run(name, func(t *testing.T) {
			fixture, count := readCommandFixture(t, name)
			var rows []commandOriginalCall
			if err := json.Unmarshal(fixture.Calls, &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != count {
				t.Fatal("missing command calls")
			}
			originalLibrary, _ := json.Marshal(fixture.Library)
			for i, r := range rows {
				var got CommandState
				var err error
				if r.Kind == "chu" {
					got, err = PrepareChuCommands(r.Before, fixture.Library)
				} else {
					kind := CommandDirection
					if r.Kind == "buttons" {
						kind = CommandButtons
					} else if r.Kind != "direction" {
						t.Fatal("unknown recognizer", r.Kind)
					}
					program, ok := fixture.Library[r.Program]
					if r.Data != nil {
						program = *r.Data
						ok = true
					}
					if !ok {
						t.Fatal("missing command program", r.Program)
					}
					got, err = RecognizeCommand(r.Before, r.Slot, kind, program)
				}
				before, e := hex.DecodeString(r.ActorBefore)
				if e != nil {
					t.Fatal(e)
				}
				after, e := hex.DecodeString(r.ActorAfter)
				if e != nil {
					t.Fatal(e)
				}
				if err != nil || got != r.After || !bytes.Equal(encodeCommandState(got, before), after) {
					t.Fatalf("command %d %s: got %+v want %+v err %v", i, r.Kind, got, r.After, err)
				}
			}
			updatedLibrary, _ := json.Marshal(fixture.Library)
			if !bytes.Equal(originalLibrary, updatedLibrary) {
				t.Fatal("command evaluation mutated read-only programs")
			}
		})
	}
}

func TestCommandsSingleInitialState(t *testing.T) {
	fixture, count := readCommandFixture(t, "commands-sequences")
	var rows []struct {
		Initial CommandState
		Calls   []commandOriginalCall
	}
	if err := json.Unmarshal(fixture.Calls, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != count || count != 12 {
		t.Fatal("missing continuous command streams")
	}
	for i, sequence := range rows {
		state := sequence.Initial
		if len(sequence.Calls) == 0 {
			t.Fatal("empty command stream")
		}
		actor, err := hex.DecodeString(sequence.Calls[0].ActorBefore)
		if err != nil {
			t.Fatal(err)
		}
		for j, step := range sequence.Calls {
			state.RawHeld = step.RawInput // Only the actual input changes externally.
			state, err = PrepareChuCommands(state, fixture.Library)
			actor = encodeCommandState(state, actor)
			expected, e := hex.DecodeString(step.ActorAfter)
			if e != nil {
				t.Fatal(e)
			}
			if err != nil || state != step.After || !bytes.Equal(actor, expected) {
				t.Fatalf("command sequence %d step %d: got %+v want %+v err %v", i, j, state, step.After, err)
			}
		}
	}
}

func TestCommandsMalformedInputsAreAtomic(t *testing.T) {
	base := CommandState{Request: 99, RawHeld: 8}
	base.Timeouts[0] = 7
	valid := CommandProgram{Window: 8, Steps: []uint8{8, 0, 8, 255}}
	long := make([]uint8, 257)
	long[256] = 255
	for _, c := range []struct {
		State   CommandState
		Slot    uint8
		Kind    CommandKind
		Program CommandProgram
	}{
		{base, 8, CommandDirection, valid}, {base, 0, CommandKind(255), valid}, {base, 0, CommandDirection, CommandProgram{}},
		{base, 0, CommandDirection, CommandProgram{Steps: []uint8{8}}}, {base, 0, CommandDirection, CommandProgram{Steps: []uint8{255, 8, 255}}}, {base, 0, CommandDirection, CommandProgram{Steps: long}},
		{CommandState{Indices: [8]uint8{4}}, 0, CommandDirection, valid},
	} {
		got, err := RecognizeCommand(c.State, c.Slot, c.Kind, c.Program)
		if err == nil || got != c.State {
			t.Fatal("malformed command changed state")
		}
	}
	fixture, _ := readCommandFixture(t, "commands-oracle")
	for _, binding := range fixture.Bindings {
		base.Indices[binding.Slot] = uint8(len(fixture.Library[binding.Program].Steps) - 1)
	}
	delete(fixture.Library, 0x1eacf) // Last slot: prior completed slots must not leak.
	got, err := PrepareChuCommands(base, fixture.Library)
	if err == nil || got != base {
		t.Fatal("missing late program caused partial preparation")
	}
}
