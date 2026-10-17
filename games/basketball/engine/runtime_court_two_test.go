package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Before Court2 integration: flags must share original 06D7..06DB bytes with
// motion, the alternating side and countdown must survive saves, animation
// must use 0300/0397 rather than video counter/new camera, and both control
// configurations must complete normal two-period matches without state seeding.
func TestRuntimeCourtTwoColdCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 2
	c.Match.Seed = 0x12345678
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtTwoAllComputerCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 2
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}

func TestCourtTwoRuntimeAnimationMatchesOriginalCanonicalSlots(t *testing.T) {
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
			if row.Court != 2 {
				continue
			}
			count++
			w := WorldLoopState{LogicFrame: row.Context.VideoFrame, Camera: CameraState{X: uint16(row.Context.CameraX) ^ 128, DisplayedX: uint16(row.Context.CameraX)}}
			a := &w.Preparation.Actions
			a.Actors[0].Player.NewYorkPassage = row.Before.AlternatingSide
			a.Actors[1].Player.NewYorkPassage = row.Before.SwitchFlags[0]
			a.Actors[2].Player.NewYorkPassage = row.Before.SwitchFlags[1]
			a.Actors[3].Player.NewYorkPassage = row.Before.SwitchTicks[0]
			a.Auxiliary[0].NewYorkPassage = row.Before.SwitchTicks[1]
			a.Auxiliary[1].NewYorkPassage = 77 // 06DC is not this animation's state.
			a.Random = row.Context.Random
			w.Preparation.Scores = row.Context.Scores
			s := RuntimeState{VideoFrames: uint64(row.Context.VideoFrame) ^ 1, Display: DisplayState{CourtEpoch: 1}}
			phase := MatchPlaying
			if !row.Context.Active {
				phase = MatchPaused
			}
			sounds, err := observeRuntimeCourtAnimation(&s, &w, WorldLoopContext{Actions: WorldActionContext{CourtID: 2}}, MatchState{Phase: phase})
			if err != nil {
				t.Fatal(err)
			}
			got := CourtAnimationState{AlternatingSide: a.Actors[0].Player.NewYorkPassage, SwitchFlags: [2]uint8{a.Actors[1].Player.NewYorkPassage, a.Actors[2].Player.NewYorkPassage}, SwitchTicks: [2]uint8{a.Actors[3].Player.NewYorkPassage, a.Auxiliary[0].NewYorkPassage}}
			if got != row.After.State || a.Auxiliary[1].NewYorkPassage != 77 || a.Random != row.After.Random || w.Preparation.Scores != row.After.Scores || len(sounds) != len(row.After.Sounds) {
				t.Fatalf("original Court2 adapter mismatch %s row%d: got%+v want%+v", name, count, got, row.After.State)
			}
			if len(s.Display.PendingPatches) != len(row.After.Patches) || len(row.After.Patches) > 0 && !reflect.DeepEqual(s.Display.PendingPatches, row.After.Patches) {
				t.Fatalf("Court2 queued wrong original tiles at camera%d", row.Context.CameraX)
			}
			if row.Before.SwitchFlags[0]&128 != 0 || row.Before.SwitchFlags[1]&128 != 0 {
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
				t.Fatal("restored Court2 tile commit changed", e)
			}
		}
	}
	if count < 3072 || triggered < 1536 {
		t.Fatal("original Court2 probe coverage missing", count, triggered)
	}
	t.Logf("%d original Court2 calls/probes through canonical runtime adapter, %d triggered probes, JSON video commit per case", count, triggered)
}
