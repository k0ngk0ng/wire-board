package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPALWorkerContentPolicy(t *testing.T) {
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "pal-worker-test"}, fstest.MapFS{"index.html": {Data: []byte("local PAL")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	response := httptest.NewRecorder()
	s.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
	policy := response.Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, "worker-src 'self' blob:;") || strings.Contains(policy, "'unsafe-eval'") || strings.Contains(policy, "script-src 'self' blob:") {
		t.Fatal("local native worker policy is unavailable or expands script execution", policy)
	}
}
