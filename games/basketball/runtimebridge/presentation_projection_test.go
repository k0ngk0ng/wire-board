package runtimebridge

// Before implementation: compact projection must retain every source row;
// reject missing/ambiguous plans, remain read-only and never allocate a game
// session. Wire rows use240 headers (4CHR,mask,u16 count), then8 bytes/segment.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

func TestPresentationProjectionCompactSourceFrames(t *testing.T) {
	f, e := os.Open("../engine/testdata/postresult-raster-rows.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var cases []struct {
		Surface engine.OriginalRasterSurface
		Plan    engine.OriginalRasterFramePlan
		Want    engine.OriginalRasterFrameRows
	}
	if e = json.NewDecoder(z).Decode(&cases); e != nil {
		t.Fatal(e)
	}
	b := New()
	created := b.Handle([]byte(`{"op":"create"}`))
	if !created.OK {
		t.Fatal(created.Error)
	}
	before := b.JSON(`{"op":"snapshot","session":1}`)
	for i, c := range cases {
		raw, _ := json.Marshal(map[string]any{"op": "projectPresentation", "projection": map[string]any{"surface": c.Surface, "plan": c.Plan}})
		response := b.Handle(raw)
		if !response.OK {
			t.Fatal(i, response.Error)
		}
		encoded, _ := json.Marshal(response.Data)
		var data struct {
			Format  int
			Palette [32]uint8
			Rows    []byte
		}
		if e = json.Unmarshal(encoded, &data); e != nil {
			t.Fatal(e)
		}
		if data.Format != 1 || data.Palette != c.Want.Palette || len(data.Rows) > 100000 {
			t.Fatal(i, "invalid packed header/size")
		}
		at := 0
		read := func(n int) []byte {
			if at+n > len(data.Rows) {
				t.Fatal(i, "truncated row")
			}
			value := data.Rows[at : at+n]
			at += n
			return value
		}
		for y, want := range c.Want.Rows {
			header := read(7)
			n := int(header[5]) | int(header[6])<<8
			if !reflect.DeepEqual(header[:4], want.SpriteCHR[:]) || header[4] != want.Mask || n != len(want.Segments) {
				t.Fatal(i, y, "row header")
			}
			for j, s := range want.Segments {
				got := read(8)
				expected := []byte{uint8(s.X), s.Length, s.Page, s.Tile, s.Row, s.Column, s.Palette, s.Mask}
				if !reflect.DeepEqual(got, expected) {
					t.Fatal(i, y, j, got, expected)
				}
			}
		}
		if at != len(data.Rows) {
			t.Fatal(i, "unparsed row bytes")
		}
	}
	if len(cases) != 16 || before != b.JSON(`{"op":"snapshot","session":1}`) {
		t.Fatal("coverage or projection changed session")
	}
	for _, request := range []string{`{"op":"projectPresentation"}`, `{"op":"projectPresentation","projection":{}}`} {
		if r := b.Handle([]byte(request)); r.OK {
			t.Fatal("invalid projection accepted")
		}
	}
}
