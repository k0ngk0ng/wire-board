package server

// Failure cases: version checks require authentication or read room state;
// release version isn't paired with its actual entry assets; a cache hides a
// deployment; unrelated external assets/configuration leak into the response.
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPublicVersionMetadataHTTP(t *testing.T) {
	files := fstest.MapFS{"index.html": {Data: []byte(`<script type="module" crossorigin src="/assets/app-old.js"></script><link rel="stylesheet" href="/assets/app-old.css"><link rel="icon" href="/favicon.svg">`)}}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "version-test", Version: "v1.27.1"}, files)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	for _, names := range [][]string{{"/assets/app-old.css", "/assets/app-old.js"}, {"/assets/app-new.css", "/assets/app-new.js"}} {
		response, err := http.Get(ts.URL + "/api/version")
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Version string   `json:"version"`
			Assets  []string `json:"assets"`
		}
		err = json.NewDecoder(response.Body).Decode(&got)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || got.Version != "v1.27.1" || !reflect.DeepEqual(got.Assets, names) {
			t.Fatalf("metadata: %+v status=%d err=%v", got, response.StatusCode, err)
		}
		if response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("version response may be cached")
		}
		files["index.html"].Data = []byte(`<script type="module" src="/assets/app-new.js"></script><link rel="stylesheet" href="/assets/app-new.css">`)
	}
}

func TestPublicVersionCDNAssetsHTTP(t *testing.T) {
	const base = "https://cdn.ichenj.com/bga/web/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/assets/"
	files := fstest.MapFS{"index.html": {Data: []byte(`<script type="module" src="` + base + `index-abcdefgh.js"></script><link rel="stylesheet" href="` + base + `index-abcdefgh.css"><img src="https://unrelated.example/avatar.jpg">`)}}
	s, e := New(Config{DataDir: t.TempDir(), InviteCode: "cdn-version-test", Version: "v2"}, files)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/version", nil))
	var got struct {
		Assets    []string `json:"assets"`
		AssetURLs []string `json:"assetURLs"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got.Assets, []string{"/assets/index-abcdefgh.css", "/assets/index-abcdefgh.js"}) || !reflect.DeepEqual(got.AssetURLs, []string{base + "index-abcdefgh.css", base + "index-abcdefgh.js"}) {
		t.Fatal(got.Assets)
	}
}

// External compiled modules must pass CSP without trusting arbitrary image hosts.
func TestFrontendCDNPolicyHTTP(t *testing.T) {
	s, e := New(Config{DataDir: t.TempDir(), InviteCode: "cdn-csp-test", AssetsBaseURL: "https://images.example.test/media"}, fstest.MapFS{"index.html": {Data: []byte("page")}})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	csp := w.Header().Get("Content-Security-Policy")
	for _, part := range []string{"script-src 'self' 'wasm-unsafe-eval' https://cdn.ichenj.com;", "style-src 'self' 'unsafe-inline' https://cdn.ichenj.com;", "font-src 'self' https://cdn.ichenj.com;"} {
		if !strings.Contains(csp, part) {
			t.Fatal(csp)
		}
	}
	if strings.Contains(csp, "script-src 'self' 'wasm-unsafe-eval' https://images.example.test") || strings.Contains(csp, "'unsafe-eval'") {
		t.Fatal("unexpected script trust", csp)
	}
}
