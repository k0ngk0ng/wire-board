package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCourtAnimationOriginalCallsAndInstructionProbes(t *testing.T) {
	for _, name := range []string{"court-animation.json.gz", "court-animation-probes.json.gz"} {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open("testdata/" + name)
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
				Court, Frame int
				Before       CourtAnimationState
				Context      CourtAnimationContext
				After        CourtAnimationStep
			}
			if err = json.NewDecoder(z).Decode(&rows); err != nil {
				t.Fatal(err)
			}
			want := 912
			if name == "court-animation-probes.json.gz" {
				want = 13312
			}
			if len(rows) != want {
				t.Fatal("missing original boundaries", len(rows))
			}
			for i, r := range rows {
				got, err := StepCourtAnimation(r.Before, r.Context)
				if err != nil || !reflect.DeepEqual(got, r.After) {
					t.Fatalf("row%d court%d frame%d camera%d before%+v: %s err %v", i, r.Court, r.Frame, r.Context.CameraX, r.Before, firstWorldLoopDifference(reflect.ValueOf(got), reflect.ValueOf(r.After), "animation"), err)
				}
			}
			t.Logf("matched %d original animation boundaries including emitted tiles, palettes, sounds, shared RNG and scores", len(rows))
		})
	}
}

func TestCourtAnimationInactiveVideoAndInvalidCourt(t *testing.T) {
	for court := uint8(0); court < 8; court++ {
		for _, ctx := range []CourtAnimationContext{{Court: court, Active: false, VideoFrame: 17, Random: 123, Scores: [2]uint16{10, 20}}, {Court: court, Active: true, VideoFrame: 18, Random: 123, Scores: [2]uint16{10, 20}}} {
			before := CourtAnimationState{WaterFrame: 3, GateFlags: 128, GateTicks: 5, SlotProgress: [2]uint8{61, 61}}
			got, err := StepCourtAnimation(before, ctx)
			if err != nil || got.State != before || got.Random != ctx.Random || got.Scores != ctx.Scores || len(got.Patches) != 0 || len(got.Sounds) != 0 || got.Palette {
				t.Fatal(court, got, err)
			}
		}
	}
	if _, err := StepCourtAnimation(CourtAnimationState{}, CourtAnimationContext{Court: 8, Active: true, VideoFrame: 1}); err == nil {
		t.Fatal("unknown court accepted")
	}
}
