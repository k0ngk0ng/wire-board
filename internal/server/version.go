package server

import (
	"io/fs"
	"net/http"
	"regexp"
	"sort"
)

var frontendAssetAttribute = regexp.MustCompile(`(?:src|href)=["']((?:https://cdn\.ichenj\.com/bga/web/[a-f0-9]{40})?/assets/[^"']+)["']`)

// Read only the small entry document, never rooms, game snapshots or SQLite.
// Asset identity detects a cached old page even if its first API response is
// already served by the new release. Release version also covers backend-only
// changes whose frontend bundle happens to remain identical.
func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	html, err := fs.ReadFile(s.files, "index.html")
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "版本信息暂不可用")
		return
	}
	assets := []string{}
	seen := map[string]bool{}
	for _, match := range frontendAssetAttribute.FindAllSubmatch(html, -1) {
		path := string(match[1])
		if !seen[path] {
			assets = append(assets, path)
			seen[path] = true
		}
	}
	sort.Strings(assets)
	respond(w, http.StatusOK, map[string]any{"version": s.cfg.Version, "assets": assets, "build": s.frontendBuild()})
}
