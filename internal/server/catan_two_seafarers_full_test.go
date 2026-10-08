package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanTwoSeafarersCompleteHTTPGames(t *testing.T) {
	runTwoSeafarersCompleteHTTPGames(t, false)
}

func TestCatanTwoFishingSeafarersCompleteHTTPGames(t *testing.T) {
	runTwoSeafarersCompleteHTTPGames(t, true)
}

func TestCatanTwoSeafarersKnightsCompleteHTTPGames(t *testing.T) {
	for _, fishing := range []bool{false, true} {
		t.Run(fmt.Sprint("fishing", fishing), func(t *testing.T) { runTwoSeafarersCompleteHTTPGames(t, fishing, true) })
	}
}
func runTwoSeafarersCompleteHTTPGames(t *testing.T, fishing bool, knightOption ...bool) {
	knights := len(knightOption) > 0 && knightOption[0]
	for i, scenario := range []string{"shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		t.Run(scenario, func(t *testing.T) {
			helpers, events := (!knights && i%2 == 0) || len(knightOption) > 1 && knightOption[1], i%2 == 1
			s, ts, clients, id := newTwoCombinationFullTable(t, scenario, events, true, true, fishing, knights, game.CatanOptions{Helpers: helpers, AllHelpers: helpers})
			restored := map[string]bool{}
			steps, automatic, timeouts := 0, 0, 0
			for ; steps < 7500 && !s.rooms[id].Game.Finished; steps++ {
				state := s.rooms[id].Game
				label := fmt.Sprintf("%s/%d", state.Phase, len(state.Catan.Two.Rolls))
				if state.Catan.Two.Pending != nil {
					label += "/" + state.Catan.Two.Pending.Kind
				}
				if state.Catan.HelperPending != nil {
					label += "/" + state.Catan.HelperPending.Kind
				}
				if !restored[label] {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored[label] = true
					state = s.rooms[id].Game
				}
				assertTwoHTTPInventory(t, state)
				assertTwoVariantsHTTP(t, state, true, true)
				assertPublicFishSeaSpecialInventory(t, state.Catan)
				actor := twoHTTPActor(state)
				if steps%41 == 0 {
					assertTwoHTTPPrivacy(t, clients, state)
				}
				action, err := state.BotAction(actor)
				if err != nil {
					t.Fatal(steps, state.Phase, err)
				}
				if steps%31 == 0 {
					version := s.rooms[id].Version
					s.mu.Lock()
					s.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
					s.expireSetups(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= version || !s.rooms[id].Seats[actor].AutoPlay {
						t.Fatal("timeout did not persist autoplay", state.Phase)
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					if !s.rooms[id].Seats[actor].AutoPlay {
						t.Fatal("restart lost autoplay")
					}
					reclaimTimeoutHumans(t, s, clients, id)
					timeouts++
				} else if steps%19 == 0 {
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					version := s.rooms[id].Version
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= version {
						t.Fatal("autoplay stalled")
					}
					if !s.rooms[id].Game.Finished {
						setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					}
					automatic++
				} else {
					old := current(clients[actor])
					clients[actor].command(old, "action", action, 200)
					if steps%53 == 0 && !s.rooms[id].Game.Finished {
						before, _ := json.Marshal(s.rooms[id])
						clients[actor].command(old, "action", action, 409)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("stale replay mutation")
						}
					}
				}
			}
			state := s.rooms[id].Game
			if !state.Finished || s.rooms[id].Status != "finished" || len(state.Winners) == 0 || automatic == 0 || timeouts == 0 {
				t.Fatal("incomplete game", steps, automatic, timeouts)
			}
			assertTwoHTTPInventory(t, state)
			assertPublicSeaVictory(t, state, state.Winners[0])
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			for p := range 2 {
				code, profile := clients[2].request("GET", "/api/players/"+s.rooms[id].Seats[p].ID, nil)
				if code != 200 {
					t.Fatal("missing history")
				}
				history := profile["history"].([]any)
				if len(history) != 1 {
					t.Fatal("duplicate history")
				}
				record := history[0].(map[string]any)
				rules := record["catanExpansionRules"].(map[string]any)
				if fishing && rules["two_fishing_seafarers"] != game.CatanTwoFishingSeafarersRules {
					t.Fatal("missing fishing sea history", rules)
				}
				if knights && (rules["two_seafarers_knights"] != game.CatanTwoSeafarersKnightsRules || rules["two_knights"] != game.CatanTwoKnightsRules) {
					t.Fatal("missing sea knights history", rules)
				}
				if helpers && knights && (rules["helpers_knights"] != game.CatanHelpersKnightsRules || rules["two_helpers"] != game.CatanTwoHelpersRules) {
					t.Fatal("missing knight helper history", rules)
				}
				if rules["two_seafarers"] != game.CatanTwoSeafarersRules || rules["two_player"] != game.CatanTwoRules || len(record["players"].([]any)) != 2 {
					t.Fatal("incorrect sea history", record)
				}
			}
			t.Logf("steps=%d autoplay=%d timeout=%d distinct restores=%d rounds=%d", steps, automatic, timeouts, len(restored), state.Round)
		})
	}
}
