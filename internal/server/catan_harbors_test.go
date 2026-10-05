package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Internally provisioned local fixture. This does not enable public recipes.
func TestCatanHarborsPortHTTPRestartAndWinningResponses(t *testing.T) {
	for _, mode := range []string{"manual", "timeout", "autoplay"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newCatanTable(t)
			clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			s.mu.Lock()
			r := s.rooms[id]
			edge, cityEdge := tribeServerFixture(t, r)
			g := r.Game.Catan
			g.Harbors = &game.CatanHarbors{Rules: game.CatanHarborsRules, Owner: -1}
			g.Ports = []game.CatanPort{{Edge: cityEdge, Resource: -1}}
			g.Vertices[g.Edges[cityEdge].A].Level = 2
			g.Seafarers.Tribe.Points[0] = 9
			now := time.Now()
			r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
			if err := r.applyGameAction(0, game.Action{Type: "catan_ship", Edge: edge}, now); err != nil {
				t.Fatal(err)
			}
			if r.Game.Phase != "catan_port" || r.Game.Catan.Players[0].Score != 12 || r.Game.Catan.Harbors.Owner != -1 || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
				t.Fatal("held port counted or response clock incorrect")
			}
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			before, _ := json.Marshal(r)
			for _, p := range []int{1, 2, 3} {
				clients[p].command(current(clients[p]), "action", map[string]any{"type": "catan_port", "edge": edge}, 400)
			}
			clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_port", "edge": edge, "slot": 99}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid action changed room")
			}
			for viewer, c := range clients {
				v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
				h := v["harbors"].(map[string]any)
				if v["victoryTarget"].(float64) != 14 || h["points"].([]any)[0].(float64) != 2 || h["owner"].(float64) != -1 {
					t.Fatal("public award/target")
				}
				for owner, raw := range v["players"].([]any) {
					p := raw.(map[string]any)
					for _, key := range []string{"dev", "resources"} {
						if _, visible := p[key]; visible != (owner == viewer) {
							t.Fatal("hand privacy", viewer, owner, key)
						}
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
				t.Fatal("award/port/clock lost on restart")
			}
			ts2 := httptest.NewServer(next.Handler())
			defer ts2.Close()
			for _, c := range clients {
				c.base = ts2.URL
			}
			switch mode {
			case "manual":
				clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_port", "edge": edge, "slot": 0}, 200)
			case "timeout":
				next.mu.Lock()
				next.expireSetups(time.UnixMilli(next.rooms[id].TurnDeadline))
				next.mu.Unlock()
			case "autoplay":
				next.mu.Lock()
				next.rooms[id].Seats[0].AutoPlay = true
				next.rooms[id].BotAt = 0
				next.runBots(time.Now())
				next.mu.Unlock()
			}
			r = next.rooms[id]
			if !r.Game.Finished || r.Status != "finished" || r.Game.Phase != "finished" || r.TurnDeadline != 0 || r.Game.CatanPendingActor() != -1 || r.Game.Catan.Seafarers.Tribe.Pending != nil || r.Game.Catan.Harbors.Owner != 0 || r.Game.Catan.Players[0].Score != 14 || len(r.Game.Winners) != 1 || r.Game.Winners[0] != 0 {
				t.Fatal("port did not finish room cleanly", r.Status, r.Game.Phase, r.Game.Catan.Players[0].Score)
			}
			clients[3].post("/api/rooms/"+id+"/watch", map[string]any{"leave": true}, 200)
			clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
		})
	}
}
