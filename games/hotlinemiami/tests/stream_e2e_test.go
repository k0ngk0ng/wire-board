package hotlinemiami_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/hotlinemiami/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestCursorProjectionAndCatchup(t *testing.T) {
	module := game.Module()
	ctx := gameapi.Context{Players: 4, Actor: 0, Now: time.Unix(100000, 0)}
	match, err := module.Start(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	before := sha256.Sum256(match.Data)
	first, err := module.StreamView(match, 0, "")
	if err != nil || first.Cursor == "" {
		t.Fatal(err)
	}
	second, err := module.StreamView(match, 0, first.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	var repeated game.View
	if json.Unmarshal(encode(second.Data), &repeated) != nil || len(repeated.Events) != 0 || repeated.Map != nil {
		t.Fatal("cursor replayed events or map")
	}
	if before != sha256.Sum256(match.Data) {
		t.Fatal("projection mutated authoritative data")
	}
	resync, err := module.StreamView(match, 1, "different-epoch:0:0")
	if err != nil {
		t.Fatal(err)
	}
	var recovered game.View
	if json.Unmarshal(encode(resync.Data), &recovered) != nil || !recovered.Recover || recovered.Map == nil || recovered.CanAct != true {
		t.Fatal("cursor epoch gap did not recover")
	}
	ctx.Now = ctx.Now.Add(500 * time.Millisecond)
	match, err = module.OnTimer(ctx, match)
	if err != nil || match.SystemDeadline <= ctx.Now.UnixMilli() {
		t.Fatal("late server clock did not catch up", err)
	}
	state := view(t, module, match, 0)
	if integer(state["frame"]) != 30 {
		t.Fatal("late server clock dropped native steps", state["frame"])
	}
	unchanged := sha256.Sum256(match.Data)
	match, err = module.OnTimer(ctx, match)
	if err != nil || sha256.Sum256(match.Data) != unchanged {
		t.Fatal("duplicate timer advanced twice")
	}
	artifact(t, map[string]any{"passed": true, "firstProjectionBytes": len(encode(first.Data)), "repeated": repeated, "recoveryMap": recovered.Map.ID, "catchupFrame": state["frame"]})
}

func TestParallelRoomBudget(t *testing.T) {
	module := game.Module()
	type runningRoom struct {
		match  gameapi.Match
		ctx    gameapi.Context
		cursor string
	}
	rooms := make([]runningRoom, 8)
	for index := range rooms {
		ctx := gameapi.Context{Players: 4, Now: time.Unix(100000, 0), Automated: []bool{true, true, true, true}}
		match, err := module.Start(ctx, encode(map[string]any{"map": []string{"no-talk", "full-house", "push-it"}[index%3]}))
		if err != nil {
			t.Fatal(err)
		}
		rooms[index] = runningRoom{match: match, ctx: ctx}
	}
	samples := make([]int64, 0, 1600)
	bytes, frames := 0, 0
	for step := 1; step <= 200; step++ {
		for index := range rooms {
			room := &rooms[index]
			room.ctx.Now = time.Unix(100000, 0).Add(time.Duration((step*2000+59)/60) * time.Millisecond)
			start := time.Now()
			for seat := range 4 {
				room.ctx.Actor = seat
				action, err := module.BotAction(room.ctx, room.match)
				if err != nil {
					t.Fatal(err)
				}
				room.match, err = module.Apply(room.ctx, room.match, action)
				if err != nil {
					t.Fatal(err)
				}
			}
			room.ctx.Actor = -1
			var err error
			room.match, err = module.OnTimer(room.ctx, room.match)
			if err != nil {
				t.Fatal(err)
			}
			packet, err := module.StreamView(room.match, 0, room.cursor)
			if err != nil {
				t.Fatal(err)
			}
			room.cursor = packet.Cursor
			bytes += len(encode(packet.Data))
			frames++
			samples = append(samples, time.Since(start).Microseconds())
		}
	}
	slices.Sort(samples)
	p95, p99 := samples[len(samples)*95/100], samples[len(samples)*99/100]
	if p95 > 8000 {
		t.Fatalf("eight-room simulation callback P95 exceeds frozen 8ms budget: %d us", p95)
	}
	artifact(t, map[string]any{"passed": true, "scope": "8 production module instances, 4 ordinary computer inputs per room, fixed 60Hz rule steps/30Hz publication, host transport measured separately by browser acceptance", "rooms": 8, "roomFrames": frames, "callbackP95Micros": p95, "callbackP99Micros": p99, "projectionBytes": bytes, "averageProjectionBytes": bytes / frames, "note": fmt.Sprintf("%d samples", len(samples))})
}
