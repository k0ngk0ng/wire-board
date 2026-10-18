package engine

// Test-first failure inventory: verify_reward_presentation.py. Persist the
// entire scene through each chain, never replace it with a later source frame.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

func TestOriginalRewardPresentationChains(t *testing.T) {
	f, e := os.Open("testdata/reward-presentation.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var chains []struct {
		Shape string
		Calls []struct {
			RawBefore, RawAfter           []byte
			BeforeProgress, AfterProgress campaign.Progress
			Sounds                        []uint8
		}
	}
	if e = json.NewDecoder(z).Decode(&chains); e != nil {
		t.Fatal(e)
	}
	count := 0
	for ci, chain := range chains {
		state := openingRAM(chain.Calls[0].RawBefore)
		progress := chain.Calls[0].BeforeProgress
		raw := chain.Calls[0].RawBefore
		context := ResultActorContext{TextRequestFlags: raw[0x6ab], TextID: raw[0x6ad], PaletteIDs: [2]uint8{raw[0x305], raw[0x306]}, DisplayMask: raw[0x304]}
		for i, row := range chain.Calls {
			if i > 0 {
				state.Projection = resetOriginalWorldDrawList(state.Projection)
				state.Transition.Random, _ = NextRandom(state.Transition.Random)
			}
			if want := openingRAM(row.RawBefore); !reflect.DeepEqual(state, want) {
				t.Fatal(ci, i, "continuous caller state", firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(want), "state"))
			}
			got, err := StepOriginalRewardPresentation(state, progress, context, row.RawBefore[4])
			if err != nil {
				t.Fatal(ci, i, err)
			}
			want := openingRAM(row.RawAfter)
			if !reflect.DeepEqual(got.State, want) {
				t.Fatal(ci, i, "complete reward", firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(want), "state"))
			}
			if got.Progress != row.AfterProgress || !reflect.DeepEqual(got.SoundIDs, row.Sounds) && len(got.SoundIDs)+len(row.Sounds) > 0 {
				t.Fatal(ci, i, "progress/sounds", got.Progress, got.SoundIDs, row.Sounds)
			}
			b, _ := json.Marshal(state)
			var restored OpeningState
			json.Unmarshal(b, &restored)
			replay, err := StepOriginalRewardPresentation(restored, progress, context, row.RawBefore[4])
			if err != nil || !reflect.DeepEqual(replay, got) {
				t.Fatal(ci, i, "restore", err)
			}
			state, progress, context = got.State, got.Progress, got.Context
			if got.Complete != (i == len(chain.Calls)-1) {
				t.Fatal(ci, i, "handoff too soon/late")
			}
			count++
		}
	}
	if len(chains) != 15 || count != 1146 {
		t.Fatal("coverage", len(chains), count)
	}
}
