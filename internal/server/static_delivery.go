package server

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var hashedAsset = regexp.MustCompile(`^/assets/[^/]+-[A-Za-z0-9_-]{8,}\.(js|css)$`)

func acceptsGzip(value string) bool {
	for _, item := range strings.Split(value, ",") {
		parts := strings.Split(strings.TrimSpace(item), ";")
		if parts[0] != "gzip" {
			continue
		}
		quality := 1.0
		for _, param := range parts[1:] {
			p := strings.SplitN(strings.TrimSpace(param), "=", 2)
			if len(p) == 2 && p[0] == "q" {
				q, e := strconv.ParseFloat(p[1], 64)
				if e != nil {
					return false
				}
				quality = q
			}
		}
		return quality > 0
	}
	return false
}
func staticFiles(files fs.FS) http.Handler {
	fallback := http.FileServerFS(files)
	delivery, deliveryErr := readDeliveredAssets(files)
	type cached struct {
		data []byte
		etag string
	}
	var cache sync.Map
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deliveryErr != nil {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "Asset delivery unavailable", http.StatusServiceUnavailable)
			return
		}
		if delivery != nil && !serverMetadataPath(r.URL.Path) {
			w.Header().Set("Cache-Control", "no-store")
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			} else if asset, ok := delivery.Files[r.URL.Path]; ok {
				http.Redirect(w, r, asset.URL, http.StatusTemporaryRedirect)
			} else {
				http.NotFound(w, r)
			}
			return
		}
		if serverMetadataPath(r.URL.Path) {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if !hashedAsset.MatchString(r.URL.Path) {
			fallback.ServeHTTP(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		zipped := false
		w.Header().Set("Vary", "Accept-Encoding")
		if acceptsGzip(r.Header.Get("Accept-Encoding")) {
			if _, e := fs.Stat(files, name+".gz"); e == nil {
				name += ".gz"
				zipped = true
			}
		}
		entry, ok := cache.Load(name)
		if !ok {
			data, e := fs.ReadFile(files, name)
			if e != nil {
				fallback.ServeHTTP(w, r)
				return
			}
			entry = cached{data, fmt.Sprintf(`"%x"`, sha256.Sum256(data))}
			cache.Store(name, entry)
		}
		c := entry.(cached)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("ETag", c.etag)
		w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(r.URL.Path)))
		if zipped {
			w.Header().Set("Content-Encoding", "gzip")
		}
		http.ServeContent(w, r, path.Base(r.URL.Path), time.Time{}, bytes.NewReader(c.data))
	})
}
