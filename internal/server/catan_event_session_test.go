package server

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Attach only the private legacy-reference catalogue via an explicit persisted
// fixture. No room option exposes this unverified catalogue to real players.
func attachReferenceEventDeck(t *testing.T, s *game.State) {
	t.Helper()
	pile := []int{}
	for id := 1; id < 36; id++ {
		if len(pile) == 5 {
			pile = append(pile, 36)
		}
		pile = append(pile, id)
	}
	pile = append(pile, 0) // Plentiful Year, production 2; every player must choose.
	encoded, err := json.Marshal(map[string]any{"eventDeck": map[string]any{
		"catalogue": "legacy-reference-v1", "deck": map[string]any{"drawPile": pile, "discard": []int{}, "cycle": 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(encoded, s.Catan); err != nil {
		t.Fatal(err)
	}
}

func TestCatanEventSessionHTTPDrawRecoveryReplayAndTimeout(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newCatanTable(t)
			clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			r := s.rooms[id]
			attachReferenceEventDeck(t, r.Game)
			r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			expected := make([][]int, 3)
			for p := range expected {
				expected[p] = slices.Clone(r.Game.Catan.Players[p].Resources)
			}
			for _, tile := range r.Game.Catan.Tiles {
				if tile.Number == 2 && tile.ID != r.Game.Catan.Robber {
					for _, v := range tile.Vertices {
						vertex := r.Game.Catan.Vertices[v]
						if vertex.Level > 0 && vertex.Owner >= 0 {
							expected[vertex.Owner][tile.Resource] += vertex.Level
						}
					}
				}
			}
			draw := map[string]any{"type": "action", "version": r.Version, "nonce": "reference-draw", "action": game.Action{Type: "catan_roll"}}
			owner := r.Game.Turn
			clients[(owner+1)%3].command(current(clients[owner]), "action", game.Action{Type: "catan_roll"}, 400)
			clients[owner].post("/api/rooms/"+id, draw, 200)
			for step := 0; step < 3; step++ {
				r = s.rooms[id]
				actor := r.Game.CatanPendingActor()
				if actor < 0 || r.Game.Catan.RollID != 1 {
					t.Fatal("missing response or extra draw")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				r = s.rooms[id]
				before, _ := json.Marshal(r)
				clients[owner].post("/api/rooms/"+id, draw, 200)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("replay consumed a card or reset the clock")
				}
				for viewer, c := range clients {
					v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					deck := v["eventDeck"].(map[string]any)
					if deck["drawPile"] != nil || deck["deck"] != nil || deck["referenceOnly"] != true || deck["remaining"] != float64(35) {
						t.Fatal("hidden pile exposed or invalid count", deck)
					}
					for p, raw := range v["players"].([]any) {
						if (raw.(map[string]any)["resources"] != nil) != (viewer == p) {
							t.Fatal("hand leaked")
						}
					}
				}
				action, err := r.Game.BotAction(actor)
				if err != nil || action.Type != "catan_event_resource" {
					t.Fatal("missing event choice", action, err)
				}
				for c, n := range action.Take {
					expected[actor][c] += n
				}
				switch mode {
				case "manual":
					clients[actor].command(current(clients[actor]), "action", action, 200)
				case "autoplay":
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					setAutoPlay(clients[actor], current(clients[actor]), false, 200)
				case "timeout":
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(s.rooms[id].TurnDeadline))
					s.mu.Unlock()
					if !s.rooms[id].Seats[actor].AutoPlay || !s.rooms[id].Seats[actor].TimeoutAutoPlay {
						t.Fatal("event timeout did not persist takeover")
					}
				}
			}
			r = s.rooms[id]
			if r.Game.Phase != "catan_turn" || r.Game.Catan.RollID != 1 || !r.Game.Catan.RevealedEvent.ProductionStarted {
				t.Fatal("production failed")
			}
			for p, want := range expected {
				if !slices.Equal(r.Game.Catan.Players[p].Resources, want) {
					t.Fatal("production or choice repeated", p, r.Game.Catan.Players[p].Resources, want)
				}
			}
			snapshot, _ := json.Marshal(r)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			restored, _ := json.Marshal(s.rooms[id])
			if !reflect.DeepEqual(snapshot, restored) {
				t.Fatal("finished event changed on restart")
			}
			for p, seat := range s.rooms[id].Seats {
				if seat.AutoPlay {
					setAutoPlay(clients[p], current(clients[p]), false, 200)
				}
			}
			clients[owner].command(current(clients[owner]), "action", game.Action{Type: "catan_roll"}, 400)
			clients[owner].command(current(clients[owner]), "action", game.Action{Type: "catan_end"}, 200)
			next := s.rooms[id].Game.Turn
			clients[next].command(current(clients[next]), "action", game.Action{Type: "catan_roll"}, 200)
			if s.rooms[id].Game.Catan.RollID != 2 {
				t.Fatal("next real turn did not draw exactly once")
			}
		})
	}
}
