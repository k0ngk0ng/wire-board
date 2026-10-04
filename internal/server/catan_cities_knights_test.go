package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// These fixtures exercise the economic response protocol, not a complete C&K
// game: the event die, progress cards and barbarian pipeline are still gated.
func catanCityResponseFixture(t *testing.T, r *Room, kind string, players []int, now time.Time) {
	t.Helper()
	state, err := game.NewCatanCitiesKnights(3, game.CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	state.Catan.SetupStep = 6
	state.Catan.TurnSerial = 1
	state.Turn = 0
	state.Phase = "catan_" + kind
	state.Catan.CitiesKnights.Pending = &game.CatanCityPending{Kind: kind, Players: players}
	r.Game = state
	r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
	if !r.adjustCatanResponseClock("catan_turn", -1, 6, now) || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("city response failed to pause original clock")
	}
}

func TestCatanCitiesKnightsHTTPPrivacyRestartAutoplayAndClock(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	now := time.Now()
	s.mu.Lock()
	r := s.rooms[id]
	catanCityResponseFixture(t, r, "aqueduct", []int{1, 2}, now)
	for p := range 3 {
		r.Game.Catan.Players[p].Resources[5+p] = 2
		r.Game.Catan.Bank[5+p] -= 2
	}
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	before, _ := json.Marshal(s.rooms[id])
	for _, p := range []int{0, 2, 3} {
		clients[p].command(current(clients[p]), "action", map[string]any{"type": "catan_aqueduct", "color": 0}, 400)
	}
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_aqueduct", "color": 5}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid HTTP city choice mutated state/deadline")
	}
	for viewer, client := range clients {
		v := current(client)["game"].(map[string]any)["catan"].(map[string]any)
		for p, raw := range v["players"].([]any) {
			hand, visible := raw.(map[string]any)["resources"]
			if visible != (viewer == p) {
				t.Fatal("private resource/commodity vector leaked", viewer, p)
			}
			if visible && len(hand.([]any)) != 8 {
				t.Fatal("own commodities unavailable")
			}
		}
	}
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	stopBotTicker(next)
	after, _ = json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("city pending state/clock changed on restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, client := range clients {
		client.base = ts2.URL
	}
	next.mu.Lock()
	r = next.rooms[id]
	r.Seats[1].AutoPlay = true
	r.BotAt = 0
	botAt := now.Add(10 * time.Second)
	next.runBots(botAt)
	r = next.rooms[id]
	if r.Game.CatanPendingActor() != 2 || r.Game.Turn != 0 || r.TurnDeadline != botAt.Add(turnLimit).UnixMilli() || r.CatanTimeLeft != 45000 {
		t.Fatal("autoplay responded for wrong seat or failed to reset next response clock")
	}
	next.mu.Unlock()
	clients[2].command(current(clients[2]), "action", map[string]any{"type": "catan_aqueduct", "color": 4}, 200)
	r = next.rooms[id]
	remaining := r.TurnDeadline - time.Now().UnixMilli()
	if r.Game.Phase != "catan_turn" || r.Game.CatanPendingActor() != -1 || remaining < 44000 || remaining > 45000 {
		t.Fatal("city response did not restore original action time", remaining)
	}
}

func TestCatanCitiesKnightsTimeoutSeparateResponsesAndMetropolis(t *testing.T) {
	s, _, _, id := newCatanTable(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	r := s.rooms[id]
	catanCityResponseFixture(t, r, "aqueduct", []int{1, 2}, now)
	r.TurnDeadline = now.UnixMilli()
	s.expireSetups(now)
	r = s.rooms[id]
	if r.Game.CatanPendingActor() != 2 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("timeout consumed multiple claimants")
	}
	later := now.Add(turnLimit)
	s.expireSetups(later)
	r = s.rooms[id]
	if r.Game.Phase != "catan_turn" || r.TurnDeadline != later.Add(45*time.Second).UnixMilli() {
		t.Fatal("timeout failed to restore remaining time")
	}
	// An actual improvement purchase enters the metropolis response and persists.
	catanCityResponseFixture(t, r, "metropolis", []int{0}, now)
	g := r.Game.Catan
	g.CitiesKnights.Pending = nil
	r.Game.Phase = "catan_turn"
	r.TurnDeadline = now.Add(31 * time.Second).UnixMilli()
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.CitiesKnights.Players[0].Improvements[0] = 3
	g.Players[0].Resources[5], g.Bank[5] = 4, 8
	if err := r.applyGameAction(0, game.Action{Type: "catan_improvement", Color: 0}, now); err != nil {
		t.Fatal(err)
	}
	if r.Game.Phase != "catan_metropolis" || r.CatanTimeLeft != 31000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("metropolis purchase failed to open response clock")
	}
	before, _ := json.Marshal(r)
	if err := r.applyGameAction(0, game.Action{Type: "catan_metropolis", Vertex: 1}, now.Add(time.Second)); err == nil {
		t.Fatal("invalid site accepted")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("invalid site mutated clock or payment")
	}
	s.expireSetups(later)
	r = s.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.Catan.CitiesKnights.Metropolises[0] != 0 || r.Game.Catan.Players[0].Score != 4 || r.TurnDeadline != later.Add(31*time.Second).UnixMilli() {
		t.Fatal("metropolis timeout failed to place and restore clock")
	}
}
