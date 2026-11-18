package engine_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func endingCreditsRuntime(t *testing.T) *engine.Runtime {
	t.Helper()
	path := "../../../.local/pal/ending-credits-fixture/rules.wasm"
	if os.Getenv("PAL_PRESENTATION_ENDING") == "1" {
		path = "../../../.local/pal/presentation-ending-fixture/rules.wasm"
	}
	program, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("build the dedicated --ending-fixture --ending-credits core first", err)
	}
	r, err := engine.NewRuntime(context.Background(), program, os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	return r
}

func TestOriginalEndingAcceptedSkipsCreditsAndSavedNativeExit(t *testing.T) {
	if os.Getenv("PAL_ENDING_CREDITS") != "1" {
		t.Skip("explicit complete original ending and added native credit-page continuation")
	}
	ctx := context.Background()
	r := endingCreditsRuntime(t)
	if _, err := engine.NewRuntimeCatalog(r); err == nil {
		t.Fatal("an additional-credit ending fixture entered the campaign catalog")
	}
	s, err := r.StartCampaignForPlayers(ctx, 12345, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, clip := range []int{4, 5, 6} {
		if s.View.Ending == nil || s.View.Ending.Clip != clip || s.View.Ending.Frame != 0 {
			t.Fatal("original ending movie order or first frame changed", clip, s.View.Ending)
		}
		s, err = r.StepWithInput(ctx, s, engine.Input{Direction: 4, Keys: 2})
		if err != nil || s.Event.Kind != engine.Delay || s.Event.Value != 500 {
			t.Fatal("original accepted skip did not retain its short wait", clip, err, s.Event)
		}
		encoded, err := r.Encode(s)
		if err != nil {
			t.Fatal(err)
		}
		r = endingCreditsRuntime(t)
		c, err := r.Continue(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.ReleaseInputs(ctx); err != nil {
			t.Fatal(err)
		}
		neutral := func(uint32) engine.Input { return engine.Input{Direction: 4} }
		partial, err := c.AdvanceScheduled(ctx, 500, 250, neutral)
		if err != nil || partial.Steps != 0 || partial.RemainingMS != 250 {
			t.Fatal("saved accepted skip lost its first timing budget", clip, err, partial)
		}
		partial, err = c.AdvanceScheduled(ctx, 250, 249, neutral)
		if err != nil || partial.Steps != 0 || partial.RemainingMS != 1 || c.View().Ending == nil || c.View().Ending.Clip != clip {
			t.Fatal("saved accepted skip lost its exact remaining wait", clip, err, partial)
		}
		if _, err = c.AdvanceScheduled(ctx, 1, 1, neutral); err != nil {
			t.Fatal(err)
		}
		encoded, err = c.Encode()
		if err != nil {
			t.Fatal(err)
		}
		s, err = r.Decode(encoded)
		if err != nil {
			t.Fatal(err)
		}
	}
	if s.Event.Kind == engine.SceneComplete || s.View.Menu.Kind != 13 || s.View.Ending != nil {
		t.Fatal("movie completion skipped the original credit page", s.Event, s.View.Menu, s.View.Ending)
	}
	var text strings.Builder
	for _, op := range s.View.Overlay {
		if op.Kind == 0 && op.Color != 0 {
			text.WriteRune(rune(op.ID))
		}
	}
	if !strings.Contains(text.String(), "SDLPAL") || !strings.Contains(text.String(), "Public License") || !strings.Contains(text.String(), "结束") {
		t.Fatal("original additional credit text was not drawn", text.String())
	}
	encoded, err := r.Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	r = endingCreditsRuntime(t)
	restored, err := r.Decode(encoded)
	if err != nil || restored.View.Menu.Kind != 13 {
		t.Fatal("original credit-page wait did not restore", err)
	}
	for i := 0; i < 128 && restored.Event.Kind != engine.SceneComplete; i++ {
		restored, err = r.StepWithInput(ctx, restored, engine.Input{Direction: 4, Keys: 2})
		if err != nil {
			t.Fatal(err)
		}
	}
	if restored.Event.Kind != engine.SceneComplete || restored.View.Ending == nil || !restored.View.Ending.Finished {
		t.Fatal("confirmed original credit page did not reach native shutdown")
	}
	result, err := json.MarshalIndent(map[string]any{"passed": true, "scope": "original three AVI calls, accepted skips saved for499/500ms with old inputs cleared, actual original credit text/menu and fresh-runtime credit-page confirmation followed by native shutdown; fixture only, not campaign acceptance", "identity": r.Identity(), "acceptedSkipSaves": 3, "creditSave": true}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := "../../../.local/pal/verification/ending-credits-engine.json"
	if os.Getenv("PAL_PRESENTATION_ENDING") == "1" {
		path = "../../../.local/pal/verification/screen-shake-ending-engine.json"
	}
	if err := os.WriteFile(path, append(result, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
