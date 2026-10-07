package server

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func explorerRecruitHTTPFixture(t *testing.T, kind string) (*game.State, game.Action) {
	t.Helper()
	raw, err := os.ReadFile("testdata/catan_explorer_spice_actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var states map[string]*game.State
	if err = json.Unmarshal(raw, &states); err != nil {
		t.Fatal(err)
	}
	state := states["transfer"]
	actor := state.Turn
	// Existing controlled cargo fixture has a full harbor and docked settler.
	// Finish its movement phase and cycle normally to the actor's next action.
	for steps := 0; steps < 100; steps++ {
		if state.Turn == actor && state.Phase == "catan_turn" {
			break
		}
		a := game.Action{Prompt: int(state.Catan.TurnSerial)}
		switch state.Phase {
		case "catan_roll":
			a.Type = "catan_roll"
		case "catan_turn":
			a.Type = "catan_explorer_begin_move"
		case "catan_explorer_move":
			a.Type = "catan_end"
		default:
			p := explorerHTTPActor(state)
			a, err = state.BotAction(p)
			if err != nil {
				t.Fatal(err)
			}
			if err = state.Apply(p, a); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err = state.Apply(state.Turn, a); err != nil {
			t.Fatal(err)
		}
	}
	if state.Turn != actor || state.Phase != "catan_turn" {
		t.Fatal("failed to reach action phase")
	}
	g, x := state.Catan, state.Catan.Explorer
	for r := range g.Bank {
		need := 3 - g.Players[actor].Resources[r]
		if need > g.Bank[r] {
			t.Fatal("fixture resource shortfall")
		}
		g.Players[actor].Resources[r] += need
		g.Bank[r] -= need
	}
	harbor, sack := -1, -1
	for i, c := range x.Cargo.Spice {
		if c.Owner == actor && c.At.Kind == "harbor" {
			harbor, sack = c.At.Index, i
		}
	}
	if harbor < 0 || sack < 0 {
		t.Fatal("missing full fixture harbor")
	}
	a := game.Action{Type: "catan_explorer_unit", Prompt: int(g.TurnSerial), Card: actor*11 + 10, Choice: "harbor", Target: harbor}
	if kind == "fish" {
		for i, c := range x.Cargo.Spice {
			if c.At.Kind == "harbor" && c.At.Index == harbor {
				x.Cargo.Spice[i].At.Kind = "supply"
				x.Cargo.Spice[i].At.Index = -1
			}
		}
		if x.Cargo.Fish[0].Kind != "supply" {
			t.Fatal("fixture fish unavailable")
		}
		x.Cargo.Fish[0].Kind = "harbor"
		x.Cargo.Fish[0].Index = harbor
		a.Targets = []int{0}
	} else {
		a.SpiceUnload = []int{sack}
	}
	return state, a
}

func TestCatanExplorerRecruitFreightHTTP(t *testing.T) {
	for _, kind := range []string{"fish", "spice"} {
		t.Run(kind, func(t *testing.T) {
			s, ts, clients, id := newExplorerSpiceHTTP(t)
			state, a := explorerRecruitHTTPFixture(t, kind)
			actor := state.Turn
			s.mu.Lock()
			room := s.rooms[id]
			room.Game = state
			room.startTurnClock(time.Now())
			room.TurnDeadline = time.Now().Add(43 * time.Second).UnixMilli()
			err := s.save(room)
			s.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			deadline := room.TurnDeadline
			a = fishHTTPPreview(t, current(clients[actor]), a)
			before, _ := json.Marshal(room)
			clients[(actor+1)%3].command(current(clients[(actor+1)%3]), "action", a, 400)
			clients[3].command(current(clients[3]), "action", a, 400)
			bad := a
			bad.Prompt--
			clients[actor].command(current(clients[actor]), "action", bad, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("rejection changed room")
			}
			request := map[string]any{"type": "action", "action": a, "version": current(clients[actor])["version"], "nonce": randomID(12)}
			clients[actor].post("/api/rooms/"+id, request, 200)
			committed, _ := json.Marshal(s.rooms[id])
			clients[actor].post("/api/rooms/"+id, request, 200)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			clients[actor].post("/api/rooms/"+id, request, 200)
			clients[actor].post("/api/rooms/"+id, map[string]any{"type": "action", "action": a, "version": request["version"], "nonce": randomID(12)}, 409)
			clients[actor].command(current(clients[actor]), "action", a, 400)
			after, _ = json.Marshal(s.rooms[id])
			if string(committed) != string(after) {
				t.Fatal("restart/replay changed committed return")
			}
			room = s.rooms[id]
			g, x := room.Game.Catan, room.Game.Catan.Explorer
			if room.TurnDeadline != deadline || g.Players[actor].Resources[2] != 2 || g.Players[actor].Resources[4] != 2 {
				t.Fatal("clock or payment changed incorrectly")
			}
			if x.Cargo.Units[a.Card].Kind != "harbor" || x.Cargo.Units[a.Card].Index != a.Target {
				t.Fatal("crew missing")
			}
			if kind == "fish" && x.Cargo.Fish[0].Kind != "supply" || kind == "spice" && x.Cargo.Spice[a.SpiceUnload[0]].At.Kind != "supply" {
				t.Fatal("freight not returned")
			}
			for _, client := range clients {
				view := current(client)["game"].(map[string]any)["catan"].(map[string]any)["explorer"].(map[string]any)
				motion := view["motion"].(map[string]any)
				if motion["kind"] != a.Type || len(motion["cargo"].([]any)) != 1 || len(motion[kind].([]any)) != 1 {
					t.Fatal("viewer lost return/recruit motion")
				}
			}
			assertExplorerSpiceHTTPPrivacy(t, clients, room.Game)
		})
	}
}
