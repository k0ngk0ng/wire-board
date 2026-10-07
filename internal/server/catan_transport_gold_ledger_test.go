package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// The initial constructor fixture plays legal actions until an actual delivery.
// Only then do we redistribute the finite coin bank to exercise exhaustion.
func transportLedgerArrival(t *testing.T, n int) (*game.State, game.Action) {
	t.Helper()
	s := transportHTTPFixture(t, n)
	for steps := 0; steps < 6000 && !s.Finished; steps++ {
		actor := twoHTTPActor(s)
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(steps, err)
		}
		if a.Type == "catan_transport_arrival" && a.Choice == "deliver" {
			tr := s.Catan.Transport
			other := (actor + 1) % n
			tr.Gold[other] += tr.GoldBank
			tr.GoldBank = 0
			return s, a
		}
		if err = s.Apply(actor, a); err != nil {
			t.Fatal(steps, err)
		}
	}
	t.Fatal("no natural delivery")
	return nil, game.Action{}
}

func TestCatanTransportHTTPGoldLedgerDelivery(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				var s *Server
				var ts *httptest.Server
				var clients []*testClient
				var id string
				if n == 2 {
					s, ts, clients, id = newTwoFullTable(t)
				} else {
					s, ts, clients, id, _, _ = newFishingActionTable(t, n, "catan_turn", "resource")
				}
				state, action := transportLedgerArrival(t, n)
				actor := state.Turn
				tr := state.Catan.Transport
				reward := tr.Wagons[actor].Level + 1
				oldGold := tr.Gold[actor]
				cargo := tr.Wagons[actor].Cargo
				delivered := len(tr.Wagons[actor].Delivered)
				deadline := time.Now().Add(45 * time.Second).UnixMilli()
				s.mu.Lock()
				r := s.rooms[id]
				r.Game, r.TurnDeadline = state, deadline
				r.CatanTimeLeft = 0
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				before, _ := json.Marshal(r)
				for _, viewer := range []int{(actor + 1) % n, n} {
					clients[viewer].command(current(clients[viewer]), "action", action, 400)
				}
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("rejected delivery changed ledger")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				request := map[string]any{"type": "action", "version": s.rooms[id].Version, "nonce": "transport-ledger-delivery", "action": action}
				switch mode {
				case "manual":
					clients[actor].post("/api/rooms/"+id, request, 200)
				case "autoplay":
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
				case "timeout":
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(deadline))
					s.mu.Unlock()
				}
				r = s.rooms[id]
				tr = r.Game.Catan.Transport
				if tr.GoldIssued != reward || tr.GoldBank != 0 || tr.Gold[actor] != oldGold+reward || len(tr.Wagons[actor].Delivered) != delivered+1 || tr.Wagons[actor].Delivered[delivered] != cargo {
					t.Fatal("delivery was blocked or mispaid")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				if mode == "manual" {
					before, _ = json.Marshal(s.rooms[id])
					clients[actor].post("/api/rooms/"+id, request, 200)
					after, _ = json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("replay rewarded twice")
					}
				}
				for viewer, c := range clients {
					v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					tv := v["transport"].(map[string]any)
					pub := tv["state"].(map[string]any)
					if pub["goldRule"] != "ledger" || pub["goldIssued"] != float64(reward) || tv["stacks"] != nil {
						t.Fatal("rule missing or cargo order leaked")
					}
					for p, raw := range v["players"].([]any) {
						if p != viewer && raw.(map[string]any)["resources"] != nil {
							t.Fatal("private hand leaked")
						}
					}
				}
			})
		}
	}
}
