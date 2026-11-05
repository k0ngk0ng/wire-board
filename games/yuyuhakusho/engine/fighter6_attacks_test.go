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

type fighter6AttackSnapshot struct {
	chuAttackSnapshot
	WorldBytes        string `json:"world_bytes"`
	ChildrenBytes     string `json:"children_bytes"`
	EnergyFraction    uint16 `json:"energy_fraction"`
	RandomSeed        uint32 `json:"random_seed"`
	AlternateVelocity int32  `json:"alternate_velocity"`
}

type fighter6AttackCall struct {
	Kind, Name    string
	ObjectID      uint16 `json:"object_id"`
	CharacterID   uint8  `json:"source_character_id"`
	EntrySP       uint32 `json:"entry_sp"`
	ExitSP        uint32 `json:"exit_sp"`
	Before, After fighter6AttackSnapshot
	External      struct {
		Held, Pressed, Changed uint8
	} `json:"external_controls"`
}

type fighter6AttackManifest struct {
	Pass         bool
	Records      int
	Calls        int
	Baseline     bool           `json:"frames_match_original_core"`
	SHA          string         `json:"fixture_sha256"`
	Gzip         string         `json:"gzip_sha256"`
	Constructors map[string]int `json:"constructor_counts"`
	RNGCalls     int            `json:"rng_calls"`
}

func decodeFighter6Attack(t *testing.T, raw fighter6AttackSnapshot, id uint16) Fighter6AttackState {
	t.Helper()
	a := attackContactBytes(t, raw.ActorBytes, 256)
	world := attackContactBytes(t, raw.WorldBytes, 4096)
	offset := int(id) - 0x8400
	if offset < 0 || offset+1024 > len(world) || a[0x58] != 6 || !bytes.Equal(world[offset:offset+256], a) ||
		hex.EncodeToString(world[offset+256:offset+1024]) != raw.ChildrenBytes ||
		hex.EncodeToString(world[offset+256:offset+512]) != raw.ChildBytes {
		t.Fatal("unbound source6 actor/children/world")
	}
	state := Fighter6AttackState{ChuAttackState: decodeChuAttack(t, raw.chuAttackSnapshot, id),
		AlternateVelocity: int32(binary.BigEndian.Uint32(a[0x48:]))}
	if state.AlternateVelocity != raw.AlternateVelocity || binary.BigEndian.Uint16(a[0xba:]) != raw.EnergyFraction {
		t.Fatal("unbound Aux48 long or energy fraction")
	}
	if !reflect.DeepEqual(encodeFighter6Attack(t, state, raw), raw) {
		t.Fatal("source6 semantic input differs from full original bytes")
	}
	return state
}

func encodeFighter6Attack(t *testing.T, state Fighter6AttackState, previous fighter6AttackSnapshot) fighter6AttackSnapshot {
	t.Helper()
	raw := previous
	raw.chuAttackSnapshot = encodeChuAttack(t, state.ChuAttackState, previous.chuAttackSnapshot)
	a := attackContactBytes(t, raw.ActorBytes, 256)
	binary.BigEndian.PutUint32(a[0x48:], uint32(state.AlternateVelocity))
	binary.BigEndian.PutUint16(a[0x78:], uint16(state.Body.Health))
	binary.BigEndian.PutUint16(a[0x7e:], uint16(state.Body.Auxiliary7E))
	binary.BigEndian.PutUint16(a[0x80:], uint16(state.Body.Auxiliary80))
	a[0xc8] = state.Body.TransitionDelay
	raw.ActorBytes = hex.EncodeToString(a)
	raw.AlternateVelocity = state.AlternateVelocity
	world := attackContactBytes(t, previous.WorldBytes, 4096)
	offset := int(state.ObjectID) - 0x8400
	copy(world[offset:offset+256], a)
	raw.WorldBytes = hex.EncodeToString(world)
	raw.ChildrenBytes = hex.EncodeToString(world[offset+256 : offset+1024])
	return raw
}

// Original complete-return fixtures and this test precede the source6 module.
// Stream the original JSON so the full worlds are not retained twice in memory.
func visitFighter6Attacks(t *testing.T, name string, visit func(*json.Decoder)) fighter6AttackManifest {
	t.Helper()
	manifestBytes, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest fighter6AttackManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	if !manifest.Pass || !manifest.Baseline || manifest.RNGCalls != 0 || hex.EncodeToString(zh[:]) != manifest.Gzip {
		t.Fatal("unverified original source6 selector fixture")
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
		t.Fatal("unbound complete original source6 fixture")
	}
	return manifest
}

