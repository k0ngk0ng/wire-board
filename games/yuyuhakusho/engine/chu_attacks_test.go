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

type chuAttackSnapshot struct {
	Body             FighterBodyState
	ActorBytes       string `json:"actor_bytes"`
	ChildBytes       string `json:"child_bytes"`
	Held             uint8
	Pressed          uint8
	Changed          uint8
	Energy           int16
	ChildSize        uint16 `json:"child_size"`
	GravityParameter int32  `json:"gravity_parameter"`
	Pool             struct {
		Cursor uint16
		Bytes  string
	}
	Throws      string
	ThrowOffset uint16 `json:"throw_offset"`
}

func decodeChuAttack(t *testing.T, raw chuAttackSnapshot, id uint16) ChuAttackState {
	t.Helper()
	a := attackContactBytes(t, raw.ActorBytes, 256)
	s := ChuAttackState{Body: raw.Body, ObjectID: id, Energy: raw.Energy, ChildSize: raw.ChildSize, GravityParameter: raw.GravityParameter,
		Controls: FighterControls{Held: Buttons(a[0x71]) | Buttons(a[0xc0])<<8, Pressed: Buttons(a[0x72]) | Buttons(a[0xc1])<<8,
			Changed: Buttons(a[0x73]) | Buttons(a[0xc2])<<8, RawHeld: a[0x74], RetainTicks: a[0x75]}}
	s.Pool, _ = decodeEffectPool(t, raw.Pool.Cursor, raw.Pool.Bytes)
	throw := decodeThrowSnapshot(t, throwOriginalSnapshot{GrabOffset: raw.ThrowOffset, Grabs: raw.Throws, Hurts: hex.EncodeToString(make([]byte, 256))})
	s.Throws = throw.Throws
	return s
}

func encodeChuAttack(t *testing.T, s ChuAttackState, previous chuAttackSnapshot) chuAttackSnapshot {
	t.Helper()
	raw := previous
	raw.Body, raw.Energy, raw.ChildSize, raw.GravityParameter = s.Body, s.Energy, s.ChildSize, s.GravityParameter
	a := encodeJumpBody(s.Body, attackContactBytes(t, previous.ActorBytes, 256))
	binary.BigEndian.PutUint16(a[0xb8:], uint16(s.Energy))
	a[0x71], a[0xc0] = uint8(s.Controls.Held), uint8(s.Controls.Held>>8)
	a[0x72], a[0xc1] = uint8(s.Controls.Pressed), uint8(s.Controls.Pressed>>8)
	a[0x73], a[0xc2] = uint8(s.Controls.Changed), uint8(s.Controls.Changed>>8)
	a[0x74], a[0x75] = s.Controls.RawHeld, s.Controls.RetainTicks
	raw.ActorBytes = hex.EncodeToString(a)
	raw.Held, raw.Pressed, raw.Changed = uint8(s.Controls.Held), uint8(s.Controls.Pressed), uint8(s.Controls.Changed)
	raw.Pool.Cursor = s.Pool.Cursor
	raw.Pool.Bytes = hex.EncodeToString(encodeEffectPool(s.Pool, attackContactBytes(t, previous.Pool.Bytes, 2048)))
	throw := encodeThrowSnapshot(t, ThrowContactState{Throws: s.Throws}, throwOriginalSnapshot{Grabs: previous.Throws, Hurts: hex.EncodeToString(make([]byte, 256))})
	raw.Throws, raw.ThrowOffset = throw.Grabs, s.Throws.Offset
	return raw
}

func readChuAttackFixture(t *testing.T, name string, target any) int {
	t.Helper()
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass         bool
		Records      int
		Baseline     bool           `json:"frames_match_original_core"`
		SHA          string         `json:"fixture_sha256"`
		Gzip         string         `json:"gzip_sha256"`
		Constructors map[string]int `json:"constructor_counts"`
	}
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original Chu attack fixture")
	}
	if name == "chu-attacks-boundaries" {
		for _, address := range []string{"0x18812", "0x18828", "0x1883e", "0x17ec2", "0x18854", "0x1886a", "0x18880", "0x188ee", "0x18930", "0x188d8", "0x18ea8", "0x18ec0", "0x1992e", "0x1994e", "0x19980", "0x199be", "0x19a84", "0x18ac4", "0x18adc", "0x19a22", "0x19a4a"} {
			if m.Constructors[address] == 0 {
				t.Fatal("missing selected original constructor", address)
			}
		}
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
	return m.Records
}

