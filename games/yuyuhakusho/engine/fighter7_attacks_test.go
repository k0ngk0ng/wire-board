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

type fighter7AttackSnapshot struct {
	chuAttackSnapshot
	WorldBytes     string `json:"world_bytes"`
	ChildrenBytes  string `json:"children_bytes"`
	EnergyFraction uint16 `json:"energy_fraction"`
	RandomSeed     uint32 `json:"random_seed"`
	Aux48          uint32 `json:"aux48"`
	Aux48High      uint8  `json:"aux48_high"`
}

type fighter7AttackCall struct {
	Kind, Name     string
	ObjectID       uint16 `json:"object_id"`
	CharacterID    uint8  `json:"source_character_id"`
	EntrySP        uint32 `json:"entry_sp"`
	ExitSP         uint32 `json:"exit_sp"`
	Before, After  fighter7AttackSnapshot
	EntrySR        uint16                  `json:"entry_sr"`
	MinSP          uint32                  `json:"min_sp"`
	Audit          fighter7RAMAudit        `json:"ram_audit"`
	ExternalEvents []fighter7ExternalEvent `json:"external_events"`
	External       struct {
		Held, Pressed, Changed uint8
	} `json:"external_controls"`
}

type fighter7AttackManifest struct {
	Pass           bool
	Records        int
	Calls          int
	Baseline       bool           `json:"frames_match_original_core"`
	PrivateContext bool           `json:"private_context_verified"`
	SHA            string         `json:"fixture_sha256"`
	Gzip           string         `json:"gzip_sha256"`
	Constructors   map[string]int `json:"constructor_counts"`
	RNGCalls       int            `json:"rng_calls"`
	FullRAM        bool           `json:"full_cpu_ram_audited"`
	CPUBytes       int            `json:"cpu_ram_bytes"`
	UnknownWrites  int            `json:"unexplained_ram_writes"`
}

type fighter7RAMAudit struct {
	CPUBytes         int    `json:"cpu_bytes"`
	BeforeSHA        string `json:"before_sha256"`
	AfterSHA         string `json:"after_sha256"`
	Changes          [][4]json.RawMessage
	Unexplained      []json.RawMessage
	ExternalVerified bool `json:"external_sequence_verified"`
	Masked           bool `json:"interrupts_masked"`
}

type fighter7ExternalEvent struct {
	Kind          string
	EntryPC       string `json:"entry_pc"`
	ExitPC        string `json:"exit_pc"`
	EntryRawPC    string `json:"entry_raw_pc"`
	ExitRawPC     string `json:"exit_raw_pc"`
	Changes       [][3]json.RawMessage
	PC            string
	Address       uint16
	Before, After uint8
}

