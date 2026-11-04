package presentation

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

type originalSnapshot struct {
	Actor, DMA, Sprites, Layers string
	DMACursor                   uint16 `json:"dma_cursor"`
	Count                       uint16
	SpriteCursor                uint16 `json:"sprite_cursor"`
	Flags                       uint8
}
type originalCall struct {
	GraphicsSet uint32 `json:"graphics_set"`
	Parameter   uint16
	Before      originalSnapshot
	After       originalSnapshot
}

func dataBytes(t *testing.T, encoded string, size int) []byte {
	t.Helper()
	b, err := hex.DecodeString(encoded)
	if err != nil || size >= 0 && len(b) != size {
		t.Fatal("invalid original bytes", err, len(b), size)
	}
	return b
}

func loadCatalog(t *testing.T) Catalog {
	t.Helper()
	f, err := os.Open("../data/world-sprites.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := os.ReadFile("../data/world-sprites.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile("../data/world-sprites-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pass     bool
		Baseline bool   `json:"frames_match_original_core"`
		RawSHA   string `json:"catalog_sha256"`
		ZipSHA   string `json:"catalog_gzip_sha256"`
	}
	if err := json.Unmarshal(mb, &manifest); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(compressed)
	if !manifest.Pass || !manifest.Baseline || hex.EncodeToString(h[:]) != manifest.RawSHA || hex.EncodeToString(zh[:]) != manifest.ZipSHA {
		t.Fatal("unbound presentation catalog")
	}
	catalog, err := DecodeCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func actorFromBytes(t *testing.T, encoded string, owner uint16) Actor {
	t.Helper()
	a := dataBytes(t, encoded, 256)
	w := func(i int) uint16 { return binary.BigEndian.Uint16(a[i:]) }
	l := func(i int) uint32 { return binary.BigEndian.Uint32(a[i:]) }
	return Actor{ObjectID: owner, GraphicsSet: l(0x1c), TileBank: l(0x20), Parameter: w(0x3c), ScreenX: w(8), ScreenY: w(12),
		OffsetX: int16(w(0x28)), OffsetY: int16(w(0x2a)), Attributes: w(0x24), Depth: w(0x3e), Layer: a[0x26], SkipTiles: a[0x27],
		Cache: [4]uint32{l(0x2c), l(0x30), l(0x34), l(0x38)}}
}

func attrs(p Part) uint16 {
	v := p.TileSlot | uint16(p.Palette)<<13
	if p.FlipX {
		v |= 0x800
	}
	if p.FlipY {
		v |= 0x1000
	}
	if p.HighPriority {
		v |= 0x8000
	}
	return v
}

func encodeSubmission(t *testing.T, state FrameState, actor Actor, before originalSnapshot) originalSnapshot {
	t.Helper()
	a := dataBytes(t, before.Actor, 256)
	dma := dataBytes(t, before.DMA, 0xc00)
	sprites := dataBytes(t, before.Sprites, 0x1000)
	layers := dataBytes(t, before.Layers, 256)
	for i, stamp := range actor.Cache {
		binary.BigEndian.PutUint32(a[0x2c+i*4:], stamp)
	}
	result := before
	for _, transfer := range state.Transfers {
		source := transfer.Bank + (transfer.Stamp>>16)*32
		length := uint16(uint8(transfer.Stamp)) << 4
		destination := transfer.Destination * 32
		command := uint32(destination&0x3fff)<<16 | uint32(destination>>14) | 0x40000080
		words := [...]uint16{0x9300 | length&255, 0x9400 | length>>8, 0x9500 | uint16(source>>1)&255,
			0x9600 | uint16(source>>9)&255, 0x9700 | uint16(source>>17)&127, 0x8f02, uint16(command >> 16), uint16(command)}
		index := int(result.DMACursor) - 0xd400
		for j, word := range words {
			binary.BigEndian.PutUint16(dma[index+j*2:], word)
		}
		result.DMACursor += 16
	}
	if len(state.Parts) != 0 {
		layer := int(actor.Layer&0xfc) + 2
		tail := binary.BigEndian.Uint16(layers[layer:])
		sprites[int(tail)-5-0xe000] = uint8(before.Count)
		for _, p := range state.Parts {
			index := int(result.SpriteCursor) - 0xe000
			size := (p.WidthTiles-1)<<2 | (p.HeightTiles - 1) | uint8(p.SourceXor>>7)&0xf0
			words := [...]uint16{p.Y, uint16(size)<<8 | uint16(uint8(p.Index+1)), attrs(p), p.X}
			for j, word := range words {
				binary.BigEndian.PutUint16(sprites[index+j*2:], word)
			}
			result.SpriteCursor += 8
		}
		binary.BigEndian.PutUint16(layers[layer:], result.SpriteCursor)
	}
	result.Actor, result.DMA, result.Sprites, result.Layers = hex.EncodeToString(a), hex.EncodeToString(dma), hex.EncodeToString(sprites), hex.EncodeToString(layers)
	result.Count, result.Flags = state.Count, state.Flags
	return result
}

func visitArray(t *testing.T, path, field string, visit func(*json.Decoder)) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	d := json.NewDecoder(z)
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		t.Fatal("invalid original fixture", err)
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			t.Fatal(err)
		}
		if key == field {
			if token, err := d.Token(); err != nil || token != json.Delim('[') {
				t.Fatal("invalid fixture array", err)
			}
			for d.More() {
				visit(d)
			}
			if _, err := d.Token(); err != nil {
				t.Fatal(err)
			}
		} else {
			var skip json.RawMessage
			if err := d.Decode(&skip); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// All original fixtures and these tests precede the reusable implementation.
func TestPresentationOriginalCPUSubmissions(t *testing.T) {
	catalog := loadCatalog(t)
	beforeLibrary, _ := json.Marshal(catalog)
	counts := []int{}
	for _, path := range []string{"../data/sprite-oracle.json.gz", "../data/world-sprites-oracle.json.gz"} {
		count := 0
		visitArray(t, path, "calls", func(d *json.Decoder) {
			var call originalCall
			if err := d.Decode(&call); err != nil {
				t.Fatal(err)
			}
			actor := actorFromBytes(t, call.Before.Actor, 0x8400)
			state := FrameState{Count: call.Before.Count, Flags: call.Before.Flags}
			got, next, err := ComposeActor(state, actor, catalog)
			if err != nil || encodeSubmission(t, got, next, call.Before) != call.After {
				t.Fatalf("original sprite call %d %x/%d differs: %v", count, call.GraphicsSet, call.Parameter, err)
			}
			count++
		})
		counts = append(counts, count)
	}
	afterLibrary, _ := json.Marshal(catalog)
	if counts[0] != 9218 || counts[1] != 3020 || !bytes.Equal(beforeLibrary, afterLibrary) {
		t.Fatal("incomplete original submission coverage or modified assets", counts)
	}
	t.Logf("Original complete submissions: characters=%d world=%d", counts[0], counts[1])
}

type naturalFrame struct {
	Scenario     string
	Frame        int
	BeforeVRAM   string `json:"before_vram"`
	UploadedVRAM string `json:"uploaded_vram"`
	InitialCount uint16 `json:"initial_count"`
	InitialFlags uint8  `json:"initial_flags"`
	Reserved     string `json:"reserved_sprites"`
	Draws        []struct {
		Owner       uint16
		Actor       string
		AfterActor  string `json:"after_actor"`
		Count       uint16
		AfterCount  uint16 `json:"after_count"`
		Flags       uint8
		AfterFlags  uint8 `json:"after_flags"`
		Cursor      uint16
		AfterCursor uint16 `json:"after_cursor"`
	}
	Palette     [64]uint16
	Registers   [32]uint8
	Width       int
	Height      int
	PixelFormat int    `json:"pixel_format"`
	Video       string `json:"video_next"`
	VideoFrame  int    `json:"video_frame"`
}

func TestPresentationNaturalFramesAndPixels(t *testing.T) {
	base := loadCatalog(t)
	frames, pixels := 0, 0
	owners := map[uint16]int{}
	visitArray(t, "../data/world-sprites-oracle.json.gz", "natural", func(d *json.Decoder) {
		var frame naturalFrame
		if err := d.Decode(&frame); err != nil {
			t.Fatal(err)
		}
		if frame.VideoFrame != frame.Frame+1 || frame.Width != 320 || frame.Height != 224 || frame.PixelFormat != 2 || frame.Registers[12]&0x0e != 0 {
			t.Fatal("unbound natural presentation timing/mode")
		}
		catalog := base
		catalog.Tiles = append([]Tile(nil), base.Tiles...)
		initial := dataBytes(t, frame.BeforeVRAM, 65536)
		state := FrameState{Count: frame.InitialCount, Flags: frame.InitialFlags}
		for slot := 0; slot < 2048; slot++ {
			var tile Tile
			copy(tile[:], initial[slot*32:(slot+1)*32])
			id := uint32(len(catalog.Tiles))
			catalog.Tiles = append(catalog.Tiles, tile)
			if err := state.Slots.Set(uint16(slot), id); err != nil {
				t.Fatal(err)
			}
		}
		caches := map[uint16][4]uint32{}
		for i, draw := range frame.Draws {
			actor := actorFromBytes(t, draw.Actor, draw.Owner)
			if cache, exists := caches[draw.Owner]; exists {
				actor.Cache = cache
			}
			if state.Count != draw.Count || state.Flags != draw.Flags {
				t.Fatal("natural composition input counter differs", frame.Frame, i)
			}
			var err error
			state, actor, err = ComposeActor(state, actor, catalog)
			if err != nil || actor.Cache != actorFromBytes(t, draw.AfterActor, draw.Owner).Cache || state.Count != draw.AfterCount || state.Flags != draw.AfterFlags {
				t.Fatalf("natural frame %d draw %d differs: %v", frame.Frame, i, err)
			}
			caches[draw.Owner] = actor.Cache
		}
		uploaded := dataBytes(t, frame.UploadedVRAM, 65536)
		for _, transfer := range state.Transfers {
			for i, tile := range transfer.Tiles {
				slot := (int(transfer.Destination) + i) & 2047
				gotTile, ok := state.Slots.Lookup(uint16(slot))
				if !ok {
					t.Fatal("missing transferred slot")
				}
				_ = tile // Later transfers may overwrite this slot in the same frame.
				if !bytes.Equal(catalog.Tiles[gotTile][:], uploaded[slot*32:(slot+1)*32]) {
					t.Fatal("semantic global tile transfer differs from original VRAM", frame.Frame, slot)
				}
			}
		}
		parts := OrderedParts(state)
		sat := int(frame.Registers[5]&0x7e) << 9
		index, partIndex := 0, 0
		var visited [80]bool
		for n := 0; n < 80; n++ {
			if index >= 80 || visited[index] {
				t.Fatal("invalid original sprite chain")
			}
			visited[index] = true
			entry := uploaded[sat+index*8:]
			if index >= int(frame.InitialCount) {
				if partIndex >= len(parts) {
					t.Fatal("original chain contains unmatched runtime part")
				}
				p := parts[partIndex]
				if p.Index != uint8(index) || p.X != binary.BigEndian.Uint16(entry[6:]) || p.Y != binary.BigEndian.Uint16(entry) || attrs(p) != binary.BigEndian.Uint16(entry[4:]) || (p.WidthTiles-1)<<2|(p.HeightTiles-1) != entry[2]&15 {
					t.Fatal("layer-ordered semantic sprite differs from final original chain", frame.Frame, partIndex)
				}
				partIndex++
			}
			if entry[3]&127 == 0 {
				break
			}
			index = int(entry[3] & 127)
		}
		if partIndex != len(parts) {
			t.Fatal("runtime layer ordering lost parts")
		}
		raster, err := RasterizeSprites(state, catalog, frame.Palette, frame.Width, frame.Height, frame.Registers[0]&4 == 0)
		if err != nil {
			t.Fatal(err)
		}
		video := dataBytes(t, frame.Video, frame.Width*frame.Height*2)
		for i, opaque := range raster.Opaque {
			if opaque && raster.HighPriority[i] {
				if raster.Colors[i] != binary.LittleEndian.Uint16(video[i*2:]) {
					t.Fatal("independent natural sprite pixel differs", frame.Frame, i)
				}
				pixels++
				owners[raster.Owners[i]]++
			}
		}
		frames++
	})
	if frames != 83 || pixels == 0 || len(owners) < 3 {
		t.Fatal("empty or incomplete natural pixel coverage", frames, pixels, owners)
	}
	t.Logf("Natural presentation: frames=%d opaque high-priority pixels=%d owners=%v", frames, pixels, owners)
}

func TestPresentationPaletteSelectors(t *testing.T) {
	catalog := loadCatalog(t)
	count := 0
	visitArray(t, "../data/character-assets-oracle.json.gz", "bindings", func(d *json.Decoder) {
		var c struct {
			CharacterID    int `json:"character_id"`
			Selector, Slot int
			Before, After  string
		}
		if err := d.Decode(&c); err != nil {
			t.Fatal(err)
		}
		before, after := dataBytes(t, c.Before, 128), dataBytes(t, c.After, 128)
		var colors [64]uint16
		for i := range colors {
			colors[i] = binary.BigEndian.Uint16(before[i*2:])
		}
		got, err := ApplyPalette(colors, uint8(c.CharacterID), uint8(c.Selector), uint8(c.Slot), catalog)
		for i, value := range got {
			binary.BigEndian.PutUint16(before[i*2:], value)
		}
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("palette selector differs from full original call", c.CharacterID, c.Selector, c.Slot, err)
		}
		count++
	})
	if count != 528 {
		t.Fatal("missing original palette binding coverage")
	}
}

func TestPresentationErrorsAreAtomic(t *testing.T) {
	catalog := loadCatalog(t)
	actor := Actor{GraphicsSet: 0xd36ba, TileBank: 0x200000, Parameter: 2, ScreenX: 384, ScreenY: 320, Depth: 256}
	base := FrameState{Count: 2, Flags: 128, Parts: []Part{{Owner: 123, WidthTiles: 1, HeightTiles: 1}}}
	before, _ := json.Marshal(base)
	missing := actor
	missing.GraphicsSet = 0xffffffff
	got, next, err := ComposeActor(base, missing, catalog)
	if err == nil || !reflect.DeepEqual(got, base) || next != missing {
		t.Fatal("missing frame caused partial presentation update")
	}
	missing.ScreenX = 0
	got, next, err = ComposeActor(base, missing, catalog)
	if err != nil || !reflect.DeepEqual(got, base) || next != missing {
		t.Fatal("culled actor attempted to resolve an unavailable frame")
	}
	wrongBank := actor
	wrongBank.TileBank++
	got, next, err = ComposeActor(base, wrongBank, catalog)
	if err == nil || !reflect.DeepEqual(got, base) || next != wrongBank {
		t.Fatal("unknown tile source caused partial presentation update")
	}
	after, _ := json.Marshal(base)
	if !bytes.Equal(before, after) {
		t.Fatal("failed composition mutated caller backing slices")
	}
	if _, err := DecodeCatalog([]byte(`{"schema":1,"tile_bytes":"00","frames":[]}`)); err == nil {
		t.Fatal("invalid tile bank accepted")
	}
	if _, err := RasterizeSprites(FrameState{Parts: []Part{{Index: 1, WidthTiles: 1, HeightTiles: 1, X: 128, Y: 128}}}, catalog, [64]uint16{}, 320, 224, false); err == nil {
		t.Fatal("missing prior slot silently painted transparent")
	}
	var colors [64]uint16
	colors[1] = 0xabcd
	for _, selector := range [][3]uint8{{11, 0, 0}, {0, 12, 0}, {0, 0, 4}} {
		got, err := ApplyPalette(colors, selector[0], selector[1], selector[2], catalog)
		if err == nil || got != colors {
			t.Fatal("invalid palette input changed caller colors")
		}
	}
}

func TestPresentationOpaqueBlackAndCoordinateWrap(t *testing.T) {
	var tile Tile
	for i := range tile {
		tile[i] = 0x11
	}
	catalog := Catalog{Tiles: []Tile{tile}}
	state := FrameState{Count: 2, Flags: 128, Parts: []Part{{Owner: 7, Index: 1, WidthTiles: 1, HeightTiles: 1, X: 640, Y: 640, HighPriority: true}}}
	if err := state.Slots.Set(0, 0); err != nil {
		t.Fatal(err)
	}
	raster, err := RasterizeSprites(state, catalog, [64]uint16{}, 320, 224, false)
	if err != nil || !raster.Opaque[0] || raster.Colors[0] != 0 || raster.Owners[0] != 7 || raster.RGBA().RGBAAt(0, 0).A != 255 {
		t.Fatal("normal9-bit sprite coordinates or opaque nonzero black index changed", err)
	}
	if _, ok := state.Slots.Lookup(2048); ok {
		t.Fatal("out-of-range logical tile slot accepted")
	}
}

// The original composer may physically write a terminal81st item, but the
// normal H40 list parser stops before index80. Later layer groups do not resume.
func TestPresentationHardwareListBoundary(t *testing.T) {
	var tile Tile
	for i := range tile {
		tile[i] = 0x11
	}
	catalog := Catalog{Tiles: []Tile{tile}}
	state := FrameState{Count: 82, Flags: 1, Parts: []Part{
		{Owner: 1, Index: 2, Layer: 0, WidthTiles: 1, HeightTiles: 1, X: 128, Y: 128},
		{Owner: 2, Index: 80, Layer: 4, WidthTiles: 1, HeightTiles: 1, X: 136, Y: 128},
		{Owner: 3, Index: 3, Layer: 8, WidthTiles: 1, HeightTiles: 1, X: 144, Y: 128},
	}}
	if err := state.Slots.Set(0, 0); err != nil {
		t.Fatal(err)
	}
	raster, err := RasterizeSprites(state, catalog, [64]uint16{}, 320, 224, false)
	if err != nil || !raster.Opaque[0] || raster.Opaque[8] || raster.Opaque[16] {
		t.Fatal("sprite list did not stop at the original index80 boundary", err)
	}
}
