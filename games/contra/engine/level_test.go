package contra

// Failure scenarios before implementation: compressed row copies reference
// uninitialized output; repeats overrun the screen or omit their value; source
// byte consumption crosses the next stream; tile quadrants/attributes/collision
// bits are transposed; the lookahead alias becomes an extra playable screen;
// callers mutate shared definitions; complete static decoding is confused with
// actual in-game traversal. Only observed screens support runtime fidelity.

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestJungleLayoutOriginalScreens(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/research/nes-contra/evidence/static-level1-terrain.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected struct {
		Screens []struct {
			Supertiles, Tiles, Attributes []uint8
			Packed                        string `json:"collision_packed_hex"`
		}
	}
	if err = json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	if len(expected.Screens) != 14 {
		t.Fatal("source screen count changed")
	}
	for i, want := range expected.Screens {
		screen, err := JungleScreen(i)
		if err != nil {
			t.Fatalf("screen %d: %v", i, err)
		}
		packed, err := hex.DecodeString(want.Packed)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(screen.SupertileIDs, want.Supertiles) || !reflect.DeepEqual(screen.Tiles, want.Tiles) || !reflect.DeepEqual(screen.Attributes, want.Attributes) || !reflect.DeepEqual(screen.Collision[:], packed) {
			t.Fatalf("screen %d differs from source-bound original map", i)
		}
	}
	meta := JungleMetadata()
	if meta.DistinctScreens != 13 || meta.LookaheadAlias != 13 || meta.AutomaticScrollScreen != 11 || meta.AlternateGraphicsScreen != 11 {
		t.Fatal("lookahead/Boss header interpretation changed", meta)
	}
	if _, err = JungleScreen(-1); err == nil {
		t.Fatal("negative screen accepted")
	}
	if _, err = JungleScreen(14); err == nil {
		t.Fatal("past-end screen accepted")
	}
	first, _ := JungleScreen(0)
	first.Tiles[0] ^= 255
	first.Collision[0] ^= 255
	again, _ := JungleScreen(0)
	if first.Tiles[0] == again.Tiles[0] || first.Collision[0] == again.Collision[0] {
		t.Fatal("caller changed shared level data")
	}
}

func TestJungleLayoutNaturalInitializationAndStreaming(t *testing.T) {
	f, err := os.Open("testdata/original-terrain.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Entry struct {
			Supertiles string `json:"supertiles_hex"`
			Collision  string `json:"collision_hex"`
			Nametables string `json:"nametables_hex"`
		}
		Streaming []struct {
			Screen   int
			Indices  string
			Vertical int
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	supertileBytes, _ := hex.DecodeString(oracle.Entry.Supertiles)
	collision, _ := hex.DecodeString(oracle.Entry.Collision)
	nametables, _ := hex.DecodeString(oracle.Entry.Nametables)
	first, _ := JungleScreen(0)
	second, _ := JungleScreen(1)
	if !reflect.DeepEqual(first.SupertileIDs, supertileBytes[:56]) || !reflect.DeepEqual(second.SupertileIDs, supertileBytes[64:120]) {
		t.Fatal("original initial screen decode differs")
	}
	if !reflect.DeepEqual(first.Collision[:], collision[:64]) || !reflect.DeepEqual(first.Tiles, nametables[:896]) || !reflect.DeepEqual(first.Attributes, nametables[0x3c0:0x3f8]) {
		t.Fatal("original first-screen PPU/collision differs")
	}
	for row := 0; row < 28; row++ {
		if !reflect.DeepEqual(second.Tiles[row*32:row*32+16], nametables[0x400+row*32:0x400+row*32+16]) {
			t.Fatalf("original streamed left half differs at row %d", row)
		}
	}
	if len(oracle.Streaming) == 0 {
		t.Fatal("missing natural later-screen decode")
	}
	for _, r := range oracle.Streaming {
		want, _ := hex.DecodeString(r.Indices)
		got, err := JungleScreen(r.Screen)
		if err != nil || r.Vertical != 0 || !reflect.DeepEqual(got.SupertileIDs, want) {
			t.Fatal("natural screen stream differs", r.Screen, err)
		}
	}
}

func TestScreenDecodeBoundaries(t *testing.T) {
	// A literal row followed by six copies is the complete seven-row screen.
	encoded := []byte{1, 2, 3, 4, 5, 6, 7, 8, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 0xf0, 99}
	got, n, err := DecodeScreen(encoded, 7)
	if err != nil || n != 14 || len(got) != 56 {
		t.Fatal("row copy or stream boundary", n, err)
	}
	for i, v := range got {
		if v != uint8(i%8+1) {
			t.Fatal("copied row order", i, v)
		}
	}
	repeated, n, err := DecodeScreen([]byte{0xb8, 5, 88}, 7)
	if err != nil || n != 2 || len(repeated) != 56 {
		t.Fatal("repeat command", n, err)
	}
	for _, v := range repeated {
		if v != 5 {
			t.Fatal("repeat value", v)
		}
	}
	for _, bad := range [][]byte{nil, {0x80, 1}, {0x81}, {0xb9, 1}, {0xf0}, {1, 2, 0xf0}, {0xb1, 1, 0xf0}} {
		if _, _, err := DecodeScreen(bad, 7); err == nil {
			t.Fatalf("invalid stream accepted: %x", bad)
		}
	}
	if _, _, err := DecodeScreen(encoded, 0); err == nil {
		t.Fatal("zero rows accepted")
	}
	if _, _, err := DecodeScreen(encoded, 9); err == nil {
		t.Fatal("oversized screen accepted")
	}
}
