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

type fighter2AttackSnapshot struct {
	chuAttackSnapshot
	WorldBytes     string `json:"world_bytes"`
	ChildrenBytes  string `json:"children_bytes"`
	EnergyFraction uint16 `json:"energy_fraction"`
	RandomSeed     uint32 `json:"random_seed"`
	Aux48          uint32 `json:"aux48"`
	Aux48High      uint8  `json:"aux48_high"`
}

type fighter2AttackCall struct {
	Kind, Name    string
	ObjectID      uint16 `json:"object_id"`
	CharacterID   uint8  `json:"source_character_id"`
	EntrySP       uint32 `json:"entry_sp"`
	ExitSP        uint32 `json:"exit_sp"`
	Before, After fighter2AttackSnapshot
	External      struct {
		Held, Pressed, Changed uint8
	} `json:"external_controls"`
}

type fighter2AttackManifest struct {
	Pass         bool
	Records      int
	Calls        int
	Baseline     bool           `json:"frames_match_original_core"`
	SHA          string         `json:"fixture_sha256"`
	Gzip         string         `json:"gzip_sha256"`
	Constructors map[string]int `json:"constructor_counts"`
	RNGCalls     int            `json:"rng_calls"`
}

func decodeFighter2Attack(t *testing.T, raw fighter2AttackSnapshot, id uint16) ChuAttackState {
	t.Helper()
	a := attackContactBytes(t, raw.ActorBytes, 256)
	world := attackContactBytes(t, raw.WorldBytes, 4096)
	offset := int(id) - 0x8400
	if offset < 0 || offset+1024 > len(world) || a[0x58] != 2 || !bytes.Equal(world[offset:offset+256], a) ||
		hex.EncodeToString(world[offset+256:offset+1024]) != raw.ChildrenBytes ||
		hex.EncodeToString(world[offset+256:offset+512]) != raw.ChildBytes {
		t.Fatal("unbound source2 actor/children/world")
	}
	state := decodeChuAttack(t, raw.chuAttackSnapshot, id)
	if a[0x48] != raw.Aux48High || binary.BigEndian.Uint16(a[0xba:]) != raw.EnergyFraction || binary.BigEndian.Uint32(a[0x48:]) != raw.Aux48 {
		t.Fatal("unbound energy fraction or Aux48 byte view")
	}
	if !reflect.DeepEqual(encodeFighter2Attack(t, state, raw), raw) {
		t.Fatal("source2 semantic input differs from full original bytes")
	}
	return state
}

func encodeFighter2Attack(t *testing.T, state ChuAttackState, previous fighter2AttackSnapshot) fighter2AttackSnapshot {
	t.Helper()
	raw := previous
	raw.chuAttackSnapshot = encodeChuAttack(t, state, previous.chuAttackSnapshot)
	a := attackContactBytes(t, raw.ActorBytes, 256)
	binary.BigEndian.PutUint16(a[0x78:], uint16(state.Body.Health))
	binary.BigEndian.PutUint16(a[0x7e:], uint16(state.Body.Auxiliary7E))
	binary.BigEndian.PutUint16(a[0x80:], uint16(state.Body.Auxiliary80))
	a[0xc8] = state.Body.TransitionDelay
	raw.ActorBytes = hex.EncodeToString(a)
	raw.Aux48High, raw.Aux48 = a[0x48], binary.BigEndian.Uint32(a[0x48:])
	raw.EnergyFraction = binary.BigEndian.Uint16(a[0xba:])
	world := attackContactBytes(t, previous.WorldBytes, 4096)
	offset := int(state.ObjectID) - 0x8400
	copy(world[offset:offset+256], a)
	raw.WorldBytes = hex.EncodeToString(world)
	raw.ChildrenBytes = hex.EncodeToString(world[offset+256 : offset+1024])
	return raw
}

// Original complete-return fixtures and this test precede the source2 module.
// Stream the original JSON so the full worlds are not retained twice in memory.
func visitFighter2Attacks(t *testing.T, name string, visit func(*json.Decoder)) fighter2AttackManifest {
	t.Helper()
	manifestBytes, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest fighter2AttackManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	if !manifest.Pass || !manifest.Baseline || manifest.RNGCalls != 0 || hex.EncodeToString(zh[:]) != manifest.Gzip {
		t.Fatal("unverified original source2 selector fixture")
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
		t.Fatal("unbound complete original source2 fixture")
	}
	return manifest
}

