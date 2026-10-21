package server

// Failure cases: version checks require authentication or read room state;
// release version isn't paired with its actual entry assets; a cache hides a
// deployment; unrelated external assets/configuration leak into the response.
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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
