package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestSanguoshaHegemonyHTTPFreezePrivacyRestartAndRatings(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, 5)
	for i := range clients {
		clients[i] = newClient(t, ts.URL)
		clients[i].register(fmt.Sprintf("国战玩家%d", i))
	}
	host := clients[0]
	room := host.post("/api/rooms", map[string]any{"name": "国战验收", "kind": "sanguosha", "capacity": 4}, 201)
	id := room["id"].(string)
	for i := 1; i < 4; i++ {
		clients[i].command(current(host), "join", nil, 200)
	}
	for i := range 4 {
		clients[i].command(current(host), "ready", nil, 200)
	}
	change := func(c *testClient, options game.SGOptions, status int) {
		r := current(c)
		c.post("/api/rooms/"+id, map[string]any{"type": "sanguosha_options", "sanguoshaOptions": options, "version": r["version"], "nonce": randomID(12)}, status)
	}
	change(clients[1], game.SGOptions{Mode: "hegemony"}, 400)
	change(host, game.SGOptions{Mode: "hegemony", Deck: "military"}, 400)
	change(host, game.SGOptions{Mode: "hegemony"}, 200)
	host.command(current(host), "start", nil, 400) // A mode change clears readiness.
	for i := range 4 {
		clients[i].command(current(host), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	change(host, game.SGOptions{Mode: "identity"}, 400)
	clients[4].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	clients[4].command(current(clients[4]), "action", game.Action{}, 400)
	for viewer, c := range clients {
		v := current(c)["game"].(map[string]any)["sanguosha"].(map[string]any)
		if v["hegemony"] != true || v["lord"] != float64(-1) || len(v["generals"].([]any)) != 60 || len(v["cards"].([]any)) != 108 {
			t.Fatal("wrong public mode", viewer)
		}
		for seat, raw := range v["players"].([]any) {
			p := raw.(map[string]any)
			_, choices := p["choices"]
			_, hand := p["hand"]
			_, skills := p["ownSkills"]
			_, reveal := p["canReveal"]
			if choices != (seat == viewer) || hand != (seat == viewer) || skills != (seat == viewer) || reveal != (seat == viewer) {
				t.Fatal("private setup fields leaked", viewer, seat)
			}
			if p["equip"] == nil || p["judgment"] == nil {
				t.Fatal("empty browser zones serialized as null")
			}
		}
	}
	if left := time.Until(time.UnixMilli(s.rooms[id].TurnDeadline)); left < 110*time.Second || left > turnLimit {
		t.Fatal("dual-general selection clock", left)
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
		t.Fatal("pending dual-general draft changed on restart")
	}
	for steps := 0; steps < 8000 && !next.rooms[id].Game.Finished; steps++ {
		r := next.rooms[id]
		i := r.Game.SanguoshaActor()
		a, err := r.Game.BotAction(i)
		if err != nil {
			t.Fatal(err)
		}
		clients[i].command(current(clients[i]), "action", a, 200)
		if r.Game.Sanguosha.Selecting && r.TurnDeadline-time.Now().UnixMilli() < 110000 {
			t.Fatal("later general selection lost 120-second clock")
		}
	}
	r := next.rooms[id]
	if r.Status != "finished" || len(r.Game.Winners) == 0 {
		t.Fatal("national-war game did not finish")
	}
	for i, seat := range r.Seats {
		rating, err := next.rating(seat.ID)
		if err != nil {
			t.Fatal(err)
		}
		want := 990
		for _, winner := range r.Game.Winners {
			if winner == i {
				want = 1020
			}
		}
		if rating.Played != 1 || rating.Points != want {
			t.Fatal("wrong faction score", i, rating, want)
		}
	}
	for range 2 {
		if err := next.save(r); err != nil {
			t.Fatal(err)
		}
	}
	for _, seat := range r.Seats {
		rating, _ := next.rating(seat.ID)
		if rating.Played != 1 {
			t.Fatal("duplicate faction scoring")
		}
	}
}

func TestSanguoshaHegemonySelectionTimeoutClock(t *testing.T) {
	s, err := game.NewSanguosha(8, game.SGOptions{Mode: "hegemony"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	r := &Room{Kind: "sanguosha", Game: s, Status: "playing"}
	r.startTurnClock(now)
	for range 8 {
		if r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
			t.Fatal("selection must receive 120 seconds")
		}
		a, err := r.Game.SanguoshaTimeoutAction()
		if err != nil {
			t.Fatal(err)
		}
		i := r.Game.SanguoshaActor()
		now = now.Add(turnLimit)
		if err := r.applyGameAction(i, a, now); err != nil {
			t.Fatal(err)
		}
	}
	if r.Game.Sanguosha.Selecting {
		t.Fatal("mandatory timed-out draft did not finish")
	}
	if r.Game.Sanguosha.Pending != nil && r.TurnDeadline != now.Add(20*time.Second).UnixMilli() {
		t.Fatal("out-of-play response must receive 20 seconds")
	}
}
