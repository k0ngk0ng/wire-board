package snooker_test

// Failure inventory, before clock implementation: optional voters time out with
// the shooter; cue placement/votes renew the clock; ball flight consumes human
// time; ready players keep expiring; reclaim renews teammates; saved deadlines
// reset on restart; legacy matches never acquire a clock; migration loses balls.
import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestSnookerDecisionClocksAndReclaim(t *testing.T) {
	mod, ctx, m := begin(t, 4, 3)
	if m.Deadline != ctx.Now.Add(120*time.Second).UnixMilli() || !reflect.DeepEqual(m.TimeoutActors, []int{0}) {
		t.Fatal("missing active-seat 120 second clock", m.Deadline, m.TimeoutActors)
	}
	initial := m.Deadline
	ctx.Now = ctx.Now.Add(25 * time.Second)
	m = action(t, mod, ctx, m, map[string]any{"type": "place", "x": game.BaulkX - .15, "y": game.TableHeight / 2})
	if m.Deadline != initial {
		t.Fatal("placement extended clock")
	}
	ctx.Actor = 2
	m = action(t, mod, ctx, m, map[string]any{"type": "rerack"})
	if m.Deadline != initial || !reflect.DeepEqual(m.TimeoutActors, []int{0}) {
		t.Fatal("optional teammate vote affected shooter clock")
	}
	ctx.Now = time.UnixMilli(initial + 1)
	unchanged, e := mod.Reclaim(ctx, m)
	if e != nil || !reflect.DeepEqual(unchanged, m) {
		t.Fatal("inactive teammate renewed shooter clock", e)
	}
	ctx.Actor = 0
	m, e = mod.Reclaim(ctx, m)
	if e != nil || m.Deadline != ctx.Now.Add(120*time.Second).UnixMilli() {
		t.Fatal("expired player did not get fresh time", e)
	}
	before := state(t, m)
	m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": 0, "power": .06})
	if m.Deadline != 0 || len(m.TimeoutActors) != 0 || m.SystemDeadline <= 0 {
		t.Fatal("rolling charges player time")
	}
	ctx.Now = time.UnixMilli(m.SystemDeadline - 1)
	m = finishStroke(t, mod, ctx, m)
	if m.Deadline != ctx.Now.Add(time.Millisecond+120*time.Second).UnixMilli() {
		t.Fatal("settle failed to start decision time")
	}
	ctx.Now = ctx.Now.Add(time.Millisecond)
	ctx.Actor = state(t, m).Turn
	m = action(t, mod, ctx, m, map[string]any{"type": "foul_choice", "choice": "restore"})
	if state(t, m).Turn != before.Turn || m.Deadline != ctx.Now.Add(120*time.Second).UnixMilli() {
		t.Fatal("restored offender lacks time")
	}
	// Labelled black-only fixture reaches simultaneous between-frame readiness.
	m = onlyBlack(t, m, [2]int{})
	ctx.Actor = 0
	m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": -1.5707963267948966, "power": .25})
	ctx.Now = time.UnixMilli(m.SystemDeadline - 1)
	m = finishStroke(t, mod, ctx, m)
	if state(t, m).Phase != "frame_end" || len(m.TimeoutActors) != 4 {
		t.Fatal("missing next-frame clocks")
	}
	allDue := m.Deadline
	ctx.Now = time.UnixMilli(allDue - 10000)
	ctx.Actor = 0
	m = action(t, mod, ctx, m, map[string]any{"type": "ready"})
	if m.Deadline != allDue || !reflect.DeepEqual(m.TimeoutActors, []int{1, 2, 3}) {
		t.Fatal("ready altered other seats' clock", m.TimeoutActors)
	}
	ctx.Now = time.UnixMilli(allDue + 1)
	ctx.Actor = 1
	m, e = mod.Reclaim(ctx, m)
	if e != nil || m.Deadline != allDue || !reflect.DeepEqual(m.TimeoutActors, []int{2, 3}) {
		t.Fatal("reclaim renewed other seats", e, m.TimeoutActors)
	}
	trace(t, m)
}

func TestSnookerTimeoutAutoplayHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 4, game.Module(), "snooker")
	h.Start(map[string]any{"bestOf": 1})
	room := h.People[0].Room()
	s := publicState(t, room)
	actor := s.Turn
	deadline, ok := room["turnDeadline"].(float64)
	if !ok || deadline <= float64(time.Now().UnixMilli()) {
		t.Fatal("room has no takeover deadline")
	}
	h.Record("120 second clock before inactivity")
	h.Restart()
	if h.People[0].Room()["turnDeadline"] != deadline {
		t.Fatal("restart replenished live deadline")
	}
	// Real production clock: no shortened timeout or synthetic takeover request.
	time.Sleep(time.Until(time.UnixMilli(int64(deadline))) + 300*time.Millisecond)
	until := time.Now().Add(8 * time.Second)
	for time.Now().Before(until) {
		room = h.People[0].Room()
		if room["seats"].([]any)[actor].(map[string]any)["autoPlay"] == true {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	for p, seat := range room["seats"].([]any) {
		auto := seat.(map[string]any)["autoPlay"] == true
		if auto != (p == actor) {
			t.Fatal("timeout controlled wrong seat", p, actor, seat)
		}
	}
	h.Record("expired decision automatically controlled only the responsible seat")
	h.People[actor].Command("autoplay", map[string]any{"enabled": false}, 200)
	room = h.People[actor].Room()
	if room["seats"].([]any)[actor].(map[string]any)["autoPlay"] == true {
		t.Fatal("could not reclaim")
	}
	s = publicState(t, room)
	if s.Phase == "aim" && s.Turn == actor {
		due := room["turnDeadline"].(float64)
		if due < float64(time.Now().Add(100*time.Second).UnixMilli()) {
			t.Fatal("reclaim left player expired")
		}
		h.People[actor].Action(map[string]any{"type": "shoot", "window": s.Window, "angle": 0, "power": .06}, 200)
		if value := h.People[actor].Room()["turnDeadline"]; value != nil && value != float64(0) {
			t.Fatal("rolling retained takeover clock")
		}
	}
	h.Record("human cancelled autoplay and retained the match")
}

func TestSnookerLegacyClockMigrationHTTP(t *testing.T) {
	legacy := game.Module()
	start := legacy.Start
	legacy.Start = func(c gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		m, e := start(c, raw)
		if e != nil {
			return m, e
		}
		var data map[string]any
		_ = json.Unmarshal(m.Data, &data)
		delete(data, "due")
		m.Data, _ = json.Marshal(data)
		m.Deadline = 0
		m.TimeoutActors = nil
		return m, nil
	}
	h := gametest.NewHTTP(t, 2, legacy, "snooker")
	h.Start(nil)
	before := publicState(t, h.People[0].Room())
	h.Config.Modules = []gameapi.Module{game.Module()}
	h.Restart()
	room := h.People[0].Room()
	after := publicState(t, room)
	if room["turnDeadline"] == nil || room["turnDeadline"].(float64) < float64(time.Now().Add(100*time.Second).UnixMilli()) || !reflect.DeepEqual(before.Balls, after.Balls) || before.Window != after.Window || before.Turn != after.Turn || before.Scores != after.Scores {
		t.Fatal("legacy migration missing clock or changed game")
	}
	due := room["turnDeadline"]
	h.Restart()
	if h.People[0].Room()["turnDeadline"] != due {
		t.Fatal("migration repeated on restart")
	}
	h.Record("legacy match receives one fresh deadline, second restore preserves it")
}
