package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanAttackShoresHTTP(t *testing.T)      { runAttackShoresHTTP(t, 4) }
func TestCatanAttackShoresThreeHTTP(t *testing.T) { runAttackShoresHTTP(t, 3) }
func TestCatanAttackShoresExtendedHTTP(t *testing.T) {
	for _, n := range []int{5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) { runAttackShoresHTTP(t, n) })
	}
}
func runAttackShoresHTTP(t *testing.T, n int) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n)
	for i := range clients {
		clients[i] = newClient(t, ts.URL)
		clients[i].register(fmt.Sprintf("蛮族航海%d", i))
	}
	h := clients[0]
	r := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "蛮族新海岸", "capacity": n, "catanScenario": "attack-shores", "catanEvents": game.CatanEventCatalogue}, 201)
	id := r["id"].(string)
	for _, c := range clients[1:2] {
		c.command(current(h), "join", nil, 200)
	}
	for _, c := range clients[:2] {
		c.command(current(c), "ready", nil, 200)
	}
	before, _ := json.Marshal(s.rooms[id])
	h.command(current(h), "start", nil, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("rejected start mutated room")
	}
	for _, c := range clients[2:] {
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
	for step := 0; step < 14000 && !s.rooms[id].Game.Finished; step++ {
		g := s.rooms[id].Game
		p := twoHTTPActor(g)
		if !timedOut && g.Phase == "catan_attack_end" {
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
			timedOut = true
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
	if !timedOut {
		t.Fatal("battle deadline not exercised")
	}
	if !s.rooms[id].Game.Finished {
		t.Fatal("unfinished")
	}
	_, profile := h.request("GET", "/api/players/"+s.rooms[id].Host, nil)
	record := profile["history"].([]any)[0].(map[string]any)
	if record["catanScenario"] != "attack-shores" || record["catanRules"] != game.CatanAttackSeafarersRules {
		t.Fatal("history")
	}
}

func TestCatanAttackShoresRoomBounds(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: n}
		err := r.setCatanScenario("attack-shores")
		if (err == nil) != (n >= 3 && n <= 6) {
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
