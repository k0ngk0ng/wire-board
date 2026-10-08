package server

import (
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanTwoAttackPublicRematchAndBaseReset(t *testing.T) {
	s, ts, clients, id := newTwoScenarioFullTable(t, "barbarian-attack", true)
	for _, scene := range []string{"", "barbarian-attack"} {
		clients[0].command(current(clients[0]), "close", nil, 200)
		clients[0].command(current(clients[0]), "rematch", nil, 200)
		clients[0].post("/api/rooms/"+id, map[string]any{"type": "catan_two_scenario", "catanTwoScenario": scene, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
		s, ts = restartRiversHTTP(t, s, ts, clients, id)
		for p := range 2 {
			clients[p].command(current(clients[p]), "ready", nil, 200)
		}
		clients[0].command(current(clients[0]), "start", nil, 200)
		g := s.rooms[id].Game.Catan
		if (g.Attack != nil) != (scene == "barbarian-attack") || g.EventDeck == nil {
			t.Fatal("rematch isolation")
		}
		if scene == "" && (g.Attack != nil || len(g.Bank) != 5 || len(g.DevDeck) != 25) {
			t.Fatal("base city state leak")
		}
		if scene == "barbarian-attack" && (g.Attack.TwoRules != game.CatanTwoAttackRules || len(g.Bank) != 5 || len(g.DevDeck) != 0) {
			t.Fatal("city recipe missing")
		}
	}
}

func TestCatanTwoAttackFirstNeutralClockRecovery(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newTwoScenarioFullTable(t, "barbarian-attack")
			state := s.rooms[id].Game
			for state.Catan.SetupStep < state.Catan.SetupLimit() {
				a, e := state.BotAction(state.Turn)
				if e != nil {
					t.Fatal(e)
				}
				if e = state.Apply(state.Turn, a); e != nil {
					t.Fatal(e)
				}
			}
			state.Phase = "catan_turn"
			state.Catan.Two.Rolls = []int{4, 5}
			state.Catan.RollID = 2
			state.Catan.Dice = []int{2, 3}
			actor := state.Turn
			prepareAttackHTTPCard(t, state, "knighthood")
			s.rooms[id].TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
			if e := s.save(s.rooms[id]); e != nil {
				t.Fatal(e)
			}
			clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_buy_dev"}, 200)
			r := s.rooms[id]
			deadline, left := r.TurnDeadline, r.CatanTimeLeft
			first, e := r.Game.BotAction(actor)
			if e != nil {
				t.Fatal(e)
			}
			clients[actor].command(current(clients[actor]), "action", first, 200)
			r = s.rooms[id]
			if !r.Game.Catan.Attack.Pending.Neutral || r.TurnDeadline != deadline || r.CatanTimeLeft != left {
				t.Fatal("neutral continuation reset clock")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			r = s.rooms[id]
			if r.TurnDeadline != deadline || !r.Game.Catan.Attack.Pending.Neutral {
				t.Fatal("recovery lost timer/neutral")
			}
			act, e := r.Game.BotAction(actor)
			if e != nil {
				t.Fatal(e)
			}
			clients[1-actor].command(current(clients[1-actor]), "action", act, 400)
			at := time.Now()
			switch mode {
			case "manual":
				clients[actor].command(current(clients[actor]), "action", act, 200)
			case "autoplay":
				setAutoPlay(clients[actor], current(clients[actor]), true, 200)
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
			if r.Game.Catan.Attack.Pending != nil || r.Game.Phase != "catan_turn" || len(r.Game.Catan.Attack.Knights) != 2 || len(r.Game.Catan.Attack.Discard) != 1 {
				t.Fatal("neutral response did not finish once")
			}
			if restored := r.TurnDeadline - at.UnixMilli(); restored < left || restored > left+1000 {
				t.Fatal("action clock not restored", restored, left)
			}
			if mode == "timeout" && !r.Seats[actor].TimeoutAutoPlay {
				t.Fatal("no timeout autoplay")
			}
		})
	}
}
