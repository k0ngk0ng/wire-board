package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
)

type fighter5AttackSnapshot struct {
	chuAttackSnapshot
	WorldBytes        string `json:"world_bytes"`
	ChildrenBytes     string `json:"children_bytes"`
	EnergyFraction    uint16 `json:"energy_fraction"`
	RandomSeed        uint32 `json:"random_seed"`
	AlternateVelocity int32  `json:"alternate_velocity"`
}

type fighter5AttackDelta struct {
	Offset        int
	Before, After uint8
}

type fighter5AttackIRQ struct {
	Entry, Exit string
	Changes     []fighter5AttackDelta
}

type fighter5AttackCall struct {
	Kind, Name, Context string
	ObjectID            uint16 `json:"object_id"`
	CharacterID         uint8  `json:"source_character_id"`
	SourceRegister      string `json:"source_register"`
	EntrySP             uint32 `json:"entry_sp"`
	ExitSP              uint32 `json:"exit_sp"`
	Before, After       fighter5AttackSnapshot
	RAMBeforeSHA        string                `json:"ram_before_sha256"`
	RAMAfterSHA         string                `json:"ram_after_sha256"`
	RAMChanges          []fighter5AttackDelta `json:"ram_changes"`
	StackChanges        []fighter5AttackDelta `json:"stack_changes"`
	ExternalChanges     []fighter5AttackDelta `json:"external_changes"`
	UnexplainedChanges  []fighter5AttackDelta `json:"unexplained_changes"`
	Interrupts          []fighter5AttackIRQ
	External            struct {
		Held, Pressed, Changed uint8
	} `json:"external_controls"`
}

type fighter5AttackManifest struct {
	Pass         bool
	Records      int
	Calls        int
	Baseline     bool           `json:"frames_match_original_core"`
	SHA          string         `json:"fixture_sha256"`
	Gzip         string         `json:"gzip_sha256"`
	Constructors map[string]int `json:"constructor_counts"`
	Successful   map[string]int `json:"successful_constructor_counts"`
	RNGCalls     int            `json:"rng_calls"`
	AuditBytes   int            `json:"full_ram_audit_bytes"`
	VideoSteps   int            `json:"video_steps"`
	Points       int            `json:"observer_point_count"`
	Interrupts   int
}

func decodeFighter5Attack(t *testing.T, raw fighter5AttackSnapshot, id uint16) Fighter5AttackState {
	t.Helper()
	a := attackContactBytes(t, raw.ActorBytes, 256)
	world := attackContactBytes(t, raw.WorldBytes, 4096)
	offset := int(id) - 0x8400
	if offset < 0 || offset+1024 > len(world) || a[0x58] != 5 || !bytes.Equal(world[offset:offset+256], a) ||
		hex.EncodeToString(world[offset+256:offset+1024]) != raw.ChildrenBytes ||
		hex.EncodeToString(world[offset+256:offset+512]) != raw.ChildBytes {
		t.Fatal("unbound source5 actor/children/world")
	}
	state := Fighter5AttackState{ChuAttackState: decodeChuAttack(t, raw.chuAttackSnapshot, id), AlternateVelocity: raw.AlternateVelocity}
	if binary.BigEndian.Uint16(a[0xba:]) != raw.EnergyFraction || int32(binary.BigEndian.Uint32(a[0x48:])) != raw.AlternateVelocity {
		t.Fatal("unbound energy fraction or alternate velocity")
	}
	if !reflect.DeepEqual(encodeFighter5Attack(t, state, raw), raw) {
		t.Fatal("source5 semantic input differs from full original bytes")
	}
	return state
}

func encodeFighter5Attack(t *testing.T, state Fighter5AttackState, previous fighter5AttackSnapshot) fighter5AttackSnapshot {
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
	raw.AlternateVelocity = int32(binary.BigEndian.Uint32(a[0x48:]))
	raw.EnergyFraction = binary.BigEndian.Uint16(a[0xba:])
	world := attackContactBytes(t, previous.WorldBytes, 4096)
	offset := int(state.ObjectID) - 0x8400
	copy(world[offset:offset+256], a)
	raw.WorldBytes = hex.EncodeToString(world)
	raw.ChildrenBytes = hex.EncodeToString(world[offset+256 : offset+1024])
	return raw
}

func applyFighter5Attack(t *testing.T, state Fighter5AttackState, kind string) (Fighter5AttackState, error) {
	t.Helper()
	switch kind {
	case "combination-high":
		return FinishFighter5Combination(state, false), nil
	case "combination-low":
		return FinishFighter5Combination(state, true), nil
	case "final-0", "final-1", "final-2":
		return BeginFighter5FinalProjectile(state, kind[len(kind)-1]-'0')
	default:
		return SelectFighter5Attack(state, chuAttackMode(t, kind))
	}
}

