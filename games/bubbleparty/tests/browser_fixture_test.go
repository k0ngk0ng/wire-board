package bubbleparty_test

// Real browser fixture: production registry, HTTP/auth, SQLite and host timers.
import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/internal/nettest"
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
	latencyMS, _ := strconv.Atoi(os.Getenv("WIRE_BOARD_REALTIME_RTT_MS"))
	if latencyMS < 0 || latencyMS > 1000 {
		t.Fatal("test RTT must be 0..1000ms")
	}
	// Opt-in test-only transport fault: close actual hijacked TCP sockets.
	// This endpoint is compiled only into this local acceptance fixture.
	control := os.Getenv("WIRE_BOARD_REALTIME_NETWORK_CONTROL") == "1"
	var connMu sync.Mutex
	connections := map[net.Conn]bool{}
	drop := func() int {
		connMu.Lock()
		list := []net.Conn{}
		for c := range connections {
			list = append(list, c)
			delete(connections, c)
		}
		connMu.Unlock()
		for _, c := range list {
			_ = c.Close()
		}
		return len(list)
	}
	defer drop()
	delayed := nettest.Delay(s.Handler(), time.Duration(latencyMS)*time.Millisecond)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if control && r.Method == "POST" && r.URL.Path == "/_test/disconnect" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]int{"closed": drop()})
			return
		}
		delayed.ServeHTTP(w, r)
	})
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ConnState: func(c net.Conn, state http.ConnState) {
		if !control {
			return
		}
		connMu.Lock()
		defer connMu.Unlock()
		if state == http.StateHijacked {
			connections[c] = true
		} else if state == http.StateClosed {
			delete(connections, c)
		}
	}}
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
