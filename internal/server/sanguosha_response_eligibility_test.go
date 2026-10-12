package server

// Failure scenarios fixed before changing the engine: only real responders may
// receive canRespond or submit a pass, remaining responders share one clock and
// may use the same version, reload/restart cannot restore empty prompts, timeout
// must not take over non-responders, and no-candidate tricks resolve immediately.

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func sgResponseHTTPFixture(t *testing.T) (*Server, *httptest.Server, []*testClient, string, func(int, string) int) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := []*testClient{}
	for i := range 5 {
		c := newClient(t, ts.URL)
		c.register(fmt.Sprintf("响应验收%d", i))
		clients = append(clients, c)
	}
	a := clients[0]
	r := a.post("/api/rooms", map[string]any{"kind": "sanguosha", "name": "响应资格", "capacity": 4}, 201)
	id := r["id"].(string)
	for i := 1; i < 4; i++ {
		clients[i].command(current(a), "join", nil, 200)
	}
	for i := range 4 {
		clients[i].command(current(a), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	rm := s.rooms[id]
	g := rm.Game
	sg := g.Sanguosha
	catalog := g.View(0)["sanguosha"].(map[string]any)["cards"].([]game.SGCard)
	g.Turn, g.Phase, g.Round = 0, "sg_play", 1
	sg.Selecting, sg.Selected, sg.InPlay, sg.ActivePhase = false, 4, true, "play"
	sg.Pending, sg.Queue, sg.Table, sg.Discard = nil, nil, nil, nil
	sg.Deck = nil
	for _, card := range catalog {
		sg.Deck = append(sg.Deck, card.ID)
	}
	for i := range sg.Players {
		sg.Players[i] = game.SGPlayer{General: "zhangfei", Role: []string{"lord", "rebel", "renegade", "loyalist"}[i], HP: 4, MaxHP: 4, Hand: []int{}, Equip: []int{}, Judgment: []game.SGDelayed{}, Used: map[string]int{}}
	}
	sg.Lord = 0
	rm.TurnDeadline = time.Now().Add(120 * time.Second).UnixMilli()
	rm.SGTimeLeft = 120000
	give := func(player int, kind string) int {
		for index, id := range sg.Deck {
			for _, card := range catalog {
				if card.ID == id && card.Kind == kind {
					sg.Deck = append(sg.Deck[:index], sg.Deck[index+1:]...)
					sg.Players[player].Hand = append(sg.Players[player].Hand, id)
					return id
				}
			}
		}
		t.Fatalf("missing %s", kind)
		return 0
	}
	clients[4].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	return s, ts, clients, id, give
}

func TestSanguoshaResponseEligibilityHTTP(t *testing.T) {
	trace := []any{}
	t.Cleanup(func() {
		dir := filepath.Join("..", "..", ".local", "sanguosha-responses")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "trace": trace}, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "http-acceptance.json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	t.Run("no candidate resolves without a prompt", func(t *testing.T) {
		s, _, clients, id, give := sgResponseHTTPFixture(t)
		card := give(0, "ex_nihilo")
		deadline := s.rooms[id].TurnDeadline
		clients[0].command(current(clients[0]), "action", game.Action{Type: "sg_play", Cards: []int{card}}, 200)
		if q := s.rooms[id].Game.Sanguosha.Pending; q != nil {
			t.Fatalf("empty response window: %+v", q)
		}
		if s.rooms[id].TurnDeadline != deadline {
			t.Fatal("automatic skip reset turn budget")
		}
		trace = append(trace, map[string]any{"scenario": "no candidate", "room": current(clients[0])})
	})
	t.Run("eligible seats privacy shared clock and restart", func(t *testing.T) {
		s, ts, clients, id, give := sgResponseHTTPFixture(t)
		card := give(0, "ex_nihilo")
		give(1, "nullification")
		give(2, "nullification")
		clients[0].command(current(clients[0]), "action", game.Action{Type: "sg_play", Cards: []int{card}}, 200)
		q := s.rooms[id].Game.Sanguosha.Pending
		if q == nil || q.Kind != "nullification" {
			t.Fatal("counter window missing")
		}
		deadline := s.rooms[id].TurnDeadline
		for viewer, c := range clients {
			view := current(c)["game"].(map[string]any)["sanguosha"].(map[string]any)
			pending := view["pending"].(map[string]any)
			if pending["canRespond"] != (viewer == 1 || viewer == 2) {
				t.Fatal("wrong prompt visibility", viewer, pending)
			}
			for owner, player := range view["players"].([]any) {
				if owner != viewer && player.(map[string]any)["hand"] != nil {
					t.Fatal("eligibility leaked another hand")
				}
			}
		}
		before := current(clients[0])
		clients[3].command(before, "action", game.Action{Prompt: q.ID, Choice: "pass"}, 400)
		if !reflect.DeepEqual(before, current(clients[0])) {
			t.Fatal("ineligible pass changed state")
		}
		clients[1].command(before, "action", game.Action{Prompt: q.ID, Choice: "pass"}, 200)
		if s.rooms[id].TurnDeadline != deadline {
			t.Fatal("pass reset response clock")
		}
		saved := current(clients[2])
		ts.Close()
		s.Close()
		next, err := New(s.cfg, s.files)
		if err != nil {
			t.Fatal(err)
		}
		stopBotTicker(next)
		ts2 := httptest.NewServer(next.Handler())
		t.Cleanup(func() { ts2.Close(); next.Close() })
		for _, client := range clients {
			client.base = ts2.URL
		}
		if !reflect.DeepEqual(saved, current(clients[2])) {
			t.Fatal("restart changed response eligibility")
		}
		clients[2].command(before, "action", game.Action{Prompt: q.ID, Choice: "pass"}, 200)
		if next.rooms[id].Game.Sanguosha.Pending != nil {
			t.Fatal("still waiting after eligible seats passed")
		}
		trace = append(trace, map[string]any{"scenario": "filtered response restart", "before": saved, "after": current(clients[2])})
	})
	t.Run("only eligible seats time out", func(t *testing.T) {
		s, _, clients, id, give := sgResponseHTTPFixture(t)
		card := give(0, "ex_nihilo")
		give(1, "nullification")
		clients[0].command(current(clients[0]), "action", game.Action{Type: "sg_play", Cards: []int{card}}, 200)
		r := s.rooms[id]
		deadline := r.TurnDeadline
		s.mu.Lock()
		s.expireSetups(time.UnixMilli(deadline + 1))
		s.mu.Unlock()
		for i, seat := range s.rooms[id].Seats {
			if seat.AutoPlay != (i == 1) || seat.TimeoutAutoPlay != (i == 1) {
				t.Fatal("timeout took over a non-responder", i)
			}
		}
		if q := s.rooms[id].Game.Sanguosha.Pending; q != nil && q.Kind == "nullification" {
			t.Fatal("timeout did not settle window", q)
		}
		trace = append(trace, map[string]any{"scenario": "filtered timeout", "room": current(clients[0])})
	})
	t.Run("old empty response window resumes on restart", func(t *testing.T) {
		s, ts, clients, id, give := sgResponseHTTPFixture(t)
		card := give(0, "ex_nihilo")
		counter := give(1, "nullification")
		clients[0].command(current(clients[0]), "action", game.Action{Type: "sg_play", Cards: []int{card}}, 200)
		r := s.rooms[id]
		// Reproduce a snapshot written by the old all-player response behavior.
		r.Game.Sanguosha.Players[1].Hand = nil
		r.Game.Sanguosha.Discard = append(r.Game.Sanguosha.Discard, counter)
		if err := s.save(r); err != nil {
			t.Fatal(err)
		}
		ts.Close()
		s.Close()
		next, err := New(s.cfg, s.files)
		if err != nil {
			t.Fatal(err)
		}
		stopBotTicker(next)
		ts2 := httptest.NewServer(next.Handler())
		t.Cleanup(func() { ts2.Close(); next.Close() })
		clients[0].base = ts2.URL
		if next.rooms[id].Game.Sanguosha.Pending != nil || len(next.rooms[id].Game.Sanguosha.Players[0].Hand) != 2 {
			t.Fatal("old empty response window is stuck after restart")
		}
		trace = append(trace, map[string]any{"scenario": "old empty window recovery", "room": current(clients[0])})
	})
	t.Run("expired rooms close without advancing old responses", func(t *testing.T) {
		s, ts, clients, id, give := sgResponseHTTPFixture(t)
		card, counter := give(0, "ex_nihilo"), give(1, "nullification")
		clients[0].command(current(clients[0]), "action", game.Action{Type: "sg_play", Cards: []int{card}}, 200)
		r := s.rooms[id]
		r.Game.Sanguosha.Players[1].Hand = nil
		r.Game.Sanguosha.Discard = append(r.Game.Sanguosha.Discard, counter)
		r.LastActive = time.Now().Add(-25 * time.Hour).Unix()
		if err := s.save(r); err != nil {
			t.Fatal(err)
		}
		ts.Close()
		s.Close()
		next, err := New(s.cfg, s.files)
		if err != nil {
			t.Fatal(err)
		}
		defer next.Close()
		stopBotTicker(next)
		if next.rooms[id].Status != "closed" || len(next.rooms[id].Game.Sanguosha.Players[0].Hand) != 0 {
			t.Fatal("response migration advanced an expired room")
		}
		trace = append(trace, map[string]any{"scenario": "expired window closed", "status": next.rooms[id].Status})
	})
}

// An opt-in real-browser fixture; no extra route or fixture enters production.
func TestSanguoshaResponseBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_SG_RESPONSE_BROWSER") != "1" {
		t.Skip("browser acceptance only")
	}
	s, ts, _, id, give := sgResponseHTTPFixture(t)
	card := give(0, "ex_nihilo")
	give(1, "nullification")
	give(2, "slash")
	if err := s.save(s.rooms[id]); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	s.files = os.DirFS(filepath.Join(root, "web", "dist"))
	web := httptest.NewServer(s.Handler())
	defer web.Close()
	defer ts.Close()
	dir := filepath.Join(root, ".local", "sanguosha-responses", "browser")
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"url": web.URL, "card": card, "room": id})
	if err = os.WriteFile(filepath.Join(dir, "fixture.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	for start := time.Now(); time.Since(start) < 5*time.Minute; {
		if _, err = os.Stat(filepath.Join(dir, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}
