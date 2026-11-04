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

type fighter10AttackSnapshot struct {
	chuAttackSnapshot
	WorldBytes     string `json:"world_bytes"`
	ChildrenBytes  string `json:"children_bytes"`
	EnergyFraction uint16 `json:"energy_fraction"`
	RandomSeed     uint32 `json:"random_seed"`
	Aux48          uint32 `json:"aux48"`
	Aux48High      uint8  `json:"aux48_high"`
}

type fighter10AttackCall struct {
	Kind, Name    string
	ObjectID      uint16 `json:"object_id"`
	CharacterID   uint8  `json:"source_character_id"`
	EntrySP       uint32 `json:"entry_sp"`
	ExitSP        uint32 `json:"exit_sp"`
	Before, After fighter10AttackSnapshot
	External      struct {
		Held, Pressed, Changed uint8
	} `json:"external_controls"`
}

type fighter10AttackManifest struct {
	Pass         bool
	Records      int
	Calls        int
	Baseline     bool           `json:"frames_match_original_core"`
	SHA          string         `json:"fixture_sha256"`
	Gzip         string         `json:"gzip_sha256"`
	Constructors map[string]int `json:"constructor_counts"`
	RNGCalls     int            `json:"rng_calls"`
}

func decodeFighter10Attack(t *testing.T, raw fighter10AttackSnapshot, id uint16) Fighter10AttackState {
	t.Helper()
	a := attackContactBytes(t, raw.ActorBytes, 256)
	world := attackContactBytes(t, raw.WorldBytes, 4096)
	offset := int(id) - 0x8400
	if offset < 0 || offset+1024 > len(world) || a[0x58] != 10 || !bytes.Equal(world[offset:offset+256], a) ||
		hex.EncodeToString(world[offset+256:offset+1024]) != raw.ChildrenBytes ||
		hex.EncodeToString(world[offset+256:offset+512]) != raw.ChildBytes {
		t.Fatal("unbound source10 actor/children/world")
	}
	state := Fighter10AttackState{ChuAttackState: decodeChuAttack(t, raw.chuAttackSnapshot, id),
		Aux48High: a[0x48], EnergyFraction: binary.BigEndian.Uint16(a[0xba:]), RandomSeed: raw.RandomSeed}
	if state.Aux48High != raw.Aux48High || state.EnergyFraction != raw.EnergyFraction || binary.BigEndian.Uint32(a[0x48:]) != raw.Aux48 {
		t.Fatal("unbound energy fraction or Aux48 byte view")
	}
	if !reflect.DeepEqual(encodeFighter10Attack(t, state, raw), raw) {
		t.Fatal("source10 semantic input differs from full original bytes")
	}
	return state
}

func encodeFighter10Attack(t *testing.T, state Fighter10AttackState, previous fighter10AttackSnapshot) fighter10AttackSnapshot {
	t.Helper()
	raw := previous
	raw.chuAttackSnapshot = encodeChuAttack(t, state.ChuAttackState, previous.chuAttackSnapshot)
	a := attackContactBytes(t, raw.ActorBytes, 256)
	a[0x48] = state.Aux48High // Caller owns the canonical Aux48 low 24 bits.
	binary.BigEndian.PutUint16(a[0xba:], state.EnergyFraction)
	binary.BigEndian.PutUint16(a[0x78:], uint16(state.Body.Health))
	binary.BigEndian.PutUint16(a[0x7e:], uint16(state.Body.Auxiliary7E))
	binary.BigEndian.PutUint16(a[0x80:], uint16(state.Body.Auxiliary80))
	a[0xc8] = state.Body.TransitionDelay
	raw.ActorBytes = hex.EncodeToString(a)
	raw.Aux48High, raw.Aux48 = state.Aux48High, binary.BigEndian.Uint32(a[0x48:])
	raw.EnergyFraction, raw.RandomSeed = state.EnergyFraction, state.RandomSeed
	world := attackContactBytes(t, previous.WorldBytes, 4096)
	offset := int(state.ObjectID) - 0x8400
	copy(world[offset:offset+256], a)
	raw.WorldBytes = hex.EncodeToString(world)
	raw.ChildrenBytes = hex.EncodeToString(world[offset+256 : offset+1024])
	return raw
}

// Original complete-return fixtures and this test precede the source10 module.
// Stream the original JSON so the full worlds are not retained twice in memory.
func visitFighter10Attacks(t *testing.T, name string, visit func(*json.Decoder)) fighter10AttackManifest {
	t.Helper()
	manifestBytes, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest fighter10AttackManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	if !manifest.Pass || !manifest.Baseline || manifest.RNGCalls != 0 || hex.EncodeToString(zh[:]) != manifest.Gzip {
		t.Fatal("unverified original source10 selector fixture")
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
		t.Fatal("unbound complete original source10 fixture")
	}
	return manifest
}

