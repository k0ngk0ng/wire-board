package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

type deliveredAsset struct {
	URL    string `json:"url"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
	MIME   string `json:"mime"`
}
type deliveredAssets struct {
	Version   int                       `json:"version"`
	AssetBase string                    `json:"assetBase"`
	Files     map[string]deliveredAsset `json:"files"`
}

var immutableMediaBase = regexp.MustCompile(`^https://cdn\.ichenj\.com/bga/web/[a-f0-9]{40}/$`)
var mediaPath = regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`)
var mediaHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func serverMetadataPath(name string) bool {
	switch name {
	case "/", "/index.html", "/game-load-manifest.json", "/contra-delivery.json", "/cdn-assets-manifest.json":
		return true
	}
	return false
}

// Absence is the local-development mode. A present production manifest must be
// valid in full; never silently fall back to local media on malformed metadata.
func readDeliveredAssets(files fs.FS) (*deliveredAssets, error) {
	raw, err := fs.ReadFile(files, "cdn-assets-manifest.json")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var manifest deliveredAssets
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&manifest); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF || manifest.Version != 1 || !immutableMediaBase.MatchString(manifest.AssetBase) || len(manifest.Files) == 0 {
		return nil, errors.New("invalid CDN asset manifest")
	}
	for name, asset := range manifest.Files {
		if !mediaPath.MatchString(name) || path.Clean(name) != name || serverMetadataPath(name) || asset.URL != manifest.AssetBase+strings.TrimPrefix(name, "/") || asset.Bytes <= 0 || !mediaHash.MatchString(asset.SHA256) || asset.MIME == "" {
			return nil, errors.New("invalid CDN asset entry")
		}
		for _, part := range strings.Split(strings.TrimPrefix(name, "/"), "/") {
			if strings.HasPrefix(part, ".") {
				return nil, errors.New("private CDN asset path")
			}
		}
	}
	return &manifest, nil
}
