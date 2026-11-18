package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func palPerformanceModule(t *testing.T) (gameapi.Module, string) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var release struct{ Current string }
	raw, err := os.ReadFile(filepath.Join(root, "games/pal/assets/server-release.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &release); err != nil || len(release.Current) != 64 {
		t.Fatal("current server distribution is missing", err)
	}
	bundle, err := pal.OpenProductionBundle(context.Background(), os.DirFS(filepath.Join(root, ".local/pal/prepared-server", release.Current, "pal")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bundle.Close(context.Background()) })
	return bundle.Module, bundle.Identity.BuildID
}

func TestPALCurrentOriginalFieldEntryStartsAndProjects(t *testing.T) {
	if os.Getenv("PAL_CURRENT_FIELD_ENTRY") != "1" {
		t.Skip("explicit current original field checkpoint projection")
	}
	base, build := palPerformanceModule(t)
	module := palFieldCheckpointModule(base)
	match, err := module.Start(gameapi.Context{Now: time.Now(), Players: 1, Actor: 0, Version: 1}, nil)
	if err != nil {
		t.Fatal("current original field start", err)
	}
	for name, project := range map[string]func(gameapi.Match, int) (any, error){"view": module.View, "snapshot": module.SyncSnapshot} {
		value, err := project(match, 0)
		if err != nil {
			t.Fatal(name, err)
		}
		view := value.(pal.View)
		if view.Scene != 2 || view.Phase != "field" || view.Dialogue != nil || len(view.Party) != 1 || view.Audio == nil || view.Audio.Music == nil || view.Audio.Music.StartedAt > view.ControlAt {
			t.Fatal(name, "current original field or imported audio clock is invalid", view)
		}
	}
	if _, err := module.StreamView(match, 0, ""); err != nil {
		t.Fatal("stream", err)
	}
	streamEvidence(t, "pal-current-original-field-entry", map[string]any{"passed": true, "buildID": build, "players": 1, "scope": "current production rule-only bundle, actual original new-game opening to inn checkpoint, imported clock, view/snapshot/stream; not campaign or performance acceptance"})
}

func palPerformanceMenuEntry(base gameapi.Module) gameapi.Module {
	module := base
	module.Start = func(c gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
		match, err := base.Start(c, options)
		if err != nil {
			return match, err
		}
		value, err := base.View(match, 0)
		if err != nil {
			return match, err
		}
		view := value.(pal.View)
		action, err := json.Marshal(pal.Action{Type: "input", Epoch: view.Epoch, Seq: view.Seq + 1, ControlAt: view.ControlAt, Direction: 4, Keys: 1})
		if err != nil {
			return match, err
		}
		match, err = base.Apply(c, match, action)
		if err != nil {
			return match, err
		}
		now := c
		for step := 0; step < 12; step++ {
			now.Now = time.UnixMilli(match.SystemDeadline)
			match, err = base.OnTimer(now, match)
			if err != nil {
				return match, err
			}
			value, err = base.View(match, 0)
			if err != nil {
				return match, err
			}
			view = value.(pal.View)
			if view.Phase == "menu" && view.Menu.Kind == 1 {
				match, err = palBrowserImportClock(c, match)
				if err != nil {
					return match, err
				}
				return base.ResumeSaved(c, match)
			}
		}
		return match, errors.New("original menu did not open using an ordinary native input")
	}
	return module
}

func TestPALCurrentOriginalFieldTimerBudget(t *testing.T) {
	if os.Getenv("PAL_FIELD_TIMER_BENCH") != "1" {
		t.Skip("explicit isolated current original field timer profiling")
	}
	base, build := palPerformanceModule(t)
	module := palFieldCheckpointModule(base)
	c := gameapi.Context{Now: time.Now(), Players: 1, Actor: 0, Version: 1}
	match, err := module.Start(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	samples := make([]float64, 0, 60)
	for len(samples) < cap(samples) {
		if match.SystemDeadline == 0 {
			t.Fatal("original field lost its scheduled frame")
		}
		c.Now = time.UnixMilli(match.SystemDeadline)
		begin := time.Now()
		match, err = module.OnTimer(c, match)
		samples = append(samples, float64(time.Since(begin))/float64(time.Millisecond))
		if err != nil {
			t.Fatal(err)
		}
	}
	ordered := append([]float64(nil), samples...)
	sort.Float64s(ordered)
	p95 := ordered[56]
	streamEvidence(t, "pal-current-original-field-timer", map[string]any{"passed": p95 <= 20, "buildID": build, "samplesMS": samples, "p95MS": p95, "budgetMS": 20, "scope": "isolated current original rule-only production module, 60 consecutive due timer callbacks at original schedule; CPU diagnosis, not HTTP or browser acceptance"})
	if p95 > 20 {
		t.Fatal("original field rule timer exceeds frozen budget", p95)
	}
}
