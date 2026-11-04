package engine

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type characterCommandOriginalCall struct {
	CharacterID   uint8 `json:"character_id"`
	Before, After CommandState
	ActorBefore   string `json:"actor_before"`
	ActorAfter    string `json:"actor_after"`
	RawInput      uint8  `json:"raw_input"`
}

// All original complete-preparation fixtures and these tests precede
// character_commands.go. Unknown bytes remain in the original input adapter;
// every byte of each actual original 256-byte result must match.
func TestCharacterCommandsOriginalCompleteCalls(t *testing.T) {
	for _, name := range []string{"character-commands-oracle", "character-commands-boundaries"} {
		t.Run(name, func(t *testing.T) {
			fixture, count := readCommandFixture(t, name)
			var rows []characterCommandOriginalCall
			if err := json.Unmarshal(fixture.Calls, &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != count {
				t.Fatal("missing complete character-preparation calls")
			}
			oldLibrary, _ := json.Marshal(fixture.Library)
			seen := map[uint8]int{}
			for i, row := range rows {
				before, err := hex.DecodeString(row.ActorBefore)
				if err != nil || len(before) != 256 || before[0x58] != row.CharacterID {
					t.Fatal("invalid original character input", err)
				}
				after, err := hex.DecodeString(row.ActorAfter)
				if err != nil || len(after) != 256 {
					t.Fatal("invalid original character result", err)
				}
				got, err := PrepareCharacterCommands(row.CharacterID, row.Before, fixture.Library)
				if err != nil || got != row.After || !bytes.Equal(encodeCommandState(got, before), after) {
					t.Fatalf("complete preparation %d, source %d: got %+v want %+v err %v", i, row.CharacterID, got, row.After, err)
				}
				seen[row.CharacterID]++
			}
			if name == "character-commands-boundaries" && (len(seen) != 11 || count != 14080) {
				t.Fatal("missing all-character preparation matrix", seen, count)
			}
			newLibrary, _ := json.Marshal(fixture.Library)
			if !bytes.Equal(oldLibrary, newLibrary) {
				t.Fatal("character preparation mutated immutable programs")
			}
			t.Logf("complete original calls=%d, source-character counts=%v", count, seen)
		})
	}
}

func TestCharacterCommandsSingleInitialState(t *testing.T) {
	fixture, count := readCommandFixture(t, "character-commands-sequences")
	var streams []struct {
		CharacterID uint8 `json:"character_id"`
		Initial     CommandState
		Calls       []characterCommandOriginalCall
	}
	if err := json.Unmarshal(fixture.Calls, &streams); err != nil {
		t.Fatal(err)
	}
	if len(streams) != count || count != 33 {
		t.Fatal("missing all-character continuous streams")
	}
	seen, steps := map[uint8]int{}, 0
	for i, stream := range streams {
		if len(stream.Calls) == 0 {
			t.Fatal("empty original stream")
		}
		state := stream.Initial
		actor, err := hex.DecodeString(stream.Calls[0].ActorBefore)
		if err != nil || len(actor) != 256 {
			t.Fatal("invalid stream initial actor", err)
		}
		for j, call := range stream.Calls {
			state.RawHeld = call.RawInput // Only the actual input changes externally.
			state, err = PrepareCharacterCommands(stream.CharacterID, state, fixture.Library)
			actor = encodeCommandState(state, actor)
			want, decodeErr := hex.DecodeString(call.ActorAfter)
			if err != nil || decodeErr != nil || state != call.After || !bytes.Equal(actor, want) {
				t.Fatalf("character stream %d source %d step %d differs: err=%v decode=%v", i, stream.CharacterID, j, err, decodeErr)
			}
			steps++
		}
		seen[stream.CharacterID]++
	}
	for id := uint8(0); id < 11; id++ {
		if seen[id] != 3 {
			t.Fatal("missing facing streams for source", id)
		}
	}
	t.Logf("single-initial-state streams=%d steps=%d", count, steps)
}

func TestCharacterCommandsInvalidInputsAreAtomic(t *testing.T) {
	fixture, _ := readCommandFixture(t, "character-commands-oracle")
	raw, err := os.ReadFile("../data/command-programs.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Characters []struct {
			ID       uint8 `json:"character_id"`
			Bindings []struct {
				Slot    uint8
				Program uint32
			}
		}
	}
	if err := json.Unmarshal(raw, &catalog); err != nil || len(catalog.Characters) != 11 {
		t.Fatal("invalid original command catalog", err)
	}
	base := CommandState{RawHeld: 255, Facing: 128, Request: 99}
	for _, id := range []uint8{11, 255} {
		got, err := PrepareCharacterCommands(id, base, fixture.Library)
		if err == nil || got != base {
			t.Fatal("invalid source character changed state")
		}
	}
	for _, character := range catalog.Characters {
		state := base
		for _, b := range character.Bindings {
			state.Indices[b.Slot] = uint8(len(fixture.Library[b.Program].Steps) - 1)
			state.Timeouts[b.Slot] = 17
		}
		last := character.Bindings[len(character.Bindings)-1]
		missing := make(CommandLibrary, len(fixture.Library))
		for id, program := range fixture.Library {
			missing[id] = program
		}
		delete(missing, last.Program)
		got, err := PrepareCharacterCommands(character.ID, state, missing)
		if err == nil || got != state {
			t.Fatal("missing late program leaked earlier slot completion", character.ID)
		}
		invalid := state
		invalid.Indices[last.Slot] = uint8(len(fixture.Library[last.Program].Steps))
		got, err = PrepareCharacterCommands(character.ID, invalid, fixture.Library)
		if err == nil || got != invalid {
			t.Fatal("invalid late slot leaked earlier completion", character.ID)
		}
	}
}
