package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanGoldHTTPActorsPrivacyRestartAndBotClock(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	s.mu.Lock()
	r := s.rooms[id]
	g := r.Game.Catan
	g.Seafarers = &game.CatanSeafarers{Pirate: -1}
	g.GoldPending = &game.CatanGoldPending{Claims: []game.CatanGoldClaim{{Player: 1, Count: 1}, {Player: 2, Count: 2}}, Resume: "catan_turn", Received: []int{0, 0, 0}}
	r.Game.Turn = 0
	r.Game.Phase = "catan_gold"
	now := time.Now()
	r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
	if !r.adjustCatanResponseClock("catan_roll", -1, now) || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("gold did not pause the active clock")
	}
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_gold", "take": []int{1, 0, 0, 0, 0}}, 400)
	clients[3].command(current(clients[3]), "action", map[string]any{"type": "catan_gold", "take": []int{1, 0, 0, 0, 0}}, 400)
	for i, c := range clients {
		view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		for j, raw := range view["players"].([]any) {
			_, visible := raw.(map[string]any)["resources"]
			if visible != (i == j) {
				t.Fatal("gold choice leaked hand", i, j)
			}
		}
	}
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
		t.Fatal("gold pending/timer lost after restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	// Only the non-active responding seat is in autoplay.
	next.mu.Lock()
	r = next.rooms[id]
	r.Seats[1].AutoPlay = true
	r.BotAt = 0
	botAt := now.Add(10 * time.Second)
	next.runBots(botAt)
	r = next.rooms[id]
	if r.Game.CatanPendingActor() != 2 || r.Game.Turn != 0 || r.TurnDeadline != botAt.Add(turnLimit).UnixMilli() || r.CatanTimeLeft != 45000 {
		t.Fatal("gold bot actor/next claimant clock incorrect")
	}
	next.mu.Unlock()
	clients[2].command(current(clients[2]), "action", map[string]any{"type": "catan_gold", "take": []int{0, 0, 0, 1, 1}}, 200)
	r = next.rooms[id]
	if r.Game.Catan.GoldPending != nil || r.Game.Phase != "catan_turn" || r.Game.Turn != 0 {
		t.Fatal("HTTP choice failed to resume active turn")
	}
	remaining := r.TurnDeadline - time.Now().UnixMilli()
	if remaining < 44000 || remaining > 45000 {
		t.Fatal("gold gave the active player a fresh turn", remaining)
	}
}

func TestCatanGoldTimeoutEachClaimantAndHelperChainClock(t *testing.T) {
	s, _, _, id := newCatanTable(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	g := r.Game.Catan
	g.Options.Helpers = true
	g.Seafarers = &game.CatanSeafarers{Pirate: -1}
	g.TurnSerial = 2
	g.Players[2].Helper = &game.CatanHelperSeat{ID: 3}
	g.HelperDisplay = []int{1, 2, 4}
	g.GoldPending = &game.CatanGoldPending{Claims: []game.CatanGoldClaim{{Player: 0, Count: 1}, {Player: 1, Count: 2}}, Resume: "catan_turn", Received: []int{0, 0, 0}}
	r.Game.Turn = 0
	r.Game.Phase = "catan_gold"
	r.CatanTimeLeft = 35000
	now := time.Now()
	r.TurnDeadline = now.Add(-time.Second).UnixMilli()
	s.expireSetups(now)
	r = s.rooms[id]
	if r.Game.CatanPendingActor() != 1 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() || r.CatanTimeLeft != 35000 {
		t.Fatal("timeout consumed all players' choices at once or lost clock")
	}
	later := now.Add(turnLimit)
	s.expireSetups(later)
	r = s.rooms[id]
	if r.Game.Phase != "catan_helper" || r.Game.CatanPendingActor() != 2 || r.TurnDeadline != later.Add(turnLimit).UnixMilli() || r.CatanTimeLeft != 35000 {
		t.Fatal("Hilda chain did not receive its own response window")
	}
	last := later.Add(turnLimit)
	s.expireSetups(last)
	r = s.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.CatanPendingActor() != -1 || r.TurnDeadline != last.Add(35*time.Second).UnixMilli() {
		t.Fatal("timeout chain did not restore original remaining action time")
	}
}

func TestCatanGoldInvalidChoiceCannotRefreshDeadline(t *testing.T) {
	s, _, _, id := newCatanTable(t)
	r := s.rooms[id]
	r.Game.Catan.Seafarers = &game.CatanSeafarers{Pirate: -1}
	r.Game.Catan.GoldPending = &game.CatanGoldPending{Claims: []game.CatanGoldClaim{{Player: 1, Count: 1}}, Resume: "catan_turn"}
	r.Game.Phase = "catan_gold"
	now := time.Now()
	r.TurnDeadline = now.Add(30 * time.Second).UnixMilli()
	r.CatanTimeLeft = 12000
	before, _ := json.Marshal(r)
	if err := r.applyGameAction(1, game.Action{Type: "catan_gold", Take: []int{1, 1, 0, 0, 0}}, now); err == nil {
		t.Fatal("overclaim accepted")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("invalid gold selection mutated game or clock")
	}
}
