package bubbleparty_test

// Real browser fixture: production registry, HTTP/auth, SQLite and host timers.
import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestBubblepartyBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_BUBBLEPARTY_BROWSER") != "1" {
		t.Skip("explicit real-browser acceptance only")
	}
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	out := os.Getenv("WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT")
	if out == "" {
		out = filepath.Join(root, ".local/bubbleparty/browser")
	}
	out, err = filepath.Abs(out)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, out)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == "." {
		t.Fatal("browser output must be a dedicated directory inside the repository")
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(out, "url"))
	_ = os.Remove(filepath.Join(out, "stop"))
	if err = os.RemoveAll(filepath.Join(out, "data")); err != nil {
		t.Fatal(err)
	}
	s, err := server.New(server.Config{
		DataDir: filepath.Join(out, "data"), InviteCode: "bubbleparty-browser",
		AdminUsername: "泡泡验收管理员", AdminPassword: "bubbleparty-browser-test-only",
		Modules: games.Modules(),
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
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("bubbleparty browser acceptance exceeded its deadline")
}
