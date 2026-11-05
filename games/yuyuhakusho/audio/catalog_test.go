package audio_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	finite "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio"
)

const sourceSHA = "617737f8ef07043401fa74f7f48ba551a64dd91cedd3a94dbfcf4883e0ff1541"

func sum(raw []byte) string {
	value := sha256.Sum256(raw)
	return hex.EncodeToString(value[:])
}

func catalogRaw(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("../data/audio-finite-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func catalog(t *testing.T) finite.Catalog {
	t.Helper()
	value, err := finite.DecodeCatalog(catalogRaw(t))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

type legacyClip struct {
	Code     uint8       `json:"code"`
	Kind     finite.Kind `json:"classification"`
	Frames   uint64      `json:"stereo_frames"`
	WAV      string      `json:"wav_sha256"`
	PCM      string      `json:"pcm_sha256"`
	Onset    *uint64     `json:"first_nonzero_stereo_frame"`
	Complete bool        `json:"complete"`
}

func legacy(t *testing.T) (map[uint8]legacyClip, [11][]uint8) {
	t.Helper()
	raw, err := os.ReadFile("../data/audio-assets.json")
	if err != nil {
		t.Fatal(err)
	}
	if sum(raw) != sourceSHA {
		t.Fatal("immutable original source catalog differs")
	}
	var source struct {
		Assets   []legacyClip `json:"assets"`
		Bindings []struct {
			Code       uint8
			Characters []uint8
		} `json:"animation_bindings"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	clips := make(map[uint8]legacyClip)
	var references [11][]uint8
	for _, row := range source.Assets {
		if !row.Complete {
			t.Fatal("nonfinite original source")
		}
		clips[row.Code] = row
	}
	for _, row := range source.Bindings {
		for _, character := range row.Characters {
			if character >= 11 {
				t.Fatal("invalid original source character")
			}
			references[character] = append(references[character], row.Code)
		}
	}
	if len(clips) != 134 {
		t.Fatal("incomplete original finite set")
	}
	return clips, references
}

func TestCatalogAgainstCompleteImmutableSource(t *testing.T) {
	c := catalog(t)
	want, references := legacy(t)
	for code := 0; code < 256; code++ {
		clip, ok := c.Lookup(uint8(code))
		row, exists := want[uint8(code)]
		if ok != exists {
			t.Fatalf("code%02x present=%v want%v", code, ok, exists)
		}
		if !exists {
			continue
		}
		onset := uint64(971) //8f is missing only this field in the original metadata.
		if row.Onset != nil {
			onset = *row.Onset
		}
		if clip.Code != uint8(code) || clip.Kind != row.Kind || clip.StereoFrames != row.Frames || clip.WAVSHA256 != row.WAV || clip.PCMSHA256 != row.PCM || clip.FirstNonzeroStereoFrame != onset || clip.Loop {
			t.Fatalf("clip%02x differs from original finite acquisition: %+v", code, clip)
		}
		if clip.DurationSeconds() != float64(row.Frames)/44100 || clip.MediaPath() != fmt.Sprintf("yuyuhakusho/audio/%02x.wav", code) {
			t.Fatalf("clip%02x derived duration or media path differs", code)
		}
	}
	unique := make(map[uint8]bool)
	for character, expected := range references {
		codes, err := c.AnimationCodes(uint8(character))
		if err != nil || !reflect.DeepEqual(codes, expected) {
			t.Fatalf("source character%d animation references %v want%v: %v", character, codes, expected, err)
		}
		for _, code := range codes {
			unique[code] = true
		}
	}
	if len(unique) != 78 {
		t.Fatalf("animation references union=%d want78", len(unique))
	}
	for _, character := range []uint8{11, 127, 255} {
		if codes, err := c.AnimationCodes(character); err == nil || len(codes) != 0 {
			t.Fatal("invalid source character accepted")
		}
	}
	t.Log("134 finite clips; all256 byte IDs checked; exact11 source-character AN lists,78 unique references")
}

func object(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func marshal(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func firstClip(value map[string]any) map[string]any {
	return value["clips"].([]any)[0].(map[string]any)
}
func firstBinding(value map[string]any) map[string]any {
	return value["animation_bindings"].([]any)[0].(map[string]any)
}

func TestCatalogRejectsInvalidIdentityStructureAndConflictsAtomically(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"schema":            func(m map[string]any) { m["schema"] = 2 },
		"rom":               func(m map[string]any) { m["rom_sha256"] = strings.Repeat("0", 64) },
		"source-catalog":    func(m map[string]any) { m["source_catalog_sha256"] = strings.Repeat("0", 64) },
		"source-origin":     func(m map[string]any) { m["source_origin"] = "first-nonzero-sample" },
		"missing-root":      func(m map[string]any) { delete(m, "source_origin") },
		"null-root":         func(m map[string]any) { m["clips"] = nil },
		"path-payload":      func(m map[string]any) { m["media_root"] = "/tmp/unverified" },
		"format-rate":       func(m map[string]any) { m["format"].(map[string]any)["sample_rate"] = 48000 },
		"format-channel":    func(m map[string]any) { m["format"].(map[string]any)["channels"] = 1 },
		"format-width":      func(m map[string]any) { m["format"].(map[string]any)["sample_width_bytes"] = 1 },
		"format-encoding":   func(m map[string]any) { m["format"].(map[string]any)["pcm_encoding"] = "signed16-big-endian" },
		"format-missing":    func(m map[string]any) { delete(m["format"].(map[string]any), "channels") },
		"missing-clip":      func(m map[string]any) { m["clips"] = m["clips"].([]any)[1:] },
		"extra-clip":        func(m map[string]any) { m["clips"] = append(m["clips"].([]any), firstClip(m)) },
		"duplicate-code":    func(m map[string]any) { m["clips"].([]any)[1].(map[string]any)["code"] = 6 },
		"control-code":      func(m map[string]any) { firstClip(m)["code"] = 4 },
		"silent-diagnostic": func(m map[string]any) { firstClip(m)["code"] = 127 },
		"bounded82":         func(m map[string]any) { firstClip(m)["code"] = 130 },
		"bounded87":         func(m map[string]any) { firstClip(m)["code"] = 135 },
		"other-bgm":         func(m map[string]any) { firstClip(m)["code"] = 134 },
		"wrong-kind":        func(m map[string]any) { firstClip(m)["kind"] = "finite-music-cue" },
		"missing-loop":      func(m map[string]any) { delete(firstClip(m), "loop") },
		"null-loop":         func(m map[string]any) { firstClip(m)["loop"] = nil },
		"true-loop":         func(m map[string]any) { firstClip(m)["loop"] = true },
		"uppercase-sha": func(m map[string]any) {
			firstClip(m)["wav_sha256"] = strings.ToUpper(firstClip(m)["wav_sha256"].(string))
		},
		"invalid-sha":                func(m map[string]any) { firstClip(m)["pcm_sha256"] = strings.Repeat("g", 64) },
		"short-sha":                  func(m map[string]any) { firstClip(m)["pcm_sha256"] = "00" },
		"zero-frames":                func(m map[string]any) { firstClip(m)["stereo_frames"] = 0 },
		"negative-frames":            func(m map[string]any) { firstClip(m)["stereo_frames"] = -1 },
		"overflow-frames":            func(m map[string]any) { firstClip(m)["stereo_frames"] = uint64(1) << 40 },
		"late-onset":                 func(m map[string]any) { firstClip(m)["first_nonzero_stereo_frame"] = firstClip(m)["stereo_frames"] },
		"missing-onset":              func(m map[string]any) { delete(firstClip(m), "first_nonzero_stereo_frame") },
		"null-onset":                 func(m map[string]any) { firstClip(m)["first_nonzero_stereo_frame"] = nil },
		"clip-url":                   func(m map[string]any) { firstClip(m)["url"] = "https://example.invalid/audio.wav" },
		"stored-duration":            func(m map[string]any) { firstClip(m)["duration_seconds"] = 1 },
		"wrong-case-key":             func(m map[string]any) { firstClip(m)["Code"] = firstClip(m)["code"]; delete(firstClip(m), "code") },
		"missing-character":          func(m map[string]any) { m["animation_bindings"] = m["animation_bindings"].([]any)[1:] },
		"duplicate-character":        func(m map[string]any) { m["animation_bindings"].([]any)[1].(map[string]any)["character"] = 0 },
		"invalid-character":          func(m map[string]any) { firstBinding(m)["character"] = 11 },
		"null-an-codes":              func(m map[string]any) { firstBinding(m)["codes"] = nil },
		"missing-an-code":            func(m map[string]any) { firstBinding(m)["codes"] = firstBinding(m)["codes"].([]any)[1:] },
		"duplicate-an-code":          func(m map[string]any) { codes := firstBinding(m)["codes"].([]any); codes[1] = codes[0] },
		"wrong-an-character-binding": func(m map[string]any) { firstBinding(m)["codes"].([]any)[0] = 6 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			value := object(t, catalogRaw(t))
			mutate(value)
			got, err := finite.DecodeCatalog(marshal(t, value))
			if err == nil || !reflect.DeepEqual(got, finite.Catalog{}) {
				t.Fatalf("invalid%s accepted or partial catalog leaked: %v", name, err)
			}
		})
	}
	raw := catalogRaw(t)
	invalid := map[string][]byte{"empty": nil, "null": []byte("null"), "object": []byte("{}"), "truncated": raw[:len(raw)/2], "trailing": append(append([]byte(nil), raw...), []byte("{}")...), "duplicate-key": append([]byte(`{"schema":1,`), raw[1:]...), "oversize": bytes.Repeat([]byte(" "), (1<<20)+1)}
	for name, encoded := range invalid {
		t.Run(name, func(t *testing.T) {
			got, err := finite.DecodeCatalog(encoded)
			if err == nil || !reflect.DeepEqual(got, finite.Catalog{}) {
				t.Fatalf("invalid%s accepted or nonzero state returned", name)
			}
		})
	}
}

func TestCatalogCopiesAndConcurrentReads(t *testing.T) {
	c := catalog(t)
	initial, _ := c.Lookup(6)
	changed := initial
	changed.WAVSHA256 = "changed"
	changed.Code = 7
	current, _ := c.Lookup(6)
	if current != initial {
		t.Fatal("Lookup returned shared mutable state")
	}
	codes, _ := c.AnimationCodes(0)
	want := append([]uint8(nil), codes...)
	codes[0] ^= 255
	next, _ := c.AnimationCodes(0)
	if !reflect.DeepEqual(next, want) {
		t.Fatal("AN references are mutable through returned slice")
	}
	var group sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for i := 0; i < 100; i++ {
				if clip, ok := c.Lookup(6); !ok || clip != initial {
					t.Error("immutable lookup changed")
				}
				copy, _ := c.AnimationCodes(0)
				copy[0] = 255
			}
		}()
	}
	group.Wait()
	var zero finite.Catalog
	if _, ok := zero.Lookup(6); ok {
		t.Fatal("zero catalog returned finite clip")
	}
	if _, err := zero.AnimationCodes(0); err == nil {
		t.Fatal("zero catalog returned animation references")
	}
	if err := zero.ValidateWAV(6, nil); err == nil {
		t.Fatal("zero catalog accepted media")
	}
	for _, code := range []uint8{0, 4, 127, 130, 135, 134, 255} {
		if (finite.Clip{Code: code}).MediaPath() != "" {
			t.Fatal("unknown/diagnostic clip has a media path")
		}
	}
}

// Small synthetic RIFF fixtures check the WAV parser, not original-media
// authenticity. Real134 media acceptance is separately enabled below and by CLI.
func smallWAV() []byte {
	pcm := []byte{0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6, 0, 7, 0, 8, 0, 0, 0, 0, 0}
	raw := make([]byte, 44+len(pcm))
	copy(raw, "RIFF")
	binary.LittleEndian.PutUint32(raw[4:], uint32(len(raw)-8))
	copy(raw[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(raw[16:], 16)
	binary.LittleEndian.PutUint16(raw[20:], 1)
	binary.LittleEndian.PutUint16(raw[22:], 2)
	binary.LittleEndian.PutUint32(raw[24:], 44100)
	binary.LittleEndian.PutUint32(raw[28:], 176400)
	binary.LittleEndian.PutUint16(raw[32:], 4)
	binary.LittleEndian.PutUint16(raw[34:], 16)
	copy(raw[36:], "data")
	binary.LittleEndian.PutUint32(raw[40:], uint32(len(pcm)))
	copy(raw[44:], pcm)
	return raw
}

func withMedia(t *testing.T, raw []byte, frames, onset uint64, pcmSHA string) finite.Catalog {
	t.Helper()
	value := object(t, catalogRaw(t))
	clip := firstClip(value)
	clip["wav_sha256"], clip["pcm_sha256"], clip["stereo_frames"], clip["first_nonzero_stereo_frame"] = sum(raw), pcmSHA, frames, onset
	c, err := finite.DecodeCatalog(marshal(t, value))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestValidateWAVFormatHashesCountOnsetAndUntrimmedData(t *testing.T) {
	valid := smallWAV()
	pcmHash := sum(valid[44:])
	c := withMedia(t, valid, 8, 2, pcmHash)
	if err := c.ValidateWAV(6, valid); err != nil {
		t.Fatal("valid captured-format fixture rejected", err)
	}
	if err := c.ValidateWAV(7, valid); err == nil {
		t.Fatal("media accepted for wrong finite code")
	}
	for _, code := range []uint8{0, 4, 127, 130, 135, 134, 255} {
		if err := c.ValidateWAV(code, valid); err == nil {
			t.Fatalf("excluded%02x accepted", code)
		}
	}
	mutations := map[string]func([]byte) []byte{
		"riff":            func(b []byte) []byte { b[0] = 'X'; return b },
		"wave":            func(b []byte) []byte { b[8] = 'X'; return b },
		"riff-length":     func(b []byte) []byte { binary.LittleEndian.PutUint32(b[4:], 1000); return b },
		"codec":           func(b []byte) []byte { binary.LittleEndian.PutUint16(b[20:], 3); return b },
		"channels":        func(b []byte) []byte { binary.LittleEndian.PutUint16(b[22:], 1); return b },
		"rate":            func(b []byte) []byte { binary.LittleEndian.PutUint32(b[24:], 48000); return b },
		"byte-rate":       func(b []byte) []byte { binary.LittleEndian.PutUint32(b[28:], 44100); return b },
		"align":           func(b []byte) []byte { binary.LittleEndian.PutUint16(b[32:], 2); return b },
		"bits":            func(b []byte) []byte { binary.LittleEndian.PutUint16(b[34:], 8); return b },
		"missing-fmt":     func(b []byte) []byte { copy(b[12:], "JUNK"); return b },
		"missing-data":    func(b []byte) []byte { copy(b[36:], "JUNK"); return b },
		"chunk-overflow":  func(b []byte) []byte { binary.LittleEndian.PutUint32(b[40:], 0xffffffff); return b },
		"data-misaligned": func(b []byte) []byte { binary.LittleEndian.PutUint32(b[40:], 31); return b },
		"truncated":       func(b []byte) []byte { return b[:len(b)-1] },
		"trailing":        func(b []byte) []byte { return append(b, 0) },
		"duplicate-fmt": func(b []byte) []byte {
			b = append(b, b[12:36]...)
			binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
			return b
		},
		"duplicate-data": func(b []byte) []byte {
			b = append(b, []byte("data\x00\x00\x00\x00")...)
			binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
			return b
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			bad := mutate(append([]byte(nil), valid...))
			matchingFileSHA := withMedia(t, bad, 8, 2, pcmHash)
			if err := matchingFileSHA.ValidateWAV(6, bad); err == nil {
				t.Fatal("invalid WAV header accepted despite matching declared file SHA")
			}
		})
	}
	for _, name := range []string{"file-hash", "pcm-hash", "frame-count", "onset", "front-trim", "tail-trim", "pcm-damage", "silent"} {
		t.Run(name, func(t *testing.T) {
			bad := append([]byte(nil), valid...)
			active := c
			switch name {
			case "file-hash":
				bad[12] = 'X'
			case "pcm-hash":
				active = withMedia(t, bad, 8, 2, strings.Repeat("0", 64))
			case "frame-count":
				active = withMedia(t, bad, 9, 2, pcmHash)
			case "onset":
				active = withMedia(t, bad, 8, 1, pcmHash)
			case "front-trim":
				bad = append(append([]byte(nil), valid[:44]...), valid[52:]...)
				binary.LittleEndian.PutUint32(bad[4:], uint32(len(bad)-8))
				binary.LittleEndian.PutUint32(bad[40:], uint32(len(bad)-44))
			case "tail-trim":
				bad = bad[:len(bad)-4]
				binary.LittleEndian.PutUint32(bad[4:], uint32(len(bad)-8))
				binary.LittleEndian.PutUint32(bad[40:], uint32(len(bad)-44))
			case "pcm-damage":
				bad[60] ^= 127
			case "silent":
				clear(bad[44:])
				active = withMedia(t, bad, 8, 2, sum(bad[44:]))
			}
			if err := active.ValidateWAV(6, bad); err == nil {
				t.Fatal("damaged or trimmed finite media accepted")
			}
		})
	}
}

func TestMetadataIdentityDoesNotPretendToAuthenticateMedia(t *testing.T) {
	value := object(t, catalogRaw(t))
	firstClip(value)["wav_sha256"] = strings.Repeat("0", 64)
	c, err := finite.DecodeCatalog(marshal(t, value))
	if err != nil {
		t.Fatal("well-formed metadata should require caller manifest authentication", err)
	}
	if err := c.ValidateWAV(6, smallWAV()); err == nil {
		t.Fatal("metadata identity was treated as payload authentication")
	}
	t.Log("Caller must pin immutable catalog/manifest SHA; ROM and source-catalog fields alone do not authenticate altered clip metadata")
}

func TestAll134OriginalWAVs(t *testing.T) {
	root := os.Getenv("YYH_FINITE_AUDIO_MEDIA_ROOT")
	if root == "" {
		t.Skip("set YYH_FINITE_AUDIO_MEDIA_ROOT for complete existing-media E2E; pure format tests remain unconditional")
	}
	c := catalog(t)
	want, _ := legacy(t)
	total := 0
	for code := 0; code < 256; code++ {
		row, ok := want[uint8(code)]
		if !ok {
			continue
		}
		folder := "samples"
		if code == 0x8f {
			folder = "music-excerpts"
		}
		path := filepath.Join(root, "isolated", folder, fmt.Sprintf("%02x.wav", code))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if sum(raw) != row.WAV {
			t.Fatal("original WAV differs", path)
		}
		if err := c.ValidateWAV(uint8(code), raw); err != nil {
			t.Fatalf("original%02x rejected: %v", code, err)
		}
		total++
	}
	if total != 134 {
		t.Fatalf("validated%d original WAVs", total)
	}
	t.Log("All134 original finite WAV file/PCM hashes, captured format, sample counts and untrimmed onsets verified")
}
