package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestTimeoutAutoplayAllGamesPersistsAndCanBeCancelled(t *testing.T) {
	for _, kind := range []string{"splendor", "rail", "catan", "carcassonne", "sanguosha", "dota"} {
		t.Run(kind, func(t *testing.T) {
			s, ts, clients, id := autoPlayTable(t, kind)
			r := s.rooms[id]
			actors := slices.Clone(r.timeoutActors())
			host := r.Host
			match := r.MatchID
			last := r.LastActive
			deadline := time.Now()
			r.TurnDeadline = deadline.UnixMilli()
			version := r.Version
			s.mu.Lock()
			s.expireSetups(deadline)
			s.mu.Unlock()
			r = s.rooms[id]
			if r.Version != version+1 || r.Host != host || r.MatchID != match || r.LastActive != last || !r.Rated {
				t.Fatal("timeout lost identity, rating, persistence or inactivity")
			}
			for p, seat := range r.Seats {
				if seat.Bot || seat.Left || seat.AutoPlay != slices.Contains(actors, p) || seat.TimeoutAutoPlay != slices.Contains(actors, p) {
					t.Fatal("wrong takeover seat", p, seat)
				}
			}
			snapshot, _ := json.Marshal(r)
			s.mu.Lock()
			s.expireSetups(deadline)
			s.mu.Unlock()
			same, _ := json.Marshal(s.rooms[id])
			if string(snapshot) != string(same) {
				t.Fatal("repeated tick duplicated timeout")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			restored, _ := json.Marshal(s.rooms[id])
			if string(snapshot) != string(restored) {
				t.Fatal("restart lost takeover or replayed action")
			}
			for _, p := range actors {
				pub := current(clients[p])["seats"].([]any)[p].(map[string]any)
				if pub["autoPlay"] != true {
					t.Fatal("robot indicator flag missing")
				}
				setAutoPlay(clients[p], current(clients[p]), false, 200)
				if s.rooms[id].Seats[p].AutoPlay || s.rooms[id].Seats[p].TimeoutAutoPlay {
					t.Fatal("could not reclaim control")
				}
			}
			before, _ := json.Marshal(s.rooms[id].Game)
			botTick(s, id)
			after, _ := json.Marshal(s.rooms[id].Game)
			if string(before) != string(after) {
				t.Fatal("computer acted after all takeovers cancelled")
			}
		})
	}
}

func TestTimeoutAutoplayOnlyOutstandingParallelPlayers(t *testing.T) {
	t.Run("rail", func(t *testing.T) {
		s, _, clients, id := autoPlayTable(t, "rail")
		r := s.rooms[id]
		keep := r.Game.Rail.SetupPending[0]
		clients[0].command(current(clients[0]), "action", game.Action{Type: "keep", Keep: []int{keep[0].ID, keep[1].ID}}, 200)
		expireTurn(t, s, id)
		s.mu.Lock()
		s.expireSetups(time.Now())
		s.mu.Unlock()
		r = s.rooms[id]
		if r.Game.Rail.Setup || r.Seats[0].AutoPlay || !r.Seats[1].AutoPlay || r.Game.Turn != 0 {
			t.Fatal("already ready rail player was taken over")
		}
	})
	t.Run("catan discard", func(t *testing.T) {
		s, _, _, id := newCatanTable(t)
		r := s.rooms[id]
		g := r.Game.Catan
		for p := range g.Players {
			for c, count := range g.Players[p].Resources {
				g.Bank[c] += count
				g.Players[p].Resources[c] = 0
			}
		}
		for _, p := range []int{1, 2} {
			g.Players[p].Resources[p] = 10
			g.Bank[p] -= 10
			g.DiscardDue[p] = 5
		}
		r.Game.Turn = 0
		r.Game.Phase = "catan_discard"
		g.ResumePhase = "catan_turn"
		now := time.Now()
		r.TurnDeadline = now.UnixMilli()
		s.mu.Lock()
		s.expireSetups(now)
		s.mu.Unlock()
		r = s.rooms[id]
		if r.Seats[0].AutoPlay || !r.Seats[1].AutoPlay || !r.Seats[2].AutoPlay || r.Game.Phase != "catan_robber" || r.Game.Catan.DiscardDue[1] != 0 || r.Game.Catan.DiscardDue[2] != 0 {
			t.Fatal("discard took over turn owner or missed a discarder")
		}
	})
	t.Run("sanguosha response", func(t *testing.T) {
		s, _, clients, id := autoPlayTable(t, "sanguosha")
		r := s.rooms[id]
		for r.Game.Sanguosha.Selecting || r.Game.Sanguosha.Pending != nil {
			p := r.Game.SanguoshaActor()
			a, err := r.Game.BotAction(p)
			if err != nil {
				t.Fatal(err)
			}
			if err = r.applyGameAction(p, a, time.Now()); err != nil {
				t.Fatal(err)
			}
		}
		turn := r.Game.Turn
		target := (turn + 1) % len(clients)
		r.Game.Sanguosha.Players[turn].General = "zhouyu"
		clients[turn].command(current(clients[turn]), "action", game.Action{Type: "sg_skill", Skill: "fanjian", Targets: []int{target}}, 200)
		oldPrompt := s.rooms[id].Game.Sanguosha.Pending.ID
		expireTurn(t, s, id)
		s.mu.Lock()
		s.expireSetups(time.Now())
		s.mu.Unlock()
		r = s.rooms[id]
		if r.Seats[turn].AutoPlay || !r.Seats[target].AutoPlay {
			t.Fatal("response takeover assigned to turn owner")
		}
		if q := r.Game.Sanguosha.Pending; q != nil && q.ID == oldPrompt {
			t.Fatal("computer failed to respond")
		}
	})
}

func TestTimeoutAutoplayCancellationGivesExpiredActorTime(t *testing.T) {
	s, _, clients, id := autoPlayTable(t, "splendor")
	r := s.rooms[id]
	r.Game.Splendor.Players[0].Tokens = []int{3, 2, 2, 2, 2, 0}
	r.Game.Splendor.Players[0].Bonus = []int{5, 5, 5, 5, 5}
	r.Game.Phase = "discard"
	expireTurn(t, s, id)
	s.mu.Lock()
	s.expireSetups(time.Now())
	s.mu.Unlock()
	if s.rooms[id].Game.Phase != "noble" || !s.rooms[id].Seats[0].AutoPlay {
		t.Fatal("partial turn takeover fixture")
	}
	setAutoPlay(clients[0], current(clients[0]), false, 200)
	if left := time.Until(time.UnixMilli(s.rooms[id].TurnDeadline)); left < 119*time.Second || left > 120*time.Second {
		t.Fatal("returning actor has no manual response window", left)
	}
	before := s.rooms[id].Version
	s.mu.Lock()
	s.expireSetups(time.Now())
	s.mu.Unlock()
	if s.rooms[id].Version != before || s.rooms[id].Seats[0].AutoPlay {
		t.Fatal("immediately re-enabled takeover")
	}
	action, err := s.rooms[id].Game.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	clients[0].command(current(clients[0]), "action", action, 200)
	if s.rooms[id].Game.Turn != 1 {
		t.Fatal("returning player could not finish choice")
	}
}

func TestTimeoutAutoplayDoesNotKeepAbandonedRoomsAlive(t *testing.T) {
	s, _, _, id := autoPlayTable(t, "splendor")
	r := s.rooms[id]
	now := time.Now().Truncate(time.Second)
	last := now.Add(-roomIdleLimit + time.Minute).Unix()
	r.LastActive = last
	r.TurnDeadline = now.UnixMilli()
	s.mu.Lock()
	s.expireSetups(now)
	s.mu.Unlock()
	r = s.rooms[id]
	r.TurnDeadline = now.UnixMilli()
	s.mu.Lock()
	s.expireSetups(now)
	s.mu.Unlock()
	if !s.rooms[id].Seats[0].TimeoutAutoPlay || !s.rooms[id].Seats[1].TimeoutAutoPlay {
		t.Fatal("both timeout seats required")
	}
	for range 4 {
		botTick(s, id)
	}
	if s.rooms[id].LastActive != last {
		t.Fatal("forced autoplay renewed abandoned room")
	}
	idleTick(s, now.Add(time.Minute))
	if s.rooms[id].Status != "closed" || s.rooms[id].CloseReason != "inactive" {
		t.Fatal("abandoned automatic game was kept alive")
	}
}

func TestTimeoutAutoplaySaveFailureIsAtomic(t *testing.T) {
	s, _, _, id := autoPlayTable(t, "splendor")
	r := s.rooms[id]
	now := time.Now()
	r.TurnDeadline = now.UnixMilli()
	before, _ := json.Marshal(r)
	if _, err := s.db.Exec("CREATE TRIGGER reject_timeout BEFORE UPDATE ON rooms BEGIN SELECT RAISE(FAIL, 'forced save failure'); END"); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.expireSetups(now)
	s.mu.Unlock()
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("failed save changed control or game")
	}
	if _, err := s.db.Exec("DROP TRIGGER reject_timeout"); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.expireSetups(now)
	s.mu.Unlock()
	count := 0
	for _, line := range s.rooms[id].Game.Log {
		if strings.Contains(line, "已自动开启电脑托管") {
			count++
		}
	}
	if count != 1 || !s.rooms[id].Seats[0].AutoPlay {
		t.Fatal("retry did not take over exactly once")
	}
}

