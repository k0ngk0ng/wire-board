package gametest_test

// Opt-in browser fixture serves the production registry/build and a fresh
// database. No forced game outcomes, private debug API or shipping test hooks.
import (
	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/internal/server"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPokerBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_POKER_BROWSER") != "1" {
		t.Skip("browser acceptance only")
	}
	root, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local/poker/browser")
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	s, e := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "poker-browser", AdminUsername: "扑克验收管理员", AdminPassword: "poker-browser-test-only", AssetsBaseURL: os.Getenv("WIRE_BOARD_POKER_ASSETS"), Modules: games.Modules()}, os.DirFS(filepath.Join(root, "web/dist")))
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
	for started := time.Now(); time.Since(started) < 20*time.Minute; {
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}
