package racing_test

// Failure scenarios are specified in ../CONTRACT.md before implementation.
// These are production Module flows, not a duplicate physics or lap algorithm.
import (
	"bytes"
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/racing/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func raw(v any) json.RawMessage {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func begin(t *testing.T, n, laps int, track string) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	mod := game.Module()
	ctx := gameapi.Context{Players: n, Now: time.Unix(100000, 0)}
	m, e := mod.Start(ctx, raw(map[string]any{"track": track, "laps": laps}))
	if e != nil {
		t.Fatal(e)
	}
	if e = mod.ValidateMatch(m, n); e != nil {
		t.Fatal(e)
	}
	return mod, ctx, m
}
func view(t *testing.T, mod gameapi.Module, m gameapi.Match, seat int) game.View {
	t.Helper()
	v, e := mod.View(m, seat)
	if e != nil {
		t.Fatal(e)
	}
	var out game.View
	if e = json.Unmarshal(raw(v), &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func trace(t *testing.T, v any) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", ".local", "racing", "module")
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	if e := os.WriteFile(filepath.Join(dir, filepath.Base(t.Name())+".json"), b, 0644); e != nil {
		t.Fatal(e)
	}
}
func tick(t *testing.T, mod gameapi.Module, ctx *gameapi.Context, m gameapi.Match, bots []int) gameapi.Match {
	t.Helper()
	ctx.Now = ctx.Now.Add(100 * time.Millisecond)
	for _, seat := range bots {
		ctx.Actor = seat
		before := append([]byte{}, m.Data...)
		a, e := mod.BotAction(*ctx, m)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(before, m.Data) {
			t.Fatal("bot mutated input")
		}
		if len(a) > 0 {
			var typ struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(a, &typ)
			if typ.Type != "drive" {
				t.Fatalf("bot took a shortcut: %s", a)
			}
			m, e = mod.Apply(*ctx, m, a)
			if e != nil {
				t.Fatalf("bot action rejected %s: %v", a, e)
			}
		}
	}
	ctx.Actor = -1
	next, e := mod.OnTimer(*ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	if e = mod.ValidateMatch(next, ctx.Players); e != nil {
		t.Fatal(e)
	}
	return next
}
func TestRacingCompleteNaturalRaces(t *testing.T) {
	for _, track := range []string{"metro", "harbor", "woodland", "coast", "canyon"} {
		for n := 2; n <= 6; n++ {
			for _, laps := range []int{1, 3, 5} {
				t.Run(fmt.Sprintf("%s-%d-seats-%d-laps", track, n, laps), func(t *testing.T) {
					mod, ctx, m := begin(t, n, laps, track)
					initial := view(t, mod, m, -1)
					bots := make([]int, n)
					for i := range bots {
						bots[i] = i
					}
					records := []any{map[string]any{"scope": "production Module Start/BotAction/Apply/OnTimer; natural complete race; accelerated clock only", "initial": initial}}
					defer func() { trace(t, records) }()
					for steps := 0; int64(steps)*100 < initial.MaxRaceAt-initial.ServerAt+1000 && m.Result == nil; steps++ {
						m = tick(t, mod, &ctx, m, bots)
						if steps%100 == 0 {
							v := view(t, mod, m, -1)
							records = append(records, map[string]any{"at": v.ServerAt, "cars": v.Cars})
						}
					}
					v := view(t, mod, m, -1)
					records = append(records, map[string]any{"final": v, "result": m.Result})
					if m.Result == nil {
						t.Fatal("race never ended")
					}
					for _, c := range v.Cars {
						if c.FinishAt == 0 || c.Lap != laps {
							t.Fatalf("seat %d did not naturally complete: lap=%d checkpoint=%d progress=%.1f position=(%.1f,%.1f) speed=%.1f", c.Seat, c.Lap, c.Checkpoint, c.Progress, c.X, c.Y, c.Speed)
						}
					}
					if v.ServerAt >= v.MaxRaceAt {
						t.Fatal("used absolute timeout instead of finishing")
					}
					if len(m.Result.Winners) < 1 {
						t.Fatal("no winner")
					}
					if m.Result.Cooperative || m.Result.Team {
						t.Fatal("not a pure individual race")
					}
					t.Logf("all %d cars completed %d laps on %s in %.1fs", n, laps, track, float64(v.ServerAt-v.StartAt)/1000)
				})
			}
		}
	}
}
func TestRacingInputSecurityAndCountdown(t *testing.T) {
	mod, ctx, m := begin(t, 3, 3, "woodland")
	v := view(t, mod, m, 0)
	if v.Phase != "countdown" || v.StartAt-v.ServerAt != 3000 {
		t.Fatal("missing countdown")
	}
	if view(t, mod, m, -1).Controls.CanDrive {
		t.Fatal("spectator controls")
	}
	ctx.Actor = 0
	valid := map[string]any{"type": "drive", "epoch": v.Epoch, "seq": 1, "steer": .3, "throttle": 1, "brake": false, "drift": true}
	bad := []map[string]any{{"seq": 0}, {"epoch": "old"}, {"steer": 1.1}, {"throttle": -1}, {"x": 100}, {"type": "teleport"}}
	for _, patch := range bad {
		a := map[string]any{}
		for k, v := range valid {
			a[k] = v
		}
		for k, v := range patch {
			a[k] = v
		}
		before := raw(m)
		got, e := mod.Apply(ctx, m, raw(a))
		if e == nil || !bytes.Equal(raw(got), before) {
			t.Fatalf("accepted bad input or changed state: %s %v", raw(a), e)
		}
	}
	ctx.Actor = -1
	if _, e := mod.Apply(ctx, m, raw(valid)); e == nil {
		t.Fatal("spectator drove")
	}
	ctx.Actor = 0
	next, e := mod.Apply(ctx, m, raw(valid))
	if e != nil {
		t.Fatal(e)
	}
	if !mod.AcceptStale(ctx, m, raw(valid)) {
		t.Fatal("new sequence rejected only due to global revision")
	}
	if mod.AcceptStale(ctx, next, raw(valid)) {
		t.Fatal("duplicate accepted")
	}
	if _, e = mod.Apply(ctx, next, raw(valid)); e == nil {
		t.Fatal("duplicate applied")
	}
	m = next
	ctx.Now = ctx.Now.Add(100 * time.Millisecond)
	ctx.Actor = -1
	m, e = mod.OnTimer(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	after := view(t, mod, m, 0)
	for i, c := range after.Cars {
		if c.X != v.Cars[i].X || c.Y != v.Cars[i].Y {
			t.Fatal("moved before start")
		}
	}
	trace(t, after)
}
func TestRacingOptions(t *testing.T) {
	mod := game.Module()
	if e := mod.Validate(); e != nil {
		t.Fatal(e)
	}
	if !mod.Realtime {
		t.Fatal("racing must opt into simultaneous bot frames")
	}
	for _, n := range []int{0, 1, 7, 32} {
		if _, e := mod.Configure(n, raw(map[string]any{})); e == nil {
			t.Fatalf("accepted %d seats", n)
		}
	}
	for _, o := range []any{map[string]any{"laps": 2}, map[string]any{"track": "missing"}, map[string]any{"speed": 999}} {
		if _, e := mod.Configure(2, raw(o)); e == nil {
			t.Fatalf("accepted invalid options %v", o)
		}
	}
	b, e := mod.Configure(2, nil)
	if e != nil {
		t.Fatal(e)
	}
	var o game.Options
	_ = json.Unmarshal(b, &o)
	if o.Track != "metro" || o.Laps != 3 {
		t.Fatal(o)
	}
}
func TestRacingRestoreReclaimAndPacketLoss(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "coast")
	for i := 0; i < 75; i++ {
		m = tick(t, mod, &ctx, m, []int{0, 1})
	}
	before := view(t, mod, m, 0)
	var saved gameapi.Match
	_ = json.Unmarshal(raw(m), &saved)
	ctx.Now = ctx.Now.Add(100 * time.Millisecond)
	ctx.Actor = 0
	restored, e := mod.Restore(ctx, saved)
	if e != nil || !reflect.DeepEqual(saved, restored) {
		t.Fatal("restore reset race or deadlines", e)
	}
	m, e = mod.Reclaim(ctx, restored)
	if e != nil {
		t.Fatal(e)
	}
	after := view(t, mod, m, 0)
	if after.StartAt != before.StartAt || after.FinishDeadline != before.FinishDeadline || !reflect.DeepEqual(after.Cars[1], before.Cars[1]) {
		t.Fatal("reclaim altered unrelated seat or race")
	}
	if after.Cars[0].Throttle != 0 || after.Cars[0].Seq != before.Cars[0].Seq || after.Cars[0].Progress != before.Cars[0].Progress || after.Cars[0].IdleAt != ctx.Now.Add(15*time.Second).UnixMilli() {
		t.Fatal("reclaim not a neutral input/own deadline refresh")
	}
	for i := 0; i < 30; i++ {
		m = tick(t, mod, &ctx, m, []int{1})
	}
	lost := view(t, mod, m, 0)
	if lost.Cars[0].Speed > 20 {
		t.Fatalf("lost connection kept driving: %.1f", lost.Cars[0].Speed)
	}
	trace(t, []game.View{before, after, lost})
}
func TestRacingTimerReplayAndIdleDraw(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "woodland")
	ctx.Actor = -1
	ctx.Now = time.UnixMilli(m.SystemDeadline)
	m, e := mod.OnTimer(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	again, e := mod.OnTimer(ctx, m)
	if e != nil || !reflect.DeepEqual(m, again) {
		t.Fatal("duplicate timer advanced physics")
	}
	v := view(t, mod, m, 0)
	ctx.Now = time.UnixMilli(v.MaxRaceAt + 1)
	m, e = mod.OnTimer(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	if m.Result == nil || !m.Result.Draw || len(m.Result.Winners) != 0 {
		t.Fatal("no progress should be an explicit draw", m.Result)
	}
	if e = mod.ValidateMatch(m, 2); e != nil {
		t.Fatal(e)
	}
	a := raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": 1, "throttle": 1})
	ctx.Actor = 0
	if _, e = mod.Apply(ctx, m, a); e == nil {
		t.Fatal("drove after finish")
	}
	trace(t, m)
}

func TestRacingSkinDuringRaceIsCosmetic(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "woodland")
	for i := 0; i < 45; i++ {
		m = tick(t, mod, &ctx, m, []int{0, 1})
	}
	before := view(t, mod, m, 0)
	ctx.Actor = 0
	changed, e := mod.Apply(ctx, m, raw(map[string]any{"type": "skin", "epoch": before.Epoch, "skin": 4}))
	if e != nil {
		t.Fatal(e)
	}
	after := view(t, mod, changed, 0)
	after.Cars[0].Skin = before.Cars[0].Skin
	if !reflect.DeepEqual(before, after) {
		t.Fatal("cosmetic skin altered race", before, after)
	}
	trace(t, view(t, mod, changed, 0))
}

// Additional requested classic models: all six choices remain cosmetic, retain
// progress and input state, and reject unlisted IDs through the real module API.
func TestRacingSixClassicModelsRemainCosmetic(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "metro")
	ctx.Actor = 0
	before := view(t, mod, m, 0)
	for skin := 0; skin < 6; skin++ {
		changed, err := mod.Apply(ctx, m, raw(map[string]any{"type": "skin", "epoch": before.Epoch, "skin": skin}))
		if err != nil {
			t.Fatalf("model %d rejected: %v", skin, err)
		}
		after := view(t, mod, changed, 0)
		if after.Cars[0].Skin != skin {
			t.Fatal("model not selected")
		}
		after.Cars[0].Skin = before.Cars[0].Skin
		if !reflect.DeepEqual(before, after) {
			t.Fatal("model changed physics, controls or race state")
		}
	}
	if _, err := mod.Apply(ctx, m, raw(map[string]any{"type": "skin", "epoch": before.Epoch, "skin": 6})); err == nil {
		t.Fatal("unknown classic model accepted")
	}
}

// Before AI tuning: do not merely raise a car cap while bots keep cruising at
// the old speed; stronger drivers must still use ordinary inputs and finish.
func TestRacingBotUsesSportsCarSpeed(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "metro")
	fastest := 0.0
	for step := 0; step < 1500 && m.Result == nil; step++ {
		m = tick(t, mod, &ctx, m, []int{0, 1})
		v := view(t, mod, m, -1)
		for _, car := range v.Cars {
			if car.Speed > fastest {
				fastest = car.Speed
			}
		}
	}
	v := view(t, mod, m, -1)
	trace(t, map[string]any{"fastest": fastest, "final": v})
	if fastest < 500 {
		t.Fatalf("computer still cruises at the old pace: %.1f", fastest)
	}
	if m.Result == nil {
		t.Fatal("faster computer did not complete the race")
	}
	for _, car := range v.Cars {
		if car.FinishAt == 0 {
			t.Fatal("computer failed to finish naturally")
		}
	}
}
