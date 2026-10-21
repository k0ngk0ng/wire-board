package engine

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
	"reflect"
	"testing"
)

func runtimePresentationSourceShell(t *testing.T, ending bool) RuntimeState {
	t.Helper()
	var f struct {
		Cases []struct {
			Label            string
			StartingProgress campaign.Progress
			Before           struct {
				Random     RandomState
				Characters [4]uint8
			}
			Phases []struct {
				Kind     string
				RawAfter []byte
			}
		}
	}
	readPropAttackFixture(t, "testdata/campaign-settlement-cases.json.gz", &f)
	label := "0-0-1-0-empty-True"
	if ending {
		label = "0-0-1-26-empty-True"
	}
	for _, row := range f.Cases {
		if row.Label != label {
			continue
		}
		s, e := NewCampaignRuntime(row.StartingProgress, CampaignMatchSelection{Opponent: 1, Characters: row.Before.Characters, HumanTactics: [2]uint8{4, 4}, Seed: row.Before.Random})
		if e != nil {
			t.Fatal(e)
		}
		o := *s.Opening
		for _, phase := range row.Phases {
			if phase.Kind == "result-confirm" {
				o.Transition = transitionRAM(phase.RawAfter)
			}
		}
		// Original controlled confirmation boundary, completed actor tail; the
		// surrounding configuration/grades remain a semantic runtime shell.
		for actor := uint8(0); actor < 4; actor++ {
			r, e := StepOriginalResultActor(o, ResultActorContext{}, actor)
			if e != nil {
				t.Fatal(e)
			}
			o = r.State
		}
		s.Opening = nil
		s.Transition = &RuntimeTransitionState{Scene: o}
		s.LogicSteps = 1
		s.Outcome = &RuntimeOutcome{WinningTeam: 0, Scores: o.Transition.Scores, Confirmed: true, NextScene: o.Transition.Scene, NextPhase: o.Transition.Match.Phase}
		black := uint8(0)
		s.Display.PaletteLevel = &black
		s.Display.BallCHR = 13
		s.Display.PaletteID = o.Transition.Court + 1
		if e = ValidateRuntimeState(s); e != nil {
			t.Fatal("controlled runtime shell", e)
		}
		return s
	}
	t.Fatal("missing settlement source")
	return RuntimeState{}
}
func TestRuntimePresentationLegacyRemainsParked(t *testing.T) {
	s := runtimePresentationSourceShell(t, false)
	before, _ := json.Marshal(s)
	if !RuntimeFinished(s) || RuntimePresentationActive(s) || RuntimePresentationNeedsConfirm(s) {
		t.Fatal("legacy classification")
	}
	got, e := StepVideo(s, [4]Buttons{ButtonStart})
	if e != nil {
		t.Fatal(e)
	}
	after, _ := json.Marshal(got.State)
	if string(before) != string(after) {
		t.Fatal("legacy restore started animation or changed state")
	}
	if _, e = StepRecordedVideo(s, RecordedVideoInput{Frame: s.VideoFrames + 1}); e == nil {
		t.Fatal("legacy finished replay accepted")
	}
}
func TestRuntimePresentationControlledFullVideo(t *testing.T) {
	for _, ending := range []bool{false, true} {
		for _, early := range []bool{false, true} {
			if ending && early {
				continue
			}
			s := runtimePresentationSourceShell(t, ending)
			expected, e := SettleCampaignRuntime(s)
			if e != nil {
				t.Fatal(e)
			}
			video, e := NewOriginalPostResultVideoAfterResult(s.Transition.Scene, s.Display, *s.Config.Campaign, s.LogicFrame)
			if e != nil {
				t.Fatal(e)
			}
			s.Presentation = &RuntimePresentationState{PendingResult: expected, Video: &video}
			s.Transition = nil
			s.Carry = nil
			s.FramesUntilLogic = 0
			baseline := video
			immutable := *s.Outcome
			config := s.Config
			seenConfirm := false
			for tick := 0; tick < 15000 && !RuntimeFinished(s); tick++ {
				held := [4]Buttons{}
				if early && tick == 4 {
					held[0] = ButtonA
				}
				if RuntimePresentationNeedsConfirm(s) {
					seenConfirm = true
					held[0] = ButtonStart
				}
				pending := s.PendingPressed
				for i, h := range held {
					pending[i] |= h &^ s.Held[i]
				}
				var sample [4]ControllerFrame
				for i, h := range held {
					sample[i] = ControllerFrame{Held: h | pending[i], Pressed: pending[i]}
				}
				want, e := StepOriginalPostResultVideo(baseline, sample)
				if e != nil {
					t.Fatal(tick, e)
				}
				got, e := StepVideo(s, held)
				if e != nil {
					t.Fatal(ending, early, tick, e)
				}
				if got.State.Presentation == nil || got.State.Presentation.Video == nil || *got.State.Presentation.Video != want.State {
					t.Fatal("runtime doubled/skipped sourcevideo", ending, tick)
				}
				if *got.State.Outcome != immutable || !reflect.DeepEqual(got.State.Config, config) || got.State.Transition != nil || got.State.Playing != nil || got.State.Opening != nil {
					t.Fatal("immutable result/config or single stage violated", tick)
				}
				if got.LogicAdvanced != want.DispatchStarted || !reflect.DeepEqual(got.PresentationEffects, want.Effects) {
					t.Fatal("dispatch/effects", tick)
				}
				if want.DispatchStarted && got.State.PendingPressed != ([4]Buttons{}) {
					t.Fatal("sample replayed")
				}
				if !want.DispatchStarted && got.State.PendingPressed != pending {
					t.Fatal("blocked tap discarded")
				}
				if s.Presentation.Video.Resource.Kind != "" || want.State.Resource.Kind != "" || tick%64 == 0 {
					raw, _ := json.Marshal(s)
					var restored RuntimeState
					if e = json.Unmarshal(raw, &restored); e != nil {
						t.Fatal(e)
					}
					replay, e := StepVideo(restored, held)
					if e != nil || !reflect.DeepEqual(got, replay) {
						t.Fatal("restore", tick, e)
					}
					recorded, e := StepRecordedVideo(s, RecordedVideoInput{Frame: s.VideoFrames + 1, Held: held, PriorHeld: s.Held, PendingPressed: s.PendingPressed})
					if e != nil || !reflect.DeepEqual(got, recorded) {
						t.Fatal("recorded presentation diverged/double display commit", tick, e)
					}
				}
				s, baseline = got.State, want.State
			}
			if !RuntimeFinished(s) || RuntimePresentationActive(s) || ending && !seenConfirm {
				t.Fatal("presentation did not complete")
			}
			result, e := SettleCampaignRuntime(s)
			if e != nil || result.Progress != baseline.Progress || result.Random != baseline.Scene.Transition.Random || result.Reward != expected.Reward || result.NextStage != expected.NextStage {
				t.Fatal("final settlement/RNG", e)
			}
			if ending && baseline.Scene.Transition.Match.Period != 0 {
				t.Fatal("source reset missing")
			}
			if result.Progress.Period != expected.Progress.Period {
				t.Fatal("read reset Period0 as progress")
			}
		}
	}
}
func TestRuntimePresentationEarnedRewardAndFade(t *testing.T) {
	s := runCampaignHomeForSettlement(t)
	if s.Presentation == nil || s.Presentation.Video != nil || !RuntimePresentationActive(s) || RuntimeFinished(s) {
		t.Fatal("new earned campaign win did not arm presentation")
	}
	outcome := *s.Outcome
	expected := s.Presentation.PendingResult
	initialRandom := s.Transition.Scene.Transition.Random
	for i := 0; i < 8; i++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(i, e)
		}
		s = r.State
		if i < 7 && (s.Presentation.Video != nil || s.Transition == nil) {
			t.Fatal("actor scene transferred before fade finished")
		}
	}
	if s.Presentation.Video == nil || s.Transition != nil || s.Carry != nil || s.WaitVideoFrames != 0 || s.FramesUntilLogic != 0 || s.Display.PaletteLevel == nil || *s.Display.PaletteLevel != 0 || s.Presentation.Video.Clock.Counter != 1 {
		t.Fatal("fade handoff boundary")
	}
	if s.Presentation.Video.Scene.Transition.Random != initialRandom {
		t.Fatal("bootstrap RNG draw")
	}
	first, e := StepVideo(s, [4]Buttons{})
	if e != nil || first.LogicAdvanced {
		t.Fatal("reward ignored remaining2NMI gate", e)
	}
	s = first.State
	for i := 0; i < 500 && !RuntimeFinished(s); i++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	result, e := SettleCampaignRuntime(s)
	if e != nil || !RuntimeFinished(s) || result.Progress != expected.Progress || *s.Outcome != outcome || result.Random == initialRandom {
		t.Fatal("earned reward lifecycle", e)
	}
	t.Logf("earned scores=%v confirmed inventory once, final video=%d", outcome.Scores, s.VideoFrames)
}
func TestRuntimePresentationCancelAutomationAndValidation(t *testing.T) {
	s := runtimePresentationSourceShell(t, false)
	expected, _ := SettleCampaignRuntime(s)
	v, e := NewOriginalPostResultVideoAfterResult(s.Transition.Scene, s.Display, *s.Config.Campaign, 0)
	if e != nil {
		t.Fatal(e)
	}
	s.Presentation = &RuntimePresentationState{PendingResult: expected, Video: &v}
	s.Transition = nil
	s.Held = [4]Buttons{ButtonA, ButtonB}
	s.PendingPressed = s.Held
	s.Presentation.Video.Clock.Input = [4]ControllerFrame{{Held: ButtonA, Pressed: ButtonA}}
	auto, e := SetRuntimeAutomation(s, [4]bool{true, true, true, true})
	if e != nil || !reflect.DeepEqual(auto, s) {
		t.Fatal("postresult automation remapped scene", e)
	}
	cleared, e := ClearRuntimeInput(s, 0)
	if e != nil || cleared.Held[0] != 0 || cleared.PendingPressed[0] != 0 || cleared.Held[1] != s.Held[1] || cleared.Presentation.Video.Clock.Input != s.Presentation.Video.Clock.Input {
		t.Fatal("cancel changed accepted sample", e)
	}
	for _, kind := range []string{"duplicate-scene", "changed-outcome", "wrong-inventory", "false-complete"} {
		bad := cloneRuntime(s)
		switch kind {
		case "duplicate-scene":
			bad.Transition = &RuntimeTransitionState{Scene: bad.Presentation.Video.Scene}
		case "changed-outcome":
			bad.Outcome.Scores[0]++
		case "wrong-inventory":
			bad.Presentation.PendingResult.Progress.Inventory[0] = campaign.Empty
		case "false-complete":
			bad.Presentation.Complete = true
		}
		if e := ValidateRuntimeState(bad); e == nil {
			t.Fatal("accepted corrupt presentation", kind)
		}
	}
}
