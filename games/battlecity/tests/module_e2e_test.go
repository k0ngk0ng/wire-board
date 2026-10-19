package battlecity_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/battlecity/maps"
	battlecity "github.com/k0ngk0ng/wire-board/games/battlecity/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func runBotMatch(t *testing.T, players int, mapID string) gameapi.Result {
	t.Helper()
	mod := battlecity.Module()
	start := time.UnixMilli(1_700_000_000_000)
	raw, _ := json.Marshal(battlecity.Options{Map: mapID})
	m, err := mod.Start(gameapi.Context{Players: players, Now: start}, raw)
	if err != nil {
		t.Fatal(err)
	}
	now := start
	for step := 0; step < 7000 && m.Result == nil; step++ {
		viewAny, err := mod.View(m, 0)
		if err != nil {
			t.Fatal(err)
		}
		v := viewAny.(battlecity.View)
		switch v.Phase {
		case battlecity.PhaseSelect:
			for seat := 0; seat < players; seat++ {
				act, err := mod.BotAction(gameapi.Context{Actor: seat, Players: players, Now: now}, m)
				if err != nil {
					t.Fatal(err)
				}
				if len(act) == 0 {
					continue
				}
				m, err = mod.Apply(gameapi.Context{Actor: seat, Players: players, Now: now}, m, act)
				if err != nil {
					t.Fatalf("select seat %d: %v (%s)", seat, err, act)
				}
			}
			// All bots select immediately; the server is now in countdown.
			now = now.Add(3 * time.Second)
			m, err = mod.OnTimer(gameapi.Context{Now: now}, m)
			if err != nil {
				t.Fatal(err)
			}
		case battlecity.PhaseCountdown:
			now = time.UnixMilli(v.StartAt)
			m, err = mod.OnTimer(gameapi.Context{Now: now}, m)
			if err != nil {
				t.Fatal(err)
			}
		case battlecity.PhasePlaying:
			now = time.UnixMilli(v.ServerAt + 100)
			for seat := 0; seat < players; seat++ {
				act, err := mod.BotAction(gameapi.Context{Actor: seat, Players: players, Now: now}, m)
				if err != nil {
					t.Fatal(err)
				}
				if len(act) == 0 {
					continue
				}
				m, err = mod.Apply(gameapi.Context{Actor: seat, Players: players, Now: now}, m, act)
				if err != nil {
					t.Fatalf("input seat %d: %v (%s)", seat, err, act)
				}
			}
			m, err = mod.OnTimer(gameapi.Context{Now: now}, m)
			if err != nil {
				t.Fatal(err)
			}
		case battlecity.PhaseFinished:
			break
		}
	}
	if m.Result == nil {
		t.Fatalf("%s/%d: match did not finish", mapID, players)
	}
	t.Logf("%s/%d: draw=%v winners=%v scores=%v details=%s", mapID, players, m.Result.Draw, m.Result.Winners, m.Result.Scores, m.Result.Details)
	return *m.Result
}

func TestBotMatchOneMap(t *testing.T) {
	for _, players := range []int{2, 4, 6} {
		r := runBotMatch(t, players, "classic-01")
		checkResult(t, players, r)
	}
}

func checkResult(t *testing.T, players int, r gameapi.Result) {
	t.Helper()
	if len(r.Ranks) != players || len(r.Scores) != players {
		t.Fatalf("%d: result sizes %d/%d", players, len(r.Ranks), len(r.Scores))
	}
	if !r.Team {
		t.Fatalf("%d: result is not team", players)
	}
	if r.Draw {
		if len(r.Winners) != 0 {
			t.Fatalf("%d: draw winners %v", players, r.Winners)
		}
		return
	}
	if len(r.Winners) != players/2 {
		t.Fatalf("%d: winners %v", players, r.Winners)
	}
	for _, w := range r.Winners {
		if r.Ranks[w] != 1 {
			t.Fatalf("%d: winner rank %d", players, r.Ranks[w])
		}
	}
}

func TestBotMatchesAllMaps(t *testing.T) {
	if raceEnabled {
		t.Skip("full map matrix runs without -race; race covers the focused matches")
	}
	for _, m := range maps.List() {
		for _, players := range []int{2, 4, 6} {
			r := runBotMatch(t, players, m.ID)
			checkResult(t, players, r)
			if r.Draw {
				t.Logf("%s/%d ended in a draw", m.ID, players)
			}
		}
	}
}
