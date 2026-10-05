package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Provision a revealed card's response after legal setup. The unverified
// event catalogue is deliberately not exposed through a room option/action.
func cardEventResponseFixture(t *testing.T, r *Room, variant string, now time.Time) {
	t.Helper()
	if variant == "city" {
		state, err := game.NewCatanCitiesKnights(3, game.CatanOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for state.Catan.SetupStep < state.Catan.SetupLimit() {
			a, err := state.BotAction(state.Turn)
			if err != nil {
				t.Fatal(err)
			}
			if err := state.Apply(state.Turn, a); err != nil {
				t.Fatal(err)
			}
		}
		r.Game = state
	}
	g := r.Game.Catan
	for i := range g.Players {
		for color, count := range g.Players[i].Resources {
			g.Bank[color] += count
			g.Players[i].Resources[color] = 0
		}
	}
	for i := range g.Tiles {
		g.Tiles[i].Number = 0
	}
	production := []int{}
	for p := range g.Players {
		found := false
		for _, v := range g.Vertices {
			if v.Owner == p && ((variant == "city" && v.Level == 2) || (variant != "city" && v.Level == 1)) {
				production = append(production, v.ID)
				found = true
				break
			}
		}
		if !found {
			t.Fatal("fixture needs one production building per player")
		}
	}
	g.Tiles[0].Vertices, g.Tiles[0].Number, g.Tiles[0].Resource = production, 6, 0
	g.Robber = -1
	if variant == "gold" {
		g.Seafarers = &game.CatanSeafarers{Pirate: -1}
		g.Tiles[0].Resource = game.CatanGold
	}
	g.CardEvent = &game.CatanCardEvent{Kind: "earthquake", Production: 6, Players: []int{0, 1, 2}}
	if variant == "city" {
		g.CardEvent.Red, g.CardEvent.Face = 2, 3
		g.CitiesKnights.BarbarianPosition = 6
		g.CitiesKnights.RobberStart = -1
		for _, v := range g.Vertices {
			if v.Owner == -1 {
				g.CitiesKnights.Knights = []game.CatanKnight{{Owner: 0, Vertex: v.ID, Strength: 1, Active: true}}
				break
			}
		}
	}
	g.RollID++
	g.Dice = []int{g.CardEvent.Red, 0}
	r.Game.Turn, r.Game.Phase = 0, "catan_card_event"
	r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
	if !r.adjustCatanResponseClock("catan_roll", -1, g.SetupStep, now) || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("event card response did not pause active clock")
	}
}

func cardEventHTTPAction(t *testing.T, state *game.State) game.Action {
	t.Helper()
	actor := state.CatanPendingActor()
	g := state.Catan
	switch state.Phase {
	case "catan_card_event":
		if g.CardEvent.Kind == "good_neighbors" {
			give := make([]int, len(g.Bank))
			for color, count := range g.Players[actor].Resources {
				if count > 0 {
					give[color] = 1
					return game.Action{Type: "catan_event_gift", Give: give}
				}
			}
		}
		if g.CardEvent.Kind == "plentiful_year" || g.CardEvent.Kind == "calm_seas" || g.CardEvent.Kind == "tournament" {
			return game.Action{Type: "catan_event_resource", Take: []int{1, 0, 0, 0, 0}}
		}
		if g.CardEvent.Kind == "robber_flees" {
			for _, tile := range g.Tiles {
				if tile.Resource == game.CatanDesert {
					return game.Action{Type: "catan_robber_flees", Tile: tile.ID}
				}
			}
		}
		for _, e := range g.Edges {
			if e.Owner == actor && !e.Ship && !e.Damaged {
				return game.Action{Type: "catan_earthquake", Edge: e.ID}
			}
		}
	case "catan_gold":
		return game.Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}}
	case "catan_pillage":
		for _, v := range g.Vertices {
			if v.Owner == actor && v.Level == 2 {
				return game.Action{Type: "catan_pillage", Vertex: v.ID}
			}
		}
	}
	t.Fatal("unexpected fixture response", state.Phase, actor)
	return game.Action{}
}

