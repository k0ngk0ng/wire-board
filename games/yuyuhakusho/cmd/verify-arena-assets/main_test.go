package main

import (
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
)

func originalArena(t *testing.T) arenaReference {
	t.Helper()
	f, err := loadArenaReference("../../data/arena-assets-reference.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// Original natural VDP/video references and these tests precede the decoder.
func TestArenaBackgroundAgainstOriginalVideo(t *testing.T) {
	f := originalArena(t)
	old, _ := json.Marshal(f)
	report, err := verifyArenaReferences(f)
	if err != nil || !report.Pass || report.Frames != 20 || report.ComparedPixels < 1000000 || report.NonblackPixels < 500000 || report.Mismatches != 0 || report.PlanePixels[1] == 0 || report.PlanePixels[2] == 0 || report.PlanePixels[3] == 0 {
		t.Fatalf("default arena background does not match original video: %+v, %v", report, err)
	}
	now, _ := json.Marshal(f)
	if !reflect.DeepEqual(old, now) {
		t.Fatal("arena comparison mutated original immutable evidence")
	}
	t.Logf("frames=%d pixels=%d nonblack=%d layers=%v sprites_excluded=%d", report.Frames, report.ComparedPixels, report.NonblackPixels, report.PlanePixels, report.SpritePixelsExcluded)
}

func TestArenaBackgroundRejectsWrongTimingAndCorruptEvidence(t *testing.T) {
	for _, kind := range []string{"frame", "tiles", "palette", "scroll", "mode"} {
		t.Run(kind, func(t *testing.T) {
			f := originalArena(t)
			s := &f.Frames[1] // The first frame is an initial dark/fade reference.
			switch kind {
			case "frame":
				s.VideoNextFrame = s.Frame
			case "tiles":
				s.VRAM = hex.EncodeToString(make([]byte, 65536))
				s.ActiveVRAM = s.VRAM
			case "palette":
				s.CRAM = make([]uint16, 64)
				s.ActiveLines = append([]arenaActiveLine(nil), s.ActiveLines...)
				for i := range s.ActiveLines {
					s.ActiveLines[i].CRAM = make([]uint16, 64)
				}
			case "scroll":
				b, _ := hex.DecodeString(s.VRAM)
				for i := 0xf000; i < 0xf380; i++ {
					b[i] = 0
				}
				s.VRAM = hex.EncodeToString(b)
				s.ActiveLines = append([]arenaActiveLine(nil), s.ActiveLines...)
				for i := range s.ActiveLines {
					s.ActiveLines[i].HScroll = "00000000"
				}
			case "mode":
				s.Registers = append([]uint8(nil), s.Registers...)
				s.Registers[12] |= 8
			}
			if report, err := verifyArenaReferences(f); err == nil || report.Pass {
				t.Fatalf("corrupt %s reference was accepted", kind)
			}
		})
	}
}

func TestArenaBackgroundRejectsUnsupportedOrTruncatedInput(t *testing.T) {
	f := originalArena(t)
	for _, kind := range []string{"vram", "vsram", "cram", "column-scroll", "width", "window"} {
		t.Run(kind, func(t *testing.T) {
			s := f.Frames[1]
			s.Registers = append([]uint8(nil), s.Registers...)
			switch kind {
			case "vram":
				s.VRAM = "00"
			case "vsram":
				s.VSRAM = "00"
			case "cram":
				s.CRAM = []uint16{0}
			case "column-scroll":
				s.Registers[11] |= 4
			case "width":
				s.Width = 256
			case "window":
				s.Registers[17] = 5 // Partial horizontal window scrolling quirk is not covered.
			}
			if _, err := decodeArenaBackground(s); err == nil {
				t.Fatal("unsupported/truncated arena input accepted")
			}
		})
	}
}

func TestCompactArenaCatalogBindsEveryPlaneCellAndTile(t *testing.T) {
	f := originalArena(t)
	catalog, err := buildArenaCatalog(f)
	if err != nil || catalog.ArenaOffset != 0 || len(catalog.Tiles) < 100 || len(catalog.Tiles) > 1024 || len(catalog.Planes) != 2 {
		t.Fatal("missing compact default-arena plane assets", err)
	}
	if err := verifyArenaCatalog(catalog, f); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"tile", "cell", "binding"} {
		bad := catalog
		bad.Tiles = append([]string(nil), catalog.Tiles...)
		bad.Planes = append([]arenaPlaneAsset(nil), catalog.Planes...)
		for i := range bad.Planes {
			bad.Planes[i].Cells = append([]arenaCell(nil), catalog.Planes[i].Cells...)
		}
		switch mutation {
		case "tile":
			bad.Tiles[0] = hex.EncodeToString(make([]byte, 32))
			if bad.Tiles[0] == catalog.Tiles[0] {
				bad.Tiles[0] = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
			}
		case "cell":
			bad.Planes[0].Cells[0].Flags ^= 0x800
		case "binding":
			bad.SceneBehavior++
		}
		if err := verifyArenaCatalog(bad, f); err == nil {
			t.Fatal("corrupt compact arena asset accepted", mutation)
		}
	}
}
