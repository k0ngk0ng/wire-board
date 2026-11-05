// Package audio validates the bounded finite-media metadata recovered from the
// original acquisition. It does not schedule, synthesize, mix or play audio.
package audio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	ROMSHA256           = "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06"
	SourceCatalogSHA256 = "617737f8ef07043401fa74f7f48ba551a64dd91cedd3a94dbfcf4883e0ff1541"
	SampleRate          = 44100
)

type Kind string

const (
	KindPCM      Kind = "pcm-sample-command"
	KindFMPSG    Kind = "finite-fm-psg-sequence"
	KindMusicCue Kind = "finite-music-cue"
)

// Clip is a value copy. Its origin includes the original queue-to-sound delay;
// FirstNonzeroStereoFrame is an observation, not an instruction to trim it.
type Clip struct {
	Code                    uint8  `json:"code"`
	Kind                    Kind   `json:"kind"`
	StereoFrames            uint64 `json:"stereo_frames"`
	WAVSHA256               string `json:"wav_sha256"`
	PCMSHA256               string `json:"pcm_sha256"`
	FirstNonzeroStereoFrame uint64 `json:"first_nonzero_stereo_frame"`
	Loop                    bool   `json:"loop"`
}

func (c Clip) DurationSeconds() float64 { return float64(c.StereoFrames) / SampleRate }

// MediaPath is relative to the caller's immutable asset base. No catalog field
// can substitute an arbitrary URL or local file. Excluded codes have no path.
func (c Clip) MediaPath() string {
	if finiteKind(c.Code) == "" {
		return ""
	}
	return fmt.Sprintf("yuyuhakusho/audio/%02x.wav", c.Code)
}

type catalogData struct {
	clips     [256]Clip
	present   [256]bool
	animation [11][]uint8
}

// Catalog is immutable after successful decoding. The zero value is empty.
type Catalog struct{ data *catalogData }

func finiteKind(code uint8) Kind {
	switch {
	case code >= 6 && code <= 126:
		return KindPCM
	case code >= 160 && code <= 171:
		return KindFMPSG
	case code == 143:
		return KindMusicCue
	default:
		return ""
	}
}

func lowerSHA(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// uniqueJSON checks members before encoding/json can replace duplicate keys.
func uniqueJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var consume func(int) error
	consume = func(depth int) error {
		if depth > 32 {
			return errors.New("finite catalog JSON is too deep")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]bool)
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return errors.New("duplicate or invalid finite catalog JSON key")
				}
				seen[name] = true
				if err := consume(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := consume(depth + 1); err != nil {
					return err
				}
			}
		default:
			return errors.New("unexpected finite catalog JSON delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := consume(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("finite catalog contains trailing JSON")
	}
	return nil
}

func strictObject(raw []byte, fields ...string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if len(object) != len(fields) {
		return nil, errors.New("finite catalog object has missing or unknown fields")
	}
	for _, field := range fields {
		value, present := object[field]
		if !present || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("missing or null finite catalog field %s", field)
		}
	}
	return object, nil
}