func TestFighter2AttacksOriginalFullCalls(t *testing.T) {
	for _, name := range []string{"fighter2-attacks-natural", "fighter2-attacks-special-natural", "fighter2-attacks-oracle", "fighter2-attacks-boundaries"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			natural := name == "fighter2-attacks-natural" || name == "fighter2-attacks-special-natural"
			modes, sources := map[string]int{}, map[uint16]int{}
			manifest := visitFighter2Attacks(t, name, func(decoder *json.Decoder) {
				for decoder.More() {
					var row fighter2AttackCall
					if err := decoder.Decode(&row); err != nil {
						t.Fatal(err)
					}
					if row.CharacterID != 2 || row.ExitSP != row.EntrySP+4 {
						t.Fatal("not a complete source2 return")
					}
					input := decodeFighter2Attack(t, row.Before, row.ObjectID)
					copy := decodeFighter2Attack(t, row.Before, row.ObjectID)
					got, err := SelectFighter2Attack(input, chuAttackMode(t, row.Kind))
					want := decodeFighter2Attack(t, row.After, row.ObjectID)
					if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeFighter2Attack(t, got, row.Before), row.After) {
						t.Fatalf("source2 %d %s/%s differs: %v; body=%+v want=%+v energy=%d/%d gravity=%x/%x", calls, row.Kind, row.Name, err, got.Body, want.Body, got.Energy, want.Energy, got.Body.Gravity, want.Body.Gravity)
					}
					if !reflect.DeepEqual(input, copy) || row.After.RandomSeed != row.Before.RandomSeed || row.After.EnergyFraction != row.Before.EnergyFraction || row.After.Aux48 != row.Before.Aux48 {
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
			if natural && calls == 0 {
				t.Log("passive original baseline matched; no source2 natural selector call in this input")
				return
			}
			if !natural && (len(modes) != 4 || len(sources) != 4) {
				t.Fatal("missing selector or object-address coverage")
			}
			if name == "fighter2-attacks-boundaries" {
				for _, entry := range []string{"0x167c4", "0x18812", "0x18828", "0x1883e", "0x18854", "0x1886a", "0x18880", "0x188d8", "0x188ee", "0x18930", "0x18cc0", "0x18cea", "0x18d14", "0x18de0", "0x18e00", "0x18d3e", "0x18d56", "0x18d6e", "0x18d9e", "0x18db6", "0x18e60", "0x18e36"} {
					if manifest.Constructors[entry] == 0 {
						t.Fatal("missing original constructor/throw entry", entry)
					}
				}
			}
			t.Logf("complete original source2 calls=%d modes=%v objects=%v", calls, modes, sources)
		})
	}
}

func TestFighter2AttacksSingleInitialState(t *testing.T) {
	streams, steps := 0, 0
	manifest := visitFighter2Attacks(t, "fighter2-attacks-sequences", func(decoder *json.Decoder) {
		for decoder.More() {
			var stream struct {
				Kind    string
				Initial fighter2AttackSnapshot
				Calls   []fighter2AttackCall
			}
			if err := decoder.Decode(&stream); err != nil || len(stream.Calls) == 0 {
				t.Fatal("missing original stream", err)
			}
			state := decodeFighter2Attack(t, stream.Initial, stream.Calls[0].ObjectID)
			encoded := stream.Initial
			for index, call := range stream.Calls {
				state.Controls.Held = state.Controls.Held&0xff00 | Buttons(call.External.Held)
				state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(call.External.Pressed)
				state.Controls.Changed = state.Controls.Changed&0xff00 | Buttons(call.External.Changed)
				encoded = encodeFighter2Attack(t, state, encoded)
				if !reflect.DeepEqual(encoded, call.Before) {
					t.Fatal("original stream changed an undeclared intermediate state")
				}
				var err error
				state, err = SelectFighter2Attack(state, chuAttackMode(t, call.Kind))
				encoded = encodeFighter2Attack(t, state, encoded)
				if err != nil || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeFighter2Attack(t, call.After, call.ObjectID)) {
					t.Fatalf("source2 stream %s/%d step%d differs: %v", stream.Kind, streams, index, err)
				}
				steps++
			}
			streams++
		}
	})
	if streams != manifest.Records || streams != 16 || steps != manifest.Calls || steps != 320 {
		t.Fatal("missing continuous source2 evidence")
	}
}

