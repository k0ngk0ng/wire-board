package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanTwoFishingPublicRematchAndBaseReset(t *testing.T) {
	s, ts, clients, id := newTwoScenarioFullTable(t, "fishing", true)
	for _, scene := range []string{"", "fishing"} {
		clients[0].command(current(clients[0]), "close", nil, 200)
		clients[0].command(current(clients[0]), "rematch", nil, 200)
		clients[0].post("/api/rooms/"+id, map[string]any{"type": "catan_two_scenario", "catanTwoScenario": scene, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
		s, ts = restartRiversHTTP(t, s, ts, clients, id)
		for p := range 2 {
			clients[p].command(current(clients[p]), "ready", nil, 200)
		}
		clients[0].command(current(clients[0]), "start", nil, 200)
		g := s.rooms[id].Game.Catan
		if (g.Fishing != nil) != (scene == "fishing") || g.EventDeck == nil {
			t.Fatal("rematch recipe/event preservation")
		}
		if scene == "" && (g.Two.Bank != 10 || !slices.Equal(g.Two.Tokens, []int{5, 5})) {
			t.Fatal("base trade tokens not restored")
		}
		if scene == "fishing" && (g.Two.Bank != 0 || len(g.Fishing.Tokens.Hands[0]) != 5) {
			t.Fatal("fish opening missing")
		}
	}
}

// Explicit midgame fixtures isolate a full fish hand interrupting either
// production. Use real Apply rolls, persisted responses and HTTP commands.
func TestCatanTwoFishingProductionResponseHTTP(t *testing.T) {
	for _, second := range []bool{false, true} {
		for _, opponent := range []bool{false, true} {
			for _, mode := range []string{"manual", "timeout"} {
				t.Run(fmt.Sprintf("second=%v/opponent=%v/%s", second, opponent, mode), func(t *testing.T) {
					s, ts, clients, id := newTwoScenarioFullTable(t, "fishing")
					state := s.rooms[id].Game
					for state.Catan.SetupStep < state.Catan.SetupLimit() {
						a, e := state.BotAction(state.Turn)
						if e != nil {
							t.Fatal(e)
						}
						if e = state.Apply(state.Turn, a); e != nil {
							t.Fatal(e)
						}
					}
					g := state.Catan
					p := state.Turn
					if opponent {
						p = 1 - p
					}
					for i, v := range g.Vertices {
						if v.Owner >= 0 {
							g.Vertices[i].Owner = -1
							g.Vertices[i].Level = 0
						}
					}
					vertex := g.Fishing.Map.Grounds[0].Vertices[1]
					g.Vertices[vertex].Owner, g.Vertices[vertex].Level = p, 2
					for i := range g.Players {
						g.Players[i].Score = 0
					}
					g.Players[p].Score = 2
					f := &g.Fishing.Tokens
					for i, h := range f.Hands {
						f.DrawPile = append(f.DrawPile, h...)
						f.Hands[i] = []int{}
					}
					for _, token := range []int{0, 1, 2, 3, 4, 5, 6} {
						at := slices.Index(f.DrawPile, token)
						f.DrawPile = slices.Delete(f.DrawPile, at, at+1)
						f.Hands[p] = append(f.Hands[p], token)
					}
					if second {
						g.Two.Rolls = []int{2}
						g.RollID = 1
						g.Fishing.LastRollID = 1
					}
					initial, _ := json.Marshal(state)
					found := false
					for attempt := 0; attempt < 500; attempt++ {
						if e := json.Unmarshal(initial, state); e != nil {
							t.Fatal(e)
						}
						if e := state.Apply(state.Turn, game.Action{Type: "catan_roll"}); e != nil {
							t.Fatal(e)
						}
						if state.Phase == "catan_fish_replace" {
							found = true
							break
						}
					}
					if !found || state.CatanPendingActor() != p {
						t.Fatal("no fish response")
					}
					r := s.rooms[id]
					now := time.Now()
					r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
					if !r.adjustCatanResponseClock("catan_roll", -1, state.Catan.SetupStep, now) || r.CatanTimeLeft != 45000 {
						t.Fatal("clock not paused")
					}
					if e := s.save(r); e != nil {
						t.Fatal(e)
					}
					assertTwoHTTPPrivacy(t, clients, state)
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					r = s.rooms[id]
					before, _ := json.Marshal(r)
					for _, wrong := range []int{1 - p, 2} {
						clients[wrong].command(current(clients[wrong]), "action", game.Action{Type: "catan_fish_keep"}, 400)
					}
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("invalid responder changed state")
					}
					at := time.Now()
					if mode == "manual" {
						clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_fish_keep"}, 200)
					} else {
						at = time.UnixMilli(r.TurnDeadline)
						s.mu.Lock()
						s.expireSetups(at)
						s.mu.Unlock()
						if !s.rooms[id].Seats[p].AutoPlay || !s.rooms[id].Seats[p].TimeoutAutoPlay {
							t.Fatal("timeout not persistent takeover")
						}
					}
					r = s.rooms[id]
					want := "catan_roll"
					rolls := 1
					if second {
						want = "catan_turn"
						rolls = 2
					}
					if r.Game.Phase != want || len(r.Game.Catan.Two.Rolls) != rolls || r.Game.Catan.Fishing.LastRollID != rolls || r.Game.CatanPendingActor() != -1 || r.TurnDeadline-at.UnixMilli() < 44000 || r.TurnDeadline-at.UnixMilli() > 46000 {
						t.Fatal("wrong production/clock continuation", r.Game.Phase, r.TurnDeadline-at.UnixMilli())
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					assertTwoHTTPInventory(t, s.rooms[id].Game)
				})
			}
		}
	}
}
