package server

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestSplendorOptionsPermissionsFreezeAndArchive(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("宝石扩展房主")
	guest.register("宝石扩展朋友")
	host.post("/api/rooms", map[string]any{"name": "错误规则", "kind": "splendor", "capacity": 3, "splendorOptions": game.SplendorOptions{Rules: "old"}}, 400)
	room := host.post("/api/rooms", map[string]any{"name": "要塞与贸易站", "kind": "splendor", "capacity": 3, "splendorOptions": game.SplendorOptions{TradingPosts: true}}, 201)
	guest.command(room, "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	change := func(c *testClient, o game.SplendorOptions, want int) {
		r := current(c)
		c.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "splendor_options", "splendorOptions": o, "version": r["version"], "nonce": randomID(12)}, want)
	}
	options := game.SplendorOptions{TradingPosts: true, Strongholds: true}
	change(guest, options, 400)
	change(host, game.SplendorOptions{Rules: "unknown"}, 400)
	change(host, options, 200)
	for _, seat := range current(host)["seats"].([]any) {
		p := seat.(map[string]any)
		if p["bot"] != true && p["ready"] == true {
			t.Fatal("human readiness not reset")
		}
	}
	host.command(current(host), "start", nil, 400)
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	host.command(current(host), "start", nil, 200)
	change(host, game.SplendorOptions{}, 400)
	room = current(host)
	id := room["id"].(string)
	view := room["game"].(map[string]any)["splendor"].(map[string]any)
	if view["options"].(map[string]any)["rules"] != game.SplendorExpansionRules {
		t.Fatal("rules identity missing")
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
	for _, o := range []game.SplendorOptions{restored.SplendorOptions, restored.Game.Splendor.Options} {
		if !o.TradingPosts || !o.Strongholds || o.Rules != game.SplendorExpansionRules {
			t.Fatal("lost saved expansion rules", o)
		}
	}
	// Archive exactly the options used for the match, independent of rematches.
	s.mu.Lock()
	r := s.rooms[id]
	r.Status = "closed"
	err = s.save(r)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	var snapshot string
	if err = s.db.QueryRow("SELECT snapshot FROM match_history WHERE id=?", restored.MatchID).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	var record MatchRecord
	if err = json.Unmarshal([]byte(snapshot), &record); err != nil {
		t.Fatal(err)
	}
	if record.SplendorOptions != restored.SplendorOptions {
		t.Fatal("archive lost expansion identity")
	}
}

func TestSplendorOldRoomDefaultsToBase(t *testing.T) {
	var r Room
	if err := json.Unmarshal([]byte(`{"kind":"splendor","capacity":4}`), &r); err != nil {
		t.Fatal(err)
	}
	s, err := game.NewSplendor(2, r.SplendorOptions)
	if err != nil || s.Splendor.Options.TradingPosts || s.Splendor.Options.Strongholds {
		t.Fatal("old room gained expansion rules", err)
	}
}
