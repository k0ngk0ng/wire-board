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

// This fixture and test precede effects.go. Unknown bytes are carried only in
// the test adapter's input snapshot, then all 0x800 output bytes are compared.
func decodeEffectPool(t *testing.T, cursor uint16, encoded string) (EffectPool, []byte) {
	t.Helper()
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != 0x800 {
		t.Fatalf("invalid effect pool: %v, %d", err, len(raw))
	}
	pool := EffectPool{Cursor: cursor}
	for i := range pool.Slots {
		b := raw[i*0x80 : (i+1)*0x80]
		pool.Slots[i] = EffectSlot{Behavior: binary.BigEndian.Uint32(b), Size: binary.BigEndian.Uint16(b[4:]), Stage: binary.BigEndian.Uint16(b[6:]),
			VelocityX: int32(binary.BigEndian.Uint32(b[0x10:])), Attributes: binary.BigEndian.Uint16(b[0x24:]), Layer: b[0x26],
			Depth: binary.BigEndian.Uint16(b[0x3e:]), X: int32(binary.BigEndian.Uint32(b[0x40:])), Y: int32(binary.BigEndian.Uint32(b[0x44:])), Type: b[0x58]}
	}
	return pool, raw
}

func encodeEffectPool(pool EffectPool, original []byte) []byte {
	raw := append([]byte(nil), original...)
	for i, s := range pool.Slots {
		b := raw[i*0x80 : (i+1)*0x80]
		binary.BigEndian.PutUint32(b, s.Behavior)
		binary.BigEndian.PutUint16(b[4:], s.Size)
		binary.BigEndian.PutUint16(b[6:], s.Stage)
		binary.BigEndian.PutUint32(b[0x10:], uint32(s.VelocityX))
		binary.BigEndian.PutUint16(b[0x24:], s.Attributes)
		b[0x26] = s.Layer
		binary.BigEndian.PutUint16(b[0x3e:], s.Depth)
		binary.BigEndian.PutUint32(b[0x40:], uint32(s.X))
		binary.BigEndian.PutUint32(b[0x44:], uint32(s.Y))
		b[0x58] = s.Type
	}
	return raw
}

func TestEffectPoolOriginalCalls(t *testing.T) {
	compressed, err := os.ReadFile("testdata/effects-oracle.json.gz")
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
	var rows []struct {
		Scope         string       `json:"scope"`
		Kind          string       `json:"kind"`
		Name          string       `json:"name"`
		Source        EffectSource `json:"source"`
		Before, After struct {
			Cursor uint16 `json:"cursor"`
			Bytes  string `json:"bytes"`
		}
		Slots []int `json:"returned_slots"`
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	manifestRaw, err := os.ReadFile("testdata/effects-oracle-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pass    bool   `json:"pass"`
		Match   bool   `json:"frames_match_original_core"`
		Calls   int    `json:"complete_calls"`
		SHA     string `json:"fixture_sha256"`
		GzipSHA string `json:"gzip_sha256"`
	}
	if err = json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	digest, zipDigest := sha256.Sum256(raw), sha256.Sum256(compressed)
	if !manifest.Pass || !manifest.Match || manifest.Calls != len(rows) || len(rows) == 0 || hex.EncodeToString(digest[:]) != manifest.SHA || hex.EncodeToString(zipDigest[:]) != manifest.GzipSHA {
		t.Fatal("unverified original effect evidence")
	}
	natural, isolated := 0, 0
	for i, r := range rows {
		input, before := decodeEffectPool(t, r.Before.Cursor, r.Before.Bytes)
		want, after := decodeEffectPool(t, r.After.Cursor, r.After.Bytes)
		var got EffectPool
		var slots []int
		switch r.Kind {
		case "allocate":
			var selected int
			got, selected, err = AllocateEffect(input)
			slots = []int{selected}
		case "pair":
			var selected [2]int
			got, selected, err = SpawnMovementDust(input, r.Source)
			slots = selected[:]
		default:
			t.Fatalf("unknown original routine %q", r.Kind)
		}
		if err != nil || got != want || !reflect.DeepEqual(slots, r.Slots) || !bytes.Equal(encodeEffectPool(got, before), after) {
			t.Fatalf("case %d %s/%s differs: err=%v cursor=%x/%x slots=%v/%v", i, r.Scope, r.Name, err, got.Cursor, want.Cursor, slots, r.Slots)
		}
		if r.Scope == "natural" {
			natural++
		} else if r.Scope == "isolated-original-cpu" {
			isolated++
		} else {
			t.Fatalf("unknown evidence scope %q", r.Scope)
		}
	}
	if natural == 0 || isolated == 0 {
		t.Fatal("missing natural or isolated original coverage")
	}
	t.Logf("Complete original pool calls: natural=%d isolated=%d", natural, isolated)
}

func TestEffectPoolRejectsUnalignedCursorWithoutMutation(t *testing.T) {
	for _, cursor := range []uint16{1, 0x7f, 0xffff} {
		pool := EffectPool{Cursor: cursor}
		pool.Slots[4].Stage = 0xa55a
		got, index, err := AllocateEffect(pool)
		if err == nil || got != pool || index != -1 {
			t.Fatal("invalid cursor was silently normalized or state mutated")
		}
		got, indices, err := SpawnMovementDust(pool, EffectSource{})
		if err == nil || got != pool || indices != [2]int{-1, -1} {
			t.Fatal("invalid paired allocation mutated state")
		}
	}
}
