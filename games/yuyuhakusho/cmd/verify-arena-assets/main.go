// Command verify-arena-assets checks independently decoded plane pixels against
// declared original video references. It executes neither ROM nor emulator.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
)

type arenaReferenceFrame struct {
	Frame, Width, Height int
	ArenaOffset          uint16 `json:"arena_offset"`
	SceneObject          string `json:"scene_object"`
	RAM, VRAM, VSRAM     string
	CRAM                 []uint16
	Registers            []uint8
	PixelFormat          int `json:"pixel_format"`
	Video                string
	VideoNext            string            `json:"video_next"`
	VideoNextFrame       int               `json:"video_next_frame"`
	ActiveVRAM           string            `json:"active_vram"`
	ActiveLines          []arenaActiveLine `json:"active_lines"`
	ActiveVRAMChanged    bool              `json:"active_background_vram_changed"`
}

type arenaActiveLine struct {
	Line       int
	Draw       bool
	FirstCycle int32 `json:"first_cycle"`
	Registers  []uint8
	CRAM       []uint16
	VSRAM      string
	HScroll    string
}

type arenaReference struct {
	Schema   int
	Identity struct {
		ROMSHA string `json:"rom_sha256"`
	}
	Frames        []arenaReferenceFrame
	Bindings      []arenaBinding
	ObservedLoads []struct {
		Frame        int
		ResourceList uint32 `json:"resource_list"`
	} `json:"observed_loads"`
	PaletteWords []uint16 `json:"palette_words"`
}

type arenaBackground struct {
	Width, Height int
	Colors        []uint16
	Indices       []uint8
	Sources       []uint8 // Backdrop, A, B, Window.
}

type arenaReport struct {
	Pass                 bool        `json:"pass"`
	Frames               int         `json:"frames"`
	ComparedPixels       int         `json:"background_pixels"`
	NonblackPixels       int         `json:"nonblack_pixels"`
	SpritePixelsExcluded int         `json:"sprite_pixels_excluded"`
	PlanePixels          [4]int      `json:"background_source_pixels"`
	Mismatches           int         `json:"mismatches"`
	FirstMismatch        string      `json:"first_mismatch,omitempty"`
	MismatchRows         map[int]int `json:"mismatch_rows,omitempty"`
	Scope                string      `json:"scope"`
	CatalogTiles         int         `json:"catalog_tiles,omitempty"`
}

