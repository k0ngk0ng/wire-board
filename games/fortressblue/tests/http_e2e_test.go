package tests

// Failure cases specified before implementation: unpublished game; wrong
// capacity/map; non-current or spectator action; stale epoch/turn; invalid
// angle/power; restart losing the draft/turn; a completed HTTP game without a
// team result/history.

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	fortressblue "github.com/k0ngk0ng/wire-board/games/fortressblue/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func state(c *gametest.Client) fortressblue.State {
	raw, err := json.Marshal(gametest.Data(c.Room()))
	if err != nil {
		panic(err)
	}
	var s fortressblue.State
	if err := json.Unmarshal(raw, &s); err != nil {
		panic(err)
	}
	return s
}

func actionOf(s fortressblue.State, kind string) map[string]any {
	return map[string]any{"type": kind, "epoch": s.Epoch}
}

func selectAll(t *testing.T, h *gametest.HTTP) fortressblue.State {
	t.Helper()
	for i := range h.People {
		s := state(h.People[0])
		if s.Tanks[i].Selected {
			continue
		}
		a := actionOf(s, "select")
		a["tank"] = 1 + i%12
		h.People[i].Action(a, 200)
	}
	s := state(h.People[0])
	if s.Phase != fortressblue.PhasePlaying {
		t.Fatalf("phase after all selections: %s", s.Phase)
	}
	return s
}

func TestFortressBlueEightSeatHTTPAndTeamResult(t *testing.T) {
	h := gametest.NewHTTP(t, 8, fortressblue.Module(), "fortressblue")
	h.AssertUnpublished()
	h.People[0].Request("POST", "/api/admin/games/fortressblue", map[string]bool{"enabled": true}, 200)
	h.People[0].Request("POST", "/api/rooms", map[string]any{"kind": "fortressblue", "name": "人数错误", "capacity": 3}, 400)
	h.Start(map[string]any{"map": "stage-16", "turnSeconds": 20})
	s := state(h.People[0])
	if s.Phase != fortressblue.PhaseSelect || len(s.Tanks) != 8 || s.MapID != "stage-16" {
		t.Fatalf("bad select state: %+v", s)
	}
	if h.Spectator.State()["room"].(map[string]any)["spectating"] != true {
		t.Fatal("spectator not watching")
	}
	a := actionOf(s, "select")
	a["tank"] = 1
	h.Spectator.Action(a, 400)
	h.People[0].Action(a, 200)
	h.People[0].Action(a, 400)
	s = state(h.People[0])
	if s.Phase != fortressblue.PhaseSelect {
		t.Fatal("draft ended early")
	}
	s = selectAll(t, h)
	if s.Current != s.Order[0] || s.Turn != 1 || s.Tanks[0].Team != fortressblue.TeamRed || s.Tanks[4].Team != fortressblue.TeamBlue {
		t.Fatalf("bad playing state: %+v", s)
	}
	other := (s.Current + 1) % len(s.Tanks)
	wrong := map[string]any{"type": "fire", "epoch": s.Epoch, "turn": s.Turn, "angle": 45, "power": 60, "facing": 1, "weapon": "primary"}
	h.People[other].Action(wrong, 400)
	stale := map[string]any{"type": "fire", "epoch": s.Epoch, "turn": s.Turn - 1, "angle": 45, "power": 60, "facing": 1, "weapon": "primary"}
	h.People[s.Current].Action(stale, 400)
	bad := map[string]any{"type": "fire", "epoch": s.Epoch, "turn": s.Turn, "angle": -1, "power": 60, "facing": 1, "weapon": "primary"}
	h.People[s.Current].Action(bad, 400)
	bad = map[string]any{"type": "fire", "epoch": s.Epoch, "turn": s.Turn, "angle": 45, "power": 101, "facing": 1, "weapon": "primary"}
	h.People[s.Current].Action(bad, 400)
	if len(s.Tanks[s.Current].Items) != fortressblue.StartingItems {
		t.Fatalf("items not assigned: %+v", s.Tanks[s.Current].Items)
	}
	h.People[s.Current].Action(map[string]any{"type": "item", "epoch": s.Epoch, "turn": s.Turn, "slot": 99}, 400)
	h.People[s.Current].Action(map[string]any{"type": "item", "epoch": s.Epoch, "turn": s.Turn, "slot": 0}, 200)
	afterItem := state(h.People[0])
	if afterItem.Turn != s.Turn || afterItem.Current != s.Current || len(afterItem.Tanks[s.Current].Items) != fortressblue.StartingItems-1 {
		t.Fatalf("HTTP item use ended turn or lost item: %+v", afterItem)
	}
	fire := map[string]any{"type": "fire", "epoch": s.Epoch, "turn": s.Turn, "angle": 45, "power": 60, "facing": 1, "weapon": "primary"}
	h.People[s.Current].Action(fire, 200)
	after := state(h.People[0])
	if after.Turn != s.Turn+1 || after.Current == s.Current || after.LastShot == nil {
		t.Fatalf("HTTP fire did not advance: %+v", after)
	}
	beforeRestart := state(h.People[0])
	h.Restart()
	restored := state(h.People[0])
	if !reflect.DeepEqual(beforeRestart, restored) {
		t.Fatal("restart changed live state")
	}
	if h.People[0].Room()["status"] != "playing" {
		t.Fatal("HTTP room did not stay in play")
	}
	h.Record("eight-seat original draft, fire and restart")
}

