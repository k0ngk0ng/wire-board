package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanRiversEventDeckHTTPDrawRecoveryAndTimeout(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				s, ts, clients, id := newRiversFullTable(t, n)
				for s.rooms[id].Game.Catan.SetupStep < s.rooms[id].Game.Catan.SetupLimit() {
					state := s.rooms[id].Game
					a, err := state.BotAction(state.Turn)
					if err != nil {
						t.Fatal(err)
					}
					clients[state.Turn].command(current(clients[state.Turn]), "action", a, 200)
				}
				referenceEventDrawHTTP(t, s, ts, clients, id, mode)
			})
		}
	}
}

func TestCatanTwoRiversEventDeckHTTPFleeResponse(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newTwoScenarioFullTable(t, "rivers")
			attachReferenceEventDeck(t, s.rooms[id].Game)
			pile := s.rooms[id].Game.Catan.EventDeck.Deck.DrawPile
			at := slices.Index(pile, 4) // Actual reference face: Robber Flees / production 4.
			pile[at], pile[len(pile)-1] = pile[len(pile)-1], pile[at]
			for s.rooms[id].Game.Catan.SetupStep < s.rooms[id].Game.Catan.SetupLimit() {
				state := s.rooms[id].Game
				a, err := state.BotAction(state.Turn)
				if err != nil {
					t.Fatal(err)
				}
				clients[state.Turn].command(current(clients[state.Turn]), "action", a, 200)
			}
			r := s.rooms[id]
			owner := r.Game.Turn
			r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			request := map[string]any{"type": "action", "action": game.Action{Type: "catan_roll"}, "version": r.Version, "nonce": "river-flee-draw"}
			clients[owner].post("/api/rooms/"+id, request, 200)
			r = s.rooms[id]
			if r.Game.Phase != "catan_card_event" || r.Game.CatanPendingActor() != owner || r.CatanTimeLeft < 44000 || r.CatanTimeLeft > 45000 {
				t.Fatal("swamp response or paused clock missing")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			r = s.rooms[id]
			before, _ := json.Marshal(r)
			clients[owner].post("/api/rooms/"+id, request, 200)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("replay redrew event")
			}
			for viewer, c := range clients {
				v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
				choices := v["legal"].(map[string]any)["fleeDeserts"].([]any)
				if (len(choices) == 2) != (viewer == owner) {
					t.Fatal("swamp choices exposed to wrong player")
				}
				deck := v["eventDeck"].(map[string]any)
				if deck["drawPile"] != nil || deck["deck"] != nil || deck["remaining"] != float64(35) {
					t.Fatal("deck projection invalid")
				}
			}
			target := r.Game.Catan.Rivers.Map.Swamps[1]
			action := game.Action{Type: "catan_robber_flees", Tile: target}
			clients[1-owner].command(current(clients[1-owner]), "action", action, 400)
			clients[owner].command(current(clients[owner]), "action", game.Action{Type: "catan_roll"}, 400)
			clients[owner].command(current(clients[owner]), "action", game.Action{Type: "catan_robber_flees", Tile: -1}, 400)
			switch mode {
			case "manual":
				clients[owner].command(current(clients[owner]), "action", action, 200)
			case "autoplay":
				expected, err := r.Game.BotAction(owner)
				if err != nil {
					t.Fatal(err)
				}
				target = expected.Tile
				setAutoPlay(clients[owner], current(clients[owner]), true, 200)
				s.mu.Lock()
				s.rooms[id].BotAt = 0
				s.runBots(time.Now())
				s.mu.Unlock()
			case "timeout":
				expected, err := r.Game.BotAction(owner)
				if err != nil {
					t.Fatal(err)
				}
				target = expected.Tile
				s.mu.Lock()
				s.expireSetups(time.UnixMilli(s.rooms[id].TurnDeadline))
				s.mu.Unlock()
				if !s.rooms[id].Seats[owner].TimeoutAutoPlay {
					t.Fatal("no persistent timeout takeover")
				}
			}
			r = s.rooms[id]
			if r.Game.Catan.Robber != target || r.Game.Phase != "catan_roll" || r.Game.Turn != owner || r.Game.Catan.RollID != 1 || !slices.Equal(r.Game.Catan.Two.Rolls, []int{4}) {
				t.Fatal("swamp did not resume second production")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if s.rooms[id].Seats[owner].AutoPlay {
				setAutoPlay(clients[owner], current(clients[owner]), false, 200)
			}
			// Exact resource accounting and non-theft are covered by the engine fixture;
			// this path verifies actual HTTP response/clock/persistence and next draw.
			clients[owner].command(current(clients[owner]), "action", game.Action{Type: "catan_roll"}, 200)
			if s.rooms[id].Game.Catan.RollID != 2 {
				t.Fatal("next production was lost")
			}
		})
	}
}
