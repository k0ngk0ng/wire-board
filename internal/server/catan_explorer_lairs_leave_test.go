package server

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanExplorerLairsHTTPDepartureAndContinuation(t *testing.T) {
	s, ts, clients, id := newExplorerHTTP(t, 3, false)
	raw, err := os.ReadFile("testdata/catan_explorer_lairs_departure.json")
	if err != nil {
		t.Fatal(err)
	}
	var state game.State
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	actor := state.Turn
	other := (actor + 1) % 3
	s.mu.Lock()
	r := s.rooms[id]
	r.Game = &state
	r.Host = r.Seats[actor].ID
	r.startTurnClock(time.Now())
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	target := r.Seats[actor].ID
	kick(clients[other], current(clients[other]), target, 400)
	expireTurn(t, s, id)
	kick(clients[3], current(clients[3]), target, 400)
	kick(clients[actor], current(clients[actor]), target, 400)
	old := r.Game.Catan.Explorer
	kick(clients[other], current(clients[other]), target, 200)
	r = s.rooms[id]
	g := r.Game.Catan
	x := g.Explorer
	if !r.Seats[actor].Left || !g.Players[actor].Eliminated || r.Host == target || x.Pirate.Owner != -1 || x.Pirate.Tile != -1 || !x.Lairs.Retired[actor] {
		t.Fatal("departure ownership not cleaned")
	}
	for p := 0; p < 3; p++ {
		for unit := p * 11; unit < (p+1)*11; unit++ {
			if p == actor {
				if x.Cargo.Units[unit].Kind != "supply" {
					t.Fatal("departed crew remains")
				}
			} else if x.Cargo.Units[unit] != old.Cargo.Units[unit] {
				t.Fatal("other crew changed")
			}
		}
	}
	for _, site := range x.Lairs.Sites {
		if site.Ready != 0 || site.Resolved != 0 {
			t.Fatal("unresolved lair not reset")
		}
	}
	if clients[actor].state()["room"] != nil {
		t.Fatal("retired seat retained")
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	// Several actual HTTP turns, including rolls/response handling, must remain
	// playable after a departure. This is not a complete natural mission game.
	for step := 0; step < 100 && s.rooms[id].Game.Catan.TurnSerial < 6; step++ {
		r = s.rooms[id]
		st := r.Game
		p := explorerHTTPActor(st)
		if p == actor {
			t.Fatal("departed player selected")
		}
		a, err := st.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Phase == "catan_turn" {
			a = game.Action{Type: "catan_explorer_begin_move", Prompt: int(st.Catan.TurnSerial)}
		}
		if st.Phase == "catan_explorer_move" {
			a = game.Action{Type: "catan_end", Prompt: int(st.Catan.TurnSerial)}
		}
		clients[p].command(current(clients[p]), "action", a, 200)
	}
	r = s.rooms[id]
	if r.Game.Catan.TurnSerial < 6 || r.Game.Catan.Explorer.Economy.Gold[actor] != 0 {
		t.Fatal("remaining players stalled")
	}
	if r.Game.Catan.Players[actor].Resources[0]+r.Game.Catan.Players[actor].Resources[1]+r.Game.Catan.Players[actor].Resources[2]+r.Game.Catan.Players[actor].Resources[3]+r.Game.Catan.Players[actor].Resources[4] != 0 {
		t.Fatal("departed player received resources")
	}
	// The former seat cannot submit another player's response after restart.
	clients[actor].command(current(clients[other]), "action", game.Action{Type: "catan_roll", Prompt: int(r.Game.Catan.TurnSerial)}, 400)
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
}

func TestCatanExplorerLairsHTTPKickCannotInterruptMandatoryResponse(t *testing.T) {
	for _, fixture := range []string{"setup", "discard", "chase", "resolve", "battle"} {
		t.Run(fixture, func(t *testing.T) {
			s, ts, clients, id := newExplorerHTTP(t, 3, false)
			name := fixture
			if name == "battle" {
				name = "resolve"
			}
			raw, err := os.ReadFile("testdata/catan_explorer_lairs_" + name + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var state game.State
			if err = json.Unmarshal(raw, &state); err != nil {
				t.Fatal(err)
			}
			if fixture == "battle" {
				state.AutoCatanPending()
			}
			s.mu.Lock()
			r := s.rooms[id]
			r.Game = &state
			r.startTurnClock(time.Now())
			if err = s.save(r); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			actor := state.Turn
			other := (actor + 1) % 3
			expireTurn(t, s, id)
			before := *s.rooms[id].Game
			// command() first expires mandatory responses, then rejects the
			// now-stale kick version. This race must never remove the player.
			kick(clients[other], current(clients[other]), r.Seats[actor].ID, 409)
			for p, seat := range s.rooms[id].Seats {
				if seat.Left || s.rooms[id].Game.Catan.Players[p].Eliminated {
					t.Fatal("mandatory response race removed player")
				}
			}
			if reflect.DeepEqual(before, *s.rooms[id].Game) {
				t.Fatal("mandatory response did not auto advance")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
		})
	}
}
