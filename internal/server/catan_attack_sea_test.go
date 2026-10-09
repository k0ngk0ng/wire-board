package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanAttackShoresTwoHTTP(t *testing.T)   { runAttackShoresHTTP(t, 2) }
func TestCatanAttackShoresHTTP(t *testing.T)      { runAttackShoresHTTP(t, 4) }
func TestCatanAttackShoresThreeHTTP(t *testing.T) { runAttackShoresHTTP(t, 3) }
func TestCatanAttackShoresExtendedHTTP(t *testing.T) {
	for _, n := range []int{5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) { runAttackShoresHTTP(t, n) })
	}
}
func runAttackShoresHTTP(t *testing.T, n int) { runAttackSeaHTTP(t, n, "attack-shores") }
func TestCatanAttackDesertHTTP(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) { runAttackSeaHTTP(t, n, "attack-desert") })
	}
}
func TestCatanAttackTribeHTTP(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) { runAttackSeaHTTP(t, n, "attack-tribe") })
	}
}
func TestCatanAttackWondersHTTP(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) { runAttackSeaHTTP(t, n, "attack-wonders") })
	}
}
func TestCatanAttackPiratesHTTP(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) { runAttackSeaHTTP(t, n, "attack-pirates") })
	}
}
func runAttackSeaHTTP(t *testing.T, n int, scenario string) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n)
	for i := range clients {
		clients[i] = newClient(t, ts.URL)
		clients[i].register(fmt.Sprintf("蛮族航海%d", i))
	}
	h := clients[0]
	r := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "蛮族新海岸", "capacity": n, "catanScenario": scenario, "catanEvents": game.CatanEventCatalogue}, 201)
	id := r["id"].(string)
	h.command(current(h), "ready", nil, 200)
	before, _ := json.Marshal(s.rooms[id])
	h.command(current(h), "start", nil, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("rejected start mutated room")
	}
	for _, c := range clients[1:] {
		c.command(current(h), "join", nil, 200)
		c.command(current(c), "ready", nil, 200)
	}
	h.command(current(h), "start", nil, 200)
	ordered := make([]*testClient, n)
	for _, c := range clients {
		ordered[int(current(c)["you"].(float64))] = c
	}
	clients = ordered
	timedOut := false
	landingTimedOut := false
	for step := 0; step < 14000 && !s.rooms[id].Game.Finished; step++ {
		g := s.rooms[id].Game
		p := twoHTTPActor(g)
		if (!timedOut && g.Phase == "catan_attack_end") || ((scenario == "attack-wonders" || scenario == "attack-pirates") && !landingTimedOut && g.Phase == "catan_attack_landing") {
			deadline := s.rooms[id].TurnDeadline
			if left := deadline - time.Now().UnixMilli(); left < 118000 || left > 120000 {
				t.Fatal("battle clock", left)
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if s.rooms[id].TurnDeadline != deadline {
				t.Fatal("restart changed clock")
			}
			s.mu.Lock()
			s.expireSetups(time.UnixMilli(deadline + 1))
			s.mu.Unlock()
			if !s.rooms[id].Seats[p].AutoPlay || !s.rooms[id].Seats[p].TimeoutAutoPlay {
				t.Fatal("timeout did not take over")
			}
			setAutoPlay(clients[p], current(clients[p]), false, 200)
			if g.Phase == "catan_attack_landing" {
				landingTimedOut = true
			} else {
				timedOut = true
			}
			continue
		}
		a, e := g.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		clients[p].command(current(clients[p]), "action", a, 200)
		if step == 83 {
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
		}
	}
	if (scenario == "attack-wonders" || scenario == "attack-pirates") && !landingTimedOut {
		t.Fatal("landing deadline not exercised")
	}
	if !timedOut {
		t.Fatal("battle deadline not exercised")
	}
	if !s.rooms[id].Game.Finished {
		t.Fatal("unfinished")
	}
	_, profile := h.request("GET", "/api/players/"+s.rooms[id].Host, nil)
	record := profile["history"].([]any)[0].(map[string]any)
	if record["catanScenario"] != scenario || record["catanRules"] != game.CatanAttackSeafarersRules {
		t.Fatal("history")
	}
	if scenario == "attack-tribe" {
		rules := record["catanExpansionRules"].(map[string]any)
		if rules["attack_tribe_rewards"] != game.CatanAttackTribeRewardRules || record["catanLayout"] != "fixed" {
			t.Fatal("tribe rule/layout archive")
		}
		if n == 2 && rules["two_attack_seafarers"] != game.CatanTwoAttackSeaRules {
			t.Fatal("two-player archive")
		}
	}
}

func TestCatanAttackShoresRoomBounds(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: n}
		err := r.setCatanScenario("attack-shores")
		if (err == nil) != (n >= 2 && n <= 6) {
			t.Fatal(n, err)
		}
	}
}

func TestCatanAttackShoresExtendedSwitch(t *testing.T) {
	r := &Room{Kind: "catan", Status: "waiting", Capacity: 6}
	if e := r.setCatanScenario("attack-shores"); e != nil {
		t.Fatal(e)
	}
	if e := r.setCatanScenario(""); e == nil {
		t.Fatal("six seats accepted base")
	}
	if r.CatanScenario != "attack-shores" {
		t.Fatal("rejected switch mutated room")
	}
	if e := r.setCatanScenario("transport"); e != nil {
		t.Fatal(e)
	}
	if e := r.setCatanScenario("attack-shores"); e != nil {
		t.Fatal(e)
	}
}

func TestCatanAttackDesertRoomBounds(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: n}
		e := r.setCatanScenario("attack-desert")
		if (e == nil) != (n >= 2 && n <= 6) {
			t.Fatal(n, e)
		}
	}
}

func TestCatanAttackTribeRoomSelection(t *testing.T) {
	for _, capacity := range []int{1, 2, 3, 4, 5, 6, 7} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: capacity}
		err := r.setCatanScenario("attack-tribe")
		if (err == nil) != (capacity >= 2 && capacity <= 6) {
			t.Fatal(capacity, err)
		}
		if err != nil && r.CatanScenario != "" {
			t.Fatal("rejection changed room")
		}
	}
	for _, change := range []func(*Room){func(r *Room) { r.CatanOptions.Helpers = true }, func(r *Room) { r.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{} }, func(r *Room) { r.CatanFishing = true }, func(r *Room) { r.CatanFriendlyRobber = &game.CatanFriendlyRobberSetup{Enabled: true} }} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: 4, CatanScenario: "attack-tribe"}
		change(r)
		if r.validateCatanScenario() == nil {
			t.Fatal("unsupported nesting admitted")
		}
	}
	r := &Room{Kind: "catan", Status: "waiting", Capacity: 6}
	for _, scenario := range []string{"attack-tribe", "attack-desert", "attack-shores", "attack-tribe", "transport"} {
		if err := r.setCatanScenario(scenario); err != nil {
			t.Fatal(scenario, err)
		}
	}
}
