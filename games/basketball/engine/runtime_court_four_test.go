package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Before Court4 integration: palette direction shares06D8 with moving props,
// palette frame shares06D7; preserve the raw direction byte. Palette writes
// must wait for the next video commit, survive JSON and not change snapshots
// early. New court configurations must complete both normal periods.
func TestRuntimeCourtFourColdCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 4
	c.Match.Seed = 0x12345678
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtFourAllComputerCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 4
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtFourCooperativeCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 4
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, 1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtFourVersusCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 4
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, -1, 2, -1}
	verifyColdVideoRuntime(t, c, true)
}

func TestCourtFourRuntimePaletteMatchesOriginalCalls(t *testing.T) {
	count, changes := 0, 0
	for _, name := range []string{"court-animation.json.gz", "court-animation-probes.json.gz"} {
		f, e := os.Open("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		z, e := gzip.NewReader(f)
		if e != nil {
			t.Fatal(e)
		}
		var rows []struct {
			Court   int
			Before  CourtAnimationState
			Context CourtAnimationContext
			After   CourtAnimationStep
		}
		e = json.NewDecoder(z).Decode(&rows)
		z.Close()
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
		for _, r := range rows {
			if r.Court != 4 {
				continue
			}
			count++
			w := WorldLoopState{LogicFrame: r.Context.VideoFrame, Camera: CameraState{DisplayedX: uint16(r.Context.CameraX)}}
			a := &w.Preparation.Actions
			a.Actors[0].Player.NewYorkPassage = r.Before.PaletteFrame
			if r.Before.PaletteReverse {
				a.Actors[1].Player.NewYorkPassage = 255
			}
			a.Actors[2].Player.NewYorkPassage = 73
			a.Random = r.Context.Random
			w.Preparation.Scores = r.Context.Scores
			s := RuntimeState{VideoFrames: uint64(r.Context.VideoFrame) ^ 1, Display: DisplayState{CourtEpoch: 1, HasVisible: true, HasSubmission: true, Visible: BasketballRenderSnapshot{Court: 4}, Submitted: BasketballRenderSnapshot{Court: 4}}}
			phase := MatchPlaying
			if !r.Context.Active {
				phase = MatchPaused
			}
			_, e := observeRuntimeCourtAnimation(&s, &w, WorldLoopContext{Actions: WorldActionContext{CourtID: 4}}, MatchState{Phase: phase})
			if e != nil {
				t.Fatal(e)
			}
			expectedReverse := uint8(0)
			if r.Before.PaletteReverse {
				expectedReverse = 255
			}
			if a.Actors[0].Player.NewYorkPassage != r.After.State.PaletteFrame || a.Actors[1].Player.NewYorkPassage != expectedReverse || a.Actors[2].Player.NewYorkPassage != 73 || a.Random != r.After.Random || w.Preparation.Scores != r.After.Scores {
				t.Fatal("canonical palette/motion state diverged", name, count)
			}
			if s.Display.PaletteID != 0 || s.Display.Visible.PaletteID != 0 {
				t.Fatal("palette became visible before video commit")
			}
			if r.After.Palette {
				changes++
				if s.Display.PendingPaletteID != r.After.PaletteID {
					t.Fatal("original palette command missing", count)
				}
			} else if s.Display.PendingPaletteID != 0 {
				t.Fatal("inactive palette advanced")
			}
			raw, _ := json.Marshal(s.Display)
			var restored DisplayState
			if e = json.Unmarshal(raw, &restored); e != nil {
				t.Fatal(e)
			}
			next, e := CommitDisplayVideo(s.Display, 0, 0)
			if e != nil {
				t.Fatal(e)
			}
			other, e := CommitDisplayVideo(restored, 0, 0)
			if e != nil || !reflect.DeepEqual(next, other) || next.PendingPaletteID != 0 {
				t.Fatal("palette commit did not survive restore", e)
			}
			if r.After.Palette && (next.Visible.PaletteID != r.After.PaletteID || next.PaletteID != r.After.PaletteID) {
				t.Fatal("video omitted palette command")
			}
		}
	}
	if count != 1650 || changes < 1536 {
		t.Fatal("missing original Court4 coverage", count, changes)
	}
	t.Logf("%d original Court4 calls/probes, %d palette changes, raw direction preserved and JSON video commits equal", count, changes)
}
func TestDisplayPaletteCommandsPersistWithoutNewSubmission(t *testing.T) {
	s := DisplayState{CourtEpoch: 1, HasVisible: true, PaletteID: 5, Visible: BasketballRenderSnapshot{Court: 4, PaletteID: 5}}
	queued, e := QueueDisplayPalette(s, 9)
	if e != nil {
		t.Fatal(e)
	}
	if queued.Visible.PaletteID != 5 || s.PendingPaletteID != 0 {
		t.Fatal("queue modified visible/original state")
	}
	next, e := CommitDisplayVideo(queued, 0, 0)
	if e != nil || next.Visible.PaletteID != 9 || next.PaletteID != 9 {
		t.Fatal("no-submission palette commit", e)
	}
	again, e := CommitDisplayVideo(next, 0, 0)
	if e != nil || !reflect.DeepEqual(next, again) {
		t.Fatal("palette did not persist", e)
	}
	for _, invalid := range []uint8{0, 16, 255} {
		got, e := QueueDisplayPalette(next, invalid)
		if e == nil || !reflect.DeepEqual(got, next) {
			t.Fatal("invalid command was not atomic")
		}
	}
}

// Original D231 resets selection at StartOriginalCourt, whereas the result
// handler BA67/BB37 retains it. Frame14362→14548/25508→25528 original records
// in court-4-palette-complete-palette-observed independently establish this.
func TestRuntimeCourtPaletteOpeningAndResultBoundaries(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 4
	s, e := NewRuntime(c)
	if e != nil {
		t.Fatal(e)
	}
	s.Display.PaletteID = 9
	s.Display.HasVisible = true
	s.Display.Visible = BasketballRenderSnapshot{Court: 4, PaletteID: 9}
	step := RuntimeStep{State: s}
	if e = stepRuntimeOpening(&step, [4]ControllerFrame{}); e != nil {
		t.Fatal(e)
	}
	s = step.State
	if s.Display.PaletteID != 9 || s.Display.PendingPaletteID != 0 {
		t.Fatal("palette reset before original D231")
	}
	if e = stepRuntimeOpening(&step, [4]ControllerFrame{}); e != nil {
		t.Fatal(e)
	}
	s = step.State
	if s.Display.PendingPaletteID != 5 || s.Display.Visible.PaletteID != 9 {
		t.Fatal("D231 did not queue base palette for next video")
	}
	s.Display, e = CommitDisplayVideo(s.Display, 0, 0)
	if e != nil || s.Display.Visible.PaletteID != 5 {
		t.Fatal("base palette not committed", e)
	}
	result := initializeRuntimeResultDisplay(DisplayState{PaletteID: 9, PendingPaletteID: 10, HasVisible: true, Visible: BasketballRenderSnapshot{Court: 4, PaletteID: 9}})
	result, e = CommitDisplayVideo(result, 0, 0)
	if e != nil || result.PaletteID != 10 || result.Visible.PaletteID != 10 {
		t.Fatal("result discarded most recent original palette selection", e)
	}
}

// A restored palette must be available for its court. Corrupt pending commands
// must not reach the renderer or turn a valid visible snapshot into an error.
func TestDisplayPaletteRestoreValidation(t *testing.T) {
	for _, palette := range []uint8{0, 5, 9, 10} {
		s := DisplayState{CourtEpoch: 1, HasVisible: true, HasSubmission: true, PaletteID: palette, PendingPaletteID: palette, Visible: BasketballRenderSnapshot{Court: 4, PaletteID: palette}, Submitted: BasketballRenderSnapshot{Court: 4, PaletteID: palette}}
		if e := validateRuntimeDisplay(s); e != nil {
			t.Fatal("valid Court4 palette rejected", palette, e)
		}
	}
	for _, palette := range []uint8{2, 16, 255} {
		for field := 0; field < 4; field++ {
			s := DisplayState{CourtEpoch: 1, HasVisible: true, HasSubmission: true, Visible: BasketballRenderSnapshot{Court: 4}, Submitted: BasketballRenderSnapshot{Court: 4}}
			switch field {
			case 0:
				s.PaletteID = palette
			case 1:
				s.PendingPaletteID = palette
			case 2:
				s.Visible.PaletteID = palette
			case 3:
				s.Submitted.PaletteID = palette
			}
			if e := validateRuntimeDisplay(s); e == nil {
				t.Fatal("invalid court palette accepted", palette, field)
			}
		}
	}
}
