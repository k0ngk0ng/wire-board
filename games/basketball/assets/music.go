package assets

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// This is the same delivery manifest imported by the game frontend. It is
// metadata only; source recordings and the ROM are never embedded here.
//
//go:embed audio-playback.json
var playbackManifest []byte

func MusicRevision(id int) (string, error) {
	var manifest struct {
		Music map[string]struct {
			SHA            string `json:"distribution_sha256"`
			File           string `json:"file"`
			MIME           string `json:"mime_type"`
			Loop           bool   `json:"loop"`
			SampleRate     *int64 `json:"sample_rate"`
			LoopStart      *int64 `json:"loop_start_sample"`
			LoopEnd        *int64 `json:"loop_end_sample"`
			LoopSamples    *int64 `json:"loop_samples"`
			DecodedSamples *int64 `json:"decoded_samples"`
		} `json:"music"`
	}
	if err := json.Unmarshal(playbackManifest, &manifest); err != nil {
		return "", err
	}
	entry, ok := manifest.Music[strconv.Itoa(id)]
	if !ok {
		return "", fmt.Errorf("music %d has no delivery rendition", id)
	}
	if len(entry.SHA) != 64 {
		return "", fmt.Errorf("music %d has invalid delivery revision", id)
	}
	if _, err := hex.DecodeString(entry.SHA); err != nil {
		return "", err
	}
	if id == 76 || entry.LoopStart != nil {
		fields := []*int64{entry.SampleRate, entry.LoopStart, entry.LoopEnd, entry.LoopSamples, entry.DecodedSamples}
		for _, v := range fields {
			if v == nil || *v <= 0 || *v > 1<<53-1 {
				return "", fmt.Errorf("music %d has invalid loop sample metadata", id)
			}
		}
		if !entry.Loop || entry.MIME != "audio/mpeg" || !strings.HasSuffix(entry.File, ".mp3") || *entry.SampleRate != 44100 || *entry.LoopStart >= *entry.LoopEnd || *entry.LoopEnd != *entry.DecodedSamples || *entry.LoopEnd-*entry.LoopStart != *entry.LoopSamples {
			return "", fmt.Errorf("music %d has inconsistent loop delivery metadata", id)
		}
	}
	return entry.SHA, nil
}
