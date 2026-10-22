package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// HTTP failure boundaries: gzip decoded content differs, stale HTML/manifest,
// compressed response poisons identity caches, HEAD body, conditional cache miss.
func TestStaticDeliveryHTTP(t *testing.T) {
	body := []byte("export const verified = true;")
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, _ = zw.Write(body)
	_ = zw.Close()
	files := fstest.MapFS{"index.html": {Data: []byte("home")}, "game-load-manifest.json": {Data: []byte(`{}`)}, "assets/index-abcdefgh.js": {Data: body}, "assets/index-abcdefgh.js.gz": {Data: compressed.Bytes()}}
	h := staticFiles(files)
	for _, encoding := range []string{"gzip", "identity", "gzip;q=0"} {
		req := httptest.NewRequest("GET", "/assets/index-abcdefgh.js", nil)
		req.Header.Set("Accept-Encoding", encoding)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		got := w.Body.Bytes()
		if encoding == "gzip" {
			if w.Header().Get("Content-Encoding") != "gzip" {
				t.Fatal("no gzip")
			}
			zr, e := gzip.NewReader(bytes.NewReader(got))
			if e != nil {
				t.Fatal(e)
			}
			got, _ = io.ReadAll(zr)
			_ = zr.Close()
		} else if w.Header().Get("Content-Encoding") != "" {
			t.Fatal("gzip not accepted")
		}
		if !bytes.Equal(got, body) || w.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || w.Header().Get("Vary") != "Accept-Encoding" {
			t.Fatal(w.Header(), string(got))
		}
		req.Header.Set("If-None-Match", w.Header().Get("ETag"))
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusNotModified {
			t.Fatal("no conditional cache", w.Code)
		}
	}
	for _, path := range []string{"/", "/index.html", "/game-load-manifest.json"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Header().Get("Cache-Control") != "no-cache" {
			t.Fatal("mutable entry cached", path, w.Header())
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("HEAD", "/assets/index-abcdefgh.js", nil))
	if w.Body.Len() != 0 {
		t.Fatal("HEAD body")
	}
}