func TestFighter6AttacksOriginalFullCalls(t *testing.T) {
	for _, name := range []string{"fighter6-attacks-natural", "fighter6-attacks-oracle", "fighter6-attacks-boundaries", "fighter6-attacks-special-natural"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			modes, sources := map[string]int{}, map[uint16]int{}
			manifest := visitFighter6Attacks(t, name, func(decoder *json.Decoder) {
				for decoder.More() {
					var row fighter6AttackCall
					if err := decoder.Decode(&row); err != nil {
						t.Fatal(err)
					}
					if row.CharacterID != 6 || row.ExitSP != row.EntrySP+4 {
						t.Fatal("not a complete source6 return")
					}
					input := decodeFighter6Attack(t, row.Before, row.ObjectID)
					copy := decodeFighter6Attack(t, row.Before, row.ObjectID)
					got, err := SelectFighter6Attack(input, chuAttackMode(t, row.Kind))
					want := decodeFighter6Attack(t, row.After, row.ObjectID)
					if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeFighter6Attack(t, got, row.Before), row.After) {
						t.Fatalf("source6 %d %s/%s differs: %v; body=%+v want=%+v energy=%d/%d aux48=%x/%x", calls, row.Kind, row.Name, err, got.Body, want.Body, got.Energy, want.Energy, got.AlternateVelocity, want.AlternateVelocity)
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
				t.Fatal("missing source6 selector calls")
			}
			if (name == "fighter6-attacks-oracle" || name == "fighter6-attacks-boundaries") && (len(modes) != 4 || len(sources) != 4) {
				t.Fatal("missing selector or object-address coverage")
			}
			if name == "fighter6-attacks-boundaries" {
				for _, entry := range []string{"0x1743c", "0x175e0", "0x174fe", "0x17534", "0x194dc", "0x194fa", "0x18812", "0x18828", "0x1883e", "0x1886a", "0x188d8", "0x188ee", "0x18930", "0x19474", "0x194aa", "0x19518", "0x19542", "0x1956c", "0x1959a", "0x195b2"} {
					if manifest.Constructors[entry] == 0 {
						t.Fatal("missing original constructor/throw entry", entry)
					}
				}
			}
			t.Logf("complete original source6 calls=%d modes=%v objects=%v", calls, modes, sources)
		})
	}
}

func TestFighter6AttacksSingleInitialState(t *testing.T) {
	streams, steps := 0, 0
	manifest := visitFighter6Attacks(t, "fighter6-attacks-sequences", func(decoder *json.Decoder) {
		for decoder.More() {
			var stream struct {
				Kind    string
				Initial fighter6AttackSnapshot
				Calls   []fighter6AttackCall
			}
			if err := decoder.Decode(&stream); err != nil || len(stream.Calls) == 0 {
				t.Fatal("missing original stream", err)
			}
			state := decodeFighter6Attack(t, stream.Initial, stream.Calls[0].ObjectID)
			encoded := stream.Initial
			for index, call := range stream.Calls {
				state.Controls.Held = state.Controls.Held&0xff00 | Buttons(call.External.Held)
				state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(call.External.Pressed)
				state.Controls.Changed = state.Controls.Changed&0xff00 | Buttons(call.External.Changed)
				encoded = encodeFighter6Attack(t, state, encoded)
				if !reflect.DeepEqual(encoded, call.Before) {
					t.Fatal("original stream changed an undeclared intermediate state")
				}
				var err error
				state, err = SelectFighter6Attack(state, chuAttackMode(t, call.Kind))
				encoded = encodeFighter6Attack(t, state, encoded)
				if err != nil || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeFighter6Attack(t, call.After, call.ObjectID)) {
					t.Fatalf("source6 stream %s/%d step%d differs: %v", stream.Kind, streams, index, err)
				}
				steps++
			}
			streams++
		}
	})
	if streams != manifest.Records || streams != 64 || steps != manifest.Calls || steps != 1024 {
		t.Fatal("missing continuous source6 evidence")
	}
}

