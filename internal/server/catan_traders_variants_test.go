package server

import (
	"fmt"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanTradersVariantsPublicHTTP(t *testing.T) {
	for _, c := range []struct {
		scene      string
		n          int
		city, fish bool
	}{{"barbarian-attack", 3, false, false}, {"transport-desert", 2, false, false}, {"rivers-caravans", 2, true, false}, {"caravans-transport", 6, true, false}, {"rivers", 3, true, true}, {"attack-wonders", 2, false, false}, {"attack-pirates", 2, false, false}} {
		t.Run(fmt.Sprintf("%s/%d/%t/%t", c.scene, c.n, c.city, c.fish), func(t *testing.T) {
			s, ts, clients, id := newTradersHelpersHTTP(t, c.n, c.scene, c.city, c.fish, true)
			reloaded := false
			for step := 0; step < 16000 && !s.rooms[id].Game.Finished; step++ {
				g := s.rooms[id].Game
				p := twoHTTPActor(g)
				if g.Catan.TradersVariants != game.CatanTradersVariantsRules || g.Catan.FriendlyRobber == nil || g.Catan.Harbors == nil {
					t.Fatal("missing variants")
				}
				if !reloaded && step == 83 {
					deadline := s.rooms[id].TurnDeadline
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					if s.rooms[id].TurnDeadline != deadline {
						t.Fatal("restart changed clock")
					}
					reloaded = true
					continue
				}
				a, e := g.BotAction(p)
				if e != nil {
					t.Fatal(step, g.Phase, e)
				}
				clients[p].command(current(clients[p]), "action", a, 200)
			}
			r := s.rooms[id]
			if !r.Game.Finished || !reloaded {
				t.Fatal("incomplete variants match", r.Game.Round, r.Game.Phase)
			}
			_, profile := clients[c.n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
			record := profile["history"].([]any)[0].(map[string]any)
			rules := record["catanExpansionRules"].(map[string]any)
			if rules["traders_variants"] != game.CatanTradersVariantsRules || rules["harbors"] != game.CatanHarborsRules || rules["friendly_robber"] != game.CatanFriendlyRobberRules {
				t.Fatal("variant history lost")
			}
			t.Logf("complete round %d, winner score %d", r.Game.Round, r.Game.Catan.Players[r.Game.Winners[0]].Score)
		})
	}
}

func TestCatanTradersVariantsRoomCatalogue(t *testing.T) {
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
				if !r.publicCatanFriendlyAvailable() || !r.publicCatanHarborsAvailable() {
					t.Fatal("variants unavailable")
				}
				if e := r.setCatanFriendlyRobber(game.CatanFriendlyRobberSetup{Enabled: true}); e != nil {
					t.Fatal(e)
				}
				if e := r.setCatanHarbors(game.CatanHarborsSetup{Enabled: true}); e != nil {
					t.Fatal(e)
				}
				if e := r.validateCatanScenario(); e != nil {
					t.Fatal(e)
				}
				if e := r.setCatanFriendlyRobber(game.CatanFriendlyRobberSetup{Enabled: false}); e != nil {
					t.Fatal(e)
				}
				if e := r.setCatanHarbors(game.CatanHarborsSetup{Enabled: false}); e != nil {
					t.Fatal(e)
				}
				if e := r.validateCatanScenario(); e != nil {
					t.Fatal("disabled variants broke base", e)
				}
			})
		}
	}
}
