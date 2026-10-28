package contra

// Failure inventory before provider code: reading rule slots/collision instead
// of displayed pixels; exposing pending writes or inactive display parameters;
// loading all CHR pages; returning mutable buffers; sprite priority/8-per-line,
// background clipping and raster differences; unknown HUD values carrying hidden
// data. Same final pixels and public HUD must form the same classifier input.
import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type visualTestCatalog struct {
	pages map[uint8][]uint8
	rgb   []uint8
	reads []uint8
}

func (c *visualTestCatalog) CHRPage(id uint8) ([]uint8, error) {
	c.reads = append(c.reads, id)
	p, ok := c.pages[id]
	if !ok {
		return nil, fmt.Errorf("missing page%d", id)
	}
	return p, nil
}
func (c *visualTestCatalog) RGB() ([]uint8, error) { return c.rgb, nil }
func visualIndexed(raw []uint8) []uint8 {
	out := make([]uint8, 4096)
	for tile := 0; tile < 64; tile++ {
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				out[((tile>>3)*8+y)*64+(tile&7)*8+x] = raw[tile*16+y]>>uint(7-x)&1 | (raw[tile*16+y+8]>>uint(7-x)&1)<<1
			}
		}
	}
	return out
}
func visualHex(t *testing.T, s string) []uint8 {
	t.Helper()
	b, e := hex.DecodeString(s)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestBotVisualFrameOriginalPixels(t *testing.T) {
	f, e := os.Open("../tests/testdata/background-original.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var d struct {
		RGB     string
		Pages   map[string]string
		Samples []struct {
			Frame                           int
			Control, Mask, ScrollX, ScrollY uint8
			Mapping                         []string
			Palette, OAM, ExpectedRGB       string
			Screens                         []struct{ Tiles, Attributes string }
		}
	}
	if e = json.NewDecoder(z).Decode(&d); e != nil {
		t.Fatal(e)
	}
	catalog := &visualTestCatalog{pages: map[uint8][]uint8{}, rgb: visualHex(t, d.RGB)}
	for name, raw := range d.Pages {
		id, e := strconv.Atoi(strings.TrimPrefix(name, "page-"))
		if e != nil {
			t.Fatal(e)
		}
		catalog.pages[uint8(id)] = visualIndexed(visualHex(t, raw))
	}
	for _, sample := range d.Samples {
		var p PresentedFrame
		p.Scroll = CollisionScroll{X: sample.ScrollX, Y: sample.ScrollY, NameTable: sample.Control}
		p.Display.AppliedMask = sample.Mask
		for i, name := range sample.Mapping {
			id, _ := strconv.Atoi(strings.TrimPrefix(name, "page-"))
			p.Patterns[i] = uint8(id)
		}
		copy(p.Palette[:], visualHex(t, sample.Palette))
		copy(p.OAM[:], visualHex(t, sample.OAM))
		for i, s := range sample.Screens {
			copy(p.Background[i*1024:], visualHex(t, s.Tiles))
			copy(p.Background[i*1024+960:], visualHex(t, s.Attributes))
		}
		catalog.reads = nil
		v, e := NewBotVisualFrame(p, BotPublicHUD{Seats: 2}, catalog, 0, uint64(sample.Frame))
		if e != nil {
			t.Fatal(sample.Frame, e)
		}
		want, e := base64.StdEncoding.DecodeString(sample.ExpectedRGB)
		if e != nil {
			t.Fatal(e)
		}
		got := v.Pixels()
		if len(got) != 256*240*4 {
			t.Fatal("invalid RGBA dimensions")
		}
		for i := 0; i < 61440; i++ {
			if !bytes.Equal(got[i*4:i*4+3], want[i*3:i*3+3]) || got[i*4+3] != 255 {
				t.Fatalf("frame%d pixel%d differs", sample.Frame, i)
			}
		}
		unique := map[uint8]bool{}
		for _, id := range p.Patterns {
			unique[id] = true
		}
		if len(catalog.reads) != len(unique) {
			t.Fatal("did not load exactly the current unique mappings", catalog.reads)
		}
	}
	if len(d.Samples) != 13 {
		t.Fatal("incomplete original pixel oracle")
	}
}
func TestBotVisualFrameOwnershipAndNoninterference(t *testing.T) {
	c := &visualTestCatalog{pages: map[uint8][]uint8{0: make([]uint8, 4096)}, rgb: make([]uint8, 192)}
	for i := range c.rgb {
		c.rgb[i] = uint8(i)
	}
	var p PresentedFrame
	for i := 0; i < 256; i += 4 {
		p.OAM[i] = 244
	}
	p.Display.AppliedMask = 30
	h := BotPublicHUD{Seats: 2, Stage: BotPublicNumber{Value: 7}, Players: [2]BotPublicPlayer{{Occupied: true, Weapon: BotPublicNumber{Value: 4}}}}
	a, e := NewBotVisualFrame(p, h, c, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	q := p
	q.Writes = []BackgroundWrite{{Address: 0x2000, Step: 1, Values: []uint8{255}}}
	q.Display.Mask = 255
	q.Display.Busy = 99
	q.Display.SplitScroll = 111
	q.Display.RasterControl = 177
	q.Display.RasterMask = 0
	for i := 1024; i < 2048; i++ {
		q.Background[i] = 100
	}
	q.OAM[1] = 99
	b, e := NewBotVisualFrame(q, h, c, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(a.Pixels(), b.Pixels()) || a.HUD() != b.HUD() {
		t.Fatal("non-rendered state leaked")
	}
	if a.HUD().Stage.Value != 0 || a.HUD().Players[0].Weapon.Value != 0 {
		t.Fatal("unknown HUD values leaked")
	}
	original := a.Pixels()
	returned := a.Pixels()
	returned[0] ^= 255
	c.rgb[0] ^= 255
	c.pages[0][0] = 3
	p.Background[0] = 7
	if !bytes.Equal(original, a.Pixels()) {
		t.Fatal("frame aliases mutable caller or returned buffers")
	}
	other, e := a.ForActor(1)
	if e != nil || other.Actor() != 1 || other.Tick() != 1 || !bytes.Equal(other.Pixels(), a.Pixels()) {
		t.Fatal("seat reuse changed visible pixels")
	}
	if _, e = a.ForActor(2); e == nil {
		t.Fatal("invalid seat accepted")
	}
}
func TestBotVisualFrameRejectsInvalidInputs(t *testing.T) {
	c := &visualTestCatalog{pages: map[uint8][]uint8{0: make([]uint8, 4096)}, rgb: make([]uint8, 192)}
	p := PresentedFrame{}
	for _, actor := range []int{-1, 2} {
		if _, e := NewBotVisualFrame(p, BotPublicHUD{Seats: 2}, c, actor, 0); e == nil {
			t.Fatal("invalid seat")
		}
	}
	if _, e := NewBotVisualFrame(p, BotPublicHUD{Seats: 1}, nil, 0, 0); e == nil {
		t.Fatal("nil directory")
	}
	c.pages[0][4095] = 4
	if _, e := NewBotVisualFrame(p, BotPublicHUD{Seats: 1}, c, 0, 0); e == nil {
		t.Fatal("invalid indexed pixel")
	}
	c.pages[0][4095] = 0
	p.Scroll.Y = 240
	if _, e := NewBotVisualFrame(p, BotPublicHUD{Seats: 1}, c, 0, 0); e == nil {
		t.Fatal("invalid vertical origin")
	}
}
func TestBotVisualFrameContainsNoRuleTypes(t *testing.T) {
	forbidden := map[reflect.Type]bool{reflect.TypeOf(World{}): true, reflect.TypeOf(Player{}): true, reflect.TypeOf(Enemy{}): true, reflect.TypeOf(CollisionWindow{}): true}
	var visit func(reflect.Type)
	visit = func(tp reflect.Type) {
		if forbidden[tp] {
			t.Fatal("hidden rule owner in visual frame", tp)
		}
		switch tp.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			visit(tp.Elem())
		case reflect.Struct:
			for i := 0; i < tp.NumField(); i++ {
				visit(tp.Field(i).Type)
			}
		}
	}
	visit(reflect.TypeOf(BotVisualFrame{}))
}

func TestBotVisualFrameOriginalRasterPixels(t *testing.T) {
	f, e := os.Open("testdata/bot-visual-frame-raster.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var d struct {
		SourceIsOriginalExecution bool
		RGB                       []uint8
		Pages                     map[uint8][]uint8
		Samples                   []struct {
			Frame        int
			View         PresentedFrame
			ExpectedRGBA []uint8
		}
	}
	if e = json.NewDecoder(z).Decode(&d); e != nil {
		t.Fatal(e)
	}
	if !d.SourceIsOriginalExecution || len(d.Samples) != 7 {
		t.Fatal("incomplete original raster fixture")
	}
	c := &visualTestCatalog{pages: d.Pages, rgb: d.RGB}
	for _, sample := range d.Samples {
		v, e := NewBotVisualFrame(sample.View, BotPublicHUD{Seats: 2}, c, 0, uint64(sample.Frame))
		if e != nil {
			t.Fatal(sample.Frame, e)
		}
		if !bytes.Equal(v.Pixels(), sample.ExpectedRGBA) {
			t.Fatal(sample.Frame, "original raster pixels differ")
		}
	}
}
