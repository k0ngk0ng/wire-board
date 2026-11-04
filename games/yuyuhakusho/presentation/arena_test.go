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

type arenaOriginalLine struct {
	Line           int
	Draw           bool
	Registers      []uint8
	CRAM           []uint16
	VSRAM, HScroll string
}

type arenaOriginalFrame struct {
	Frame, Width, Height int
	Registers            []uint8
	ActiveVRAM           string              `json:"active_vram"`
	Lines                []arenaOriginalLine `json:"active_lines"`
	Video                string              `json:"video_next"`
	Next                 int                 `json:"video_next_frame"`
	Changed              bool                `json:"active_background_vram_changed"`
}

func arenaReadGzip(t *testing.T, name string) ([]byte, []byte) {
	t.Helper()
	compressed, err := os.ReadFile("../data/" + name + ".json.gz")
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
	return raw, compressed
}

func loadArena(t *testing.T) (ArenaCatalog, []byte) {
	t.Helper()
	raw, compressed := arenaReadGzip(t, "arena-assets-default")
	h := sha256.Sum256(compressed)
	if hex.EncodeToString(h[:]) != "8b3626b608d90ac990176370f665b42499dc404546f955aafd0beeee5cd35542" {
		t.Fatal("unbound indexed arena")
	}
	catalog, err := DecodeArenaCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	return catalog, raw
}

// This is only a reference adapter. The production API never accepts VRAM;
// dynamic window cells and tiles must be supplied by the independent HUD.
func arenaWindowFromOriginal(t *testing.T, vram []byte, base int) IndexedPlane {
	t.Helper()
	p := IndexedPlane{Columns: 64, Rows: 32}
	seen := make(map[string]uint16)
	for i := 0; i < 64*32; i++ {
		attribute := binary.BigEndian.Uint16(vram[base+i*2:])
		start := int(attribute&2047) * 32
		packed := string(vram[start : start+32])
		id, ok := seen[packed]
		if !ok {
			id = uint16(len(p.Tiles))
			seen[packed] = id
			var tile Tile
			copy(tile[:], packed)
			p.Tiles = append(p.Tiles, tile)
		}
		p.Cells = append(p.Cells, ArenaCell{Tile: id, Flags: attribute & 0xf800})
	}
	return p
}

func arenaObservedFrame(t *testing.T, f arenaOriginalFrame) ArenaFrame {
	t.Helper()
	if f.Width != 320 || f.Height != 224 || len(f.Registers) != 32 || len(f.Lines) != 224 || f.Next != f.Frame+1 || f.Changed {
		t.Fatal("unsupported original frame")
	}
	vram := dataBytes(t, f.ActiveVRAM, 65536)
	window := arenaWindowFromOriginal(t, vram, int(f.Registers[3]&0x3c)<<10)
	out := ArenaFrame{Width: f.Width, Height: f.Height, Window: &window}
	for y, line := range f.Lines {
		r := line.Registers
		if line.Line != y || len(r) != 32 || len(line.CRAM) != 64 || r[12]&0x0e != 0 || r[12]&1 == 0 || r[11]&4 != 0 || r[16] != 1 || r[17] != 0 || r[3] != f.Registers[3] {
			t.Fatal("unbound original row")
		}
		hs, vs := dataBytes(t, line.HScroll, 4), dataBytes(t, line.VSRAM, 4)
		row := ArenaRow{Horizontal: [2]uint16{binary.BigEndian.Uint16(hs), binary.BigEndian.Uint16(hs[2:])}, Vertical: [2]uint16{binary.BigEndian.Uint16(vs) & 1023, binary.BigEndian.Uint16(vs[2:]) & 1023}, Backdrop: r[7] & 63, Visible: line.Draw, Limited: r[0]&4 == 0, Window: (r[18]&0x80 != 0) == (y >= int(r[18]&31)*8)}
		copy(row.Palette[:], line.CRAM)
		out.Rows = append(out.Rows, row)
	}
	return out
}

