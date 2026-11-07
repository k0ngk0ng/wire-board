package contra

// Failure inventory before host capability implementation: the HTTP action and
// WS stream reject an authenticated release while autoplay is active; stale room
// versions bypass neither branch; a broad exception also admits held input,
// resume, wrong epochs, forged seats or observers; retries duplicate releases;
// cancellation/SQLite restart changes ownership or loses sequence monotonicity.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"testing"
	"time"
)

func TestContraHTTPWSAutoplayInputRelease(t *testing.T) {
	h := gametest.NewHTTP(t, 2, testModule(t), "contra")
	h.Start(map[string]any{})
	a := h.People[0]
	a.Command("autoplay", map[string]any{"enabled": true}, 200)
	v := awaitHTTP(t, a, func(v View) bool { return v.Inputs[0].Automated })
	release := map[string]any{"type": "release", "epoch": v.Epoch, "sequence": 1}
	request := a.ActionRequest(release)
	// Preserve an old-but-current-epoch room version across real timer frames.
	time.Sleep(130 * time.Millisecond)
	a.Request("POST", "/api/rooms/"+a.Room()["id"].(string), request, 200)
	a.Request("POST", "/api/rooms/"+a.Room()["id"].(string), request, 200)
	if next := httpView(t, a); next.Inputs[0].Sequence != 1 || !next.Inputs[0].Automated || a.Room()["seats"].([]any)[0].(map[string]any)["autoPlay"] != true {
		t.Fatal("HTTP release lost ownership or dedup")
	}
	a.Action(map[string]any{"type": "input", "epoch": v.Epoch, "sequence": 2, "held": 1}, 400)
	a.Action(map[string]any{"type": "resume", "epoch": v.Epoch, "sequence": 2}, 400)
	a.Action(map[string]any{"type": "release", "epoch": "old", "sequence": 2}, 400)
	a.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": 2, "seat": 1}, 400)
	h.Spectator.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": 2}, 400)
	ws := stream(t, a)
	state := packet(t, ws, "state")["state"].(map[string]any)
	room := state["room"].(map[string]any)
	p := map[string]any{"type": "input", "roomId": room["id"], "version": room["version"], "serverInstance": state["serverInstance"], "nonce": "contra-auto-release-ws", "action": map[string]any{"type": "release", "epoch": v.Epoch, "sequence": 2}}
	time.Sleep(130 * time.Millisecond)
	send(t, ws, p)
	if ack := packet(t, ws, "ack"); ack["status"] != float64(200) {
		t.Fatal("stream rejected release", ack)
	}
	send(t, ws, p)
	if ack := packet(t, ws, "ack"); ack["status"] != float64(200) {
		t.Fatal("stream release retry", ack)
	}
	p["nonce"] = "contra-auto-held-ws"
	p["action"] = map[string]any{"type": "input", "epoch": v.Epoch, "sequence": 3, "held": 1}
	send(t, ws, p)
	if ack := packet(t, ws, "ack"); ack["status"] == float64(200) {
		t.Fatal("held input bypassed autoplay")
	}
	p["nonce"] = "contra-auto-bad-epoch-ws"
	p["action"] = map[string]any{"type": "release", "epoch": "old", "sequence": 3}
	send(t, ws, p)
	if ack := packet(t, ws, "ack"); ack["status"] == float64(200) {
		t.Fatal("bad epoch bypassed autoplay")
	}
	watcher := stream(t, h.Spectator)
	packet(t, watcher, "state")
	p["nonce"] = "contra-auto-observer-ws"
	p["action"] = map[string]any{"type": "release", "epoch": v.Epoch, "sequence": 3}
	send(t, watcher, p)
	if ack := packet(t, watcher, "ack"); ack["status"] == float64(200) {
		t.Fatal("observer released another seat")
	}
	if next := httpView(t, a); next.Inputs[0].Sequence != 2 || !next.Inputs[0].Automated {
		t.Fatal("rejected actions changed input control")
	}
	h.Record("HTTP and WS accept only current authenticated release during autoplay; stale room versions and nonce retry are supported")
	ws.CloseNow()
	watcher.CloseNow()
	h.Restart()
	if next := httpView(t, a); next.Inputs[0].Sequence != 2 || !next.Inputs[0].Automated {
		t.Fatal("SQLite lost release sequence or autoplay")
	}
	a.Command("autoplay", map[string]any{"enabled": false}, 200)
	a.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": 3}, 200)
	if next := httpView(t, a); next.Inputs[0].Sequence != 3 || next.Inputs[0].Automated {
		t.Fatal("cancel/release changed ownership or sequence")
	}
	h.Record("SQLite restart retains autoplay and sequence; cancellation then release stays manual")
}

func TestContraHTTPWSInputReleaseCapabilityIsolation(t *testing.T) {
	for _, mode := range []string{"not-opted-in", "predicate-panics", "predicate-mutates-inputs"} {
		t.Run(mode, func(t *testing.T) {
			m := testModule(t)
			original := m.IsInputRelease
			status := 400
			switch mode {
			case "not-opted-in":
				m.IsInputRelease = nil
			case "predicate-panics":
				m.IsInputRelease = func(gameapi.Context, gameapi.Match, json.RawMessage) bool { panic("release predicate failure") }
			case "predicate-mutates-inputs":
				status = 200
				m.IsInputRelease = func(c gameapi.Context, g gameapi.Match, a json.RawMessage) bool {
					ok := original(c, g, a)
					c.Automated[0] = false
					g.Data[0] = 'X'
					g.Actors[0] = 1
					a[0] = 'X'
					return ok
				}
			}
			h := gametest.NewHTTP(t, 2, m, "contra")
			h.Start(map[string]any{})
			a := h.People[0]
			a.Command("autoplay", map[string]any{"enabled": true}, 200)
			v := awaitHTTP(t, a, func(v View) bool { return v.Inputs[0].Automated })
			a.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": 1}, status)
			ws := stream(t, a)
			state := packet(t, ws, "state")["state"].(map[string]any)
			room := state["room"].(map[string]any)
			sequence := 1
			if status == 200 {
				sequence = 2
			}
			send(t, ws, map[string]any{"type": "input", "roomId": room["id"], "version": room["version"], "serverInstance": state["serverInstance"], "nonce": "contra-release-capability-" + mode, "action": map[string]any{"type": "release", "epoch": v.Epoch, "sequence": sequence}})
			if ack := packet(t, ws, "ack"); ack["status"] != float64(status) {
				t.Fatal("wrong capability result", ack)
			}
			next := httpView(t, a)
			expected := int64(0)
			if status == 200 {
				expected = 2
			}
			if next.Inputs[0].Sequence != expected || !next.Inputs[0].Automated || next.Actor != 0 {
				t.Fatal("predicate mutated authority", next.Inputs)
			}
			h.Record("explicit release capability only; predicate panic fails closed; callback inputs detached")
		})
	}
}
