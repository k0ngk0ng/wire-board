package server

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Private snapshots use artificial [2,3,4,5,6,8] tokens, not a release recipe.
// Setup is a constructor snapshot. Discard has conserved rich hands and a
// moved victim ship. Resolve has revealed terrain and three contributed crew.
func TestCatanExplorerLairsHTTPResponseClocks(t *testing.T) {
	for _, fixture := range []string{"setup", "discard", "resolve", "chase"} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fixture+"/"+mode, func(t *testing.T) {
				s, ts, clients, id := newExplorerHTTP(t, 3, false)
				raw, err := os.ReadFile("testdata/catan_explorer_lairs_" + fixture + ".json")
				if err != nil {
					t.Fatal(err)
				}
				var state game.State
				if err = json.Unmarshal(raw, &state); err != nil {
					t.Fatal(err)
				}
				now := time.Now()
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = &state
				r.TurnDeadline = now.Add(37 * time.Second).UnixMilli()
				r.CatanPendingVersion = r.Version
				if fixture == "setup" {
					r.startTurnClock(now)
				} else {
					previous := "catan_roll"
					if fixture == "resolve" || fixture == "chase" {
						previous = "catan_explorer_move"
					}
					if !r.adjustCatanResponseClock(previous, -1, r.Game.Catan.SetupStep, now) {
						t.Fatal("response not recognized")
					}
				}
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				if mode == "autoplay" {
					for p := 0; p < 3; p++ {
						setAutoPlay(clients[p], current(clients[p]), true, 200)
					}
				}
				seen := map[string]int{}
				originalActor := state.Turn
				for step := 0; ; step++ {
					r = s.rooms[id]
					phase := r.Game.Phase
					if phase == "catan_roll" || phase == "catan_turn" || phase == "catan_explorer_move" {
						break
					}
					if step > 60 {
						t.Fatal("mandatory response stalled", fixture, mode, phase)
					}
					seen[phase]++
					assertExplorerHTTPPrivacy(t, clients)
					deadline, remaining := r.TurnDeadline, r.CatanTimeLeft
					if (fixture == "discard" || fixture == "chase") && remaining != 37000 {
						t.Fatal("original budget overwritten", remaining)
					}
					actor := explorerHTTPActor(r.Game)
					if phase != "catan_discard" && r.Game.CatanPendingActor() != actor {
						t.Fatal("mandatory actor missing")
					}
					a, err := r.Game.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					if mode == "manual" {
						// Another seat cannot answer for the active player.
						if phase != "catan_discard" {
							clients[(actor+1)%3].command(current(clients[(actor+1)%3]), "action", a, 400)
						}
						clients[actor].command(current(clients[actor]), "action", a, 200)
						now = time.Now()
					} else {
						s.mu.Lock()
						if mode == "autoplay" {
							now = time.Now()
							r.BotAt = 0
							s.runBots(now)
						} else {
							now = time.UnixMilli(max(deadline, now.UnixMilli()+1000))
							s.expireSetups(now)
						}
						s.mu.Unlock()
					}
					r = s.rooms[id]
					next := r.Game.Phase
					if (phase == "catan_discard" && next == phase) || (phase == "catan_explorer_resolve" || phase == "catan_explorer_battle") && next != "catan_roll" {
						if r.TurnDeadline != deadline {
							t.Fatal("shared response deadline reset", phase, next)
						}
					} else {
						want := int64(120000)
						if fixture == "discard" && next == "catan_turn" || fixture == "chase" && next == "catan_explorer_move" {
							want = 37000
						}
						got := r.TurnDeadline - now.UnixMilli()
						if got < want-1000 || got > want {
							t.Fatal("incorrect deadline", phase, next, got, want)
						}
					}
					// Persist and reload every response, including same-player setup steps,
					// pirate placement -> theft, and multi-player hero rolls.
					savedDeadline, savedRemaining := r.TurnDeadline, r.CatanTimeLeft
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					if s.rooms[id].TurnDeadline != savedDeadline || s.rooms[id].CatanTimeLeft != savedRemaining {
						t.Fatal("clock changed on restart")
					}
				}
				r = s.rooms[id]
				if r.CatanTimeLeft != 0 {
					t.Fatal("paused budget not cleared")
				}
				if fixture == "setup" && (seen["catan_explorer_setup"] != 12 || r.Game.Turn != originalActor || r.Game.Catan.TurnSerial != 1) {
					t.Fatal("setup incomplete", seen)
				}
				if fixture == "discard" && (seen["catan_discard"] == 0 || seen["catan_explorer_pirate_place"] != 1 || seen["catan_explorer_pirate_steal"] != 1 || r.Game.Turn != originalActor) {
					t.Fatal("seven chain incomplete", seen)
				}
				if fixture == "resolve" && (seen["catan_explorer_resolve"] != 1 || seen["catan_explorer_battle"] == 0 || r.Game.Turn == originalActor || r.Game.Catan.TurnSerial != 2) {
					t.Fatal("battle chain incomplete", seen)
				}
				if fixture == "chase" && (seen["catan_explorer_pirate_place"] != 1 || seen["catan_explorer_pirate_steal"] != 1 || r.Game.Phase != "catan_explorer_move" || r.Game.Turn != originalActor) {
					t.Fatal("chase failed to resume movement", seen)
				}
				assertExplorerHTTPPrivacy(t, clients)
				t.Log(fmt.Sprint(seen))
			})
		}
	}
}
