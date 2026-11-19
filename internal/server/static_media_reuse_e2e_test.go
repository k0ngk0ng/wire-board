package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticMediaHistoricalAndRenamedObjectsHTTP(t *testing.T) {
	current := "https://cdn.ichenj.com/bga/web/" + strings.Repeat("a", 40) + "/"
	previous := "https://cdn.ichenj.com/bga/web/" + strings.Repeat("b", 40) + "/images/original.webp"
	for _, target := range []string{previous, "https://evil.invalid/image.webp", current + "../secret", current + ".secret/token", previous + "?token=bad"} {
		manifest := map[string]any{"version": 1, "assetBase": current, "files": map[string]any{"/images/renamed.webp": map[string]any{"url": target, "bytes": 5, "sha256": strings.Repeat("c", 64), "mime": "image/webp"}}}
		raw, _ := json.Marshal(manifest)
		site := httptest.NewServer(staticFiles(fstest.MapFS{"index.html": {Data: []byte("home")}, "cdn-assets-manifest.json": {Data: raw}}))
		client := site.Client()
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		response, err := client.Get(site.URL + "/_media/images/renamed.webp")
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		site.Close()
		if target == previous {
			if response.StatusCode != http.StatusTemporaryRedirect || response.Header.Get("Location") != previous {
				t.Fatalf("historical identity lost: %d %v", response.StatusCode, response.Header)
			}
		} else if response.StatusCode != http.StatusServiceUnavailable || response.Header.Get("Location") != "" {
			t.Fatal("unsafe target accepted", target, response.StatusCode)
		}
	}
}

func TestStaticMediaCanonicalGameRouteHTTP(t *testing.T) {
	base := "https://cdn.ichenj.com/bga/"
	target := base + "splendor/" + strings.Repeat("a", 64) + ".webp"
	manifest := map[string]any{"version": 1, "assetBase": base, "files": map[string]any{"/splendor/cards.webp": map[string]any{"url": target, "bytes": 5, "sha256": strings.Repeat("a", 64), "mime": "image/webp"}}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	site := httptest.NewServer(staticFiles(fstest.MapFS{"cdn-assets-manifest.json": {Data: raw}}))
	defer site.Close()
	client := site.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Get(site.URL + "/bga/splendor/cards.webp")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusTemporaryRedirect || response.Header.Get("Location") != target {
		t.Fatalf("canonical game route: %d %v", response.StatusCode, response.Header)
	}
}

func TestStaticMediaExternalRegistryHTTP(t *testing.T) {
	base := "https://cdn.ichenj.com/bga/"
	target := base + "splendor/" + strings.Repeat("a", 64) + ".webp"
	manifest := map[string]any{"version": 1, "assetBase": base, "files": map[string]any{"/assets/app.js": map[string]any{"url": base + "_common/app-abcdefgh.js", "bytes": 5, "sha256": strings.Repeat("b", 64), "mime": "text/javascript"}}, "external": map[string]any{"/splendor/cards.webp": map[string]any{"url": target, "bytes": 5, "sha256": strings.Repeat("a", 64), "mime": "image/webp"}}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	site := httptest.NewServer(staticFiles(fstest.MapFS{"cdn-assets-manifest.json": {Data: raw}}))
	defer site.Close()
	client := site.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for _, path := range []string{"/bga/splendor/cards.webp", "/bga/splendor/" + strings.Repeat("a", 64) + ".webp"} {
		response, err := client.Get(site.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusTemporaryRedirect || response.Header.Get("Location") != target {
			t.Fatalf("external registry: %d %v", response.StatusCode, response.Header)
		}
	}
}

func TestStaticMediaConfiguredOriginHTTP(t *testing.T) {
	for _, origin := range []string{"https://cdn.example.invalid", "https://mirror.example.invalid"} {
		for _, foreign := range []bool{false, true} {
			base := origin + "/bga/"
			target := base + "splendor/" + strings.Repeat("a", 64) + ".webp"
			if foreign {
				target = "https://foreign.example.invalid/bga/splendor/" + strings.Repeat("a", 64) + ".webp"
			}
			raw, err := json.Marshal(deliveredAssets{Version: 1, AssetBase: base, Files: map[string]deliveredAsset{"/splendor/cards.webp": {URL: target, Bytes: 5, SHA256: strings.Repeat("a", 64), MIME: "image/webp"}}})
			if err != nil {
				t.Fatal(err)
			}
			site := httptest.NewServer(staticFiles(fstest.MapFS{"cdn-assets-manifest.json": {Data: raw}}))
			client := site.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			response, err := client.Get(site.URL + "/bga/splendor/cards.webp")
			if err != nil {
				site.Close()
				t.Fatal(err)
			}
			response.Body.Close()
			site.Close()
			expected := http.StatusTemporaryRedirect
			if foreign {
				expected = http.StatusServiceUnavailable
			}
			if response.StatusCode != expected || (!foreign && response.Header.Get("Location") != target) {
				t.Fatalf("configured origin: %d %v", response.StatusCode, response.Header)
			}
		}
	}
}
