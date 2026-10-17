package engine

// Failures before runtime integration: reels/progress/RNG/score maintained in
// shadow copies; missing outcome byte; incorrect hoop owner/lock context; early
// HUD/palette display; lost values on save/period reset; future courts unlocked.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestRuntimeCourtSixSlotCommandsFromOriginal(t *testing.T) {
	f, e := os.Open("testdata/court-six-slot-observed.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []struct {
		Frame   int
		Before  CourtAnimationState
		Context CourtAnimationContext
		After   CourtAnimationStep
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	awards := 0
	for _, row := range rows {
		w := WorldLoopState{LogicFrame: row.Context.VideoFrame, Camera: CameraState{DisplayedX: uint16(row.Context.CameraX)}}
		a := &w.Preparation.Actions
		for i := 0; i < 4; i++ {
			a.Actors[i].Player.NewYorkPassage = row.Before.ReelFrames[i]
		}
		for i := 0; i < 2; i++ {
			a.Auxiliary[i].NewYorkPassage = row.Before.ReelFrames[i+4]
		}
		w.Preparation.CourtScoreFlags = row.Before.SlotProgress
		w.Preparation.CourtOutcomeFlags = row.Before.OutcomeFlags
		a.Random = row.Context.Random
		w.Preparation.Scores = row.Context.Scores
		for i := range a.Hoops {
			a.Hoops[i].Owner = row.Context.HoopOwners[i]
			a.Hoops[i].ScoreLock = row.Context.HoopLocks[i]
		}
		s := RuntimeState{Display: DisplayState{CourtEpoch: 1, HasVisible: true, PaletteID: 7, Visible: BasketballRenderSnapshot{Court: 6, PaletteID: 7}, HUD: RenderHUD{Scores: row.Context.Scores}}}
		phase := MatchPlaying
		if !row.Context.Active {
			phase = MatchPaused
		}
		sounds, e := observeRuntimeCourtAnimation(&s, &w, WorldLoopContext{Actions: WorldActionContext{CourtID: 6}}, MatchState{Phase: phase})
		if e != nil {
			t.Fatal(row.Frame, e)
		}
		got := CourtAnimationState{ReelFrames: [6]uint8{a.Actors[0].Player.NewYorkPassage, a.Actors[1].Player.NewYorkPassage, a.Actors[2].Player.NewYorkPassage, a.Actors[3].Player.NewYorkPassage, a.Auxiliary[0].NewYorkPassage, a.Auxiliary[1].NewYorkPassage}, SlotProgress: w.Preparation.CourtScoreFlags, OutcomeFlags: w.Preparation.CourtOutcomeFlags}
		if got != row.After.State || a.Random != row.After.Random || w.Preparation.Scores != row.After.Scores || !slices.Equal(sounds, row.After.Sounds) {
			t.Fatalf("source frame%d state/RNG/score/sound mismatch", row.Frame)
		}
		if !reflect.DeepEqual(append([]CourtTilePatch{}, s.Display.PendingPatches...), row.After.Patches) {
			t.Fatal("original tile command order", row.Frame)
		}
		if s.Display.Visible.PaletteID != 7 || s.Display.HUD.Scores != row.Context.Scores {
			t.Fatal("palette or HUD changed before original boundary", row.Frame)
		}
		checkpoint, _ := json.Marshal(s.Display)
		var restored DisplayState
		_ = json.Unmarshal(checkpoint, &restored)
		visible, e := CommitDisplayVideo(s.Display, 0, 0)
		if e != nil {
			t.Fatal(e)
		}
		replay, e := CommitDisplayVideo(restored, 0, 0)
		if e != nil || !reflect.DeepEqual(visible, replay) {
			t.Fatal("slot display restore divergence", row.Frame, e)
		}
		wantPalette := uint8(7)
		if row.After.Palette {
			wantPalette = row.After.PaletteID
		}
		if visible.Visible.PaletteID != wantPalette {
			t.Fatal("palette queue missing", row.Frame, wantPalette)
		}
		if row.After.Scores != row.Context.Scores {
			awards++
		}
	}
	if len(rows) != 7244 || awards != 1 {
		t.Fatal("missing observed natural calls/reward", len(rows), awards)
	}
}
func TestRuntimeCourtSixOutcomePersistsAndResets(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 6
	c.Match.Seed = 0x12345678
	s, e := NewRuntime(c)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; s.Playing == nil && i < 500; i++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if s.Playing == nil {
		t.Fatal("Court6 did not open")
	}
	s.Playing.World.Preparation.CourtScoreFlags = [2]uint8{32, 64}
	s.Playing.World.Preparation.CourtOutcomeFlags = 128
	before, _ := json.Marshal(s)
	var restored RuntimeState
	_ = json.Unmarshal(before, &restored)
	if e = ValidateRuntimeState(restored); e != nil {
		t.Fatal(e)
	}
	o, e := runtimeOpeningFromPlaying(*restored.Playing, c)
	if e != nil {
		t.Fatal(e)
	}
	if o.Transition.Passages[6] != 32 || o.Transition.Passages[7] != 64 || o.Transition.Passages[8] != 128 {
		t.Fatal("transition lost canonical slot bytes")
	}
	fresh, _, e := WorldFromOriginalOpening(o, OpeningWorldOptions{LogicFrame: restored.LogicFrame})
	if e != nil {
		t.Fatal(e)
	}
	if fresh.Preparation.CourtScoreFlags != ([2]uint8{32, 64}) || fresh.Preparation.CourtOutcomeFlags != 128 {
		t.Fatal("opening lost slot state")
	}
	reset := fresh
	reset.Preparation.CourtScoreFlags = [2]uint8{}
	reset.Preparation.CourtOutcomeFlags = 0
	merged := mergeRuntimeCarriedWorld(reset, fresh)
	if merged.Preparation.CourtOutcomeFlags != 0 || merged.Preparation.CourtScoreFlags != ([2]uint8{}) {
		t.Fatal("period inherited previous slot state")
	}
	c.Match.Court = 8
	if _, e = NewRuntime(c); e == nil {
		t.Fatal("out-of-range court accepted")
	}
}
func TestRuntimeCourtSixColdCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 6
	c.Match.Seed = 0x12345678
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtSixAllComputerCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 6
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtSixCooperativeCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 6
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, 1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtSixVersusCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 6
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, -1, 2, -1}
	verifyColdVideoRuntime(t, c, true)
}