// Every original CPU byte delta is retained. Semantic regions must account for
// all projected deltas; external changes require the exact recorded IRQ chain.
func auditFighter5Attack(t *testing.T, row fighter5AttackCall) {
	t.Helper()
	if row.CharacterID != 5 || row.ExitSP != row.EntrySP+4 || row.SourceRegister != fmt.Sprintf("0x%x", uint32(0xffff0000)|uint32(row.ObjectID)) ||
		len(row.RAMBeforeSHA) != 64 || len(row.RAMAfterSHA) != 64 || len(row.UnexplainedChanges) != 0 {
		t.Fatal("incomplete source5 return or full-RAM audit")
	}
	projected := make([]fighter5AttackDelta, 0)
	add := func(base int, before, after []byte) {
		for i, value := range before {
			if value != after[i] {
				projected = append(projected, fighter5AttackDelta{base + i, value, after[i]})
			}
		}
	}
	add(0x8400, attackContactBytes(t, row.Before.WorldBytes, 4096), attackContactBytes(t, row.After.WorldBytes, 4096))
	add(0x9400, attackContactBytes(t, row.Before.Pool.Bytes, 2048), attackContactBytes(t, row.After.Pool.Bytes, 2048))
	for _, cursor := range []struct {
		offset        int
		before, after uint16
	}{{0xa00c, row.Before.ThrowOffset, row.After.ThrowOffset}, {0xa036, row.Before.Pool.Cursor, row.After.Pool.Cursor}} {
		add(cursor.offset, []byte{uint8(cursor.before >> 8), uint8(cursor.before)}, []byte{uint8(cursor.after >> 8), uint8(cursor.after)})
	}
	add(0xa600, attackContactBytes(t, row.Before.Throws, 512), attackContactBytes(t, row.After.Throws, 512))
	seedBefore, seedAfter := make([]byte, 4), make([]byte, 4)
	binary.BigEndian.PutUint32(seedBefore, row.Before.RandomSeed)
	binary.BigEndian.PutUint32(seedAfter, row.After.RandomSeed)
	add(0xff18, seedBefore, seedAfter)
	actorWrites := map[int]bool{0x48: true, 0x49: true, 0x4c: true, 0x59: true, 0x5a: true, 0x5b: true, 0x5d: true, 0x5f: true, 0x60: true, 0x61: true, 0x64: true, 0x6b: true, 0x6c: true, 0x6d: true, 0x6e: true, 0x77: true, 0xb8: true, 0xb9: true}
	for i := 0x10; i < 0x18; i++ {
		actorWrites[i] = true
	}
	observed, stacks, external := make([]fighter5AttackDelta, 0), make([]fighter5AttackDelta, 0), make([]fighter5AttackDelta, 0)
	previous := -1
	for _, delta := range row.RAMChanges {
		if delta.Offset <= previous || delta.Offset >= 65536 || delta.Before == delta.After {
			t.Fatal("invalid complete RAM delta")
		}
		previous = delta.Offset
		stack := (row.EntrySP-uint32(delta.Offset))&65535 > 0 && (row.EntrySP-uint32(delta.Offset))&65535 < 256
		pool := delta.Offset >= 0x9400 && delta.Offset < 0x9c00
		throws := delta.Offset >= 0xa600 && delta.Offset < 0xa800
		cursor := delta.Offset == 0xa00c || delta.Offset == 0xa00d || delta.Offset == 0xa036 || delta.Offset == 0xa037
		if delta.Offset >= 0x8400 && delta.Offset < 0x9400 || pool || throws || cursor || delta.Offset >= 0xff18 && delta.Offset < 0xff1c {
			observed = append(observed, delta)
		}
		if stack {
			stacks = append(stacks, delta)
		}
		if !actorWrites[delta.Offset-int(row.ObjectID)] && !pool && !throws && !cursor && !stack {
			external = append(external, delta)
		}
	}
	if !reflect.DeepEqual(projected, observed) || !reflect.DeepEqual(stacks, row.StackChanges) || !reflect.DeepEqual(external, row.ExternalChanges) {
		t.Fatal("full-RAM deltas do not match retained semantic/stack/external bytes")
	}
	for _, delta := range external {
		current, steps := delta.Before, 0
		for _, irq := range row.Interrupts {
			if irq.Entry != "0xfff000" || irq.Exit != "0xfff09a" && irq.Exit != "0xfff0a8" && irq.Exit != "0xfff13a" {
				t.Fatal("unknown original interrupt path")
			}
			for _, change := range irq.Changes {
				if change.Offset == delta.Offset {
					if change.Before != current {
						t.Fatal("broken original IRQ byte chain")
					}
					current, steps = change.After, steps+1
				}
			}
		}
		if steps == 0 || current != delta.After {
			t.Fatal("unexplained original external write")
		}
	}
	if row.Context == "declared-private" && (len(external) != 0 || len(row.Interrupts) != 0) {
		t.Fatal("private source5 call changed external state")
	}
}

