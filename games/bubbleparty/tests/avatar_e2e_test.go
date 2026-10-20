package bubbleparty_test

// Failure inventory written before implementation: realtime-efficiency/MOTION.md.
import (
	bubbleparty "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"testing"
	"time"
)

func avatarData(c *gametest.Client) map[string]any { return gametest.Data(c.Room()) }
func TestBubblepartyAvatarChoicePreservesGameAndRestores(t *testing.T) {
	for _, n := range []int{2, 4, 6, 8} {
		t.Run(string(rune('0'+n)), func(t *testing.T) {
			h := gametest.NewHTTP(t, n, bubbleparty.Module(), "bubbleparty")
			h.Start(map[string]any{"map": "community10", "roundSeconds": 300})
			before := avatarData(h.People[0])
			epoch := before["epoch"]
			for seat, c := range h.People {
				result := c.Action(map[string]any{"type": "avatar", "epoch": epoch, "avatar": (seat + 1) % 4}, 200)
				if result["durable"] != true {
					t.Fatal("avatar choice not persisted", result)
				}
			}
			after := avatarData(h.People[0])
			for _, key := range []string{"epoch", "startAt", "matchEndsAt", "cells"} {
				if !reflect.DeepEqual(after[key], before[key]) {
					t.Fatal("appearance changed match", key)
				}
			}
			for seat, p := range after["players"].([]any) {
				actual := p.(map[string]any)
				old := before["players"].([]any)[seat].(map[string]any)
				if actual["avatar"] != float64((seat+1)%4) {
					t.Fatal("wrong appearance", actual)
				}
				for _, key := range []string{"team", "x", "y", "px", "py", "seq", "speedMs", "maxBombs", "range", "alive", "trappedUntil"} {
					if !reflect.DeepEqual(actual[key], old[key]) {
						t.Fatal("avatar changed gameplay", seat, key, actual[key], old[key])
					}
				}
			}
			h.Restart()
			restored := avatarData(h.People[0])
			for seat, p := range restored["players"].([]any) {
				if p.(map[string]any)["avatar"] != float64((seat+1)%4) {
					t.Fatal("restart lost appearance", seat, p)
				}
			}
			h.People[0].Command("autoplay", map[string]any{"enabled": true}, 200)
			h.People[0].Action(map[string]any{"type": "avatar", "epoch": epoch, "avatar": 3}, 400)
			h.People[0].Command("autoplay", map[string]any{"enabled": false}, 200)
			if avatarData(h.People[0])["players"].([]any)[0].(map[string]any)["avatar"] != float64(1) {
				t.Fatal("autoplay reset appearance")
			}
			waitPlaying(t, h.People[0], 6*time.Second)
			h.People[0].Action(map[string]any{"type": "avatar", "epoch": epoch, "avatar": 2}, 200)
			h.Record("all seats select durable cosmetic-only appearance; restart and manual control preserve selection")
		})
	}
}
func TestBubblepartyAvatarChoiceRejectsForgeryAndSpectators(t *testing.T) {
	h := gametest.NewHTTP(t, 2, bubbleparty.Module(), "bubbleparty")
	h.Start(map[string]any{"map": "community10", "roundSeconds": 300})
	c := h.People[0]
	before := avatarData(c)
	epoch := before["epoch"]
	for _, action := range []map[string]any{
		{"type": "avatar", "epoch": epoch, "avatar": -1},
		{"type": "avatar", "epoch": epoch, "avatar": 4},
		{"type": "avatar", "epoch": "old-match", "avatar": 1},
		{"type": "avatar", "epoch": epoch, "avatar": 1, "team": 1},
		{"type": "avatar", "epoch": epoch, "avatar": 1, "seat": 1},
		{"type": "avatar", "epoch": epoch, "avatar": 1, "move": "right", "seq": 1},
	} {
		c.Action(action, 400)
	}
	h.Spectator.Action(map[string]any{"type": "avatar", "epoch": epoch, "avatar": 1}, 400)
	after := avatarData(c)
	for i, p := range after["players"].([]any) {
		if !reflect.DeepEqual(p, before["players"].([]any)[i]) {
			t.Fatal("rejected avatar action mutated player", i)
		}
	}
	h.Record("invalid appearance, mixed movement, seat/team forgery, old match and spectator refused")
}
