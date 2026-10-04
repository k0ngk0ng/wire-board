package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanFogHTTPPrivacySetupRewardRestartAndClock(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	s.mu.Lock()
	r := s.rooms[id]
	g := r.Game.Catan
	g.Seafarers = &game.CatanSeafarers{Scenario: "fog", VictoryPoints: 12, Pirate: -1, Fog: &game.CatanFogState{Terrain: []int{1, 6, 7}, Numbers: []int{4, 9}, StartTiles: []int{0}}}
	g.SetupStep = 0
	g.StartPlayer = 0
	g.Options.Helpers = false
	g.Edges[0].Owner = 0
	g.Edges[0].Ship = true
	g.GoldPending = &game.CatanGoldPending{Claims: []game.CatanGoldClaim{{Player: 0, Count: 1}}, Resume: "catan_setup_road", AfterRoute: &game.CatanRouteCompletion{Player: 0, Edge: 0, Setup: true}}
	r.Game.Turn = 0
	r.Game.Phase = "catan_gold"
	now := time.Now()
	r.TurnDeadline = now.Add(35 * time.Second).UnixMilli()
	r.adjustCatanResponseClock("catan_setup_road", -1, g.SetupStep, now)
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	for _, c := range clients {
		fog := current(c)["game"].(map[string]any)["catan"].(map[string]any)["seafarers"].(map[string]any)["fog"].(map[string]any)
		if _, ok := fog["terrain"]; ok {
			t.Fatal("HTTP leaked hidden terrain")
		}
		if _, ok := fog["numbers"]; ok {
			t.Fatal("HTTP leaked hidden numbers")
		}
		if fog["remaining"] != float64(3) {
			t.Fatal("HTTP fog count")
		}
	}
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_gold", "take": []int{1, 0, 0, 0, 0}}, 400)
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_end"}, 400)
	before, _ := json.Marshal(s.rooms[id])
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	stopBotTicker(next)
	after, _ := json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("fog stacks or route continuation lost")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_gold", "take": []int{1, 0, 0, 0, 0}}, 200)
	r = next.rooms[id]
	if r.Game.Turn != 1 || r.Game.Phase != "catan_setup_settlement" || r.Game.Catan.SetupStep != 1 || r.Game.Catan.GoldPending != nil {
		t.Fatal("setup discovery did not advance")
	}
	remaining := r.TurnDeadline - time.Now().UnixMilli()
	if remaining < 119000 || remaining > 120000 {
		t.Fatal("new setup seat inherited old clock", remaining)
	}
}

func TestCatanFogGoldHelperTimeoutChainRestoresClock(t *testing.T) {
	s, _, _, id := newCatanTable(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	g := r.Game.Catan
	g.Options.Helpers = true
	g.TurnSerial = 1
	g.SetupStep = 6
	g.Players[0].Helper = &game.CatanHelperSeat{ID: 2}
	g.HelperDisplay = []int{1, 3, 4}
	g.GoldPending = &game.CatanGoldPending{Claims: []game.CatanGoldClaim{{Player: 0, Count: 1}}, Resume: "catan_turn", AfterRoute: &game.CatanRouteCompletion{Player: 0, Edge: 0, Helper: true}}
	r.Game.Phase = "catan_gold"
	r.Game.Turn = 0
	now := time.Now()
	r.CatanTimeLeft = 45000
	r.TurnDeadline = now.Add(-time.Second).UnixMilli()
	s.expireSetups(now)
	r = s.rooms[id]
	if r.Game.Phase != "catan_helper" || r.Game.Catan.HelperPending.Kind != "exchange" || r.TurnDeadline != now.Add(turnLimit).UnixMilli() || r.CatanTimeLeft != 45000 {
		t.Fatal("gold/helper timeout handoff")
	}
	later := now.Add(turnLimit + time.Second)
	s.expireSetups(later)
	r = s.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.Catan.HelperPending != nil || r.TurnDeadline != later.Add(45*time.Second).UnixMilli() {
		t.Fatal("helper timeout lost remaining action clock")
	}
}

func TestCatanFogPreRollRoadBuildingGoldPreservesClock(t *testing.T) {
	s, _, _, id := newCatanTable(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	g := r.Game.Catan
	g.SetupStep = 6
	g.FreeRoads = 1
	g.ResumePhase = "catan_roll"
	g.GoldPending = &game.CatanGoldPending{Claims: []game.CatanGoldClaim{{Player: 0, Count: 1}}, Resume: "catan_roads", AfterRoute: &game.CatanRouteCompletion{Player: 0, Edge: 0, Free: true}}
	r.Game.Phase = "catan_gold"
	r.Game.Turn = 0
	r.CatanTimeLeft = 37000
	now := time.Now()
	r.TurnDeadline = now.Add(-time.Second).UnixMilli()
	s.expireSetups(now)
	r = s.rooms[id]
	if r.Game.Phase != "catan_roll" || r.Game.Catan.FreeRoads != 0 || r.TurnDeadline != now.Add(37*time.Second).UnixMilli() {
		t.Fatal("pre-roll discovery reset the action clock")
	}
}

func TestCatanFogFinalSetupGoldStartsFirstRollClock(t *testing.T) {
	s, _, _, id := newCatanTable(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	g := r.Game.Catan
	g.SetupStep = 5
	g.StartPlayer = 0
	g.Options.Helpers = false
	g.GoldPending = &game.CatanGoldPending{Claims: []game.CatanGoldClaim{{Player: 0, Count: 1}}, Resume: "catan_setup_road", AfterRoute: &game.CatanRouteCompletion{Player: 0, Edge: 0, Setup: true}}
	r.Game.Phase = "catan_gold"
	r.Game.Turn = 0
	r.CatanTimeLeft = 37000
	now := time.Now()
	if err := r.applyGameAction(0, game.Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}}, now); err != nil {
		t.Fatal(err)
	}
	if r.Game.Phase != "catan_roll" || r.Game.Turn != 0 || r.Game.Catan.SetupStep != 6 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("last setup discovery did not start a fresh production turn")
	}
}
