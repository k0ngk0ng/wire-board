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
	URL      string `json:"url"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
	MIME     string `json:"mime"`
	Encoding string `json:"encoding,omitempty"`
	Verified bool   `json:"verified,omitempty"`
}
type deliveredAssets struct {
	Version   int                       `json:"version"`
	SourceSHA string                    `json:"sourceSHA,omitempty"`
	AssetBase string                    `json:"assetBase"`
	Files     map[string]deliveredAsset `json:"files"`
	External  map[string]deliveredAsset `json:"external,omitempty"`
	Objects   map[string]deliveredAsset `json:"objects,omitempty"`
}

var immutableMediaBase = regexp.MustCompile(`^https://[A-Za-z0-9.-]+(?::[0-9]+)?/bga/(?:web/[a-f0-9]{40}/)?$`)
var immutableMediaURL = regexp.MustCompile(`^(https://[A-Za-z0-9.-]+(?::[0-9]+)?/bga/(?:web/[a-f0-9]{40}/)?)([A-Za-z0-9._/-]+)$`)
var mediaPath = regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`)
var mediaHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func serverMetadataPath(name string) bool {
	switch name {
	case "/", "/index.html", "/game-load-manifest.json", "/contra-delivery.json", "/cdn-assets-manifest.json":
		return true
	}
	return false
}

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
	for name, asset := range manifest.External {
		if previous, ok := manifest.Files[name]; ok && previous != asset {
			return nil, errors.New("conflicting external CDN asset")
		}
		manifest.Files[name] = asset
	}
	aliases := map[string]deliveredAsset{}
	for name, asset := range manifest.Files {
		if !mediaPath.MatchString(name) || path.Clean(name) != name || serverMetadataPath(name) || !validDeliveredAsset(asset, manifest.AssetBase) || asset.Bytes <= 0 || !mediaHash.MatchString(asset.SHA256) || asset.MIME == "" {
			return nil, errors.New("invalid CDN asset entry")
		}
		for _, part := range strings.Split(strings.TrimPrefix(name, "/"), "/") {
			if strings.HasPrefix(part, ".") {
				return nil, errors.New("private CDN asset path")
			}
		}
		if strings.Contains(asset.URL, "/bga/") {
			aliases["/"+strings.SplitN(asset.URL, "/bga/", 2)[1]] = asset
		}
	}
	for name, asset := range aliases {
		manifest.Files[name] = asset
	}
	for url, asset := range manifest.Objects {
		if url != asset.URL || !validDeliveredAsset(asset, manifest.AssetBase) {
			return nil, errors.New("invalid historical CDN object")
		}
	}
	return &manifest, nil
}

func validDeliveredAsset(asset deliveredAsset, base string) bool {
	parts := immutableMediaURL.FindStringSubmatch(asset.URL)
	origin := strings.SplitN(base, "/bga/", 2)[0]
	if len(parts) != 3 || !strings.HasPrefix(asset.URL, origin+"/bga/") || asset.Bytes <= 0 || !mediaHash.MatchString(asset.SHA256) || asset.MIME == "" || (asset.Encoding != "" && asset.Encoding != "gzip") {
		return false
	}
	name := "/" + parts[2]
	if !mediaPath.MatchString(name) || path.Clean(name) != name || serverMetadataPath(name) {
		return false
	}
	for _, part := range strings.Split(strings.TrimPrefix(name, "/"), "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}
