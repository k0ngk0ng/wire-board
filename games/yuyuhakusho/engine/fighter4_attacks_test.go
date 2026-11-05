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

type fighter4AttackSnapshot struct {
	chuAttackSnapshot
	WorldBytes     string `json:"world_bytes"`
	ChildrenBytes  string `json:"children_bytes"`
	EnergyFraction uint16 `json:"energy_fraction"`
	RandomSeed     uint32 `json:"random_seed"`
}

type fighter4AttackCall struct {
	Kind, Name    string
	ObjectID      uint16 `json:"object_id"`
	CharacterID   uint8  `json:"source_character_id"`
	EntrySP       uint32 `json:"entry_sp"`
	ExitSP        uint32 `json:"exit_sp"`
	Before, After fighter4AttackSnapshot
	External      struct {
		Held, Pressed, Changed uint8
	} `json:"external_controls"`
}

type fighter4AttackManifest struct {
	Pass         bool
	Records      int
	Calls        int
	Baseline     bool           `json:"frames_match_original_core"`
	SHA          string         `json:"fixture_sha256"`
	Gzip         string         `json:"gzip_sha256"`
	Constructors map[string]int `json:"constructor_counts"`
	RNGCalls     int            `json:"rng_calls"`
}

func decodeFighter4Attack(t *testing.T, raw fighter4AttackSnapshot, id uint16) ChuAttackState {
	t.Helper()
	a := attackContactBytes(t, raw.ActorBytes, 256)
	world := attackContactBytes(t, raw.WorldBytes, 4096)
	offset := int(id) - 0x8400
	if offset < 0 || offset+1024 > len(world) || a[0x58] != 4 || !bytes.Equal(world[offset:offset+256], a) ||
		hex.EncodeToString(world[offset+256:offset+1024]) != raw.ChildrenBytes ||
		hex.EncodeToString(world[offset+256:offset+512]) != raw.ChildBytes {
		t.Fatal("unbound source4 actor/children/world")
	}
	state := decodeChuAttack(t, raw.chuAttackSnapshot, id)
	if binary.BigEndian.Uint16(a[0xba:]) != raw.EnergyFraction {
		t.Fatal("unbound energy fraction")
	}
	if !reflect.DeepEqual(encodeFighter4Attack(t, state, raw), raw) {
		t.Fatal("source4 semantic input differs from full original bytes")
	}
	return state
}

func encodeFighter4Attack(t *testing.T, state ChuAttackState, previous fighter4AttackSnapshot) fighter4AttackSnapshot {
	t.Helper()
	raw := previous
	raw.chuAttackSnapshot = encodeChuAttack(t, state, previous.chuAttackSnapshot)
	a := attackContactBytes(t, raw.ActorBytes, 256)
	binary.BigEndian.PutUint16(a[0x78:], uint16(state.Body.Health))
	binary.BigEndian.PutUint16(a[0x7e:], uint16(state.Body.Auxiliary7E))
	binary.BigEndian.PutUint16(a[0x80:], uint16(state.Body.Auxiliary80))
	a[0xc8] = state.Body.TransitionDelay
	raw.ActorBytes = hex.EncodeToString(a)
	world := attackContactBytes(t, previous.WorldBytes, 4096)
	offset := int(state.ObjectID) - 0x8400
	copy(world[offset:offset+256], a)
	raw.WorldBytes = hex.EncodeToString(world)
	raw.ChildrenBytes = hex.EncodeToString(world[offset+256 : offset+1024])
	return raw
}

// Original complete-return fixtures and this test precede the source4 module.
// Stream the original JSON so the full worlds are not retained twice in memory.
func visitFighter4Attacks(t *testing.T, name string, visit func(*json.Decoder)) fighter4AttackManifest {
	t.Helper()
	manifestBytes, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest fighter4AttackManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	if !manifest.Pass || !manifest.Baseline || manifest.RNGCalls != 0 || hex.EncodeToString(zh[:]) != manifest.Gzip {
		t.Fatal("unverified original source4 selector fixture")
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
		t.Fatal("unbound complete original source4 fixture")
	}
	return manifest
}