func TestFighter2AttacksInvalidInputIsAtomic(t *testing.T) {
	var base ChuAttackState
	visitFighter2Attacks(t, "fighter2-attacks-oracle", func(decoder *json.Decoder) {
		for decoder.More() {
			var row fighter2AttackCall
			if err := decoder.Decode(&row); err != nil {
				t.Fatal(err)
			}
			if base.ObjectID == 0 {
				base = decodeFighter2Attack(t, row.Before, row.ObjectID)
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
		got, err := SelectFighter2Attack(input, mode)
		if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
			t.Fatal("invalid source2 input leaked actor/pool/throw changes")
		}
	}
	for _, mode := range []ChuAttackMode{ChuGroundAttack} {
		for _, offset := range []uint16{1, uint16(len(base.Throws.Records) * 32)} {
			input := base
			input.Body.Request = 0
			input.Controls.Held, input.Controls.Pressed = ButtonRight, ButtonA
			input.Throws.Offset = offset
			got, err := SelectFighter2Attack(input, mode)
			if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
				t.Fatal("late throw storage failure leaked changes")
			}
		}
	}
	for _, direction := range []Buttons{11, 13, 15} {
		input := base
		input.Body.Request, input.Controls.Held = 1, direction
		got, err := SelectFighter2Attack(input, ChuGroundSpecial)
		if err == nil || !reflect.DeepEqual(got, input) {
			t.Fatal("nonclosed/budget-limited diagnostic direction was not atomically rejected")
		}
	}
	input := base
	input.Controls.Held, input.Controls.Pressed = ButtonRight, ButtonA
	input.Throws.Offset, input.Throws.Records, input.Pool.Cursor = 1, nil, 1
	got, err := SelectFighter2Attack(input, ChuAirAttack)
	if err != nil || got.Body.ActionKind != 2 || got.Body.Pose != 0x95 || !reflect.DeepEqual(got.Throws, input.Throws) || got.Pool != input.Pool {
		t.Fatal("ordinary air read unused throw/pool storage")
	}
}

// These are actual CPU diagnostics, not normal table entries or natural play.
// Aliases12/14 have closed selector outputs.11/13 change unrelated fields/RNG;
// 15 did not return in the declared budget. Go rejection is not an original noop.
func TestFighter2AttacksDirectionDiagnostics(t *testing.T) {
	manifestBytes, err := os.ReadFile("testdata/fighter2-attacks-direction-diagnostics-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Records int
		SHA     string `json:"fixture_sha256"`
		Gzip    string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	zipped, err := os.ReadFile("testdata/fighter2-attacks-direction-diagnostics.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	if hex.EncodeToString(zh[:]) != manifest.Gzip || manifest.Records != 15 {
		t.Fatal("unbound CPU direction diagnostics")
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	hash := sha256.New()
	reader := io.TeeReader(z, hash)
	decoder := json.NewDecoder(reader)
	if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
		t.Fatal("invalid diagnostic array", err)
	}
	count, aliases, rejected, budget := 0, 0, 0, 0
	for decoder.More() {
		var row struct {
			Profile        string
			Direction      uint8
			Status         string
			ObjectID       uint16   `json:"object_id"`
			RegistersAfter []uint32 `json:"registers_after"`
			Before, After  fighter2AttackSnapshot
			RAMBefore      string `json:"ram_before"`
			RAMAfter       string `json:"ram_after"`
		}
		if err := decoder.Decode(&row); err != nil {
			t.Fatal(err)
		}
		beforeRAM := attackContactBytes(t, row.RAMBefore, 65536)
		afterRAM := attackContactBytes(t, row.RAMAfter, 65536)
		if !bytes.Equal(beforeRAM[int(row.ObjectID):int(row.ObjectID)+256], attackContactBytes(t, row.Before.ActorBytes, 256)) || !bytes.Equal(afterRAM[int(row.ObjectID):int(row.ObjectID)+256], attackContactBytes(t, row.After.ActorBytes, 256)) {
			t.Fatal("diagnostic semantic snapshots differ from full RAM")
		}
		input := decodeFighter2Attack(t, row.Before, row.ObjectID)
		got, err := SelectFighter2Attack(input, ChuGroundSpecial)
		switch row.Direction {
		case 12, 14:
			if row.Status != "returned" || len(row.RegistersAfter) != 21 || row.RegistersAfter[15] != 0xffff04 || row.RegistersAfter[16] != 0x3d8 {
				t.Fatal("alias did not reach the actual selector return")
			}
			if err != nil || !reflect.DeepEqual(got, decodeFighter2Attack(t, row.After, row.ObjectID)) || !reflect.DeepEqual(encodeFighter2Attack(t, got, row.Before), row.After) {
				t.Fatal("CPU-verified nonstructural alias differs", row.Profile, row.Direction, err)
			}
			aliases++
		case 11, 13, 15:
			if err == nil || !reflect.DeepEqual(got, input) {
				t.Fatal("unrepresentable diagnostic context leaked a Go fallback")
			}
			if row.Direction == 15 {
				if row.Status != "not-returned-within-cycle-budget" {
					t.Fatal("missing budget-ended real CPU outcome")
				}
				budget++
			} else if row.Status != "returned" || row.Before.RandomSeed == row.After.RandomSeed {
				t.Fatal("diagnostic side effects were hidden as a noop")
			}
			rejected++
		default:
			t.Fatal("unexpected diagnostic index")
		}
		count++
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
		t.Fatal("unterminated diagnostics", err)
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != manifest.SHA || count != 15 || aliases != 6 || rejected != 9 || budget != 3 {
		t.Fatal("incomplete or unbound actual CPU diagnostics")
	}
}
