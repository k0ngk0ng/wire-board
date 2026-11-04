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

type fighterAIOriginalCall struct {
	CharacterID   uint8  `json:"character_id"`
	ObjectID      uint16 `json:"object_id"`
	Name          string
	Before, After chuAISnapshot
	TargetID      uint16 `json:"external_target_id"`
	TargetX       int16  `json:"external_target_x"`
}

func decodeFighterAI(t *testing.T, raw chuAISnapshot, id uint16) FighterAIState {
	t.Helper()
	state := decodeChuAI(t, raw, id)
	world := chuAIBytes(t, raw.World, 4096)
	for i := range state.Fighters {
		state.Fighters[i].ChildAttackValue = world[i*0x400+0x165]
	}
	return state
}

func encodeFighterAI(t *testing.T, state FighterAIState, before chuAISnapshot) chuAISnapshot {
	t.Helper()
	raw := encodeChuAI(t, state, before)
	world := chuAIBytes(t, raw.World, 4096)
	for i, fighter := range state.Fighters {
		world[i*0x400+0x165] = fighter.ChildAttackValue
	}
	raw.World = hex.EncodeToString(world)
	off := int(state.ObjectID) - 0x8400
	raw.Actor = hex.EncodeToString(world[off : off+256])
	return raw
}

type fighterAIManifest struct {
	Pass       bool
	Records    int
	Calls      int
	Baseline   bool           `json:"frames_match_original_core"`
	SHA        string         `json:"fixture_sha256"`
	Gzip       string         `json:"gzip_sha256"`
	Characters map[string]int `json:"character_counts"`
	Odd        struct {
		Count int  `json:"observations"`
		High0 bool `json:"all_d1_high_zero"`
		N0    bool `json:"all_nonnegative_no_overflow"`
	} `json:"odd_source1_ccr"`
}

// The original all-character fixtures and these tests precede fighter_ai.go
// and the necessary shared-driver refactor in chu_ai.go. Calls are decoded
// one at a time, not held as hundreds of MB of duplicated original worlds.
func visitFighterAI(t *testing.T, name string, visit func(*json.Decoder)) fighterAIManifest {
	t.Helper()
	mb, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest fighterAIManifest
	if err := json.Unmarshal(mb, &manifest); err != nil {
		t.Fatal(err)
	}
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	if !manifest.Pass || !manifest.Baseline || manifest.Records == 0 || hex.EncodeToString(zh[:]) != manifest.Gzip {
		t.Fatal("unverified original all-character AI fixture")
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	h := sha256.New()
	reader := io.TeeReader(z, h)
	decoder := json.NewDecoder(reader)
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		t.Fatal("invalid original fixture object", err)
	}
	visited := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			t.Fatal(err)
		}
		if key == "calls" {
			if visited {
				t.Fatal("duplicate original call array")
			}
			visited = true
			if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
				t.Fatal("missing calls", err)
			}
			visit(decoder)
			if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
				t.Fatal("unterminated calls", err)
			}
		} else {
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				t.Fatal(err)
			}
		}
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') || !visited {
		t.Fatal("unterminated fixture", err)
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(h.Sum(nil)) != manifest.SHA {
		t.Fatal("unbound complete original fixture")
	}
	return manifest
}

