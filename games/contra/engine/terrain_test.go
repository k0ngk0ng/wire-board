package contra

// Failure scenarios defined before implementation: reversed packed-bit order;
// selecting the wrong streamed screen after horizontal carry; wrapping Y at
// 256 instead of the original 240-pixel nametable boundary; treating the hidden
// bottom area as ground; confusing floor/water/solid codes; flattening partially
// loaded windows into a complete screen; matching a code by chance while reading
// the wrong source byte/column. Full player collision and all stages are separate.

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalTerrainQueries(t *testing.T) {
	raw, err := os.ReadFile("testdata/original-terrain.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != "de35044e7c68f4c99cd70c2103466c8f9770c33cbcf6c8f61b6208aad4ca5925" {
		t.Fatal("original terrain capture identity changed")
	}
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
		Maps []struct {
			ID    int
			Bytes string
		} `json:"collision_maps"`
		Queries []struct {
			Frame, Map                                             int
			X, Y, ScrollX, ScrollY, PPUControl                     uint8
			Code, Carry, Negative, SourceIndex, SourceByte, Column uint8
		}
	}
	if err := json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if len(oracle.Maps) != 13 || len(oracle.Queries) != 3234 {
		t.Fatalf("oracle coverage changed: %d maps / %d queries", len(oracle.Maps), len(oracle.Queries))
	}
	windows := map[int]CollisionWindow{}
	for _, m := range oracle.Maps {
		data, err := hex.DecodeString(m.Bytes)
		if err != nil || len(data) != 128 {
			t.Fatal("invalid original collision window", m.ID, err)
		}
		windows[m.ID] = CollisionWindow(data)
	}
	for i, q := range oracle.Queries {
		window, ok := windows[q.Map]
		if !ok {
			t.Fatal("missing original map", q.Map)
		}
		got := window.At(q.X, q.Y, CollisionScroll{X: q.ScrollX, Y: q.ScrollY, NameTable: q.PPUControl})
		if uint8(got.Kind) != q.Code || uint8(got.Kind)&1 != q.Carry || uint8(got.Kind)&0x80 != q.Negative || got.SourceIndex != q.SourceIndex || got.SourceByte != q.SourceByte || got.Column != q.Column {
			t.Fatalf("original query %d frame %d: got %+v, expected %+v", i, q.Frame, got, q)
		}
	}
	t.Logf("verified %d original terrain reads across %d streamed windows", len(oracle.Queries), len(oracle.Maps))
}

func TestTerrainPackedOrderAndScrollBoundaries(t *testing.T) {
	var window CollisionWindow
	window[0] = 0x1b // From left to right: empty, platform, water, solid.
	for i, want := range []TerrainKind{TerrainEmpty, TerrainPlatform, TerrainWater, TerrainSolid} {
		got := window.At(uint8(i*16), 0, CollisionScroll{})
		if got.Kind != want || got.Column != uint8(i) || got.SourceIndex != 0 {
			t.Fatalf("packed slot %d: %+v want %d", i, got, want)
		}
	}
	window[64] = 0x80
	got := window.At(250, 0, CollisionScroll{X: 20})
	if got.Kind != TerrainWater || got.SourceIndex != 64 || got.Column != 0 {
		t.Fatal("X carry did not switch screens", got)
	}
	got = window.At(250, 0, CollisionScroll{X: 20, NameTable: 0xff})
	if got.Kind != TerrainEmpty || got.SourceIndex != 0 {
		t.Fatal("X carry did not switch back to first screen", got)
	}
	window[56] = 0xc0
	got = window.At(0, 15, CollisionScroll{Y: 224})
	if got.Kind != TerrainSolid || got.SourceIndex != 56 {
		t.Fatal("last row before vertical wrap", got)
	}
	got = window.At(0, 16, CollisionScroll{Y: 224})
	if got.Kind != TerrainEmpty || got.SourceIndex != 0 {
		t.Fatal("vertical wrap must occur at 240", got)
	}
	window[56] = 0xff
	got = window.At(0, 224, CollisionScroll{})
	if got.Kind != TerrainEmpty || got.SourceIndex != 56 || got.SourceByte != 0xff {
		t.Fatal("hidden bottom area must be empty", got)
	}
}
