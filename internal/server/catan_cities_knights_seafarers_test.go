package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCatanCitiesKnightsSeafarersHTTPResponsesRestart(t *testing.T) {
	for _, kind := range []string{"gold", "diplomacy"} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				s, ts, clients, id := newCatanTable(t)
				clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				state, err := game.NewCatanCitiesKnightsSeafarers(3, game.CatanOptions{}, game.CatanSeafarersSetup{Scenario: "shores"}, nil)
				if err != nil {
					t.Fatal(err)
				}
				g := state.Catan
				k := g.CitiesKnights
				g.SetupStep = 6
				g.TurnSerial = 1
				state.Turn = 0
				state.Phase = "catan_turn"
				now := time.Now()
				edge := -1
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
				if kind == "gold" {
					k.Players[2].Improvements[0] = 3
					g.GoldPending = &game.CatanGoldPending{Claims: []game.CatanGoldClaim{{Player: 1, Count: 2}}, Received: []int{1, 0, 0}, Resume: "catan_turn"}
					state.Phase = "catan_gold"
					if !r.adjustCatanResponseClock("catan_turn", -1, 6, now) {
						t.Fatal("gold did not enter response")
					}
				} else {
					for _, e := range g.Edges {
						for _, tile := range e.Tiles {
							if g.Tiles[tile].Resource == game.CatanSea {
								edge = e.ID
								g.Seafarers.Pirate = tile
								break
							}
						}
						if edge >= 0 {
							break
						}
					}
					if edge < 0 {
						t.Fatal("no sea edge")
					}
					g.Edges[edge].Owner = 0
					g.Edges[edge].Ship = true
					root := g.Edges[edge].A
					g.Vertices[root].Owner = 0
					g.Vertices[root].Level = 1
					cityProgressGive(t, g, 0, 16)
				}
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				if kind == "diplomacy" {
					clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_progress", "card": 16, "edge": edge}, 200)
					if !s.rooms[id].Game.Catan.CitiesKnights.Pending.Ship {
						t.Fatal("ship response missing")
					}
				}
				if s.rooms[id].CatanTimeLeft < 44000 || s.rooms[id].CatanTimeLeft > 45000 {
					t.Fatal("original clock not paused")
				}
				before, _ := json.Marshal(s.rooms[id])
				actor := 0
				request := map[string]any{"type": "catan_diplomacy", "edge": edge}
				if kind == "gold" {
					actor = 1
					request = map[string]any{"type": "catan_gold", "take": []int{1, 1, 0, 0, 0}}
					clients[actor].command(current(clients[actor]), "action", map[string]any{"type": "catan_gold", "take": []int{0, 0, 0, 0, 0, 2, 0, 0}}, 400)
				}
				clients[3].command(current(clients[3]), "action", request, 400)
				clients[(actor+1)%3].command(current(clients[(actor+1)%3]), "action", request, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("illegal mixed response mutated game or clock")
				}
				for p, c := range clients {
					v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					for q, raw := range v["players"].([]any) {
						_, visible := raw.(map[string]any)["resources"]
						if visible != (p == q) {
							t.Fatal("commodity hand leaked")
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
					t.Fatal("combined response changed on restart")
				}
				ts2 := httptest.NewServer(next.Handler())
				defer ts2.Close()
				for _, c := range clients {
					c.base = ts2.URL
				}
				completedAt := time.Now()
				switch mode {
				case "manual":
					clients[actor].command(current(clients[actor]), "action", request, 200)
				case "autoplay":
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					next.mu.Lock()
					next.rooms[id].BotAt = 0
					next.runBots(time.Now())
					next.mu.Unlock()
				case "timeout":
					next.mu.Lock()
					r = next.rooms[id]
					completedAt = time.UnixMilli(r.TurnDeadline).Add(time.Millisecond)
					next.expireSetups(completedAt)
					next.mu.Unlock()
				}
				r = next.rooms[id]
				if kind == "gold" {
					if r.Game.Phase != "catan_aqueduct" || r.Game.CatanPendingActor() != 2 || r.Game.Catan.GoldPending != nil {
						t.Fatal("gold-to-aqueduct did not continue", r.Game.Phase)
					}
					if sumServer(r.Game.Catan.Players[1].Resources) != 2 {
						t.Fatal("gold gain")
					}
					// Same action's second expansion response has its own fresh deadline.
					due := time.UnixMilli(r.TurnDeadline)
					if due.Before(completedAt.Add(119*time.Second)) || due.After(completedAt.Add(121*time.Second)) {
						t.Fatal("next responder did not get independent time")
					}
					completedAt = time.Now()
					clients[2].command(current(clients[2]), "action", map[string]any{"type": "catan_aqueduct", "color": 0}, 200)
				}
				r = next.rooms[id]
				if r.Game.Phase != "catan_turn" || r.Game.CatanPendingActor() != -1 || r.TurnDeadline < completedAt.Add(43*time.Second).UnixMilli() || r.TurnDeadline > completedAt.Add(46*time.Second).UnixMilli() {
					t.Fatal("original action clock not restored", r.Game.Phase, r.CatanTimeLeft)
				}
				if kind == "diplomacy" {
					ships := 0
					for _, e := range r.Game.Catan.Edges {
						if e.Owner == 0 {
							if !e.Ship {
								t.Fatal("ship turned into road")
							}
							ships++
						}
					}
					if ships != 1 {
						t.Fatal("ship missing after relocation", ships)
					}
				}
			})
		}
	}
}
func sumServer(values []int) int {
	n := 0
	for _, v := range values {
		n += v
	}
	return n
}

func TestCatanCitiesKnightsSeafarersHTTPChaseTargetSurvivesRestart(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	state, err := game.NewCatanCitiesKnightsSeafarers(3, game.CatanOptions{}, game.CatanSeafarersSetup{Scenario: "shores"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g := state.Catan
	k := g.CitiesKnights
	g.SetupStep = 6
	state.Turn = 0
	state.Phase = "catan_turn"
	k.Invasions = 1
	k.ActionSerial = 5
	g.Robber = k.RobberStart
	pirate, vertex := -1, -1
	for _, tile := range g.Tiles {
		if tile.Resource == game.CatanSea {
			pirate = tile.ID
			vertex = tile.Vertices[0]
			break
		}
	}
	if pirate < 0 {
		t.Fatal("no pirate tile")
	}
	g.Seafarers.Pirate = pirate
	k.Knights = []game.CatanKnight{{Owner: 0, Vertex: vertex, Strength: 1, Active: true, ActivatedAt: 1}}
	s.mu.Lock()
	r := s.rooms[id]
	r.Game = state
	r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
	deadline := r.TurnDeadline
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_knight_chase", "vertex": vertex, "choice": "pirate"}, 200)
	before, _ := json.Marshal(s.rooms[id])
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
	after, _ := json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("chase state lost")
	}
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_robber", "tile": 0}, 400)
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_pirate", "tile": -1}, 400)
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_pirate", "tile": -1}, 200)
	r = next.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.Catan.CitiesKnights.Chase != "" || r.Game.Catan.CitiesKnights.Knights[0].Active || r.TurnDeadline != deadline {
		t.Fatal("chase changed bandit, activation or ordinary clock")
	}
}