// Original sprite plane adapter, independent of the runtime composition. It
// preserves SAT order and complete nonzero pixel opacity, including black.
func arenaOriginalSprites(t *testing.T, f arenaOriginalFrame, rows []ArenaRow) Raster {
	t.Helper()
	vram := dataBytes(t, f.ActiveVRAM, 65536)
	n := f.Width * f.Height
	out := Raster{Width: f.Width, Height: f.Height, Colors: make([]uint16, n), Opaque: make([]bool, n), HighPriority: make([]bool, n), Owners: make([]uint16, n)}
	base := int(f.Registers[5]&0x7e) << 9
	index := 0
	seen := [80]bool{}
	for count := 0; count < 80; count++ {
		if index >= 80 || seen[index] {
			t.Fatal("invalid original sprite chain")
		}
		seen[index] = true
		e := vram[base+index*8:]
		x0, y0 := int(binary.BigEndian.Uint16(e[6:])&511)-128, int(binary.BigEndian.Uint16(e)&1023)-128
		a := binary.BigEndian.Uint16(e[4:])
		w, h := int((e[2]>>2)&3)+1, int(e[2]&3)+1
		for y := 0; y < h*8; y++ {
			for x := 0; x < w*8; x++ {
				px, py := x0+x, y0+y
				if px < 0 || px >= f.Width || py < 0 || py >= f.Height {
					continue
				}
				pos := py*f.Width + px
				if out.Opaque[pos] {
					continue
				}
				u, v := x, y
				if a&0x800 != 0 {
					u = w*8 - 1 - u
				}
				if a&0x1000 != 0 {
					v = h*8 - 1 - v
				}
				tile := (int(a&2047) + (u/8)*h + v/8) & 2047
				b := vram[tile*32+(v&7)*4+(u&7)/2]
				value := b >> uint(4*(1-u&1)) & 15
				if value == 0 {
					continue
				}
				out.Opaque[pos], out.HighPriority[pos], out.Owners[pos] = true, a&0x8000 != 0, uint16(index+1)
				out.Colors[pos] = color565(rows[py].Palette[int(a>>9&0x30)|int(value)], rows[py].Limited)
			}
		}
		link := int(e[3] & 127)
		if link == 0 {
			return out
		}
		index = link
	}
	t.Fatal("unterminated original sprite chain")
	return Raster{}
}

