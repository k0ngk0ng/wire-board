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

type bodyOriginalSnapshot struct {
	Count      uint16
	ShapeBytes string            `json:"shape_bytes"`
	ActorBytes map[uint16]string `json:"actor_bytes"`
}

func decodeBodySnapshot(t *testing.T, raw bodyOriginalSnapshot) BodySeparationState {
	t.Helper()
	s := BodySeparationState{Actors: make(map[uint16]BodySeparationActor)}
	s.Buffer.Offset = raw.Count
	b, err := hex.DecodeString(raw.ShapeBytes)
	if err != nil || len(b) != 256 {
		t.Fatal("invalid body buffer")
	}
	for i := range s.Buffer.Records {
		v := b[i*16 : (i+1)*16]
		s.Buffer.Records[i] = AnimationShapeRecord{X: int16(binary.BigEndian.Uint16(v)), Y: int16(binary.BigEndian.Uint16(v[4:])), Width: v[8], Height: v[9], Owner: binary.BigEndian.Uint16(v[12:]), Preserved: [4]uint16{binary.BigEndian.Uint16(v[2:]), binary.BigEndian.Uint16(v[6:]), binary.BigEndian.Uint16(v[10:]), binary.BigEndian.Uint16(v[14:])}}
	}
	for id, encoded := range raw.ActorBytes {
		a, err := hex.DecodeString(encoded)
		if err != nil || len(a) != 256 {
			t.Fatal("invalid body owner")
		}
		s.Actors[id] = BodySeparationActor{X: int32(binary.BigEndian.Uint32(a[0x40:])), Ground: int32(binary.BigEndian.Uint32(a[0x9c:])), Team: a[0x6e], ContactFlags: a[0x89]}
	}
	return s
}

func encodeBodySnapshot(t *testing.T, state BodySeparationState, previous bodyOriginalSnapshot) bodyOriginalSnapshot {
	t.Helper()
	s := bodyOriginalSnapshot{Count: state.Buffer.Offset, ActorBytes: make(map[uint16]string)}
	b, err := hex.DecodeString(previous.ShapeBytes)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range state.Buffer.Records {
		v := b[i*16 : (i+1)*16]
		binary.BigEndian.PutUint16(v, uint16(r.X))
		binary.BigEndian.PutUint16(v[4:], uint16(r.Y))
		v[8], v[9] = r.Width, r.Height
		binary.BigEndian.PutUint16(v[12:], r.Owner)
		for n, offset := range []int{2, 6, 10, 14} {
			binary.BigEndian.PutUint16(v[offset:], r.Preserved[n])
		}
	}
	s.ShapeBytes = hex.EncodeToString(b)
	for id, actor := range state.Actors {
		a, err := hex.DecodeString(previous.ActorBytes[id])
		if err != nil || len(a) != 256 {
			t.Fatal("missing original actor")
		}
		binary.BigEndian.PutUint32(a[0x40:], uint32(actor.X))
		binary.BigEndian.PutUint32(a[0x9c:], uint32(actor.Ground))
		a[0x6e], a[0x89] = actor.Team, actor.ContactFlags
		s.ActorBytes[id] = hex.EncodeToString(a)
	}
	return s
}

// Captured original calls, the failure matrix and these tests precede the Go
// implementation. Complete owners and buffer bytes are compared without masks.
func TestBodySeparationAgainstOriginal(t *testing.T) {
	for _, name := range []string{"body-separation-oracle", "body-separation-boundaries"} {
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
			mraw, err := os.ReadFile("testdata/" + name + "-manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Pass     bool
				Records  int
				SHA      string `json:"fixture_sha256"`
				Gzip     string `json:"gzip_sha256"`
				Baseline bool   `json:"frames_match_original_core"`
				External int    `json:"external_change_calls"`
			}
			if err = json.Unmarshal(mraw, &manifest); err != nil {
				t.Fatal(err)
			}
			h, zh := sha256.Sum256(raw), sha256.Sum256(compressed)
			if !manifest.Pass || !manifest.Baseline || manifest.Records == 0 || manifest.External != 0 || hex.EncodeToString(h[:]) != manifest.SHA || hex.EncodeToString(zh[:]) != manifest.Gzip {
				t.Fatal("unverified original body fixture")
			}
			var rows []struct {
				Name          string
				Before, After bodyOriginalSnapshot
			}
			if err = json.Unmarshal(raw, &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != manifest.Records {
				t.Fatal("missing body calls")
			}
			for i, row := range rows {
				state := decodeBodySnapshot(t, row.Before)
				original, _ := json.Marshal(state)
				got, err := SeparateBodies(state)
				if err != nil {
					t.Fatalf("call %d %s: %v", i, row.Name, err)
				}
				if !reflect.DeepEqual(encodeBodySnapshot(t, got, row.Before), row.After) {
					t.Fatalf("full state mismatch call %d %s: got %+v want %+v", i, row.Name, got, decodeBodySnapshot(t, row.After))
				}
				unchanged, _ := json.Marshal(state)
				if !bytes.Equal(original, unchanged) {
					t.Fatalf("call %d mutated caller state", i)
				}
			}
		})
	}
}

func TestBodySeparationValidationIsAtomic(t *testing.T) {
	base := BodySeparationState{Actors: map[uint16]BodySeparationActor{0x8400: {X: 123, Team: 1}, 0x9000: {X: 456, Team: 2}}, Buffer: AnimationShapeBuffer{Offset: 32}}
	base.Buffer.Records[0] = AnimationShapeRecord{X: 10, Width: 15, Height: 15, Owner: 0x8400}
	base.Buffer.Records[1] = AnimationShapeRecord{X: 10, Width: 15, Height: 15, Owner: 0x9000}
	for _, offset := range []uint16{1, 15, 17, 255, 257, 65535} {
		state := base
		state.Buffer.Offset = offset
		before, _ := json.Marshal(state)
		got, err := SeparateBodies(state)
		after, _ := json.Marshal(state)
		if err == nil || !reflect.DeepEqual(got, state) || !bytes.Equal(before, after) {
			t.Fatal("invalid buffer partially changed state")
		}
	}
	missing := base
	missing.Buffer.Offset = 48
	missing.Buffer.Records[2].Owner = 0x9800
	before, _ := json.Marshal(missing)
	got, err := SeparateBodies(missing)
	after, _ := json.Marshal(missing)
	if err == nil || !reflect.DeepEqual(got, missing) || !bytes.Equal(before, after) {
		t.Fatal("late missing owner partially changed state")
	}
	if _, err = SeparateBodies(BodySeparationState{}); err != nil {
		t.Fatal("empty shape list requires owners")
	}
}
