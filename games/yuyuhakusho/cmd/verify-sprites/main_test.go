package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// The fixture producer, failure list and these tests precede the independent
// decoder/composer. Expected sprite and DMA bytes come from original CPU calls.
func readSpriteFixture(t *testing.T) spriteFixture {
	t.Helper()
	zipped, err := os.ReadFile("../../data/sprite-oracle.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("../../data/sprite-oracle-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass     bool
		Baseline bool   `json:"frames_match_original_core"`
		Complete bool   `json:"all_library_frames_checked"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(manifest, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || !m.Complete || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original sprite fixture")
	}
	var f spriteFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestSpritesAgainstOriginalCPU(t *testing.T) {
	f := readSpriteFixture(t)
	report, err := verifySpriteFixture(f)
	if err != nil || !report.Pass || report.Frames != len(f.Library) || report.Calls != len(f.Calls) {
		t.Fatalf("original sprite comparison failed: %+v %v", report, err)
	}
	t.Logf("original sprite frames=%d full CPU outputs=%d", report.Frames, report.Calls)
}

func TestSpritesRejectCorruptedOriginalEvidence(t *testing.T) {
	f := readSpriteFixture(t)
	for _, mutate := range []func(*spriteFixture){
		func(f *spriteFixture) { f.Library[0].Payload = "0000" },
		func(f *spriteFixture) { f.Library = f.Library[1:] },
		func(f *spriteFixture) { f.Calls[0].Return = "0x17ca" },
		func(f *spriteFixture) { f.Calls[0].After.Flags ^= 1 },
		func(f *spriteFixture) { f.Calls[0].After.Actor = "0000" },
		func(f *spriteFixture) { f.Calls[0].After.Sprites = "0000" },
		func(f *spriteFixture) { f.Calls[0].After.DMA = "0000" },
		func(f *spriteFixture) { f.Calls[0].After.Layers = "0000" },
		func(f *spriteFixture) { f.Calls[0].Parameter ^= 2 },
		func(f *spriteFixture) { f.Calls = f.Calls[:len(f.Calls)-1] },
		func(f *spriteFixture) { f.Calls = append(f.Calls, f.Calls[0]) },
	} {
		// Snapshot strings are immutable; copying these value-only slices
		// isolates every mutation without reserializing the large fixture.
		changed := f
		changed.Library = append([]spriteFrame(nil), f.Library...)
		changed.Calls = append([]spriteCall(nil), f.Calls...)
		mutate(&changed)
		if _, err := verifySpriteFixture(changed); err == nil {
			t.Fatal("corrupted original sprite evidence accepted")
		}
	}
}

func TestSpriteMalformedInputsAreAtomic(t *testing.T) {
	f := readSpriteFixture(t)
	frame, input := f.Library[0], f.Calls[0].Before
	for _, malformed := range []string{"", "00000000", "8000000000000000", "not-hex"} {
		frame.Payload = malformed
		got, err := composeSpriteSnapshot(input, frame)
		if err == nil || got != input {
			t.Fatal("invalid sprite payload caused partial output")
		}
	}
	frame = f.Library[0]
	for _, cursor := range []uint16{0xd401, 0xd3fe, 0xe001} {
		bad := input
		bad.DMACursor = cursor
		got, err := composeSpriteSnapshot(bad, frame)
		if err == nil || got != bad {
			t.Fatal("invalid DMA queue caused partial output")
		}
	}
}
