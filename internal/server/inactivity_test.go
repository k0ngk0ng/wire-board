package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func idleTick(s *Server, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expireIdleRooms(now)
}

func TestInactiveGamesCloseWithoutAwardingResults(t *testing.T) {
	for _, kind := range []string{"splendor", "rail", "catan", "carcassonne", "sanguosha"} {
		t.Run(kind, func(t *testing.T) {
			s, _, clients, id := autoPlayTable(t, kind)
			now := time.Now().Truncate(time.Second)
			r := s.rooms[id]
			r.LastActive = now.Add(-roomIdleLimit + time.Second).Unix()
			version := r.Version
			idleTick(s, now)
			if s.rooms[id].Status != "playing" {
				t.Fatal("closed active room before 24 hours")
			}
			idleTick(s, now.Add(time.Second))
			r = s.rooms[id]
			if r.Status != "closed" || r.CloseReason != "inactive" || r.TurnDeadline != 0 || r.BotAt != 0 || r.Version != version+1 || r.Game.Finished {
				t.Fatal("idle closure changed game outcome or left a clock running")
			}
			state := clients[0].state()
			if len(state["rooms"].([]any)) != 0 || state["room"].(map[string]any)["closeReason"] != "inactive" {
				t.Fatal("closed table still advertised or unexplained")
			}
			for _, seat := range r.Seats {
				rating, err := s.rating(seat.ID)
				if err != nil || rating.Played != 0 {
					t.Fatal("idle closure changed rating", rating, err)
				}
			}
			var raw []byte
			if err := s.db.QueryRow("SELECT snapshot FROM match_history WHERE id=?", r.MatchID).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var record MatchRecord
			if err := json.Unmarshal(raw, &record); err != nil || record.Status != "closed" || record.Rated {
				t.Fatal("idle game not archived as aborted")
			}
			for _, p := range record.Players {
				if p.Won {
					t.Fatal("idle game awarded a winner")
				}
			}
			idleTick(s, now.Add(48*time.Hour))
			if s.rooms[id].Version != version+1 {
				t.Fatal("closed same room again")
			}
			clients[0].command(current(clients[0]), "rematch", nil, 200)
			if s.rooms[id].Status != "waiting" || s.rooms[id].CloseReason != "" || s.rooms[id].LastActive < now.Unix() {
				t.Fatal("rematch did not restore active room")
			}
		})
	}
}

func TestInactiveWaitingRoomClosesWithoutHistory(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	a := newClient(t, ts.URL)
	a.register("等待玩家")
	r := a.post("/api/rooms", map[string]any{"name": "空闲牌桌", "kind": "splendor", "capacity": 2}, 201)
	id := r["id"].(string)
	s.rooms[id].LastActive = time.Now().Add(-25 * time.Hour).Unix()
	// An attempted late command expires the table before it can revive a stale game.
	a.command(r, "ready", nil, 409)
	if current(a)["status"] != "closed" {
		t.Fatal("waiting room not closed")
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM match_history").Scan(&count); err != nil || count != 0 {
		t.Fatal("unstarted game created a match", err)
	}
	a.command(current(a), "leave", nil, 200)
	if len(s.rooms) != 0 {
		t.Fatal("closed empty room retained")
	}
}

func TestInactiveDeadlineIgnoresObserversChatAndTimeouts(t *testing.T) {
	s, ts, clients, id := autoPlayTable(t, "rail")
	a := clients[0]
	v := newClient(t, ts.URL)
	v.register("看牌玩家")
	now := time.Now().Truncate(time.Second)
	s.rooms[id].LastActive = now.Add(-roomIdleLimit + 10*time.Second).Unix()
	s.rooms[id].TurnDeadline = now.Add(-time.Second).UnixMilli()
	last := s.rooms[id].LastActive
	a.state()
	v.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	a.post("/api/rooms/"+id+"/chat", map[string]any{"text": "等等朋友", "nonce": randomID(12)}, 201)
	s.mu.Lock()
	s.expireSetups(now)
	s.mu.Unlock()
	if s.rooms[id].Game.Rail.Setup || s.rooms[id].Updated != now.Unix() || s.rooms[id].LastActive != last {
		t.Fatal("forced setup counted as activity")
	}
	idleTick(s, now.Add(10*time.Second))
	if s.rooms[id].Status != "closed" {
		t.Fatal("passive activity prevented closure")
	}
	v.post("/api/rooms/"+id+"/watch", map[string]any{"leave": true}, 200)
	v.post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
}

func TestInactiveDeadlineRenewedByPlayAndAutoplay(t *testing.T) {
	s, _, clients, id := autoPlayTable(t, "splendor")
	nearlyExpired := time.Now().Add(-roomIdleLimit + time.Minute).Unix()
	s.rooms[id].LastActive = nearlyExpired
	clients[0].command(current(clients[0]), "action", game.Action{Type: "reserve", Tier: 1}, 200)
	if s.rooms[id].LastActive <= nearlyExpired {
		t.Fatal("human action did not renew inactivity")
	}
	setAutoPlay(clients[1], current(clients[1]), true, 200)
	s.rooms[id].LastActive = nearlyExpired
	botTick(s, id)
	if s.rooms[id].LastActive <= nearlyExpired {
		t.Fatal("autoplay action did not renew inactivity")
	}
	idleTick(s, time.Now().Add(2*time.Minute))
	if s.rooms[id].Status != "playing" {
		t.Fatal("active managed room closed")
	}
}

func TestInactiveRoomsCloseAfterRestartAndLegacyMigration(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "persisted deadline", true: "legacy updated time"}[legacy], func(t *testing.T) {
			s, ts, clients, id := autoPlayTable(t, "splendor")
			r := s.rooms[id]
			r.Updated = time.Now().Add(-25 * time.Hour).Unix()
			r.LastActive = r.Updated
			if legacy {
				r.LastActive = 0
			}
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			ts.Close()
			s.Close()
			restarted, err := New(s.cfg, s.files)
			if err != nil {
				t.Fatal(err)
			}
			defer restarted.Close()
			stopBotTicker(restarted)
			if restarted.rooms[id].Status != "closed" {
				t.Fatal("restart reset idle deadline")
			}
			ts2 := httptest.NewServer(restarted.Handler())
			defer ts2.Close()
			clients[0].base = ts2.URL
			if current(clients[0])["closeReason"] != "inactive" {
				t.Fatal("close reason missing after restart")
			}
		})
	}
}

func TestInactiveTimerRunsWithoutBrowser(t *testing.T) {
	s, ts := setupServer(t)
	a := newClient(t, ts.URL)
	a.register("暂离玩家")
	r := a.post("/api/rooms", map[string]any{"name": "无人牌桌", "kind": "splendor", "capacity": 2}, 201)
	id := r["id"].(string)
	s.mu.Lock()
	s.rooms[id].LastActive = time.Now().Add(-25 * time.Hour).Unix()
	s.mu.Unlock()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		s.mu.Lock()
		closed := s.rooms[id].Status == "closed"
		s.mu.Unlock()
		if closed {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("background timer did not close abandoned room")
}
