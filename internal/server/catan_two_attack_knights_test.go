package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"slices"
	"testing"
	"time"
)

func TestCatanTwoAttackKnightsNeutralHTTP(t *testing.T) {
	s, ts, clients, id := newTradersKnightsHTTP(t, 2, true, "barbarian-attack")
	for step := 0; ; step++ {
		r := s.rooms[id]
		if q := r.Game.Catan.Two.Pending; q != nil && q.Kind == "knight" {
			break
		}
		if step > 2500 || r.Game.Finished {
			t.Fatal("missing neutral recruit")
		}
		p := twoHTTPActor(r.Game)
		a, e := r.Game.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		clients[p].command(current(clients[p]), "action", a, 200)
	}
	r := s.rooms[id]
	p := twoHTTPActor(r.Game)
	deadline := r.TurnDeadline
	if left := deadline - time.Now().UnixMilli(); left < 118000 || left > 120000 {
		t.Fatal("neutral response clock", left)
	}
	a, e := r.Game.BotAction(p)
	if e != nil {
		t.Fatal(e)
	}
	version := r.Version
	clients[2].command(current(clients[2]), "action", a, 400)
	clients[1-p].command(current(clients[1-p]), "action", a, 400)
	if s.rooms[id].Version != version || s.rooms[id].TurnDeadline != deadline {
		t.Fatal("invalid response mutated")
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	if s.rooms[id].TurnDeadline != deadline {
		t.Fatal("restore reset response clock")
	}
	r = s.rooms[id]
	r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
	s.mu.Lock()
	s.expireSetups(time.Now())
	s.mu.Unlock()
	r = s.rooms[id]
	if !r.Seats[p].AutoPlay || r.Version <= version || r.Game.Catan.Two.Pending != nil {
		t.Fatal("timeout did not complete neutral recruit")
	}
	if r.Game.Catan.Attack.TwoRules != game.CatanTwoAttackKnightsRules {
		t.Fatal("lost rule marker")
	}
	reclaimTimeoutHumans(t, s, clients, id)
}

func TestCatanTwoAttackKnightsWaitingToggle(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("双蛮骑房主")
	guest.register("双蛮骑玩家")
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "双人组合切换", "capacity": 2, "catanTwoScenario": "barbarian-attack"}, 201)
	id := raw["id"].(string)
	guest.command(current(host), "join", nil, 200)
	for _, enabled := range []bool{true, false, true} {
		for _, c := range []*testClient{host, guest} {
			c.command(current(c), "ready", nil, 200)
		}
		var setup *game.CatanCitiesKnightsSetup
		if enabled {
			setup = &game.CatanCitiesKnightsSetup{}
		}
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_cities_knights", "catanCitiesKnights": setup, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
		for _, seat := range s.rooms[id].Seats {
			if seat.Ready {
				t.Fatal("toggle retained readiness")
			}
		}
		if (s.rooms[id].CatanCitiesKnights != nil) != enabled {
			t.Fatal("toggle state")
		}
	}
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	host.command(current(host), "start", nil, 200)
	if s.rooms[id].Game.Catan.Attack.TwoRules != game.CatanTwoAttackKnightsRules {
		t.Fatal("wrong constructor")
	}
}

