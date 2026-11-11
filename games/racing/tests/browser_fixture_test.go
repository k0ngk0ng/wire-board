package racing_test

// Real browser fixture: production registry, HTTP/auth, SQLite and host timers.
// No debug actions, direct race-state writes, accelerated clock or fake results.
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

func TestRacingBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_RACING_BROWSER") != "1" {
		t.Skip("explicit real-browser acceptance only")
	}
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local/racing/browser")
	if selected := os.Getenv("WIRE_BOARD_RACING_BROWSER_OUT"); selected != "" {
		out, err = filepath.Abs(selected)
		if err != nil {
			t.Fatal(err)
		}
		relative, pathErr := filepath.Rel(root, out)
		if pathErr != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			t.Fatal("racing browser evidence must remain inside the repository")
		}
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	s, err := server.New(server.Config{
		DataDir: t.TempDir(), InviteCode: "racing-browser", AdminUsername: "赛车验收管理员",
		AdminPassword: "racing-browser-test-only", AssetsBaseURL: os.Getenv("WIRE_BOARD_RACING_ASSETS"), Modules: games.Modules(), ClientLoading: true,
	}, os.DirFS(filepath.Join(root, "web/dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	// These opt-in faults exist only in this test binary. Production routes,
	// clocks and gameplay remain the actual server implementations.
	control := os.Getenv("WIRE_BOARD_REALTIME_NETWORK_CONTROL") == "1"
	var connectionMu sync.Mutex
	connections := map[net.Conn]bool{}
	drop := func() int {
		connectionMu.Lock()
		list := make([]net.Conn, 0, len(connections))
		for c := range connections {
			list = append(list, c)
			delete(connections, c)
		}
		connectionMu.Unlock()
		for _, c := range list {
			_ = c.Close()
		}
		return len(list)
	}
	defer drop()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if control && r.Method == "POST" && r.URL.Path == "/_test/disconnect" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]int{"closed": drop()})
			return
		}
		if control && strings.HasPrefix(r.URL.Path, "/api/") {
			ms, _ := strconv.Atoi(r.Header.Get("X-Acceptance-RTT"))
			if ms > 0 && ms <= 1000 {
				nettest.Delay(s.Handler(), time.Duration(ms)*time.Millisecond).ServeHTTP(w, r)
				return
			}
		}
		s.Handler().ServeHTTP(w, r)
	})
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ConnState: func(c net.Conn, state http.ConnState) {
		if !control {
			return
		}
		connectionMu.Lock()
		defer connectionMu.Unlock()
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
	for start := time.Now(); time.Since(start) < 24*time.Minute; {
		if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("racing browser acceptance exceeded its deadline")
}
