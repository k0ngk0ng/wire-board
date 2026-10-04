package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
	"time"
)

func fleetServerFixture(r *Room, total int) {
	g := r.Game.Catan
	g.SetupStep = g.SetupLimit()
	r.Game.Turn = 0
	r.Game.Phase = "catan_fleet_reward"
	for i := range g.Tiles {
		g.Tiles[i].Number = 0
	}
	for i := range g.Players {
		for c, n := range g.Players[i].Resources {
			g.Bank[c] += n
			g.Players[i].Resources[c] = 0
		}
	}
	g.Seafarers = &game.CatanSeafarers{Pirate: -1, PirateIslands: &game.CatanPirateIslands{Raid: &game.CatanPirateRaid{Rewards: []int{1}, Total: total}}}
}
func TestCatanFleetRewardHTTPPrivacyRestartAndClock(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	s.mu.Lock()
	r := s.rooms[id]
	fleetServerFixture(r, 6)
	now := time.Now()
	r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
	if !r.adjustCatanResponseClock("catan_roll", -1, r.Game.Catan.SetupStep, now) || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("reward did not pause action clock")
	}
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	for _, i := range []int{0, 2, 3} {
		clients[i].command(current(clients[i]), "action", map[string]any{"type": "catan_fleet_reward", "color": 0}, 400)
	}
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_fleet_reward", "color": 5}, 400)
	if s.rooms[id].TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("invalid choice refreshed clock")
	}
	for i, c := range clients {
		v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		for j, raw := range v["players"].([]any) {
			p := raw.(map[string]any)
			_, hand := p["resources"]
			_, dev := p["dev"]
			if hand != (i == j) || dev != (i == j) {
				t.Fatal("fleet response leaked hand")
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
		t.Fatal("fleet response/deadline lost on restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_fleet_reward", "color": 4}, 200)
	r = next.rooms[id]
	left := r.TurnDeadline - time.Now().UnixMilli()
	if r.Game.Catan.Seafarers.PirateIslands.Raid != nil || r.Game.Catan.Players[1].Resources[4] != 1 || r.Game.Phase != "catan_turn" || r.Game.Turn != 0 || left < 44000 || left > 45000 {
		t.Fatal("fleet continuation / action clock", left)
	}
}
func TestCatanFleetTimeoutAndAutoplayResumeSeven(t *testing.T) {
	for _, autoplay := range []bool{false, true} {
		s, _, _, id := newCatanTable(t)
		s.mu.Lock()
		r := s.rooms[id]
		fleetServerFixture(r, 7)
		g := r.Game.Catan
		g.Players[1].Resources = []int{7, 0, 0, 0, 0}
		g.Bank[0] -= 7
		now := time.Now()
		r.CatanTimeLeft = 43000
		r.TurnDeadline = now.Add(-time.Second).UnixMilli()
		if autoplay {
			r.Seats[1].AutoPlay = true
			r.BotAt = 0
			r.TurnDeadline = now.Add(time.Minute).UnixMilli()
			s.runBots(now)
		} else {
			s.expireSetups(now)
		}
		r = s.rooms[id]
		if r.Game.Phase != "catan_discard" || r.Game.Catan.DiscardDue[1] != 4 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() || r.CatanPendingVersion == 0 {
			t.Fatal("fleet reward did not start fresh discard window", autoplay, r.Game.Phase)
		}
		s.mu.Unlock()
	}
}
