package server

// Before implementation: invented/client-supplied latency, old connection
// cleanup erasing a newer connection, ping blocking snapshots, peer data leaking
// beyond a room, bots shown as 0ms, and disconnect still showing good signal.
import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNetworkLatencyIsMeasuredAndVisibleToRoomPeersE2E(t *testing.T) {
	_, ts, a := adminServer(t)
	b := newClient(t, ts.URL)
	b.register("LatencyPeer")
	room := a.post("/api/rooms", map[string]any{"name": "Network room", "kind": "rail", "capacity": 2}, 201)
	b.post("/api/rooms/"+room["id"].(string), map[string]any{"type": "join", "version": room["version"], "nonce": "network-join"}, 200)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	open := func(c *testClient, delay time.Duration) *websocket.Conn {
		conn, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: c.client, OnPingReceived: func(context.Context, []byte) bool { time.Sleep(delay); return true }})
		if e != nil {
			t.Fatal(e)
		}
		conn.SetReadLimit(2 << 20)
		go func() {
			for {
				if _, _, e := conn.Read(ctx); e != nil {
					return
				}
			}
		}()
		return conn
	}
	fast, slow := open(a, 0), open(b, 300*time.Millisecond)
	defer fast.CloseNow()
	defer slow.CloseNow()
	var own, peer map[string]any
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		state := a.state()
		own, _ = state["network"].(map[string]any)
		seats := state["room"].(map[string]any)["seats"].([]any)
		peer, _ = seats[1].(map[string]any)["network"].(map[string]any)
		if own != nil && peer != nil && own["rttMs"] != nil && peer["rttMs"] != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if own == nil || peer == nil || own["rttMs"] == nil || peer["rttMs"] == nil {
		t.Fatal("no measured RTT in shared state")
	}
	if peer["rttMs"].(float64) < 200 || peer["rttMs"].(float64) <= own["rttMs"].(float64)+100 {
		t.Fatal("latency does not reflect delayed pong", own, peer)
	}
	slow.CloseNow()
	until = time.Now().Add(time.Second)
	for time.Now().Before(until) {
		v := a.state()["room"].(map[string]any)["seats"].([]any)[1].(map[string]any)["network"].(map[string]any)
		if v["connected"] == false {
			out := filepath.Join("..", "..", ".local", "near-order")
			os.MkdirAll(out, 0755)
			raw, _ := json.MarshalIndent(map[string]any{"passed": true, "fast": own, "slow": peer, "disconnected": v}, "", "  ")
			os.WriteFile(filepath.Join(out, "network-http.json"), raw, 0600)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("disconnected peer remained connected")
}

func TestNetworkProbeDoesNotBlockStateDeliveryE2E(t *testing.T) {
	_, ts, c := adminServer(t)
	room := c.post("/api/rooms", map[string]any{"name": "Probe room", "kind": "rail", "capacity": 2}, 201)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	ping := make(chan struct{}, 1)
	ready := make(chan struct{}, 1)
	conn, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: c.client, OnPingReceived: func(context.Context, []byte) bool {
		select {
		case ping <- struct{}{}:
		default:
		}
		return false
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(2 << 20)
	go func() {
		for {
			_, raw, e := conn.Read(ctx)
			if e != nil {
				return
			}
			var packet struct {
				Type  string
				State struct {
					Room struct{ Seats []struct{ Ready bool } }
				}
			}
			json.Unmarshal(raw, &packet)
			if packet.Type == "state" && len(packet.State.Room.Seats) > 0 && packet.State.Room.Seats[0].Ready {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
		}
	}()
	select {
	case <-ping:
	case <-time.After(time.Second):
		t.Fatal("initial latency probe did not start")
	}
	c.post("/api/rooms/"+room["id"].(string), map[string]any{"type": "ready", "version": room["version"], "nonce": "network-ready"}, http.StatusOK)
	select {
	case <-ready:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("pending ping blocked normal state delivery")
	}
}

func TestNetworkHeaderUsesItsOwnConnectionE2E(t *testing.T) {
	_, ts, c := adminServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	open := func(delay time.Duration) (*websocket.Conn, chan map[string]any) {
		conn, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: c.client, OnPingReceived: func(context.Context, []byte) bool { time.Sleep(delay); return true }})
		if e != nil {
			t.Fatal(e)
		}
		conn.SetReadLimit(2 << 20)
		packets := make(chan map[string]any, 16)
		go func() {
			for {
				_, raw, e := conn.Read(ctx)
				if e != nil {
					return
				}
				var p map[string]any
				json.Unmarshal(raw, &p)
				if p["type"] == "network" {
					select {
					case packets <- p:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
		return conn, packets
	}
	fast, packets := open(0)
	defer fast.CloseNow()
	slow, _ := open(300 * time.Millisecond)
	defer slow.CloseNow()
	var aggregate map[string]any
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		aggregate, _ = c.state()["network"].(map[string]any)
		if aggregate != nil && aggregate["rttMs"] != nil && aggregate["rttMs"].(float64) >= 200 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if aggregate == nil || aggregate["rttMs"] == nil || aggregate["rttMs"].(float64) < 200 {
		t.Fatal("slow connection was not measured")
	}
	for {
		select {
		case p := <-packets:
			if p["serverNow"].(float64) >= aggregate["sampledAt"].(float64) {
				own := p["self"].(map[string]any)
				if own["rttMs"] == nil || own["rttMs"].(float64) >= 150 {
					t.Fatal("fast device inherited other device latency", own)
				}
				return
			}
		case <-ctx.Done():
			t.Fatal("missing current connection metric")
		}
	}
}
