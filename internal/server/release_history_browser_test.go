package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReleaseHistoryBrowser(t *testing.T) {
	dir := os.Getenv("WIRE_BOARD_RELEASE_HISTORY_BROWSER")
	if dir == "" {
		t.Skip("browser acceptance is opt-in")
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "release-history-test", Version: "v1.59.0"}, os.DirFS(filepath.Join("..", "..", "web", "dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	if err := os.WriteFile(filepath.Join(dir, "url"), []byte(ts.URL), 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		if result, err := os.ReadFile(filepath.Join(dir, "done")); err == nil {
			if string(result) != "passed" {
				t.Fatal("browser acceptance failed")
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("browser acceptance timed out")
}
