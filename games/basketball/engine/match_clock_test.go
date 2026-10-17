package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalMatchClockAndPeriodChecks(t *testing.T) {
	f, e := os.Open("testdata/match-clock.json.gz")
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
		Kind                              string
		Frame                             int
		Before, After                     MatchState
		Context                           MatchContext
		VideoBeforeClock, VideoAfterClock uint8
		Sounds                            []uint8
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	counts := map[string]int{}
	ended, decrements := 0, 0
	for _, r := range rows {
		s := r.Before
		s.Clock.VideoRemainder += r.VideoBeforeClock
		var got MatchStep
		switch r.Kind {
		case "video":
			got = AdvanceMatchVideoTick(s)
		case "clock":
			got = StepMatchClock(s)
		case "hud":
			got = StepMatchHUD(s, r.Context)
		case "pause":
			got = CheckMatchPause(s, r.Context.Buttons)
		case "resume":
			got = CheckMatchResume(s, r.Context.Buttons)
		case "end":
			got = CheckMatchPeriodEnd(s, r.Context)
		default:
			t.Fatal("unknown event", r.Kind)
		}
		got.State.Clock.VideoRemainder += r.VideoAfterClock
		if got.State != r.After {
			t.Fatalf("%s frame %d\nbefore %+v\ngot %+v\nwant %+v", r.Kind, r.Frame, r.Before, got.State, r.After)
		}
		if len(got.SoundIDs) != len(r.Sounds) || (len(r.Sounds) > 0 && !reflect.DeepEqual(got.SoundIDs, r.Sounds)) {
			t.Fatal("sound differs", r.Kind, r.Frame, got.SoundIDs, r.Sounds)
		}
		if r.Kind == "end" && r.Before.Phase != r.After.Phase {
			ended++
		}
		if r.Kind == "clock" && r.Before.Clock.Ones != r.After.Clock.Ones {
			decrements++
		}
		counts[r.Kind]++
	}
	wantCounts := map[string]int{"clock": 1824, "end": 7293, "hud": 7293, "pause": 7293, "resume": 10, "video": 22834}
	if !reflect.DeepEqual(counts, wantCounts) || ended != 2 || decrements != 367 {
		t.Fatal("missing clock/period coverage", counts, ended, decrements)
	}
	t.Logf("matched all %d original events: %v; %d seconds decremented and %d period end transitions", len(rows), counts, decrements, ended)
}
func TestMatchPauseAndWrapSemantics(t *testing.T) {
	s := MatchState{Phase: MatchPlaying, Entered: true, Clock: MatchClock{Minutes: 3, Ones: 1, VideoRemainder: 255}}
	s = AdvanceMatchVideoTick(s).State
	if s.Clock.VideoRemainder != 0 {
		t.Fatal(s)
	}
	paused := CheckMatchPause(s, 0x10).State
	if paused.Phase != MatchPaused || paused.Entered || AdvanceMatchVideoTick(paused).State != paused {
		t.Fatal(paused)
	}
	if CheckMatchPause(s, 0x11).State != s {
		t.Fatal("start combined with another edge paused")
	}
	resumed := CheckMatchResume(paused, 0x11).State
	if resumed.Phase != MatchPlaying || !resumed.Entered {
		t.Fatal(resumed)
	}
	next := StepMatchClock(s)
	if next.State.Clock.VideoRemainder != 196 || next.State.Clock.Minutes != 3 || next.State.Clock.Ones != 0 || next.State.Clock.DisplayedSecond != 1 {
		t.Fatal(next)
	}
	for _, n := range []uint8{60, 128, 255} {
		s.Clock.VideoRemainder = n
		if StepMatchClock(s).State != s {
			t.Fatal("decremented above wrapped threshold", n)
		}
	}
}
func TestMatchEndFreeBallOvertimeAndScoreCache(t *testing.T) {
	s := MatchState{Phase: MatchPlaying, Entered: true}
	ctx := MatchContext{Holder: 6, BallAirborne: true, Scores: [2]uint16{0, 3}}
	if CheckMatchPeriodEnd(s, ctx).State != s {
		t.Fatal("ended before free ball landed")
	}
	ctx.Holder = 0
	got := CheckMatchPeriodEnd(s, ctx)
	if got.State.Phase != MatchPeriodEnding || got.State.Entered || !reflect.DeepEqual(got.SoundIDs, []uint8{55}) {
		t.Fatal(got)
	}
	s.Period = 1
	got = CheckMatchPeriodEnd(s, ctx)
	if got.State.Phase != MatchPlaying || got.State.OvertimeFlags != 128 || got.State.Clock != s.Clock {
		t.Fatal("tied displayed score must enter untimed overtime", got)
	}
	if again := CheckMatchPeriodEnd(got.State, ctx); again.State != got.State || len(again.SoundIDs) != 0 {
		t.Fatal("repeated overtime whistle", again)
	}
	s = got.State
	s.DisplayedScores[1] = [3]uint8{0, 0, 3}
	ctx.Holder = 6
	if CheckMatchPeriodEnd(s, ctx).State != s {
		t.Fatal("sudden-death winning shot still airborne")
	}
	ctx.BallAirborne = false
	got = CheckMatchPeriodEnd(s, ctx)
	if got.State.Phase != MatchPeriodEnding {
		t.Fatal(got)
	}
}
func TestHUDSelectionScoreCachingAndLateWhistles(t *testing.T) {
	s := MatchState{Clock: MatchClock{Ones: 3, DisplayedSecond: 3, HUDTurn: 0}}
	c := MatchContext{Scores: [2]uint16{0, 133}}
	first := StepMatchHUD(s, c)
	if first.State.Clock.HUDTurn != 3 || first.State.DisplayedScores[1] != ([3]uint8{1, 3, 3}) || first.State.Clock.ScoreLowCache[1] != 133 || first.State.Clock.Ones != 3 {
		t.Fatal(first)
	}
	c.FrameParity = 1
	if StepMatchHUD(first.State, c).State != first.State {
		t.Fatal("odd frame updated HUD")
	}
	c.FrameParity = 0
	second := StepMatchHUD(first.State, c)
	if second.State.Clock.HUDTurn != 3 {
		t.Fatal("clean four-slot scan should wrap to original slot", second)
	}
	for _, n := range []uint8{3, 2, 1} {
		s.Clock.Ones = n
		s.Clock.VideoRemainder = 0
		r := StepMatchClock(s)
		want := 1
		if n == 1 {
			want = 0
		}
		if len(r.SoundIDs) != want || r.State.Clock.Ones != n-1 {
			t.Fatal(r)
		}
	}
}
