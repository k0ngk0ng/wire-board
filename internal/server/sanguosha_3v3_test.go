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

func sanguoshaThreeMissingRoundSeat(acted, flipped []bool, active int, after *game.State) int {
	for seat, marked := range acted {
		player := after.Sanguosha.Players[seat]
		if !marked && !player.Dead && seat != active && !(flipped[seat] && !player.Flipped) {
			return seat
		}
	}
	return -1
}

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
	opening := s.rooms[id].Game.Sanguosha
	if opening.Pending == nil || opening.Pending.Kind != "sg_3v3_pile" {
		t.Fatalf("first 3v3 prompt %+v", opening.Pending)
	}
	if three := opening.ThreeV3; three.Stage != "piles" || len(three.Piles) != 4 || len(three.Pool) != 0 {
		t.Fatalf("opening draft %+v", three)
	}
	warmLeader := opening.ThreeV3.Leaders[1]
	if opening.Pending.Player != warmLeader {
		t.Fatalf("the warm leader must take the first pile, prompt %d leader %d", opening.Pending.Player, warmLeader)
	}
	if len(opening.Deck) != 106 {
		t.Fatalf("the official 3v3 deck holds %d cards, want 106", len(opening.Deck))
	}
	for viewer := 0; viewer < 6; viewer++ {
		draft := current(clients[viewer])["game"].(map[string]any)["sanguosha"].(map[string]any)
		three := draft["threeV3"].(map[string]any)
		pool, ok := three["pool"].([]any)
		if !ok {
			t.Fatalf("viewer %d received a non-array pool %#v", viewer, three["pool"])
		}
		if len(pool) != 0 {
			t.Fatalf("viewer %d saw %d laid-out generals before both piles were taken", viewer, len(pool))
		}
		sizes, ok := three["pileSizes"].([]any)
		if !ok || len(sizes) != 4 {
			t.Fatalf("viewer %d received pileSizes %#v, want four piles", viewer, three["pileSizes"])
		}
		for _, size := range sizes {
			if size != float64(8) {
				t.Fatalf("viewer %d saw pile size %#v, want 8", viewer, size)
			}
		}
		if turn, ok := three["pileTurn"].(float64); !ok || (turn != 0 && turn != 1) {
			t.Fatalf("viewer %d received pileTurn %#v", viewer, three["pileTurn"])
		}
		if viewer == warmLeader {
			continue
		}
		if _, seen := three["piles"]; seen {
			t.Fatalf("viewer %d saw the face-down piles", viewer)
		}
		if pending, ok := draft["pending"].(map[string]any); ok {
			if _, seen := pending["choices"]; seen && pending["canRespond"] == true {
				t.Fatalf("viewer %d received the pile choices", viewer)
			}
		}
	}
	var turns []map[string]int
	t.Cleanup(func() {
		path := filepath.Join("..", "..", ".local", "sanguosha", "verification", "3v3-opening-rounds.json")
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
	campOf := func(seat int) int {
		if s.rooms[id].Game.Sanguosha.ThreeV3.Camps[seat] == "cold" {
			return 0
		}
		return 1
	}
	lastSequence, requestedFirst, restarted, rounds := 0, -1, false, 0
	for step := 0; step < 400 && rounds < 2; step++ {
		state = s.rooms[id].Game
		g := state.Sanguosha
		if g.TurnSequence != lastSequence {
			delta := g.TurnSequence - lastSequence
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
				partner := delta == 2 && campOf(state.Turn) == campOf(requestedFirst) &&
					state.Turn != g.ThreeV3.Leaders[campOf(state.Turn)] && g.ThreeV3.Acted[requestedFirst]
				if !partner {
					t.Fatalf("vanguard order ignored: requested %d got %d", requestedFirst, state.Turn)
				}
			}
			requestedFirst = -1
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
				a = game.Action{Type: "sg_respond", Prompt: q.ID, Choice: "leader"}
				for i := 0; i < 6; i++ {
					if g.ThreeV3.Camps[i] == g.ThreeV3.Camps[actor] && i != actor && !g.ThreeV3.Acted[i] {
						a.Choice, a.Target = "vanguards", i
						requestedFirst = i
					}
				}
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
		beforeActed := append([]bool{}, g.ThreeV3.Acted...)
		beforeTurn := state.Turn
		beforeFlipped := make([]bool, len(g.Players))
		for seat, player := range g.Players {
			beforeFlipped[seat] = player.Flipped
		}
		beforeRound := state.Round
		clients[actor].command(current(clients[actor]), "action", a, 200)
		after := s.rooms[id].Game
		if after.Round > beforeRound {
			if seat := sanguoshaThreeMissingRoundSeat(beforeActed, beforeFlipped, beforeTurn, after); seat >= 0 {
				t.Fatalf("round %d advanced while seat %d had not acted: %v", beforeRound, seat, beforeActed)
			}
			rounds++
			if campOf(after.Turn) == after.Sanguosha.ThreeV3.Side {
				t.Fatalf("round %d started with the camp that just acted (seat %d)", after.Round, after.Turn)
			}
		}
	}
	if rounds != 2 || !restarted {
		t.Fatalf("two complete rounds not played: %v restart=%t", turns, restarted)
	}
	t.Logf("HTTP draft, opening cards, two rounds, vanguard order and restart passed: %v", turns)
}

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
