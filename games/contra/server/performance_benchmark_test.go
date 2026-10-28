package contra

// Measures the real two-human module timer + two JSON views after filling the
// replay ring. No bot runs: the injected observer fails if accidentally called.
// This excludes HTTP/WebSocket framing, SQLite, network delay and bot vision.
import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func BenchmarkContraTwoHumanBatchAndViews(b *testing.B) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		b.Fatal(err)
	}
	build, _, err := runtime.ReadRulesBuild(root)
	if err != nil {
		b.Fatal(err)
	}
	manifest, rgb, pages := syntheticVisualData()
	visuals, err := NewBotVisualResources(manifest, rgb, pages)
	if err != nil {
		b.Fatal(err)
	}
	m, err := ModuleWithConfig(Config{RulesBuildID: build, Visuals: visuals, ObserveBot: func(engine.BotVisualFrame) (engine.BotObservation, error) {
		return engine.BotObservation{}, errors.New("human-only benchmark invoked bot")
	}})
	if err != nil {
		b.Fatal(err)
	}
	c := gameapi.Context{Actor: -1, Players: 2, Now: time.UnixMilli(100000), Automated: []bool{false, false}}
	g, err := m.Start(c, json.RawMessage(`{}`))
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		if i == 1 || i == 2 {
			g = benchmarkStartInput(b, m, c, g, i == 1)
		}
		c.Now = c.Now.Add(50 * time.Millisecond)
		g, err = m.OnTimer(c, g)
		if err != nil {
			b.Fatal(err)
		}
	}
	var saved State
	if err = json.Unmarshal(g.Data, &saved); err != nil {
		b.Fatal(err)
	}
	if len(saved.Frames) != replaySize || saved.Session.Random.Frames != 600 || saved.Session.World.Active.Combat.World.LevelRoutine != 4 || g.Result != nil {
		b.Fatal("benchmark did not reach normal battle with full replay history")
	}
	b.Logf("battle fixture frame=%d routine=%d seed=%d", saved.Session.Random.Frames, saved.Session.World.Active.Combat.World.LevelRoutine, saved.Session.Random.Seed)
	base := g
	c.Now = c.Now.Add(50 * time.Millisecond)
	var viewBytes int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		next, err := m.OnTimer(c, base)
		if err != nil {
			b.Fatal(err)
		}
		viewBytes = 0
		for actor := 0; actor < 2; actor++ {
			view, err := m.View(next, actor)
			if err != nil {
				b.Fatal(err)
			}
			wire, err := json.Marshal(view)
			if err != nil {
				b.Fatal(err)
			}
			viewBytes += len(wire)
		}
	}
	b.ReportMetric(float64(viewBytes), "view-B/batch")
	b.ReportMetric(float64(viewBytes*20), "view-B/s")
}

func benchmarkStartInput(b *testing.B, m gameapi.Module, c gameapi.Context, g gameapi.Match, pressed bool) gameapi.Match {
	b.Helper()
	var state State
	if err := json.Unmarshal(g.Data, &state); err != nil {
		b.Fatal(err)
	}
	held := engine.Buttons(0)
	if pressed {
		held = engine.ButtonStart
	}
	raw, err := json.Marshal(map[string]any{"type": "input", "epoch": state.Epoch, "sequence": state.Inputs[0].Sequence + 1, "held": int(held)})
	if err != nil {
		b.Fatal(err)
	}
	c.Actor = 0
	next, err := m.Apply(c, g, raw)
	if err != nil {
		b.Fatal(err)
	}
	return next
}
