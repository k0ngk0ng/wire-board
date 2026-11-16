package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPublicReleaseHistoryRoutesHTTP(t *testing.T) {
	for _, cdn := range []bool{false, true} {
		t.Run(fmt.Sprint(cdn), func(t *testing.T) {
			files := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>Release history</title>")}}
			if cdn {
				body := []byte("export const release = true")
				manifest := map[string]any{"version": 1, "assetBase": "https://cdn.ichenj.com/bga/web/" + strings.Repeat("a", 40) + "/", "files": map[string]any{"/assets/history-abcdefgh.js": map[string]any{"url": "https://cdn.ichenj.com/bga/web/" + strings.Repeat("a", 40) + "/assets/history-abcdefgh.js", "mime": "text/javascript", "bytes": len(body), "sha256": fmt.Sprintf("%x", sha256.Sum256(body))}}}
				raw, err := json.Marshal(manifest)
				if err != nil {
					t.Fatal(err)
				}
				files["cdn-assets-manifest.json"] = &fstest.MapFile{Data: raw}
			}
			s, err := New(Config{DataDir: t.TempDir(), InviteCode: "release-route-test"}, files)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			ts := httptest.NewServer(s.Handler())
			defer ts.Close()
			for _, path := range []string{"/releases", "/releases/", "/releases/v1.59.0", "/releases/v999.0.0"} {
				for _, method := range []string{http.MethodGet, http.MethodHead} {
					req, err := http.NewRequest(method, ts.URL+path, nil)
					if err != nil {
						t.Fatal(err)
					}
					response, err := http.DefaultClient.Do(req)
					if err != nil {
						t.Fatal(err)
					}
					body, err := io.ReadAll(response.Body)
					response.Body.Close()
					if err != nil || response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" || !strings.Contains(response.Header.Get("Content-Type"), "text/html") {
						t.Fatalf("%s %s status=%d headers=%v error=%v", method, path, response.StatusCode, response.Header, err)
					}
					if method == http.MethodGet && !strings.Contains(string(body), "Release history") || method == http.MethodHead && len(body) != 0 {
						t.Fatalf("%s %s unexpected body %q", method, path, body)
					}
				}
			}
			response, err := http.Get(ts.URL + "/releases-other")
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusNotFound {
				t.Fatal("unrelated path must remain missing", response.StatusCode)
			}
		})
	}
}
