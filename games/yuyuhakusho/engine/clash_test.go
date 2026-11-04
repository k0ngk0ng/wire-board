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

type clashOriginalSnapshot struct {
	Offset       uint16
	Shapes       string
	Actors       map[uint16]string
	Effects      string
	Cursor       uint16
	PendingSound uint8 `json:"pending_sound"`
}

func decodeClashSnapshot(t *testing.T, original clashOriginalSnapshot) ClashState {
	t.Helper()
	state := ClashState{Actors: map[uint16]ClashActor{}, PendingSound: original.PendingSound}
	state.Effects, _ = decodeEffectPool(t, original.Cursor, original.Effects)
	state.Buffer.Offset = original.Offset
	shapeBytes, err := hex.DecodeString(original.Shapes)
	if err != nil || len(shapeBytes) != 256 {
		t.Fatal("invalid original clash shapes", err)
	}
	for i := range state.Buffer.Records {
		b := shapeBytes[i*16 : (i+1)*16]
		state.Buffer.Records[i] = AnimationShapeRecord{X: int16(binary.BigEndian.Uint16(b)), Y: int16(binary.BigEndian.Uint16(b[4:])), Width: b[8], Height: b[9], Owner: binary.BigEndian.Uint16(b[12:]), Preserved: [4]uint16{binary.BigEndian.Uint16(b[2:]), binary.BigEndian.Uint16(b[6:]), binary.BigEndian.Uint16(b[10:]), binary.BigEndian.Uint16(b[14:])}}
	}
	for id, encoded := range original.Actors {
		b, err := hex.DecodeString(encoded)
		if err != nil || len(b) != 256 {
			t.Fatal("invalid original clash actor", err)
		}
		state.Actors[id] = ClashActor{Team: b[0x6e], Ground: int32(binary.BigEndian.Uint32(b[0x9c:])), Y: int32(binary.BigEndian.Uint32(b[0x44:])),
			TargetBit: b[0x70], TargetMask: binary.BigEndian.Uint32(b[0x94:]), AttackValue: b[0x65], AttackFlags: b[0x64], FrameTicks: b[0x5b],
			ContactFlags: b[0x89], HitStopTicks: b[0x69], InputRetention: b[0x75], CancelTicks: b[0x6d], Residual: int32(binary.BigEndian.Uint32(b[0x98:])),
			Shake: b[0x6a], Attributes: binary.BigEndian.Uint16(b[0x24:]), Layer: b[0x26], Depth: binary.BigEndian.Uint16(b[0x3e:])}
	}
	return state
}

func encodeClashSnapshot(t *testing.T, state ClashState, original clashOriginalSnapshot) clashOriginalSnapshot {
	t.Helper()
	result := original
	result.Offset, result.Cursor, result.PendingSound = state.Buffer.Offset, state.Effects.Cursor, state.PendingSound
	result.Actors = make(map[uint16]string, len(state.Actors))
	for id, a := range state.Actors {
		b, err := hex.DecodeString(original.Actors[id])
		if err != nil || len(b) != 256 {
			t.Fatal("missing original clash actor")
		}
		b[0x6e], b[0x70], b[0x65], b[0x64], b[0x5b] = a.Team, a.TargetBit, a.AttackValue, a.AttackFlags, a.FrameTicks
		b[0x89], b[0x69], b[0x75], b[0x6d], b[0x6a] = a.ContactFlags, a.HitStopTicks, a.InputRetention, a.CancelTicks, a.Shake
		binary.BigEndian.PutUint32(b[0x9c:], uint32(a.Ground))
		binary.BigEndian.PutUint32(b[0x44:], uint32(a.Y))
		binary.BigEndian.PutUint32(b[0x94:], a.TargetMask)
		binary.BigEndian.PutUint32(b[0x98:], uint32(a.Residual))
		binary.BigEndian.PutUint16(b[0x24:], a.Attributes)
		b[0x26] = a.Layer
		binary.BigEndian.PutUint16(b[0x3e:], a.Depth)
		result.Actors[id] = hex.EncodeToString(b)
	}
	shapeBytes := make([]byte, 256)
	for i, r := range state.Buffer.Records {
		b := shapeBytes[i*16 : (i+1)*16]
		binary.BigEndian.PutUint16(b, uint16(r.X))
		binary.BigEndian.PutUint16(b[4:], uint16(r.Y))
		b[8], b[9] = r.Width, r.Height
		binary.BigEndian.PutUint16(b[12:], r.Owner)
		for j, off := range []int{2, 6, 10, 14} {
			binary.BigEndian.PutUint16(b[off:], r.Preserved[j])
		}
	}
	result.Shapes = hex.EncodeToString(shapeBytes)
	oldEffects, err := hex.DecodeString(original.Effects)
	if err != nil {
		t.Fatal(err)
	}
	result.Effects = hex.EncodeToString(encodeEffectPool(state.Effects, oldEffects))
	return result
}