func TestFighter10AttacksOriginalFullCalls(t *testing.T) {
	for _, name := range []string{"fighter10-attacks-natural", "fighter10-attacks-oracle", "fighter10-attacks-boundaries"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			modes, sources := map[string]int{}, map[uint16]int{}
			manifest := visitFighter10Attacks(t, name, func(decoder *json.Decoder) {
				for decoder.More() {
					var row fighter10AttackCall
					if err := decoder.Decode(&row); err != nil {
						t.Fatal(err)
					}
					if row.CharacterID != 10 || row.ExitSP != row.EntrySP+4 {
						t.Fatal("not a complete source10 return")
					}
					input := decodeFighter10Attack(t, row.Before, row.ObjectID)
					copy := decodeFighter10Attack(t, row.Before, row.ObjectID)
					got, err := SelectFighter10Attack(input, chuAttackMode(t, row.Kind))
					want := decodeFighter10Attack(t, row.After, row.ObjectID)
					if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeFighter10Attack(t, got, row.Before), row.After) {
						t.Fatalf("source10 %d %s/%s differs: %v; body=%+v want=%+v energy=%d/%d aux=%x/%x", calls, row.Kind, row.Name, err, got.Body, want.Body, got.Energy, want.Energy, got.Aux48High, want.Aux48High)
					}
					if !reflect.DeepEqual(input, copy) || got.RandomSeed != input.RandomSeed || got.EnergyFraction != input.EnergyFraction {
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
			if name == "fighter10-attacks-natural" && calls == 0 {
				t.Log("passive original baseline matched; no source10 natural selector call in this input")
				return
			}
			if name != "fighter10-attacks-natural" && (len(modes) != 4 || len(sources) != 4) {
				t.Fatal("missing selector or object-address coverage")
			}
			if name == "fighter10-attacks-boundaries" {
				for _, entry := range []string{"0x18214", "0x1833c", "0x18812", "0x18828", "0x1883e", "0x18854", "0x1886a", "0x18880", "0x188d8", "0x188ee", "0x18930", "0x19ac0", "0x19aea", "0x19b14", "0x19b46", "0x19b78", "0x19ba0", "0x19bd8"} {
					if manifest.Constructors[entry] == 0 {
						t.Fatal("missing original constructor/throw entry", entry)
					}
				}
			}
			t.Logf("complete original source10 calls=%d modes=%v objects=%v", calls, modes, sources)
		})
	}
}

func TestFighter10AttacksSingleInitialState(t *testing.T) {
	streams, steps := 0, 0
	manifest := visitFighter10Attacks(t, "fighter10-attacks-sequences", func(decoder *json.Decoder) {
		for decoder.More() {
			var stream struct {
				Kind    string
				Initial fighter10AttackSnapshot
				Calls   []fighter10AttackCall
			}
			if err := decoder.Decode(&stream); err != nil || len(stream.Calls) == 0 {
				t.Fatal("missing original stream", err)
			}
			state := decodeFighter10Attack(t, stream.Initial, stream.Calls[0].ObjectID)
			encoded := stream.Initial
			for index, call := range stream.Calls {
				state.Controls.Held = state.Controls.Held&0xff00 | Buttons(call.External.Held)
				state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(call.External.Pressed)
				state.Controls.Changed = state.Controls.Changed&0xff00 | Buttons(call.External.Changed)
				encoded = encodeFighter10Attack(t, state, encoded)
				if !reflect.DeepEqual(encoded, call.Before) {
					t.Fatal("original stream changed an undeclared intermediate state")
				}
				var err error
				state, err = SelectFighter10Attack(state, chuAttackMode(t, call.Kind))
				encoded = encodeFighter10Attack(t, state, encoded)
				if err != nil || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeFighter10Attack(t, call.After, call.ObjectID)) {
					t.Fatalf("source10 stream %s/%d step%d differs: %v", stream.Kind, streams, index, err)
				}
				steps++
			}
			streams++
		}
	})
	if streams != manifest.Records || streams != 16 || steps != manifest.Calls || steps != 320 {
		t.Fatal("missing continuous source10 evidence")
	}
}

func TestFighter10AttacksInvalidInputIsAtomic(t *testing.T) {
	var base Fighter10AttackState
	visitFighter10Attacks(t, "fighter10-attacks-oracle", func(decoder *json.Decoder) {
		for decoder.More() {
			var row fighter10AttackCall
			if err := decoder.Decode(&row); err != nil {
				t.Fatal(err)
			}
			if base.ObjectID == 0 {
				base = decodeFighter10Attack(t, row.Before, row.ObjectID)
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
		got, err := SelectFighter10Attack(input, mode)
		if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
			t.Fatal("invalid source10 input leaked actor/pool/throw changes")
		}
	}
	for _, mode := range []ChuAttackMode{ChuGroundAttack, ChuAirAttack} {
		for _, offset := range []uint16{1, uint16(len(base.Throws.Records) * 32)} {
			input := base
			input.Body.Request = 0
			input.Controls.Held, input.Controls.Pressed = ButtonRight, ButtonA
			input.Throws.Offset = offset
			got, err := SelectFighter10Attack(input, mode)
			if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
				t.Fatal("late throw storage failure leaked changes")
			}
		}
	}
}