func TestFighter4AttacksOriginalFullCalls(t *testing.T) {
	for _, name := range []string{"fighter4-attacks-natural", "fighter4-attacks-oracle", "fighter4-attacks-boundaries", "fighter4-attacks-special-natural"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			modes, sources := map[string]int{}, map[uint16]int{}
			manifest := visitFighter4Attacks(t, name, func(decoder *json.Decoder) {
				for decoder.More() {
					var row fighter4AttackCall
					if err := decoder.Decode(&row); err != nil {
						t.Fatal(err)
					}
					if row.CharacterID != 4 || row.ExitSP != row.EntrySP+4 {
						t.Fatal("not a complete source4 return")
					}
					input := decodeFighter4Attack(t, row.Before, row.ObjectID)
					copy := decodeFighter4Attack(t, row.Before, row.ObjectID)
					got, err := applyFighter4Attack(t, input, row.Kind)
					want := decodeFighter4Attack(t, row.After, row.ObjectID)
					if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeFighter4Attack(t, got, row.Before), row.After) {
						t.Fatalf("source4 %d %s/%s differs: %v; body=%+v want=%+v energy=%d/%d gravity=%x/%x", calls, row.Kind, row.Name, err, got.Body, want.Body, got.Energy, want.Energy, got.Body.Gravity, want.Body.Gravity)
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
				t.Fatal("missing source4 selector calls")
			}
			if (name == "fighter4-attacks-oracle" || name == "fighter4-attacks-boundaries") && (len(modes) != 6 || len(sources) != 4) {
				t.Fatal("missing selector or object-address coverage")
			}
			if name == "fighter4-attacks-boundaries" {
				for _, entry := range []string{"0x16e36", "0x18812", "0x18828", "0x1883e", "0x18854", "0x1886a", "0x18880", "0x188d8", "0x188ee", "0x18930", "0x1904e", "0x190a8", "0x190d2", "0x190f0", "0x1911a", "0x1913c", "0x19190", "0x191d4", "0x191ec"} {
					if manifest.Constructors[entry] == 0 {
						t.Fatal("missing original constructor/throw entry", entry)
					}
				}
			}
			t.Logf("complete original source4 calls=%d modes=%v objects=%v", calls, modes, sources)
		})
	}
}

func TestFighter4AttacksSingleInitialState(t *testing.T) {
	streams, steps := 0, 0
	manifest := visitFighter4Attacks(t, "fighter4-attacks-sequences", func(decoder *json.Decoder) {
		for decoder.More() {
			var stream struct {
				Kind    string
				Initial fighter4AttackSnapshot
				Calls   []fighter4AttackCall
			}
			if err := decoder.Decode(&stream); err != nil || len(stream.Calls) == 0 {
				t.Fatal("missing original stream", err)
			}
			state := decodeFighter4Attack(t, stream.Initial, stream.Calls[0].ObjectID)
			encoded := stream.Initial
			for index, call := range stream.Calls {
				state.Controls.Held = state.Controls.Held&0xff00 | Buttons(call.External.Held)
				state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(call.External.Pressed)
				state.Controls.Changed = state.Controls.Changed&0xff00 | Buttons(call.External.Changed)
				encoded = encodeFighter4Attack(t, state, encoded)
				if !reflect.DeepEqual(encoded, call.Before) {
					t.Fatal("original stream changed an undeclared intermediate state")
				}
				var err error
				state, err = applyFighter4Attack(t, state, call.Kind)
				encoded = encodeFighter4Attack(t, state, encoded)
				if err != nil || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeFighter4Attack(t, call.After, call.ObjectID)) {
					t.Fatalf("source4 stream %s/%d step%d differs: %v", stream.Kind, streams, index, err)
				}
				steps++
			}
			streams++
		}
	})
	if streams != manifest.Records || streams != 84 || steps != manifest.Calls || steps != 1344 {
		t.Fatal("missing continuous source4 evidence")
	}
}

func TestFighter4AttacksInvalidInputIsAtomic(t *testing.T) {
	var base ChuAttackState
	visitFighter4Attacks(t, "fighter4-attacks-oracle", func(decoder *json.Decoder) {
		for decoder.More() {
			var row fighter4AttackCall
			if err := decoder.Decode(&row); err != nil {
				t.Fatal(err)
			}
			if base.ObjectID == 0 {
				base = decodeFighter4Attack(t, row.Before, row.ObjectID)
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
		got, err := SelectFighter4Attack(input, mode)
		if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
			t.Fatal("invalid source4 input leaked actor/pool/throw changes")
		}
	}
	for _, mode := range []ChuAttackMode{ChuGroundAttack} {
		for _, offset := range []uint16{1, uint16(len(base.Throws.Records) * 32)} {
			input := base
			input.Body.Request = 0
			input.Controls.Held, input.Controls.Pressed = ButtonRight, ButtonA
			input.Throws.Offset = offset
			got, err := SelectFighter4Attack(input, mode)
			if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
				t.Fatal("late throw storage failure leaked changes")
			}
		}
	}
}

// Air requests other than3 leave even branch-unused invalid storage untouched.
func TestFighter4AirSpecialPreservesUnobservedState(t *testing.T) {
	input := ChuAttackState{}
	input.Body.Request, input.Body.Health = 6, -32768
	input.Controls.Held, input.Controls.Pressed, input.Controls.Changed = 0xffff, 0xffff, 0xffff
	input.ChildSize, input.Pool.Cursor, input.Throws.Offset = 0xffff, 1, 1
	input.Energy, input.GravityParameter = -32768, -2147483648
	got, err := SelectFighter4Attack(input, ChuAirSpecial)
	if err != nil || !reflect.DeepEqual(got, input) {
		t.Fatal("source4 empty air-special selector read or changed unobserved fields", err)
	}
}

func applyFighter4Attack(t *testing.T, state ChuAttackState, kind string) (ChuAttackState, error) {
	t.Helper()
	switch kind {
	case "rush-finish":
		return FinishFighter4Rush(state), nil
	case "dragon-followup":
		return BeginFighter4DragonFollowup(state), nil
	default:
		return SelectFighter4Attack(state, chuAttackMode(t, kind))
	}
}
