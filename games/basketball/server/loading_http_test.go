package basketball_test

import (
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"testing"
	"time"
)

// Real authentication, SQLite restarts, host timers and seat ownership. These
// regressions complement the browser's delayed transfers and actual progress UI.
func TestBasketballRealtimeLoadingHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 2, observedModule(t), "basketball")
	h.Start(map[string]any{"mode": "versus"})
	record := evidence(t)
	a, b := h.People[0], h.People[1]
	v := view(t, a)
	epoch := v.Epoch
	reject(t, h.Spectator, map[string]any{"type": "loaded", "epoch": epoch})
	reject(t, a, map[string]any{"type": "loaded", "epoch": "stale"})
	a.Action(map[string]any{"type": "loaded", "epoch": epoch}, 200)
	h.Restart()
	time.Sleep(250 * time.Millisecond)
	v = view(t, a)
	if v.Phase != "loading" || v.Runtime.VideoFrames != 0 || !v.Loading.Ready[0] || v.Loading.Ready[2] {
		t.Fatal("persisted partial readiness changed", v.Phase)
	}
	record("partial readiness survives SQLite restart; spectator cannot unblock", v)
	b.Command("autoplay", map[string]any{"enabled": true}, 200)
	v = await(t, a, func(v game.View) bool { return v.Runtime.Playing != nil })
	if !v.Inputs[2].Automated {
		t.Fatal("explicit autoplay did not use original computer")
	}
	frame := v.Runtime.VideoFrames
	b.Command("autoplay", map[string]any{"enabled": false}, 200)
	b.Action(map[string]any{"type": "loaded", "epoch": epoch}, 200)
	v = view(t, b)
	if v.Phase == "loading" || v.Epoch != epoch || v.Runtime.VideoFrames < frame || !v.CanInput {
		t.Fatal("late takeover/reconnect reset running game")
	}
	record("explicit autoplay releases barrier; takeover never reacquires it", v)
}
