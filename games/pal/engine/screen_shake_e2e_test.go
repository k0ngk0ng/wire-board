package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tetratelabs/wazero/api"
)

func TestOriginalScreenShakeProjectionAndFreshSavedField(t *testing.T) {
	program := os.Getenv("PAL_PRESENTATION_CORE")
	if program == "" {
		t.Skip("explicit independent physical presentation and fresh-engine save verification")
	}
	ctx := context.Background()
	root := filepath.Join("..", "..", "..")
	load := func() *Runtime {
		t.Helper()
		wasm, err := os.ReadFile(filepath.Join(root, program))
		if err != nil {
			t.Fatal(err)
		}
		r, err := NewRuntime(ctx, wasm, os.DirFS(filepath.Join(root, ".local/pal/original-98")))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close(ctx) })
		return r
	}
	r := load()
	s, err := r.StartCampaignForPlayers(ctx, 12345, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; !(s.View.Scene == 2 && s.View.Mode == FieldMode && s.Event.Kind == Frame && s.View.Messages == 72); i++ {
		if i > 4096 {
			t.Fatal("genuine new game did not reach the original inn")
		}
		response := uint32(0)
		if s.Event.Kind == Dialogue {
			response = 1
		}
		s, err = r.Step(ctx, s, response)
		if err != nil {
			t.Fatal(err)
		}
	}
	module, err := r.restoreValidated(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	defer module.Close(ctx)
	probe := func(name string, args ...uint64) {
		t.Helper()
		stack := module.ExportedGlobal("__stack_pointer").(api.MutableGlobal)
		old := stack.Get()
		stack.Set(module.ExportedGlobal("wb_service_stack").Get() + 262144)
		defer stack.Set(old)
		if _, err := module.ExportedFunction(name).Call(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	probe("wb_probe_scene_wave", 8, 0)
	probe("VIDEO_ShakeScreen", 4, 3)
	probe("VIDEO_UpdateScreen", 0)
	s, err = r.capture(ctx, module, s.Entry, s.Event)
	if err != nil || s.View.Raster == nil || s.View.Field == nil {
		t.Fatal("physical display graph or original public field state is missing", err)
	}
	encoded, err := r.Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	fresh := load()
	restored, err := fresh.Decode(encoded)
	if err != nil || !reflect.DeepEqual(restored, s) {
		t.Fatal("new engine lost the original physical display boundary", err)
	}
	baseline, err := r.ReleaseInputs(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	restored, err = fresh.ReleaseInputs(ctx, restored)
	if err != nil || !reflect.DeepEqual(restored.View.Raster, s.View.Raster) {
		t.Fatal("input cleanup changed the saved physical frame", err)
	}
	for i := 0; i < 8; i++ {
		baseline, err = r.StepWithInput(ctx, baseline, Input{Direction: 4})
		if err != nil {
			t.Fatal(err)
		}
		restored, err = fresh.StepWithInput(ctx, restored, Input{Direction: 4})
		if err != nil || !reflect.DeepEqual(restored, baseline) {
			t.Fatal("saved physical shake diverged during original field continuation", i, err)
		}
	}
	if restored.View.Raster != nil || restored.View.Field == nil || restored.View.Cash != s.View.Cash || !reflect.DeepEqual(restored.View.Party, s.View.Party) {
		t.Fatal("completed physical shake lost normal field prediction or resources")
	}
	report, err := json.MarshalIndent(map[string]any{"passed": true, "scope": "Go engine capture, encode, new-runtime restore, input release and eight original field continuations of an independently configured physical shake after genuine new-game inn arrival; no story shake or full platform save claim", "identity": r.Identity(), "frozenRasterNodes": len(s.View.Raster.Nodes), "continuedFrames": 8, "normalFieldRecovered": true}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, ".local/pal/verification/screen-shake-engine.json"), append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
