package engine

// Failure inventory before runtime integration: initial column48 mistaken for
// last generated47; column source updated after the launch; no-scroll updates
// lose retained source; JSON/next-period restores the wrong phase; missing or
// invalid alias silently uses player0; Court5 animation uses different slots or
// future courts are enabled; normal clocks/seeds cannot complete all modes.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func courtFivePlaying(t *testing.T) RuntimeState {
	t.Helper()
	c := DefaultRuntimeConfig()
	c.Match.Court = 5
	c.Match.Seed = 0x12345678
	s, e := NewRuntime(c)
	if e != nil {
		t.Fatal(e)
	}
	for tick := 0; s.Playing == nil && tick < 500; tick++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(tick, e)
		}
		s = r.State
	}
	if s.Playing == nil {
		t.Fatal("court5 failed opening handoff")
	}
	return s
}

func TestCourtFiveGeneratedColumnPrecedesUnownedLaunch(t *testing.T) {
	s := courtFivePlaying(t)
	w := s.Playing.World
	if w.LaunchColumn == nil || *w.LaunchColumn != (CourtColumnCommit{Column: 47, Phase: 0}) || w.Camera.TileColumn != 48 {
		t.Fatal("initial source is the last generated column47, not loop-end48", w.LaunchColumn, w.Camera)
	}
	w.Camera.X = 200
	w.Started = true
	a := &w.Preparation.Actions
	a.Holder = 6
	a.Ball = Ball{Body: Body{Position: Position{X: 400 * Pixel, Depth: 40 * Pixel}}, Action: BallRest, Entered: true}
	a.SelectedHoop = 7
	a.MountFlags[0] = 1
	a.Auxiliary[0] = AuxiliaryState{Body: Body{Position: Position{X: 150 * Pixel, Depth: 40 * Pixel, Height: 32 * Pixel}}, Owner: -1, Action: 2}
	encoded, e := json.Marshal(w)
	if e != nil {
		t.Fatal(e)
	}
	var restored WorldLoopState
	if e = json.Unmarshal(encoded, &restored); e != nil {
		t.Fatal(e)
	}
	got, e := StepWorldLoop(w, WorldLoopInput{}, s.Playing.Context)
	if e != nil {
		t.Fatal(e)
	}
	again, e := StepWorldLoop(restored, WorldLoopInput{}, s.Playing.Context)
	if e != nil || !reflect.DeepEqual(got, again) {
		t.Fatal("world column restore diverged", e)
	}
	if !got.StreamColumn || got.State.LaunchColumn == nil || *got.State.LaunchColumn != (CourtColumnCommit{Column: 57, Phase: 1}) || got.State.Preparation.Actions.LaunchTarget.Height != 88*Pixel {
		t.Fatal("launch used stale column or player height", got.State.LaunchColumn, got.State.Preparation.Actions.LaunchTarget)
	}
	if *w.LaunchColumn != (CourtColumnCommit{Column: 47, Phase: 0}) {
		t.Fatal("transaction mutated input pointer")
	}
	retained := got.State
	retained.Camera.X = 100
	retained.Preparation.Actions.Ball.Body.Position.X = 228 * Pixel
	quiet, e := StepWorldLoop(retained, WorldLoopInput{}, s.Playing.Context)
	if e != nil {
		t.Fatal(e)
	}
	if quiet.StreamColumn || !reflect.DeepEqual(quiet.State.LaunchColumn, retained.LaunchColumn) {
		t.Fatal("idle camera changed retained source")
	}
	fresh := s.Playing.World
	merged := mergeRuntimeCarriedWorld(fresh, got.State)
	if !reflect.DeepEqual(merged.LaunchColumn, fresh.LaunchColumn) {
		t.Fatal("next-period court initialization lost source47/0")
	}
	for _, column := range []*CourtColumnCommit{nil, {Column: 64}, {Column: 0, Phase: 2}} {
		bad := cloneRuntime(s)
		bad.Playing.World.LaunchColumn = column
		if e = ValidateRuntimeState(bad); e == nil {
			t.Fatal("invalid column source restored", column)
		}
	}
}

func TestCourtFiveRuntimeAnimationUsesOriginalCanonicalSlots(t *testing.T) {
	count := 0
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
		if e = json.NewDecoder(z).Decode(&rows); e != nil {
			t.Fatal(e)
		}
		z.Close()
		f.Close()
		for _, row := range rows {
			if row.Court != 5 {
				continue
			}
			count++
			w := WorldLoopState{LogicFrame: row.Context.VideoFrame, Camera: CameraState{DisplayedX: uint16(row.Context.CameraX)}}
			a := &w.Preparation.Actions
			a.Actors[0].Player.NewYorkPassage = row.Before.AlternatingSide
			a.Actors[1].Player.NewYorkPassage = row.Before.SwitchFlags[0]
			a.Actors[2].Player.NewYorkPassage = row.Before.SwitchFlags[1]
			a.Actors[3].Player.NewYorkPassage = row.Before.SwitchTicks[0]
			a.Auxiliary[0].NewYorkPassage = row.Before.SwitchTicks[1]
			a.Auxiliary[1].NewYorkPassage = 77
			a.Random = row.Context.Random
			w.Preparation.Scores = row.Context.Scores
			s := RuntimeState{Display: DisplayState{CourtEpoch: 1}}
			phase := MatchPlaying
			if !row.Context.Active {
				phase = MatchPaused
			}
			sounds, e := observeRuntimeCourtAnimation(&s, &w, WorldLoopContext{Actions: WorldActionContext{CourtID: 5}}, MatchState{Phase: phase})
			if e != nil {
				t.Fatal(e)
			}
			got := CourtAnimationState{AlternatingSide: a.Actors[0].Player.NewYorkPassage, SwitchFlags: [2]uint8{a.Actors[1].Player.NewYorkPassage, a.Actors[2].Player.NewYorkPassage}, SwitchTicks: [2]uint8{a.Actors[3].Player.NewYorkPassage, a.Auxiliary[0].NewYorkPassage}}
			if got != row.After.State || a.Auxiliary[1].NewYorkPassage != 77 || a.Random != row.After.Random || w.Preparation.Scores != row.After.Scores || len(sounds) != len(row.After.Sounds) || len(s.Display.PendingPatches) != len(row.After.Patches) || len(row.After.Patches) > 0 && !reflect.DeepEqual(s.Display.PendingPatches, row.After.Patches) {
				t.Fatal("court5 runtime source mismatch", name, count, got, row.After.State)
			}
		}
	}
	if count == 0 {
		t.Fatal("no Court5 animation source")
	}
	t.Logf("matched %d original Court5 animation calls/probes", count)
}

func TestRuntimeCourtFiveColdCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 5
	c.Match.Seed = 0x12345678
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtFiveAllComputerCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 5
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtFiveCooperativeCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 5
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, 1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtFiveVersusCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 5
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, -1, 2, -1}
	verifyColdVideoRuntime(t, c, true)
}
