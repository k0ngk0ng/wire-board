package platformtest

// Real TCP backpressure/seat isolation acceptance. No view padding, artificial
// lock or fake response writer is added to the actual game/host implementation.
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

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	game "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestRealSlowSpectatorDoesNotBlockFastPlayerOrClock(t *testing.T) {
	h := gametest.NewHTTP(t, 2, registered(t), "yuyuhakusho")
	h.Start(map[string]any{"characters": []int{3, 9}, "arena": 10})
	load(t, h)
	h.Server.Close()
	h.Server = httptest.NewUnstartedServer(h.Host.Handler())
	h.Server.Config.ConnState = func(c net.Conn, s http.ConnState) {
		if s == http.StateNew {
			if tcp, ok := c.(*net.TCPConn); ok {
				_ = tcp.SetWriteBuffer(1024)
			}
		}
	}
	h.Server.Start()
	for _, c := range append(append([]*gametest.Client{}, h.People...), h.Spectator) {
		c.Base = h.Server.URL
	}
	endpoint, _ := url.Parse(h.Server.URL)
	conn, err := net.Dial("tcp", endpoint.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetReadBuffer(1024)
	}
	cookies := []string{}
	for _, cookie := range h.Spectator.HTTP.Jar.Cookies(endpoint) {
		cookies = append(cookies, cookie.Name+"="+cookie.Value)
	}
	_, err = fmt.Fprintf(conn, "GET /api/ws?stream=1 HTTP/1.1\r\nHost: %s\r\nCookie: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n", endpoint.Host, strings.Join(cookies, "; "))
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(line, " 101 ") {
		t.Fatal("slow websocket failed real authenticated upgrade", line, err)
	}
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	if _, err = reader.ReadByte(); err != nil {
		t.Fatal(err)
	} // Leave the rest unread, including all future game frames.
	before := wire(t, h.People[0])
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", h.People[0].Base+"/api/state", nil)
	start := time.Now()
	response, err := h.People[0].HTTP.Do(req)
	if err != nil {
		t.Fatal("slow spectator blocked authenticated fast user", err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.StatusCode)
	}
	elapsed := time.Since(start)
	h.People[0].Action(input(before, 0, engine.ButtonRight|engine.ButtonZ), 200)
	after := wait(t, h.People[0], func(v game.View) bool { return v.Frame > before.Frame+120 })
	if after.Inputs[0].AppliedSequence < before.Inputs[0].Sequence+1 || after.Inputs[0].Automated {
		t.Fatal("fast input/neutral release stopped or changed to AI", after.Inputs[0])
	}
	if elapsed > time.Second {
		t.Fatal("slow peer delayed independent state request", elapsed)
	}
	h.Record(fmt.Sprintf("unread authenticated spectator socket;fast request %d microseconds;authority advanced%d frames", elapsed.Microseconds(), after.Frame-before.Frame))
}
