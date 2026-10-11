package server

import (
	"fmt"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// A six-seat 3v3 room drafts over HTTP, plays to a leader death and records the
// mode in the player history.
func TestSanguoshaThreeV3HTTPAcceptance(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, 7)
	for i := range clients {
		clients[i] = newClient(t, ts.URL)
		clients[i].register(fmt.Sprintf("3v3玩家%d", i))
	}
	host := clients[0]
	room := host.post("/api/rooms", map[string]any{"name": "3v3验收", "kind": "sanguosha", "capacity": 6, "sanguoshaOptions": game.SGOptions{Mode: "3v3"}}, 201)
	id := room["id"].(string)
	for i := 1; i < 6; i++ {
		clients[i].command(current(host), "join", nil, 200)
	}
	for i := 0; i < 6; i++ {
		clients[i].command(current(host), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	clients[6].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)

	picks, assignments, steps := 0, 0, 0
	for ; steps < 4000 && !s.rooms[id].Game.Finished; steps++ {
		r := s.rooms[id]
		actor := r.Game.SanguoshaActor()
		q := r.Game.Sanguosha.Pending
		a, err := r.Game.BotAction(actor)
		if err != nil {
			t.Fatalf("step %d actor %d: %v", steps, actor, err)
		}
		if q != nil {
			switch q.Kind {
			case "sg_3v3_pick":
				picks++
			case "sg_3v3_assign":
				assignments++
			}
		}
		clients[actor].command(current(clients[actor]), "action", a, 200)
		if steps%97 == 0 {
			v := current(clients[6])["game"].(map[string]any)["sanguosha"].(map[string]any)
			if _, ok := v["threeV3"].(map[string]any); !ok {
				t.Fatal("spectator lost the 3v3 panel")
			}
			for seat, raw := range v["players"].([]any) {
				p := raw.(map[string]any)
				if _, hand := p["hand"]; hand {
					t.Fatal("spectator saw a hand", seat)
				}
				if p["camp"] == nil {
					t.Fatal("camps must stay public", seat, p)
				}
			}
		}
	}
	r := s.rooms[id]
	if !r.Game.Finished || len(r.Game.Winners) != 3 || picks != 16 || assignments != 6 {
		t.Fatalf("unfinished or wrong draft: finished=%t winners=%v picks=%d assignments=%d steps=%d",
			r.Game.Finished, r.Game.Winners, picks, assignments, steps)
	}
	view := current(clients[6])["game"].(map[string]any)["sanguosha"].(map[string]any)
	three := view["threeV3"].(map[string]any)
	leaders := three["leaders"].([]any)
	players := view["players"].([]any)
	camps := map[int]int{}
	for seat, raw := range players {
		camps[seat] = int(raw.(map[string]any)["camp"].(float64))
	}
	winnerCamp := camps[r.Game.Winners[0]]
	for _, w := range r.Game.Winners {
		if camps[w] != winnerCamp {
			t.Fatal("winners are not one camp", r.Game.Winners, camps)
		}
	}
	loserLeader := int(leaders[1-winnerCamp].(float64))
	if !r.Game.Sanguosha.Players[loserLeader].Dead {
		t.Fatal("losing leader survived", loserLeader)
	}
	_, profile := clients[6].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
	record := profile["history"].([]any)[0].(map[string]any)
	if record["sanguoshaOptions"].(map[string]any)["mode"] != "3v3" || record["status"] != "finished" {
		t.Fatal("history lost the 3v3 mode", record["sanguoshaOptions"], record["status"])
	}
}
