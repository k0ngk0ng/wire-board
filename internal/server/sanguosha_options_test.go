package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestSanguoshaOptionsPermissionsFreezeAndRestore(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	guest := newClient(t, ts.URL)
	host.register("扩展房主")
	guest.register("扩展朋友")
	host.post("/api/rooms", map[string]any{"name": "bad", "kind": "sanguosha", "capacity": 4, "sanguoshaOptions": game.SGOptions{Deck: "missing"}}, 400)
	room := host.post("/api/rooms", map[string]any{"name": "军争测试", "kind": "sanguosha", "capacity": 4, "sanguoshaOptions": game.SGOptions{Deck: "military"}}, 201)
	guest.command(room, "join", nil, 200)
	for range 2 {
		host.command(current(host), "add_bot", nil, 200)
	}
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	change := func(c *testClient, deck string, want int) {
		r := current(c)
		c.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "sanguosha_options", "sanguoshaOptions": game.SGOptions{Deck: deck}, "version": r["version"], "nonce": randomID(12)}, want)
	}
	change(guest, "standard", 400)
	change(host, "missing", 400)
	change(host, "standard", 200)
	room = current(host)
	for _, seat := range room["seats"].([]any) {
		p := seat.(map[string]any)
		if p["bot"] != true && p["ready"] == true {
			t.Fatal("did not reset human readiness")
		}
	}
	host.command(room, "start", nil, 400)
	change(host, "military", 200)
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	host.command(current(host), "start", nil, 200)
	change(host, "standard", 400)
	room = current(host)
	id := room["id"].(string)
	view := room["game"].(map[string]any)["sanguosha"].(map[string]any)
	if len(view["cards"].([]any)) != 160 || view["options"].(map[string]any)["deck"] != "military" {
		t.Fatal("wrong deck in play")
	}
	s.mu.Lock()
	raw, err := json.Marshal(s.rooms[id])
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	var restored Room
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.SanguoshaOptions.Deck != "military" || restored.Game.Sanguosha.Options.Deck != "military" {
		t.Fatal("settings not persisted")
	}
	if len(restored.Game.Sanguosha.Deck) != 160 {
		t.Fatal("deck lost")
	}
}
