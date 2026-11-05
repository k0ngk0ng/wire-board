package platformtest

// Actual authenticated WebSocket input and compact publication on the real
// host. Bootstrap is allowed once; ordinary game updates must stay separate
// from recovery checkpoints, account/lobby fields and pixel payloads.
import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	game "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestRealWebsocketCompactFramesAndIdempotentInputs(t *testing.T) {
	h := gametest.NewHTTP(t, 2, registered(t), "yuyuhakusho")
	h.Start(map[string]any{"characters": []int{3, 9}, "arena": 4})
	load(t, h)
	c, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(h.Server.URL, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: h.People[0].HTTP})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	c.SetReadLimit(512 << 10)
	read := func() (map[string]any, int) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		_, raw, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err = json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		return value, len(raw)
	}
	until := func(kind string) map[string]any {
		t.Helper()
		for n := 0; n < 100; n++ {
			v, _ := read()
			if v["type"] == kind {
				return v
			}
		}
		t.Fatal("websocket expected message missing", kind)
		return nil
	}
	first, _ := read()
	if first["type"] != "state" {
		t.Fatal("authenticated bootstrap missing", first)
	}
	v := wire(t, h.People[0])
	room := h.People[0].Room()
	request := map[string]any{"type": "input", "roomId": room["id"], "version": room["version"], "nonce": "yuyu-ws-once", "action": input(v, 0, engine.ButtonRight|engine.ButtonZ)}
	send := func() {
		t.Helper()
		raw, _ := json.Marshal(request)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err = c.Write(ctx, websocket.MessageText, raw); err != nil {
			t.Fatal(err)
		}
	}
	send()
	ack := until("ack")
	if ack["status"] != float64(200) || ack["nonce"] != "yuyu-ws-once" {
		t.Fatal("input websocket acknowledgement differs", ack)
	}
	send()
	duplicate := until("ack")
	if duplicate["status"] != float64(200) || duplicate["version"] != ack["version"] {
		t.Fatal("same nonce replay was not idempotent", duplicate, ack)
	}
	deadline := time.Now().Add(2 * time.Second)
	packets, maxBytes := 0, 0
	var before, after uint64
	for time.Now().Before(deadline) {
		message, size := read()
		if message["type"] != "game" {
			continue
		}
		if size > maxBytes {
			maxBytes = size
		}
		packets++
		for _, key := range []string{"state", "rooms", "availableGames", "modules", "chat", "user", "checkpoint"} {
			if _, ok := message[key]; ok {
				t.Fatal("ordinary realtime packet leaked bootstrap/recovery", key)
			}
		}
		payload := message["game"].(map[string]any)
		raw, _ := json.Marshal(payload["data"])
		var current game.View
		if err = json.Unmarshal(raw, &current); err != nil {
			t.Fatal(err)
		}
		if before == 0 {
			before = current.Frame
		}
		after = current.Frame
		if current.Inputs[0].Sequence != 1 {
			t.Fatal("duplicate websocket incremented accepted input", current.Inputs[0])
		}
		data := payload["data"].(map[string]any)
		if _, ok := data["frame"].(string); !ok {
			t.Fatal("uint64 frame passed unsafe JSON number")
		}
		for _, key := range []string{"world", "presentation", "checkpoint", "save"} {
			if data[key] != nil {
				t.Fatal("ordinary game view includes recovery state", key)
			}
		}
	}
	if packets < 3 || after <= before || maxBytes > 32768 {
		t.Fatal("compact realtime clock did not progress in bounds", packets, before, after, maxBytes)
	}
	h.Record("actual authenticated websocket input retry and compact clock publication")
}
