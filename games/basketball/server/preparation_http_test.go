package basketball_test

import (
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"testing"
)

// Failures are listed in PREPARATION_SELECTION.md before implementation.
func TestBasketballHTTPVisualPreparation(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "basketball")
	a, b := h.People[0], h.People[1]
	a.Request("POST", "/api/admin/games/basketball", map[string]bool{"enabled": true}, 200)
	room := a.Request("POST", "/api/rooms", map[string]any{"kind": "basketball", "name": "球场与阵容", "capacity": 2, "options": map[string]any{"mode": "versus"}}, 201)
	b.Request("POST", "/api/rooms/"+room["id"].(string), map[string]any{"type": "join", "version": a.Room()["version"], "nonce": "prep-join-00001"}, 200)
	act := func(c *gametest.Client, action map[string]any, status int) {
		c.Command("prepare", map[string]any{"action": action}, status)
	}
	a.Command("ready", nil, 200)
	b.Command("ready", nil, 200)
	before := a.Room()["options"]
	act(b, map[string]any{"type": "court", "court": "court-0"}, 400)
	act(b, map[string]any{"type": "character", "actor": 0, "character": 1}, 400)
	h.Spectator.Request("POST", "/api/rooms/"+room["id"].(string), map[string]any{"type": "prepare", "version": a.Room()["version"], "nonce": "preparation-observer-001", "action": map[string]any{"type": "character", "actor": 2, "character": 1}}, 400)
	act(a, map[string]any{"type": "character", "actor": 9, "character": 1}, 400)
	if !reflect.DeepEqual(before, a.Room()["options"]) {
		t.Fatal("rejected preparation changed options")
	}
	act(b, map[string]any{"type": "character", "actor": 2, "character": 1}, 200)
	r := a.Room()
	chars := r["options"].(map[string]any)["characters"]
	if !reflect.DeepEqual(chars, []any{float64(0), float64(1), float64(1), float64(0)}) {
		t.Fatal("wrong character swap", chars)
	}
	for _, seat := range r["seats"].([]any) {
		if seat.(map[string]any)["ready"] == true {
			t.Fatal("old ready retained")
		}
	}
	act(a, map[string]any{"type": "court", "court": "court-4"}, 200)
	act(a, map[string]any{"type": "court", "court": "court-3"}, 200)
	act(a, map[string]any{"type": "court", "court": "court-0"}, 200)
	h.Restart()
	r = a.Room()
	if r["options"].(map[string]any)["court"] != "court-0" || !reflect.DeepEqual(chars, r["options"].(map[string]any)["characters"]) {
		t.Fatal("restart lost selections")
	}
	a.Command("ready", nil, 200)
	b.Command("ready", nil, 200)
	a.Command("start", nil, 200)
	v := view(t, a)
	if v.Runtime.Config.Match.Court != 0 || v.Runtime.Config.Match.Characters != ([4]uint8{0, 1, 1, 0}) || v.Runtime.Config.Match.ControllerAssignments != ([4]int8{0, -1, 2, -1}) {
		t.Fatal("chosen roster/court not used", v.Runtime.Config.Match)
	}
	act(a, map[string]any{"type": "court", "court": "new-york"}, 400)
	h.Record("prepared visible court and independent player roster persist and initialize original match")
}
