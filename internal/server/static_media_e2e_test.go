package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// Failure inventory: trimmed media breaks relative game URLs; existing local
// payloads bypass CDN policy; redirects cache another release; invalid metadata
// permits arbitrary origins or traversal. Local development still serves files.
func TestStaticMediaCDNRedirectHTTP(t *testing.T) {
	base := "https://cdn.ichenj.com/bga/web/" + strings.Repeat("a", 40) + "/"
	name := "/snooker/v1/ball.webp"
	manifest := map[string]any{"version": 1, "assetBase": base, "files": map[string]any{name: map[string]any{"url": base + strings.TrimPrefix(name, "/"), "bytes": 4, "sha256": strings.Repeat("b", 64), "mime": "image/webp"}}}
	raw, _ := json.Marshal(manifest)
	for _, local := range []bool{false, true} {
		files := fstest.MapFS{"index.html": {Data: []byte("home")}, "cdn-assets-manifest.json": {Data: raw}}
		if local {
			files[strings.TrimPrefix(name, "/")] = &fstest.MapFile{Data: []byte("must not serve local media")}
		}
		ts := httptest.NewServer(staticFiles(files))
		client := ts.Client()
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		for _, method := range []string{"GET", "HEAD"} {
			req, _ := http.NewRequest(method, ts.URL+name+"?discard=query", nil)
			req.Header.Set("Range", "bytes=0-1")
			response, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode != http.StatusTemporaryRedirect || response.Header.Get("Location") != base+strings.TrimPrefix(name, "/") || response.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("media redirect: status=%d headers=%v", response.StatusCode, response.Header)
			}
			if strings.Contains(string(body), "must not serve") || method == "HEAD" && len(body) > 0 {
				t.Fatal("local payload or HEAD body leaked")
			}
		}
		ts.Close()
	}
}

func TestStaticMediaInvalidManifestFailsClosedHTTP(t *testing.T) {
	for _, raw := range []string{`broken`, `{"version":1,"assetBase":"https://evil.invalid/","files":{}}`} {
		ts := httptest.NewServer(staticFiles(fstest.MapFS{"cdn-assets-manifest.json": {Data: []byte(raw)}, "game/image.webp": {Data: []byte("private fallback")}}))
		response, err := ts.Client().Get(ts.URL + "/game/image.webp")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		ts.Close()
		if response.StatusCode != http.StatusServiceUnavailable || strings.Contains(string(body), "private fallback") || response.Header.Get("Location") != "" {
			t.Fatal("invalid asset metadata fell through", response.StatusCode, string(body))
		}
	}
}

// The client follows the production Location, while a test transport routes
// only the final CDN request to a real local HTTP range server.
type mediaRoundTripper func(*http.Request) (*http.Response, error)

func (f mediaRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestStaticMediaAudioRangeAfterRedirectHTTP(t *testing.T) {
	body := []byte("RIFFverified-wave-data")
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bga/web/"+strings.Repeat("a", 40)+"/audio/tone.wav" {
			t.Error("changed CDN path", r.URL.Path)
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.ServeContent(w, r, "tone.wav", time.Time{}, bytes.NewReader(body))
	}))
	defer cdn.Close()
	base := "https://cdn.ichenj.com/bga/web/" + strings.Repeat("a", 40) + "/"
	raw, _ := json.Marshal(deliveredAssets{Version: 1, AssetBase: base, Files: map[string]deliveredAsset{"/audio/tone.wav": {URL: base + "audio/tone.wav", Bytes: int64(len(body)), SHA256: strings.Repeat("b", 64), MIME: "audio/wav"}}})
	app := httptest.NewServer(staticFiles(fstest.MapFS{"cdn-assets-manifest.json": {Data: raw}}))
	defer app.Close()
	target, _ := url.Parse(cdn.URL)
	client := app.Client()
	original := client.Transport
	client.Transport = mediaRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "cdn.ichenj.com" {
			r = r.Clone(r.Context())
			u := *r.URL
			u.Scheme = target.Scheme
			u.Host = target.Host
			r.URL = &u
			r.Host = target.Host
		}
		return original.RoundTrip(r)
	})
	request, _ := http.NewRequest("GET", app.URL+"/audio/tone.wav", nil)
	request.Header.Set("Range", "bytes=0-3")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 206 || string(got) != "RIFF" || response.Header.Get("Content-Range") != "bytes 0-3/22" || response.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("range response lost", response.StatusCode, response.Header, string(got))
	}
}

func TestStaticMediaDevelopmentAndUnknownPathsHTTP(t *testing.T) {
	local := httptest.NewServer(staticFiles(fstest.MapFS{"game/a.webp": {Data: []byte("local image")}}))
	defer local.Close()
	response, err := local.Client().Get(local.URL + "/game/a.webp")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || string(body) != "local image" {
		t.Fatal("development assets unavailable")
	}
	base := "https://cdn.ichenj.com/bga/web/" + strings.Repeat("a", 40) + "/"
	manifest := deliveredAssets{Version: 1, AssetBase: base, Files: map[string]deliveredAsset{"/known.webp": {URL: base + "known.webp", Bytes: 3, SHA256: strings.Repeat("b", 64), MIME: "image/webp"}}}
	raw, _ := json.Marshal(manifest)
	app := httptest.NewServer(staticFiles(fstest.MapFS{"cdn-assets-manifest.json": {Data: raw}, "unknown.webp": {Data: []byte("unverified image")}}))
	defer app.Close()
	response, err = app.Client().Get(app.URL + "/unknown.webp")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 404 || strings.Contains(string(body), "unverified image") {
		t.Fatal("unlisted payload served")
	}
	for _, name := range []string{"/../escape.webp", "/.private.webp", "/index.html", "/known.webp"} {
		broken := manifest
		target := base + strings.TrimPrefix(name, "/")
		if name == "/known.webp" {
			target = "https://invalid.test/known.webp"
		}
		broken.Files = map[string]deliveredAsset{name: {URL: target, Bytes: 3, SHA256: strings.Repeat("b", 64), MIME: "image/webp"}}
		raw, _ = json.Marshal(broken)
		bad := httptest.NewServer(staticFiles(fstest.MapFS{"cdn-assets-manifest.json": {Data: raw}, "known.webp": {Data: []byte("unverified image")}}))
		response, err = bad.Client().Get(bad.URL + "/known.webp")
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		bad.Close()
		if response.StatusCode != 503 {
			t.Fatal("invalid redirect metadata accepted", name, response.StatusCode)
		}
	}
}
