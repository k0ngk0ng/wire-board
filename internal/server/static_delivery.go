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
	workerEntry, workerErr := residentWorkerEntry(files, delivery)
	type cached struct {
		data []byte
		etag string
	}
	var cache sync.Map
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/_media/") || strings.HasPrefix(r.URL.Path, "/bga/") {
			request := r.Clone(r.Context())
			target := *r.URL
			target.Path = strings.TrimPrefix(target.Path, "/_media")
			target.Path = strings.TrimPrefix(target.Path, "/bga")
			target.RawPath = ""
			request.URL = &target
			r = request
			if serverMetadataPath(r.URL.Path) {
				http.NotFound(w, r)
				return
			}
		}

		if deliveryErr != nil {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "Asset delivery unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/asset-cache-worker.js" {
			w.Header().Set("Cache-Control", "no-cache")
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			} else if workerErr != nil {
				http.Error(w, "Resident worker entry unavailable", http.StatusServiceUnavailable)
			} else {
				w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
				w.Header().Set("Service-Worker-Allowed", "/")
				w.Header().Set("Content-Length", strconv.Itoa(len(workerEntry)))
				if r.Method != http.MethodHead {
					_, _ = w.Write(workerEntry)
				}
			}
			return
		}
		if r.URL.Path == "/releases" || strings.HasPrefix(r.URL.Path, "/releases/") {
			w.Header().Set("Cache-Control", "no-store")
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			html, err := fs.ReadFile(files, "index.html")
			if err != nil {
				http.Error(w, "Release history unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(html))
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
