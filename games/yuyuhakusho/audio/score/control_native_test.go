package score_test

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func musicAction(t *testing.T, name string) score.MusicControlAction {
	t.Helper()
	switch name {
	case "stop-dac-flags":
		return score.ActionStopMusicDACFlags
	case "fade-step":
		return score.ActionFadeMusicStep
	case "reset-score":
		return score.ActionResetScore
	}
	t.Fatal("unknown native action", name)
	return 0
}

func scoreOwnedView(d *score.Driver) map[string]any {
	v := d.View()
	return map[string]any{"tempo_remaining": v["tempo_remaining"], "tempo_period": v["tempo_period"], "voices": v["voices"]}
}

func TestControlActionsAgainstOriginalWindows(t *testing.T) {
	f, err := os.Open("testdata/control-oracle/native-actions.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			ID    string `json:"id"`
			Setup struct {
				Dispatch []struct {
					Tick int   `json:"whole_tick"`
					ID   uint8 `json:"sound_id"`
				} `json:"accepted_score_dispatches"`
			} `json:"setup"`
			Windows []struct {
				ID            int    `json:"id"`
				Tick          int    `json:"whole_tick"`
				Action        string `json:"action"`
				Before, After json.RawMessage
				Intents       []score.Intent `json:"score_intents"`
				RootHardware  []score.Intent `json:"root_hardware_intents"`
			} `json:"windows"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	c := catalog(t)
	windows, writes, stop, fade, reset := 0, 0, 0, 0, 0
	for _, test := range fixture.Cases {
		t.Run(test.ID, func(t *testing.T) {
			d, _ := score.New(c)
			commands := map[int]uint8{}
			for _, cmd := range test.Setup.Dispatch {
				commands[cmd.Tick] = cmd.ID
			}
			current := 0
			for _, window := range test.Windows {
				for current < window.Tick {
					if _, err := d.Advance(score.Input{SoundID: commands[current]}); err != nil {
						t.Fatal(err)
					}
					current++
				}
				equalJSON(t, "native action before", scoreOwnedView(d), window.Before)
				before := d.Snapshot()
				out, err := d.ApplyMusicControlObserved(musicAction(t, window.Action), controlHooks())
				if err != nil {
					t.Fatal(err)
				}
				equalJSON(t, "native action after", scoreOwnedView(d), window.After)
				if !reflect.DeepEqual(out.Intents, window.Intents) && !(len(out.Intents) == 0 && len(window.Intents) == 0) {
					t.Fatalf("native action IO differs %s step%d: actual%v expected%v", window.Action, window.Tick, out.Intents, window.Intents)
				}
				if out.ClearPrimaryPriority || d.Snapshot().Ticks != before.Ticks {
					t.Fatal("zero-tick action mutated Root priority or tick")
				}
				switch window.Action {
				case "stop-dac-flags":
					stop++
				case "fade-step":
					fade++
				case "reset-score":
					reset++
					if len(out.Intents) != 0 || len(window.RootHardware) != 35 {
						t.Fatal("Root04 hardware ownership lost")
					}
				}
				windows++
				writes += len(out.Intents)
			}
		})
	}
	if windows != 94 || stop != 2 || fade != 88 || reset != 4 || writes != 1010 {
		t.Fatalf("partial original window scope: %d stop%d fade%d reset%d writes%d", windows, stop, fade, reset, writes)
	}
	t.Logf("94 original action windows, 88 fade steps, 968 YM and 42 PSG writes; no voice clock advancement")
}