func TestFighter6AttacksInvalidInputIsAtomic(t *testing.T) {
	var base, noThrowDash, noThrowDashWant Fighter6AttackState
	visitFighter6Attacks(t, "fighter6-attacks-oracle", func(decoder *json.Decoder) {
		for decoder.More() {
			var row fighter6AttackCall
			if err := decoder.Decode(&row); err != nil {
				t.Fatal(err)
			}
			if row.Kind == "air" && row.Before.Held == 0 && row.Before.Pressed == 0 && row.Before.Body.Request == 8 && row.Before.Energy == 8 {
				noThrowDash = decodeFighter6Attack(t, row.Before, row.ObjectID)
				noThrowDashWant = decodeFighter6Attack(t, row.After, row.ObjectID)
			}
			if base.ObjectID == 0 {
				base = decodeFighter6Attack(t, row.Before, row.ObjectID)
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
		got, err := SelectFighter6Attack(input, mode)
		if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
			t.Fatal("invalid source6 input leaked actor/pool/throw changes")
		}
	}
	for _, mode := range []ChuAttackMode{ChuGroundAttack, ChuAirAttack} {
		for _, offset := range []uint16{1, uint16(len(base.Throws.Records) * 32)} {
			input := base
			input.Body.Request = 0
			input.Controls.Held, input.Controls.Pressed = ButtonRight|ButtonC, ButtonA
			if mode == ChuAirAttack {
				input.Controls.Pressed |= ButtonB
			}
			input.Throws.Offset = offset
			got, err := SelectFighter6Attack(input, mode)
			if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
				t.Fatal("late throw storage failure leaked changes")
			}
		}
	}
	if noThrowDash.ObjectID == 0 {
		t.Fatal("missing original air dash without throw")
	}
	noThrowDash.Pool.Cursor, noThrowDash.Throws.Offset, noThrowDash.Throws.Records = 1, 1, nil
	noThrowDashWant.Pool, noThrowDashWant.Throws = noThrowDash.Pool, noThrowDash.Throws
	got, err := SelectFighter6Attack(noThrowDash, ChuAirAttack)
	if err != nil || !reflect.DeepEqual(got, noThrowDashWant) {
		t.Fatal("air dash inspected unobserved effect/throw storage", err)
	}

}

// Reuse existing complete original preparation streams. Identical DD programs
// finish in slot order: later slot5 writes Request6 over slot3's Request4.
// This does not claim a human DD naturally selects the air Request4 branch.
func TestFighter6AttacksCommandOrder(t *testing.T) {
	fixture, _ := readCommandFixture(t, "character-commands-sequences")
	var streams []struct {
		CharacterID uint8 `json:"character_id"`
		Initial     CommandState
		Calls       []characterCommandOriginalCall
	}
	if err := json.Unmarshal(fixture.Calls, &streams); err != nil {
		t.Fatal(err)
	}
	seen, steps, completed := 0, 0, 0
	for _, stream := range streams {
		if stream.CharacterID != 6 {
			continue
		}
		state := stream.Initial
		actor, err := hex.DecodeString(stream.Calls[0].ActorBefore)
		if err != nil {
			t.Fatal(err)
		}
		for _, call := range stream.Calls {
			state.RawHeld = call.RawInput
			state, err = PrepareCharacterCommands(6, state, fixture.Library)
			actor = encodeCommandState(state, actor)
			want, decodeErr := hex.DecodeString(call.ActorAfter)
			if err != nil || decodeErr != nil || state != call.After || !bytes.Equal(actor, want) {
				t.Fatal("source6 original command stream changed", err, decodeErr)
			}
			if state.Request == 6 {
				completed++
			}
			steps++
		}
		seen++
	}
	if seen != 3 || steps != 342 || completed == 0 {
		t.Fatal("missing original source6 command order evidence")
	}
	for _, facing := range []uint8{0, 1, 128} {
		state := CommandState{Facing: facing}
		for _, held := range []uint8{2, 0, 2, 0} {
			state.RawHeld = held
			var err error
			state, err = PrepareCharacterCommands(6, state, fixture.Library)
			if err != nil {
				t.Fatal(err)
			}
		}
		if state.Request != 6 || state.Indices[3] != 0 || state.Indices[5] != 0 {
			t.Fatal("DD did not preserve later-slot override", state)
		}
		input := Fighter6AttackState{}
		input.Body.Request, input.Body.Health = state.Request, 111
		input.Controls.Changed, input.Energy = ButtonB|ButtonA, 255
		got, err := SelectFighter6Attack(input, ChuAirSpecial)
		if err != nil || !reflect.DeepEqual(got, input) {
			t.Fatal("air selector invented Request4 from DD Request6", err)
		}
	}
	t.Logf("source6 existing original command streams=%d steps=%d Request6 states=%d; DD completes as Request6", seen, steps, completed)
}
