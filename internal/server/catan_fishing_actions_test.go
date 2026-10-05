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

func newFishingActionTable(t *testing.T, n int, phase, kind string) (*Server, *httptest.Server, []*testClient, string, int, game.Action) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := []*testClient{}
	for p := 0; p <= n; p++ {
		c := newClient(t, ts.URL)
		c.register(fmt.Sprintf("捕鱼玩家%d", p))
		clients = append(clients, c)
	}
	opts := game.CatanOptions{FiveSix: n > 4}
	raw := clients[0].post("/api/rooms", map[string]any{"name": "捕鱼行动验证", "kind": "catan", "capacity": n, "catanOptions": opts}, 201)
	id := raw["id"].(string)
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := 0; p < n; p++ {
		clients[p].command(current(clients[0]), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	state, err := game.NewCatanFishing(n, opts)
	if kind == "progress" {
		state, err = game.NewCatanFishingCitiesKnights(n, opts)
	}
	if kind == "ship" || kind == "pirate" {
		state, err = game.NewCatanFishingSeafarers(n, opts, game.CatanSeafarersSetup{Scenario: "islands"}, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	g := state.Catan
	p := 0
	if n > 4 {
		p = 3
		g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, true
	}
	target := (p + 1) % n
	state.Turn, state.Phase = p, phase
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	g.Vertices[0].Owner, g.Vertices[0].Level = p, 1
	g.Players[p].Score = 1
	g.Vertices[10].Owner, g.Vertices[10].Level = target, 1
	g.Players[target].Score = 1
	if g.Seafarers == nil {
		g.Robber = g.Fishing.Map.Lakes[0].Tile
	}
	if g.CitiesKnights != nil {
		g.Robber = -1
	}
	for _, id := range []int{0, 1, 11, 12, 21, 22, 23} {
		at := slices.Index(g.Fishing.Tokens.DrawPile, id)
		g.Fishing.Tokens.DrawPile = slices.Delete(g.Fishing.Tokens.DrawPile, at, at+1)
		g.Fishing.Tokens.Hands[p] = append(g.Fishing.Tokens.Hands[p], id)
	}
	a := game.Action{Type: "catan_fish_" + kind, Tokens: []int{21}, Target: target, Color: 4}
	switch kind {
	case "ship", "pirate":
		for i := range g.Vertices {
			g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
		}
		v := g.Fishing.Map.Grounds[0].Vertices[1]
		g.Vertices[v].Owner, g.Vertices[v].Level = p, 1
		for _, tile := range g.Tiles {
			if tile.Resource == game.CatanSea {
				g.Seafarers.Pirate = tile.ID
				break
			}
		}
		if kind == "ship" {
			g.Seafarers.Pirate = -1
			a.Tokens = []int{11, 21}
			view := state.View(p)["catan"].(map[string]any)["fishing"].(map[string]any)["legal"].(map[string]any)
			ships := view["ships"].([]int)
			if len(ships) == 0 {
				t.Fatal("fixture has no ship")
			}
			a.Edge = ships[0]
		}
	case "steal":
		g.Bank[3]--
		g.Players[target].Resources[3]++
	case "resource":
		a.Tokens = []int{0, 21}
	case "road":
		a.Tokens = []int{11, 21}
		a.Edge = -1
		for _, e := range g.Edges {
			if e.A == 0 || e.B == 0 {
				a.Edge = e.ID
				break
			}
		}
	case "dev":
		a.Tokens = []int{0, 21, 22}
		at := slices.Index(g.DevDeck, 0)
		g.DevDeck[at], g.DevDeck[len(g.DevDeck)-1] = g.DevDeck[len(g.DevDeck)-1], g.DevDeck[at]
	case "progress":
		a.Tokens = []int{0, 21, 22}
		a.Color = 2
		deck := g.CitiesKnights.ProgressDecks[2]
		at := slices.Index(deck, 16)
		deck[at], deck[len(deck)-1] = deck[len(deck)-1], deck[at]
	case "boot":
		a.Tokens = nil
		at := slices.Index(g.Fishing.Tokens.DrawPile, 29)
		g.Fishing.Tokens.DrawPile = slices.Delete(g.Fishing.Tokens.DrawPile, at, at+1)
		g.Fishing.Tokens.BootOwner = p
	}
	s.mu.Lock()
	r := s.rooms[id]
	r.Game = state
	r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	return s, ts, clients, id, p, a
}

func TestCatanFishingPaidActionsHTTPAndPairedRestart(t *testing.T) {
	for _, variant := range []struct {
		n     int
		phase string
	}{{3, "catan_roll"}, {3, "catan_turn"}, {6, "catan_turn"}} {
		for _, kind := range []string{"robber", "steal", "resource", "road", "dev", "boot"} {
			t.Run(fmt.Sprintf("%d/%s/%s", variant.n, variant.phase, kind), func(t *testing.T) {
				s, ts, clients, id, p, action := newFishingActionTable(t, variant.n, variant.phase, kind)
				deadline := s.rooms[id].TurnDeadline
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
					t.Cleanup(func() { _ = next.Close() })
					after, _ := json.Marshal(next.rooms[id])
					if string(before) != string(after) {
						t.Fatal("paid action room lost state on restart")
					}
					nextHTTP := httptest.NewServer(next.Handler())
					t.Cleanup(nextHTTP.Close)
					for _, c := range clients {
						c.base = nextHTTP.URL
					}
					s, ts = next, nextHTTP
				}
				restart()
				before, _ := json.Marshal(s.rooms[id])
				clients[(p+1)%variant.n].command(current(clients[(p+1)%variant.n]), "action", action, 400)
				clients[variant.n].command(current(clients[variant.n]), "action", action, 400)
				invalid := action
				invalid.Tokens = []int{29}
				if kind == "boot" {
					invalid.Target = p
				}
				clients[p].command(current(clients[p]), "action", invalid, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("illegal action changed room or clock")
				}
				clients[p].command(current(clients[p]), "action", action, 200)
				r := s.rooms[id]
				g := r.Game.Catan
				if r.TurnDeadline != deadline || r.Game.Phase != variant.phase || r.Game.Turn != p || g.Fishing.LastRollID != -1 || len(g.Fishing.Tokens.Hands[p]) != 7-len(action.Tokens) {
					t.Fatal("paid action reset clock/phase or wrong payment")
				}
				switch kind {
				case "robber":
					if g.Robber != -1 {
						t.Fatal("robber not removed")
					}
				case "steal":
					if g.Players[action.Target].Resources[3] != 0 || g.Players[p].Resources[3] != 1 {
						t.Fatal("resource theft")
					}
				case "resource":
					if g.Players[p].Resources[4] != 1 {
						t.Fatal("bank resource")
					}
				case "road":
					if g.Edges[action.Edge].Owner != p || g.Players[p].Resources[0] != 0 || g.Players[p].Resources[1] != 0 {
						t.Fatal("fish road")
					}
				case "dev":
					if g.Players[p].Dev[0] != 1 || g.Players[p].NewDev[0] != 1 {
						t.Fatal("development card")
					}
				case "boot":
					if g.Fishing.Tokens.BootOwner != action.Target || g.Players[p].Score != 1 {
						t.Fatal("boot changed score")
					}
				}
				for viewer, c := range clients {
					v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					fish := v["fishing"].(map[string]any)
					tokens := fish["tokens"].(map[string]any)
					if tokens["drawPile"] != nil || fish["lastRollId"] != nil {
						t.Fatal("save data exposed")
					}
					if viewer != p && len(fish["legal"].(map[string]any)["actions"].([]any)) != 0 {
						t.Fatal("other viewer can act")
					}
					for seat, raw := range tokens["players"].([]any) {
						_, visible := raw.(map[string]any)["tokens"]
						if visible != (seat == viewer && len(g.Fishing.Tokens.Hands[seat]) > 0) {
							t.Fatal("fish face privacy")
						}
					}
					for seat, raw := range v["players"].([]any) {
						_, visible := raw.(map[string]any)["dev"]
						if visible != (seat == viewer) {
							t.Fatal("development privacy")
						}
					}
				}
				restart()
			})
		}
	}
}
