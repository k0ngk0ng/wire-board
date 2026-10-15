package racing_test

// Real browser fixture: production registry, HTTP/auth, SQLite and host timers.
// No debug actions, direct race-state writes, accelerated clock or fake results.
import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestRacingBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_RACING_BROWSER") != "1" {
		t.Skip("explicit real-browser acceptance only")
	}
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local/racing/browser")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	s, err := server.New(server.Config{
		DataDir: t.TempDir(), InviteCode: "racing-browser", AdminUsername: "赛车验收管理员",
		AdminPassword: "racing-browser-test-only", AssetsBaseURL: os.Getenv("WIRE_BOARD_RACING_ASSETS"), Modules: games.Modules(),
	}, os.DirFS(filepath.Join(root, "web/dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer srv.Close()
	go srv.Serve(listener)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for start := time.Now(); time.Since(start) < 24*time.Minute; {
		if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("racing browser acceptance exceeded its deadline")
}
