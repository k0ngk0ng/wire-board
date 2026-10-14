package pairs_test

import (
	game "github.com/k0ngk0ng/wire-board/games/pairs/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPairsBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_PAIRS_BROWSER") != "1" {
		t.Skip("explicit browser acceptance")
	}
	root, e := filepath.Abs("../../..")
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local/pairs/browser")
	os.MkdirAll(out, 0755)
	s, e := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "pairs-browser", AdminUsername: "连连看验收", AdminPassword: "pairs-browser-only", AssetsBaseURL: os.Getenv("WIRE_BOARD_PAIRS_ASSETS"), Modules: []gameapi.Module{game.Module()}}, os.DirFS(filepath.Join(root, "web/dist")))
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
	os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600)
	until := time.Now().Add(12 * time.Minute)
	for time.Now().Before(until) {
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser acceptance timeout")
}
