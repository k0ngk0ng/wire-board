package tests

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	fortressblue "github.com/k0ngk0ng/wire-board/games/fortressblue/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func runBotMatch(t *testing.T, players int, mapID string) {
	t.Helper()
	module := fortressblue.Module()
	now := time.Unix(1_800_000_000, 0)
	options, _ := json.Marshal(fortressblue.Options{Map: mapID, TurnSeconds: 20})
	m, err := module.Start(gameapi.Context{Players: players, Now: now}, options)
	if err != nil {
		t.Fatal(err)
	}
	actions := 0
	for m.Result == nil {
		actions++
		if actions > 5000 {
			t.Fatalf("%d-player %s bot game did not finish; log=%v", players, mapID, m.Log)
		}
		evolved := false
		if len(m.Actors) > 0 {
			actor := m.Actors[0]
			raw, err := module.BotAction(gameapi.Context{Players: players, Actor: actor, Now: now, Version: actions}, m)
			if err != nil {
				t.Fatalf("bot %d: %v", actor, err)
			}
			if raw != nil {
				next, err := module.Apply(gameapi.Context{Players: players, Actor: actor, Now: now, Version: actions}, m, raw)
				if err != nil {
					t.Fatalf("%d-player %s apply after %d actions: %v", players, mapID, actions, err)
				}
				if err := next.Validate(players); err != nil {
					t.Fatalf("invalid match: %v", err)
				}
				m = next
				evolved = true
			}
		}
		if m.Result == nil && m.SystemDeadline > 0 && now.UnixMilli() >= m.SystemDeadline {
			next, err := module.OnTimer(gameapi.Context{Players: players, Now: now}, m)
			if err != nil {
				t.Fatal(err)
			}
			if next.SystemDeadline != 0 && next.SystemDeadline <= now.UnixMilli() {
				t.Fatalf("timer did not advance: %+v", next)
			}
			m = next
			evolved = true
		}
		if !evolved {
			// The host would schedule the next deadline/bot; advance the clock so
			// timeouts and sudden death are exercised.
			now = now.Add(5 * time.Second)
			continue
		}
		now = now.Add(5 * time.Second)
	}
	if m.Result == nil || !m.Result.Team || len(m.Result.Ranks) != players {
		t.Fatalf("bad result: %+v", m.Result)
	}
	if m.Result.Draw {
		if len(m.Result.Winners) != 0 {
			t.Fatal("draw has winners")
		}
		for _, rank := range m.Result.Ranks {
			if rank != 1 {
				t.Fatal("draw rank")
			}
		}
	} else {
		if len(m.Result.Winners) != players/2 {
			t.Fatalf("team winners %d", len(m.Result.Winners))
		}
	}
	if err := m.Validate(players); err != nil {
		t.Fatal(err)
	}
	t.Logf("completed %d players on %s after %d actions: %s", players, mapID, actions, m.Log[len(m.Log)-1])
}

func TestAllBotGamesFinish(t *testing.T) {
	maps := []string{"hills", "canyon", "islands", "snow", "volcano"}
	for i, players := range []int{2, 4, 6, 8, 10} {
		players, mapID := players, maps[i]
		t.Run(fmt.Sprintf("%s-%d", mapID, players), func(t *testing.T) {
			runBotMatch(t, players, mapID)
		})
	}
}
