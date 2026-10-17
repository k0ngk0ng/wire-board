package engine

// Before implementation: logic writes visible tiles before video commit; a
// streamed column resurrects an older patch after restore; patch-only commits
// don't change serial; histories grow without bound; shared slices mutate an
// earlier snapshot; malformed patches panic; reset leaves prior-court patches.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDisplayCourtPatchesCommitOverwriteAndRestore(t *testing.T) {
	d := DisplayState{CourtEpoch: 1}
	patches := []CourtTilePatch{{Plane: 1, Row: 15, Column: 3, Tiles: []int{10, 11}}, {Plane: 0, Row: 8, Column: 3, Tiles: []int{12}}}
	queued, err := QueueDisplayPatches(d, patches)
	if err != nil {
		t.Fatal(err)
	}
	patches[0].Tiles[0] = 200
	if len(queued.CourtPatches) != 0 || queued.CourtSerial != 0 {
		t.Fatal("logic published patches before video")
	}
	committed, err := CommitDisplayVideo(queued, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(committed.CourtPatches) != 3 || committed.CourtSerial != 1 || len(committed.PendingPatches) != 0 {
		t.Fatal(committed)
	}
	original, _ := json.Marshal(committed)
	next, err := QueueDisplayColumn(committed, 35, 1)
	if err != nil {
		t.Fatal(err)
	}
	next, err = QueueDisplayPatches(next, []CourtTilePatch{{Plane: 1, Row: 15, Column: 3, Tiles: []int{20}}})
	if err != nil {
		t.Fatal(err)
	}
	next, err = CommitDisplayVideo(next, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if next.CourtSerial <= committed.CourtSerial || len(next.CourtPatches) != 2 {
		t.Fatal(next)
	}
	for _, p := range next.CourtPatches {
		if p.Column == 3 && (p.Plane != 1 || p.Tiles[0] != 20) {
			t.Fatal("column retained old tile", p)
		}
	}
	after, _ := json.Marshal(committed)
	if string(original) != string(after) {
		t.Fatal("earlier snapshot mutated")
	}
	data, _ := json.Marshal(next)
	var restored DisplayState
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if err = validateRuntimeCourtAnimationDisplay(restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, next) {
		t.Fatal("restore lost ordered remaining patches")
	}
	idle, err := CommitDisplayVideo(restored, 0, 0)
	if err != nil || !reflect.DeepEqual(idle, restored) {
		t.Fatal("restore replayed old patch", err)
	}
	reset := ResetDisplayCourt(restored)
	if len(reset.CourtPatches) != 0 || len(reset.PendingPatches) != 0 || reset.CourtSerial != 0 {
		t.Fatal(reset)
	}
}

func TestDisplayCourtPatchHistoryBoundedAndValidated(t *testing.T) {
	d := DisplayState{CourtEpoch: 1}
	var all []CourtTilePatch
	for plane := uint8(0); plane < 2; plane++ {
		for row := uint8(0); row < 30; row++ {
			if plane == 0 && row < 6 {
				continue
			}
			for column := uint8(0); column < 32; column++ {
				all = append(all, CourtTilePatch{Plane: plane, Row: row, Column: column, Tiles: []int{int(row)}})
			}
		}
	}
	for iteration := 0; iteration < 3; iteration++ {
		var err error
		d, err = QueueDisplayPatches(d, all)
		if err != nil {
			t.Fatal(err)
		}
		d, err = CommitDisplayVideo(d, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(d.CourtPatches) != 1728 {
			t.Fatal("unbounded or missing tiles", len(d.CourtPatches))
		}
	}
	for _, p := range []CourtTilePatch{{Plane: 2, Row: 1, Tiles: []int{1}}, {Plane: 0, Row: 5, Tiles: []int{1}}, {Plane: 1, Row: 30, Tiles: []int{1}}, {Plane: 1, Column: 31, Tiles: []int{1, 2}}, {Plane: 1, Tiles: []int{256}}, {Plane: 1}} {
		got, err := QueueDisplayPatches(d, []CourtTilePatch{p})
		if err == nil || !reflect.DeepEqual(got, d) {
			t.Fatal("invalid patch changed display", p, err)
		}
	}
	bad := d
	bad.CourtPatches = append(append([]CourtTilePatch(nil), d.CourtPatches...), d.CourtPatches[0])
	if validateRuntimeCourtAnimationDisplay(bad) == nil {
		t.Fatal("duplicate tile history accepted")
	}
}

func TestCourtZeroRuntimeAnimationUsesCanonicalLogicCounter(t *testing.T) {
	state := RuntimeState{VideoFrames: 100, Display: DisplayState{CourtEpoch: 1}}
	world := WorldLoopState{LogicFrame: 123, Camera: CameraState{X: 160, DisplayedX: 0}}
	world.Preparation.Actions.Actors[0].Player.NewYorkPassage = 2
	ctx := WorldLoopContext{Actions: WorldActionContext{CourtID: 0}}
	sounds, err := observeRuntimeCourtAnimation(&state, &world, ctx, MatchState{Phase: MatchPlaying})
	if err != nil || len(sounds) != 0 || world.Preparation.Actions.Actors[0].Player.NewYorkPassage != 3 || len(state.Display.PendingPatches) != 42 || len(state.Display.CourtPatches) != 0 {
		t.Fatal(world, state.Display, sounds, err)
	}
	world.LogicFrame = 124
	state.VideoFrames = 101
	before := state.Display
	_, err = observeRuntimeCourtAnimation(&state, &world, ctx, MatchState{Phase: MatchPlaying})
	if err != nil || world.Preparation.Actions.Actors[0].Player.NewYorkPassage != 3 || !reflect.DeepEqual(before, state.Display) {
		t.Fatal("video parity was used instead of $0300")
	}
	world.LogicFrame = 125
	_, err = observeRuntimeCourtAnimation(&state, &world, ctx, MatchState{Phase: MatchPaused})
	if err != nil || world.Preparation.Actions.Actors[0].Player.NewYorkPassage != 3 {
		t.Fatal("paused court advanced")
	}
	ctx.Actions.CourtID = 1
	_, err = observeRuntimeCourtAnimation(&state, &world, ctx, MatchState{Phase: MatchPlaying})
	if err != nil || world.Preparation.Actions.Actors[0].Player.NewYorkPassage != 3 {
		t.Fatal("New York passage alias changed")
	}
}

func TestCourtZeroRuntimeWaterVideoAndRestore(t *testing.T) {
	config := DefaultRuntimeConfig()
	config.Match.Court = 0
	config.Match.Seed = 0x12345678
	state, err := NewRuntime(config)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; state.Playing == nil && n < 400; n++ {
		step, e := StepVideo(state, [4]Buttons{})
		if e != nil {
			t.Fatal(n, e)
		}
		state = step.State
	}
	if state.Playing == nil {
		t.Fatal("court zero did not enter gameplay")
	}
	changes, commits := 0, 0
	for video := 0; video < 36; video++ {
		prior := state
		raw, _ := json.Marshal(state)
		var restored RuntimeState
		if err = json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		a, e := StepVideo(state, [4]Buttons{})
		if e != nil {
			t.Fatal(video, e)
		}
		b, e := StepVideo(restored, [4]Buttons{})
		if e != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("restored court animation diverged", video, e)
		}
		state = a.State
		expected := prior.Playing.World.Preparation.Actions.Actors[0].Player.NewYorkPassage
		if a.LogicAdvanced && state.LogicFrame&1 != 0 {
			expected = (expected + 1) & 3
			changes++
		}
		if got := state.Playing.World.Preparation.Actions.Actors[0].Player.NewYorkPassage; got != expected {
			t.Fatalf("video%d logic%d worldlogic%d court%d phase%d water before%d got%d want%d", video, state.LogicFrame, state.Playing.World.LogicFrame, state.Playing.Context.Actions.CourtID, state.Playing.Match.Phase, prior.Playing.World.Preparation.Actions.Actors[0].Player.NewYorkPassage, got, expected)
		}
		if len(prior.Display.PendingPatches) > 0 {
			commits++
			if state.Display.CourtSerial <= prior.Display.CourtSerial || len(state.Display.CourtPatches) == 0 {
				t.Fatal("video did not commit queued water")
			}
		}
	}
	if changes < 4 || commits < 4 {
		t.Fatal("water animation did not progress through original video cadence", changes, commits)
	}
	t.Logf("36 real runtime videos, %d canonical water updates, %d video patch commits, JSON restored every tick", changes, commits)
}

func TestDisplayCourtFinalPatchOverwriteRoundTrips(t *testing.T) {
	d := DisplayState{CourtEpoch: 1, CourtPatches: []CourtTilePatch{{Plane: 1, Row: 20, Column: 4, Tiles: []int{9}}}}
	next, err := QueueDisplayColumn(d, 36, 1)
	if err != nil {
		t.Fatal(err)
	}
	next, err = CommitDisplayVideo(next, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(next)
	var restored DisplayState
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(next, restored) {
		t.Fatal("last column overwrite left noncanonical empty patch slice", next.CourtPatches, restored.CourtPatches)
	}
	if next.CourtPatches != nil {
		t.Fatal("empty optional history must be nil")
	}
}