func arenaHash(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func arenaBytes(encoded string, length int) ([]byte, error) {
	b, err := hex.DecodeString(encoded)
	if err != nil || len(b) != length {
		return nil, errors.New("invalid original arena byte length")
	}
	return b, nil
}

func loadArenaReference(path string) (arenaReference, error) {
	var f arenaReference
	zipped, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return f, err
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		return f, err
	}
	manifest, err := os.ReadFile(filepath.Join(filepath.Dir(path), "arena-assets-reference-manifest.json"))
	if err != nil {
		return f, err
	}
	var m struct {
		Pass     bool
		Baseline bool   `json:"frames_match_original_core"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
		Records  int
	}
	if err := json.Unmarshal(manifest, &m); err != nil {
		return f, err
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	if !m.Pass || !m.Baseline || arenaHash(raw) != m.SHA || arenaHash(zipped) != m.Gzip || m.Records != len(f.Frames) || f.Schema != 1 || f.Identity.ROMSHA != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" {
		return f, errors.New("unbound original arena reference")
	}
	return f, nil
}

func arenaColor565(compact uint16, limited bool) uint16 {
	if limited {
		compact &= 0x49
	}
	r, g, b := (compact&7)*2, (compact>>3&7)*2, (compact>>6&7)*2
	return r<<12 | r>>3<<11 | g<<7 | g>>2<<5 | b<<1 | b>>3
}

func arenaTilePixel(vram []byte, attribute uint16, x, y int) uint8 {
	if attribute&0x800 != 0 {
		x = 7 - x
	}
	if attribute&0x1000 != 0 {
		y = 7 - y
	}
	packed := vram[int(attribute&0x7ff)*32+y*4+x/2]
	return packed >> uint(4*(1-x&1)) & 15
}

// decodeArenaBackground composes ordinary H40 planes with the fixed 64x32
// geometry and global vertical scrolling actually observed in these scenes.
// Actual visible/blank rows, scrolling and palette state are supplied by the
// observed raster trace. The earlier frame-boundary snapshot is provenance,
// not a substitute for those changing display inputs.
func decodeArenaBackground(f arenaReferenceFrame) (arenaBackground, error) {
	out := arenaBackground{Width: f.Width, Height: f.Height}
	if f.Width != 320 || f.Height != 224 || f.PixelFormat != 2 || len(f.Registers) != 32 || len(f.CRAM) != 64 || f.Registers[12]&0x0e != 0 || f.Registers[12]&1 == 0 || f.Registers[11]&4 != 0 || f.Registers[16] != 1 || f.Registers[17] != 0 {
		return out, errors.New("unsupported arena geometry, scroll, window or color mode")
	}
	for _, c := range f.CRAM {
		if c > 511 {
			return out, errors.New("invalid compact palette value")
		}
	}
	_, err := arenaBytes(f.VRAM, 65536)
	if err != nil {
		return out, err
	}
	_, err = arenaBytes(f.VSRAM, 128)
	if err != nil {
		return out, err
	}
	if len(f.ActiveLines) != f.Height || f.ActiveVRAMChanged {
		return out, errors.New("missing raster observations or unsupported active background VRAM change")
	}
	vram, err := arenaBytes(f.ActiveVRAM, 65536)
	if err != nil {
		return out, err
	}
	word := func(address int) uint16 { return uint16(vram[address&65535])<<8 | uint16(vram[(address+1)&65535]) }
	out.Colors, out.Indices, out.Sources = make([]uint16, f.Width*f.Height), make([]uint8, f.Width*f.Height), make([]uint8, f.Width*f.Height)
	for y := 0; y < f.Height; y++ {
		line := f.ActiveLines[y]
		if line.Line != y || len(line.Registers) != 32 || len(line.CRAM) != 64 {
			return out, errors.New("unbound raster row")
		}
		for _, c := range line.CRAM {
			if c > 511 {
				return out, errors.New("invalid raster palette")
			}
		}
		vsram, err := arenaBytes(line.VSRAM, 4)
		if err != nil {
			return out, err
		}
		hscroll, err := arenaBytes(line.HScroll, 4)
		if err != nil {
			return out, err
		}
		regs := line.Registers
		if regs[12]&0x0e != 0 || regs[12]&1 == 0 || regs[11]&4 != 0 || regs[16] != 1 || regs[17] != 0 {
			return out, errors.New("unsupported raster line profile")
		}
		baseA, baseB, baseW := int(regs[2]&0x38)<<10, int(regs[4]&7)<<13, int(regs[3]&0x3c)<<10
		vertical := [2]int{int(binary.BigEndian.Uint16(vsram) & 1023), int(binary.BigEndian.Uint16(vsram[2:]) & 1023)}
		horizontal := [2]int{int(binary.BigEndian.Uint16(hscroll)), int(binary.BigEndian.Uint16(hscroll[2:]))}
		window := (regs[18]&0x80 != 0) == (y >= int(regs[18]&31)*8)
		for x := 0; x < f.Width; x++ {
			if !line.Draw {
				index, p := regs[7]&63, y*f.Width+x
				out.Indices[p], out.Colors[p] = index, arenaColor565(line.CRAM[index], regs[0]&4 == 0)
				continue
			}
			attributes := [2]uint16{}
			values := [2]uint8{}
			for plane, base := range [...]int{baseA, baseB} {
				u, v := (x-horizontal[plane])&511, (y+vertical[plane])&255
				if plane == 0 && window {
					u, v, base = x, y, baseW
				}
				attributes[plane] = word(base + ((v/8)*64+u/8)*2)
				values[plane] = arenaTilePixel(vram, attributes[plane], u&7, v&7)
			}
			index, source := regs[7]&63, uint8(0)
			if values[1] != 0 {
				index, source = uint8(attributes[1]>>9&0x30)|values[1], 2
			}
			if values[0] != 0 && (values[1] == 0 || attributes[0]&0x8000 != 0 || attributes[1]&0x8000 == 0) {
				index, source = uint8(attributes[0]>>9&0x30)|values[0], 1
				if window {
					source = 3
				}
			}
			p := y*f.Width + x
			out.Indices[p], out.Sources[p], out.Colors[p] = index, source, arenaColor565(line.CRAM[index], regs[0]&4 == 0)
		}
	}
	return out, nil
}

// Every opaque sprite sample is conservatively excluded. This verifies only
// background pixels and does not claim hardware sprite/plane merge behavior.
func arenaSpriteMask(f arenaReferenceFrame) ([]bool, error) {
	vram, err := arenaBytes(f.ActiveVRAM, 65536)
	if err != nil {
		return nil, err
	}
	mask := make([]bool, f.Width*f.Height)
	base := int(f.Registers[5]&0x7e) << 9
	seen, index := [80]bool{}, 0
	for count := 0; count < 80; count++ {
		if index >= 80 || seen[index] {
			return nil, errors.New("invalid sprite exclusion chain")
		}
		seen[index] = true
		entry := vram[(base+index*8)&65535:]
		sx, sy := int(binary.BigEndian.Uint16(entry[6:])&511)-128, int(binary.BigEndian.Uint16(entry)&1023)-128
		attr := binary.BigEndian.Uint16(entry[4:])
		size, link := entry[2]&15, entry[3]&127
		width, height := int(size>>2)+1, int(size&3)+1
		for py := 0; py < height*8; py++ {
			for px := 0; px < width*8; px++ {
				x, y := sx+px, sy+py
				if x < 0 || x >= f.Width || y < 0 || y >= f.Height {
					continue
				}
				u, v := px, py
				if attr&0x800 != 0 {
					u = width*8 - 1 - u
				}
				if attr&0x1000 != 0 {
					v = height*8 - 1 - v
				}
				tile := (int(attr&0x7ff) + (u/8)*height + v/8) & 2047
				if arenaTilePixel(vram, uint16(tile), u&7, v&7) != 0 {
					mask[y*f.Width+x] = true
				}
			}
		}
		if link == 0 {
			return mask, nil
		}
		index = int(link)
	}
	return nil, errors.New("unterminated sprite exclusion chain")
}

func verifyArenaReferences(f arenaReference) (arenaReport, error) {
	r := arenaReport{Frames: len(f.Frames), Scope: "Default arena normal-mode A/B/Window pixels excluding every opaque sprite; source state f with observed raster/video f+1"}
	seen := make(map[int]bool)
	for _, frame := range f.Frames {
		if frame.Frame <= 0 || seen[frame.Frame] || frame.VideoNextFrame != frame.Frame+1 || frame.ArenaOffset != 0 {
			return r, errors.New("unbound arena, frame identity or video timing")
		}
		seen[frame.Frame] = true
		ram, err := arenaBytes(frame.RAM, 65536)
		if err != nil || binary.BigEndian.Uint16(ram[0xa060:]) != frame.ArenaOffset || binary.BigEndian.Uint32(ram[0x8180:]) != 0x20918 {
			return r, errors.New("reference is not the confirmed default arena object")
		}
		bg, err := decodeArenaBackground(frame)
		if err != nil {
			return r, err
		}
		mask, err := arenaSpriteMask(frame)
		if err != nil {
			return r, err
		}
		video, err := arenaBytes(frame.VideoNext, frame.Width*frame.Height*2)
		if err != nil {
			return r, err
		}
		for i, c := range bg.Colors {
			if mask[i] {
				r.SpritePixelsExcluded++
				continue
			}
			r.ComparedPixels++
			r.PlanePixels[bg.Sources[i]]++
			if c != 0 {
				r.NonblackPixels++
			}
			expected := binary.LittleEndian.Uint16(video[i*2:])
			if c != expected {
				r.Mismatches++
				if r.MismatchRows == nil {
					r.MismatchRows = make(map[int]int)
				}
				r.MismatchRows[i/frame.Width]++
				if r.FirstMismatch == "" {
					r.FirstMismatch = fmt.Sprintf("frame=%d x=%d y=%d source=%d got=%04x want=%04x", frame.Frame, i%frame.Width, i/frame.Width, bg.Sources[i], c, expected)
				}
			}
		}
	}
	if r.Frames == 0 || r.ComparedPixels == 0 || r.Mismatches != 0 {
		return r, errors.New("original arena background pixel mismatch")
	}
	r.Pass = true
	return r, nil
}

func saveArenaPNG(path string, bg arenaBackground) error {
	im := image.NewRGBA(image.Rect(0, 0, bg.Width, bg.Height))
	for i, c := range bg.Colors {
		im.SetRGBA(i%bg.Width, i/bg.Width, color.RGBA{uint8((c >> 11 & 31) * 255 / 31), uint8((c >> 5 & 63) * 255 / 63), uint8((c & 31) * 255 / 31), 255})
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, im)
}

func main() {
	path := flag.String("reference", "games/yuyuhakusho/data/arena-assets-reference.json.gz", "verified original reference")
	preview := flag.String("preview", "", "optional independent background PNG")
	frame := flag.Int("frame", 0, "optional single reference state frame")
	catalogOut := flag.String("catalog-out", "", "optional compact default-arena catalog")
	flag.Parse()
	f, err := loadArenaReference(*path)
	if err == nil && *frame != 0 {
		selected := []arenaReferenceFrame{}
		for _, s := range f.Frames {
			if s.Frame == *frame {
				selected = append(selected, s)
			}
		}
		f.Frames = selected
	}
	if err == nil {
		var report arenaReport
		if err == nil && *preview != "" {
			var bg arenaBackground
			index := 0
			if len(f.Frames) > 1 {
				index = 1
			}
			bg, err = decodeArenaBackground(f.Frames[index])
			if err == nil {
				err = saveArenaPNG(*preview, bg)
			}
		}
		if err == nil {
			report, err = verifyArenaReferences(f)
		}
		if err == nil && *catalogOut != "" {
			var catalog arenaCatalog
			catalog, err = buildArenaCatalog(f)
			if err == nil {
				var raw []byte
				raw, err = json.Marshal(catalog)
				if err == nil {
					err = os.MkdirAll(filepath.Dir(*catalogOut), 0755)
				}
				if err == nil {
					var file *os.File
					file, err = os.Create(*catalogOut)
					if err == nil {
						z := gzip.NewWriter(file)
						_, err = z.Write(raw)
						if closeErr := z.Close(); err == nil {
							err = closeErr
						}
						if closeErr := file.Close(); err == nil {
							err = closeErr
						}
					}
				}
				report.CatalogTiles = len(catalog.Tiles)
			}
		}
		_ = json.NewEncoder(os.Stdout).Encode(report)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
