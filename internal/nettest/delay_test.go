package nettest

// Failure inventory: delay accumulates once per packet rather than preserving
// pipelining; byte order changes; connections leak on close; HTTP is delayed only
// on one leg. This uses a real TCP HTTP/WebSocket echo server, not a fake clock.
import (
	"context"
	"fmt"
	"github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDelayedWebSocketRemainsOrderedAndPipelined(t *testing.T) {
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			kind, b, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if c.Write(r.Context(), kind, b) != nil {
				return
			}
		}
	})
	s := httptest.NewServer(Delay(echo, 80*time.Millisecond))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(s.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	start := time.Now()
	for i := 0; i < 20; i++ {
		if err := c.Write(ctx, websocket.MessageText, []byte(fmt.Sprint(i))); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 20; i++ {
		_, b, err := c.Read(ctx)
		if err != nil || string(b) != fmt.Sprint(i) {
			t.Fatal("ordering/read", i, string(b), err)
		}
	}
	elapsed := time.Since(start)
	if elapsed < 70*time.Millisecond || elapsed > time.Second {
		t.Fatal("RTT missing or delay multiplied per message", elapsed)
	}
	t.Logf("20 ordered messages through 80ms RTT: %v", elapsed)
}
func TestDelayedHTTPRoundTrip(t *testing.T) {
	s := httptest.NewServer(Delay(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }), 80*time.Millisecond))
	defer s.Close()
	start := time.Now()
	r, err := http.Get(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if time.Since(start) < 70*time.Millisecond {
		t.Fatal("HTTP latency absent")
	}
}
