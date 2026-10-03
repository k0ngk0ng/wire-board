package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSanguoshaHTTPFinishRatingsAndRestart(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := []*testClient{}
	for i := range 5 {
		c := newClient(t, ts.URL)
		c.register(fmt.Sprintf("群雄%d", i))
		clients = append(clients, c)
	}
	a := clients[0]
	a.post("/api/rooms", map[string]any{"name": "bad", "kind": "sanguosha", "capacity": 3}, 400)
	room := a.post("/api/rooms", map[string]any{"name": "三国验证", "kind": "sanguosha", "capacity": 4}, 201)
	id := room["id"].(string)
	for i := 1; i < 4; i++ {
		clients[i].command(current(a), "join", nil, 200)
	}
	for i := range 4 {
		clients[i].command(current(a), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	clients[4].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	clients[4].command(current(clients[4]), "action", game.Action{}, 400)
	view := current(clients[4])["game"].(map[string]any)["sanguosha"].(map[string]any)
	if _, ok := view["queue"]; ok {
		t.Fatal("private effects exposed")
	}
	saved, _ := json.Marshal(s.rooms[id])
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	stopBotTicker(next)
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	restored, _ := json.Marshal(next.rooms[id])
	if string(saved) != string(restored) {
		t.Fatal("restart changed pending selection")
	}
	for steps := 0; steps < 6000 && !next.rooms[id].Game.Finished; steps++ {
		r := next.rooms[id]
		i := r.Game.SanguoshaActor()
		action, err := r.Game.BotAction(i)
		if err != nil {
			t.Fatal(err)
		}
		clients[i].command(current(clients[i]), "action", action, 200)
	}
	r := next.rooms[id]
	if r.Status != "finished" {
		t.Fatal("match did not finish")
	}
	for i, seat := range r.Seats {
		rating, err := next.rating(seat.ID)
		if err != nil {
			t.Fatal(err)
		}
		want := 990
		for _, j := range r.Game.Winners {
			if i == j {
				want = 1020
			}
		}
		if rating.Points != want || rating.Played != 1 {
			t.Fatal(i, rating, want)
		}
	}
	for range 3 {
		if err := next.save(r); err != nil {
			t.Fatal(err)
		}
	}
	for _, seat := range r.Seats {
		rating, _ := next.rating(seat.ID)
		if rating.Played != 1 {
			t.Fatal("duplicate score")
		}
	}
	code, profile := a.request("GET", "/api/players/"+r.Seats[0].ID, nil)
	if code != 200 || profile["stats"].(map[string]any)["sanguosha"].(map[string]any)["played"] != float64(1) {
		t.Fatal(profile)
	}
}
func TestSanguoshaClockPausesAndTimeoutResponds(t *testing.T) {
	g, _ := game.New("sanguosha", 4)
	for g.Sanguosha.Selecting || g.Sanguosha.Pending != nil {
		a, err := g.BotAction(g.SanguoshaActor())
		if err != nil {
			t.Fatal(err)
		}
		g.Apply(g.SanguoshaActor(), a)
	}
	r := &Room{Game: g, Kind: "sanguosha", Status: "playing"}
	now := time.Now()
	r.startTurnClock(now)
	r.TurnDeadline = now.Add(57 * time.Second).UnixMilli()
	i := g.Turn
	g.Sanguosha.Players[i].General = "zhouyu"
	// Fanjian pauses this player's remaining thinking time while the target guesses.
	target := (i + 1) % 4
	if err := r.applyGameAction(i, game.Action{Type: "sg_skill", Skill: "fanjian", Targets: []int{target}}, now); err != nil {
		t.Fatal(err)
	}
	if r.SGTimeLeft != 57000 || r.TurnDeadline != now.Add(20*time.Second).UnixMilli() {
		t.Fatal("response clock", r.SGTimeLeft, r.TurnDeadline)
	}
	a, err := g.SanguoshaTimeoutAction()
	if err != nil {
		t.Fatal(err)
	}
	if err = r.applyGameAction(target, a, now.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	// Damage can trigger optional skills; complete these using their own windows.
	for r.Game.Sanguosha.Pending != nil && !r.Game.Finished {
		now = now.Add(20 * time.Second)
		a, _ = r.Game.SanguoshaTimeoutAction()
		if err = r.applyGameAction(r.Game.SanguoshaActor(), a, now); err != nil {
			t.Fatal(err)
		}
	}
	if r.SGTimeLeft != 57000 {
		t.Fatal("thinking time reset", r.SGTimeLeft)
	}
}
func TestSanguoshaConcurrentHTTPPassAndSharedTimeout(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := []*testClient{}
	for i := range 4 {
		c := newClient(t, ts.URL)
		c.register(fmt.Sprintf("响应者%d", i))
		clients = append(clients, c)
	}
	a := clients[0]
	a.post("/api/rooms", map[string]any{"name": "共同响应", "kind": "sanguosha", "capacity": 4}, 201)
	for i := 1; i < 4; i++ {
		clients[i].command(current(a), "join", nil, 200)
	}
	for _, c := range clients {
		c.command(current(a), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	room := current(a)
	id := room["id"].(string)
	for s.rooms[id].Game.Sanguosha.Selecting || s.rooms[id].Game.Sanguosha.Pending != nil {
		g := s.rooms[id].Game
		i := g.SanguoshaActor()
		action, _ := g.BotAction(i)
		clients[i].command(current(clients[i]), "action", action, 200)
	}
	s.mu.Lock()
	r := s.rooms[id]
	g := r.Game
	actor := g.Turn
	// Build an ordinary real-card effect, then use only HTTP for all responses.
	var card int
	for _, c := range g.View(actor)["sanguosha"].(map[string]any)["cards"].([]game.SGCard) {
		if c.Kind == "ex_nihilo" {
			card = c.ID
			break
		}
	}
	for i := range g.Sanguosha.Players {
		p := &g.Sanguosha.Players[i]
		for j, v := range p.Hand {
			if v == card {
				p.Hand = append(p.Hand[:j], p.Hand[j+1:]...)
				break
			}
		}
		p.General = "zhangfei"
	}
	for j, v := range g.Sanguosha.Deck {
		if v == card {
			g.Sanguosha.Deck = append(g.Sanguosha.Deck[:j], g.Sanguosha.Deck[j+1:]...)
			break
		}
	}
	g.Sanguosha.Players[actor].Hand = append(g.Sanguosha.Players[actor].Hand, card)
	if err := r.applyGameAction(actor, game.Action{Type: "sg_play", Cards: []int{card}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	snapshot := current(a)
	q := s.rooms[id].Game.Sanguosha.Pending.ID
	deadline := s.rooms[id].TurnDeadline
	clients[1].command(snapshot, "action", game.Action{Prompt: q, Choice: "pass"}, 200)
	clients[2].command(snapshot, "action", game.Action{Prompt: q, Choice: "pass"}, 200)
	if s.rooms[id].TurnDeadline != deadline {
		t.Fatal("parallel pass reset shared clock")
	}
	clients[1].command(snapshot, "action", game.Action{Prompt: q, Choice: "pass"}, 400)
	s.mu.Lock()
	s.expireSetups(time.UnixMilli(deadline + 1))
	s.mu.Unlock()
	if s.rooms[id].Game.Sanguosha.Pending != nil {
		t.Fatal("shared timeout should finish all remaining passes")
	}
	clients[3].command(snapshot, "action", game.Action{Prompt: q, Choice: "pass"}, 409)
}
