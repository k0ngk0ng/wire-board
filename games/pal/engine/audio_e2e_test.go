package engine_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func audioRuntime(t *testing.T) *engine.Runtime {
	t.Helper()
	program, err := os.ReadFile("../../../.local/pal/runtime-candidates/audio/original/rules.wasm")
	if err != nil {
		t.Fatal(err)
	}
	r, err := engine.NewRuntime(context.Background(), program, os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	return r
}

func TestOriginalAudioJournalAndSavedOpening(t *testing.T) {
	if os.Getenv("PAL_AUDIO_E2E") != "1" {
		t.Skip("explicit original audio candidate")
	}
	r, ctx := audioRuntime(t), context.Background()
	s, err := r.StartCampaignForPlayers(ctx, 12345, 1)
	if err != nil {
		t.Fatal(err)
	}
	pages, saves, commands := 0, 0, uint32(0)
	for i := 0; i < 4096; i++ {
		if s.View.Audio == nil {
			t.Fatal("new core omitted the native audio journal")
		}
		if s.View.Audio.Serial < commands {
			t.Fatal("native audio sequence went backwards")
		}
		commands = s.View.Audio.Serial
		if s.Event.Kind == engine.Dialogue {
			pages++
			if pages == 1 || pages == 8 || pages == 23 {
				encoded, err := r.Encode(s)
				if err != nil {
					t.Fatal(err)
				}
				fresh := audioRuntime(t)
				restored, err := fresh.Decode(encoded)
				if err != nil || !reflect.DeepEqual(restored, s) {
					t.Fatal("native audio save changed", err)
				}
				projection, err := r.View(s)
				if err != nil {
					t.Fatal(err)
				}
				if len(projection.Audio.Commands) > 0 {
					projection.Audio.Commands[0].ID = 65534
					if reflect.DeepEqual(projection.Audio, s.View.Audio) {
						t.Fatal("audio projection aliases snapshot")
					}
				}
				expected, err := r.Step(ctx, s, 1)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := fresh.Step(ctx, restored, 1)
				if err != nil || !reflect.DeepEqual(actual, expected) {
					t.Fatal("saved native audio continuation diverged", err)
				}
				for _, mutate := range []func(*engine.AudioJournal){
					func(a *engine.AudioJournal) { a.Serial++ },
					func(a *engine.AudioJournal) {
						if len(a.Commands) > 0 {
							a.Commands[0].Kind = 4
						} else {
							a.Serial = 1
						}
					},
				} {
					bad := s
					audio := *s.View.Audio
					audio.Commands = append([]engine.AudioCommand(nil), audio.Commands...)
					bad.View.Audio = &audio
					mutate(&audio)
					if _, err := r.Encode(bad); err == nil {
						t.Fatal("corrupt audio journal accepted")
					}
				}
				saves++
			}
		}
		if pages > 0 && s.Event.Kind == engine.Frame && s.View.Mode == engine.FieldMode && !s.View.Entering {
			break
		}
		response := uint32(0)
		if s.Event.Kind == engine.Dialogue {
			response = 1
		}
		s, err = r.Step(ctx, s, response)
		if err != nil {
			t.Fatal(err)
		}
		if i == 4095 {
			t.Fatal("original opening never reached its field")
		}
	}
	if pages != 23 || saves != 3 || commands == 0 {
		t.Fatal(pages, saves, commands)
	}
	data, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original23-page opening, native ordered audio observation, three fresh-runtime continuations, detached projection and corrupt metadata rejection; no platform audio clock or browser claim", "identity": r.Identity(), "pages": pages, "saves": saves, "commands": commands, "audio": s.View.Audio}, "", "  ")
	if err := os.WriteFile("../../../.local/pal/verification/audio-engine.json", append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