func TestFighterAIOriginalCompleteCalls(t *testing.T) {
	for _, name := range []string{"fighter-ai-oracle", "fighter-ai-boundaries"} {
		t.Run(name, func(t *testing.T) {
			calls, seen := 0, map[uint8]int{}
			manifest := visitFighterAI(t, name, func(decoder *json.Decoder) {
				for decoder.More() {
					var row fighterAIOriginalCall
					if err := decoder.Decode(&row); err != nil {
						t.Fatal(err)
					}
					input := decodeFighterAI(t, row.Before, row.ObjectID)
					if input.Actor.CharacterID != row.CharacterID {
						t.Fatal("source-character binding differs")
					}
					got, err := StepFighterAI(input)
					want := decodeFighterAI(t, row.After, row.ObjectID)
					if err != nil || got != want || encodeFighterAI(t, got, row.Before) != row.After {
						t.Fatalf("AI %d source%d %s differs: err=%v got=%+v want=%+v seed=%x/%x", calls, row.CharacterID, row.Name, err, got.Actor, want.Actor, got.RandomSeed, want.RandomSeed)
					}
					if got.Fighters != input.Fighters {
						t.Fatal("AI modified the read-only visible world")
					}
					if row.CharacterID == 9 {
						old, oldErr := StepChuAI(input)
						if oldErr != nil || old != got {
							t.Fatal("Chu wrapper diverged from generic driver")
						}
					}
					calls++
					seen[row.CharacterID]++
				}
			})
			if calls != manifest.Records {
				t.Fatal("missing original full AI calls")
			}
			if name == "fighter-ai-boundaries" && (len(seen) != 11 || manifest.Odd.Count == 0 || !manifest.Odd.High0 || !manifest.Odd.N0) {
				t.Fatal("missing all-character or source1 CCR evidence")
			}
			t.Logf("complete original AI calls=%d characters=%v", calls, seen)
		})
	}
}

func TestFighterAISingleInitialState(t *testing.T) {
	streams, steps, seen := 0, 0, map[uint8]int{}
	manifest := visitFighterAI(t, "fighter-ai-sequences", func(decoder *json.Decoder) {
		for decoder.More() {
			var stream struct {
				CharacterID uint8 `json:"character_id"`
				Initial     chuAISnapshot
				Calls       []fighterAIOriginalCall
			}
			if err := decoder.Decode(&stream); err != nil || len(stream.Calls) == 0 {
				t.Fatal("empty original stream", err)
			}
			state := decodeFighterAI(t, stream.Initial, stream.Calls[0].ObjectID)
			encoded := stream.Initial
			for i, call := range stream.Calls {
				index, ok := chuAIIndex(call.TargetID)
				if !ok {
					t.Fatal("declared target is unavailable")
				}
				state.Fighters[index].X = call.TargetX // Only this declared external fact changes.
				var err error
				state, err = StepFighterAI(state)
				encoded = encodeFighterAI(t, state, encoded)
				if err != nil || state != decodeFighterAI(t, call.After, call.ObjectID) || encoded != call.After {
					t.Fatalf("AI stream%d source%d step%d differs: %v", streams, stream.CharacterID, i, err)
				}
				steps++
			}
			seen[stream.CharacterID]++
			streams++
		}
	})
	if streams != manifest.Records || streams != 33 || steps != manifest.Calls || steps != 792 || len(seen) != 11 {
		t.Fatal("missing all-character continuous evidence")
	}
	t.Logf("single-initial-state streams=%d steps=%d", streams, steps)
}

func TestFighterAIInvalidStateIsAtomic(t *testing.T) {
	base := FighterAIState{ObjectID: 0x9000, Difficulty: 6, RandomSeed: 1, Actor: FighterAIActor{CharacterID: 0, Energy: 100, Countdown: 10, Behavior: 2, FollowTarget: 0x8400}}
	for i := range base.Fighters {
		base.Fighters[i] = FighterAIFighter{Size: 256, Team: 1, Health: 223, X: 180, Y: 320, Ground: 320}
	}
	base.Fighters[3].Team = 0
	for _, mutate := range []func(*FighterAIState){func(s *FighterAIState) { s.Actor.CharacterID = 11 }, func(s *FighterAIState) { s.ObjectID = 0x8500 }, func(s *FighterAIState) { s.Difficulty = 1 }, func(s *FighterAIState) { s.Actor.TargetCursor = 1 }, func(s *FighterAIState) { s.Actor.Behavior = 255 }, func(s *FighterAIState) { s.Actor.Behavior = 16; s.Actor.Elapsed = 0; s.Actor.FollowTarget = 0x8500 }} {
		input := base
		mutate(&input)
		got, err := StepFighterAI(input)
		if err == nil || got != input {
			t.Fatal("invalid early/late AI input leaked state")
		}
	}
	if got, err := StepChuAI(base); err == nil || got != base {
		t.Fatal("legacy Chu wrapper accepted another character")
	}
}