// The original complete-call fixtures and these tests precede clash.go.
func TestClashOriginalCompleteCalls(t *testing.T) {
	for _, name := range []string{"clash-oracle", "clash-boundaries"} {
		t.Run(name, func(t *testing.T) {
			compressed, err := os.ReadFile("testdata/" + name + ".json.gz")
			if err != nil {
				t.Fatal(err)
			}
			z, err := gzip.NewReader(bytes.NewReader(compressed))
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			raw, err := io.ReadAll(z)
			if err != nil {
				t.Fatal(err)
			}
			manifestRaw, err := os.ReadFile("testdata/" + name + "-manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Pass     bool
				Records  int
				Contacts int
				Match    bool   `json:"frames_match_original_core"`
				SHA      string `json:"fixture_sha256"`
				Gzip     string `json:"gzip_sha256"`
			}
			if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
				t.Fatal(err)
			}
			h, zh := sha256.Sum256(raw), sha256.Sum256(compressed)
			if !manifest.Pass || !manifest.Match || manifest.Records == 0 || hex.EncodeToString(h[:]) != manifest.SHA || hex.EncodeToString(zh[:]) != manifest.Gzip {
				t.Fatal("unverified original clash fixture")
			}
			var rows []struct {
				Name          string
				Before, After clashOriginalSnapshot
				Contacts      int
			}
			if err := json.Unmarshal(raw, &rows); err != nil || len(rows) != manifest.Records {
				t.Fatal("missing original clash calls", err)
			}
			contacts := 0
			for i, row := range rows {
				input := decodeClashSnapshot(t, row.Before)
				inputCopy := decodeClashSnapshot(t, row.Before)
				want := decodeClashSnapshot(t, row.After)
				got, err := ResolveClashes(input)
				if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeClashSnapshot(t, got, row.Before), row.After) {
					t.Fatalf("clash %d %s: complete original state differs: %v", i, row.Name, err)
				}
				if !reflect.DeepEqual(input, inputCopy) {
					t.Fatal("clash mutated caller's owner map")
				}
				contacts += row.Contacts
			}
			if contacts != manifest.Contacts || name == "clash-boundaries" && contacts == 0 {
				t.Fatal("missing original contact coverage")
			}
			t.Logf("complete original clash calls=%d contacts=%d", len(rows), contacts)
		})
	}
}

func TestClashInvalidStateIsAtomic(t *testing.T) {
	base := ClashState{Actors: map[uint16]ClashActor{1: {Team: 0, TargetMask: 0xffffffff}, 2: {Team: 1, TargetMask: 0xffffffff}}, Buffer: AnimationShapeBuffer{Offset: 32}}
	base.Buffer.Records[0].Owner, base.Buffer.Records[1].Owner = 1, 2
	for _, offset := range []uint16{1, 15, 257, 0xffff} {
		input := base
		input.Buffer.Offset = offset
		got, err := ResolveClashes(input)
		if err == nil || !reflect.DeepEqual(got, input) {
			t.Fatal("invalid offset changed clash state")
		}
	}
	missing := base
	missing.Buffer.Records[1].Owner = 3
	got, err := ResolveClashes(missing)
	if err == nil || !reflect.DeepEqual(got, missing) {
		t.Fatal("missing owner caused partial collision")
	}
	badPool := base
	badPool.Effects.Cursor = 1
	got, err = ResolveClashes(badPool)
	if err == nil || !reflect.DeepEqual(got, badPool) || base.Actors[1].ContactFlags != 0 {
		t.Fatal("invalid pool caused partial collision")
	}
}
