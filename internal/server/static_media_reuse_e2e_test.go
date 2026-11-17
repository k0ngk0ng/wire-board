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
