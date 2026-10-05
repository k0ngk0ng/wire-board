package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanCardConflictHTTPRestart(t *testing.T) {
	for _, variant := range []string{"base", "city", "gold"} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%s/%s", variant, mode), func(t *testing.T) {
				s, ts, clients, id := newCatanTable(t)
				clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				s.mu.Lock()
				r := s.rooms[id]
				cardEventResponseFixture(t, r, variant, time.Now())
				g := r.Game.Catan
				g.CardEvent.Kind, g.CardEvent.Players = "conflict", []int{1}
				g.CardEvent.Optional = variant == "base"
				g.Players[1].Knights = 2
				color := 1
				if variant == "gold" {
					g.ArmyOwner, g.Players[1].Knights = 1, 3
				}
				if variant == "city" {
					color = game.CatanCommodityCloth
					g.CardEvent.Red, g.CardEvent.Face = 6, 0
					g.Dice = []int{6, 0}
					g.CitiesKnights.BarbarianPosition = 0
					g.CitiesKnights.Knights[0].Owner, g.CitiesKnights.Knights[0].Strength = 1, 2
				}
				g.Players[0].Resources[color] = 2
				g.Bank[color] -= 2
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				before, _ := json.Marshal(s.rooms[id])
				for _, p := range []int{0, 2, 3} {
					clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_event_steal", Target: 0}, 400)
				}
				clients[1].command(current(clients[1]), "action", game.Action{Type: "catan_event_steal", Target: 2}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid theft changed room")
				}
				checkPrivacy := func() {
					t.Helper()
					for viewer, c := range clients {
						v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
						for seat, raw := range v["players"].([]any) {
							_, visible := raw.(map[string]any)["resources"]
							if visible != (viewer == seat) {
								t.Fatal("conflict HTTP exposed opponent hand")
							}
						}
						if q, ok := v["cardEvent"].(map[string]any); ok {
							targets := v["legal"].(map[string]any)["eventTargets"].([]any)
							if (len(targets) == 1) != (viewer == 1) || q["canSkip"] != (viewer == 1 && variant == "base") {
								t.Fatal("wrong theft responder hint")
							}
						}
					}
					view := current(clients[3])["game"].(map[string]any)
					for _, raw := range view["log"].([]any) {
						// Public theft log contains no resource/commodity name.
						if line := raw.(string); strings.Contains(line, "冲突：从") && !strings.HasSuffix(line, "冲突：从玩家1随机偷取1张牌") {
							t.Fatal("unexpected theft log or private card detail", line)
						}
					}
				}
				restart := func() {
					t.Helper()
					before, _ := json.Marshal(s.rooms[id])
					ts.Close()
					s.Close()
					next, err := New(s.cfg, s.files)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = next.Close() })
					stopBotTicker(next)
					after, _ := json.Marshal(next.rooms[id])
					if string(before) != string(after) {
						t.Fatal("conflict/continuation room changed on restart")
					}
					nextHTTP := httptest.NewServer(next.Handler())
					t.Cleanup(nextHTTP.Close)
					for _, c := range clients {
						c.base = nextHTTP.URL
					}
					s, ts = next, nextHTTP
					checkPrivacy()
				}
				checkPrivacy()
				restart()
				for step := 0; step < 5 && s.rooms[id].Game.CatanPendingActor() >= 0; step++ {
					r = s.rooms[id]
					actor := r.Game.CatanPendingActor()
					resolvedAt := time.Now()
					if mode == "manual" {
						clients[actor].command(current(clients[actor]), "action", cardEventHTTPAction(t, r.Game), 200)
					} else if mode == "autoplay" {
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						s.mu.Lock()
						s.rooms[id].BotAt = 0
						s.runBots(resolvedAt)
						s.mu.Unlock()
					} else {
						resolvedAt = time.UnixMilli(r.TurnDeadline)
						s.mu.Lock()
						s.expireSetups(resolvedAt)
						s.mu.Unlock()
					}
					r = s.rooms[id]
					want := int64(45000)
					if r.Game.CatanPendingActor() >= 0 {
						want = 120000
					}
					got := r.TurnDeadline - resolvedAt.UnixMilli()
					if got < want || got > want+1000 || r.CatanTimeLeft != 45000 {
						t.Fatal("conflict/continuation response clock", got, want)
					}
					if step == 0 && variant == "gold" {
						if r.Game.Phase != "catan_gold" || r.Game.Catan.Players[1].Resources[color] != 1 || r.Game.Catan.Players[0].Resources[color] != 1 {
							t.Fatal("gold continuation started before theft")
						}
						restart() // A second real restart after theft must not repeat it.
					}
				}
				r = s.rooms[id]
				g = r.Game.Catan
				if r.Game.Phase != "catan_turn" || g.CardEvent != nil || g.RollID != 1 || r.Game.Turn != 0 || g.Robber != -1 || len(g.Victims) != 0 {
					t.Fatal("conflict did not resume original player's turn")
				}
				for p, hand := range g.Players {
					total := 0
					for _, count := range hand.Resources {
						total += count
					}
					want := 1
					if variant == "city" {
						want++
					}
					if p < 2 {
						want++
					}
					if total != want {
						t.Fatal("theft or production duplicated", p, total, want)
					}
					if variant == "city" && (hand.Resources[0] != 1 || hand.Resources[5] != 1) {
						t.Fatal("city resource/commodity production wrong")
					}
				}
				checkPrivacy()
			})
		}
	}
}