// Original complete-return fixtures and this test precede the source5 module.
func visitFighter5Attacks(t *testing.T, name string, visit func(*json.Decoder)) fighter5AttackManifest {
	t.Helper()
	manifestBytes, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest fighter5AttackManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	if !manifest.Pass || !manifest.Baseline || manifest.RNGCalls != 0 || manifest.AuditBytes != 65536 || manifest.VideoSteps != 3602 || manifest.Points > 64 || manifest.Points == 0 || hex.EncodeToString(zh[:]) != manifest.Gzip {
		t.Fatal("unverified original source5 selector fixture")
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	hash := sha256.New()
	reader := io.TeeReader(z, hash)
	decoder := json.NewDecoder(reader)
	for _, want := range []any{json.Delim('{'), "calls", json.Delim('[')} {
		if token, err := decoder.Token(); err != nil || token != want {
			t.Fatal("invalid original fixture envelope", err)
		}
	}
	visit(decoder)
	for _, want := range []any{json.Delim(']'), json.Delim('}')} {
		if token, err := decoder.Token(); err != nil || token != want {
			t.Fatal("unterminated original fixture", err)
		}
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != manifest.SHA {
		t.Fatal("unbound complete original source5 fixture")
	}
	return manifest
}

func TestFighter5AttacksOriginalFullCalls(t *testing.T) {
	for _, name := range []string{"fighter5-attacks-natural", "fighter5-attacks-special-natural", "fighter5-attacks-oracle", "fighter5-attacks-boundaries"} {
		t.Run(name, func(t *testing.T) {
			calls, interrupts := 0, 0
			modes, sources := map[string]int{}, map[uint16]int{}
			manifest := visitFighter5Attacks(t, name, func(decoder *json.Decoder) {
				for decoder.More() {
					var row fighter5AttackCall
					if err := decoder.Decode(&row); err != nil {
						t.Fatal(err)
					}
					auditFighter5Attack(t, row)
					input := decodeFighter5Attack(t, row.Before, row.ObjectID)
					copy := decodeFighter5Attack(t, row.Before, row.ObjectID)
					got, err := applyFighter5Attack(t, input, row.Kind)
					want := decodeFighter5Attack(t, row.After, row.ObjectID)
					if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeFighter5Attack(t, got, row.Before), row.After) {
						t.Fatalf("source5 %d %s/%s differs: %v; body=%+v want=%+v energy=%d/%d alt=%x/%x gravity=%x/%x", calls, row.Kind, row.Name, err, got.Body, want.Body, got.Energy, want.Energy, got.AlternateVelocity, want.AlternateVelocity, got.Body.Gravity, want.Body.Gravity)
					}
					if !reflect.DeepEqual(input, copy) || row.After.RandomSeed != row.Before.RandomSeed || row.After.EnergyFraction != row.Before.EnergyFraction {
						t.Fatal("selector changed caller storage, RNG or energy fraction")
					}
					calls++
					interrupts += len(row.Interrupts)
					modes[row.Kind]++
					sources[row.ObjectID]++
				}
			})
			if calls == 0 || calls != manifest.Records || calls != manifest.Calls || interrupts != manifest.Interrupts {
				t.Fatal("missing complete original source5 selector calls or IRQ audit")
			}
			if name == "fighter5-attacks-natural" && (calls != 1964 || len(modes) != 4) || name == "fighter5-attacks-oracle" && (calls != 34 || len(modes) != 9 || len(sources) != 4) || name == "fighter5-attacks-boundaries" && (calls != 33280 || len(modes) != 9 || len(sources) != 4) {
				t.Fatal("missing selector, followup or object-address coverage")
			}
			if name == "fighter5-attacks-boundaries" || name == "fighter5-attacks-special-natural" {
				for _, entry := range []string{"0x19216", "0x19240", "0x19284", "0x192c8", "0x192f6", "0x19324", "0x19364", "0x193a4", "0x193bc", "0x193d4"} {
					if manifest.Successful[entry] == 0 {
						t.Fatal("missing successful original special root", entry)
					}
				}
			}
			if name == "fighter5-attacks-boundaries" {
				for _, entry := range []string{"0x1710c", "0x18812", "0x18828", "0x1883e", "0x18854", "0x1886a", "0x18880", "0x188d8", "0x188ee", "0x18930", "0x1926a", "0x192ae", "0x19408", "0x19426", "0x19444"} {
					if manifest.Constructors[entry] == 0 {
						t.Fatal("missing original ordinary/followup entry", entry)
					}
				}
			}
			t.Logf("complete original source5 calls=%d modes=%v objects=%v IRQs=%d", calls, modes, sources, interrupts)
		})
	}
}

func TestFighter5AttacksSingleInitialState(t *testing.T) {
	streams, steps := 0, 0
	manifest := visitFighter5Attacks(t, "fighter5-attacks-sequences", func(decoder *json.Decoder) {
		for decoder.More() {
			var stream struct {
				Kind    string
				Initial fighter5AttackSnapshot
				Calls   []fighter5AttackCall
			}
			if err := decoder.Decode(&stream); err != nil || len(stream.Calls) == 0 {
				t.Fatal("missing original stream", err)
			}
			state := decodeFighter5Attack(t, stream.Initial, stream.Calls[0].ObjectID)
			encoded := stream.Initial
			for index, call := range stream.Calls {
				auditFighter5Attack(t, call)
				state.Controls.Held = state.Controls.Held&0xff00 | Buttons(call.External.Held)
				state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(call.External.Pressed)
				state.Controls.Changed = state.Controls.Changed&0xff00 | Buttons(call.External.Changed)
				encoded = encodeFighter5Attack(t, state, encoded)
				if !reflect.DeepEqual(encoded, call.Before) {
					t.Fatal("original stream changed an undeclared intermediate state")
				}
				var err error
				state, err = applyFighter5Attack(t, state, call.Kind)
				encoded = encodeFighter5Attack(t, state, encoded)
				if err != nil || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeFighter5Attack(t, call.After, call.ObjectID)) {
					t.Fatalf("source5 stream %s/%d step%d differs: %v", stream.Kind, streams, index, err)
				}
				steps++
			}
			streams++
		}
	})
	if streams != manifest.Records || streams != 108 || steps != manifest.Calls || steps != 1728 {
		t.Fatal("missing continuous source5 evidence")
	}
}

