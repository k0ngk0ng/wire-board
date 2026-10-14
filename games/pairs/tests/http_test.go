package pairs_test

import (
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/pairs/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"testing"
	"time"
)

func public(r map[string]any) game.View {
	raw, _ := json.Marshal(gametest.Data(r))
	var v game.View
	_ = json.Unmarshal(raw, &v)
	return v
}
func action(v game.View, p int, kind string) map[string]any {
	a := map[string]any{"type": kind, "round": v.Round, "revision": v.Players[p].Revision}
	if kind == "match" {
		pair := game.FindMove(*v.Players[p].Board)
		if pair != nil {
			a["a"] = pair[0]
			a["b"] = pair[1]
		}
	}
	return a
}
func waitPlay(t *testing.T, c *gametest.Client) game.View {
	t.Helper()
	until := time.Now().Add(6 * time.Second)
	for time.Now().Before(until) {
		v := public(c.Room())
		if v.Phase == "playing" {
			return v
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("countdown did not end")
	return game.View{}
}
func TestPairsRaceHTTP(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := gametest.NewHTTP(t, n, game.Module(), "pairs")
			h.AssertUnpublished()
			h.Start(map[string]any{})
			admin := h.People[0]
			v := public(admin.Room())
			if v.Players[0].Board != nil {
				t.Fatal("pre-start board revealed")
			}
			admin.Action(map[string]any{"type": "hint", "round": v.Round, "revision": 0}, 400)
			v = waitPlay(t, admin)
			for p := 1; p < n; p++ {
				if !reflect.DeepEqual(v.Players[0].Board, v.Players[p].Board) {
					t.Fatal("different random layouts")
				}
			}
			// Prepare all requests from the SAME room version, then submit them as real users.
			requests := []map[string]any{}
			for p, c := range h.People {
				requests = append(requests, c.ActionRequest(action(v, p, "match")))
			}
			id := admin.Room()["id"].(string)
			for p, c := range h.People {
				c.Request("POST", "/api/rooms/"+id, requests[p], 200)
			}
			original := public(admin.Room())
			admin.Request("POST", "/api/rooms/"+id, requests[0], 200)
			if !reflect.DeepEqual(original, public(admin.Room())) {
				t.Fatal("same nonce replayed")
			}
			requests[0]["nonce"] = "another-old-request"
			admin.Request("POST", "/api/rooms/"+id, requests[0], 409)
			h.Spectator.Action(action(public(h.Spectator.Room()), 0, "match"), 400)
			admin.Action(action(public(admin.Room()), 0, "hint"), 200)
			admin.Action(action(public(admin.Room()), 0, "shuffle"), 200)
			before := public(admin.Room())
			h.Restart()
			if !reflect.DeepEqual(before, public(admin.Room())) {
				t.Fatal("restore changed board, tools or deadlines")
			}
			h.Record("identical starts; concurrent version requests; tools and SQLite restore")
			steps := 0
			for public(admin.Room()).Phase != "finished" && steps < 100 {
				v = public(admin.Room())
				admin.Action(action(v, 0, "match"), 200)
				steps++
			}
			v = public(admin.Room())
			if v.Phase != "finished" || v.Winner != 0 || v.Players[0].Remaining != 0 || v.FinishAt < v.StartAt {
				t.Fatal("not first-to-clear", v.Phase)
			}
			admin.Action(action(v, 0, "hint"), 400)
			h.Record("natural complete race; first clear wins")
			h.Restart()
			if !reflect.DeepEqual(v, public(admin.Room())) {
				t.Fatal("completed result changed")
			}
		})
	}
}
func TestPairsComputerAndTakeoverHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 1, game.Module(), "pairs")
	c := h.People[0]
	c.Request("POST", "/api/admin/games/pairs", map[string]bool{"enabled": true}, 200)
	c.Request("POST", "/api/rooms", map[string]any{"kind": "pairs", "name": "连连看电脑验收", "capacity": 2}, 201)
	c.Command("add_bot", nil, 200)
	c.Command("ready", nil, 200)
	c.Command("start", nil, 200)
	c.Command("autoplay", map[string]any{"enabled": true}, 200)
	until := time.Now().Add(12 * time.Second)
	for time.Now().Before(until) {
		v := public(c.Room())
		if v.Players[0].Revision > 0 && v.Players[1].Revision > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	c.Command("autoplay", map[string]any{"enabled": false}, 200)
	v := public(c.Room())
	if v.Players[0].Revision == 0 || v.Players[1].Revision == 0 {
		t.Fatal("platform bot scheduler did not play both seats")
	}
	c.Action(action(v, 0, "match"), 200)
	h.Record("human took over after both computer seats played")
	c.Command("autoplay", map[string]any{"enabled": true}, 200)
	h.Restart()
	until = time.Now().Add(160 * time.Second)
	for time.Now().Before(until) {
		v = public(c.Room())
		if v.Phase == "finished" {
			h.Record("complete natural computer race with restart")
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("computer race did not finish")
}

func TestPairsTimeoutHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "pairs")
	h.Start(map[string]any{})
	c := h.People[0]
	v := waitPlay(t, c)
	startAt := v.StartAt
	due := v.Players[0].Due
	until := time.UnixMilli(due).Add(8 * time.Second)
	// Keep seat 1 active once, so reclaiming seat 0 must not touch its later clock.
	time.Sleep(time.Second)
	h.People[1].Action(action(public(h.People[1].Room()), 1, "hint"), 200)
	for time.Now().Before(until) {
		r := c.Room()
		if r["seats"].([]any)[0].(map[string]any)["autoPlay"] == true {
			before := public(r)
			c.Command("autoplay", map[string]any{"enabled": false}, 200)
			after := public(c.Room())
			if after.StartAt != startAt || after.Players[1].Due != before.Players[1].Due || after.Players[0].Revision != before.Players[0].Revision {
				t.Fatal("timeout reclaim changed race or opponent")
			}
			c.Action(action(after, 0, "match"), 200)
			h.Record("real 120 second inactivity triggered autoplay; reclaimed and matched")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("real inactivity timeout did not enable autoplay")
}

// Directed HTTP fixtures select a naturally generated obstacle board, without
// changing any action validation, board cell, tool count or host clock.
func TestPairsObstacleToolsHTTP(t *testing.T) {
	for _, mechanic := range []string{"stone", "crate", "ice"} {
		t.Run(mechanic, func(t *testing.T) {
			mod := game.Module()
			original := mod.Start
			mod.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
				for i := 0; i < 500; i++ {
					candidate := ctx
					candidate.Now = ctx.Now.Add(time.Duration(i) * time.Nanosecond)
					m, e := original(candidate, raw)
					if e != nil {
						return m, e
					}
					v, e := mod.View(m, 0)
					if e != nil {
						return m, e
					}
					if v.(game.View).Mechanic == mechanic {
						return m, nil
					}
				}
				return gameapi.Match{}, fmt.Errorf("could not generate %s fixture", mechanic)
			}
			h := gametest.NewHTTP(t, 1, mod, "pairs")
			h.Start(map[string]any{})
			c := h.People[0]
			v := waitPlay(t, c)
			ordinary := -1
			for i, cell := range v.Players[0].Board.Cells {
				if cell != nil && cell.Kind == "tile" && !cell.Ice {
					ordinary = i
					break
				}
			}
			invalid := action(v, 0, "hammer")
			invalid["a"] = ordinary
			c.Action(invalid, 400)
			if public(c.Room()).Players[0].Tools.Hammer != 2 {
				t.Fatal("invalid target charged a hammer")
			}
			for use := 0; use < 2; use++ {
				v = public(c.Room())
				target := -1
				for i, cell := range v.Players[0].Board.Cells {
					if cell != nil && (cell.Kind != "tile" || cell.Ice) {
						target = i
						break
					}
				}
				if target < 0 {
					t.Fatal("fixture missing obstacles")
				}
				before := v.Players[0].Board.Cells[target]
				a := action(v, 0, "hammer")
				a["a"] = target
				c.Action(a, 200)
				after := public(c.Room())
				if after.Players[0].Tools.Hammer != 1-use || after.Players[0].Remaining != v.Players[0].Remaining {
					t.Fatal("hammer changed tile count or tools")
				}
				cell := after.Players[0].Board.Cells[target]
				if before.Kind == "tile" {
					if cell == nil || cell.Ice || cell.Icon != before.Icon {
						t.Fatal("ice did not thaw")
					}
				} else if cell != nil {
					t.Fatal("obstacle not removed")
				}
			}
			v = public(c.Room())
			a := action(v, 0, "hammer")
			a["a"] = ordinary
			c.Action(a, 400)
			for use := 0; use < 3; use++ {
				c.Action(action(public(c.Room()), 0, "hint"), 200)
			}
			c.Action(action(public(c.Room()), 0, "hint"), 400)
			for use := 0; use < 2; use++ {
				c.Action(action(public(c.Room()), 0, "shuffle"), 200)
			}
			c.Action(action(public(c.Room()), 0, "shuffle"), 400)
			h.Record("obstacle tool effects and exhausted tool rejection")
		})
	}
}
