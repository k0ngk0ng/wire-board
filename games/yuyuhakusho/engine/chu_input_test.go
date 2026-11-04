package engine

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type chuInputSnapshot struct {
	AI       chuAISnapshot
	Envelope envelopeSnapshot
}

func decodeChuInput(t *testing.T, raw chuInputSnapshot, id uint16) ChuInputState {
	t.Helper()
	if raw.Envelope.Actor != raw.AI.Actor {
		t.Fatal("input/AI adapters do not share the same source object")
	}
	return ChuInputState{Envelope: decodeEnvelopeInput(t, raw.Envelope), AI: decodeChuAI(t, raw.AI, id)}
}

func encodeChuInput(t *testing.T, s ChuInputState, previous chuInputSnapshot) chuInputSnapshot {
	t.Helper()
	raw := previous
	raw.Envelope = encodeEnvelopeInput(t, s.Envelope, previous.Envelope)
	aiBefore := previous.AI
	aiBefore.Actor = raw.Envelope.Actor
	world := chuAIBytes(t, aiBefore.World, 4096)
	copy(world[int(s.AI.ObjectID)-0x8400:], chuAIBytes(t, raw.Envelope.Actor, 256))
	aiBefore.World = hex.EncodeToString(world)
	raw.AI = encodeChuAI(t, s.AI, aiBefore)
	raw.Envelope.Actor = raw.AI.Actor
	return raw
}

type chuInputCall struct {
	Name, Route   string
	ObjectID      uint16 `json:"object_id"`
	Before, After chuInputSnapshot
	AICalls       int             `json:"ai_calls"`
	Records       int             `json:"record_calls"`
	External      ControllerFrame `json:"external_input"`
	Enemy         struct {
		ObjectID uint16 `json:"object_id"`
		X        int16
	} `json:"external_enemy"`
}

func readChuInput(t *testing.T, name string, target any) int {
	t.Helper()
	count := readEnvelopeFixture(t, name, target)
	b, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Canonical bool `json:"canonical_source_registers"`
		Routes    map[string]int
	}
	if err := json.Unmarshal(b, &m); err != nil || !m.Canonical {
		t.Fatal("unverified complete input source registers", err)
	}
	if name == "chu-input-boundaries" {
		for _, route := range []string{"disabled", "human", "ai", "human-record", "ai-record", "playback", "playback-stopped"} {
			if m.Routes[route] == 0 {
				t.Fatal("missing original full input route", route)
			}
		}
	}
	return count
}

// Complete15540 original returns and these tests precede chu_input.go.
func TestChuInputOriginalCompleteCalls(t *testing.T) {
	for _, name := range []string{"chu-input-oracle", "chu-input-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []chuInputCall
			count := readChuInput(t, name, &rows)
			if count != len(rows) {
				t.Fatal("missing complete original input returns")
			}
			aiCalls, records := 0, 0
			for i, row := range rows {
				input := decodeChuInput(t, row.Before, row.ObjectID)
				copy := decodeChuInput(t, row.Before, row.ObjectID)
				got, err := StepChuInput(input)
				want := decodeChuInput(t, row.After, row.ObjectID)
				if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeChuInput(t, got, row.Before), row.After) {
					t.Fatalf("complete input %d %s/%s differs: %v", i, row.Route, row.Name, err)
				}
				if !reflect.DeepEqual(input, copy) || got.Envelope.Controls != got.AI.Actor.Controls {
					t.Fatal("input mutated caller storage or split the shared control state")
				}
				if row.AICalls == 0 && got.AI.RandomSeed != input.AI.RandomSeed {
					t.Fatal("non-AI input consumed world RNG")
				}
				aiCalls += row.AICalls
				records += row.Records
			}
			t.Logf("complete input calls=%d AI executions=%d record continuations=%d", count, aiCalls, records)
		})
	}
}

func TestChuInputSingleInitialState(t *testing.T) {
	var streams []struct {
		Initial chuInputSnapshot
		Calls   []chuInputCall
	}
	count := readChuInput(t, "chu-input-sequences", &streams)
	if count != len(streams) || count != 24 {
		t.Fatal("missing full input streams")
	}
	steps := 0
	for _, stream := range streams {
		if len(stream.Calls) == 0 {
			t.Fatal("empty complete input stream")
		}
		state := decodeChuInput(t, stream.Initial, stream.Calls[0].ObjectID)
		encoded := stream.Initial
		for i, call := range stream.Calls {
			state.Envelope.Input = call.External
			index := int((call.Enemy.ObjectID - 0x8400) / 0x400)
			if index < 0 || index >= len(state.AI.Fighters) {
				t.Fatal("invalid declared external enemy")
			}
			state.AI.Fighters[index].X = call.Enemy.X
			var err error
			state, err = StepChuInput(state)
			encoded = encodeChuInput(t, state, encoded)
			if err != nil || !reflect.DeepEqual(state, decodeChuInput(t, call.After, call.ObjectID)) || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("complete input stream step %d differs: %v", i, err)
			}
			steps++
		}
	}
	if steps != 288 {
		t.Fatal("missing full input stream steps")
	}
}

func TestChuInputControlsAndFailuresAreAtomic(t *testing.T) {
	var rows []chuInputCall
	readChuInput(t, "chu-input-oracle", &rows)
	var chosen *chuInputCall
	for i := range rows {
		if rows[i].Route == "ai" {
			chosen = &rows[i]
			break
		}
	}
	if chosen == nil {
		t.Fatal("missing actual AI full-return case")
	}
	base := decodeChuInput(t, chosen.Before, chosen.ObjectID)
	base.AI.Actor.Controls = FighterControls{Held: 0xffff, Pressed: 0xffff, Changed: 0xffff, RawHeld: 99, RetainTicks: 255}
	got, err := StepChuInput(base)
	if err != nil || !reflect.DeepEqual(got, decodeChuInput(t, chosen.After, chosen.ObjectID)) {
		t.Fatal("envelope was not the authoritative shared control source", err)
	}
	base = decodeChuInput(t, chosen.Before, chosen.ObjectID)
	badAI := base
	badAI.AI.Actor.CharacterID = 0
	got, err = StepChuInput(badAI)
	if err == nil || !reflect.DeepEqual(got, badAI) {
		t.Fatal("failed AI leaked envelope clearing or RNG state")
	}
	badRecord := decodeChuInput(t, chosen.Before, chosen.ObjectID)
	badRecord.Envelope.Mode, badRecord.Envelope.Stopped = -1, 0
	badRecord.Envelope.Position = badRecord.Envelope.StreamBase + uint32(len(badRecord.Envelope.Stream)) - 2
	before := append([]byte(nil), badRecord.Envelope.Stream...)
	got, err = StepChuInput(badRecord)
	if err == nil || !reflect.DeepEqual(got, badRecord) || !reflect.DeepEqual(before, badRecord.Envelope.Stream) {
		t.Fatal("late record failure leaked real AI state or caller stream writes")
	}
	// Disabled input never executes or validates an unused AI state.
	badAI.Envelope.Disabled = 1
	got, err = StepChuInput(badAI)
	if err != nil || got.AI.RandomSeed != badAI.AI.RandomSeed || got.AI.Actor.CharacterID != 0 || got.Envelope.Controls.Held != 0 || got.Envelope.Controls.Pressed != 0 || got.Envelope.Controls.Changed != 0 || got.Envelope.Controls != got.AI.Actor.Controls {
		t.Fatal("disabled input ran an unused AI strategy", err)
	}
}