// Create a legal early action state, then isolate rare responses while preserving
// real board geometry and card inventories. Commands still use authenticated HTTP.
func attackHTTPActionPhase(t *testing.T, s *Server, clients []*testClient, id string) {
	t.Helper()
	for step := 0; s.rooms[id].Game.Phase != "catan_turn"; step++ {
		if step > 150 {
			t.Fatal("no action phase")
		}
		p := twoHTTPActor(s.rooms[id].Game)
		a, e := s.rooms[id].Game.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		clients[p].command(current(clients[p]), "action", a, 200)
	}
}
func TestCatanAttackKnightsTreasonHTTPResponses(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newTradersKnightsHTTP(t, n, false, "barbarian-attack")
			attackHTTPActionPhase(t, s, clients, id)
			r := s.rooms[id]
			p := r.Game.Turn
			owner := (p + 1) % n
			if n == 2 {
				owner = -2
			}
			k := r.Game.Catan.CitiesKnights
			found := false
			for track, deck := range k.ProgressDecks {
				for i, card := range deck {
					if card == 22 {
						k.ProgressDecks[track] = append(deck[:i], deck[i+1:]...)
						k.Players[p].Progress = append(k.Players[p].Progress, card)
						found = true
						break
					}
				}
			}
			if !found {
				t.Fatal("missing fixture card")
			}
			raw, _ := json.Marshal([]map[string]any{{"owner": owner, "edge": 0, "strength": 1}})
			if e := json.Unmarshal(raw, &r.Game.Catan.Attack.City.Knights); e != nil {
				t.Fatal(e)
			}
			r.TurnDeadline = time.Now().Add(39 * time.Second).UnixMilli()
			clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_progress", Card: 22, Target: owner}, 200)
			r = s.rooms[id]
			budget := r.CatanTimeLeft
			if budget < 37000 || budget > 39000 {
				t.Fatal("lost action budget", budget)
			}
			actor := twoHTTPActor(r.Game)
			if n == 2 && actor != p || n > 2 && actor != owner {
				t.Fatal("wrong responder")
			}
			a, e := r.Game.BotAction(actor)
			if e != nil {
				t.Fatal(e)
			}
			version := r.Version
			deadline := r.TurnDeadline
			clients[n].command(current(clients[n]), "action", a, 400)
			clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", a, 400)
			if s.rooms[id].Version != version || s.rooms[id].TurnDeadline != deadline {
				t.Fatal("invalid action mutation")
			}
			for viewer, c := range clients {
				city := current(c)["game"].(map[string]any)["catan"].(map[string]any)["attack"].(map[string]any)["city"].(map[string]any)
				if (city["choices"] != nil) != (viewer == actor) {
					t.Fatal("treason choices leaked")
				}
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if s.rooms[id].TurnDeadline != deadline || s.rooms[id].CatanTimeLeft != budget {
				t.Fatal("restore changed budget")
			}
			clients[actor].command(current(clients[actor]), "action", a, 200)
			r = s.rooms[id]
			if r.Game.Phase != "catan_attack_city_treason_place" || r.CatanTimeLeft != budget {
				t.Fatal("missing placement")
			}
			actor = twoHTTPActor(r.Game)
			a, e = r.Game.BotAction(actor)
			if e != nil {
				t.Fatal(e)
			}
			clients[actor].command(current(clients[actor]), "action", a, 200)
			r = s.rooms[id]
			left := r.TurnDeadline - time.Now().UnixMilli()
			if r.Game.Phase != "catan_turn" || left > budget || left < budget-1000 {
				t.Fatal("resume budget", left, budget)
			}
		})
	}
}

