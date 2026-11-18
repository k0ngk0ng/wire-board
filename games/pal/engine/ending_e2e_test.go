package engine_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func endingRuntime(t *testing.T) *engine.Runtime {
	t.Helper()
	program, err := os.ReadFile("../../../.local/pal/ending-fixture/rules.wasm")
	if err != nil {
		t.Fatal("build the dedicated --ending-fixture core first", err)
	}
	r, err := engine.NewRuntime(context.Background(), program, os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	return r
}

func TestOriginalEndingTimelineAndFreshRuntimeSaves(t *testing.T) {
	if os.Getenv("PAL_ENDING_E2E") != "1" {
		t.Skip("explicit original ending fixture and fresh-runtime save acceptance")
	}
	ctx := context.Background()
	r := endingRuntime(t)
	s, err := r.StartCampaignForPlayers(ctx, 12345, 1)
	if err != nil {
		t.Fatal(err)
	}
	initialCash, initialSeed := s.View.Cash, s.View.Seed
	seen, saved := map[int]int{}, []engine.EndingState{}
	for i := 0; i < 5000 && s.Event.Kind != engine.SceneComplete; i++ {
		v := s.View.Ending
		if v == nil || v.Finished || v.Frame != seen[v.Clip] {
			t.Fatal("native ending public timeline is missing or skipped a frame", s.Event, v)
		}
		seen[v.Clip]++
		if v.Frame == 0 || v.Frame == 14 || v.Frame == v.Frames/2 || v.Frame == v.Frames-1 {
			encoded, err := r.Encode(s)
			if err != nil {
				t.Fatal(err)
			}
			fresh := endingRuntime(t)
			resumed, err := fresh.Decode(encoded)
			if err != nil || !reflect.DeepEqual(resumed, s) {
				t.Fatal("saved ending changed the frozen boundary", err)
			}
			if _, err := engine.NewRuntimeCatalog(r); err == nil {
				t.Fatal("an ending fixture entered the campaign runtime catalog")
			}
			baseline, e := r.StepWithInput(ctx, s, engine.Input{Direction: 4})
			actual, err := fresh.StepWithInput(ctx, resumed, engine.Input{Direction: 4})
			if e != nil || err != nil || !reflect.DeepEqual(actual, baseline) {
				t.Fatal("fresh runtime did not continue the original ending", v, e, err)
			}
			for _, corrupt := range []func(*engine.EndingState){
				func(v *engine.EndingState) { v.Clip = 3 },
				func(v *engine.EndingState) { v.Frame = v.Frames },
				func(v *engine.EndingState) { v.ElapsedMS++ },
				func(v *engine.EndingState) { v.Rate = 16 },
				func(v *engine.EndingState) { v.Finished = true },
			} {
				bad := s
				copy := *v
				bad.View.Ending = &copy
				corrupt(&copy)
				if _, err := r.Encode(bad); err == nil {
					t.Fatal("corrupt ending state was accepted", copy)
				}
			}
			saved = append(saved, *v)
			s, r = actual, fresh
		} else {
			s, err = r.StepWithInput(ctx, s, engine.Input{Direction: 4})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if !reflect.DeepEqual(seen, map[int]int{4: 200, 5: 2294, 6: 2196}) || len(saved) != 12 || s.View.Ending == nil || !s.View.Ending.Finished || s.Event.Kind != engine.SceneComplete || s.View.Cash != initialCash || s.View.Seed != initialSeed {
		t.Fatal("original ending did not finish unchanged", seen, len(saved), s.Event, s.View.Ending)
	}
	result, err := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original ending fixture through Go authority, all frames and twelve fresh-runtime continuations, invalid positions and fixture-catalog rejection; not a full campaign or browser/audio acceptance", "identity": r.Identity(), "frames": seen, "saves": saved}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("../../../.local/pal/verification/ending-engine.json", append(result, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