// DecodeCatalog validates structure, fixed identities and the complete finite
// code/AN registry. Callers must authenticate raw using a trusted immutable
// manifest SHA: these identity fields do not authenticate altered clip hashes,
// sample counts or onsets by themselves. ValidateWAV then checks actual media.
func DecodeCatalog(raw []byte) (Catalog, error) {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return Catalog{}, errors.New("invalid finite catalog size")
	}
	if err := uniqueJSON(raw); err != nil {
		return Catalog{}, err
	}
	root, err := strictObject(raw, "schema", "rom_sha256", "source_catalog_sha256", "format", "source_origin", "clips", "animation_bindings")
	if err != nil {
		return Catalog{}, err
	}
	var header struct {
		Schema int    `json:"schema"`
		ROM    string `json:"rom_sha256"`
		Source string `json:"source_catalog_sha256"`
		Origin string `json:"source_origin"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return Catalog{}, err
	}
	if header.Schema != 1 || header.ROM != ROMSHA256 || header.Source != SourceCatalogSHA256 || header.Origin != "isolated-capture-start" {
		return Catalog{}, errors.New("invalid finite catalog identity")
	}
	if _, err := strictObject(root["format"], "sample_rate", "channels", "sample_width_bytes", "pcm_encoding"); err != nil {
		return Catalog{}, err
	}
	var format struct {
		Rate     int    `json:"sample_rate"`
		Channels int    `json:"channels"`
		Width    int    `json:"sample_width_bytes"`
		Encoding string `json:"pcm_encoding"`
	}
	if err := json.Unmarshal(root["format"], &format); err != nil {
		return Catalog{}, err
	}
	if format.Rate != 44100 || format.Channels != 2 || format.Width != 2 || format.Encoding != "signed16-little-endian" {
		return Catalog{}, errors.New("unsupported finite audio format")
	}
	var clips []json.RawMessage
	if err := json.Unmarshal(root["clips"], &clips); err != nil {
		return Catalog{}, err
	}
	if len(clips) != 134 {
		return Catalog{}, errors.New("incomplete finite audio clip set")
	}
	data := new(catalogData)
	for _, encoded := range clips {
		if _, err := strictObject(encoded, "code", "kind", "stereo_frames", "wav_sha256", "pcm_sha256", "first_nonzero_stereo_frame", "loop"); err != nil {
			return Catalog{}, err
		}
		var clip Clip
		if err := json.Unmarshal(encoded, &clip); err != nil {
			return Catalog{}, err
		}
		if finiteKind(clip.Code) == "" || clip.Kind != finiteKind(clip.Code) || data.present[clip.Code] || clip.Loop || clip.StereoFrames == 0 || clip.StereoFrames > ((uint64(1)<<32)-1)/4 || clip.FirstNonzeroStereoFrame >= clip.StereoFrames || !lowerSHA(clip.WAVSHA256) || !lowerSHA(clip.PCMSHA256) {
			return Catalog{}, errors.New("invalid, duplicate or nonfinite audio clip")
		}
		data.clips[clip.Code], data.present[clip.Code] = clip, true
	}
	for code := 0; code < 256; code++ {
		if (finiteKind(uint8(code)) != "") != data.present[code] {
			return Catalog{}, errors.New("finite audio code set differs")
		}
	}
	var bindings []json.RawMessage
	if err := json.Unmarshal(root["animation_bindings"], &bindings); err != nil {
		return Catalog{}, err
	}
	if len(bindings) != 11 {
		return Catalog{}, errors.New("incomplete source-character animation references")
	}
	var seen [11]bool
	for _, encoded := range bindings {
		if _, err := strictObject(encoded, "character", "codes"); err != nil {
			return Catalog{}, err
		}
		var binding struct {
			Character uint8   `json:"character"`
			Codes     []uint8 `json:"codes"`
		}
		if err := json.Unmarshal(encoded, &binding); err != nil {
			return Catalog{}, err
		}
		if binding.Character >= 11 || seen[binding.Character] || !bytes.Equal(binding.Codes, sourceAnimationCodes[binding.Character]) {
			return Catalog{}, errors.New("invalid or conflicting source-character animation references")
		}
		seen[binding.Character] = true
		data.animation[binding.Character] = append([]uint8(nil), binding.Codes...)
	}
	return Catalog{data: data}, nil
}

func (c Catalog) Lookup(code uint8) (Clip, bool) {
	if c.data == nil || !c.data.present[code] {
		return Clip{}, false
	}
	return c.data.clips[code], true
}

// AnimationCodes returns only decoded animation references, in original source
// character numbering. It is not a complete gameplay or required preload set.
func (c Catalog) AnimationCodes(character uint8) ([]uint8, error) {
	if c.data == nil || character >= 11 {
		return nil, errors.New("unknown source character or empty finite catalog")
	}
	return append([]uint8(nil), c.data.animation[character]...), nil
}

// Exact139 AN references from the pinned legacy catalog;78 unique sound codes.
var sourceAnimationCodes = [11][]uint8{
	{0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x2a, 0x2b, 0x2c, 0x74, 0x75, 0x76, 0xa2, 0xa3, 0xa6},
	{0x16, 0x17, 0x18, 0x1a, 0x1c, 0x74, 0x75, 0x76, 0x7e, 0xa2, 0xa3, 0xa6},
	{0x0e, 0x0f, 0x10, 0x11, 0x14, 0x15, 0x74, 0x75, 0x76, 0xa2, 0xa3, 0xa6},
	{0x06, 0x07, 0x08, 0x09, 0x0a, 0x0c, 0x0d, 0x74, 0x75, 0x76, 0x7b, 0xa2, 0xa3, 0xa6},
	{0x1d, 0x1e, 0x1f, 0x20, 0x22, 0x74, 0x75, 0x76, 0xa2, 0xa3, 0xa6},
	{0x34, 0x35, 0x36, 0x37, 0x3a, 0x74, 0x75, 0x76, 0x7b, 0xa2, 0xa3, 0xa6},
	{0x50, 0x51, 0x52, 0x53, 0x54, 0x56, 0x57, 0x74, 0x75, 0x76, 0xa2, 0xa3, 0xa4, 0xa6},
	{0x4b, 0x4c, 0x4e, 0x4f, 0x74, 0x75, 0x76, 0xa2, 0xa3, 0xa6, 0xaa},
	{0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x33, 0x74, 0x75, 0x76, 0xa2, 0xa3, 0xa6},
	{0x3b, 0x3c, 0x3d, 0x3e, 0x40, 0x41, 0x74, 0x75, 0x76, 0xa2, 0xa3, 0xa6},
	{0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x49, 0x4a, 0x74, 0x75, 0x76, 0xa2, 0xa3, 0xa6},
}
