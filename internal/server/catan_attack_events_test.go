package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Explicit revealed-effect fixture, not a public event-deck game. The private
// effect entry is exercised by game tests; this covers real HTTP persistence.
func attackHTTPEventFixture(t *testing.T, n int) *game.State {
	t.Helper()
	state := attackHTTPFixture(t, n)
	g := state.Catan
	state.Turn = 0
	if g.Paired != nil {
		g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, false
	}
	clear(g.Attack.Barbarians)
	for p := range g.Players {
		for c, count := range g.Players[p].Resources {
			g.Bank[c] += count
			g.Players[p].Resources[c] = 0
		}
	}
	g.Players[2].Resources[1] = 2
	g.Bank[1] -= 2
	if err := json.Unmarshal([]byte(`[{"player":1,"edge":0},{"player":1,"edge":1}]`), &g.Attack.Knights); err != nil {
		t.Fatal(err)
	}
	g.RollID++
	g.Dice = []int{0, 0}
	g.CardEvent = &game.CatanCardEvent{Kind: "conflict", Production: 6, Players: []int{1}}
	g.RevealedEvent = &game.CatanRevealedEvent{Kind: "conflict", Production: 6, RollID: g.RollID}
	state.Phase = "catan_card_event"
	return state
}

func TestCatanAttackEventHTTPConflictRecovery(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
				state := attackHTTPEventFixture(t, n)
				// Expected ordinary production on the actual starting map, with no
				// conquered hexes, resource shortages or city modifiers in this fixture.
				expected := make([][]int, n)
				for p := range expected {
					expected[p] = slices.Clone(state.Catan.Players[p].Resources)
				}
				expected[1][1]++
				expected[2][1]--
				for _, tile := range state.Catan.Tiles {
					if tile.Number == 6 && tile.Resource >= 0 && tile.Resource < 5 {
						for _, v := range tile.Vertices {
							b := state.Catan.Vertices[v]
							if b.Level > 0 && b.Owner >= 0 {
								expected[b.Owner][tile.Resource] += b.Level
							}
						}
					}
				}
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				now := time.Now()
				r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
				if !r.adjustCatanResponseClock("catan_roll", -1, state.Catan.SetupStep, now) || r.CatanTimeLeft != 45000 {
					t.Fatal("event response clock did not pause")
				}
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				for viewer, c := range clients {
					v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					targets := v["legal"].(map[string]any)["eventTargets"].([]any)
					if (len(targets) == 1) != (viewer == 1) || v["cardEvent"].(map[string]any)["canSkip"] != false {
						t.Fatal("wrong responder/skip controls")
					}
					for p, raw := range v["players"].([]any) {
						if (raw.(map[string]any)["resources"] != nil) != (p == viewer) {
							t.Fatal("opponent hand exposed")
						}
					}
				}
				action := game.Action{Type: "catan_event_steal", Target: 2}
				before, _ := json.Marshal(s.rooms[id])
				for _, viewer := range []int{0, 2, n} {
					clients[viewer].command(current(clients[viewer]), "action", action, 400)
				}
				clients[1].command(current(clients[1]), "action", game.Action{Type: "catan_event_skip"}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid request changed room")
				}
				var replay map[string]any
				resolvedAt := time.Now()
				switch mode {
				case "manual":
					replay = map[string]any{"type": "action", "version": s.rooms[id].Version, "nonce": "attack-event-choice", "action": action}
					clients[1].post("/api/rooms/"+id, replay, 200)
				case "autoplay":
					setAutoPlay(clients[1], current(clients[1]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(resolvedAt)
					s.mu.Unlock()
					setAutoPlay(clients[1], current(clients[1]), false, 200)
				case "timeout":
					resolvedAt = time.UnixMilli(s.rooms[id].TurnDeadline)
					s.mu.Lock()
					s.expireSetups(resolvedAt)
					s.mu.Unlock()
				}
				r = s.rooms[id]
				g := r.Game.Catan
				if r.Game.Phase != "catan_turn" || r.Game.Turn != 0 || g.CardEvent != nil || !g.RevealedEvent.ProductionStarted || g.Robber != -1 {
					t.Fatal("conflict continuation failed")
				}
				for p := range expected {
					if !slices.Equal(g.Players[p].Resources, expected[p]) {
						t.Fatal("theft/production count", p, g.Players[p].Resources, expected[p])
					}
				}
				if left := r.TurnDeadline - resolvedAt.UnixMilli(); left < 45000 || left > 46000 {
					t.Fatal("original time not restored", left)
				}
				if n == 6 && g.Paired.Second {
					t.Fatal("event changed active pair")
				}
				before, _ = json.Marshal(r)
				if replay != nil {
					clients[1].post("/api/rooms/"+id, replay, 200)
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				if replay != nil {
					clients[1].post("/api/rooms/"+id, replay, 200)
				}
				after, _ = json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("restart/replay duplicated theft or production")
				}
			})
		}
	}
}
