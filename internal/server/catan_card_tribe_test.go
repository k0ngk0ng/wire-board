package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"
	"time"
)

// Explicitly revealed-effect fixture on an official scenario map, not an
// assertion that the unverified full 2025 event deck has been integrated.
func TestCatanCardTribeFleeHTTPRestart(t *testing.T) {
	for _, layout := range []string{"fixed", "variable"} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%s/%s", layout, mode), func(t *testing.T) {
				s, ts, clients, id := newCatanTable(t)
				clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				state, err := game.NewCatanSeafarers(3, game.CatanOptions{}, game.CatanSeafarersSetup{Scenario: "tribe", Layout: layout}, nil)
				if err != nil {
					t.Fatal(err)
				}
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
				actor := state.Turn
				deserts := []int{}
				for _, tile := range g.Tiles {
					if tile.Resource == game.CatanDesert {
						deserts = append(deserts, tile.ID)
					}
					if tile.Number > 0 {
						g.Robber = tile.ID
					}
				}
				if len(deserts) < 2 {
					t.Fatal("missing official deserts")
				}
				wantHands := [][]int{}
				for _, p := range g.Players {
					wantHands = append(wantHands, append([]int{}, p.Resources...))
				}
				// No stealing: only the explicitly selected card's ordinary production
				// may change hands after the robber leaves numbered terrain.
				for _, tile := range g.Tiles {
					if tile.Number == 4 && tile.Resource < 5 {
						for _, vID := range tile.Vertices {
							v := g.Vertices[vID]
							if v.Owner >= 0 && v.Level > 0 {
								wantHands[v.Owner][tile.Resource] += v.Level
							}
						}
					}
				}
				tribeBefore, _ := json.Marshal(g.Seafarers.Tribe)
				pirate := g.Seafarers.Pirate
				g.RollID++
				g.Dice = []int{0, 0}
				g.CardEvent = &game.CatanCardEvent{Kind: "robber_flees", Production: 4, Players: []int{actor}}
				state.Phase = "catan_card_event"
				now := time.Now()
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
				if !r.adjustCatanResponseClock("catan_roll", -1, g.SetupStep, now) {
					t.Fatal("response clock did not start")
				}
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				action := game.Action{Type: "catan_robber_flees", Tile: deserts[len(deserts)-1]}
				before, _ := json.Marshal(r)
				clients[(actor+1)%3].command(current(clients[(actor+1)%3]), "action", action, 400)
				clients[3].command(current(clients[3]), "action", action, 400)
				clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_robber_flees", Tile: g.Robber}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid action changed room")
				}
				for viewer, c := range clients {
					view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					legal := view["legal"].(map[string]any)["fleeDeserts"].([]any)
					if (len(legal) == len(deserts)) != (viewer == actor) || viewer != actor && len(legal) != 0 {
						t.Fatal("wrong event legal view", viewer, legal)
					}
					for _, raw := range view["seafarers"].(map[string]any)["tribe"].(map[string]any)["development"].([]any) {
						if len(raw.(map[string]any)) != 1 {
							t.Fatal("reward identities leaked")
						}
					}
					for p, raw := range view["players"].([]any) {
						if _, ok := raw.(map[string]any)["resources"]; ok != (viewer == p) {
							t.Fatal("hand leak")
						}
					}
				}
				ts.Close()
				s.Close()
				next, err := New(s.cfg, s.files)
				if err != nil {
					t.Fatal(err)
				}
				defer next.Close()
				stopBotTicker(next)
				after, _ = json.Marshal(next.rooms[id])
				if string(before) != string(after) {
					t.Fatal("pending flee failed to restore")
				}
				ts2 := httptest.NewServer(next.Handler())
				defer ts2.Close()
				for _, c := range clients {
					c.base = ts2.URL
				}
				at := time.Now()
				switch mode {
				case "manual":
					clients[actor].command(current(clients[actor]), "action", action, 200)
				case "autoplay":
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					next.mu.Lock()
					next.rooms[id].BotAt = 0
					next.runBots(at)
					next.mu.Unlock()
				case "timeout":
					at = time.UnixMilli(next.rooms[id].TurnDeadline)
					next.mu.Lock()
					next.expireSetups(at)
					next.mu.Unlock()
				}
				r = next.rooms[id]
				g = r.Game.Catan
				if r.Game.Phase != "catan_turn" || g.CardEvent != nil || !slices.Contains(deserts, g.Robber) || len(g.Victims) != 0 || g.Seafarers.Pirate != pirate || g.RollID != 1 {
					t.Fatal("flee continuation changed wrong game state", r.Game.Phase)
				}
				if mode == "manual" && g.Robber != action.Tile {
					t.Fatal("selected desert ignored")
				}
				for p, hand := range wantHands {
					if !reflect.DeepEqual(g.Players[p].Resources, hand) {
						t.Fatal("missing/repeated production or unintended theft", p, g.Players[p].Resources, hand)
					}
				}
				tribeAfter, _ := json.Marshal(g.Seafarers.Tribe)
				if string(tribeBefore) != string(tribeAfter) {
					t.Fatal("flee changed rewards or ports")
				}
				if remaining := r.TurnDeadline - at.UnixMilli(); remaining < 45000 || remaining > 46000 {
					t.Fatal("original action time not restored", remaining)
				}
				face := g.RevealedEvent
				if face == nil || face.Kind != "robber_flees" || face.Production != 4 || !face.ProductionStarted {
					t.Fatal("revealed face lost")
				}
			})
		}
	}
}
