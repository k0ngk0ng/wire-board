package basketball_test

// Failure inventory before implementation: late join guesses music from court;
// duplicate events restart a track; 253/255 reset time; a new music request
// retains pause incorrectly; 0 leaves old music; SQLite loses the clock; game
// pause incorrectly pauses BGM; asset revisions are not bound to the timeline.
import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
)

func TestMusicOriginalCommandsAndMediaClock(t *testing.T) {
	var state game.MusicState
	apply := func(ids []int, seq uint64, at int64) {
		t.Helper()
		if err := state.Apply(ids, seq, at); err != nil {
			t.Fatal(err)
		}
	}
	apply([]int{253, 255, 66}, 1, 1000)
	if state.TrackID != 66 || state.Paused || state.Epoch != 1 || len(state.AssetSHA256) != 64 || state.PositionMS(1600) != 600 {
		t.Fatal("invalid music start", state)
	}
	before := state
	apply([]int{66}, 1, 1700)
	if !reflect.DeepEqual(state, before) {
		t.Fatal("duplicate event restarted music")
	}
	apply([]int{19, 21}, 2, 1700)
	if !reflect.DeepEqual(state, before) {
		t.Fatal("effect inferred a music change")
	}
	apply([]int{253}, 3, 2000)
	if !state.Paused || state.PositionMS(9000) != 1000 {
		t.Fatal("pause did not freeze media clock", state)
	}
	apply([]int{255}, 4, 5000)
	if state.Paused || state.PositionMS(5600) != 1600 || state.Epoch != 1 {
		t.Fatal("resume restarted media", state)
	}
	apply([]int{253, 66}, 5, 6000)
	if state.Paused || state.Epoch != 2 || state.PositionMS(6500) != 500 {
		t.Fatal("new music must clear original pause", state)
	}
	apply([]int{253, 0}, 6, 7000)
	if state.TrackID != 0 || state.Paused || state.PositionMS(9000) != 0 || state.Epoch != 3 {
		t.Fatal("original clear retained music/pause", state)
	}
}

func TestHostMusicPersistsWithoutReplayingOldEffects(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	s := hostState(t, m)
	if s.Music.TrackID != 66 || s.Music.Paused {
		t.Fatal("original opening did not establish NY music", s.Music)
	}
	m = hostButtons(t, module, ctx, m, 0, "buttons", engine.ButtonStart)
	m = hostTick(t, module, &ctx, m)
	before := hostState(t, m)
	if before.Runtime.Playing.Match.Phase != engine.MatchPaused {
		t.Fatal("expected original game pause")
	}
	for i := 0; i < 5; i++ {
		m = hostTick(t, module, &ctx, m)
	}
	after := hostState(t, m)
	if after.Music.Paused || after.Music.PositionMS(after.ServerAt)-before.Music.PositionMS(before.ServerAt) != 500 {
		t.Fatal("game pause incorrectly stopped BGM")
	}
	ctx.Now = ctx.Now.Add(5 * time.Second)
	restored, err := module.Restore(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	got := hostState(t, restored)
	if !reflect.DeepEqual(got.Music, after.Music) || !reflect.DeepEqual(got.Events, after.Events) {
		t.Fatal("restore rewrote music or replayed effects")
	}
	view, err := module.View(restored, -1)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(view)
	var v struct {
		Music struct {
			TrackID     int    `json:"trackID"`
			PositionMS  int64  `json:"positionMS"`
			At          int64  `json:"at"`
			AssetSHA256 string `json:"assetSHA256"`
		} `json:"music"`
	}
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if v.Music.TrackID != 66 || v.Music.At != got.ServerAt || v.Music.PositionMS != got.Music.PositionMS(got.ServerAt) || v.Music.AssetSHA256 != got.Music.AssetSHA256 {
		t.Fatal("spectator lacks authoritative music position", v)
	}
	writeVideoFixture(t, "host-music-view.json", view)
}
