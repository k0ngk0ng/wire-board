package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in browser fixture for the already-tested controlled Orient chain.
// Failure cases: wrong interactive choice, premature refill, lost continuation
// after reload, clipped confirmation controls, and missing acquisition flights.
// The catalog/inventory fixture is explicit; this is not a natural-game claim.
func TestSplendorOrientBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_ORIENT_BROWSER") != "1" {
		t.Skip("manual browser acceptance fixture")
	}
	s, ts, _, id := orientChainTable(t, os.Getenv("WIRE_BOARD_ORIENT_CITIES") == "1")
	s.files = os.DirFS(filepath.Join("..", "..", "web", "dist"))
	s.cfg.AssetsBaseURL = os.Getenv("ASSETS_BASE_URL")
	ts.Close()
	ts = httptest.NewServer(s.Handler())
	defer ts.Close()
	dir := filepath.Join("..", "..", ".local")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(dir, "orient-browser.json")
	stop := filepath.Join(dir, "orient-browser-stop")
	_ = os.Remove(stop)
	raw, err := json.Marshal(map[string]any{"url": ts.URL, "room": id, "name": s.rooms[id].Seats[0].Name, "password": "test-password-123"})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(ready, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(ready); _ = os.Remove(stop) })
	t.Log("fixture ready; finish by creating", stop)
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(stop); err == nil {
			var result struct {
				Passed   bool   `json:"passed"`
				Artifact string `json:"artifact"`
			}
			if err := json.Unmarshal(raw, &result); err != nil || !result.Passed {
				t.Fatalf("browser acceptance incomplete; artifact=%s error=%v", result.Artifact, err)
			}
			t.Log("browser acceptance passed", result.Artifact)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("browser acceptance fixture timed out")
}
