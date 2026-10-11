package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Opt-in browser fixture for the already-tested controlled Orient chain.
// Failure cases: wrong interactive choice, premature refill, lost continuation
// after reload, clipped confirmation controls, and missing acquisition flights.
// The catalog/inventory fixture is explicit; this is not a natural-game claim.
func TestSplendorOrientBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_ORIENT_BROWSER") != "1" {
		t.Skip("manual browser acceptance fixture")
	}
	s, ts, clients, id := orientChainTable(t, os.Getenv("WIRE_BOARD_ORIENT_CITIES") == "1")
	// Additional failures: blind candidate disclosure, changed choices after
	// reload, returning the wrong card to the deck, and charging physical gold
	// instead of the selected Orient gold card.
	mode := os.Getenv("WIRE_BOARD_ORIENT_MODE")
	if mode == "" {
		mode = "chain"
	}
	if !slices.Contains([]string{"chain", "gold", "reserve", "double_gold", "extra_token", "posts", "cities"}, mode) {
		t.Fatal("unknown browser fixture mode", mode)
	}
	finishCard := 0
	if mode == "cities" {
		finishCard = prepareSplendorCityBrowser(t, s.rooms[id])
	}
	if mode == "gold" {
		orientChainGold(s.rooms[id].Game.Splendor)
	}
	g := s.rooms[id].Game.Splendor
	player := &g.Players[0]
	if mode == "double_gold" {
		player.TradingPosts = append(player.TradingPosts, game.GemPostDoubleGold)
		player.Tokens[5]++
		g.Bank[5]--
	}
	if mode == "extra_token" {
		player.TradingPosts = append(player.TradingPosts, game.GemPostExtraToken)
	}
	if mode == "posts" {
		// Real zero-point cards establish multiple post choices without changing
		// score or supply. Copy/double bonuses still count as one physical card.
		for color, count := range map[int]int{0: 4, 1: 2} {
			for range count {
				found := false
				for row, deck := range g.Decks {
					for i, card := range deck {
						if card.Orient == "" && card.Color == color && card.Points == 0 {
							player.Cards = append(player.Cards, card)
							player.Bonus[color]++
							g.Decks[row] = append(deck[:i], deck[i+1:]...)
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				if !found {
					t.Fatal("missing zero-point fixture card", color)
				}
			}
		}
	}
	if mode != "chain" && mode != "reserve" {
		s.rooms[id].Version++
		if err := s.save(s.rooms[id]); err != nil {
			t.Fatal(err)
		}
	}
	var reserveChoices []game.Card
	reserveCount := 0
	if mode == "reserve" {
		reserveChoices = slices.Clone(g.Decks[3][:2])
		reserveCount = len(g.Decks[3])
	}
	playerNames := []string{}
	for _, seat := range s.rooms[id].Seats {
		playerNames = append(playerNames, seat.Name)
	}
	observerNames := []string{s.rooms[id].Seats[1].Name, clients[3].state()["user"].(map[string]any)["name"].(string)}
	s.files = os.DirFS(filepath.Join("..", "..", "web", "dist"))
	s.cfg.AssetsBaseURL = os.Getenv("ASSETS_BASE_URL")
	ts.Close()
	ts = httptest.NewServer(s.Handler())
	defer ts.Close()
	for _, c := range clients {
		c.base = ts.URL
	}
	dir := filepath.Join("..", "..", ".local")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(dir, "orient-browser.json")
	stop := filepath.Join(dir, "orient-browser-stop")
	_ = os.Remove(stop)
	raw, err := json.Marshal(map[string]any{"url": ts.URL, "room": id, "name": s.rooms[id].Seats[0].Name, "password": "test-password-123", "mode": mode, "observers": observerNames, "players": playerNames, "finishCard": finishCard})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(ready, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(ready); _ = os.Remove(stop) })
	t.Log("fixture ready; finish by creating", stop)
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(stop); err == nil {
			var result struct {
				Passed   bool   `json:"passed"`
				Artifact string `json:"artifact"`
			}
			if err := json.Unmarshal(raw, &result); err != nil || !result.Passed {
				t.Fatalf("browser acceptance incomplete; artifact=%s error=%v", result.Artifact, err)
			}
			t.Log("browser acceptance passed", result.Artifact)
			g := s.rooms[id].Game.Splendor
			assertModernComponents(t, s.rooms[id].Game, []int{5, 5, 5, 5, 5, 5})
			assertModernPrivacy(t, clients, s.rooms[id].Game)
			if mode == "cities" {
				assertModernCityWinner(t, s.rooms[id].Game)
				if !s.rooms[id].Game.Finished || !slices.Equal(s.rooms[id].Game.Winners, []int{0}) {
					t.Fatal("city fixture did not settle correctly")
				}
			}
			if mode == "reserve" && (len(g.Decks[3]) != reserveCount-1 || g.Decks[3][len(g.Decks[3])-1].ID != reserveChoices[0].ID || g.Players[0].Reserved[0].ID != reserveChoices[1].ID) {
				t.Fatal("browser reservation changed deck order")
			}
			if mode == "gold" && (len(g.Exiled) != 1 || g.Exiled[0].ID != 1001 || g.Bank[5] != 5) {
				t.Fatal("browser gold payment changed card or token inventory")
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("browser acceptance fixture timed out")
}

// Failure cases: premature qualification, ending before the other players'
// turns, or losing city eligibility on reload. Use actual cards and printed
// city goals, with one real one-point purchase left before qualification.
func prepareSplendorCityBrowser(t *testing.T, room *Room) int {
	t.Helper()
	st, err := game.NewSplendor(3, game.SplendorOptions{Cities: true})
	if err != nil {
		t.Fatal(err)
	}
	g := st.Splendor
	pool := map[int]game.Card{}
	for _, rows := range [][][]game.Card{g.Market, g.Decks} {
		for _, row := range rows {
			for _, c := range row {
				pool[c.ID] = c
			}
		}
	}
	ids := []int{}
	for id := range pool {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var finish game.Card
	for _, id := range ids {
		c := pool[id]
		if c.Tier == 1 && c.Points == 1 {
			finish = c
			delete(pool, id)
			break
		}
	}
	if finish.ID == 0 {
		t.Fatal("missing one-point city finish card")
	}
	minimum, count := 100, 0
	for _, city := range g.Cities {
		minimum = min(minimum, city.Points)
		count = max(count, city.Any)
		for _, n := range city.Cost {
			count = max(count, n)
		}
	}
	p := &g.Players[0]
	take := func(c game.Card) {
		delete(pool, c.ID)
		p.Cards = append(p.Cards, c)
		p.Bonus[c.Color]++
		p.Score += c.Points
	}
	for color := range 5 {
		left := count
		for _, id := range ids {
			c, ok := pool[id]
			if ok && c.Color == color && c.Points == 0 && left > 0 {
				take(c)
				left--
			}
		}
		if left != 0 {
			t.Fatal("missing city requirement cards", color, left)
		}
	}
	for p.Score < minimum-1 {
		var best game.Card
		for _, id := range ids {
			c, ok := pool[id]
			if ok && c.Points > best.Points && c.Points <= minimum-1-p.Score {
				best = c
			}
		}
		if best.ID == 0 {
			t.Fatal("cannot construct exact pre-city score")
		}
		take(best)
	}
	g.Market = make([][]game.Card, 3)
	g.Decks = make([][]game.Card, 3)
	g.Market[0] = append(g.Market[0], finish)
	for _, id := range ids {
		if c, ok := pool[id]; ok {
			row := c.Tier - 1
			if len(g.Market[row]) < 4 {
				g.Market[row] = append(g.Market[row], c)
			} else {
				g.Decks[row] = append(g.Decks[row], c)
			}
		}
	}
	st.Turn, g.StartPlayer = 0, 0
	for color, cost := range finish.Cost {
		need := max(0, cost-p.Bonus[color])
		p.Tokens[color] += need
		g.Bank[color] -= need
	}
	room.Game, room.SplendorOptions = st, g.Options
	room.startTurnClock(time.Now())
	return finish.ID
}
