package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func setAutoPlay(c *testClient, r map[string]any, enabled bool, want int) {
	c.t.Helper()
	c.post("/api/rooms/"+r["id"].(string), map[string]any{
		"type": "autoplay", "enabled": enabled, "version": r["version"], "nonce": randomID(12),
	}, want)
}

func autoPlayTable(t *testing.T, kind string) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	n := 2
	if kind == "catan" {
		n = 3
	}
	if kind == "sanguosha" {
		n = 4
	}
	clients := make([]*testClient, n)
	for i := range clients {
		clients[i] = newClient(t, ts.URL)
		clients[i].register(fmt.Sprintf("托管玩家%d", i))
	}
	a := clients[0]
	r := a.post("/api/rooms", map[string]any{"name": "托管验证", "kind": kind, "capacity": n}, 201)
	setAutoPlay(a, r, true, 400)
	for _, c := range clients[1:] {
		c.command(current(a), "join", nil, 200)
	}
	for _, c := range clients {
		c.command(current(c), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	return s, ts, clients, r["id"].(string)
}

func TestAutoPlayPermissionsCancellationAndOwnership(t *testing.T) {
	s, ts, clients, id := autoPlayTable(t, "splendor")
	a, b := clients[0], clients[1]
	v := newClient(t, ts.URL)
	v.register("旁观玩家")
	r := current(a)
	setAutoPlay(v, r, true, 400)
	v.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	setAutoPlay(v, current(v), true, 400)
	a.post("/api/rooms/"+id, map[string]any{"type": "autoplay", "version": r["version"], "nonce": randomID(12)}, 400)
	a.post("/api/rooms/"+id, map[string]any{"type": "autoplay", "enabled": true, "target": s.rooms[id].Seats[1].ID, "version": r["version"], "nonce": randomID(12)}, 400)
	deadline := s.rooms[id].TurnDeadline
	userID := s.rooms[id].Seats[0].ID
	body := map[string]any{"type": "autoplay", "enabled": true, "version": r["version"], "nonce": randomID(12)}
	a.post("/api/rooms/"+id, body, 200)
	a.post("/api/rooms/"+id, body, 200)
	if s.rooms[id].Version != int(r["version"].(float64))+1 {
		t.Fatal("replayed control command")
	}
	seat := s.rooms[id].Seats[0]
	if !seat.AutoPlay || seat.Bot || seat.ID != userID || !s.rooms[id].Rated || s.rooms[id].TurnDeadline != deadline {
		t.Fatal("control changed identity, ratings or thinking time")
	}
	if current(b)["seats"].([]any)[0].(map[string]any)["autoPlay"] != true || current(v)["seats"].([]any)[0].(map[string]any)["autoPlay"] != true {
		t.Fatal("control status not shared")
	}
	a.command(current(a), "action", game.Action{Type: "reserve", Tier: 1}, 400)
	expireTurn(t, s, id)
	kick(b, current(b), userID, 400)
	botTick(s, id)
	if s.rooms[id].Game.Turn != 1 {
		t.Fatal("managed player did not act")
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM actions WHERE room_id=? AND user_id=? AND json_extract(action,'$.autoPlay')=1", id, userID).Scan(&count); err != nil || count != 1 {
		t.Fatal("action not attributed to human", count, err)
	}
	// A stale version is expected while the computer plays; reclaiming is atomic.
	deadline = s.rooms[id].TurnDeadline
	setAutoPlay(a, r, false, 200)
	if s.rooms[id].Seats[0].AutoPlay || s.rooms[id].TurnDeadline != deadline {
		t.Fatal("cancel reset clock or failed")
	}
	b.command(current(b), "action", game.Action{Type: "reserve", Tier: 1}, 200)
	before, _ := json.Marshal(s.rooms[id].Game)
	botTick(s, id)
	after, _ := json.Marshal(s.rooms[id].Game)
	if string(before) != string(after) {
		t.Fatal("computer acted after cancellation")
	}
	a.command(current(a), "action", game.Action{Type: "reserve", Tier: 1}, 200)
	setAutoPlay(a, current(a), true, 200)
	a.command(current(a), "close", nil, 200)
	setAutoPlay(a, current(a), false, 400)
	a.command(current(a), "rematch", nil, 200)
	if s.rooms[id].Seats[0].AutoPlay || s.rooms[id].Seats[0].Ready {
		t.Fatal("rematch retained managed human")
	}
	for _, c := range clients {
		c.command(current(c), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	setAutoPlay(a, r, true, 409) // Delayed commands from the previous match cannot take over the new one.
}

func TestAutoPlayAllGamesFinishAndCreditHumanPlayers(t *testing.T) {
	for _, kind := range []string{"splendor", "rail", "catan", "carcassonne", "sanguosha"} {
		t.Run(kind, func(t *testing.T) {
			s, _, clients, id := autoPlayTable(t, kind)
			initial := current(clients[0])
			for _, c := range clients {
				setAutoPlay(c, initial, true, 200)
			}
			for step := 0; step < 6000 && s.rooms[id].Status == "playing"; step++ {
				version := s.rooms[id].Version
				botTick(s, id)
				if s.rooms[id].Version != version+1 {
					t.Fatalf("managed game stalled at %s, step %d", s.rooms[id].Game.Phase, step)
				}
			}
			r := s.rooms[id]
			if r.Status != "finished" || r.TurnDeadline != 0 {
				t.Fatal("managed game failed to finish")
			}
			for _, seat := range r.Seats {
				if seat.Bot || seat.Left {
					t.Fatal("managed human became a bot or left")
				}
				rating, err := s.rating(seat.ID)
				if err != nil || rating.Played != 1 {
					t.Fatal("human did not retain rated result", rating, err)
				}
			}
			var archived MatchRecord
			var raw []byte
			if err := s.db.QueryRow("SELECT snapshot FROM match_history WHERE id=?", r.MatchID).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &archived); err != nil || !archived.Rated {
				t.Fatal("managed match not archived for humans")
			}
		})
	}
}

func TestAutoPlayPersistsWithoutBrowserAndRestoresControl(t *testing.T) {
	s, ts, clients, id := autoPlayTable(t, "rail")
	a := clients[0]
	setAutoPlay(a, current(a), true, 200)
	before := current(a)
	deadline := s.rooms[id].TurnDeadline
	ts.Close()
	s.Close()
	resumed, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	stopBotTicker(resumed)
	if !resumed.rooms[id].Seats[0].AutoPlay || resumed.rooms[id].Seats[0].Bot || resumed.rooms[id].TurnDeadline != deadline {
		t.Fatal("restart changed seat control")
	}
	botTick(resumed, id)
	if len(resumed.rooms[id].Game.Rail.SetupPending[0]) != 0 || len(resumed.rooms[id].Game.Rail.SetupPending[1]) == 0 || resumed.rooms[id].TurnDeadline != deadline {
		t.Fatal("parallel destination selection failed")
	}
	ts2 := httptest.NewServer(resumed.Handler())
	defer ts2.Close()
	a.base = ts2.URL
	setAutoPlay(a, before, false, 200)
	// Both identity and private destinations are still available to the owner.
	view := current(a)["game"].(map[string]any)["rail"].(map[string]any)["players"].([]any)
	if len(view[0].(map[string]any)["tickets"].([]any)) < 2 {
		t.Fatal("owner lost private destinations")
	}
	if _, ok := view[1].(map[string]any)["tickets"]; ok {
		t.Fatal("opponent destinations exposed")
	}
}

func TestAutoPlayFinishesPartialTurns(t *testing.T) {
	t.Run("splendor return and noble", func(t *testing.T) {
		s, _, clients, id := autoPlayTable(t, "splendor")
		r := s.rooms[id]
		r.Game.Splendor.Players[0].Tokens = []int{3, 2, 2, 2, 2, 0}
		r.Game.Splendor.Players[0].Bonus = []int{5, 5, 5, 5, 5}
		r.Game.Phase = "discard"
		setAutoPlay(clients[0], current(clients[0]), true, 200)
		botTick(s, id)
		if s.rooms[id].Game.Phase != "noble" {
			t.Fatal("did not return excess tokens")
		}
		botTick(s, id)
		if s.rooms[id].Game.Turn != 1 || len(s.rooms[id].Game.Splendor.Players[0].Nobles) != 1 {
			t.Fatal("did not select noble")
		}
	})
	t.Run("rail second draw", func(t *testing.T) {
		s, _, clients, id := autoPlayTable(t, "rail")
		s.rooms[id].Game.AutoChooseRailSetup()
		clients[0].command(current(clients[0]), "action", game.Action{Type: "draw", Slot: -1}, 200)
		setAutoPlay(clients[0], current(clients[0]), true, 200)
		botTick(s, id)
		if s.rooms[id].Game.Turn != 1 || s.rooms[id].Game.Rail.Drawn != 0 || len(s.rooms[id].Game.Rail.Players[0].Hand) == 0 {
			t.Fatal("second draw stalled")
		}
	})
}

func TestAutoPlayCatanRespondsOutsideOwnTurn(t *testing.T) {
	s, _, clients, id := newCatanTable(t)
	r := s.rooms[id]
	g := r.Game.Catan
	for i := range g.Players {
		for color, n := range g.Players[i].Resources {
			g.Bank[color] += n
			g.Players[i].Resources[color] = 0
		}
	}
	g.Players[1].Resources[1] = 10
	g.Bank[1] -= 10
	g.DiscardDue = []int{0, 5, 0}
	g.ResumePhase = "catan_turn"
	r.Game.Phase = "catan_discard"
	r.CatanPendingVersion = r.Version
	r.startTurnClock(time.Now())
	setAutoPlay(clients[1], current(clients[1]), true, 200)
	botTick(s, id)
	if s.rooms[id].Game.Catan.DiscardDue[1] != 0 || s.rooms[id].Game.Phase != "catan_robber" {
		t.Fatal("non-turn discard stalled")
	}
	r = s.rooms[id]
	r.Game.Phase = "catan_turn"
	g = r.Game.Catan
	g.Players[0].Resources[0] = 1
	g.Bank[0]--
	g.Players[1].Resources[2] = 1
	g.Bank[2]--
	clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_trade_offer", Give: []int{1, 0, 0, 0, 0}, Take: []int{0, 0, 1, 0, 0}}, 200)
	botTick(s, id)
	if s.rooms[id].Game.Catan.Trade.Responses[1] == 0 || s.rooms[id].Game.Catan.Trade.Responses[2] != 0 {
		t.Fatal("managed trade response changed human decision")
	}
}

func TestAutoPlaySanguoshaRespondsOutsideOwnTurn(t *testing.T) {
	s, _, clients, id := autoPlayTable(t, "sanguosha")
	r := s.rooms[id]
	for r.Game.Sanguosha.Selecting || r.Game.Sanguosha.Pending != nil {
		i := r.Game.SanguoshaActor()
		a, err := r.Game.BotAction(i)
		if err != nil {
			t.Fatal(err)
		}
		if err = r.applyGameAction(i, a, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	i := r.Game.Turn
	target := (i + 1) % len(clients)
	r.Game.Sanguosha.Players[i].General = "zhouyu"
	clients[i].command(current(clients[i]), "action", game.Action{Type: "sg_skill", Skill: "fanjian", Targets: []int{target}}, 200)
	if s.rooms[id].Game.Sanguosha.Pending.Player != target {
		t.Fatal("expected response target")
	}
	setAutoPlay(clients[target], current(clients[target]), true, 200)
	before := s.rooms[id].Game.Sanguosha.Pending.ID
	botTick(s, id)
	if p := s.rooms[id].Game.Sanguosha.Pending; p != nil && p.ID == before {
		t.Fatal("managed response stalled")
	}
	setAutoPlay(clients[target], current(clients[target]), false, 200)
	state, _ := json.Marshal(s.rooms[id].Game)
	botTick(s, id)
	after, _ := json.Marshal(s.rooms[id].Game)
	if !reflect.DeepEqual(state, after) {
		t.Fatal("response continued after cancellation")
	}
}
