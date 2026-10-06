package server

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanAttackEndHTTPClockPrivacyRestartAutoplayTimeout(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
				state := attackHTTPFixture(t, n)
				actor := state.Turn
				// Legally recruit through the special card effect, preserving physical
				// inventory; do not inject knight structs or bypass the response API.
				prepareAttackHTTPCard(t, state, "knighthood")
				if err := state.Apply(actor, game.Action{Type: "catan_buy_dev"}); err != nil {
					t.Fatal(err)
				}
				state.AutoCatanPending()
				if state.Phase != "catan_turn" || len(state.Catan.Attack.Knights) != 1 {
					t.Fatal("recruit fixture failed")
				}
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_end"}, 200)
				r = s.rooms[id]
				q := r.Game.Catan.Attack.EndPlan
				if q == nil || r.Game.Phase != "catan_attack_end" || r.Game.CatanPendingActor() != actor {
					t.Fatal("missing end response")
				}
				deadline := r.TurnDeadline
				if left := deadline - time.Now().UnixMilli(); left < 119000 || left > 120000 {
					t.Fatal("end stage not given 120 seconds", left)
				}
				// Cannot confirm a castle knight in place, replay a stale phase, or act
				// from another seat/observer. Rejection must not alter deadline or state.
				move, err := r.Game.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
				if move.Choice == "confirm" {
					t.Fatal("castle departure was skipped")
				}
				before, _ := json.Marshal(r)
				for _, viewer := range []int{(actor + 1) % n, n} {
					clients[viewer].command(current(clients[viewer]), "action", move, 400)
				}
				clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_attack_move", Prompt: q.ID, Choice: "confirm"}, 400)
				stale := move
				stale.Prompt--
				clients[actor].command(current(clients[actor]), "action", stale, 400)
				clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_roll"}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid action changed room")
				}
				clients[actor].command(current(clients[actor]), "action", move, 200)
				r = s.rooms[id]
				if r.TurnDeadline != deadline || len(r.Game.Catan.Attack.EndPlan.Moves) != 1 || r.Game.Catan.Attack.Knights[0].Edge != move.Edge {
					t.Fatal("draft committed or reset timer")
				}
				clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_attack_move", Prompt: q.ID, Choice: "undo"}, 200)
				if s.rooms[id].TurnDeadline != deadline || len(s.rooms[id].Game.Catan.Attack.EndPlan.Moves) != 0 {
					t.Fatal("undo reset timer/did not undo")
				}
				clients[actor].command(current(clients[actor]), "action", move, 200)
				// Restart with an unconfirmed plan and its already-running clock.
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				r = s.rooms[id]
				if r.TurnDeadline != deadline || len(r.Game.Catan.Attack.EndPlan.Moves) != 1 {
					t.Fatal("restart lost plan/deadline")
				}
				for viewer, c := range clients {
					v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					a := v["attack"].(map[string]any)
					plan := a["endPlan"].(map[string]any)
					if a["deck"] != nil || a["canAct"] != (viewer == actor) {
						t.Fatal("privacy/control mismatch")
					}
					if viewer == actor {
						if len(plan["moves"].([]any)) != 1 || a["canConfirm"] != true || a["previewKnights"] == nil {
							t.Fatal("actor lost draft preview")
						}
					} else if plan["moves"] != nil || a["moveChoices"] != nil || a["previewKnights"] != nil || a["previewWheat"] != nil || a["canConfirm"] != nil {
						t.Fatal("draft leak to opponent/observer")
					}
					for owner, raw := range v["players"].([]any) {
						if owner != viewer && raw.(map[string]any)["resources"] != nil {
							t.Fatal("private hand leak")
						}
					}
				}
				// Undo again so automatic modes must both choose a move and confirm it.
				clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_attack_move", Prompt: q.ID, Choice: "undo"}, 200)
				at := time.Now()
				if mode == "autoplay" {
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
				}
				for step := 0; s.rooms[id].Game.Phase == "catan_attack_end"; step++ {
					if step > 6 {
						t.Fatal("end response did not finish")
					}
					switch mode {
					case "manual":
						action, err := s.rooms[id].Game.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
						at = time.Now()
						clients[actor].command(current(clients[actor]), "action", action, 200)
					case "autoplay":
						at = time.Now()
						s.mu.Lock()
						s.rooms[id].BotAt = 0
						s.runBots(at)
						s.mu.Unlock()
					case "timeout":
						at = time.UnixMilli(deadline)
						s.mu.Lock()
						s.expireSetups(at)
						s.mu.Unlock()
					}
					r = s.rooms[id]
					if r.Game.Phase == "catan_attack_end" && r.TurnDeadline != deadline {
						t.Fatal("draft got a fresh timeout window")
					}
				}
				if mode == "autoplay" {
					setAutoPlay(clients[actor], current(clients[actor]), false, 200)
				}
				r = s.rooms[id]
				if r.Game.Phase != "catan_roll" || r.Game.Turn == actor || r.Game.CatanPendingActor() != -1 || r.Game.Catan.Attack.EndPlan != nil || r.Game.Catan.Attack.EndSequence != 1 {
					t.Fatal("end response/handoff failed", r.Game.Phase, r.Game.Turn)
				}
				if n == 6 && r.Game.Catan.Paired.Second {
					t.Fatal("secondary paired action not ended")
				}
				if left := r.TurnDeadline - at.UnixMilli(); left < 120000 || left > 121000 || r.CatanTimeLeft != 0 {
					t.Fatal("new turn did not receive full clock", left, r.CatanTimeLeft)
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				clients[actor].command(current(clients[actor]), "action", move, 400)
				if s.rooms[id].Game.Catan.Attack.EndSequence != 1 {
					t.Fatal("replay settled twice")
				}
			})
		}
	}
}