func chuAttackMode(t *testing.T, kind string) ChuAttackMode {
	t.Helper()
	switch kind {
	case "ground":
		return ChuGroundAttack
	case "air":
		return ChuAirAttack
	case "ground-special":
		return ChuGroundSpecial
	case "air-special":
		return ChuAirSpecial
	default:
		t.Fatal("unknown original Chu selector", kind)
		return 0
	}
}

type chuAttackCall struct {
	Kind          string
	Name          string
	ObjectID      uint16 `json:"object_id"`
	Before, After chuAttackSnapshot
}

// Original full-return fixtures and these tests precede chu_attacks.go.
func TestChuAttacksOriginalFullCalls(t *testing.T) {
	for _, name := range []string{"chu-attacks-oracle", "chu-attacks-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []chuAttackCall
			count := readChuAttackFixture(t, name, &rows)
			if len(rows) != count {
				t.Fatal("missing original selector calls")
			}
			for i, row := range rows {
				input := decodeChuAttack(t, row.Before, row.ObjectID)
				copy := decodeChuAttack(t, row.Before, row.ObjectID)
				got, err := SelectChuAttack(input, chuAttackMode(t, row.Kind))
				want := decodeChuAttack(t, row.After, row.ObjectID)
				if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeChuAttack(t, got, row.Before), row.After) {
					t.Fatalf("Chu %d %s/%s differs: %v; body=%+v want=%+v energy=%d/%d", i, row.Kind, row.Name, err, got.Body, want.Body, got.Energy, want.Energy)
				}
				if !reflect.DeepEqual(input, copy) {
					t.Fatal("Chu selector mutated caller throw records")
				}
			}
			t.Logf("original complete Chu selector calls=%d", count)
		})
	}
}

func TestChuAttacksSingleInitialState(t *testing.T) {
	var rows []struct {
		Kind    string
		Initial chuAttackSnapshot
		Calls   []chuAttackCall
	}
	count := readChuAttackFixture(t, "chu-attacks-sequences", &rows)
	if len(rows) != count || count != 4 {
		t.Fatal("missing original Chu streams")
	}
	steps := 0
	for _, row := range rows {
		if len(row.Calls) == 0 {
			t.Fatal("empty original Chu stream")
		}
		state := decodeChuAttack(t, row.Initial, row.Calls[0].ObjectID)
		encoded := row.Initial
		for i, call := range row.Calls {
			var err error
			state, err = SelectChuAttack(state, chuAttackMode(t, row.Kind))
			encoded = encodeChuAttack(t, state, encoded)
			if err != nil || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeChuAttack(t, call.After, call.ObjectID)) {
				t.Fatalf("Chu stream %s step %d differs: %v", row.Kind, i, err)
			}
			steps++
		}
	}
	if steps != 32 {
		t.Fatal("missing continuous selector steps")
	}
}

func TestChuAttacksInvalidInputIsAtomic(t *testing.T) {
	var rows []chuAttackCall
	readChuAttackFixture(t, "chu-attacks-boundaries", &rows)
	base := decodeChuAttack(t, rows[0].Before, rows[0].ObjectID)
	got, err := SelectChuAttack(base, ChuAttackMode(255))
	if err == nil || !reflect.DeepEqual(got, base) {
		t.Fatal("unknown selector changed state")
	}
	base.Body.Request = 8
	base.Pool.Cursor = 1
	got, err = SelectChuAttack(base, ChuGroundAttack)
	if err == nil || !reflect.DeepEqual(got, base) {
		t.Fatal("failed dash allocation leaked actor changes")
	}
	base.Body.Request = 0
	base.Controls.Held, base.Controls.Pressed = ButtonRight, ButtonA
	base.Throws.Offset = 1
	got, err = SelectChuAttack(base, ChuGroundAttack)
	if err == nil || !reflect.DeepEqual(got, base) {
		t.Fatal("failed throw storage leaked actor or pool changes")
	}
}
