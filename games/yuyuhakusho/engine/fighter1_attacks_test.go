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

type fighter1AttackSnapshot struct {
	chuAttackSnapshot
	WorldBytes     string `json:"world_bytes"`
	ChildrenBytes  string `json:"children_bytes"`
	EnergyFraction uint16 `json:"energy_fraction"`
	RandomSeed     uint32 `json:"random_seed"`
	Power          int16  `json:"power"`
}

type fighter1AttackCall struct {
	Kind, Name    string
	ObjectID      uint16 `json:"object_id"`
	CharacterID   uint8  `json:"source_character_id"`
	EntrySP       uint32 `json:"entry_sp"`
	ExitSP        uint32 `json:"exit_sp"`
	Before, After fighter1AttackSnapshot
	External      struct {
		Held, Pressed, Changed uint8
	} `json:"external_controls"`
}

type fighter1AttackManifest struct {
	Pass         bool
	Records      int
	Calls        int
	Baseline     bool           `json:"frames_match_original_core"`
	SHA          string         `json:"fixture_sha256"`
	Gzip         string         `json:"gzip_sha256"`
	Constructors map[string]int `json:"constructor_counts"`
	RNGCalls     int            `json:"rng_calls"`
}

func decodeFighter1Attack(t *testing.T, raw fighter1AttackSnapshot, id uint16) Fighter1AttackState {
	t.Helper()
	a := attackContactBytes(t, raw.ActorBytes, 256)
	world := attackContactBytes(t, raw.WorldBytes, 4096)
	offset := int(id) - 0x8400
	if offset < 0 || offset+1024 > len(world) || a[0x58] != 1 || !bytes.Equal(world[offset:offset+256], a) ||
		hex.EncodeToString(world[offset+256:offset+1024]) != raw.ChildrenBytes ||
		hex.EncodeToString(world[offset+256:offset+512]) != raw.ChildBytes {
		t.Fatal("unbound source1 actor/children/world")
	}
	state := Fighter1AttackState{ChuAttackState: decodeChuAttack(t, raw.chuAttackSnapshot, id),
		Power: int16(binary.BigEndian.Uint16(a[0x7c:]))}
	if state.Power != raw.Power || binary.BigEndian.Uint16(a[0xba:]) != raw.EnergyFraction {
		t.Fatal("unbound Power word or energy fraction")
	}
	if !reflect.DeepEqual(encodeFighter1Attack(t, state, raw), raw) {
		t.Fatal("source1 semantic input differs from full original bytes")
	}
	return state
}

func encodeFighter1Attack(t *testing.T, state Fighter1AttackState, previous fighter1AttackSnapshot) fighter1AttackSnapshot {
	t.Helper()
	raw := previous
	raw.chuAttackSnapshot = encodeChuAttack(t, state.ChuAttackState, previous.chuAttackSnapshot)
	a := attackContactBytes(t, raw.ActorBytes, 256)
	binary.BigEndian.PutUint16(a[0x7c:], uint16(state.Power))
	binary.BigEndian.PutUint16(a[0x78:], uint16(state.Body.Health))
	binary.BigEndian.PutUint16(a[0x7e:], uint16(state.Body.Auxiliary7E))
	binary.BigEndian.PutUint16(a[0x80:], uint16(state.Body.Auxiliary80))
	a[0xc8] = state.Body.TransitionDelay
	raw.ActorBytes = hex.EncodeToString(a)
	raw.Power = state.Power
	world := attackContactBytes(t, previous.WorldBytes, 4096)
	offset := int(state.ObjectID) - 0x8400
	copy(world[offset:offset+256], a)
	raw.WorldBytes = hex.EncodeToString(world)
	raw.ChildrenBytes = hex.EncodeToString(world[offset+256 : offset+1024])
	return raw
}

// Original complete-return fixtures and this test precede the source1 module.
// Stream the original JSON so the full worlds are not retained twice in memory.
func visitFighter1Attacks(t *testing.T, name string, visit func(*json.Decoder)) fighter1AttackManifest {
	t.Helper()
	manifestBytes, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest fighter1AttackManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	if !manifest.Pass || !manifest.Baseline || manifest.RNGCalls != 0 || hex.EncodeToString(zh[:]) != manifest.Gzip {
		t.Fatal("unverified original source1 selector fixture")
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	hash := sha256.New()
	reader := io.TeeReader(z, hash)
	decoder := json.NewDecoder(reader)
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		t.Fatal("invalid fixture object", err)
	}
	if token, err := decoder.Token(); err != nil || token != "calls" {
		t.Fatal("missing original call field", err)
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
		t.Fatal("missing original call array", err)
	}
	visit(decoder)
	if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
		t.Fatal("unterminated original calls", err)
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		t.Fatal("unterminated original fixture", err)
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != manifest.SHA {
		t.Fatal("unbound complete original source1 fixture")
	}
	return manifest
}

