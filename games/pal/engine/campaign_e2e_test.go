package engine_test

// Before campaign bridge implementation: opening-only harness mistaken for a
// game loop; script input leaks into free movement; missing scene resources;
// original scene reload resets state; collision removed with graphics; held
// controls survive restoring; a restored field continuation loses native locals.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestNativeCampaignReachesOriginalInn(t *testing.T) {
	runtime := nativeRuntime(t)
	s, err := runtime.StartCampaign(context.Background(), 12345)
	if err != nil {
		t.Fatal(err)
	}
	seenInnDialogue := false
	for i := 0; i < 4096; i++ {
		if s.Event.Kind == engine.Dialogue && s.View.Scene == 2 {
			v, e := runtime.View(s)
			if e != nil {
				t.Fatal(e)
			}
			if slices.Contains(s.View.DialogPage, 1318) {
				if v.Text != "李大娘∶\n李逍遥！你皮痒啊？\n敢说老娘是什么鬼婆！" || s.View.X != 1312 || s.View.Y != 288 {
					t.Fatal("original inn entry changed", v)
				}
				seenInnDialogue = true
			}
		}
		if seenInnDialogue && s.Event.Kind == engine.Frame && s.View.Mode == engine.FieldMode && s.View.Scene == 2 {
			break
		}
		previous := s
		ack := uint32(0)
		if s.Event.Kind == engine.Dialogue {
			ack = 1
		}
		s, err = runtime.Step(context.Background(), s, ack)
		if err != nil {
			t.Fatal("campaign failed before free movement", previous.View, previous.Event, err)
		}
	}
	if !seenInnDialogue || s.View.Scene != 2 || s.View.Mode != engine.FieldMode || s.Event.Kind != engine.Frame {
		t.Fatal("did not finish original inn introduction", s.View, s.Event)
	}
	if s.View.Sprite == 193 {
		t.Fatal("dream sprite did not change back in original story")
	}
	if s.View.Map != 12 || s.View.CameraX+160 != s.View.X || s.View.CameraY+112 != s.View.Y {
		t.Fatal("original camera or scene map changed", s.View)
	}
	heroVisible := false
	for _, op := range s.View.Draws {
		if op.Kind == 0 && op.A == s.View.Sprite {
			heroVisible = true
		}
	}
	if !heroVisible {
		t.Fatal("native scene did not expose its original player frame")
	}
	before := s.View
	moved := false
	for direction := 0; direction < 4 && !moved; direction++ {
		for i := 0; i < 4; i++ {
			s, err = runtime.StepWithInput(context.Background(), s, engine.Input{Direction: direction})
			if err != nil {
				t.Fatal(err)
			}
			if s.View.X != before.X || s.View.Y != before.Y {
				moved = true
				break
			}
		}
	}
	if !moved {
		t.Fatal("original character cannot move after the inn story", s.View)
	}
	saved, err := runtime.Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := runtime.Decode(saved)
	if err != nil {
		t.Fatal(err)
	}
	released, err := runtime.ReleaseInputs(context.Background(), restored)
	if err != nil {
		t.Fatal(err)
	}
	want := restored.View
	want.HeldDirection = 4
	want.PendingKeys = 0
	want.Keys = 0
	if !reflect.DeepEqual(released.View, want) || released.Event != restored.Event {
		t.Fatal("releasing restored inputs changed game progress", restored.View, released.View)
	}
	if released.View.HeldDirection != 4 || released.View.Keys != 0 || released.View.PendingKeys != 0 {
		t.Fatal("restored physical input was retained")
	}
	// Releasing is idempotent and can be applied before the room resumes.
	again, err := runtime.ReleaseInputs(context.Background(), released)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again, released) {
		t.Fatal("releasing controls twice changes native continuation")
	}
	for i := 0; i < 3; i++ {
		a, e := runtime.StepWithInput(context.Background(), s, engine.Input{Direction: 4})
		if e != nil {
			t.Fatal(e)
		}
		b, e := runtime.StepWithInput(context.Background(), restored, engine.Input{Direction: 4})
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(a.View, b.View) || a.Event != b.Event {
			t.Fatal("field continuation changed after save")
		}
		s, restored = a, b
	}
	out := filepath.Join("..", "..", "..", ".local", "pal", "verification")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "scope": "real original GameMain, inn story, field movement and native continuation; no browser/full campaign claim", "beforeMovement": before, "afterMovement": s.View, "savedBytes": len(saved)}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "native-inn.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
