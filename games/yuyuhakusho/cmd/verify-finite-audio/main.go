// Command verify-finite-audio checks the complete catalog-to-original-media
// path without any MD core, synthesis, browser player or media publication.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"

	finite "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio"
)

type result struct {
	Pass        bool     `json:"pass"`
	Scope       string   `json:"scope"`
	GOOS        string   `json:"goos"`
	GOARCH      string   `json:"goarch"`
	CatalogSHA  string   `json:"catalog_sha256,omitempty"`
	ManifestSHA string   `json:"manifest_sha256,omitempty"`
	SourceSHA   string   `json:"source_catalog_sha256,omitempty"`
	Clips       int      `json:"validated_clips"`
	WAVBytes    uint64   `json:"validated_wav_bytes"`
	Characters  int      `json:"animation_characters"`
	ANUnique    int      `json:"animation_unique_codes"`
	Codes       []int    `json:"validated_codes,omitempty"`
	Error       string   `json:"error,omitempty"`
	Limits      []string `json:"limits"`
}

func sha(raw []byte) string { value := sha256.Sum256(raw); return hex.EncodeToString(value[:]) }

func run(catalogPath, manifestPath, manifestSHA, sourcePath, mediaRoot string) (r result, err error) {
	r = result{Scope: "Complete134 finite original WAVs, immutable catalog digest and source-character AN references only", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		Limits: []string{"Trusted manifest SHA must be supplied by the caller, not learned from untrusted adjacent metadata", "No sound timing, mixing, browser playback, BGM loop or publication acceptance"}}
	if len(manifestSHA) != 64 {
		return r, errors.New("a trusted immutable manifest SHA-256 is required")
	}
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		return r, err
	}
	r.ManifestSHA = sha(manifestRaw)
	if r.ManifestSHA != manifestSHA {
		return r, errors.New("immutable finite manifest SHA differs")
	}
	var manifest struct {
		Pass    bool
		Schema  int
		ROM     string `json:"rom_sha256"`
		Source  string `json:"source_catalog_sha256"`
		Catalog string `json:"catalog_sha256"`
		Bytes   int    `json:"catalog_bytes"`
		Clips   int
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return r, err
	}
	if !manifest.Pass || manifest.Schema != 1 || manifest.ROM != finite.ROMSHA256 || manifest.Source != finite.SourceCatalogSHA256 || manifest.Clips != 134 {
		return r, errors.New("finite manifest identity or coverage differs")
	}
	raw, err := os.ReadFile(catalogPath)
	if err != nil {
		return r, err
	}
	r.CatalogSHA = sha(raw)
	if r.CatalogSHA != manifest.Catalog || len(raw) != manifest.Bytes {
		return r, errors.New("finite catalog differs from immutable manifest")
	}
	catalog, err := finite.DecodeCatalog(raw)
	if err != nil {
		return r, err
	}
	sourceRaw, err := os.ReadFile(sourcePath)
	if err != nil {
		return r, err
	}
	r.SourceSHA = sha(sourceRaw)
	if r.SourceSHA != finite.SourceCatalogSHA256 {
		return r, errors.New("immutable original source catalog differs")
	}
	var source struct {
		Assets []struct {
			Code   uint8
			Kind   finite.Kind `json:"classification"`
			Frames uint64      `json:"stereo_frames"`
			WAV    string      `json:"wav_sha256"`
			PCM    string      `json:"pcm_sha256"`
			Onset  *uint64     `json:"first_nonzero_stereo_frame"`
		} `json:"assets"`
		Bindings []struct {
			Code       uint8
			Characters []uint8
		} `json:"animation_bindings"`
	}
	if err := json.Unmarshal(sourceRaw, &source); err != nil {
		return r, err
	}
	if len(source.Assets) != 134 {
		return r, errors.New("original finite source coverage differs")
	}
	for _, original := range source.Assets {
		clip, ok := catalog.Lookup(original.Code)
		onset := uint64(971)
		if original.Onset != nil {
			onset = *original.Onset
		}
		if !ok || clip.Kind != original.Kind || clip.StereoFrames != original.Frames || clip.WAVSHA256 != original.WAV || clip.PCMSHA256 != original.PCM || clip.FirstNonzeroStereoFrame != onset || clip.Loop {
			return r, fmt.Errorf("finite clip%02x differs from original acquisition", original.Code)
		}
		folder := "samples"
		if original.Code == 0x8f {
			folder = "music-excerpts"
		}
		path := filepath.Join(mediaRoot, "isolated", folder, fmt.Sprintf("%02x.wav", original.Code))
		wav, err := os.ReadFile(path)
		if err != nil {
			return r, err
		}
		if err := catalog.ValidateWAV(original.Code, wav); err != nil {
			return r, fmt.Errorf("original%02x media failed: %w", original.Code, err)
		}
		r.Clips++
		r.WAVBytes += uint64(len(wav))
		r.Codes = append(r.Codes, int(original.Code))
	}
	unique := make(map[uint8]bool)
	for character := uint8(0); character < 11; character++ {
		var expected []uint8
		for _, binding := range source.Bindings {
			for _, owner := range binding.Characters {
				if owner == character {
					expected = append(expected, binding.Code)
				}
			}
		}
		codes, err := catalog.AnimationCodes(character)
		if err != nil {
			return r, err
		}
		if !reflect.DeepEqual(codes, expected) {
			return r, fmt.Errorf("source character%d animation references differ", character)
		}
		for _, code := range codes {
			unique[code] = true
		}
		r.Characters++
	}
	r.ANUnique = len(unique)
	if r.Clips != 134 || r.Characters != 11 || r.ANUnique != 78 {
		return r, errors.New("incomplete finite catalog acceptance")
	}
	r.Pass = true
	return r, nil
}

func main() {
	catalog := flag.String("catalog", "games/yuyuhakusho/data/audio-finite-catalog.json", "finite runtime JSON")
	manifest := flag.String("manifest", "games/yuyuhakusho/data/audio-finite-catalog-manifest.json", "finite immutable manifest")
	manifestSHA := flag.String("manifest-sha256", "", "trusted immutable manifest SHA-256 (required)")
	source := flag.String("source-catalog", "games/yuyuhakusho/data/audio-assets.json", "pinned legacy acquisition metadata")
	media := flag.String("media-root", ".local/md-yuyuhakusho/audio-assets-final", "existing canonical original WAV tree")
	flag.Parse()
	report, err := run(*catalog, *manifest, *manifestSHA, *source, *media)
	if err != nil {
		report.Error = err.Error()
	}
	raw, marshalErr := json.MarshalIndent(report, "", "  ")
	if marshalErr != nil {
		fmt.Fprintln(os.Stderr, marshalErr)
		os.Exit(1)
	}
	fmt.Println(string(raw))
	if err != nil {
		os.Exit(1)
	}
}
