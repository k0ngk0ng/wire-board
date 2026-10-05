package server

import (
	"encoding/json"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func cityEventFixture(t *testing.T, r *Room, kind string, now time.Time) {
	t.Helper()
	catanCityResponseFixture(t, r, kind, []int{1}, now)
	g := r.Game.Catan
	k := g.CitiesKnights
	for i := range g.Tiles {
		g.Tiles[i].Number = 0
		g.Tiles[i].Resource = 5
	}
	k.RobberStart = 1
	k.BarbarianPosition = 7
	k.Event = &game.CatanCityEvent{Red: 1, Yellow: 5, Face: 3, Attack: true, Tasks: []game.CatanCityEventTask{{Kind: kind, Player: 1}, {Kind: kind, Player: 2}}}
	for p := range 3 {
		g.Vertices[p].Owner, g.Vertices[p].Level = p, 2
	}
	g.Tiles[0].Resource, g.Tiles[0].Number, g.Tiles[0].Vertices = 0, 6, []int{0, 1, 2}
}
func cityProgressGive(t *testing.T, g *game.Catan, p int, cards ...int) {
	t.Helper()
	rules := game.CatanProgressRules()
	for _, c := range cards {
		track := rules[c].Track
		at := slices.Index(g.CitiesKnights.ProgressDecks[track], c)
		if at < 0 {
			t.Fatal("fixture progress card not in deck")
		}
		g.CitiesKnights.ProgressDecks[track] = slices.Delete(g.CitiesKnights.ProgressDecks[track], at, at+1)
		g.CitiesKnights.Players[p].Progress = append(g.CitiesKnights.Players[p].Progress, c)
	}
}
func TestCatanCityEventsHTTPPillageRestartAutoplayAndProduction(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	now := time.Now()
	s.mu.Lock()
	r := s.rooms[id]
	cityEventFixture(t, r, "pillage", now)
	r.Game.Catan.CitiesKnights.Knights = []game.CatanKnight{{Owner: 0, Vertex: 10, Strength: 1, Active: true}}
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	before, _ := json.Marshal(r)
	for _, p := range []int{0, 2, 3} {
		clients[p].command(current(clients[p]), "action", map[string]any{"type": "catan_pillage", "vertex": 1}, 400)
	}
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_pillage", "vertex": 0}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid pillage changed room/clock")
	}
	for viewer, c := range clients {
		k := current(c)["game"].(map[string]any)["catan"].(map[string]any)["citiesKnights"].(map[string]any)
		if _, ok := k["progressDecks"]; ok {
			t.Fatal("hidden progress decks exposed")
		}
		for p, raw := range k["players"].([]any) {
			_, ok := raw.(map[string]any)["progress"]
			if ok != (viewer == p) {
				t.Fatal("private progress hand exposed")
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
		t.Fatal("barbarian pending tasks lost on restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	next.mu.Lock()
	r = next.rooms[id]
	r.Seats[1].AutoPlay = true
	r.BotAt = 0
	botAt := now.Add(10 * time.Second)
	next.runBots(botAt)
	r = next.rooms[id]
	if r.Game.CatanPendingActor() != 2 || r.Game.Catan.Vertices[1].Level != 1 || r.TurnDeadline != botAt.Add(turnLimit).UnixMilli() || r.CatanTimeLeft != 45000 {
		t.Fatal("pillage autoplay/next-player clock")
	}
	if r.Game.Catan.Players[0].Resources[0] != 0 {
		t.Fatal("production ran before all cities pillaged")
	}
	later := botAt.Add(turnLimit)
	next.expireSetups(later)
	r = next.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.Catan.CitiesKnights.Event != nil || r.Game.Catan.CitiesKnights.Invasions != 1 || r.TurnDeadline != later.Add(45*time.Second).UnixMilli() {
		t.Fatal("event completion did not restore original action clock")
	}
	for p := range 3 {
		hand := r.Game.Catan.Players[p].Resources
		if hand[0] != 1 || (p == 0 && hand[5] != 1) || (p != 0 && hand[5] != 0) {
			t.Fatal("post-pillage production wrong", p, hand)
		}
	}
	next.mu.Unlock()
}
func TestCatanCityEventsRewardDiscardResponseClocks(t *testing.T) {
	s, _, clients, id := newCatanTable(t)
	s.mu.Lock()
	r := s.rooms[id]
	now := time.Now()
	cityEventFixture(t, r, "defender_reward", now)
	cityProgressGive(t, r.Game.Catan, 1, 3, 4, 5, 6)
	deck := r.Game.Catan.CitiesKnights.ProgressDecks[0]
	at := slices.Index(deck, 0)
	deck = slices.Delete(deck, at, at+1)
	r.Game.Catan.CitiesKnights.ProgressDecks[0] = append(deck, 0)
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_defender_reward", "color": 0}, 200)
	r = s.rooms[id]
	if r.Game.Phase != "catan_progress_discard" || r.Game.CatanPendingActor() != 1 || r.CatanTimeLeft != 45000 {
		t.Fatal("reward discard lost response clock")
	}
	remaining := r.TurnDeadline - time.Now().UnixMilli()
	if remaining < 119000 || remaining > 120000 {
		t.Fatal("reward discard did not receive fresh window")
	}
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_progress_discard", "cards": []int{3}}, 200)
	r = s.rooms[id]
	if r.Game.Phase != "catan_defender_reward" || r.Game.CatanPendingActor() != 2 || r.CatanTimeLeft != 45000 {
		t.Fatal("next reward did not resume after discard")
	}
	s.mu.Lock()
	atTime := time.UnixMilli(r.TurnDeadline)
	s.expireSetups(atTime)
	r = s.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.CatanPendingActor() != -1 || r.TurnDeadline != atTime.Add(45*time.Second).UnixMilli() {
		t.Fatal("reward/discard/next reward clock chain")
	}
	s.mu.Unlock()
}
func TestCatanCityEventsEndHandLimitGetsNextFullTurn(t *testing.T) {
	for _, n := range []int{3, 5} {
		state, err := game.NewCatanCitiesKnights(n, game.CatanOptions{FiveSix: n > 4})
		if err != nil {
			t.Fatal(err)
		}
		state.Catan.SetupStep = 2 * n
		state.Catan.TurnSerial = 1
		state.Turn, state.Phase = 0, "catan_turn"
		if n > 4 {
			state.Catan.Paired.Primary, state.Catan.Paired.Secondary = 0, 3
		}
		cityProgressGive(t, state.Catan, 0, 0, 1, 2, 3, 4)
		now := time.Now()
		r := &Room{Game: state, Status: "playing", TurnDeadline: now.Add(33 * time.Second).UnixMilli()}
		if err = r.applyGameAction(0, game.Action{Type: "catan_end"}, now); err != nil {
			t.Fatal(err)
		}
		if r.Game.Phase != "catan_progress_end" || r.CatanTimeLeft != 33000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
			t.Fatal("end-turn progress choice window")
		}
		later := now.Add(10 * time.Second)
		if err = r.applyGameAction(0, game.Action{Type: "catan_progress_discard", Cards: []int{0}}, later); err != nil {
			t.Fatal(err)
		}
		want := 1
		if n > 4 {
			want = 3
		}
		if r.Game.Turn != want || r.TurnDeadline != later.Add(turnLimit).UnixMilli() {
			t.Fatal("next primary/paired player inherited previous player's remaining time")
		}
	}
}
