package engine

// E2E first: a real original scene loses its map-wave operation or public
// motor; saving loses phase/progression or aliases metadata; malformed wave
// commands survive snapshot validation; a waved scene cannot seed the raster.
// Only rendering is configured by a native test probe, after new-game arrival.
import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tetratelabs/wazero/api"
)

func TestOriginalSceneWaveProjectionAndSavedContinuation(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join("..", "..", "..")
	program := os.Getenv("PAL_WAVE_CORE")
	if program == "" {
		t.Skip("explicit candidate acceptance: set PAL_WAVE_CORE to the retained wave-capable core")
	}
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
			t.Fatal("original new game never reached the inn")
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
		fn := module.ExportedFunction(name)
		if fn == nil {
			t.Fatal("missing original scene-wave fixture", name)
		}
		stack := module.ExportedGlobal("__stack_pointer").(api.MutableGlobal)
		old := stack.Get()
		stack.Set(module.ExportedGlobal("wb_service_stack").Get() + 262144)
		defer stack.Set(old)
		if _, err := fn.Call(ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	probe("wb_probe_scene_wave", 8, 0)
	s, err = r.capture(ctx, module, s.Entry, s.Event)
	if err != nil {
		t.Fatal(err)
	}
	if s.View.Field == nil || len(s.View.Draws) == 0 || s.View.Draws[0].Kind != 2 || s.View.Draws[0].A != 8 {
		t.Fatal("original map wave or field projection missing", s.View.Field, s.View.Draws)
	}
	encoded, err := r.Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	configuredEvent := s.Event
	fresh := load()
	after, err := fresh.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	after, err = fresh.ReleaseInputs(ctx, after)
	if err != nil || !reflect.DeepEqual(s.View.Draws, after.View.Draws) || !reflect.DeepEqual(s.View.Field, after.View.Field) {
		t.Fatal("saved wave or motor changed on fresh restore", err)
	}
	for _, change := range []func(*DrawOp){
		func(d *DrawOp) { d.A = 0 }, func(d *DrawOp) { d.A = 256 },
		func(d *DrawOp) { d.B = -1 }, func(d *DrawOp) { d.B = 32 },
		func(d *DrawOp) { d.C = -32769 }, func(d *DrawOp) { d.C = 32768 },
		func(d *DrawOp) { d.SortY = 0 }, func(d *DrawOp) { d.D = 1 },
	} {
		bad, err := fresh.Decode(encoded)
		if err != nil {
			t.Fatal(err)
		}
		change(&bad.View.Draws[0])
		if _, err = fresh.Encode(bad); err == nil {
			t.Fatal("invalid saved original map wave accepted", bad.View.Draws[0])
		}
	}
	for i := 0; i < 12; i++ {
		s, err = r.Step(ctx, s, 0)
		if err != nil {
			t.Fatal(err)
		}
		after, err = fresh.Step(ctx, after, 0)
		if err != nil || !reflect.DeepEqual(s.View, after.View) || s.Event != after.Event {
			t.Fatal("fresh runtime lost original scene-wave continuation", i, err)
		}
	}
	// A delta movie/offscreen effect captures the wave-bearing scene rather
	// than applying a second whole-screen wave over actors or dialogue.
	probe("wb_raster_enable")
	seeded, err := r.capture(ctx, module, "campaign", configuredEvent)
	if err != nil || seeded.View.Raster == nil || seeded.View.Raster.Base == nil || seeded.View.Raster.Base.Draws[0].Kind != 2 {
		t.Fatal("immutable scene seed lost original map wave", err)
	}
	if _, err = r.Encode(seeded); err != nil {
		t.Fatal("wave-bearing scene seed rejected by snapshot validation", err)
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "real new-game inn with explicit map-wave rendering fixture; public wave/motor, original signed progression bounds, fresh-runtime continuation and waved raster seed; not blood-pool story acceptance", "buildID": s.BuildID, "resourceID": s.ResourceID, "savedContinuationSteps": 12, "invalidWaveCasesRejected": 8}, "", "  ")
	if err = os.WriteFile(filepath.Join(root, ".local/pal/verification/scene-wave-go.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