func TestCatanCardEventHTTPResponseRestartAutoplayTimeoutAndProduction(t *testing.T) {
	for _, variant := range []string{"base", "gold", "city"} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%s/%s", variant, mode), func(t *testing.T) {
				s, ts, clients, id := newCatanTable(t)
				clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				s.mu.Lock()
				cardEventResponseFixture(t, s.rooms[id], variant, time.Now())
				if err := s.save(s.rooms[id]); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				first := cardEventHTTPAction(t, s.rooms[id].Game)
				before, _ := json.Marshal(s.rooms[id])
				for _, p := range []int{1, 2, 3} {
					clients[p].command(current(clients[p]), "action", first, 400)
				}
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_roll"}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid responder mutated room/clock")
				}
				clients[0].command(current(clients[0]), "action", first, 200)
				if s.rooms[id].Game.CatanPendingActor() != 1 {
					t.Fatal("first choice did not advance queue")
				}
				for viewer, c := range clients {
					v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					legal := v["legal"].(map[string]any)
					if (len(legal["earthquakeRoads"].([]any)) == 2) != (viewer == 1) {
						t.Fatal("road choices exposed to wrong viewer")
					}
					for p, raw := range v["players"].([]any) {
						_, visible := raw.(map[string]any)["resources"]
						if visible != (p == viewer) {
							t.Fatal("event hand privacy")
						}
					}
					if k, ok := v["citiesKnights"].(map[string]any); ok {
						if _, leak := k["progressDecks"]; leak {
							t.Fatal("progress deck leaked")
						}
					}
				}
				before, _ = json.Marshal(s.rooms[id])
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
					t.Fatal("pending queue/dice/number/damage/timer changed on restart")
				}
				ts2 := httptest.NewServer(next.Handler())
				defer ts2.Close()
				for _, c := range clients {
					c.base = ts2.URL
				}
				steps := 0
				for next.rooms[id].Game.CatanPendingActor() >= 0 && steps < 8 {
					r := next.rooms[id]
					actor := r.Game.CatanPendingActor()
					if r.Game.Catan.CardEvent != nil {
						for _, p := range r.Game.Catan.Players {
							for _, count := range p.Resources {
								if count != 0 {
									t.Fatal("production before road choices completed")
								}
							}
						}
					}
					resolvedAt := time.Now()
					if mode == "manual" {
						clients[actor].command(current(clients[actor]), "action", cardEventHTTPAction(t, r.Game), 200)
					} else if mode == "autoplay" {
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						next.mu.Lock()
						next.rooms[id].BotAt = 0
						next.runBots(resolvedAt)
						next.mu.Unlock()
					} else {
						resolvedAt = time.UnixMilli(r.TurnDeadline)
						next.mu.Lock()
						next.expireSetups(resolvedAt)
						next.mu.Unlock()
					}
					r = next.rooms[id]
					want := int64(45000)
					if r.Game.CatanPendingActor() >= 0 {
						want = 120000
					}
					got := r.TurnDeadline - resolvedAt.UnixMilli()
					if r.CatanTimeLeft != 45000 || got < want || got > want+1000 || r.Game.Turn != 0 {
						t.Fatal("response window/original clock incorrect", r.Game.Phase, got, want)
					}
					steps++
				}
				result := next.rooms[id].Game
				g := result.Catan
				if result.Phase != "catan_turn" || g.CardEvent != nil || result.CatanPendingActor() != -1 || g.RollID != 1 {
					t.Fatal("response chain did not finish", result.Phase)
				}
				for p, hand := range g.Players {
					damaged, resources := 0, 0
					for _, e := range g.Edges {
						if e.Owner == p && e.Damaged {
							damaged++
						}
					}
					for _, count := range hand.Resources {
						resources += count
					}
					want := 1
					if variant == "city" && p == 0 {
						want = 2
					}
					if damaged != 1 || resources != want || (variant != "gold" && hand.Resources[0] != 1) {
						t.Fatal("damage or production repeated/missing", p, damaged, hand.Resources)
					}
				}
				if variant == "city" && (g.CitiesKnights.Invasions != 1 || g.CitiesKnights.Event != nil || g.Players[0].Resources[5] != 1) {
					t.Fatal("city continuation used wrong dice or pre-pillage buildings")
				}
			})
		}
	}
}