func TestFortressBlueHTTPReselectAndMapValidation(t *testing.T) {
	h := gametest.NewHTTP(t, 4, fortressblue.Module(), "fortressblue")
	h.AssertUnpublished()
	h.People[0].Request("POST", "/api/admin/games/fortressblue", map[string]bool{"enabled": true}, 200)
	h.People[0].Request("POST", "/api/rooms", map[string]any{"kind": "fortressblue", "name": "坏地图", "capacity": 4, "options": map[string]any{"map": "moon", "turnSeconds": 20}}, 400)
	h.Start(map[string]any{"map": "random", "turnSeconds": 20})
	s := state(h.People[0])
	if s.Phase != fortressblue.PhaseSelect || s.MapID == "" || s.MapID == "random" {
		t.Fatalf("random map not resolved: %+v", s)
	}
	s = selectAll(t, h)
	if s.Tanks[0].Team != s.Tanks[1].Team || s.Tanks[2].Team != fortressblue.TeamBlue {
		t.Fatal("2v2 team split")
	}
	h.People[s.Current].Action(map[string]any{"type": "move", "epoch": s.Epoch, "turn": s.Turn, "dir": 1}, 200)
	h.People[s.Current].Action(map[string]any{"type": "move", "epoch": s.Epoch, "turn": s.Turn, "dir": 0}, 400)
}

func resultFixture() gameapi.Module {
	module := fortressblue.Module()
	start := module.Start
	module.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		match, err := start(ctx, raw)
		if err != nil {
			return match, err
		}
		var s fortressblue.State
		if err := json.Unmarshal(match.Data, &s); err != nil {
			return match, err
		}
		for i := range s.Tanks {
			t := &s.Tanks[i]
			t.Selected, t.TankID, t.MaxHP, t.HP = true, 1, 950, 950
			t.Fuel, t.Special, t.Alive = 50, 2, true
		}
		s.Tanks[1].Alive, s.Tanks[1].HP = false, 0
		s.Phase = fortressblue.PhasePlaying
		s.Current, s.Turn, s.Order, s.Wind = 0, 1, []int{0, 1}, 0
		s.StartAt = ctx.Now.UnixMilli()
		s.SuddenAt = s.StartAt + 8*60*1000
		s.MatchEndsAt = s.StartAt + 12*60*1000
		s.Deadline = ctx.Now.Add(20 * time.Second).UnixMilli()
		match.Data, _ = json.Marshal(s)
		match.Actors = []int{0}
		match.Attention = []int{0}
		match.TimeoutActors = []int{0}
		match.Deadline = s.Deadline
		match.SystemDeadline = s.SuddenAt
		match.Eliminated = []int{1}
		return match, nil
	}
	return module
}

func TestFortressBlueHTTPResultAndHistory(t *testing.T) {
	h := gametest.NewHTTP(t, 2, resultFixture(), "fortressblue")
	h.Start(map[string]any{"map": "stage-16", "turnSeconds": 20})
	s := state(h.People[0])
	if s.Phase != fortressblue.PhasePlaying {
		t.Fatalf("fixture did not start: %+v", s)
	}
	h.People[0].Action(map[string]any{
		"type": "fire", "epoch": s.Epoch, "turn": 1, "angle": 45, "power": 60, "facing": 1, "weapon": "primary",
	}, 200)
	room := h.People[0].Room()
	if room["status"] != "finished" {
		t.Fatalf("fixture did not finish: %v", room)
	}
	result := room["result"].(map[string]any)
	players := result["players"].([]any)
	if result["draw"] == true || len(players) != 2 || players[0].(map[string]any)["won"] != true || players[1].(map[string]any)["won"] != false {
		t.Fatalf("bad result: %v", result)
	}
	me := h.People[0].State()["user"].(map[string]any)["id"].(string)
	profile := h.People[0].Request("GET", "/api/players/"+me, nil, 200)
	if profile["history"] == nil {
		t.Fatalf("history missing: %v", profile)
	}
	h.Record("natural team result and history via HTTP")
}
