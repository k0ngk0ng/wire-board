package engine

// Failure inventory is retained before implementation in verify_reward_phase.py.
// These comparisons stop before the three actor calls, not before a whole frame.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

func TestOriginalRewardPhaseSource(t *testing.T) {
	f, err := os.Open("testdata/reward-phase.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		RawBefore, RawAfter      []byte
		Before, After            struct{ Progress campaign.Progress }
		OmittedPresentationCalls []struct {
			Entry string
			A     uint8
		}
		RNGCalls int
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		s, want := openingRAM(row.RawBefore), openingRAM(row.RawAfter)
		got, e := StepOriginalRewardPhase(s, row.Before.Progress, row.RawBefore[4])
		if e != nil {
			t.Fatal(i, e)
		}
		if !reflect.DeepEqual(got.State, want) {
			t.Fatal(i, "phase state", firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(want), "state"))
		}
		if got.Progress != row.After.Progress {
			t.Fatal(i, "progress", got.Progress, row.After.Progress)
		}
		var sounds, messages, backgrounds []uint8
		fadeIn, fadeOut := false, false
		for _, event := range row.OmittedPresentationCalls {
			switch event.Entry {
			case "C000":
				sounds = append(sounds, event.A)
			case "C036":
				messages = append(messages, event.A)
			case "C024":
				backgrounds = append(backgrounds, event.A)
			case "C021":
				if event.A == 0 {
					fadeIn = true
				} else {
					fadeOut = true
				}
			default:
				t.Fatal("unexpected omission", event)
			}
		}
		if !reflect.DeepEqual(got.SoundIDs, sounds) || !reflect.DeepEqual(got.Messages, messages) || !reflect.DeepEqual(got.Backgrounds, backgrounds) || got.FadeIn != fadeIn || got.FadeOut != fadeOut {
			t.Fatal(i, "presentation requests", got, sounds, messages, backgrounds, fadeIn, fadeOut)
		}
		if got.ChangeCHR != !s.Transition.Match.Entered || got.ChangeCHR && got.CHRPages != [2]uint8{0x3b, 0x3b} {
			t.Fatal(i, "reward CHR request")
		}
		if got.Complete != (want.Transition.Scene == 2) {
			t.Fatal(i, "handoff")
		}
		encoded, _ := json.Marshal(s)
		var restored OpeningState
		if e = json.Unmarshal(encoded, &restored); e != nil {
			t.Fatal(e)
		}
		replay, e := StepOriginalRewardPhase(restored, row.Before.Progress, row.RawBefore[4])
		if e != nil || !reflect.DeepEqual(replay, got) {
			t.Fatal(i, "restore", e)
		}
	}
	if len(rows) != 695 {
		t.Fatal("source coverage", len(rows))
	}
}
