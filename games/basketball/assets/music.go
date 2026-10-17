package assets

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
)

// This is the same delivery manifest imported by the game frontend. It is
// metadata only; source recordings and the ROM are never embedded here.
//
//go:embed audio-playback.json
var playbackManifest []byte

func MusicRevision(id int) (string, error) {
	var manifest struct {
		Music map[string]struct {
			SHA string `json:"distribution_sha256"`
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
	return entry.SHA, nil
}
