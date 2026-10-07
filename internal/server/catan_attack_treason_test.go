package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func prepareAttackHTTPTreason(t *testing.T, n int, kind string) *game.State {
	t.Helper()
	state := attackHTTPFixture(t, n)
	g, a := state.Catan, state.Catan.Attack
	clear(a.Barbarians)
	left := a.Map.Barbarians
	switch kind {
	case "empty_stock":
	case "single_supply":
		left--
	case "one_destination":
		for _, tile := range a.Map.Coast {
			a.Barbarians[tile] = 3
			left -= 3
		}
		a.Barbarians[a.Map.Coast[0]] = 2
		left = 0 // Leave unplaced pieces in supply in this crowded-board fixture.
	default:
		t.Fatal("unknown HTTP treason fixture")
	}
	for i := 0; i < left; i++ {
		a.Prisoners[i%n]++
	}
	for p := range g.Players {
		score := a.Prisoners[p] / 2
		for _, v := range g.Vertices {
			if v.Owner != p || v.Level == 0 {
				continue
			}
			for _, tile := range g.Tiles {
				if slices.Contains(tile.Vertices, v.ID) && a.Barbarians[tile.ID] != 3 {
					score += v.Level
					break
				}
			}
		}
		g.Players[p].Score = score
	}
	prepareAttackHTTPCard(t, state, "treason")
	return state
}

func TestCatanAttackPartialTreasonHTTPResponseRecovery(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, kind := range []string{"empty_stock", "single_supply", "one_destination"} {
			for _, mode := range []string{"manual", "autoplay", "timeout"} {
				t.Run(fmt.Sprintf("%d/%s/%s", n, kind, mode), func(t *testing.T) {
					s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
					state := prepareAttackHTTPTreason(t, n, kind)
					actor := state.Turn
					beforeCounts := slices.Clone(state.Catan.Attack.Barbarians)
					s.mu.Lock()
					r := s.rooms[id]
					r.Game = state
					r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
					deadline := r.TurnDeadline
					if err := s.save(r); err != nil {
						t.Fatal(err)
					}
					s.mu.Unlock()
					purchase := map[string]any{"type": "action", "version": r.Version, "nonce": "partial-treason-buy", "action": game.Action{Type: "catan_buy_dev"}}
					clients[actor].post("/api/rooms/"+id, purchase, 200)
					r = s.rooms[id]
					var replay map[string]any
					if kind == "empty_stock" {
						if r.Game.Phase != "catan_turn" || r.Game.Catan.Attack.Pending != nil || r.TurnDeadline != deadline {
							t.Fatal("empty effect should resolve without response clock")
						}
						replay = purchase
					} else {
						if r.Game.Phase != "catan_attack_card" || r.TurnDeadline-time.Now().UnixMilli() < 119000 || r.CatanTimeLeft < 40000 {
							t.Fatal("partial effect missing response clock")
						}
						choice, err := r.Game.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
						if len(choice.Give) != 1 || len(choice.Take) != 1 {
							t.Fatal("expected exactly one move")
						}
						before, _ := json.Marshal(r)
						bad := choice
						bad.Give = nil
						bad.Take = nil
						clients[actor].command(current(clients[actor]), "action", bad, 400)
						for _, viewer := range []int{(actor + 1) % n, n} {
							clients[viewer].command(current(clients[viewer]), "action", choice, 400)
						}
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("invalid partial response changed room")
						}
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						for viewer, c := range clients {
							pub := current(c)["game"].(map[string]any)["catan"].(map[string]any)["attack"].(map[string]any)
							if pub["treasonRule"] != "as-much-as-possible" || pub["deck"] != nil {
								t.Fatal("public rule/secret deck")
							}
							if viewer == actor {
								if pub["treasonCount"] != float64(1) || pub["treasonPlans"] == nil {
									t.Fatal("missing exact partial controls")
								}
							} else if pub["treasonCount"] != nil || pub["treasonPlans"] != nil {
								t.Fatal("response controls leaked")
							}
						}
						switch mode {
						case "manual":
							replay = map[string]any{"type": "action", "version": s.rooms[id].Version, "nonce": "partial-treason-choice", "action": choice}
							clients[actor].post("/api/rooms/"+id, replay, 200)
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
						}
					}
					r = s.rooms[id]
					a := r.Game.Catan.Attack
					if r.Game.Phase != "catan_turn" || r.Game.Turn != actor || a.Pending != nil || len(a.Discard) != 1 || len(a.Deck) != 25 || a.Gold[actor] != 2 || a.CardSequence != 1 {
						t.Fatal("partial card not completed exactly once")
					}
					if n == 6 && !r.Game.Catan.Paired.Second {
						t.Fatal("paired secondary action lost")
					}
					moves := 0
					for tile, count := range a.Barbarians {
						if count > beforeCounts[tile] {
							moves += count - beforeCounts[tile]
						}
					}
					want := 1
					if kind == "empty_stock" {
						want = 0
					}
					if moves != want {
						t.Fatal("wrong number of moved pieces")
					}
					before, _ := json.Marshal(r)
					if replay != nil {
						clients[actor].post("/api/rooms/"+id, replay, 200)
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					if replay != nil {
						clients[actor].post("/api/rooms/"+id, replay, 200)
					}
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("replay/restart repeated partial effect")
					}
				})
			}
		}
	}
}