func TestTimeoutAutoplayReclaimSanguoshaResponsePreservesPausedClock(t *testing.T) {
	s, _, clients, id := autoPlayTable(t, "sanguosha")
	r := s.rooms[id]
	for r.Game.Sanguosha.Selecting || r.Game.Sanguosha.Pending != nil {
		actor := r.Game.SanguoshaActor()
		action, err := r.Game.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		if err = r.applyGameAction(actor, action, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	owner := r.Game.Turn
	responder := (owner + 1) % len(clients)
	r.Game.Sanguosha.Players[owner].General = "zhouyu"
	r.Game.Sanguosha.Players[responder].General = "lvbu"
	r.Game.Sanguosha.Players[responder].HP = 4
	r.Game.Sanguosha.Players[responder].MaxHP = 4
	clients[owner].command(current(clients[owner]), "action", game.Action{Type: "sg_skill", Skill: "fanjian", Targets: []int{responder}}, 200)
	r = s.rooms[id]
	r.SGTimeLeft = 37000
	r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
	r.BotAt = time.Now().Add(time.Minute).UnixMilli()
	r.Seats[responder].AutoPlay = true
	r.Seats[responder].TimeoutAutoPlay = true
	prompt := r.Game.Sanguosha.Pending.ID
	setAutoPlay(clients[responder], current(clients[responder]), false, 200)
	r = s.rooms[id]
	if r.SGTimeLeft != 37000 || r.Seats[responder].AutoPlay || r.Game.Sanguosha.Pending.ID != prompt {
		t.Fatal("reclaim changed the paused action budget or pending response")
	}
	if left := time.Until(time.UnixMilli(r.TurnDeadline)); left < 19*time.Second || left > 20*time.Second {
		t.Fatal("reclaimed responder did not receive the normal 20 second window", left)
	}
	action, err := r.Game.BotAction(responder)
	if err != nil {
		t.Fatal(err)
	}
	clients[responder].command(current(clients[responder]), "action", action, 200)
	r = s.rooms[id]
	if r.Game.Sanguosha.Pending != nil || r.Game.Turn != owner {
		t.Fatal("response did not resume the original turn")
	}
	if left := time.Until(time.UnixMilli(r.TurnDeadline)); left < 36*time.Second || left > 37*time.Second {
		t.Fatal("response granted extra action time", left)
	}
}

// Mixed-control full-game scenarios deliberately return to manual HTTP play
// after exercising a timeout. Reclaim explicitly through the public command;
// never make testClient silently bypass persistent computer ownership.
func reclaimTimeoutHumans(t *testing.T, s *Server, clients []*testClient, id string) {
	t.Helper()
	actors := []int{}
	for p, seat := range s.rooms[id].Seats {
		if seat.TimeoutAutoPlay {
			if !seat.AutoPlay || seat.Bot || seat.Left {
				t.Fatal("invalid timeout-controlled human seat", p)
			}
			actors = append(actors, p)
		}
	}
	if len(actors) == 0 {
		t.Fatal("timeout did not persist human autoplay")
	}
	if s.rooms[id].Game.Finished {
		return
	}
	for _, p := range actors {
		setAutoPlay(clients[p], current(clients[p]), false, 200)
		if s.rooms[id].Seats[p].AutoPlay || s.rooms[id].Seats[p].TimeoutAutoPlay {
			t.Fatal("explicit reclaim left human under computer control", p)
		}
	}
}

func TestTimeoutAutoplayAllGamesContinueToNaturalFinish(t *testing.T) {
	for _, kind := range []string{"splendor", "rail", "catan", "carcassonne", "sanguosha", "dota"} {
		t.Run(kind, func(t *testing.T) {
			s, _, _, id := autoPlayTable(t, kind)
			host := s.rooms[id].Host
			steps := 0
			for ; !s.rooms[id].Game.Finished && steps < 8000; steps++ {
				r := s.rooms[id]
				actors := r.timeoutActors()
				if len(actors) == 0 {
					t.Fatal("unfinished game has no decision maker")
				}
				// Let each newly required human decision expire. Controlled
				// responders continue at BotAt without waiting a whole turn.
				at := time.UnixMilli(r.BotAt)
				for _, actor := range actors {
					if !r.Seats[actor].AutoPlay {
						at = time.UnixMilli(max(r.BotAt, r.TurnDeadline))
						break
					}
				}
				version := r.Version
				s.mu.Lock()
				s.expireSetups(at)
				s.runBots(at)
				s.mu.Unlock()
				r = s.rooms[id]
				if r.Version <= version || r.Host != host || !r.Rated {
					t.Fatal("timeout game stalled or changed ownership/rating", steps, r.Game.Phase)
				}
				for _, seat := range r.Seats {
					if seat.Left || seat.Bot {
						t.Fatal("automatic control replaced or removed a human")
					}
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) == 0 {
				t.Fatal("automatic game did not reach its own victory condition", steps, r.Game.Phase)
			}
			var count int
			if err := s.db.QueryRow("SELECT count(*) FROM match_history WHERE id=?", r.MatchID).Scan(&count); err != nil || count != 1 {
				t.Fatal("finished timeout game not archived exactly once", count, err)
			}
			t.Logf("%s finished after %d timeout/computer ticks", kind, steps)
		})
	}
}

func TestTimeoutAutoplayReclaimSuspendedOwnerCanResume(t *testing.T) {
	for _, remaining := range []int64{0, 37000} {
		t.Run(fmt.Sprint(remaining), func(t *testing.T) {
			s, _, clients, id := newCatanTable(t)
			r := s.rooms[id]
			r.Game.Turn = 0
			r.Game.Phase = "catan_gold"
			r.Game.Catan.Seafarers = &game.CatanSeafarers{Pirate: -1}
			r.Game.Catan.GoldPending = &game.CatanGoldPending{Claims: []game.CatanGoldClaim{{Player: 1, Count: 1}}, Resume: "catan_turn"}
			r.CatanTimeLeft = remaining
			r.Seats[0].AutoPlay, r.Seats[0].TimeoutAutoPlay = true, true
			r.startTurnClock(time.Now())
			deadline := r.TurnDeadline
			setAutoPlay(clients[0], current(clients[0]), false, 200)
			r = s.rooms[id]
			if r.TurnDeadline != deadline || r.Game.CatanPendingActor() != 1 || r.Seats[1].AutoPlay {
				t.Fatal("reclaim disturbed another player's response")
			}
			action, err := r.Game.BotAction(1)
			if err != nil {
				t.Fatal(err)
			}
			clients[1].command(current(clients[1]), "action", action, 200)
			r = s.rooms[id]
			want := remaining
			if want == 0 {
				want = turnLimit.Milliseconds()
			}
			if r.Game.Phase != "catan_turn" || r.TurnDeadline-time.Now().UnixMilli() < want-1000 || r.TurnDeadline-time.Now().UnixMilli() > want {
				t.Fatal("reclaimed owner cannot resume with correct action budget", r.Game.Phase, r.TurnDeadline-time.Now().UnixMilli(), want)
			}
			clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_end"}, 200)
		})
	}
}
