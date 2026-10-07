package server

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Default tables and tables with every expansion switched back off must both
// start the original base game after an actual server restart.
func TestBaseGamesRemainBaseAfterExpansionSettings(t *testing.T) {
	for _, kind := range []string{"splendor", "catan"} {
		for _, toggle := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/toggle=%v", kind, toggle), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				h := newClient(t, ts.URL)
				h.register("基础版兼容验收")
				raw := h.post("/api/rooms", map[string]any{"kind": kind, "capacity": 4, "name": "原本的基础版"}, 201)
				id := raw["id"].(string)
				command := func(name string, values map[string]any) {
					values["type"], values["version"], values["nonce"] = name, s.rooms[id].Version, randomID(12)
					h.post("/api/rooms/"+id, values, 200)
				}
				if toggle {
					if kind == "splendor" {
						command("splendor_options", map[string]any{"splendorOptions": game.SplendorOptions{Orient: true, Cities: true, Strongholds: true, TradingPosts: true}})
						command("splendor_options", map[string]any{"splendorOptions": game.SplendorOptions{}})
					} else {
						selectCatanHarbors(h, true, 200)
						selectCatanFriendlyRobber(h, true, 200)
						command("catan_options", map[string]any{"catanOptions": game.CatanOptions{FiveSix: true}})
						selectCatanBase(h, &game.CatanBaseConfiguration{Layout: "fixed"}, 200)
						command("catan_options", map[string]any{"catanOptions": game.CatanOptions{}})
						selectCatanHarbors(h, false, 200)
						selectCatanFriendlyRobber(h, false, 200)
					}
				}
				if _, added := current(h)["catanBaseLayouts"]; added {
					t.Fatal("ordinary base room gained a layout picker")
				}
				for range 3 {
					h.command(current(h), "add_bot", nil, 200)
				}
				h.command(current(h), "ready", nil, 200)
				s, ts = restartRiversHTTP(t, s, ts, []*testClient{h}, id)
				h.command(current(h), "start", nil, 200)
				state := s.rooms[id].Game
				if kind == "splendor" {
					g := state.Splendor
					if g.Options != (game.SplendorOptions{}) || g.Catalog != "" || len(g.Cities) != 0 || g.Strongholds != nil || len(g.Effects) != 0 || len(g.Nobles) != 5 || len(g.Market) != 3 || len(g.Decks) != 3 || !reflect.DeepEqual(g.Bank, []int{7, 7, 7, 7, 7, 5}) {
						t.Fatal("base Splendor gained expansion components")
					}
					want := make(map[int]game.Card)
					for _, c := range game.Cards() {
						want[c.ID] = c
					}
					for tier, cards := range g.Market {
						if len(cards) != 4 {
							t.Fatal("base market is no longer four cards per tier")
						}
						for _, c := range append(append([]game.Card{}, cards...), g.Decks[tier]...) {
							if !reflect.DeepEqual(want[c.ID], c) {
								t.Fatal("base card cost, score or bonus changed", c.ID)
							}
							delete(want, c.ID)
						}
					}
					if len(want) != 0 {
						t.Fatal("base cards missing")
					}
					for _, noble := range g.Nobles {
						if noble.ID >= 100 {
							t.Fatal("extra noble leaked into base")
						}
					}
				} else {
					g := state.Catan
					if g.Options != (game.CatanOptions{}) || g.Harbors != nil || g.FriendlyRobber != nil || g.Paired != nil || g.CitiesKnights != nil || g.Seafarers != nil || g.Explorer != nil || g.EventDeck != nil || g.Rivers != nil || g.Caravans != nil || g.Two != nil || g.Fishing != nil || g.Transport != nil || g.Attack != nil || len(g.HelperDisplay) != 0 {
						t.Fatal("base Catan retained expansion rules")
					}
					if state.Phase != "catan_setup_settlement" || g.SetupStep != 0 || len(g.Tiles) != 19 || len(g.Ports) != 9 || len(g.DevDeck) != 25 || !reflect.DeepEqual(g.Bank, []int{19, 19, 19, 19, 19}) {
						t.Fatal("base Catan opening or inventory changed")
					}
					counts := [5]int{}
					for _, card := range g.DevDeck {
						counts[card]++
					}
					if counts != [5]int{14, 2, 2, 2, 5} {
						t.Fatal("base development deck changed", counts)
					}
					for _, p := range g.Players {
						if p.Score != 0 || p.Helper != nil {
							t.Fatal("base Catan skipped original setup")
						}
					}
				}
			})
		}
	}
}
