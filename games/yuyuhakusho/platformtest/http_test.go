package platformtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	game "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func registered(t *testing.T) gameapi.Module {
	t.Helper()
	for _, m := range games.Modules() {
		if m.ID == "yuyuhakusho" {
			return m
		}
	}
	t.Fatal("魔强统一战尚未注册到平台composition root")
	return gameapi.Module{}
}
func wire(t *testing.T, c *gametest.Client) game.View {
	t.Helper()
	raw, err := json.Marshal(gametest.Data(c.Room()))
	if err != nil {
		t.Fatal(err)
	}
	var v game.View
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func wait(t *testing.T, c *gametest.Client, predicate func(game.View) bool) game.View {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var v game.View
	for time.Now().Before(deadline) {
		v = wire(t, c)
		if predicate(v) {
			return v
		}
		time.Sleep(35 * time.Millisecond)
	}
	t.Fatalf("platform condition not reached: frame=%d phase=%s", v.Frame, v.Phase)
	return v
}
func load(t *testing.T, h *gametest.HTTP) {
	t.Helper()
	v := wire(t, h.People[0])
	for _, c := range h.People {
		c.Action(map[string]any{"type": "loaded", "epoch": v.Epoch}, 200)
	}
}
func input(v game.View, seat int, buttons engine.Buttons) map[string]any {
	return map[string]any{"type": "input", "epoch": v.Epoch, "sequence": v.Inputs[seat].Sequence + 1, "buttons": buttons}
}
func reject(t *testing.T, c *gametest.Client, a map[string]any) {
	t.Helper()
	room := c.Room()
	body, _ := json.Marshal(c.ActionRequest(a))
	req, err := http.NewRequest("POST", c.Base+"/api/rooms/"+room["id"].(string), bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	r, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != 400 && r.StatusCode != 403 && r.StatusCode != 409 {
		t.Fatalf("unauthorized/invalid action accepted: %d", r.StatusCode)
	}
}
func checkpoint(t *testing.T, c *gametest.Client) game.SyncSnapshot {
	t.Helper()
	room := c.Room()
	reply := c.Request("GET", "/api/rooms/"+room["id"].(string)+"/sync", nil, 200)
	raw, _ := json.Marshal(reply["data"])
	var s game.SyncSnapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if s.Checkpoint == "" {
		t.Fatal("recovery checkpoint is not opaque exactJSON")
	}
	return s
}
func TestRegisteredGameUsesRealHostLoadingAndIndependentSeatInputs(t *testing.T) {
	h := gametest.NewHTTP(t, 2, registered(t), "yuyuhakusho")
	h.AssertPublished()
	h.Start(map[string]any{"arena": 10, "characters": []int{7, 9}})
	v := wire(t, h.People[0])
	if v.Phase != "loading" || v.Frame != 0 || len(v.SeatSlots) != 2 || v.SeatSlots[0] != 0 || v.SeatSlots[1] != 3 {
		t.Fatal("initial platform loading/mapping differs", v.Phase, v.Frame, v.SeatSlots)
	}
	time.Sleep(150 * time.Millisecond)
	if wire(t, h.People[0]).Frame != 0 {
		t.Fatal("unloaded game advanced")
	}
	h.People[0].Action(map[string]any{"type": "loaded", "epoch": v.Epoch}, 200)
	if wire(t, h.People[0]).Frame != 0 {
		t.Fatal("one player bypassed all-ready gate")
	}
	h.People[1].Action(map[string]any{"type": "loaded", "epoch": v.Epoch}, 200)
	v = wait(t, h.People[0], func(v game.View) bool { return v.Frame > 2 })
	reject(t, h.Spectator, input(v, 0, engine.ButtonA))
	room := h.People[0].Room()
	requests := []map[string]any{h.People[0].ActionRequest(input(v, 0, engine.ButtonRight|engine.ButtonX|engine.ButtonZ)), h.People[1].ActionRequest(input(v, 1, engine.ButtonLeft|engine.ButtonY))}
	for i, request := range requests {
		request["version"] = room["version"]
		h.People[i].Request("POST", "/api/rooms/"+room["id"].(string), request, 200)
	}
	v = wait(t, h.People[0], func(v game.View) bool {
		return v.Inputs[0].Sequence == 1 && v.Inputs[1].Sequence == 1 && v.Inputs[0].AppliedSequence == 1 && v.Inputs[1].AppliedSequence == 1
	})
	if v.Inputs[0].AppliedButtons != (engine.ButtonRight|engine.ButtonX|engine.ButtonZ) || v.Inputs[1].AppliedButtons != (engine.ButtonLeft|engine.ButtonY) {
		t.Fatal("MD12bit independent input lost", v.Inputs)
	}
	reject(t, h.People[0], map[string]any{"type": "input", "epoch": "other", "sequence": 2, "buttons": 0})
	reject(t, h.People[0], map[string]any{"type": "input", "epoch": v.Epoch, "sequence": 1, "buttons": 0})
	reject(t, h.People[0], map[string]any{"type": "input", "epoch": v.Epoch, "sequence": 2, "buttons": 4096})
	v = wait(t, h.People[0], func(v game.View) bool { return v.Inputs[0].Buttons == 0 && v.Inputs[1].Buttons == 0 })
	if v.Inputs[0].Automated || v.Inputs[1].Automated {
		t.Fatal("lost input caused automatic AI")
	}
	data := gametest.Data(h.People[0].Room())
	for _, key := range []string{"world", "World", "checkpoint", "presentation", "HUD", "save", "snapshot"} {
		if data[key] != nil {
			t.Fatal("ordinary update leaked recovery payload", key)
		}
	}
	h.Record("real loading,seat auth,12bit,release,unrated-independent inputs")
}
func TestRealHostOpaqueRecoveryAndExplicitAutoplay(t *testing.T) {
	h := gametest.NewHTTP(t, 3, registered(t), "yuyuhakusho")
	h.Start(map[string]any{"characters": []int{3, 7, 9}, "arena": 2})
	load(t, h)
	v := wait(t, h.People[0], func(v game.View) bool { return v.Frame > 250 })
	beforeEpoch := v.Epoch
	h.People[1].Command("autoplay", map[string]any{"enabled": true}, 200)
	wait(t, h.People[0], func(v game.View) bool { return v.Inputs[1].Automated && v.Players[1].Computer })
	h.People[1].Command("autoplay", map[string]any{"enabled": false}, 200)
	v = wait(t, h.People[0], func(v game.View) bool { return !v.Inputs[1].Automated && !v.Players[1].Computer })
	if v.Epoch != beforeEpoch || v.Frame < 250 {
		t.Fatal("takeover recreated game")
	}
	snap := checkpoint(t, h.People[0])
	var c game.Checkpoint
	if err := json.Unmarshal([]byte(snap.Checkpoint), &c); err != nil {
		t.Fatal(err)
	}
	if c.Frame != snap.View.Frame || c.Epoch != snap.View.Epoch || c.ResourceDescriptorSHA256 == "" {
		t.Fatal("non-atomic checkpoint boundary")
	}
	high := false
	for _, word := range c.Presentation.SpriteSlots.Present {
		high = high || word > 1<<53
	}
	if !high {
		t.Fatal("test did not exercise high uint64 sprite bits")
	}
	h.Restart()
	v = wire(t, h.People[0])
	if v.Epoch != beforeEpoch || v.Frame < snap.View.Frame || v.Phase == "loading" || v.Options.Arena != 2 {
		t.Fatal("persisted room reset on restart", v.Frame, v.Phase)
	}
	h.Record(fmt.Sprintf("opaque checkpoint frame%d survived SQL restart and explicit AI handoff", c.Frame))
}
