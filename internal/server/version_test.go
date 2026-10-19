package server

import (
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStateExposesSanitizedVersion(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"v1.21.1", "v1.21.1"},
		{"", "dev"},
		{"bad version\n", "dev"},
		{"<script>", "dev"},
	} {
		s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Version: tc.raw}, fstest.MapFS{"index.html": {Data: []byte("version")}})
		if err != nil {
			t.Fatal(err)
		}
		ts := httptest.NewServer(s.Handler())
		c := newClient(t, ts.URL)
		c.register("VersionPlayer")
		if got := c.state()["version"]; got != tc.want {
			t.Fatalf("version %q: got %v want %v", tc.raw, got, tc.want)
		}
		ts.Close()
		s.Close()
	}
}
