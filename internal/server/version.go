package server

import (
	"io/fs"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

var frontendAssetAttribute = regexp.MustCompile(`(?:src|href)=["']((?:https://cdn\.ichenj\.com/bga/(?:web/[a-f0-9]{40}/assets|[a-z_][a-z0-9_-]*)|/assets)/[^"']+\.(?:js|css))["']`)

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
	identities := make([]string, len(assets))
	for i, asset := range assets {
		identities[i] = "/assets/" + asset[strings.LastIndex(asset, "/")+1:]
	}
	sort.Strings(identities)
	respond(w, http.StatusOK, map[string]any{"version": s.cfg.Version, "assets": identities, "assetURLs": assets, "build": s.frontendBuild()})
}
