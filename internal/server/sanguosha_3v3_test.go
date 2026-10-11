package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestSanguoshaThreeV3RoomCapacityHTTP(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("席位验收")
	for _, capacity := range []int{4, 5, 7, 8} {
		host.post("/api/rooms", map[string]any{"name": "错误席位", "kind": "sanguosha", "capacity": capacity, "sanguoshaOptions": game.SGOptions{Mode: "3v3"}}, 400)
	}
	host.post("/api/rooms", map[string]any{"name": "切换模式", "kind": "sanguosha", "capacity": 4}, 201)
	r := current(host)
	host.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "sanguosha_options", "version": r["version"], "nonce": randomID(12), "sanguoshaOptions": game.SGOptions{Mode: "3v3"}}, 200)
	if got := current(host)["capacity"]; got != float64(6) {
		t.Fatal("switching to 3v3 must provide six seats", got)
	}
}

// Failure scenarios: premature lineup disclosure; missing/repeated opening
// cards; a vanguard opening the game; duplicate or skipped turns in a round;
// ignored vanguard order; a handoff resetting marks; and a restart losing the
// pending action right. All commands below use the real authenticated HTTP API.
func TestSanguoshaThreeV3OpeningAndRoundsHTTP(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, 7)
	for i := range clients {
		clients[i] = newClient(t, ts.URL)
		clients[i].register(fmt.Sprintf("轮次验收%d", i))
	}
	host := clients[0]
	room := host.post("/api/rooms", map[string]any{"name": "行动轮次", "kind": "sanguosha", "capacity": 6, "sanguoshaOptions": game.SGOptions{Mode: "3v3"}}, 201)
	id := room["id"].(string)
	for i := 1; i < 6; i++ {
		clients[i].command(current(host), "join", nil, 200)
	}
	for i := 0; i < 6; i++ {
		clients[i].command(current(host), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	clients[6].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	var turns []map[string]int
	t.Cleanup(func() {
		path := filepath.Join("..", "..", ".local", "3v3-opening-rounds.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Error(err)
			return
		}
		raw, err := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "turns": turns, "state": s.rooms[id].Game}, "", "  ")
		if err == nil {
			err = os.WriteFile(path, raw, 0o600)
		}
		if err != nil {
			t.Error(err)
		}
	})
	// Draft using public choices. No fixture edits or direct engine commands.
	for step := 0; step < 30 && s.rooms[id].Game.Sanguosha.ThreeV3.Stage != "firstSide"; step++ {
		if left := time.Until(time.UnixMilli(s.rooms[id].TurnDeadline)); left < 118*time.Second || left > 120*time.Second {
			t.Errorf("draft choices need 120 seconds, got %s", left)
		}
		state := s.rooms[id].Game
		actor := state.SanguoshaActor()
		a, err := state.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		clients[actor].command(current(clients[actor]), "action", a, 200)
		state = s.rooms[id].Game
		three := state.Sanguosha.ThreeV3
		if three.Stage == "assign" {
			for viewer := range clients {
				v := current(clients[viewer])["game"].(map[string]any)["sanguosha"].(map[string]any)
				assigned := v["threeV3"].(map[string]any)["assigned"].([]any)
				for camp, value := range assigned {
					if viewer < 6 && three.Camps[viewer] == []string{"cold", "warm"}[camp] {
						continue
					}
					if list, ok := value.([]any); ok && len(list) != 0 {
						t.Errorf("viewer %d saw camp %d's unrevealed lineup: %v", viewer, camp, list)
					}
				}
			}
		}
	}
	state := s.rooms[id].Game
	if state.Sanguosha.ThreeV3.Stage != "firstSide" {
		t.Fatal("draft did not reach first-side choice")
	}
	leader := state.Sanguosha.ThreeV3.Leaders[0]
	clients[leader].command(current(clients[leader]), "action", game.Action{Type: "sg_respond", Prompt: state.Sanguosha.Pending.ID, Choice: "cold"}, 200)
	state = s.rooms[id].Game
	for i, p := range state.Sanguosha.Players {
		if len(p.Hand) < 4 {
			t.Errorf("seat %d missing its four initial cards: %v", i, p.Hand)
		}
	}
	if state.Turn != leader || state.Sanguosha.TurnSequence != 1 {
		t.Errorf("first turn must begin immediately with cold leader %d, got turn=%d sequence=%d", leader, state.Turn, state.Sanguosha.TurnSequence)
	}
	seen := map[int]map[int]bool{}
	lastSequence, requestedFirst, restarted := 0, -1, false
	for step := 0; step < 300 && len(turns) < 13; step++ {
		state = s.rooms[id].Game
		g := state.Sanguosha
		if g.TurnSequence != lastSequence {
			lastSequence = g.TurnSequence
			if seen[state.Round] == nil {
				seen[state.Round] = map[int]bool{}
			}
			if seen[state.Round][state.Turn] {
				t.Fatalf("seat %d acted twice in round %d: %v", state.Turn, state.Round, turns)
			}
			seen[state.Round][state.Turn] = true
			turns = append(turns, map[string]int{"round": state.Round, "seat": state.Turn})
			if requestedFirst >= 0 && state.Turn != requestedFirst {
				t.Fatalf("vanguard order ignored: requested %d got %d", requestedFirst, state.Turn)
			}
			requestedFirst = -1
		}
		if len(turns) == 13 {
			break
		}
		actor := state.SanguoshaActor()
		var a game.Action
		if q := g.Pending; q != nil {
			if q.Kind == "sg_3v3_side" {
				if left := time.Until(time.UnixMilli(s.rooms[id].TurnDeadline)); left < 118*time.Second || left > 120*time.Second {
					t.Errorf("action-right choice needs 120 seconds, got %s", left)
				}
				if !restarted && len(turns) >= 3 {
					saved, _ := json.Marshal(s.rooms[id])
					ts.Close()
					s.Close()
					next, err := New(s.cfg, s.files)
					if err != nil {
						t.Fatal(err)
					}
					s = next
					stopBotTicker(s)
					ts = httptest.NewServer(s.Handler())
					t.Cleanup(func() { ts.Close(); s.Close() })
					for _, c := range clients {
						c.base = ts.URL
					}
					restored, _ := json.Marshal(s.rooms[id])
					if string(saved) != string(restored) {
						t.Fatal("restart changed the pending action right")
					}
					restarted = true
				}
				// Always request the highest-numbered eligible vanguard first.
				a = game.Action{Type: "sg_respond", Prompt: q.ID, Choice: "leader"}
				for i := 0; i < 6; i++ {
					if g.ThreeV3.Camps[i] == g.ThreeV3.Camps[actor] && i != actor && !g.ThreeV3.Acted[i] {
						a.Choice, a.Target = "vanguards", i
						requestedFirst = i
					}
				}
				// An exhausted leader must be rejected without changing state.
				if g.ThreeV3.Acted[actor] {
					before := current(clients[actor])
					clients[actor].command(before, "action", game.Action{Type: "sg_respond", Prompt: q.ID, Choice: "leader"}, 400)
					if !reflect.DeepEqual(before["game"], current(clients[actor])["game"]) {
						t.Fatal("rejected repeated turn changed state")
					}
				}
			} else {
				var err error
				a, err = state.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
			}
		} else {
			a = game.Action{Type: "sg_end"}
		}
		clients[actor].command(current(clients[actor]), "action", a, 200)
	}
	if len(turns) != 13 || len(seen[1]) != 6 || len(seen[2]) != 6 || !restarted {
		t.Fatalf("two complete rounds not played: %v restart=%t", turns, restarted)
	}
	if turns[12]["round"] != 3 {
		t.Fatal("round counter advanced before all six acted", turns)
	}
	three := s.rooms[id].Game.Sanguosha.ThreeV3
	if three.Camps[turns[12]["seat"]] == three.Camps[turns[11]["seat"]] {
		t.Fatal("new round did not pass the action right to the other camp")
	}
	t.Logf("HTTP draft, opening cards, two rounds, vanguard order and restart passed: %v", turns)
}

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
