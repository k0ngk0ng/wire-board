package runtimebridge

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func TestPredictionAdvanceMatchesRealRulesWithoutPixels(t *testing.T) {
	s := performanceSession(t, 0, false)
	fast, err := s.Fork()
	if err != nil {
		t.Fatal(err)
	}
	for tick := 0; tick < 720; tick++ {
		held := [4]engine.Buttons{}
		if tick%40 < 20 {
			held[0] = engine.ButtonRight | engine.ButtonB
		}
		if tick%50 < 15 {
			held[1] = engine.ButtonLeft | engine.ButtonA
		}
		want, err := s.StepControlled(held, 0)
		if err != nil {
			t.Fatal(tick, err)
		}
		got, err := fast.AdvancePredictionControlled(held, 0)
		if err != nil {
			t.Fatal(tick, err)
		}
		if !reflect.DeepEqual(got.View, want.View) || !bytes.Equal(performanceSave(t, s), performanceSave(t, fast)) {
			t.Fatal("canonical prediction changed", tick)
		}
		if want.View.Completed {
			if !bytes.Equal(got.Pixels, want.Pixels) {
				t.Fatal("early terminal drawing missing", tick)
			}
		} else if len(got.Pixels) != 0 {
			t.Fatal("invisible normal tick still rendered", tick)
		}
		if len(got.SoundRequests) != 0 {
			t.Fatal("historical replay emitted audio", tick)
		}
	}
}

func TestPredictionAdvanceKeepsActualTerminalAndRollback(t *testing.T) {
	a := networkAssets(t, 0)
	raw, err := os.ReadFile("../verification/client-terminal-preview/sample-legacy-four/prior-checkpoint.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := RestoreCheckpoint(raw, a)
	if err != nil {
		t.Fatal(err)
	}
	fast, err := s.Fork()
	if err != nil {
		t.Fatal(err)
	}
	want, err := s.StepControlled([4]engine.Buttons{}, 15)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fast.AdvancePredictionControlled([4]engine.Buttons{}, 15)
	if err != nil {
		t.Fatal(err)
	}
	if !want.View.Completed || !got.View.Completed || !bytes.Equal(got.Pixels, want.Pixels) || len(got.Pixels) != 320*224*4 || !bytes.Equal(performanceSave(t, s), performanceSave(t, fast)) {
		t.Fatal("terminal prediction lost actual draw")
	}
	frozen, err := os.ReadFile("../verification/client-terminal-preview/sample-legacy-four/final.rgba")
	if err != nil || !bytes.Equal(got.Pixels, frozen) {
		t.Fatal("original frozen terminal pixels differ", err)
	}
	broken := performanceSession(t, 0, false)
	before := performanceSave(t, broken)
	if _, err = broken.AdvancePredictionControlled([4]engine.Buttons{4096}, 0); err == nil || !bytes.Equal(before, performanceSave(t, broken)) {
		t.Fatal("invalid step committed")
	}
}
