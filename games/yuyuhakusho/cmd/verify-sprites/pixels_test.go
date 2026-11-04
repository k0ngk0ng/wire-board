package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// Original video/VRAM references and these tests precede pixels.go.
func readSpritePixels(t *testing.T) []spritePixelFrame {
	t.Helper()
	raw, err := os.ReadFile("../../data/sprite-pixel-reference.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	decoded, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	mraw, err := os.ReadFile("../../data/sprite-oracle-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		SHA string `json:"pixel_reference_sha256"`
	}
	if err := json.Unmarshal(mraw, &m); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(decoded)
	if hex.EncodeToString(h[:]) != m.SHA {
		t.Fatal("unbound original sprite pixels")
	}
	var frames []spritePixelFrame
	if err := json.Unmarshal(decoded, &frames); err != nil {
		t.Fatal(err)
	}
	return frames
}

func TestNaturalSpritePixelsAgainstVideo(t *testing.T) {
	frames := readSpritePixels(t)
	report, err := verifyNaturalSpritePixels(frames)
	if err != nil || !report.Pass || report.Frames != 20 || report.CharacterPixels == 0 || report.ComparedPixels == 0 || report.Mismatches != 0 || report.ROMTransfers != 41 {
		t.Fatalf("original sprite pixel comparison: %+v %v", report, err)
	}
	t.Logf("original video frames=%d high sprite pixels=%d character pixels=%d ROM transfers=%d", report.Frames, report.ComparedPixels, report.CharacterPixels, report.ROMTransfers)
}

func TestSpritePixelsRejectWrongTimingOrData(t *testing.T) {
	frames := readSpritePixels(t)
	for _, mutate := range []func([]spritePixelFrame){
		func(f []spritePixelFrame) { f[0].VideoNext = f[0].Video },
		func(f []spritePixelFrame) { f[0].VideoNextFrame++ },
		func(f []spritePixelFrame) { f[0].VRAM = strings.Repeat("00", 65536) },
		func(f []spritePixelFrame) { f[0].CRAMCompact = make([]uint16, 64) },
		func(f []spritePixelFrame) { f[0].ROMSources[0].Bytes = "00" },
		func(f []spritePixelFrame) { f[0].ROMSources[0].Destination++ },
		func(f []spritePixelFrame) { f[0].VDPRegisters[12] |= 8 },
	} {
		raw, err := json.Marshal(frames)
		if err != nil {
			t.Fatal(err)
		}
		var changed []spritePixelFrame
		if err := json.Unmarshal(raw, &changed); err != nil {
			t.Fatal(err)
		}
		mutate(changed)
		if _, err := verifyNaturalSpritePixels(changed); err == nil {
			t.Fatal("wrong timing or corrupted sprite pixel evidence accepted")
		}
	}
}
