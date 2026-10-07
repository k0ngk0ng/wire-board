package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanAttackPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("蛮族进攻房主")
	guest.register("蛮族进攻朋友")
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 2},
		{"kind": "catan", "capacity": 7},
		{"kind": "catan", "capacity": 4, "catanOptions": game.CatanOptions{Helpers: true}},
		{"kind": "catan", "capacity": 6, "catanOptions": game.CatanOptions{FiveSix: true}},
		{"kind": "catan", "capacity": 3, "catanCitiesKnights": game.CatanCitiesKnightsSetup{}},
		{"kind": "splendor", "capacity": 3},
	} {
		body["name"], body["catanScenario"] = "无效蛮族进攻", "barbarian-attack"
		host.post("/api/rooms", body, 400)
	}
	raw := host.post("/api/rooms", map[string]any{"name": "蛮族进攻选择", "kind": "catan", "capacity": 6, "catanScenario": "barbarian-attack"}, 201)
	id := raw["id"].(string)
	guest.command(raw, "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	choose := func(c *testClient, choice string, status int) {
		c.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": choice, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	ready := func() {
		for _, c := range []*testClient{host, guest} {
			c.command(current(c), "ready", nil, 200)
		}
	}
	ready()
	before, _ := json.Marshal(s.rooms[id])
	choose(guest, "spices-for-catan", 400)
	choose(host, "fishing", 400)
	choose(host, "", 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid choice changed room")
	}
	choose(host, "barbarian-attack", 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same recipe cleared readiness")
		}
	}
	for _, scenario := range []string{"spices-for-catan", "barbarian-attack"} {
		choose(host, scenario, 200)
		for _, seat := range s.rooms[id].Seats {
			if seat.Ready != seat.Bot {
				t.Fatal("changed recipe retained readiness")
			}
		}
		ready()
	}
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	// Explicit corrupt saved waiting states must not silently become base games.
	valid := *s.rooms[id]
	for _, mutate := range []func(*Room){
		func(r *Room) { r.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{} },
		func(r *Room) { r.CatanSeafarers = &game.CatanSeafarersSetup{} },
		func(r *Room) { r.CatanTwoRules = game.CatanTwoRules },
		func(r *Room) { r.CatanOptions.Helpers = true },
		func(r *Room) { r.CatanOptions.FiveSix = true },
	} {
		trial := valid
		mutate(&trial)
		s.rooms[id] = &trial
		before, _ = json.Marshal(s.rooms[id])
		host.command(current(host), "start", nil, 400)
		after, _ = json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("corrupt recipe changed state")
		}
	}
	s.rooms[id] = &valid
	host.command(current(host), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if g.Attack == nil || len(g.Players) != 3 || len(g.Tiles) != 19 || g.Paired != nil || g.Options.FiveSix {
		t.Fatal("six-seat room did not use actual three-player map")
	}
	choose(host, "spices-for-catan", 400)
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	if s.rooms[id].CatanScenario != "barbarian-attack" {
		t.Fatal("rematch lost recipe")
	}
	choose(host, "spices-for-catan", 200)
	ready()
	host.command(current(host), "start", nil, 200)
	if s.rooms[id].Game.Catan.Attack != nil || s.rooms[id].Game.Catan.Explorer == nil {
		t.Fatal("rematch kept old scenario")
	}
}

func assertPublicAttackInventory(t *testing.T, state *game.State) {
	t.Helper()
	g, a := state.Catan, state.Catan.Attack
	supply := 19
	if len(g.Players) > 4 {
		supply = 24
	}
	for color, total := range g.Bank {
		if total < 0 {
			t.Fatal("negative bank")
		}
		for _, p := range g.Players {
			if p.Resources[color] < 0 {
				t.Fatal("negative hand")
			}
			total += p.Resources[color]
		}
		if total != supply {
			t.Fatal("resource supply", color, total)
		}
	}
	gold, used := a.GoldBank, 0
	for _, count := range a.Barbarians {
		if count < 0 || count > 3 {
			t.Fatal("invalid occupation")
		}
		used += count
	}
	for p, player := range g.Players {
		gold += a.Gold[p]
		used += a.Prisoners[p]
		score := a.Prisoners[p] / 2
		if g.LongestOwner == p {
			score += 2
		}
		for _, v := range g.Vertices {
			if v.Owner == p && v.Level > 0 {
				conquered := true
				for _, tile := range g.Tiles {
					if slices.Contains(tile.Vertices, v.ID) && a.Barbarians[tile.ID] != 3 {
						conquered = false
					}
				}
				if !conquered {
					score += v.Level
				}
			}
		}
		if score != player.Score {
			t.Fatal("occupation score", p, score, player.Score)
		}
	}
	if gold != a.Map.Gold+a.GoldIssued || used > a.Map.Barbarians {
		t.Fatal("gold/barbarian supply")
	}
	counts := map[string]int{}
	for _, cards := range [][]string{a.Deck, a.Discard} {
		for _, card := range cards {
			counts[card]++
		}
	}
	if a.Pending != nil {
		counts[a.Pending.Card]++
	}
	if len(counts) != 4 || counts["capture"] != 4 || counts["knighthood"] != 14 || counts["swift_knight"] != 4 || counts["treason"] != 4 {
		t.Fatal("special card supply", counts)
	}
	knights := make([]int, len(g.Players))
	edges := map[int]bool{}
	for _, k := range a.Knights {
		knights[k.Player]++
		if knights[k.Player] > 6 || edges[k.Edge] {
			t.Fatal("knight supply/overlap")
		}
		edges[k.Edge] = true
	}
}

