package monopoly_test

// Failure scenarios before autonomous investment/trade policy: whole games
// never finish without houses; permanent split color sets; repeated proposals
// monopolize a turn; bots overspend or read hidden deck/roll state; liquidation,
// inheritance, a card chain or a declined auction stalls an otherwise live game.
// Full games use fixed legal initial snapshots and the real policy/HTTP engine.
// Test accounts submit policy actions directly for speed; this is not a timing
// benchmark of the platform's deliberately paced bot scheduler.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestMonopolyBotInvestmentHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) { group(s, 0, []int{1, 3}, []int{0, 0}) })
	tb.people[0].command("autoplay", map[string]any{"enabled": true}, 200)
	for until := time.Now().Add(5 * time.Second); time.Now().Before(until); {
		if level(tb.people[1].room(), 1) > 0 {
			tb.record("bot invested")
			return
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatal("bot never built on its complete color set")
}

func TestMonopolyBotCompletesSetHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Properties[6].Owner = 0
		s.Properties[8].Owner = 0
		s.Properties[9].Owner = 1
	})
	a, b := tb.people[0], tb.people[1]
	a.command("autoplay", map[string]any{"enabled": true}, 200)
	for until := time.Now().Add(5 * time.Second); time.Now().Before(until); {
		if len(offers(b)) > 0 {
			break
		}
		time.Sleep(40 * time.Millisecond)
	}
	if len(offers(b)) == 0 {
		t.Fatal("bot never proposed a completing trade")
	}
	b.act(map[string]any{"type": "trade_accept", "tradeId": lastOffer(b)}, 200)
	for until := time.Now().Add(5 * time.Second); time.Now().Before(until); {
		if level(b.room(), 6) > 0 || level(b.room(), 8) > 0 || level(b.room(), 9) > 0 {
			tb.record("bot completed and built its color set")
			return
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatal("completed group never developed")
}

func TestMonopolyFullGamesHTTP(t *testing.T) {
	for _, n := range []int{2, 4, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			var mu sync.Mutex
			var latest, initial gameapi.Match
			capture := func(m gameapi.Match, err error) (gameapi.Match, error) {
				if err == nil {
					mu.Lock()
					latest = m
					mu.Unlock()
				}
				return m, err
			}
			tb := newTableWithModule(t, n, func(s *monopoly.State) {
				s.Phase = "roll"
				s.Log = []string{"完整局验收：标准初始资金和位置，固定服务器随机源"}
				s.Random = monopoly.Random{}
				s.Random.Key[31] = byte(17 + n)
				rng := rand.New(rand.NewSource(int64(400 + n)))
				for d := range s.Decks {
					for i := range s.Decks[d] {
						s.Decks[d][i] = i
					}
					rng.Shuffle(16, func(i, j int) { s.Decks[d][i], s.Decks[d][j] = s.Decks[d][j], s.Decks[d][i] })
				}
			}, func(m *gameapi.Module) {
				start, apply, timer := m.Start, m.Apply, m.OnTimer
				m.Start = func(c gameapi.Context, r json.RawMessage) (gameapi.Match, error) {
					v, e := start(c, r)
					initial = v
					return capture(v, e)
				}
				m.Apply = func(c gameapi.Context, v gameapi.Match, r json.RawMessage) (gameapi.Match, error) {
					next, e := apply(c, v, r)
					return capture(next, e)
				}
				m.OnTimer = func(c gameapi.Context, v gameapi.Match) (gameapi.Match, error) {
					next, e := timer(c, v)
					return capture(next, e)
				}
			})
			policy := monopoly.Module().BotAction
			actions := []any{}
			seen := map[string]int{}
			complete := false
			t.Cleanup(func() {
				out := filepath.Join("..", "..", "..", ".local", "monopoly", "acceptance", "full-games")
				if err := os.MkdirAll(out, 0755); err != nil {
					t.Error(err)
					return
				}
				raw, err := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "players": n, "scope": "full HTTP game from fixed standard initial state; test accounts drive the real bot policy", "initial": initial, "actions": actions, "actionCounts": seen}, "", "  ")
				if err != nil {
					t.Error(err)
					return
				}
				if err = os.WriteFile(filepath.Join(out, fmt.Sprintf("players-%d.json", n)), raw, 0600); err != nil {
					t.Error(err)
				}
			})
			for step := 0; step < 15000; step++ {
				mu.Lock()
				match := latest
				mu.Unlock()
				if match.Result != nil {
					complete = true
					break
				}
				if len(match.Actors) == 0 {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				actor := match.Actors[0]
				c := tb.people[actor]
				room := c.room()
				ctx := gameapi.Context{Actor: actor, Players: n, Now: time.Now()}
				raw, err := policy(ctx, match)
				if err != nil {
					t.Fatal("bot policy failed", step, err)
				}
				if step%100 == 0 {
					var hidden monopoly.State
					if err = json.Unmarshal(match.Data, &hidden); err != nil {
						t.Fatal(err)
					}
					hidden.Random.Key[0] ^= 255
					for d := range hidden.Decks {
						for i, j := 0, len(hidden.Decks[d])-1; i < j; i, j = i+1, j-1 {
							hidden.Decks[d][i], hidden.Decks[d][j] = hidden.Decks[d][j], hidden.Decks[d][i]
						}
					}
					altered := match
					altered.Data, _ = json.Marshal(hidden)
					other, err := policy(ctx, altered)
					if err != nil || !bytes.Equal(raw, other) {
						t.Fatal("policy depends on hidden future randomness", step, err)
					}
				}
				var action map[string]any
				if err = json.Unmarshal(raw, &action); err != nil {
					t.Fatal(err)
				}
				c.serial++
				req := map[string]any{"type": "action", "version": room["version"], "nonce": fmt.Sprintf("e2e-%012d", c.serial), "action": action}
				// Real timers can win the race with a proposal. Retry only when a newer
				// committed version proves that the old decision window changed.
				code, response := c.requestStatus("POST", "/api/rooms/"+room["id"].(string), req)
				if code != 200 {
					if newer := c.room(); (code == 409 || code == 400) && newer["version"] != room["version"] {
						continue
					}
					t.Fatal("bot action rejected", step, actor, string(raw), code, response)
				}
				seen[action["type"].(string)]++
				actions = append(actions, map[string]any{"actor": actor, "action": action, "version": room["version"]})
				if step%250 == 0 {
					tb.record(fmt.Sprintf("full-game step %d", step))
				}
				if step == 300 {
					tb.restart()
				}
			}
			if !complete {
				tb.record("stalled game")
				t.Fatal("full game did not reach a natural winner", seen)
			}
			if seen["build"] == 0 || seen["bankrupt"] != n-1 {
				t.Fatal("game omitted construction or natural elimination", seen)
			}
			tb.restart()
			r := tb.people[1].room()
			if r["status"] != "finished" || len(r["result"].(map[string]any)["players"].([]any)) != n {
				t.Fatal("full game result/history lost")
			}
			tb.record("complete full game, restored result")
		})
	}
}
