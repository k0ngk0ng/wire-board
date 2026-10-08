package server

import (
	"fmt"
	"testing"
	"time"
)

func TestCatanTransportEventsResponseAutomation(t *testing.T) {
	for _, n := range []int{2, 6} {
		for _, mode := range []string{"autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				s, ts, clients, id := newPublicFishingScenarioVariants(t, n, "transport", false, false, true)
				for step := 0; s.rooms[id].Game.Phase != "catan_card_event"; step++ {
					g := s.rooms[id].Game
					if step > 800 || g.Finished {
						t.Fatal("did not reach a natural event response")
					}
					p := twoHTTPActor(g)
					a, err := g.BotAction(p)
					if err != nil {
						t.Fatal(err)
					}
					clients[p].command(current(clients[p]), "action", a, 200)
				}
				r := s.rooms[id]
				deadline, remaining := r.TurnDeadline, r.CatanTimeLeft
				if deadline <= time.Now().UnixMilli() || remaining <= 0 {
					t.Fatal("event did not pause turn and grant response time")
				}
				actor, roll := r.Game.CatanPendingActor(), r.Game.Catan.RollID
				assertPublicEventsPrivacy(t, clients)
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				r = s.rooms[id]
				if r.TurnDeadline != deadline || r.CatanTimeLeft != remaining || r.Game.CatanPendingActor() != actor {
					t.Fatal("restart changed event response/clock")
				}
				if mode == "autoplay" {
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
				}
				version := s.rooms[id].Version
				s.mu.Lock()
				if mode == "autoplay" {
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
				} else {
					s.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
					s.expireSetups(time.Now())
				}
				s.mu.Unlock()
				r = s.rooms[id]
				if r.Version <= version || r.Game.Catan.RollID != roll || r.Game.Catan.Transport.Travel != nil {
					t.Fatal("automatic event did not advance or drew/moved twice")
				}
				if !r.Seats[actor].AutoPlay {
					t.Fatal("automatic control was not persistent")
				}
				assertTransportInventory(t, r.Game)
			})
		}
	}
}
