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

func newTradersHelpersHTTP(t *testing.T, n int, scene string, city, fish bool, variants ...bool) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("组合助手%d", p))
	}
	body := map[string]any{"kind": "catan", "name": "商人与蛮族助手", "capacity": n, "catanScenario": scene, "catanOptions": game.CatanOptions{Helpers: true, AllHelpers: true, FiveSix: n > 4 && (scene == "rivers" || scene == "caravans")}, "catanEvents": game.CatanEventCatalogue, "catanFishing": fish}
	if n == 2 && (scene == "rivers" || scene == "caravans" || scene == "barbarian-attack") {
		body["catanScenario"] = ""
		body["catanTwoScenario"] = scene
	}
	if len(variants) > 0 && variants[0] {
		body["catanFriendlyRobber"] = game.CatanFriendlyRobberSetup{Enabled: true}
		body["catanHarbors"] = game.CatanHarborsSetup{Enabled: true}
	}
	if city {
		body["catanCitiesKnights"] = game.CatanCitiesKnightsSetup{}
	}
	r := clients[0].post("/api/rooms", body, 201)
	id := r["id"].(string)
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := 0; p < n; p++ {
		clients[p].command(current(clients[0]), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	return s, ts, clients, id
}

func TestCatanTradersHelpersPublicHTTP(t *testing.T) {
	for _, c := range []struct {
		scene      string
		n          int
		city, fish bool
	}{{"barbarian-attack", 3, false, false}, {"transport-desert", 2, false, false}, {"rivers-caravans", 2, true, false}, {"caravans-transport", 6, true, false}, {"rivers", 3, false, true}} {
		t.Run(fmt.Sprintf("%s/%d/%t/%t", c.scene, c.n, c.city, c.fish), func(t *testing.T) {
			s, _, clients, id := newTradersHelpersHTTP(t, c.n, c.scene, c.city, c.fish)
			timeout, used := false, 0
			for step := 0; step < 16000 && !s.rooms[id].Game.Finished; step++ {
				r := s.rooms[id]
				g := r.Game
				p := twoHTTPActor(g)
				if g.Catan.TradersHelpers == nil {
					t.Fatal("missing helper recipe")
				}
				if g.Catan.HelperPending != nil && !timeout {
					deadline := r.TurnDeadline
					if left := deadline - time.Now().UnixMilli(); left < 118000 || left > 120000 {
						t.Fatal("helper clock", left)
					}
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(deadline + 1))
					s.mu.Unlock()
					if !s.rooms[id].Seats[p].AutoPlay {
						t.Fatal("timeout did not take over")
					}
					setAutoPlay(clients[p], current(clients[p]), false, 200)
					timeout = true
					continue
				}
				a, err := g.BotAction(p)
				if err != nil {
					t.Fatal(step, g.Phase, err)
				}
				if a.Type == "catan_helper" || a.Skill == "helper" {
					used++
				}
				clients[p].command(current(clients[p]), "action", a, 200)
			}
			r := s.rooms[id]
			if !r.Game.Finished || !timeout || used == 0 {
				t.Fatal("incomplete helper match", r.Game.Phase, used)
			}
			_, profile := clients[c.n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
			record := profile["history"].([]any)[0].(map[string]any)
			if record["catanExpansionRules"].(map[string]any)["traders_helpers"] != game.CatanTradersHelpersRules {
				t.Fatal("missing helper history")
			}
			t.Logf("complete round %d, helper actions %d", r.Game.Round, used)
		})
	}
}

func TestCatanTradersHelpersPrivateCardRecovery(t *testing.T) {
	s, ts, clients, id := newTradersHelpersHTTP(t, 3, "barbarian-attack", false, false)
	for step := 0; s.rooms[id].Game.Phase != "catan_turn" && step < 150; step++ {
		g := s.rooms[id].Game
		p := twoHTTPActor(g)
		a, e := g.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		clients[p].command(current(clients[p]), "action", a, 200)
	}
	r := s.rooms[id]
	g := r.Game.Catan
	p := r.Game.Turn
	old := g.Players[p].Helper.ID
	if at := slices.Index(g.HelperDisplay, 6); at >= 0 {
		g.HelperDisplay[at] = old
	} else {
		for i := range g.Players {
			if g.Players[i].Helper.ID == 6 {
				g.Players[i].Helper.ID = old
			}
		}
	}
	g.Players[p].Helper = &game.CatanHelperSeat{ID: 6}
	for _, color := range []int{0, 2, 4} {
		g.Players[p].Resources[color]++
		g.Bank[color]--
	}
	r.TurnDeadline = time.Now().Add(41 * time.Second).UnixMilli()
	if e := s.save(r); e != nil {
		t.Fatal(e)
	}
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_buy_dev", Skill: "helper", Tokens: []int{1, 0, 1, 0, 1}}, 200)
	r = s.rooms[id]
	deadline, left := r.TurnDeadline, r.CatanTimeLeft
	if r.Game.Catan.HelperPending.Kind != "attack_development" || left < 39000 || left > 41000 {
		t.Fatal("candidate clock")
	}
	for viewer, c := range clients {
		v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		if (v["helperPending"].(map[string]any)["cards"] != nil) != (viewer == p) {
			t.Fatal("candidate leak")
		}
	}
	before, _ := json.Marshal(r)
	clients[(p+1)%3].command(current(clients[(p+1)%3]), "action", game.Action{Type: "catan_helper_choice", Card: 0}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("wrong actor changed card state")
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	r = s.rooms[id]
	if r.TurnDeadline != deadline || r.CatanTimeLeft != left {
		t.Fatal("restart reset helper clock")
	}
	card := r.Game.Catan.HelperPending.Cards[0]
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_helper_choice", Card: card}, 200)
	r = s.rooms[id]
	if r.Game.Catan.HelperPending.Kind != "exchange" || r.Game.Catan.Attack.Pending != nil {
		t.Fatal("card executed before helper exchange")
	}
	for _, c := range clients {
		h := current(c)["game"].(map[string]any)["catan"].(map[string]any)["tradersHelpers"].(map[string]any)
		if len(h) != 1 || h["attackCard"] != nil {
			t.Fatal("chosen secret leaked")
		}
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_helper_choice", Choice: "flip"}, 200)
	if s.rooms[id].Game.Catan.TradersHelpers.AttackCard != "" {
		t.Fatal("chosen card not resumed")
	}
	t.Log("private candidates, chosen card, two reloads and paused clock passed")
}

func TestCatanTradersHelpersRoomCatalogue(t *testing.T) {
	scenes := []string{"rivers", "caravans", "barbarian-attack", "transport", "rivers-caravans", "rivers-attack", "rivers-transport", "caravans-attack", "caravans-transport", "attack-transport", "rivers-shores", "rivers-fog", "rivers-desert", "rivers-desert-belt", "rivers-tribe", "rivers-new-world", "caravans-shores", "caravans-islands", "caravans-desert", "caravans-tribe", "caravans-new-world", "attack-shores", "attack-desert", "attack-tribe", "attack-wonders", "attack-pirates", "transport-shores", "transport-desert"}
	for _, scene := range scenes {
		for _, n := range []int{2, 6} {
			t.Run(fmt.Sprintf("%s/%d", scene, n), func(t *testing.T) {
				options, _ := game.NormalizeCatanOptions(game.CatanOptions{Helpers: true, AllHelpers: true, FiveSix: n > 4 && (scene == "rivers" || scene == "caravans")})
				r := &Room{Kind: "catan", Status: "waiting", Capacity: n, CatanOptions: options, CatanEvents: game.CatanEventCatalogue}
				if n == 2 && (scene == "rivers" || scene == "caravans" || scene == "barbarian-attack") {
					if e := r.setCatanTwoScenario(scene); e != nil {
						t.Fatal(e)
					}
				} else {
					if e := r.setCatanScenario(scene); e != nil {
						t.Fatal(e)
					}
				}
				if !r.catanTradersHelpersAvailable() || !r.CatanOptions.Helpers || !r.CatanOptions.AllHelpers {
					t.Fatal("lost helper availability or preference")
				}
				if e := r.validateCatanScenario(); e != nil {
					t.Fatal(e)
				}
				r.CatanOptions.Helpers, r.CatanOptions.AllHelpers = false, false
				r.CatanOptions, _ = game.NormalizeCatanOptions(r.CatanOptions)
				if e := r.validateCatanScenario(); e != nil {
					t.Fatal("base toggle", e)
				}
			})
		}
	}
}
