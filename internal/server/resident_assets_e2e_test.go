package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func residentFiles(t *testing.T, worker any, mimeType string) fstest.MapFS {
	t.Helper()
	manifest, err := json.Marshal(map[string]any{"worker": worker})
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := json.Marshal(map[string]any{"version": 1, "assetBase": "https://assets.example.test/bga/", "files": map[string]any{"/assets/resident-worker-qwertyui.js": map[string]any{"url": "https://assets.example.test/bga/_common/resident-worker-qwertyui.js", "bytes": 17, "sha256": strings.Repeat("1", 64), "mime": mimeType}}})
	if err != nil {
		t.Fatal(err)
	}
	return fstest.MapFS{"index.html": {Data: []byte("home")}, "game-load-manifest.json": {Data: manifest}, "cdn-assets-manifest.json": {Data: delivery}}
}

func TestResidentWorkerEntryUsesVerifiedCDNCodeOnly(t *testing.T) {
	files := residentFiles(t, "https://assets.example.test/bga/_common/resident-worker-qwertyui.js", "text/javascript")
	site := httptest.NewServer(staticFiles(files))
	defer site.Close()
	response, err := http.Get(site.URL + "/asset-cache-worker.js?build=current")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || string(body) != "importScripts(\"https://assets.example.test/bga/_common/resident-worker-qwertyui.js\");\n" {
		t.Fatal(response.StatusCode, string(body))
	}
	if response.Header.Get("Service-Worker-Allowed") != "/" || response.Header.Get("Cache-Control") != "no-cache" || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/javascript") {
		t.Fatal(response.Header)
	}
	for _, method := range []string{http.MethodHead, http.MethodPost} {
		request, err := http.NewRequest(method, site.URL+"/asset-cache-worker.js", nil)
		if err != nil {
			t.Fatal(err)
		}
		result, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(result.Body)
		result.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if method == http.MethodHead && (result.StatusCode != 200 || len(content) != 0) {
			t.Fatal(result.StatusCode, string(content))
		}
		if method == http.MethodPost && result.StatusCode != 405 {
			t.Fatal(result.StatusCode)
		}
	}
	response, err = http.Get(site.URL + "/bga/asset-cache-worker.js")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatal(response.StatusCode)
	}
}

func TestResidentWorkerEntryRejectsUnboundMissingAndWrongMIME(t *testing.T) {
	for _, row := range []struct {
		name     string
		worker   any
		mimeType string
	}{{"missing", "", "text/javascript"}, {"invalid-type", 17, "text/javascript"}, {"outside", "https://outside.example.test/worker.js", "text/javascript"}, {"wrong-mime", "https://assets.example.test/bga/_common/resident-worker-qwertyui.js", "image/png"}} {
		t.Run(row.name, func(t *testing.T) {
			site := httptest.NewServer(staticFiles(residentFiles(t, row.worker, row.mimeType)))
			defer site.Close()
			response, err := http.Get(site.URL + "/asset-cache-worker.js")
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusServiceUnavailable {
				t.Fatal(response.StatusCode)
			}
		})
	}
}
