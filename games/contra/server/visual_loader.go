package contra

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"strings"

	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
)

// LoadVisualResources verifies and freezes the exact normalized browser pixels.
// This is a setup-only reader for builders and hosts with an explicit fs.FS.
// All filesystem reads finish before module construction or any room callback.
func LoadVisualResources(files fs.FS, raw []byte, build string) (*BotVisualResources, error) {
	type asset struct {
		engine.BotVisualAsset
		URL string `json:"url"`
	}
	var manifest struct {
		RulesBuildID string          `json:"rulesBuildID"`
		VisualsID    string          `json:"visualsID"`
		RGB          asset           `json:"rgb"`
		Pages        map[uint8]asset `json:"pages"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	if !validSHA(build) || manifest.RulesBuildID != build {
		return nil, errors.New("visual bundle engine identity differs")
	}
	declared := engine.BotVisualCatalogManifest{RGB: manifest.RGB.BotVisualAsset, Pages: map[uint8]engine.BotVisualAsset{}}
	for id, entry := range manifest.Pages {
		declared.Pages[id] = entry.BotVisualAsset
	}
	identity, err := engine.BotVisualCatalogIdentity(declared)
	if err != nil {
		return nil, err
	}
	if manifest.VisualsID != "" && manifest.VisualsID != identity {
		return nil, errors.New("visual catalogue identity differs")
	}
	read := func(entry asset) ([]byte, error) {
		u, err := url.Parse(entry.URL)
		if err != nil || u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || !strings.HasPrefix(entry.URL, "/") || entry.URL != u.Path {
			return nil, errors.New("fixture visual asset must be a local mounted path")
		}
		name := strings.TrimPrefix(u.Path, "/")
		if !fs.ValidPath(name) {
			return nil, errors.New("fixture visual asset escapes mounted directory")
		}
		file, err := files.Open(name)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, int64(entry.Bytes)+1))
		if err != nil {
			return nil, err
		}
		if len(data) != entry.Bytes {
			return nil, errors.New("fixture visual asset size differs")
		}
		return data, nil
	}
	rgb, err := read(manifest.RGB)
	if err != nil {
		return nil, err
	}
	pages := make(map[uint8][]byte, len(manifest.Pages))
	for id, entry := range manifest.Pages {
		pages[id], err = read(entry)
		if err != nil {
			return nil, err
		}
	}
	return NewBotVisualResources(declared, rgb, pages)
}
