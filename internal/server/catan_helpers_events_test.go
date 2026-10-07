package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func newHelperEventTable(t *testing.T, n int) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("助手事件%d", p))
	}
	options := game.CatanOptions{FiveSix: n > 4, Helpers: true, AllHelpers: true}
	raw := clients[0].post("/api/rooms", map[string]any{"name": "助手事件验证", "kind": "catan", "capacity": n, "catanOptions": options}, 201)
	id := raw["id"].(string)
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := 0; p < n; p++ {
		clients[p].command(current(clients[p]), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	// Only the explicitly private reference catalogue is installed. Setup,
	// allocation of all initial helpers, and actions use the actual HTTP routes.
	attachReferenceEventDeck(t, s.rooms[id].Game)
	for s.rooms[id].Game.Catan.SetupStep < s.rooms[id].Game.Catan.SetupLimit() {
		state := s.rooms[id].Game
		a, err := state.BotAction(state.Turn)
		if err != nil {
			t.Fatal(err)
		}
		clients[state.Turn].command(current(clients[state.Turn]), "action", a, 200)
	}
	return s, ts, clients, id
}

func TestCatanHelperEventsHTTPRecoveryClocksAndTimeout(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, kind := range []string{"hilda", "thorolf"} {
			for _, mode := range []string{"manual", "autoplay", "timeout"} {
				t.Run(fmt.Sprintf("%d/%s/%s", n, kind, mode), func(t *testing.T) {
					s, ts, clients, id := newHelperEventTable(t, n)
					r := s.rooms[id]
					g := r.Game.Catan
					owner := r.Game.Turn
					helper := (owner + 1) % n
					want := 3
					if kind == "thorolf" {
						want = 5
					}
					old := g.Players[helper].Helper.ID
					if old != want {
						if at := slices.Index(g.HelperDisplay, want); at >= 0 {
							g.HelperDisplay[at] = old
						} else {
							for p := range g.Players {
								if g.Players[p].Helper.ID == want {
									g.Players[p].Helper = &game.CatanHelperSeat{ID: old}
									break
								}
							}
						}
						g.Players[helper].Helper = &game.CatanHelperSeat{ID: want}
					}
					if kind == "hilda" {
						// Isolate zero production for Hilda without changing the deck,
						// terrain or production numbers. Explicit middle-game fixture.
						for i := range g.Vertices {
							if g.Vertices[i].Owner == helper {
								g.Vertices[i].Owner = -1
								g.Vertices[i].Level = 0
							}
						}
					} else {
						// Keep seven cards: the compulsory resource response then
						// exchange must precede the other player's discard.
						for p := range g.Players {
							for c, num := range g.Players[p].Resources {
								g.Bank[c] += num
								g.Players[p].Resources[c] = 0
							}
						}
						g.Bank[0] -= 7
						g.Players[helper].Resources[0] = 7
						g.Bank[1] -= 8
						g.Players[owner].Resources[1] = 8
						pile := g.EventDeck.Deck.DrawPile
						at := slices.Index(pile, 15)
						pile[at], pile[len(pile)-1] = pile[len(pile)-1], pile[at]
					}
					r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
					if err := s.save(r); err != nil {
						t.Fatal(err)
					}
					clients[owner].command(current(clients[owner]), "action", game.Action{Type: "catan_roll"}, 200)
					seenResource, seenExchange := false, false
					for step := 0; s.rooms[id].Game.Phase != "catan_turn" && step < 40; step++ {
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						r = s.rooms[id]
						actor := twoHTTPActor(r.Game)
						action, err := r.Game.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
						q := r.Game.Catan.HelperPending
						if q != nil {
							if actor != helper || r.Game.Turn != owner || r.Game.Catan.CardEvent != nil {
								t.Fatal("wrong helper continuation")
							}
							seenResource = seenResource || q.Kind == "resource"
							seenExchange = seenExchange || q.Kind == "exchange"
							clients[owner].command(current(clients[owner]), "action", action, 400)
						}
						clients[n].command(current(clients[n]), "action", action, 400)
						for p, client := range clients {
							v := current(client)["game"].(map[string]any)["catan"].(map[string]any)
							deck := v["eventDeck"].(map[string]any)
							if deck["deck"] != nil || deck["drawPile"] != nil || deck["remaining"] != float64(35) {
								t.Fatal("hidden deck leak or extra draw")
							}
							for seat, raw := range v["players"].([]any) {
								if (raw.(map[string]any)["resources"] != nil) != (seat == p) {
									t.Fatal("hand privacy")
								}
							}
						}
						request := map[string]any{"type": "action", "version": r.Version, "nonce": fmt.Sprintf("helper-event-%d", step), "action": action}
						switch mode {
						case "manual":
							clients[actor].post("/api/rooms/"+id, request, 200)
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
							if !s.rooms[id].Seats[actor].TimeoutAutoPlay {
								t.Fatal("timeout did not enable persistent takeover")
							}
							reclaimTimeoutHumans(t, s, clients, id)
						}
						if mode == "manual" {
							before, _ := json.Marshal(s.rooms[id])
							s, ts = restartRiversHTTP(t, s, ts, clients, id)
							clients[actor].post("/api/rooms/"+id, request, 200)
							after, _ := json.Marshal(s.rooms[id])
							if string(before) != string(after) {
								t.Fatal("response replay duplicated helper/event")
							}
						}
					}
					r = s.rooms[id]
					if !seenResource || !seenExchange || r.Game.Phase != "catan_turn" || r.Game.Turn != owner || r.Game.Catan.RollID != 1 || r.Game.Catan.HelperPending != nil {
						t.Fatal("event/helper response chain stalled")
					}
					// Timeout uses a future clock; compare remaining budget with the
					// restored timer fields, not with the wall-clock of this test.
					if mode != "timeout" {
						left := r.TurnDeadline - time.Now().UnixMilli()
						if left < 35000 || left > 45000 {
							t.Fatal("helper did not restore original action budget", left)
						}
					}
					for c, total := range r.Game.Catan.Bank {
						for _, p := range r.Game.Catan.Players {
							total += p.Resources[c]
						}
						stock := 19
						if n > 4 {
							stock = 24
						}
						if total != stock {
							t.Fatal("resource conservation")
						}
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
				})
			}
		}
	}
}
