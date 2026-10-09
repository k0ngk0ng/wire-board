package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanCaravansSeaPublicHTTP(t *testing.T) { runCaravansSeaHTTP(t, false, "caravans-desert") }
func TestCatanCaravansSeaEventsHTTP(t *testing.T) { runCaravansSeaHTTP(t, true, "caravans-desert") }
func TestCatanCaravansTribeHTTP(t *testing.T)     { runCaravansSeaHTTP(t, true, "caravans-tribe") }
func TestCatanCaravansShoresHTTP(t *testing.T)    { runCaravansSeaHTTP(t, true, "caravans-shores") }
func runCaravansSeaHTTP(t *testing.T, events bool, scenario string) {
	counts := []int{2, 3, 4, 5, 6}
	if scenario == "caravans-shores" {
		counts = []int{4, 5, 6}
	}
	for _, n := range counts {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			clients := make([]*testClient, n)
			for p := range clients {
				clients[p] = newClient(t, ts.URL)
				clients[p].register(fmt.Sprintf("航海商队%d", p))
			}
			h := clients[0]
			recipe := map[string]any{"kind": "catan", "name": "商队沙漠", "capacity": n, "catanScenario": scenario}
			if events {
				recipe["catanEvents"] = game.CatanEventCatalogue
			}
			raw := h.post("/api/rooms", recipe, 201)
			id := raw["id"].(string)
			for p := 1; p < n; p++ {
				clients[p].command(current(h), "join", nil, 200)
			}
			for _, c := range clients {
				c.command(current(c), "ready", nil, 200)
			}
			h.command(current(h), "start", nil, 200)
			ordered := make([]*testClient, n)
			for _, c := range clients {
				ordered[int(current(c)["you"].(float64))] = c
			}
			clients = ordered
			bidTimedOut := false
			for step := 0; step < 18000 && !s.rooms[id].Game.Finished; step++ {
				g := s.rooms[id].Game
				if events && !bidTimedOut && g.Phase == "catan_caravan_bid" {
					actor := twoHTTPActor(g)
					deadline := s.rooms[id].TurnDeadline
					if left := deadline - time.Now().UnixMilli(); left < 118000 || left > 120000 {
						t.Fatal("bid clock", left)
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					if s.rooms[id].TurnDeadline != deadline {
						t.Fatal("restart reset bid clock")
					}
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(deadline + 1))
					s.mu.Unlock()
					if !s.rooms[id].Seats[actor].AutoPlay || !s.rooms[id].Seats[actor].TimeoutAutoPlay {
						t.Fatal("bid timeout did not take over")
					}
					setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					bidTimedOut = true
					continue
				}
				p := twoHTTPActor(g)
				a, err := g.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				clients[p].command(current(clients[p]), "action", a, 200)
				if step == 83 {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
				}
			}
			if events && !bidTimedOut {
				t.Fatal("no bid timeout exercised")
			}
			if !s.rooms[id].Game.Finished {
				t.Fatal("unfinished")
			}
			_, profile := h.request("GET", "/api/players/"+s.rooms[id].Host, nil)
			record := profile["history"].([]any)[0].(map[string]any)
			if record["catanScenario"] != scenario || record["catanRules"] != game.CatanCaravansSeafarersRules {
				t.Fatal("history")
			}
			if events {
				rules := record["catanExpansionRules"].(map[string]any)
				if rules["event_cards"] != game.CatanEventCatalogue {
					t.Fatal("missing events history")
				}
			}
			h.command(current(h), "rematch", nil, 200)
			cmd, field := "catan_scenario", "catanScenario"
			if n == 2 {
				cmd, field = "catan_two_scenario", "catanTwoScenario"
			}
			target := ""
			if n > 4 {
				target = "transport"
			}
			h.post("/api/rooms/"+id, map[string]any{"type": cmd, field: target, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			if s.rooms[id].CatanScenario != target {
				t.Fatal("base switch")
			}
		})
	}
}

func TestCatanCaravansSeaRejectsUnsupported(t *testing.T) {
	for _, n := range []int{1, 7} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: n}
		if r.setCatanScenario("caravans-desert") == nil {
			t.Fatal("unsupported count", n)
		}
	}
	for _, mutate := range []func(*Room){func(r *Room) { r.CatanFishing = true }, func(r *Room) { r.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{} }, func(r *Room) { r.CatanOptions.Helpers = true }} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: 3, CatanScenario: "caravans-desert"}
		mutate(r)
		if r.validateCatanScenario() == nil {
			t.Fatal("unsupported mix")
		}
	}
}
