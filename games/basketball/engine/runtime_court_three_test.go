package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Court3 failures specified before wiring: device trigger and animation must
// share canonical 06D7/06D8 rather than independent state, use logic-frame and
// displayed camera, preserve unrelated bytes, and survive JSON video commits.
// All public controller configurations must finish both original periods.
func TestRuntimeCourtThreeColdCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 3
	c.Match.Seed = 0x12345678
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtThreeAllComputerCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 3
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}

func TestCourtThreeRuntimeAnimationMatchesOriginalCanonicalSlots(t *testing.T) {
	count, triggered := 0, 0
	for _, name := range []string{"court-animation.json.gz", "court-animation-probes.json.gz"} {
		f, err := os.Open("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		z, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		var rows []struct {
			Court   int
			Before  CourtAnimationState
			Context CourtAnimationContext
			After   CourtAnimationStep
		}
		if err = json.NewDecoder(z).Decode(&rows); err != nil {
			t.Fatal(err)
		}
		z.Close()
		f.Close()
		for _, row := range rows {
			if row.Court != 3 {
				continue
			}
			count++
			w := WorldLoopState{LogicFrame: row.Context.VideoFrame, Camera: CameraState{X: uint16(row.Context.CameraX) ^ 128, DisplayedX: uint16(row.Context.CameraX)}}
			a := &w.Preparation.Actions
			a.Actors[0].Player.NewYorkPassage = row.Before.GateFlags
			a.Actors[1].Player.NewYorkPassage = row.Before.GateTicks
			a.Actors[2].Player.NewYorkPassage = 71
			a.Actors[3].Player.NewYorkPassage = 72
			a.Auxiliary[0].NewYorkPassage = 73
			a.Auxiliary[1].NewYorkPassage = 74

			a.Random = row.Context.Random
			w.Preparation.Scores = row.Context.Scores
			s := RuntimeState{VideoFrames: uint64(row.Context.VideoFrame) ^ 1, Display: DisplayState{CourtEpoch: 1}}
			phase := MatchPlaying
			if !row.Context.Active {
				phase = MatchPaused
			}
			sounds, err := observeRuntimeCourtAnimation(&s, &w, WorldLoopContext{Actions: WorldActionContext{CourtID: 3}}, MatchState{Phase: phase})
			if err != nil {
				t.Fatal(err)
			}
			got := CourtAnimationState{GateFlags: a.Actors[0].Player.NewYorkPassage, GateTicks: a.Actors[1].Player.NewYorkPassage}
			if got != row.After.State || a.Actors[2].Player.NewYorkPassage != 71 || a.Actors[3].Player.NewYorkPassage != 72 || a.Auxiliary[0].NewYorkPassage != 73 || a.Auxiliary[1].NewYorkPassage != 74 || a.Random != row.After.Random || w.Preparation.Scores != row.After.Scores || len(sounds) != len(row.After.Sounds) {
				t.Fatalf("original Court3 adapter mismatch %s row%d: got%+v want%+v", name, count, got, row.After.State)
			}

			if len(s.Display.PendingPatches) != len(row.After.Patches) || len(row.After.Patches) > 0 && !reflect.DeepEqual(s.Display.PendingPatches, row.After.Patches) {
				t.Fatalf("Court3 queued wrong original tiles at camera%d", row.Context.CameraX)
			}
			if row.Before.GateFlags&128 != 0 {
				triggered++
			}
			before, _ := json.Marshal(s.Display)
			var restored DisplayState
			if err = json.Unmarshal(before, &restored); err != nil {
				t.Fatal(err)
			}
			x, e := CommitDisplayVideo(s.Display, 0, 0)
			if e != nil {
				t.Fatal(e)
			}
			y, e := CommitDisplayVideo(restored, 0, 0)
			if e != nil || !reflect.DeepEqual(x, y) {
				t.Fatal("restored Court3 tile commit changed", e)
			}
		}
	}
	if count < 1138 || triggered < 256 {
		t.Fatal("original Court3 probe coverage missing", count, triggered)
	}
	t.Logf("%d original Court3 calls/probes through canonical runtime adapter, %d triggered probes, JSON video commit per case", count, triggered)
}

func TestRuntimeCourtThreeCooperativeCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 3
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, 1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtThreeVersusCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 3
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, -1, 2, -1}
	verifyColdVideoRuntime(t, c, true)
}
