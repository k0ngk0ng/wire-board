package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanHelpersOptionsPermissionsFreezeAndArchive(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("卡坦扩展房主")
	guest.register("卡坦扩展朋友")
	host.post("/api/rooms", map[string]any{"name": "错误规则", "kind": "catan", "capacity": 3, "catanOptions": game.CatanOptions{Rules: "old"}}, 400)
	room := host.post("/api/rooms", map[string]any{"name": "Helpers", "kind": "catan", "capacity": 3, "catanOptions": game.CatanOptions{Helpers: true}}, 201)
	guest.command(room, "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	change := func(c *testClient, o game.CatanOptions, want int) {
		r := current(c)
		c.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "catan_options", "catanOptions": o, "version": r["version"], "nonce": randomID(12)}, want)
	}
	options := game.CatanOptions{Helpers: true, AllHelpers: true}
	change(guest, options, 400)
	change(host, game.CatanOptions{Rules: "unknown"}, 400)
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
	change(host, game.CatanOptions{}, 400)
	room = current(host)
	id := room["id"].(string)
	view := room["game"].(map[string]any)["catan"].(map[string]any)
	if view["options"].(map[string]any)["rules"] != game.CatanExpansionRules {
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
	for _, o := range []game.CatanOptions{restored.CatanOptions, restored.Game.Catan.Options} {
		if !o.Helpers || !o.AllHelpers || o.Rules != game.CatanExpansionRules {
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
	if record.CatanOptions != restored.CatanOptions {
		t.Fatal("archive lost expansion identity")
	}
}

func TestCatanHelpersPrivateResponseRestartAndBotActor(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	s.mu.Lock()
	r := s.rooms[id]
	r.CatanOptions = game.CatanOptions{Rules: game.CatanExpansionRules, Helpers: true, AllHelpers: true}
	g := r.Game.Catan
	g.Options = r.CatanOptions
	g.TurnSerial = 8
	g.Players[1].Helper = &game.CatanHelperSeat{ID: 7}
	g.HelperDisplay = []int{1, 2, 3, 4, 5, 6, 8, 9, 10, 11, 12}
	g.HelperPending = &game.CatanHelperPending{Player: 1, Kind: "leader", Target: 2, Resume: "catan_turn"}
	g.HelperSequence++
	r.Game.Turn = 0
	r.Game.Phase = "catan_helper"
	r.CatanTimeLeft = 45000
	r.startTurnClock(time.Now())
	g.Players[2].Resources[4]++
	g.Bank[4]--
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	for i, c := range clients {
		view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		q := view["helperPending"].(map[string]any)
		_, visible := q["resources"]
		if visible != (i == 1) {
			t.Fatalf("viewer %d private helper hand visibility %v", i, visible)
		}
	}
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_helper_choice", "color": 4}, 400)
	before, _ := json.Marshal(s.rooms[id])
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	stopBotTicker(next)
	after, _ := json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("pending helper or timer lost on restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	deadline := next.rooms[id].TurnDeadline
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_helper_choice", "color": 4}, 200)
	if next.rooms[id].TurnDeadline != deadline {
		t.Fatal("same helper response refreshed deadline")
	}
	// Autoplay belongs to the responding seat, not the normal active player.
	next.mu.Lock()
	next.rooms[id].Seats[1].AutoPlay = true
	next.rooms[id].BotAt = 0
	now := time.Now()
	next.runBots(now)
	result := next.rooms[id]
	if result.Game.Catan.HelperPending != nil || result.Game.Turn != 0 || result.Game.Phase != "catan_turn" {
		t.Fatal("bot did not resolve the helper owner")
	}
	if result.TurnDeadline != now.UnixMilli()+45000 {
		t.Fatal("active turn remaining time not restored")
	}
	next.mu.Unlock()
}

func TestCatanHelpersClockAndTimeoutVictory(t *testing.T) {
	s, _, _, id := newCatanTable(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	r.Game.Catan.Options = game.CatanOptions{Helpers: true}
	r.Game.Catan.SetupStep = 6
	r.Game.Catan.TurnSerial = 2
	r.Game.Catan.Players[0].Helper = &game.CatanHelperSeat{ID: 11}
	r.Game.Catan.HelperDisplay = []int{1, 2, 3}
	r.Game.Turn = 0
	r.Game.Phase = "catan_turn"
	now := time.Now()
	r.TurnDeadline = now.Add(35 * time.Second).UnixMilli()
	if err := r.applyGameAction(0, game.Action{Type: "catan_helper", Color: 0}, now); err != nil {
		t.Fatal(err)
	}
	if r.CatanTimeLeft != 35000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("helper did not pause active clock")
	}
	if err := r.applyGameAction(0, game.Action{Type: "catan_helper_choice", Choice: "flip"}, now.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	if r.TurnDeadline != now.Add(55*time.Second).UnixMilli() {
		t.Fatal("helper use granted a fresh normal turn")
	}
	// A timed out private development choice may immediately end the game.
	g := r.Game.Catan
	for i := 0; i < 5; i++ {
		g.Vertices[i].Owner = 0
		g.Vertices[i].Level = 2
	}
	g.Players[0].Helper = &game.CatanHelperSeat{ID: 6}
	g.HelperPending = &game.CatanHelperPending{Player: 0, Kind: "development", Cards: []int{4}, Resume: "catan_turn"}
	r.Game.Phase = "catan_helper"
	r.TurnDeadline = now.Add(-time.Second).UnixMilli()
	s.expireSetups(now)
	r = s.rooms[id]
	if !r.Game.Finished || r.Status != "finished" || r.TurnDeadline != 0 {
		t.Fatal("helper timeout victory was not finalized")
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM match_history WHERE id=?", r.MatchID).Scan(&count); err != nil || count != 1 {
		t.Fatal("helper timeout victory not archived", err)
	}
}

func TestCatanHelpersTimeoutResumesParallelDiscard(t *testing.T) {
	s, _, clients, id := newCatanTable(t)
	s.mu.Lock()
	r := s.rooms[id]
	g := r.Game.Catan
	g.Options = game.CatanOptions{Helpers: true}
	g.TurnSerial = 2
	g.Players[0].Helper = &game.CatanHelperSeat{ID: 5}
	g.HelperDisplay = []int{1, 2, 3}
	g.HelperPending = &game.CatanHelperPending{Player: 0, Kind: "exchange", Resume: "catan_discard"}
	g.DiscardDue = []int{0, 1, 1}
	for i := 1; i <= 2; i++ {
		g.Players[i].Resources[0]++
		g.Bank[0]--
	}
	r.Game.Phase = "catan_helper"
	r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
	s.expireSetups(time.Now())
	if s.rooms[id].CatanPendingVersion != s.rooms[id].Version {
		t.Fatal("timeout did not set concurrent discard baseline")
	}
	s.mu.Unlock()
	initial := current(clients[1])
	clients[1].command(initial, "action", map[string]any{"type": "catan_discard", "tokens": []int{1, 0, 0, 0, 0}}, 200)
	clients[2].command(initial, "action", map[string]any{"type": "catan_discard", "tokens": []int{1, 0, 0, 0, 0}}, 200)
	if s.rooms[id].Game.Phase != "catan_robber" {
		t.Fatal("concurrent discards failed after helper timeout")
	}
}

func TestCatanFiveSixRoomCapacityAndOptionFreeze(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("六人岛主")
	host.post("/api/rooms", map[string]any{"name": "wrong", "kind": "catan", "capacity": 6}, 400)
	host.post("/api/rooms", map[string]any{"name": "wrong", "kind": "catan", "capacity": 4, "catanOptions": game.CatanOptions{FiveSix: true}}, 400)
	room := host.post("/api/rooms", map[string]any{"name": "五至六人", "kind": "catan", "capacity": 6, "catanOptions": game.CatanOptions{FiveSix: true, Helpers: true}}, 201)
	id := room["id"].(string)
	for i := 0; i < 3; i++ {
		host.command(current(host), "add_bot", nil, 200)
	}
	host.command(current(host), "ready", nil, 200)
	host.command(current(host), "start", nil, 400)
	host.command(current(host), "add_bot", nil, 200)
	change := func(options game.CatanOptions, want int) {
		r := current(host)
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": options, "version": r["version"], "nonce": randomID(12)}, want)
	}
	change(game.CatanOptions{}, 400)
	host.command(current(host), "start", nil, 200)
	change(game.CatanOptions{FiveSix: true}, 400)
	if len(s.rooms[id].Game.Catan.Tiles) != 30 || s.rooms[id].Game.Catan.Paired == nil {
		t.Fatal("five-player game lost extension")
	}
	for s.rooms[id].Game.Catan.SetupStep < 10 {
		s.mu.Lock()
		s.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
		s.expireSetups(time.Now())
		s.mu.Unlock()
	}
	r := s.rooms[id]
	r.Game.Phase = "catan_turn"
	actor := r.Game.Turn
	now := time.Now()
	r.TurnDeadline = now.Add(20 * time.Second).UnixMilli()
	if err := r.applyGameAction(actor, game.Action{Type: "catan_end"}, now); err != nil {
		t.Fatal(err)
	}
	if r.Game.Turn == actor || !r.Game.Catan.Paired.Second || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("secondary action lacks own deadline")
	}
	raw, _ := json.Marshal(r)
	var restored Room
	if err := json.Unmarshal(raw, &restored); err != nil || !restored.CatanOptions.FiveSix || !restored.Game.Catan.Paired.Second {
		t.Fatal("pair lost on persistence", err)
	}
}
