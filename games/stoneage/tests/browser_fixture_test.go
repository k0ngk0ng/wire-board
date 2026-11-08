package stoneage_test

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	stoneage "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestStoneageBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_STONEAGE_BROWSER") != "1" {
		t.Skip("explicit browser acceptance")
	}
	root, e := filepath.Abs("../../..")
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local/stoneage/browser")
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	s, e := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "stoneage-browser", AdminUsername: "石器验收", AdminPassword: "stoneage-browser-only", Modules: []gameapi.Module{stoneage.Module()}}, os.DirFS(filepath.Join(root, "web/dist")))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer srv.Close()
	go srv.Serve(l)
	if e = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+l.Addr().String()), 0600); e != nil {
		t.Fatal(e)
	}
	for start := time.Now(); time.Since(start) < 30*time.Minute; {
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			if e = os.Remove(filepath.Join(out, "stop")); e != nil {
				t.Fatal(e)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}