func TestCatanAttackKnightsDisplacementHTTPResponses(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newTradersKnightsHTTP(t, n, false, "barbarian-attack")
			attackHTTPActionPhase(t, s, clients, id)
			r := s.rooms[id]
			g := r.Game.Catan
			p := r.Game.Turn
			owner := (p + 1) % n
			if n == 2 {
				owner = -2
			}
			castles := map[int]bool{}
			for _, tile := range g.Tiles {
				if slices.Contains(g.Attack.Map.Castles, tile.ID) {
					for _, edge := range g.Edges {
						for _, adj := range edge.Tiles {
							if adj == tile.ID {
								castles[edge.ID] = true
							}
						}
					}
				}
			}
			// Search the real geometry for an adjacent non-castle pair. The engine's
			// projected choices verify that the target is legal before HTTP submission.
			from, to := -1, -1
			for _, a := range g.Edges {
				for _, b := range g.Edges {
					if a.ID != b.ID && !castles[a.ID] && !castles[b.ID] && (a.A == b.A || a.A == b.B || a.B == b.A || a.B == b.B) {
						from, to = a.ID, b.ID
						break
					}
				}
				if from >= 0 {
					break
				}
			}
			if from < 0 {
				t.Fatal("no adjacent pair")
			}
			raw, _ := json.Marshal([]map[string]any{{"owner": p, "edge": from, "strength": 3, "active": true}, {"owner": owner, "edge": to, "strength": 1}})
			if e := json.Unmarshal(raw, &g.Attack.City.Knights); e != nil {
				t.Fatal(e)
			}
			clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_end"}, 200)
			r = s.rooms[id]
			r.TurnDeadline = time.Now().Add(43 * time.Second).UnixMilli()
			prompt := r.Game.Catan.Attack.City.Sequence
			clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_attack_city_move", Prompt: prompt, Choice: "displace", Edge: from, Target: to}, 200)
			r = s.rooms[id]
			if r.Game.Phase != "catan_attack_city_retreat" {
				t.Fatal("missing retreat")
			}
			budget := r.CatanTimeLeft
			deadline := r.TurnDeadline
			actor := twoHTTPActor(r.Game)
			if n == 2 && actor != p || n > 2 && actor != owner || budget < 41000 || budget > 43000 {
				t.Fatal("retreat actor/budget", actor, budget)
			}
			a, e := r.Game.BotAction(actor)
			if e != nil {
				t.Fatal(e)
			}
			version := r.Version
			clients[n].command(current(clients[n]), "action", a, 400)
			clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", a, 400)
			if s.rooms[id].Version != version || s.rooms[id].TurnDeadline != deadline {
				t.Fatal("invalid retreat changed room")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if s.rooms[id].TurnDeadline != deadline || s.rooms[id].CatanTimeLeft != budget {
				t.Fatal("restore changed retreat")
			}
			clients[actor].command(current(clients[actor]), "action", a, 200)
			r = s.rooms[id]
			left := r.TurnDeadline - time.Now().UnixMilli()
			if r.Game.Phase != "catan_attack_city_move" || left > budget || left < budget-1000 {
				t.Fatal("retreat resume clock", left, budget)
			}
			version = r.Version
			clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_attack_city_move", Prompt: prompt, Choice: "undo"}, 400)
			if s.rooms[id].Version != version {
				t.Fatal("completed retreat was undone")
			}
			clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_attack_city_move", Prompt: prompt, Choice: "confirm"}, 200)
		})
	}
}

func TestCatanTwoAttackKnightsExchangeHTTP(t *testing.T) {
	s, _, clients, id := newTradersKnightsHTTP(t, 2, false, "barbarian-attack")
	attackHTTPActionPhase(t, s, clients, id)
	r := s.rooms[id]
	p := r.Game.Turn
	raw, _ := json.Marshal([]map[string]any{{"owner": p, "edge": 0, "strength": 2}, {"owner": -2, "edge": 1, "strength": 1}})
	if e := json.Unmarshal(raw, &r.Game.Catan.Attack.City.Knights); e != nil {
		t.Fatal(e)
	}
	for viewer, c := range clients {
		q := current(c)["game"].(map[string]any)["catan"].(map[string]any)["two"].(map[string]any)
		if viewer == p {
			edges := q["exchangeKnightEdges"].([]any)
			if len(edges) != 1 || edges[0] != float64(0) || q["canExchangeKnight"] != true {
				t.Fatal("road exchange projection", q)
			}
		} else if q["exchangeKnightEdges"] != nil {
			t.Fatal("exchange choices leaked")
		}
	}
	before := r.Game.Catan.Two.Tokens[p]
	deadline := r.TurnDeadline
	version := r.Version
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_two_knight", Edge: 1}, 400)
	if s.rooms[id].Version != version {
		t.Fatal("neutral exchange changed state")
	}
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_two_knight", Edge: 0}, 200)
	r = s.rooms[id]
	if r.Game.Catan.Two.Tokens[p] != before+2 || !r.Game.Catan.Two.KnightExchanged || r.TurnDeadline != deadline {
		t.Fatal("exchange or clock")
	}
}
