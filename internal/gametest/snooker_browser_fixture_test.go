package gametest_test

import (
	"github.com/k0ngk0ng/wire-board/internal/server"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnookerBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_SNOOKER_BROWSER") != "1" {
		t.Skip("explicit real-browser acceptance only")
	}
	root, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local/snooker/browser")
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	s, e := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "snooker-browser", AdminUsername: "斯诺克验收管理员", AdminPassword: "snooker-browser-test-only", AssetsBaseURL: os.Getenv("WIRE_BOARD_SNOOKER_ASSETS"), Modules: snookerBrowserModules()}, os.DirFS(filepath.Join(root, "web/dist")))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer srv.Close()
	go srv.Serve(listener)
	if e = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); e != nil {
		t.Fatal(e)
	}
	for start := time.Now(); time.Since(start) < 20*time.Minute; {
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("snooker browser acceptance exceeded its deadline")
}
