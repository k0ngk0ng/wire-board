package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// These initial states come from the private 2025-secondary-v1 constructor.
// They contain no granted resources/cards or forced winners. Installing them
// here does not enable candidate catalogs in the public room configuration.
func modernSplendorTable(t *testing.T, n, mask int) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for i := range clients {
		clients[i] = newClient(t, ts.URL)
		clients[i].register(fmt.Sprintf("扩展验收%d", i))
	}
	r := clients[0].post("/api/rooms", map[string]any{"name": "宝石组合", "kind": "splendor", "capacity": n}, 201)
	for _, c := range clients[1:n] {
		c.command(current(clients[0]), "join", nil, 200)
	}
	for _, c := range clients[:n] {
		c.command(current(c), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	id := r["id"].(string)
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	raw, err := os.ReadFile(fmt.Sprintf("testdata/splendor_modern_%d.json", n))
	if err != nil {
		t.Fatal(err)
	}
	var state game.State
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	g := state.Splendor
	g.Options = game.SplendorOptions{Rules: game.SplendorExpansionRules, TradingPosts: mask&1 != 0, Strongholds: mask&2 != 0, Cities: mask&4 != 0, Orient: mask&8 != 0}
	if mask == 0 {
		g.Options.Rules = ""
	}
	if mask < 4 {
		g.Catalog = ""
	}
	if !g.Options.Orient {
		g.Decks = g.Decks[:3]
		g.Market = g.Market[:3]
	}
	if !g.Options.Cities {
		g.Cities = nil
		g.Nobles = game.Nobles()[:n+1]
	}
	if !g.Options.Strongholds {
		g.Strongholds = nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	room := s.rooms[id]
	room.Game = &state
	room.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
	// Deliberately retain the base-only room draft: archive the actual match.
	if err = s.save(room); err != nil {
		t.Fatal(err)
	}
	return s, ts, clients, id
}

func assertModernComponents(t *testing.T, state *game.State, initial []int) {
	t.Helper()
	g := state.Splendor
	cards := map[int]int{}
	add := func(cs []game.Card) {
		for _, c := range cs {
			if c.ID > 0 {
				cards[c.ID]++
			}
		}
	}
	for _, row := range g.Market {
		add(row)
	}
	for _, deck := range g.Decks {
		add(deck)
	}
	add(g.ReserveChoice)
	add(g.Exiled)
	tokens := slices.Clone(g.Bank)
	for _, p := range g.Players {
		add(p.Cards)
		add(p.Reserved)
		bonus := make([]int, 5)
		points := 3 * len(p.Nobles)
		for _, c := range p.Cards {
			points += c.Points
			if c.Color >= 0 && c.Color < 5 {
				amount := 1
				if c.Orient == "double" {
					amount = 2
				} else if c.BonusCount > 0 {
					amount = c.BonusCount
				}
				bonus[c.Color] += amount
			}
		}
		if slices.Contains(p.TradingPosts, game.GemPostPrestige) {
			points += len(p.TradingPosts)
		}
		if !slices.Equal(bonus, p.Bonus) || points != p.Score {
			t.Fatal("score/bonus drift", p)
		}
		for i, count := range p.Tokens {
			if count < 0 {
				t.Fatal("negative tokens")
			}
			tokens[i] += count
		}
	}
	want := 90
	if g.Options.Orient {
		want = 120
	}
	if len(cards) != want || !slices.Equal(tokens, initial) {
		t.Fatal("component loss", len(cards), tokens, initial)
	}
	for id, count := range cards {
		if count != 1 {
			t.Fatal("duplicate card", id, count)
		}
	}
	for _, count := range g.Bank {
		if count < 0 {
			t.Fatal("negative bank")
		}
	}
}

func assertModernPrivacy(t *testing.T, clients []*testClient, state *game.State) {
	t.Helper()
	for viewer, c := range clients {
		g := current(c)["game"].(map[string]any)["splendor"].(map[string]any)
		if _, ok := g["decks"]; ok {
			t.Fatal("hidden decks disclosed")
		}
		if viewer != state.Turn {
			for _, key := range []string{"reserveChoice", "postChoices", "strongholdActions", "conquestCard", "freeCardChoices"} {
				if _, ok := g[key]; ok {
					t.Fatal("other viewer received actor choices", viewer, key)
				}
			}
		}
		for i, p := range g["players"].([]any) {
			if i != viewer && !state.Finished {
				if _, ok := p.(map[string]any)["reserved"]; ok {
					t.Fatal("private reservation disclosed", viewer, i)
				}
			}
		}
	}
}

func assertModernCityWinner(t *testing.T, state *game.State) {
	t.Helper()
	if !state.Splendor.Options.Cities {
		return
	}
	for _, winner := range state.Winners {
		p := state.Splendor.Players[winner]
		counts := [5]int{}
		for _, c := range p.Cards {
			if c.Color >= 0 && c.Color < 5 {
				counts[c.Color]++
			}
		}
		qualified := false
		for _, city := range state.Splendor.Cities {
			match := p.Score >= city.Points
			any := city.Any == 0
			for color, need := range city.Cost {
				match = match && counts[color] >= need
				if need == 0 && counts[color] >= city.Any {
					any = true
				}
			}
			qualified = qualified || match && any
		}
		if p.Eliminated || !qualified {
			t.Fatal("unqualified city winner", winner)
		}
	}
}

func TestSplendorModernHTTPCombinationGames(t *testing.T) {
	for mask := 0; mask < 16; mask++ {
		for n := 2; n <= 4; n++ {
			t.Run(fmt.Sprintf("mask%d/players%d", mask, n), func(t *testing.T) {
				s, ts, clients, id := modernSplendorTable(t, n, mask)
				initial := slices.Clone(s.rooms[id].Game.Splendor.Bank)
				seen := map[string]bool{}
				steps := 0
				for ; !s.rooms[id].Game.Finished && steps < 2500; steps++ {
					r := s.rooms[id]
					state := r.Game
					actor := state.Turn
					assertModernComponents(t, state, initial)
					if !seen[state.Phase] {
						seen[state.Phase] = true
						assertModernPrivacy(t, clients, state)
						before, _ := json.Marshal(r)
						action, err := state.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
						for _, v := range []int{(actor + 1) % n, n} {
							clients[v].command(current(clients[v]), "action", action, 400)
						}
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("rejected action mutated room")
						}
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						r = s.rooms[id]
						state = r.Game
					}
					turn, round, deadline, version := state.Turn, state.Round, r.TurnDeadline, r.Version
					at := time.Now()
					if steps%2 == 0 {
						a, err := state.BotAction(actor)
						if err != nil {
							t.Fatal(steps, state.Phase, err)
						}
						body := map[string]any{"type": "action", "action": a, "version": version, "nonce": randomID(12)}
						clients[actor].post("/api/rooms/"+id, body, 200)
						clients[actor].post("/api/rooms/"+id, body, 200)
						if s.rooms[id].Version != version+1 {
							t.Fatal("duplicate nonce applied twice")
						}
						body["nonce"] = randomID(12)
						clients[actor].post("/api/rooms/"+id, body, 409)
					} else {
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						s.mu.Lock()
						s.rooms[id].BotAt = 0
						s.runBots(at)
						s.mu.Unlock()
						if s.rooms[id].Version != version+2 {
							t.Fatal("autoplay did not perform exactly one action", steps, state.Phase)
						}
						if !s.rooms[id].Game.Finished {
							setAutoPlay(clients[actor], current(clients[actor]), false, 200)
						}
					}
					r = s.rooms[id]
					if r.Game.Finished {
						if r.TurnDeadline != 0 {
							t.Fatal("finished clock retained")
						}
					} else if r.Game.Turn == turn && r.Game.Round == round {
						if r.TurnDeadline != deadline {
							t.Fatal("effect chain reset shared clock", state.Phase, r.Game.Phase)
						}
					} else if left := r.TurnDeadline - at.UnixMilli(); left < 120000 || left > 122000 {
						t.Fatal("new turn clock", left)
					}
					if steps%37 == 0 {
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
					}
				}
				r := s.rooms[id]
				if !r.Game.Finished || len(r.Game.Winners) == 0 {
					t.Fatal("game stalled", steps, r.Game.Phase)
				}
				assertModernComponents(t, r.Game, initial)
				assertModernCityWinner(t, r.Game)
				matchID := r.MatchID
				var before string
				if err := s.db.QueryRow("SELECT snapshot FROM match_history WHERE id=?", matchID).Scan(&before); err != nil {
					t.Fatal(err)
				}
				var record MatchRecord
				if err := json.Unmarshal([]byte(before), &record); err != nil {
					t.Fatal(err)
				}
				if record.SplendorOptions != r.Game.Splendor.Options || record.Status != "finished" || !record.Rated {
					t.Fatal("wrong archived rules/status", record)
				}
				for i, p := range record.Players {
					if p.Won != slices.Contains(r.Game.Winners, i) || p.Score == nil || *p.Score != r.Game.Splendor.Players[i].Score {
						t.Fatal("wrong archived result", i)
					}
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				s.mu.Lock()
				err := s.save(s.rooms[id])
				s.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				var after string
				if err = s.db.QueryRow("SELECT snapshot FROM match_history WHERE id=?", matchID).Scan(&after); err != nil {
					t.Fatal(err)
				}
				if before != after {
					t.Fatal("repeated archive changed match")
				}
				var ledger int
				if err = s.db.QueryRow("SELECT count(*) FROM rating_ledger WHERE match_id=?", matchID).Scan(&ledger); err != nil || ledger != n {
					t.Fatal("rating ledger duplicated/lost", ledger, err)
				}
				t.Logf("%d legal actions; phases %v; winners %v", steps, seen, r.Game.Winners)
			})
		}
	}
}

func TestSplendorModernPendingTimeoutAndContinuation(t *testing.T) {
	cases := []struct {
		phase   string
		mask, n int
	}{
		{"discard", 15, 3}, {"gem_copy", 15, 3}, {"gem_free_card", 14, 3},
		{"gem_stronghold", 15, 3}, {"gem_conquest", 15, 3}, {"gem_token", 15, 3},
		{"gem_post", 13, 4}, {"gem_reserve", 15, 3},
	}
	for _, tc := range cases {
		for _, mode := range []string{"reclaim", "autoplay", "kick_rejected"} {
			t.Run(tc.phase+"/"+mode, func(t *testing.T) {
				s, ts, clients, id := modernSplendorTable(t, tc.n, tc.mask)
				initial := slices.Clone(s.rooms[id].Game.Splendor.Bank)
				for step := 0; s.rooms[id].Game.Phase != tc.phase && step < 500; step++ {
					state := s.rooms[id].Game
					if state.Finished {
						t.Fatal("natural game did not reach target phase", tc.phase)
					}
					a, err := state.BotAction(state.Turn)
					if err != nil {
						t.Fatal(err)
					}
					if tc.phase == "gem_reserve" && state.Phase == "turn" && slices.Contains(state.Splendor.Players[state.Turn].TradingPosts, game.GemPostBlindReserve) && len(state.Splendor.Players[state.Turn].Reserved) < 3 && len(state.Splendor.Decks[3]) > 0 {
						a = game.Action{Type: "reserve", Tier: 1, Choice: "orient"}
					}
					clients[state.Turn].command(current(clients[state.Turn]), "action", a, 200)
				}
				r := s.rooms[id]
				if r.Game.Phase != tc.phase {
					t.Fatal("target not reached")
				}
				actor := r.Game.Turn
				target := r.Seats[actor].ID
				other := (actor + 1) % tc.n
				assertModernComponents(t, r.Game, initial)
				assertModernPrivacy(t, clients, r.Game)
				kick(clients[other], current(clients[other]), target, 400)
				expireTurn(t, s, id)
				version, host := s.rooms[id].Version, s.rooms[id].Host
				now := time.Now()
				s.mu.Lock()
				s.expireSetups(now)
				s.runBots(now)
				s.mu.Unlock()
				r = s.rooms[id]
				if r.Version != version+1 || !r.Seats[actor].AutoPlay || !r.Seats[actor].TimeoutAutoPlay || r.Seats[actor].Left || r.Game.Splendor.Players[actor].Eliminated || r.Host != host {
					t.Fatal("timeout must take over exactly once and retain the seat")
				}
				assertModernComponents(t, r.Game, initial)
				assertModernPrivacy(t, clients, r.Game)
				before, _ := json.Marshal(r)
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("restart changed pending choice or autoplay")
				}
				switch mode {
				case "reclaim":
					setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					if s.rooms[id].Seats[actor].AutoPlay || s.rooms[id].Seats[actor].TimeoutAutoPlay {
						t.Fatal("reclaim did not restore manual control")
					}
				case "kick_rejected":
					for _, p := range []int{tc.n, actor, other} {
						kick(clients[p], current(clients[p]), target, 400)
					}
					if s.rooms[id].Seats[actor].Left || !s.rooms[id].Seats[actor].AutoPlay {
						t.Fatal("kick request removed the timeout player")
					}
				}
				assertModernComponents(t, s.rooms[id].Game, initial)
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				steps, computerSteps := 0, 0
				for ; !s.rooms[id].Game.Finished && steps < 2500; steps++ {
					state := s.rooms[id].Game
					if s.rooms[id].Seats[state.Turn].AutoPlay {
						version := s.rooms[id].Version
						botTick(s, id)
						if s.rooms[id].Version != version+1 {
							t.Fatal("persistent timeout autoplay stalled")
						}
						computerSteps++
					} else {
						action, err := state.BotAction(state.Turn)
						if err != nil {
							t.Fatal(err)
						}
						clients[state.Turn].command(current(clients[state.Turn]), "action", action, 200)
					}
					assertModernComponents(t, s.rooms[id].Game, initial)
				}
				if !s.rooms[id].Game.Finished {
					t.Fatal("continuation or persistent autoplay missing")
				}
				assertModernCityWinner(t, s.rooms[id].Game)
				t.Logf("%s resolved via %s, then %d legal actions (%d automatic) to finish", tc.phase, mode, steps, computerSteps)
			})
		}
	}
}
