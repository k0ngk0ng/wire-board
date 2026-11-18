package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALLocalFirstNativeQueueReplayPrivacyAndResume(t *testing.T) {
	wasm, err := os.ReadFile(filepath.Join("../..", ".local/pal/native/rules.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := engine.NewRuntime(t.Context(), wasm, os.DirFS(filepath.Join("../..", ".local/pal/original-98")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { runtime.Close(context.Background()) })
	module := palFieldCheckpointModule(pal.NewModule(runtime))
	c := gameapi.Context{Now: time.Now(), Actor: 0, Players: 1, Version: 1}
	match, err := module.Start(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	var clockState struct {
		ServerAt    int64
		RemainingMS uint32
	}
	if err := json.Unmarshal(match.Data, &clockState); err != nil {
		t.Fatal(err)
	}
	if match.SystemDeadline-clockState.ServerAt != int64(clockState.RemainingMS) {
		t.Fatal("original field schedules redundant non-native timer frames", match.SystemDeadline-clockState.ServerAt, clockState.RemainingMS)
	}
	project := func(snapshot bool, viewer int) map[string]any {
		t.Helper()
		fn := module.View
		if snapshot {
			fn = module.SyncSnapshot
		}
		value, err := fn(match, viewer)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(value)
		var result map[string]any
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	owner := project(true, 0)
	local, ok := owner["local"].(map[string]any)
	if !ok || local["checkpoint"] == nil || local["core"] == nil {
		t.Fatal("owner lacks original local execution checkpoint", owner["phase"])
	}
	if project(true, -1)["local"] != nil || project(false, -1)["local"] != nil {
		t.Fatal("spectator received private native state")
	}
	ordinary := project(false, 0)["local"].(map[string]any)
	if ordinary["checkpoint"] != nil {
		t.Fatal("ordinary view repeated the native checkpoint")
	}
	var body struct {
		Native      []byte
		RemainingMS uint32
	}
	if err := json.Unmarshal(match.Data, &body); err != nil {
		t.Fatal(err)
	}
	native, err := runtime.Continue(body.Native)
	if err != nil {
		t.Fatal(err)
	}
	fieldBoundary := native.InputBoundary()
	remaining := body.RemainingMS
	for i := 0; i < 24 && native.View().Menu.Kind != 1; i++ {
		keys := uint32(0)
		if i == 0 {
			keys = 1
		}
		step, err := native.AdvanceScheduled(t.Context(), remaining, 100, func(uint32) engine.Input { return engine.Input{Direction: 4, Keys: keys} })
		if err != nil {
			t.Fatal(err)
		}
		remaining = step.RemainingMS
	}
	if native.View().Menu.Kind != 1 {
		t.Fatal("original local menu fixture failed")
	}
	menuBoundary := native.InputBoundary()
	apply := func(sequence int, boundary string, keys int, generation any) error {
		payload, _ := json.Marshal(map[string]any{"type": "local_input", "epoch": generation, "seq": sequence, "boundary": boundary, "keys": keys, "direction": 4, "controlAt": c.Now.UnixMilli()})
		next, err := module.Apply(c, match, payload)
		if err == nil {
			match = next
		}
		return err
	}
	if err := apply(1, fieldBoundary, 1, local["generation"]); err != nil {
		t.Fatal(err)
	}
	if err := apply(2, menuBoundary, 2, local["generation"]); err != nil {
		t.Fatal("future native menu input was rejected before confirmation", err)
	}
	stream, err := module.StreamView(match, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	cursor := stream.Cursor
	for i := 0; i < 32 && project(false, 0)["menu"].(map[string]any)["kind"] != float64(2); i++ {
		c.Now = time.UnixMilli(match.SystemDeadline)
		match, err = module.OnTimer(c, match)
		if err != nil {
			t.Fatal(err)
		}
	}
	status := project(false, 0)
	if status["menu"].(map[string]any)["kind"] != float64(2) {
		t.Fatal("consecutive local inputs did not cross the original menu boundary")
	}
	meta := status["local"].(map[string]any)
	if meta["generation"] != local["generation"] || meta["appliedSeq"] != float64(2) {
		t.Fatal("phase change reset the local control generation or lost applied input", meta)
	}
	stream, err = module.StreamView(match, 0, cursor)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(stream.Data)
	var delta struct {
		Local struct {
			Checkpoint []byte
			Records    []json.RawMessage
			Resync     bool
		}
	}
	if err := json.Unmarshal(raw, &delta); err != nil || len(delta.Local.Checkpoint) != 0 || len(delta.Local.Records) == 0 || delta.Local.Resync {
		t.Fatal("incremental stream did not send native operations", string(raw), err)
	}
	streamAgain, err := module.StreamView(match, 0, stream.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(streamAgain.Data)
	delta.Local.Records = nil
	if err := json.Unmarshal(raw, &delta); err != nil || len(delta.Local.Records) == 0 || len(delta.Local.Records) > 6 {
		t.Fatal("stream lacks bounded redundancy for an HTTP state overtaking its cursor", err)
	}
	if err := apply(3, "wrong-boundary", 2, local["generation"]); err != nil {
		t.Fatal(err)
	}
	c.Now = c.Now.Add(1100 * time.Millisecond)
	match, err = module.OnTimer(c, match)
	if err != nil {
		t.Fatal(err)
	}
	if project(false, 0)["local"].(map[string]any)["discardedSeq"] != float64(3) {
		t.Fatal("expired speculative input was not discarded")
	}
	if err := apply(4, "interrupted-boundary", 2, local["generation"]); err != nil {
		t.Fatal(err)
	}
	release, _ := json.Marshal(map[string]any{"type": "local_release", "epoch": local["generation"], "seq": 5, "direction": 4, "boundary": meta["boundary"], "controlAt": c.Now.UnixMilli()})
	match, err = module.Apply(c, match, release)
	if err != nil {
		t.Fatal("interruption could not release speculative inputs", err)
	}
	if project(false, 0)["local"].(map[string]any)["discardedSeq"] != float64(5) {
		t.Fatal("interruption retained old speculative commands")
	}
	saved, err := module.SaveControl(c, match, true)
	if err != nil {
		t.Fatal(err)
	}
	match, err = module.ResumeSaved(c, saved)
	if err != nil {
		t.Fatal(err)
	}
	if project(false, 0)["local"].(map[string]any)["generation"] == local["generation"] {
		t.Fatal("save resume retained stale local inputs")
	}
	if err := apply(4, menuBoundary, 2, local["generation"]); err == nil {
		t.Fatal("previous control generation survived save resume")
	}
	resumed := project(false, 0)["local"].(map[string]any)
	if err := apply(1, resumed["boundary"].(string), 1, resumed["generation"]); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 24 && project(false, 0)["phase"] != "field"; i++ {
		c.Now = time.UnixMilli(match.SystemDeadline)
		match, err = module.OnTimer(c, match)
		if err != nil {
			t.Fatal(err)
		}
	}
	visible, _ := module.View(match, 0)
	v := visible.(pal.View)
	for _, input := range []struct{ sequence, direction, keys int }{{2, 4, 16}, {3, 2, 0}, {4, 4, 0}} {
		payload, _ := json.Marshal(map[string]any{"type": "local_input", "epoch": v.Local.Generation, "seq": input.sequence, "direction": input.direction, "keys": input.keys, "boundary": v.Local.Boundary, "controlAt": v.ControlAt})
		match, err = module.Apply(c, match, payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 8; i++ {
		c.Now = time.UnixMilli(match.SystemDeadline)
		match, err = module.OnTimer(c, match)
		if err != nil {
			t.Fatal(err)
		}
		value, _ := module.View(match, 0)
		current := value.(pal.View)
		if current.Local.AppliedSeq > 1 && current.Local.AppliedSeq != 4 {
			t.Fatal("held-level release was delayed behind a preserved tap", current.Local.AppliedSeq)
		}
		if current.Local.AppliedSeq == 4 {
			break
		}
	}
	visible, _ = module.View(match, 0)
	v = visible.(pal.View)
	c.Automated = []bool{true}
	legacy, _ := json.Marshal(pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Keys: 1, Direction: 4, ControlAt: v.ControlAt})
	match, err = module.Apply(c, match, legacy)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 24 && project(false, 0)["phase"] != "menu"; i++ {
		c.Now = time.UnixMilli(match.SystemDeadline)
		match, err = module.OnTimer(c, match)
		if err != nil {
			t.Fatal(err)
		}
	}
	if project(false, 0)["phase"] != "menu" {
		t.Fatal("manual local input prevented explicit computer takeover")
	}
	streamEvidence(t, "pal-local-first-authority", map[string]any{"passed": true, "crossPhaseInputs": 2, "ownerCheckpointOnly": true, "incrementalReplay": true, "expiredInputDiscarded": true, "saveResumeGeneration": true})
}
