package basketball_test

// E2E failure inventory: consecutive stream edges are lost; retransmitted
// nonce applies twice; restart retains pressed controls or loses autoplay;
// stale server instances can control a restored game; observers can send input.
import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"strings"
	"testing"
	"time"
)

func basketballStream(t *testing.T, c *gametest.Client) *websocket.Conn {
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
func basketballPacket(t *testing.T, c *websocket.Conn, kind string) map[string]any {
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
func basketballSend(t *testing.T, c *websocket.Conn, packet any) {
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

func TestBasketballRealtimeStreamAndRestore(t *testing.T) {
	h := gametest.NewHTTP(t, 2, observedModule(t), "basketball")
	h.Start(map[string]any{"mode": "versus"})
	basketballReadyHTTP(t, h)
	a, b := h.People[0], h.People[1]
	await(t, a, func(v game.View) bool { return v.Runtime.Playing != nil })
	b.Command("autoplay", map[string]any{"enabled": true}, 200)
	await(t, a, func(v game.View) bool { return v.Inputs[2].Automated })
	conn := basketballStream(t, a)
	state := basketballPacket(t, conn, "state")["state"].(map[string]any)
	room := state["room"].(map[string]any)
	v := view(t, a)
	var first map[string]any
	for n := int64(1); n <= 8; n++ {
		buttons := 0
		if n%2 == 1 {
			buttons = 1
		}
		packet := map[string]any{"type": "input", "roomId": room["id"], "version": room["version"], "serverInstance": state["serverInstance"], "nonce": fmt.Sprintf("basketball-stream-%d", n), "action": map[string]any{"type": "inputs", "epoch": v.Epoch, "seat": 0, "basedOn": v.ServerAt, "sequence": n, "events": []any{map[string]any{"sequence": n, "buttons": buttons, "offsetMS": 0, "release": false}}}}
		if n == 1 {
			first = packet
		}
		basketballSend(t, conn, packet)
	}
	for n := 0; n < 8; n++ {
		ack := basketballPacket(t, conn, "ack")
		if ack["status"] != float64(200) || ack["durable"] != false {
			t.Fatal("stream edge rejected or incorrect durability", ack)
		}
	}
	v = view(t, a)
	if v.Inputs[0].Sequence != 8 || v.Inputs[0].Buttons != 0 || !v.Inputs[2].Automated {
		t.Fatal("stream lost release or other seat autoplay")
	}
	basketballSend(t, conn, first)
	if ack := basketballPacket(t, conn, "ack"); ack["status"] != float64(200) {
		t.Fatal("nonce retry failed", ack)
	}
	if view(t, a).Inputs[0].Sequence != 8 {
		t.Fatal("retry resurrected old press")
	}
	h.Record("eight ordered inputs before reading ACKs; release retained; duplicate nonce does not replay")
	conn.CloseNow()
	a.Action(buttons(v, 0, 1), 200)
	before := view(t, a)
	h.Restart()
	after := view(t, a)
	if after.Epoch != before.Epoch || after.Runtime.VideoFrames < before.Runtime.VideoFrames || after.Inputs[0].Buttons != 0 || !after.Inputs[2].Automated {
		t.Fatal("restore lost game or kept physical input")
	}
	conn = basketballStream(t, a)
	next := basketballPacket(t, conn, "state")["state"].(map[string]any)
	if next["serverInstance"] == state["serverInstance"] {
		t.Fatal("restart did not change instance")
	}
	first["nonce"] = "stale-instance-after-restart"
	basketballSend(t, conn, first)
	if ack := basketballPacket(t, conn, "ack"); ack["status"] == float64(200) {
		t.Fatal("old instance input accepted")
	}
	watcher := basketballStream(t, h.Spectator)
	basketballPacket(t, watcher, "state")
	first["serverInstance"] = next["serverInstance"]
	first["nonce"] = "spectator-input"
	basketballSend(t, watcher, first)
	if ack := basketballPacket(t, watcher, "ack"); ack["status"] == float64(200) {
		t.Fatal("spectator controlled player")
	}
	h.Record("restart preserves match and explicit autoplay; clears physical controls; rejects old instance and spectator")
}
