package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

func residentWorkerEntry(files fs.FS, delivery *deliveredAssets) ([]byte, error) {
	raw, err := fs.ReadFile(files, "game-load-manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Worker string `json:"worker"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	if manifest.Worker == "" || len(manifest.Worker) > 4096 {
		return nil, errors.New("missing resident worker entry")
	}
	found := false
	if delivery != nil {
		for _, asset := range delivery.Files {
			if asset.URL == manifest.Worker && (asset.MIME == "text/javascript" || asset.MIME == "application/javascript") {
				found = true
				break
			}
		}
	} else if strings.HasPrefix(manifest.Worker, "/assets/") && strings.HasSuffix(manifest.Worker, ".js") && path.Clean(manifest.Worker) == manifest.Worker {
		_, err = fs.Stat(files, strings.TrimPrefix(manifest.Worker, "/"))
		found = err == nil
	}
	if !found {
		return nil, errors.New("unbound resident worker entry")
	}
	return []byte("importScripts(" + strconv.Quote(manifest.Worker) + ");\n"), nil
}
