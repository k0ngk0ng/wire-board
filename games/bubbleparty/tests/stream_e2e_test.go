package bubbleparty_test

// Failure inventory: eight authenticated human inputs collide at one version;
// a second tab duplicates a bomb; reconnect rolls back live input; spectators
// control a seat; stream acknowledgement incorrectly promises per-step storage.
import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	bubbleparty "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func bubbleStream(t *testing.T, c *gametest.Client) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(c.Base, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: c.HTTP})
	if err != nil {
		t.Fatal(err)
	}
	conn.SetReadLimit(2 << 20)
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}
func bubblePacket(t *testing.T, c *websocket.Conn, kind string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for i := 0; i < 100; i++ {
		_, raw, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var packet map[string]any
		if err = json.Unmarshal(raw, &packet); err != nil {
			t.Fatal(err)
		}
		if packet["type"] == kind {
			return packet
		}
	}
	t.Fatal("missing stream message", kind)
	return nil
}
func bubbleSend(t *testing.T, c *websocket.Conn, packet any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
}
func TestBubblepartyEightHumanStreamTabsAndReconnect(t *testing.T) {
	h := gametest.NewHTTP(t, 8, bubbleparty.Module(), "bubbleparty")
	h.Start(map[string]any{"map": "community10", "roundSeconds": 300})
	initial := waitPlaying(t, h.People[0], 6*time.Second)
	sockets := make([]*websocket.Conn, 8)
	requests := make([]map[string]any, 8)
	for i, c := range h.People {
		sockets[i] = bubbleStream(t, c)
		state := bubblePacket(t, sockets[i], "state")["state"].(map[string]any)
		room := state["room"].(map[string]any)
		requests[i] = map[string]any{"type": "input", "roomId": room["id"], "version": room["version"], "serverInstance": state["serverInstance"], "nonce": fmt.Sprintf("eight-human-bomb-%d", i), "action": map[string]any{"type": "input", "epoch": initial.Epoch, "seq": 1, "move": "none", "bomb": true}}
	}
	for i, c := range sockets {
		bubbleSend(t, c, requests[i])
	}
	var firstAck map[string]any
	for i, c := range sockets {
		ack := bubblePacket(t, c, "ack")
		if ack["status"] != float64(200) || ack["durable"] != false {
			t.Fatal("human stream rejected or promised per-step durability", i, ack)
		}
		if i == 0 {
			firstAck = ack
		}
	}
	tab := bubbleStream(t, h.People[0])
	bubblePacket(t, tab, "state")
	bubbleSend(t, tab, requests[0])
	ack := bubblePacket(t, tab, "ack")
	if ack["status"] != float64(200) || ack["version"] != firstAck["version"] {
		t.Fatal("second tab replayed nonce", ack, firstAck)
	}
	sockets[0].CloseNow()
	reconnected := bubbleStream(t, h.People[0])
	state := bubblePacket(t, reconnected, "state")["state"].(map[string]any)
	room := state["room"].(map[string]any)
	data := gametest.Data(room)
	for i, p := range data["players"].([]any) {
		if p.(map[string]any)["seq"] != float64(1) {
			t.Fatal("reconnect lost or repeated human input", i, p)
		}
	}
	watcher := bubbleStream(t, h.Spectator)
	bubblePacket(t, watcher, "state")
	bad := map[string]any{}
	for k, v := range requests[0] {
		bad[k] = v
	}
	bad["nonce"] = "spectator-eight-input"
	bubbleSend(t, watcher, bad)
	if ack = bubblePacket(t, watcher, "ack"); ack["status"] == float64(200) {
		t.Fatal("spectator controlled player", ack)
	}
	h.Record("eight real human streams; second-tab duplicate rejected by receipt; reconnect retains all sequences; spectator rejected")
}

func TestBubblepartyHTTPPineCollisionAndHerbPassage(t *testing.T) {
	h := gametest.NewHTTP(t, 8, bubbleparty.Module(), "bubbleparty")
	h.Start(map[string]any{"map": "community10", "roundSeconds": 300})
	c := h.People[2]
	v := waitPlaying(t, c, 6*time.Second)
	if v.Players[2].X != 6 || v.Players[2].Y != 6 {
		t.Fatal("unexpected public spawn")
	}
	c.Action(map[string]any{"type": "input", "epoch": v.Epoch, "seq": 1, "move": "left", "bomb": false}, 200)
	time.Sleep(400 * time.Millisecond)
	c.Action(map[string]any{"type": "input", "epoch": v.Epoch, "seq": 2, "move": "none", "bomb": false}, 200)
	v = httpView(t, c)
	p := v.Players[2]
	if p.PX < 4.78-.000001 || p.X != 5 || v.Cells[6*15+4] != bubbleparty.CellWall || v.Tiles[6*15+4] != bubbleparty.TileTree {
		t.Fatal("real human walked into pine", p, v.Cells[6*15+4], v.Tiles[6*15+4])
	}
	c.Action(map[string]any{"type": "input", "epoch": v.Epoch, "seq": 3, "move": "down", "bomb": false}, 200)
	time.Sleep(200 * time.Millisecond)
	c.Action(map[string]any{"type": "input", "epoch": v.Epoch, "seq": 4, "move": "none", "bomb": false}, 200)
	v = httpView(t, c)
	p = v.Players[2]
	if p.Y != 7 || v.Cells[7*15+5] != bubbleparty.CellEmpty || v.Tiles[7*15+5] != bubbleparty.TileHedge {
		t.Fatal("real human cannot enter fruit herb", p)
	}
	h.Record("eight human room: native module collision stops at pine; movement enters passable herb")
}