func fighter7AuditByte(t *testing.T, raw json.RawMessage) uint8 {
	t.Helper()
	var value uint8
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func fighter7AuditOffset(t *testing.T, raw json.RawMessage) uint16 {
	t.Helper()
	var value uint16
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

// Byte-complete functional fixtures retain unrelated data, and the separately
// recorded CPU delta must agree with every captured range. External bytes must
// follow the actual ordered IRQ spans, including repeated 0->2->4 changes.
func verifyFighter7RAM(t *testing.T, row fighter7AttackCall) {
	t.Helper()
	audit := row.Audit
	if audit.CPUBytes != 65536 || !audit.ExternalVerified || len(audit.Unexplained) != 0 || audit.Masked != (row.EntrySR&0x700 == 0x700) {
		t.Fatal("missing complete source7 CPU RAM audit")
	}
	for _, value := range []string{audit.BeforeSHA, audit.AfterSHA} {
		if bytes, err := hex.DecodeString(value); err != nil || len(bytes) != 32 {
			t.Fatal("invalid CPU RAM binding")
		}
	}
	if audit.Masked && len(row.ExternalEvents) != 0 {
		t.Fatal("masked private selector contains external execution")
	}
	type region struct {
		offset        int
		before, after []byte
	}
	regions := []region{
		{0x8400, attackContactBytes(t, row.Before.WorldBytes, 4096), attackContactBytes(t, row.After.WorldBytes, 4096)},
		{0x9400, attackContactBytes(t, row.Before.Pool.Bytes, 2048), attackContactBytes(t, row.After.Pool.Bytes, 2048)},
		{0xa600, attackContactBytes(t, row.Before.Throws, 512), attackContactBytes(t, row.After.Throws, 512)},
	}
	for _, value := range []struct {
		offset        int
		before, after uint16
	}{
		{0xa00c, row.Before.ThrowOffset, row.After.ThrowOffset},
		{0xa036, row.Before.Pool.Cursor, row.After.Pool.Cursor},
	} {
		regions = append(regions, region{value.offset, []byte{byte(value.before >> 8), byte(value.before)}, []byte{byte(value.after >> 8), byte(value.after)}})
	}
	type delta struct {
		before, after uint8
		owner         string
	}
	deltas, current := map[uint16]delta{}, map[uint16]uint8{}
	previous := -1
	for _, change := range audit.Changes {
		offset, old, next := fighter7AuditOffset(t, change[0]), fighter7AuditByte(t, change[1]), fighter7AuditByte(t, change[2])
		var owner string
		if err := json.Unmarshal(change[3], &owner); err != nil {
			t.Fatal(err)
		}
		if int(offset) <= previous || old == next {
			t.Fatal("unordered or redundant CPU delta")
		}
		previous = int(offset)
		switch owner {
		case "actor":
			if offset < row.ObjectID || int(offset) >= int(row.ObjectID)+256 {
				t.Fatal("actor delta crosses owner")
			}
		case "effect-pool":
			if offset < 0x9400 || offset >= 0x9c00 {
				t.Fatal("pool delta crosses owner")
			}
		case "throw-record":
			if offset < 0xa600 || offset >= 0xa800 {
				t.Fatal("throw delta crosses owner")
			}
		case "shared-cursor":
			if offset != 0xa00c && offset != 0xa00d && offset != 0xa036 && offset != 0xa037 {
				t.Fatal("unowned cursor delta")
			}
		case "observed-stack":
			if offset < uint16(row.MinSP) || offset >= uint16(row.EntrySP) {
				t.Fatal("stack delta lacks observed SP range")
			}
		case "observed-external":
			current[offset] = old
		default:
			t.Fatal("unexplained CPU delta", owner)
		}
		deltas[offset] = delta{old, next, owner}
	}
	for _, region := range regions {
		for index, old := range region.before {
			next, offset := region.after[index], uint16(region.offset+index)
			change, exists := deltas[offset]
			if old != next && (!exists || change.before != old || change.after != next) || old == next && exists {
				t.Fatal("CPU audit differs from complete captured bytes", offset)
			}
		}
	}
	for _, event := range row.ExternalEvents {
		switch event.Kind {
		case "irq":
			if event.EntryPC != "0xfff000" || event.EntryRawPC != "0xfff000" && event.EntryRawPC != "0xfffff000" ||
				event.ExitPC != "0xfff09a" && event.ExitPC != "0xfff0a8" && event.ExitPC != "0xfff13a" || event.ExitRawPC == "" {
				t.Fatal("unbound IRQ span")
			}
			for _, change := range event.Changes {
				offset := fighter7AuditOffset(t, change[0])
				if value, exists := current[offset]; exists {
					if value != fighter7AuditByte(t, change[1]) {
						t.Fatal("IRQ delta chain skipped an intermediate write", offset)
					}
					current[offset] = fighter7AuditByte(t, change[2])
				}
			}
		case "sound-consumer":
			if event.PC != "0x37c56" && event.PC != "0x37cac" || event.After != 0 {
				t.Fatal("unbound external sound consumer")
			}
		default:
			t.Fatal("unknown external source7 event")
		}
	}
	for offset, value := range current {
		if value != deltas[offset].after {
			t.Fatal("IRQ chain does not reach complete CPU result", offset)
		}
	}
}

func decodeFighter7Attack(t *testing.T, raw fighter7AttackSnapshot, id uint16) Fighter7AttackState {
	t.Helper()
	a := attackContactBytes(t, raw.ActorBytes, 256)
	world := attackContactBytes(t, raw.WorldBytes, 4096)
	offset := int(id) - 0x8400
	if offset < 0 || offset+1024 > len(world) || a[0x58] != 7 || !bytes.Equal(world[offset:offset+256], a) ||
		hex.EncodeToString(world[offset+256:offset+1024]) != raw.ChildrenBytes ||
		hex.EncodeToString(world[offset+256:offset+512]) != raw.ChildBytes {
		t.Fatal("unbound source7 actor/children/world")
	}
	state := Fighter7AttackState{ChuAttackState: decodeChuAttack(t, raw.chuAttackSnapshot, id), AlternateVelocity: int32(binary.BigEndian.Uint32(a[0x48:]))}
	if a[0x48] != raw.Aux48High || binary.BigEndian.Uint16(a[0xba:]) != raw.EnergyFraction || binary.BigEndian.Uint32(a[0x48:]) != raw.Aux48 {
		t.Fatal("unbound energy fraction or Aux48 byte view")
	}
	if !reflect.DeepEqual(encodeFighter7Attack(t, state, raw), raw) {
		t.Fatal("source7 semantic input differs from full original bytes")
	}
	return state
}

func encodeFighter7Attack(t *testing.T, state Fighter7AttackState, previous fighter7AttackSnapshot) fighter7AttackSnapshot {
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
	raw.Aux48High, raw.Aux48 = a[0x48], binary.BigEndian.Uint32(a[0x48:])
	raw.EnergyFraction = binary.BigEndian.Uint16(a[0xba:])
	world := attackContactBytes(t, previous.WorldBytes, 4096)
	offset := int(state.ObjectID) - 0x8400
	copy(world[offset:offset+256], a)
	raw.WorldBytes = hex.EncodeToString(world)
	raw.ChildrenBytes = hex.EncodeToString(world[offset+256 : offset+1024])
	return raw
}

// Original CPU fixtures and these tests were frozen before the source7 module.
// Stream the original JSON so the full worlds are not retained twice in memory.
func visitFighter7Attacks(t *testing.T, name string, visit func(*json.Decoder)) fighter7AttackManifest {
	t.Helper()
	manifestBytes, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest fighter7AttackManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	if !manifest.Pass || !manifest.FullRAM || manifest.CPUBytes != 65536 || manifest.UnknownWrites != 0 || (!manifest.Baseline && !manifest.PrivateContext) || manifest.RNGCalls != 0 || hex.EncodeToString(zh[:]) != manifest.Gzip {
		t.Fatal("unverified original source7 selector fixture")
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
		t.Fatal("unbound complete original source7 fixture")
	}
	return manifest
}

func TestFighter7AttacksOriginalFullCalls(t *testing.T) {
	names := []string{"fighter7-attacks-natural", "fighter7-attacks-oracle", "fighter7-attacks-boundaries"}
	if _, err := os.Stat("testdata/fighter7-attacks-special-natural-manifest.json"); err == nil {
		names = append(names, "fighter7-attacks-special-natural")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	} else {
		t.Log("source7 special natural evidence not captured yet; private roots are separate")
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			calls := 0
			natural := name == "fighter7-attacks-natural" || name == "fighter7-attacks-special-natural"
			modes, sources := map[string]int{}, map[uint16]int{}
			manifest := visitFighter7Attacks(t, name, func(decoder *json.Decoder) {
				for decoder.More() {
					var row fighter7AttackCall
					if err := decoder.Decode(&row); err != nil {
						t.Fatal(err)
					}
					if row.CharacterID != 7 || row.ExitSP != row.EntrySP+4 {
						t.Fatal("not a complete source7 return")
					}
					verifyFighter7RAM(t, row)
					input := decodeFighter7Attack(t, row.Before, row.ObjectID)
					copy := decodeFighter7Attack(t, row.Before, row.ObjectID)
					got, err := SelectFighter7Attack(input, chuAttackMode(t, row.Kind))
					want := decodeFighter7Attack(t, row.After, row.ObjectID)
					if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeFighter7Attack(t, got, row.Before), row.After) {
						t.Fatalf("source7 %d %s/%s differs: %v; body=%+v want=%+v energy=%d/%d gravity=%x/%x", calls, row.Kind, row.Name, err, got.Body, want.Body, got.Energy, want.Energy, got.Body.Gravity, want.Body.Gravity)
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
			if natural && calls == 0 {
				t.Log("passive original baseline matched; no source7 natural selector call in this input")
				return
			}
			if !natural && (len(modes) != 4 || len(sources) != 4) {
				t.Fatal("missing selector or object-address coverage")
			}
			if name == "fighter7-attacks-boundaries" {
				for _, entry := range []string{"0x17840", "0x18812", "0x18828", "0x1883e", "0x18854", "0x1886a", "0x18880", "0x188d8", "0x188ee", "0x18930", "0x195dc", "0x195fa", "0x19624", "0x19676", "0x1968e", "0x196a6", "0x196d6", "0x196ee", "0x19706", "0x19736", "0x1975e", "0x19798"} {
					if manifest.Constructors[entry] == 0 {
						t.Fatal("missing original constructor/throw entry", entry)
					}
				}
			}
			t.Logf("complete original source7 calls=%d modes=%v objects=%v", calls, modes, sources)
		})
	}
}

func TestFighter7AttacksSingleInitialState(t *testing.T) {
	streams, steps := 0, 0
	manifest := visitFighter7Attacks(t, "fighter7-attacks-sequences", func(decoder *json.Decoder) {
		for decoder.More() {
			var stream struct {
				Kind    string
				Initial fighter7AttackSnapshot
				Calls   []fighter7AttackCall
			}
			if err := decoder.Decode(&stream); err != nil || len(stream.Calls) == 0 {
				t.Fatal("missing original stream", err)
			}
			state := decodeFighter7Attack(t, stream.Initial, stream.Calls[0].ObjectID)
			encoded := stream.Initial
			for index, call := range stream.Calls {
				verifyFighter7RAM(t, call)
				state.Controls.Held = state.Controls.Held&0xff00 | Buttons(call.External.Held)
				state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(call.External.Pressed)
				state.Controls.Changed = state.Controls.Changed&0xff00 | Buttons(call.External.Changed)
				encoded = encodeFighter7Attack(t, state, encoded)
				if !reflect.DeepEqual(encoded, call.Before) {
					t.Fatal("original stream changed an undeclared intermediate state")
				}
				var err error
				state, err = SelectFighter7Attack(state, chuAttackMode(t, call.Kind))
				encoded = encodeFighter7Attack(t, state, encoded)
				if err != nil || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeFighter7Attack(t, call.After, call.ObjectID)) {
					t.Fatalf("source7 stream %s/%d step%d differs: %v", stream.Kind, streams, index, err)
				}
				steps++
			}
			streams++
		}
	})
	if streams != manifest.Records || streams != 52 || steps != manifest.Calls || steps != 1040 {
		t.Fatal("missing continuous source7 evidence")
	}
}

func TestFighter7AttacksInvalidInputIsAtomic(t *testing.T) {
	var base Fighter7AttackState
	visitFighter7Attacks(t, "fighter7-attacks-oracle", func(decoder *json.Decoder) {
		for decoder.More() {
			var row fighter7AttackCall
			if err := decoder.Decode(&row); err != nil {
				t.Fatal(err)
			}
			if base.ObjectID == 0 {
				base = decodeFighter7Attack(t, row.Before, row.ObjectID)
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
		got, err := SelectFighter7Attack(input, mode)
		if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
			t.Fatal("invalid source7 input leaked actor/pool/throw changes")
		}
	}
	for _, mode := range []ChuAttackMode{ChuGroundAttack} {
		for _, offset := range []uint16{1, uint16(len(base.Throws.Records) * 32)} {
			input := base
			input.Body.Request = 0
			input.Controls.Held, input.Controls.Pressed = ButtonRight, ButtonA
			input.Throws.Offset = offset
			got, err := SelectFighter7Attack(input, mode)
			if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input.Throws.Records, base.Throws.Records) {
				t.Fatal("late throw storage failure leaked changes")
			}
		}
	}
	// Source7 has no air-throw prelude, so inaccessible throw/pool storage is
	// not consulted by this mode. Its ordinary attack still runs normally.
	input := base
	input.Controls.Held, input.Controls.Pressed = ButtonRight, ButtonA
	input.Throws.Offset, input.Throws.Records, input.Pool.Cursor = 1, nil, 1
	got, err := SelectFighter7Attack(input, ChuAirAttack)
	if err != nil || got.Body.ActionKind != 2 || got.Body.Pose != 0x95 || !reflect.DeepEqual(got.Throws, input.Throws) || got.Pool != input.Pool || got.AlternateVelocity != input.AlternateVelocity {
		t.Fatal("source7 air selector read unused throw storage or failed ordinary attack")
	}
}