func TestFighter5AttacksInvalidInputIsAtomic(t *testing.T) {
	var base Fighter5AttackState
	visitFighter5Attacks(t, "fighter5-attacks-oracle", func(decoder *json.Decoder) {
		for decoder.More() {
			var row fighter5AttackCall
			if err := decoder.Decode(&row); err != nil {
				t.Fatal(err)
			}
			if base.ObjectID == 0 {
				base = decodeFighter5Attack(t, row.Before, row.ObjectID)
			}
		}
	})
	for _, mode := range []ChuAttackMode{255, ChuGroundAttack} {
		input := base
		if mode == ChuGroundAttack {
			input.Body.Request, input.Pool.Cursor = 8, 1
		}
		got, err := SelectFighter5Attack(input, mode)
		if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
			t.Fatal("invalid source5 input leaked actor/pool/throw changes")
		}
	}
	for _, offset := range []uint16{1, uint16(len(base.Throws.Records) * 32)} {
		input := base
		input.Body.Request = 0
		input.Controls.Held, input.Controls.Pressed = ButtonRight, ButtonA
		input.Throws.Offset = offset
		got, err := SelectFighter5Attack(input, ChuGroundAttack)
		if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
			t.Fatal("late source5 throw storage failure leaked changes")
		}
	}
	for _, variant := range []uint8{3, 4, 128, 255} {
		got, err := BeginFighter5FinalProjectile(base, variant)
		if err == nil || !reflect.DeepEqual(got, base) {
			t.Fatal("invalid source5 final variant changed state")
		}
	}
	input := base
	input.Controls.Held, input.Controls.Pressed = ButtonRight, ButtonA
	input.Throws.Offset, input.Throws.Records, input.Pool.Cursor = 1, nil, 1
	got, err := SelectFighter5Attack(input, ChuAirAttack)
	if err != nil || got.Body.ActionKind != 2 || got.Body.Pose != 0x95 || !reflect.DeepEqual(got.Throws, input.Throws) || got.Pool != input.Pool || got.AlternateVelocity != input.AlternateVelocity {
		t.Fatal("source5 air selector read unused storage or failed ordinary attack")
	}
}
