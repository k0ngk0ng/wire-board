package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func fishingWondersVictoryState(t *testing.T, n int, layout, mode string, secondary bool) *game.State {
	t.Helper()
	s, err := game.NewCatanFishingSeafarers(n, game.CatanOptions{FiveSix: n > 4}, game.CatanSeafarersSetup{Scenario: "wonders", Layout: layout}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	s.Phase, s.Turn = "catan_turn", 0
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	if n > 4 {
		g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, false
		if secondary {
			g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = n-3, 0, true
		}
	}
	// Four separated cities plus one VP card give nine points. This is an
	// explicit midgame accounting fixture, not a full natural game transcript.
	cities := 0
	for _, v := range g.Vertices {
		land, adjacent := false, false
		for _, tile := range g.Tiles {
			if tile.Resource < 5 && slices.Contains(tile.Vertices, v.ID) {
				land = true
			}
		}
		for _, e := range g.Edges {
			if e.A == v.ID && g.Vertices[e.B].Level > 0 || e.B == v.ID && g.Vertices[e.A].Level > 0 {
				adjacent = true
			}
		}
		if !land || adjacent {
			continue
		}
		g.Vertices[v.ID].Owner, g.Vertices[v.ID].Level = 0, 2
		cities++
		if cities == 4 {
			break
		}
	}
	if cities != 4 {
		t.Fatal("four-city fixture")
	}
	at := slices.Index(g.DevDeck, 4)
	g.DevDeck = slices.Delete(g.DevDeck, at, at+1)
	g.Players[0].Dev[4], g.Players[0].Score = 1, 9
	// The next two purchases are real VP cards removed from the same deck.
	for range 2 {
		at = slices.Index(g.DevDeck, 4)
		g.DevDeck = slices.Delete(g.DevDeck, at, at+1)
	}
	g.DevDeck = append(g.DevDeck, 4, 4)
	w := g.Seafarers.Wonders
	w.Cards[0].Owner, w.Cards[0].Level = 0, 2
	w.Cards[1].Owner, w.Cards[1].Level = 1, 1
	for _, token := range []int{0, 1, 21, 22, 23, 24} {
		at := slices.Index(g.Fishing.Tokens.DrawPile, token)
		g.Fishing.Tokens.DrawPile = slices.Delete(g.Fishing.Tokens.DrawPile, at, at+1)
		g.Fishing.Tokens.Hands[0] = append(g.Fishing.Tokens.Hands[0], token)
	}
	if mode != "plain-vp" {
		at := slices.Index(g.Fishing.Tokens.DrawPile, 29)
		g.Fishing.Tokens.DrawPile = slices.Delete(g.Fishing.Tokens.DrawPile, at, at+1)
		g.Fishing.Tokens.BootOwner = 0
	}
	if mode == "boot-four" {
		w.Cards[0].Level = 3
		for color, count := range []int{0, 1, 0, 1, 3} {
			g.Players[0].Resources[color] += count
			g.Bank[color] -= count
		}
	}
	return s
}

func TestCatanFishingWondersVictoryHTTPAndRestart(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		layouts := []string{"fixed", "variable"}
		if n > 4 {
			layouts = []string{"fixed"}
		}
		for _, layout := range layouts {
			for _, secondary := range []bool{false, true} {
				if secondary && n < 5 {
					continue
				}
				for _, mode := range []string{"plain-vp", "boot-vp", "boot-four"} {
					t.Run(fmt.Sprintf("%d/%s/%s/secondary%v", n, layout, mode, secondary), func(t *testing.T) { testFishingWondersVictoryHTTP(t, n, layout, mode, secondary) })
				}
			}
		}
	}
}
func testFishingWondersVictoryHTTP(t *testing.T, n int, layout, mode string, secondary bool) {
	s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
	state := fishingWondersVictoryState(t, n, layout, mode, secondary)
	s.mu.Lock()
	r := s.rooms[id]
	r.Game = state
	deadline := time.Now().Add(45 * time.Second).UnixMilli()
	r.TurnDeadline = deadline
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	restart := func() {
		t.Helper()
		before, _ := json.Marshal(s.rooms[id])
		ts.Close()
		s.Close()
		next, err := New(s.cfg, s.files)
		if err != nil {
			t.Fatal(err)
		}
		stopBotTicker(next)
		t.Cleanup(func() { next.Close() })
		ts = httptest.NewServer(next.Handler())
		t.Cleanup(ts.Close)
		for _, c := range clients {
			c.base = ts.URL
		}
		s = next
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("restart changed wonder/boot/payment/result")
		}
	}
	restart()
	a := game.Action{Type: "catan_fish_dev", Tokens: []int{0, 21, 22}}
	if mode == "boot-four" {
		a = game.Action{Type: "catan_wonder_build", Card: 0}
	}
	for viewer, c := range clients {
		v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		f := v["fishing"].(map[string]any)
		target := f["victoryTargets"].([]any)[0].(float64)
		want := float64(10)
		if mode != "plain-vp" {
			want++
		}
		if target != want || f["tokens"].(map[string]any)["drawPile"] != nil {
			t.Fatal("boot target/private pile")
		}
		for p, raw := range f["tokens"].(map[string]any)["players"].([]any) {
			_, shown := raw.(map[string]any)["tokens"]
			if shown != (p == 0 && viewer == 0) {
				t.Fatal("fish faces leaked")
			}
		}
	}
	before, _ := json.Marshal(s.rooms[id])
	clients[1].command(current(clients[1]), "action", a, 400)
	clients[n].command(current(clients[n]), "action", a, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("wrong actor mutated room")
	}
	clients[0].command(current(clients[0]), "action", a, 200)
	if mode == "boot-vp" {
		r = s.rooms[id]
		if r.Game.Finished || r.Status != "playing" || r.Game.Catan.Players[0].Score != 10 || r.TurnDeadline != deadline {
			t.Fatal("boot wrongly won at ten or reset clock")
		}
		restart()
		clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_fish_dev", Tokens: []int{1, 23, 24}}, 200)
	}
	r = s.rooms[id]
	if !r.Game.Finished || r.Status != "finished" || !slices.Equal(r.Game.Winners, []int{0}) {
		t.Fatal("wonder victory did not finish room")
	}
	wantScore, wantDiscard := 10, 3
	if mode == "boot-vp" {
		wantScore, wantDiscard = 11, 6
	} else if mode == "boot-four" {
		wantScore, wantDiscard = 9, 0
		if r.Game.Catan.Seafarers.Wonders.Cards[0].Level != 4 {
			t.Fatal("fourth level missing")
		}
	}
	if r.Game.Catan.Players[0].Score != wantScore || len(r.Game.Catan.Fishing.Tokens.Discard) != wantDiscard {
		t.Fatal("wrong winning score/payment")
	}
	restart()
}
