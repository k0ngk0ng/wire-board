package engine_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func gameOverRuntime(t *testing.T) *engine.Runtime {
	t.Helper()
	program, err := os.ReadFile("../../../.local/pal/runtime-candidates/game-over/fixture/rules.wasm")
	if err != nil {
		t.Fatal("build the dedicated original --game-over-fixture core", err)
	}
	r, err := engine.NewRuntime(context.Background(), program, os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	return r
}

func TestOriginalGameOverScriptPaletteDialogueAndSavedReload(t *testing.T) {
	if os.Getenv("PAL_GAME_OVER_E2E") != "1" {
		t.Skip("explicit original failure-script fixture and saved reload boundary")
	}
	ctx := context.Background()
	r := gameOverRuntime(t)
	if _, err := engine.NewRuntimeCatalog(r); err == nil {
		t.Fatal("a failure-script fixture entered the campaign catalog")
	}
	s, err := r.StartCampaignForPlayers(ctx, 12345, 1)
	if err != nil {
		t.Fatal(err)
	}
	cash, seed := s.View.Cash, s.View.Seed
	red, saved, messages := map[int]bool{}, map[string]bool{}, map[int]bool{}
	dialogueSeen, faded := false, false
	for i := 0; i < 512; i++ {
		g := s.View.GameOver
		if g == nil || s.View.Ending != nil || s.Event.Kind == engine.SceneComplete {
			t.Fatal("original failure became a victory or lost its native state", s.Event, g)
		}
		red[s.View.PaletteEffect.RedStep] = true
		for _, id := range s.View.DialogPage {
			messages[id] = true
		}
		dialogueSeen = dialogueSeen || s.Event.Kind == engine.Dialogue
		faded = faded || dialogueSeen && len(s.View.PaletteEffect.Factors) > 0
		label := ""
		switch {
		case s.Event.Kind == engine.GameOver:
			label = "reload"
		case s.Event.Kind == engine.Dialogue:
			label = "dialogue"
		case faded:
			label = "fade"
		case s.View.PaletteEffect.RedStep == 1:
			label = "red-first"
		case s.View.PaletteEffect.RedStep == 16:
			label = "red-middle"
		case s.View.PaletteEffect.RedStep == 32:
			label = "red-last"
		}
		response := uint32(0)
		if s.Event.Kind == engine.Dialogue {
			response = 1
		}
		if label != "" && !saved[label] {
			encoded, err := r.Encode(s)
			if err != nil {
				t.Fatal(err)
			}
			fresh := gameOverRuntime(t)
			actual, err := fresh.Decode(encoded)
			if err != nil || !reflect.DeepEqual(actual, s) {
				t.Fatal("fresh runtime lost the exact native failure boundary", label, err)
			}
			baseline, err := r.ReleaseInputs(ctx, s)
			if err != nil {
				t.Fatal(err)
			}
			actual, err = fresh.ReleaseInputs(ctx, actual)
			if err != nil {
				t.Fatal(err)
			}
			baseline, err = r.Step(ctx, baseline, response)
			if err != nil {
				t.Fatal(err)
			}
			actual, err = fresh.Step(ctx, actual, response)
			if err != nil || !reflect.DeepEqual(actual, baseline) {
				t.Fatal("saved original failure continuation diverged", label, err)
			}
			saved[label] = true
		}
		bad := s
		copy := *g
		copy.Stage = 3
		bad.View.GameOver = &copy
		if _, err := r.Encode(bad); err == nil {
			t.Fatal("invalid original failure metadata was accepted")
		}
		if s.Event.Kind == engine.GameOver {
			if g.Stage != 2 || g.Slot != 0 || s.View.Cash != cash || s.View.Seed != seed {
				t.Fatal("retained original reload changed rules or file slot", g)
			}
			break
		}
		s, err = r.Step(ctx, s, response)
		if err != nil {
			t.Fatal(err)
		}
		if i == 511 {
			t.Fatal("original failure did not reach its retained reload")
		}
	}
	for i := 1; i <= 32; i++ {
		if !red[i] {
			t.Fatal("an original red-palette step was omitted", i)
		}
	}
	for _, id := range []int{13135, 13136, 13137, 13138} {
		if !messages[id] {
			t.Fatal("original failure message omitted", id)
		}
	}
	if len(saved) != 6 || !dialogueSeen || !faded {
		t.Fatal("original failure save positions were incomplete", saved)
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "original script fixture through Go authority,32 original red steps, actual failure messages, original fade and retained reload, six fresh full-memory saves and metadata/catalog rejection; not actual battle defeat or platform acceptance", "identity": r.Identity(), "saved": saved}, "", "  ")
	if err = os.WriteFile("../../../.local/pal/verification/game-over-engine.json", append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
