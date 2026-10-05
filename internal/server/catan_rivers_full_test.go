package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// The scenario stays gated in public creation. Install only its pristine
// constructor state; all setup, resource acquisition and scoring below happen
// through production HTTP, timeout and autoplay paths, never fixture grants.
func newRiversFullTable(t *testing.T, n int) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("河流玩家%d", p))
	}
	opts := game.CatanOptions{FiveSix: n > 4}
	raw := clients[0].post("/api/rooms", map[string]any{"name": "河流完整对局", "kind": "catan", "capacity": n, "catanOptions": opts}, 201)
	id := raw["id"].(string)
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := 0; p < n; p++ {
		clients[p].command(current(clients[0]), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	initial, err := game.NewCatanRivers(n, opts)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	r.Game = initial
	r.startTurnClock(time.Now())
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	return s, ts, clients, id
}

func assertRiversFullInventory(t *testing.T, state *game.State) {
	t.Helper()
	g := state.Catan
	r := g.Rivers
	resourceSupply, coinSupply, devSupply := 19, 100, 25
	if len(g.Players) > 4 {
		resourceSupply, coinSupply, devSupply = 24, 152, 34
	}
	gold := r.Bank
	if gold < 0 || r.Bought < 0 || r.Bought > 2 {
		t.Fatal("invalid coin bank/purchase cap")
	}
	high, low, ties, richest := -1, int(^uint(0)>>1), 0, -1
	for p, n := range r.Gold {
		if n < 0 {
			t.Fatal("negative player coins")
		}
		gold += n
		if n > high {
			high, ties, richest = n, 1, p
		} else if n == high {
			ties++
		}
		if n < low {
			low = n
		}
	}
	if ties != 1 {
		richest = -1
	}
	if gold != coinSupply {
		t.Fatal("gold conservation", gold, coinSupply)
	}
	for c, total := range g.Bank {
		if total < 0 {
			t.Fatal("negative resource bank")
		}
		for _, p := range g.Players {
			if p.Resources[c] < 0 {
				t.Fatal("negative hand")
			}
			total += p.Resources[c]
		}
		if total != resourceSupply {
			t.Fatal("resource conservation", c, total)
		}
	}
	cards := len(g.DevDeck) + len(g.DevDiscard)
	for p, player := range g.Players {
		if player.Eliminated {
			t.Fatal("natural complete game unexpectedly eliminated a player")
		}
		for _, count := range player.Dev {
			cards += count
		}
		bridges, roads, score := 0, 0, player.Dev[4]
		for _, e := range g.Edges {
			if e.Owner == p {
				if e.Bridge {
					bridges++
				} else {
					roads++
				}
			}
		}
		if bridges > 3 || roads > 15 {
			t.Fatal("piece inventory", p, bridges, roads)
		}
		for _, v := range g.Vertices {
			if v.Owner == p {
				score += v.Level
			}
		}
		if g.LongestOwner == p {
			score += 2
		}
		if g.ArmyOwner == p {
			score += 2
		}
		if richest == p {
			score++
		}
		if r.Gold[p] == low {
			score -= 2
		}
		if player.Score != score {
			t.Fatal("independent wealth/building/victory score", p, player.Score, score)
		}
	}
	if cards != devSupply {
		t.Fatal("development conservation", cards)
	}
}

func TestCatanRiversCompleteHTTPGames(t *testing.T) {
	allBridges, allBuys, allSales := 0, 0, 0
	for _, n := range []int{3, 4, 5, 6} {
		for sample := 0; sample < 2; sample++ {
			t.Run(fmt.Sprintf("%d/sample%d", n, sample), func(t *testing.T) {
				s, ts, clients, id := newRiversFullTable(t, n)
				restored := map[string]bool{}
				restart := func(label string) {
					t.Helper()
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored[label] = true
				}
				restart("start")
				steps, automatic, timeouts, bridges, buys, sales := 0, 0, 0, 0, 0, 0
				minBank := s.rooms[id].Game.Catan.Rivers.Bank
				for ; steps < 6000 && !s.rooms[id].Game.Finished; steps++ {
					r := s.rooms[id]
					state, g := r.Game, r.Game.Catan
					assertRiversFullInventory(t, state)
					minBank = min(minBank, g.Rivers.Bank)
					label := ""
					switch {
					case g.SetupStep == 1:
						label = "first-setup-pair"
					case g.SetupStep >= g.SetupLimit() && state.Phase == "catan_roll":
						label = "normal-turn"
					case g.Paired != nil && g.Paired.Second:
						label = "secondary"
					case g.Rivers.Bought == 1:
						label = "first-purchase"
					case g.Rivers.Bought == 2:
						label = "purchase-cap"
					}
					if label != "" && !restored[label] {
						restart(label)
						r = s.rooms[id]
						state, g = r.Game, r.Game.Catan
					}
					if steps%31 == 0 {
						for _, viewer := range []int{state.Turn, n} {
							v := current(clients[viewer])["game"].(map[string]any)["catan"].(map[string]any)
							if v["devDeck"] != nil {
								t.Fatal("hidden deck exposed")
							}
							public := v["rivers"].(map[string]any)
							for p, amount := range public["gold"].([]any) {
								if int(amount.(float64)) != g.Rivers.Gold[p] {
									t.Fatal("public coins changed")
								}
							}
							for p, raw := range v["players"].([]any) {
								seat := raw.(map[string]any)
								_, hand := seat["resources"]
								_, dev := seat["dev"]
								if hand != (p == viewer) || dev != (p == viewer) {
									t.Fatal("private cards exposed")
								}
							}
						}
					}
					actor := state.Turn
					if pending := state.CatanPendingActor(); pending >= 0 {
						actor = pending
					} else if state.Phase == "catan_discard" {
						for p, due := range g.DiscardDue {
							if due > 0 {
								actor = p
								break
							}
						}
					}
					version := r.Version
					if steps%29 == 0 && (g.SetupStep < g.SetupLimit() || state.CatanPendingActor() >= 0 || state.Phase == "catan_discard") {
						s.mu.Lock()
						r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
						s.expireSetups(time.Now())
						s.mu.Unlock()
						if s.rooms[id].Version <= version {
							t.Fatal("timeout stalled", steps, state.Phase)
						}
						timeouts++
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
						automatic++
						continue
					}
					action, err := state.BotAction(actor)
					if err != nil {
						t.Fatal(steps, state.Phase, err)
					}
					switch action.Type {
					case "catan_bridge":
						bridges++
					case "catan_coin_buy":
						buys++
					case "catan_coin_sell":
						sales++
					}
					clients[actor].command(current(clients[actor]), "action", action, 200)
				}
				r := s.rooms[id]
				assertRiversFullInventory(t, r.Game)
				if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || !restored["first-setup-pair"] || !restored["normal-turn"] || n > 4 && !restored["secondary"] || automatic == 0 || timeouts == 0 {
					t.Fatal("incomplete full game", steps, automatic, timeouts, restored)
				}
				winner := r.Game.Winners[0]
				if winner != r.Game.Turn || r.Game.Catan.Players[winner].Score < 10 {
					t.Fatal("victory outside own action or below target")
				}
				restart("finished")
				checkHistory := func() {
					t.Helper()
					for p := range n {
						code, profile := clients[n].request("GET", "/api/players/"+s.rooms[id].Seats[p].ID, nil)
						if code != 200 {
							t.Fatal("history unavailable")
						}
						wins := profile["stats"].(map[string]any)["catan"].(map[string]any)["wins"]
						want := float64(0)
						if p == winner {
							want = 1
						}
						if wins != want {
							t.Fatal("wrong history winner", p, wins)
						}
					}
				}
				checkHistory()
				allBridges += bridges
				allBuys += buys
				allSales += sales
				// Finished commands must not settle or award history a second time.
				before, _ := json.Marshal(s.rooms[id])
				clients[winner].command(map[string]any{"id": id, "version": s.rooms[id].Version}, "action", game.Action{Type: "catan_end"}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("finished action mutated room")
				}
				checkHistory()
				minBank = min(minBank, s.rooms[id].Game.Catan.Rivers.Bank)
				t.Logf("steps=%d autoplay=%d timeouts=%d manual bridges/buys/sales=%d/%d/%d minBank=%d winner=%d score=%d restarts=%v", steps, automatic, timeouts, bridges, buys, sales, minBank, winner, s.rooms[id].Game.Catan.Players[winner].Score, restored)
			})
		}
	}
	// Natural games need not use every move. Require scenario-specific actions
	// across the batch, with directed tests covering every rule and rejection.
	if allBridges == 0 || allBuys == 0 || allSales == 0 {
		t.Fatal("missing river action coverage", allBridges, allBuys, allSales)
	}
}
