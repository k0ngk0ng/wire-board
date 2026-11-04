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

type fighterBoundsSnapshot struct {
	Actor, Samples string
	Used           uint16
	Left, Right    int16
}

type fighterBoundsCall struct {
	Name          string
	Before, After fighterBoundsSnapshot
	External      *struct{ X, Y int32 } `json:"external_position"`
}

func boundsBytes(t *testing.T, value string, size int) []byte {
	t.Helper()
	b, err := hex.DecodeString(value)
	if err != nil || len(b) != size {
		t.Fatalf("invalid original bounds bytes: %v size%d/%d", err, len(b), size)
	}
	return b
}

func decodeFighterBounds(t *testing.T, raw fighterBoundsSnapshot) FighterBoundsState {
	a := boundsBytes(t, raw.Actor, 256)
	b := boundsBytes(t, raw.Samples, 128)
	s := FighterBoundsState{X: int32(binary.BigEndian.Uint32(a[0x40:])), Y: int32(binary.BigEndian.Uint32(a[0x44:])), Used: raw.Used, Left: raw.Left, Right: raw.Right}
	for i := range s.XSamples {
		s.XSamples[i], s.YSamples[i] = int32(binary.BigEndian.Uint32(b[i*4:])), int32(binary.BigEndian.Uint32(b[64+i*4:]))
	}
	return s
}

func encodeFighterBounds(t *testing.T, state FighterBoundsState, previous fighterBoundsSnapshot) fighterBoundsSnapshot {
	raw := previous
	a := boundsBytes(t, raw.Actor, 256)
	binary.BigEndian.PutUint32(a[0x40:], uint32(state.X))
	binary.BigEndian.PutUint32(a[0x44:], uint32(state.Y))
	raw.Actor = hex.EncodeToString(a)
	b := make([]byte, 128)
	for i := range state.XSamples {
		binary.BigEndian.PutUint32(b[i*4:], uint32(state.XSamples[i]))
		binary.BigEndian.PutUint32(b[64+i*4:], uint32(state.YSamples[i]))
	}
	raw.Samples, raw.Used, raw.Left, raw.Right = hex.EncodeToString(b), state.Used, state.Left, state.Right
	return raw
}

func readFighterBounds(t *testing.T, name string, target any) int {
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
	mb, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass    bool
		Records int
		Match   bool   `json:"frames_match_original_core"`
		SHA     string `json:"fixture_sha256"`
		Gzip    string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Match || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original fighter-bounds evidence")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
	return m.Records
}

// Original full-function fixtures and these tests precede fighter_bounds.go.
func TestFighterBoundsOriginalCalls(t *testing.T) {
	for _, name := range []string{"fighter-bounds-oracle", "fighter-bounds-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []fighterBoundsCall
			count := readFighterBounds(t, name, &rows)
			if len(rows) != count {
				t.Fatal("missing original bounds calls")
			}
			for i, row := range rows {
				input := decodeFighterBounds(t, row.Before)
				got, err := StepFighterBounds(input)
				if err != nil || got != decodeFighterBounds(t, row.After) || !reflect.DeepEqual(encodeFighterBounds(t, got, row.Before), row.After) {
					t.Fatalf("bounds %d %s differs: %v", i, row.Name, err)
				}
			}
			t.Logf("complete original fighter-bounds calls=%d", count)
		})
	}
}

func TestFighterBoundsSingleInitialState(t *testing.T) {
	var rows []struct {
		Initial fighterBoundsSnapshot
		Calls   []fighterBoundsCall
	}
	count := readFighterBounds(t, "fighter-bounds-sequences", &rows)
	if len(rows) != count || count != 4 {
		t.Fatal("missing original bounds streams")
	}
	steps := 0
	for _, row := range rows {
		state := decodeFighterBounds(t, row.Initial)
		encoded := row.Initial
		for i, step := range row.Calls {
			if step.External == nil {
				t.Fatal("missing declared upstream position input")
			}
			state.X, state.Y = step.External.X, step.External.Y
			var err error
			state, err = StepFighterBounds(state)
			encoded = encodeFighterBounds(t, state, encoded)
			if err != nil || state != decodeFighterBounds(t, step.After) || !reflect.DeepEqual(encoded, step.After) {
				t.Fatalf("bounds stream step%d differs: %v", i, err)
			}
			steps++
		}
	}
	if steps != 128 {
		t.Fatal("missing sequential saturation steps")
	}
}

func TestFighterBoundsInvalidLowOffsetsAreAtomic(t *testing.T) {
	for _, used := range []uint16{1, 2, 3, 6, 61, 62} {
		input := FighterBoundsState{X: -1, Y: 0x12345678, Left: 32, Right: 600, Used: used}
		got, err := StepFighterBounds(input)
		if err == nil || got != input {
			t.Fatal("invalid low sample offset leaked clipping or writes")
		}
	}
}