func TestFighter1AttacksOriginalFullCalls(t *testing.T) {
	for _, name := range []string{"fighter1-attacks-natural", "fighter1-attacks-oracle", "fighter1-attacks-boundaries", "fighter1-attacks-special-natural"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			modes, sources := map[string]int{}, map[uint16]int{}
			manifest := visitFighter1Attacks(t, name, func(decoder *json.Decoder) {
				for decoder.More() {
					var row fighter1AttackCall
					if err := decoder.Decode(&row); err != nil {
						t.Fatal(err)
					}
					if row.CharacterID != 1 || row.ExitSP != row.EntrySP+4 {
						t.Fatal("not a complete source1 return")
					}
					input := decodeFighter1Attack(t, row.Before, row.ObjectID)
					copy := decodeFighter1Attack(t, row.Before, row.ObjectID)
					got, err := SelectFighter1Attack(input, chuAttackMode(t, row.Kind))
					want := decodeFighter1Attack(t, row.After, row.ObjectID)
					if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeFighter1Attack(t, got, row.Before), row.After) {
						t.Fatalf("source1 %d %s/%s differs: %v; body=%+v want=%+v energy=%d/%d power=%d/%d", calls, row.Kind, row.Name, err, got.Body, want.Body, got.Energy, want.Energy, got.Power, want.Power)
					}
					if !reflect.DeepEqual(input, copy) || row.After.RandomSeed != row.Before.RandomSeed || row.After.EnergyFraction != row.Before.EnergyFraction {
						t.Fatal("selector changed caller storage, RNG or energy fraction")
					}
					calls++
					modes[row.Kind]++
					sources[row.ObjectID]++
				}
			})
			if calls != manifest.Records || calls != manifest.Calls {
				t.Fatal("missing complete original selector calls")
			}
			if calls == 0 {
				t.Fatal("missing source1 selector calls")
			}
			if (name == "fighter1-attacks-oracle" || name == "fighter1-attacks-boundaries") && (len(modes) != 4 || len(sources) != 4) {
				t.Fatal("missing selector or object-address coverage")
			}
			if name == "fighter1-attacks-boundaries" {
				for _, entry := range []string{"0x164ae", "0x18812", "0x18828", "0x1883e", "0x18854", "0x1886a", "0x18880", "0x188d8", "0x188ee", "0x18930", "0x18b7a", "0x18ba4", "0x18bbc", "0x18bea", "0x18c04", "0x18c36", "0x18c4e", "0x18c66", "0x18c96"} {
					if manifest.Constructors[entry] == 0 {
						t.Fatal("missing original constructor/throw entry", entry)
					}
				}
			}
			t.Logf("complete original source1 calls=%d modes=%v objects=%v", calls, modes, sources)
		})
	}
}

func TestFighter1AttacksSingleInitialState(t *testing.T) {
	streams, steps := 0, 0
	manifest := visitFighter1Attacks(t, "fighter1-attacks-sequences", func(decoder *json.Decoder) {
		for decoder.More() {
			var stream struct {
				Kind    string
				Initial fighter1AttackSnapshot
				Calls   []fighter1AttackCall
			}
			if err := decoder.Decode(&stream); err != nil || len(stream.Calls) == 0 {
				t.Fatal("missing original stream", err)
			}
			state := decodeFighter1Attack(t, stream.Initial, stream.Calls[0].ObjectID)
			encoded := stream.Initial
			for index, call := range stream.Calls {
				state.Controls.Held = state.Controls.Held&0xff00 | Buttons(call.External.Held)
				state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(call.External.Pressed)
				state.Controls.Changed = state.Controls.Changed&0xff00 | Buttons(call.External.Changed)
				encoded = encodeFighter1Attack(t, state, encoded)
				if !reflect.DeepEqual(encoded, call.Before) {
					t.Fatal("original stream changed an undeclared intermediate state")
				}
				var err error
				state, err = SelectFighter1Attack(state, chuAttackMode(t, call.Kind))
				encoded = encodeFighter1Attack(t, state, encoded)
				if err != nil || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeFighter1Attack(t, call.After, call.ObjectID)) {
					t.Fatalf("source1 stream %s/%d step%d differs: %v", stream.Kind, streams, index, err)
				}
				steps++
			}
			streams++
		}
	})
	if streams != manifest.Records || streams != 52 || steps != manifest.Calls || steps != 1040 {
		t.Fatal("missing continuous source1 evidence")
	}
}

func TestFighter1AttacksInvalidInputIsAtomic(t *testing.T) {
	var base Fighter1AttackState
	visitFighter1Attacks(t, "fighter1-attacks-oracle", func(decoder *json.Decoder) {
		for decoder.More() {
			var row fighter1AttackCall
			if err := decoder.Decode(&row); err != nil {
				t.Fatal(err)
			}
			if base.ObjectID == 0 {
				base = decodeFighter1Attack(t, row.Before, row.ObjectID)
			}
		}
	})
	for _, mode := range []ChuAttackMode{255, ChuGroundAttack} {
		input := base
		switch mode {
		case ChuGroundAttack:
			input.Body.Request = 8
			input.Pool.Cursor = 1
		}
		got, err := SelectFighter1Attack(input, mode)
		if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
			t.Fatal("invalid source1 input leaked actor/pool/throw changes")
		}
	}
	for _, mode := range []ChuAttackMode{ChuGroundAttack} {
		for _, offset := range []uint16{1, uint16(len(base.Throws.Records) * 32)} {
			input := base
			input.Body.Request = 0
			input.Controls.Held, input.Controls.Pressed = ButtonRight, ButtonA
			input.Throws.Offset = offset
			got, err := SelectFighter1Attack(input, mode)
			if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
				t.Fatal("late throw storage failure leaked changes")
			}
		}
	}
}

// 0x1668c is exactly RTS: even storage invalid for another mode is unobserved.
func TestFighter1AirSpecialPreservesUnobservedState(t *testing.T) {
	input := Fighter1AttackState{Power: -32768}
	input.Body.Request, input.Body.Health = 6, -32768
	input.Controls.Held, input.Controls.Pressed, input.Controls.Changed = 0xffff, 0xffff, 0xffff
	input.ChildSize, input.Pool.Cursor, input.Throws.Offset = 0xffff, 1, 1
	input.Energy, input.GravityParameter = -32768, -2147483648
	got, err := SelectFighter1Attack(input, ChuAirSpecial)
	if err != nil || !reflect.DeepEqual(got, input) {
		t.Fatal("source1 empty air-special selector read or changed unobserved fields", err)
	}
}
