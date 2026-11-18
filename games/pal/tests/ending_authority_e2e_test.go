package pal_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func endingModule(t *testing.T) gameapi.Module {
	t.Helper()
	program, err := os.ReadFile("../../../.local/pal/ending-fixture/rules.wasm")
	if err != nil {
		t.Fatal(err)
	}
	r, err := engine.NewRuntime(context.Background(), program, os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	return pal.NewModule(r)
}

func TestOriginalEndingAuthorityFreezeResumeAndTerminalBoundary(t *testing.T) {
	if os.Getenv("PAL_ENDING_E2E") != "1" {
		t.Skip("explicit isolated ending authority and save-control acceptance")
	}
	m := endingModule(t)
	c := gameapi.Context{Players: 1, Actor: 0, Now: time.Unix(1800000000, 0), Automated: []bool{false}}
	g, err := m.Start(c, raw(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	seen, saved := map[int]bool{}, map[string]bool{}
	normalize := func(v pal.View) pal.View {
		v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
		return v
	}
	for i := 0; i < 3000 && g.Result == nil; i++ {
		v := view(t, m, g, 0)
		if v.Phase != "ending" || v.Ending == nil || v.Ending.Finished {
			t.Fatal("native movie was not exposed as an active ending", v.Phase, v.Ending)
		}
		seen[v.Ending.Clip] = true
		label := fmt.Sprint(v.Ending.Clip, "/", v.Ending.Frame >= v.Ending.Frames/2)
		if !saved[label] {
			g, err = m.SaveControl(c, g, true)
			if err != nil {
				t.Fatal(err)
			}
			c.Now = c.Now.Add(30 * 24 * time.Hour)
			frozen := bytes.Clone(g.Data)
			g, err = m.OnTimer(c, g)
			if err != nil || !bytes.Equal(frozen, g.Data) {
				t.Fatal("saved ending replayed offline time", err)
			}
			m = endingModule(t)
			g, err = m.ResumeSaved(c, g)
			if err != nil || !reflect.DeepEqual(normalize(view(t, m, g, 0)), normalize(v)) {
				t.Fatal("authority lost the exact frozen ending frame", label, err)
			}
			if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: v.Epoch, ControlAt: v.ControlAt, Seq: v.Seq + 1, Direction: 4, Keys: 2})); err == nil {
				t.Fatal("old ending input crossed the save boundary")
			}
			saved[label] = true
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 150)
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	v := view(t, m, g, 0)
	if g.Result == nil || !g.Result.Unrated || !reflect.DeepEqual(g.Result.Winners, []int{0}) || v.Phase != "finished" || v.Ending == nil || !v.Ending.Finished || len(saved) != 6 || len(seen) != 3 || !g.Unrated || g.SystemDeadline != 0 {
		t.Fatal("ending authority terminal boundary is incomplete", g.Result, v.Phase, v.Ending, saved, seen)
	}
	c.Now = c.Now.Add(24 * time.Hour)
	stable, err := m.OnTimer(c, g)
	if err != nil || !reflect.DeepEqual(stable, g) {
		t.Fatal("completed ending continued to advance", err)
	}
	report, err := json.MarshalIndent(map[string]any{"passed": true, "scope": "isolated original ending through game module; six private save-control freezes and fresh runtimes, no offline playback or old inputs, full three-clip completion and inert unrated terminal; no campaign acceptance", "clips": seen, "saved": saved, "finished": v}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("../../../.local/pal/verification/ending-authority.json", append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
