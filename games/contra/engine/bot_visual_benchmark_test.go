package contra

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Renderer workload uses static original art and a public display frame, never
// a rules snapshot. Include two actors sharing the same immutable rendered view.
func BenchmarkBotVisualFrameTwoSeats(b *testing.B) {
	f, e := os.Open("../tests/testdata/background-original.json.gz")
	if e != nil {
		b.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		b.Fatal(e)
	}
	defer z.Close()
	var d struct {
		RGB     string
		Pages   map[string]string
		Samples []struct {
			Control, Mask, ScrollX, ScrollY uint8
			Mapping                         []string
			Palette, OAM                    string
			Screens                         []struct{ Tiles, Attributes string }
		}
	}
	if e = json.NewDecoder(z).Decode(&d); e != nil {
		b.Fatal(e)
	}
	decode := func(s string) []byte {
		raw, e := hex.DecodeString(s)
		if e != nil {
			b.Fatal(e)
		}
		return raw
	}
	c := &visualTestCatalog{pages: map[uint8][]uint8{}, rgb: decode(d.RGB)}
	for name, raw := range d.Pages {
		id, e := strconv.Atoi(strings.TrimPrefix(name, "page-"))
		if e != nil {
			b.Fatal(e)
		}
		c.pages[uint8(id)] = visualIndexed(decode(raw))
	}
	s := d.Samples[len(d.Samples)-1]
	p := PresentedFrame{Scroll: CollisionScroll{X: s.ScrollX, Y: s.ScrollY, NameTable: s.Control}, Display: DisplayState{AppliedMask: s.Mask}}
	for i, name := range s.Mapping {
		id, _ := strconv.Atoi(strings.TrimPrefix(name, "page-"))
		p.Patterns[i] = uint8(id)
	}
	copy(p.Palette[:], decode(s.Palette))
	copy(p.OAM[:], decode(s.OAM))
	for i, v := range s.Screens {
		copy(p.Background[i*1024:], decode(v.Tiles))
		copy(p.Background[i*1024+960:], decode(v.Attributes))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.reads = c.reads[:0]
		frame, e := NewBotVisualFrame(p, BotPublicHUD{Seats: 2}, c, 0, uint64(i))
		if e != nil {
			b.Fatal(e)
		}
		other, e := frame.ForActor(1)
		if e != nil {
			b.Fatal(e)
		}
		if len(frame.Pixels()) != 256*240*4 || len(other.Pixels()) != 256*240*4 {
			b.Fatal("incomplete visual input")
		}
	}
}