func TestCatanAttackPublicFullHTTPGames(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newPublicScenarioTable(t, n, "barbarian-attack")
			restored := map[string]bool{}
			restart := func(label string) { s, ts = restartRiversHTTP(t, s, ts, clients, id); restored[label] = true }
			restart("setup")
			manual, auto, timed, steps := 0, 0, 0, 0
			for ; steps < 8000 && !s.rooms[id].Game.Finished; steps++ {
				r := s.rooms[id]
				state, g := r.Game, r.Game.Catan
				assertPublicAttackInventory(t, state)
				label := ""
				if g.SetupStep >= g.SetupLimit() {
					label = state.Phase
				}
				if g.Paired != nil && g.Paired.Second {
					label = "second-" + label
				}
				if label != "" && !restored[label] {
					restart(label)
					r = s.rooms[id]
					state, g = r.Game, r.Game.Catan
				}
				actor := state.CatanPendingActor()
				if actor < 0 {
					actor = state.Turn
				}
				if state.Phase == "catan_discard" {
					for p, due := range g.DiscardDue {
						if due > 0 {
							actor = p
							break
						}
					}
				}
				if steps%31 == 0 {
					for _, viewer := range []int{actor, n} {
						view := current(clients[viewer])["game"].(map[string]any)["catan"].(map[string]any)
						a := view["attack"].(map[string]any)
						if a["deck"] != nil {
							t.Fatal("secret attack deck leaked")
						}
						if viewer != actor {
							if plan, ok := a["endPlan"].(map[string]any); ok && plan["moves"] != nil {
								t.Fatal("private knight plan leaked")
							}
						}
						for p, raw := range view["players"].([]any) {
							if p != viewer && raw.(map[string]any)["resources"] != nil {
								t.Fatal("resource hand leaked")
							}
						}
					}
				}
				version := r.Version
				if steps%29 == 0 {
					s.mu.Lock()
					r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
					s.expireSetups(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= version {
						t.Fatal("timeout stalled", steps, state.Phase)
					}
					timed++
					reclaimTimeoutHumans(t, s, clients, id)
					continue
				}
				if steps%17 == 0 {
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					version = s.rooms[id].Version
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= version {
						t.Fatal("autoplay stalled", steps, state.Phase)
					}
					if !s.rooms[id].Game.Finished {
						setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					}
					auto++
					continue
				}
				action, err := state.BotAction(actor)
				if err != nil {
					t.Fatal(steps, state.Phase, err)
				}
				clients[actor].command(current(clients[actor]), "action", action, 200)
				manual++
			}
			r := s.rooms[id]
			assertPublicAttackInventory(t, r.Game)
			if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || auto == 0 || timed == 0 || manual == 0 {
				t.Fatal("incomplete public match", steps, auto, timed)
			}
			winner := r.Game.Winners[0]
			if winner != r.Game.Turn || r.Game.Catan.Players[winner].Score < 12 || r.Game.Catan.Attack.Sequence == 0 {
				t.Fatal("invalid attack victory")
			}
			if n > 4 && !restored["second-catan_turn"] {
				t.Fatal("no paired action coverage")
			}
			restart("finished")
			for attempt := 0; attempt < 2; attempt++ {
				if err := s.save(s.rooms[id]); err != nil {
					t.Fatal(err)
				}
				code, profile := clients[n].request("GET", "/api/players/"+s.rooms[id].Seats[winner].ID, nil)
				if code != 200 {
					t.Fatal("history unavailable")
				}
				history := profile["history"].([]any)
				if len(history) != 1 {
					t.Fatal("duplicate history")
				}
				record := history[0].(map[string]any)
				if record["catanScenario"] != "barbarian-attack" || record["catanExpansionRules"].(map[string]any)["barbarian_attack"] != r.Game.Catan.Attack.Rules {
					t.Fatal("wrong attack archive")
				}
				stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
				if stats["played"] != float64(1) || stats["wins"] != float64(1) {
					t.Fatal("duplicate rating")
				}
			}
			t.Logf("steps=%d manual=%d autoplay=%d timeouts=%d score=%d", steps, manual, auto, timed, r.Game.Catan.Players[winner].Score)
		})
	}
}