func TestArenaIndexedAgainstOriginalVideo(t *testing.T) {
	catalog, _ := loadArena(t)
	raw, compressed := arenaReadGzip(t, "arena-assets-reference")
	mb, err := os.ReadFile("../data/arena-assets-reference-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pass     bool
		Baseline bool   `json:"frames_match_original_core"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
		Records  int
	}
	if err := json.Unmarshal(mb, &manifest); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(compressed)
	if !manifest.Pass || !manifest.Baseline || manifest.SHA != hex.EncodeToString(h[:]) || manifest.Gzip != hex.EncodeToString(zh[:]) {
		t.Fatal("unbound original arena evidence")
	}
	var source struct{ Frames []arenaOriginalFrame }
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	if len(source.Frames) != 20 || manifest.Records != 20 {
		t.Fatal("incomplete original frames")
	}
	backgroundPixels, spriteExcluded, combinedPixels := 0, 0, 0
	for _, f := range source.Frames {
		input := arenaObservedFrame(t, f)
		bg, err := RasterizeArena(catalog, input)
		if err != nil {
			t.Fatal(err)
		}
		sprites := arenaOriginalSprites(t, f, input.Rows)
		combined, err := CompositeArenaSprites(bg, sprites)
		if err != nil {
			t.Fatal(err)
		}
		video := dataBytes(t, f.Video, f.Width*f.Height*2)
		for i, c := range bg.Colors {
			want := binary.LittleEndian.Uint16(video[i*2:])
			if !sprites.Opaque[i] {
				backgroundPixels++
				if c != want {
					t.Fatalf("background frame%d x%d y%d got%04x want%04x", f.Frame, i%f.Width, i/f.Width, c, want)
				}
			} else {
				spriteExcluded++
			}
			combinedPixels++
			if combined.Colors[i] != want {
				t.Fatalf("combined frame%d x%d y%d got%04x want%04x", f.Frame, i%f.Width, i/f.Width, combined.Colors[i], want)
			}
		}
	}
	if backgroundPixels != 1354388 || spriteExcluded != 79212 || combinedPixels != 1433600 {
		t.Fatal("pixel coverage changed", backgroundPixels, spriteExcluded, combinedPixels)
	}
	t.Logf("20 original frames: %d background pixels, %d sprite-covered pixels, %d complete pixels, zero differences", backgroundPixels, spriteExcluded, combinedPixels)
}

func tinyArena() ArenaCatalog {
	var tile Tile
	for i := range tile {
		tile[i] = 0x11
	}
	p := IndexedPlane{Columns: 64, Rows: 32, Tiles: []Tile{tile}, Cells: make([]ArenaCell, 64*32)}
	return ArenaCatalog{Planes: [2]IndexedPlane{p, p}}
}

func TestArenaPriorityOpaqueBlackAndBlankRows(t *testing.T) {
	// This truth table is the normal-mode make_lut_bg/make_lut_bgobj rule in
	// the fixed original reference core, separate from runtime implementation.
	for mask := 0; mask < 128; mask++ {
		a, b, s := mask&1 != 0, mask&2 != 0, mask&4 != 0
		ap, bp, sp, draw := mask&8 != 0, mask&16 != 0, mask&32 != 0, mask&64 != 0
		catalog := tinyArena()
		for p, on := range []bool{a, b} {
			catalog.Planes[p].Tiles = append([]Tile(nil), catalog.Planes[p].Tiles...)
			catalog.Planes[p].Cells = append([]ArenaCell(nil), catalog.Planes[p].Cells...)
			if !on {
				catalog.Planes[p].Tiles[0] = Tile{}
			}
		}
		if ap {
			catalog.Planes[0].Cells[0].Flags = 0x8000
		}
		if bp {
			catalog.Planes[1].Cells[0].Flags = 0x8000
		}
		catalog.Planes[1].Cells[0].Flags |= 0x2000
		row := ArenaRow{Visible: draw, Backdrop: 2}
		row.Palette[1], row.Palette[17], row.Palette[2] = 0, 7, 56
		bg, err := RasterizeArena(catalog, ArenaFrame{Width: 1, Height: 1, Rows: []ArenaRow{row}})
		if err != nil {
			t.Fatal(err)
		}
		ss := Raster{Width: 1, Height: 1, Colors: []uint16{0xfeed}, Opaque: []bool{s}, HighPriority: []bool{sp}, Owners: []uint16{0x8400}}
		out, err := CompositeArenaSprites(bg, ss)
		if err != nil {
			t.Fatal(err)
		}
		want, high, opaque := color565(56, false), false, false
		if draw {
			if b {
				want, high, opaque = color565(7, false), bp, true
			}
			if a && (!b || ap || !bp) {
				want, high, opaque = 0, ap, true
			}
			if s && (sp || !high || !opaque) {
				want = 0xfeed
			}
		}
		if out.Colors[0] != want {
			t.Fatalf("priority mask%02x got%04x want%04x", mask, out.Colors[0], want)
		}
		if draw && a && (!b || ap || !bp) && (!s || !sp && ap) && !bg.Opaque[0] {
			t.Fatal("black indexed pixel lost opacity")
		}
	}
}

func TestArenaRejectsInvalidDataWithoutMutation(t *testing.T) {
	catalog, raw := loadArena(t)
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(map[string]any){
		func(v map[string]any) { v["schema"] = 2 }, func(v map[string]any) { v["rom_sha256"] = "wrong" }, func(v map[string]any) { v["arena_offset"] = 2 },
		func(v map[string]any) { v["packed_4bpp_tiles"] = []string{"00"} }, func(v map[string]any) { v["initial_scene_palette_0"] = []int{0} },
		func(v map[string]any) { v["planes"].([]any)[0].(map[string]any)["columns"] = 32 },
		func(v map[string]any) {
			v["planes"].([]any)[0].(map[string]any)["cells"].([]any)[0].(map[string]any)["tile"] = 65535
		},
		func(v map[string]any) {
			v["planes"].([]any)[0].(map[string]any)["cells"].([]any)[0].(map[string]any)["flags"] = 1
		},
	} {
		var bad map[string]any
		if err := json.Unmarshal(raw, &bad); err != nil {
			t.Fatal(err)
		}
		mutate(bad)
		encoded, _ := json.Marshal(bad)
		if got, err := DecodeArenaCatalog(encoded); err == nil || !reflect.DeepEqual(got, ArenaCatalog{}) {
			t.Fatal("corrupt catalog accepted")
		}
	}
	input := ArenaFrame{Width: 1, Height: 1, Rows: []ArenaRow{{Visible: true}}}
	before, _ := json.Marshal(catalog)
	for _, bad := range []ArenaFrame{{Width: 0, Height: 1, Rows: input.Rows}, {Width: 1, Height: 1}, {Width: 1, Height: 1, Rows: []ArenaRow{{Window: true, Visible: true}}}, {Width: 1, Height: 1, Rows: []ArenaRow{{Backdrop: 64}}}, {Width: 1, Height: 1, Rows: []ArenaRow{{Palette: [64]uint16{512}}}}} {
		if got, err := RasterizeArena(catalog, bad); err == nil || !reflect.DeepEqual(got, ArenaRaster{}) {
			t.Fatal("bad frame accepted")
		}
	}
	bg, err := RasterizeArena(catalog, input)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := json.Marshal(bg)
	if got, err := CompositeArenaSprites(bg, Raster{Width: 1, Height: 1}); err == nil || !reflect.DeepEqual(got, Raster{}) {
		t.Fatal("truncated sprite raster accepted")
	}
	after, _ := json.Marshal(bg)
	if !bytes.Equal(old, after) {
		t.Fatal("source raster mutated on error")
	}
	after, _ = json.Marshal(catalog)
	if !bytes.Equal(before, after) {
		t.Fatal("catalog mutated")
	}
}
