package server

// Failure inventory: a slow HTTP snapshot reader holds the platform mutex and
// stalls another authenticated user. A large fixture view forces real TCP write
// backpressure; no fake ResponseWriter or artificial lock is used.
import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestRealtimeSlowSnapshotReaderDoesNotBlockOtherUsers(t *testing.T) {
	module := realtimeFixture(true, time.Hour, 0, "")
	module.View = func(gameapi.Match, int) (any, error) {
		return map[string]any{"padding": strings.Repeat("x", 4<<20)}, nil
	}
	f := newRealtimeTable(t, module, false)
	other := newClient(t, f.ts.URL)
	other.register("慢连接隔离验收")
	f.ts.Close()
	f.ts = httptest.NewUnstartedServer(f.s.Handler())
	f.ts.Config.ConnState = func(c net.Conn, state http.ConnState) {
		if state == http.StateNew {
			if tcp, ok := c.(*net.TCPConn); ok {
				_ = tcp.SetWriteBuffer(1024)
			}
		}
	}
	f.ts.Start()
	f.a.base = f.ts.URL
	other.base = f.ts.URL
	endpoint, _ := url.Parse(f.ts.URL)
	conn, err := net.Dial("tcp", endpoint.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetReadBuffer(1024)
	}
	cookies := []string{}
	for _, cookie := range f.a.client.Jar.Cookies(endpoint) {
		cookies = append(cookies, cookie.Name+"="+cookie.Value)
	}
	_, err = fmt.Fprintf(conn, "GET /api/state HTTP/1.1\r\nHost: %s\r\nCookie: %s\r\nConnection: close\r\n\r\n", endpoint.Host, strings.Join(cookies, "; "))
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	// Stop consuming the 4MiB body; the first handler must remain write-blocked.
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, other.base+"/api/state", nil)
	start := time.Now()
	response, err := other.client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("slow snapshot receiver blocked independent authenticated state request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("independent request failed", response.StatusCode)
	}
	streamEvidence(t, "slow-http-reader", map[string]any{"passed": true, "blockedBodyBytes": 4 << 20, "otherRequestMS": float64(elapsed.Microseconds()) / 1000})
}

// A real WebSocket peer stops reading after the first frame byte. Its bounded
// kernel buffers force bootstrap write backpressure; concurrent actions must
// still commit in memory and the blocked socket must time out and unregister.
func TestRealtimeSlowWebSocketDoesNotBlockActions(t *testing.T) {
	module := realtimeFixture(true, time.Hour, 0, "")
	module.View = func(gameapi.Match, int) (any, error) {
		return map[string]any{"padding": strings.Repeat("w", 4<<20)}, nil
	}
	f := newRealtimeTable(t, module, false)
	other := newClient(t, f.ts.URL)
	other.register("慢流式连接隔离")
	f.ts.Close()
	f.ts = httptest.NewUnstartedServer(f.s.Handler())
	f.ts.Config.ConnState = func(c net.Conn, state http.ConnState) {
		if state == http.StateNew {
			if tcp, ok := c.(*net.TCPConn); ok {
				_ = tcp.SetWriteBuffer(1024)
			}
		}
	}
	f.ts.Start()
	f.a.base = f.ts.URL
	other.base = f.ts.URL
	endpoint, _ := url.Parse(f.ts.URL)
	conn, err := net.Dial("tcp", endpoint.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetReadBuffer(1024)
	}
	cookies := []string{}
	for _, cookie := range f.a.client.Jar.Cookies(endpoint) {
		cookies = append(cookies, cookie.Name+"="+cookie.Value)
	}
	_, err = fmt.Fprintf(conn, "GET /api/ws?stream=1 HTTP/1.1\r\nHost: %s\r\nCookie: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n", endpoint.Host, strings.Join(cookies, "; "))
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(status, " 101 ") {
		t.Fatal("websocket upgrade failed", status, err)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	if _, err = reader.ReadByte(); err != nil {
		t.Fatal("no websocket frame", err)
	}
	// Do not consume more data: both network buffers are deliberately tiny.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, other.base+"/api/state", nil)
	start := time.Now()
	res, err := other.client.Do(req)
	latency := time.Since(start)
	if err != nil {
		t.Fatal("blocked WS stalled another authenticated user", err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal(res.StatusCode)
	}
	f.s.mu.Lock()
	version := f.s.rooms[f.id].Version
	f.s.mu.Unlock()
	for i := 0; i < 12; i++ {
		response := f.a.post("/api/rooms/"+f.id, map[string]any{"type": "action", "nonce": fmt.Sprintf("slow-ws-action-%d", i), "version": version, "action": map[string]any{"drive": 1}}, 200)
		version = int(response["version"].(float64))
	}
	f.s.mu.Lock()
	liveVersion := f.s.rooms[f.id].Version
	f.s.mu.Unlock()
	if liveVersion != version {
		t.Fatal("actions did not advance behind blocked writer")
	}
	removed := false
	for until := time.Now().Add(7 * time.Second); time.Now().Before(until); {
		f.s.mu.Lock()
		removed = len(f.s.streamFrames) == 0
		f.s.mu.Unlock()
		if removed {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !removed {
		t.Fatal("blocked stream did not time out and release subscription")
	}
	streamEvidence(t, "slow-websocket-reader", map[string]any{"passed": true, "blockedFrameBytes": 4 << 20, "otherRequestMS": float64(latency.Microseconds()) / 1000, "acceptedActions": 12, "subscriptionRemoved": removed})
}
