package rootcontrol_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type orchestrationVoice struct {
	Active, NoteOff, Claimed, Tie               bool
	Transpose                                   int8
	BaseLevel, NoteWait                         uint8
	BasePitch                                   uint16
	GateRemaining, GateTicks, ModDelay, ModRate uint8
	ModDelta                                    int8
	ModSteps                                    uint8
	ModOffset                                   int16
	Detune                                      int8
}
type orchestrationScore struct {
	TempoRemaining, TempoPeriod uint8
	Voices                      [18]orchestrationVoice
}

func orchestrationProjection(state score.State) orchestrationScore {
	result := orchestrationScore{TempoRemaining: state.TempoRemaining, TempoPeriod: state.TempoPeriod}
	for i, v := range state.Voices {
		pitch := v.BasePitch
		if i == 0 {
			pitch = 0
		}
		result.Voices[i] = orchestrationVoice{v.Active, v.NoteOff, v.Claimed, v.Tie, v.Transpose, v.BaseLevel, v.NoteWait, pitch, v.GateRemaining, v.GateTicks, v.ModDelay, v.ModRate, v.ModDelta, v.ModSteps, v.ModOffset, v.Detune}
	}
	return result
}
func orchestrationCatalog(t *testing.T) *score.Catalog {
	t.Helper()
	f, err := os.Open("../../data/audio-score.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	c, err := score.DecodeCatalog(z)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type orchestrationFrame struct {
	Step               int      `json:"step"`
	InputQueue         [4]uint8 `json:"input_queue"`
	InputPause         uint8    `json:"input_pause"`
	AfterTickPause     uint8    `json:"after_tick_pause"`
	WorldPauseRequest  *uint8   `json:"world_pause_request"`
	ScoreIsZero        bool     `json:"score_is_zero"`
	NativePCMModeFlags uint8    `json:"native_pcm_mode_flags"`
	Root               struct {
		Priority, Selected, FadeSteps, FadeRemaining, AudioPause, ControlMode, FMDelta, PSGDelta, DuckPhase, ChoiceBits uint8
		Queue                                                                                                           [4]uint8
	} `json:"root"`
	Score    orchestrationScore `json:"score"`
	Requests []score.Intent     `json:"requests"`
}
type orchestrationCase struct {
	ID     string               `json:"id"`
	Frames []orchestrationFrame `json:"frames"`
}
type orchestrationFixture struct {
	Version           int                       `json:"version"`
	FightCandidateIDs []uint8                   `json:"fight_candidate_ids"`
	Priorities        engine.SoundPriorityTable `json:"priorities"`
	Cases             []orchestrationCase       `json:"cases"`
}

func orchestrationOriginal(t *testing.T) orchestrationFixture {
	return loadOrchestrationFixture(t, "testdata/orchestrator-native-v3.json.gz", 5)
}
func loadOrchestrationFixture(t *testing.T, path string, count int) orchestrationFixture {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	body, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var fixture orchestrationFixture
	if err = json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Cases) != count {
		t.Fatal("retained native matrix changed")
	}
	return fixture
}
func replayOrchestrationCases(t *testing.T, fixture orchestrationFixture, callableStress bool, startAt int) (int, int) {
	t.Helper()
	catalog := orchestrationCatalog(t)
	frames, requests := 0, 0
	for _, test := range fixture.Cases {
		t.Run(test.ID, func(t *testing.T) {
			d, err := score.New(catalog)
			if err != nil {
				t.Fatal(err)
			}
			root, queue, pause := rootcontrol.InitialState(), [4]uint8{}, uint8(0)
			if startAt > 0 {
				prior := test.Frames[startAt-1]
				if !prior.ScoreIsZero || prior.Score != (orchestrationScore{}) || prior.Root.AudioPause != 0 || prior.Root.ControlMode != 0 || prior.Root.FMDelta != 0 || prior.Root.PSGDelta != 0 || prior.Root.DuckPhase != 0 {
					t.Fatal("checkpoint is not the proved typed idle profile")
				}
				root, err = rootcontrol.RestoreState(rootcontrol.State{Priority: prior.Root.Priority, Selected: prior.Root.Selected, ChoiceBits: prior.Root.ChoiceBits, Fade: rootcontrol.FadeState{Steps: prior.Root.FadeSteps, Remaining: prior.Root.FadeRemaining}})
				if err != nil {
					t.Fatal(err)
				}
				d, err = score.Restore(catalog, score.State{Ticks: uint64(startAt)})
				if err != nil {
					t.Fatal(err)
				}
			}
			chip, err := chips.NewYM(chips.ResetYMState())
			if err != nil {
				t.Fatal(err)
			}
			// The retained quiet checkpoint follows original Root 04. Its
			// physical TL baseline is mute, rather than hardware power-on zero.
			for _, w := range rootcontrol.PauseMuteWrites() {
				if err = chip.WriteRegister(w.Bank, w.Register, w.Value); err != nil {
					t.Fatal(err)
				}
			}
			psgChip, err := psg.New(psg.IntegratedHQ150Stereo, psg.ResetState())
			if err != nil {
				t.Fatal(err)
			}
			for localIndex, frame := range test.Frames[startAt:] {
				index := localIndex + startAt
				if frame.Step != index {
					t.Fatal("native step order changed")
				}
				// Native World/Scheduler input, delivered before this actual tick.
				queue, pause = frame.InputQueue, frame.InputPause
				actual := []score.Intent{}
				write := func(w score.Intent) error {
					if w.Kind != "ym" && w.Kind != "psg" {
						return nil
					}
					w.Order = 0
					actual = append(actual, w)
					if w.Kind == "psg" {
						_, err := psgChip.Write(psg.Event{Data: w.Value})
						return err
					}
					return chip.WriteRegister(w.Bank, w.Register, w.Value)
				}
				hooks := rootcontrol.RuleHooks{
					Operation: func(op rootcontrol.RuleOperation) error {
						if op.Kind == rootcontrol.RuleChipWrite {
							return write(op.Write)
						}
						return nil
					},
					Score: score.ObservedHooks{Operation: func(op score.Operation) error {
						for _, w := range op.Writes[:op.WriteCount] {
							if err := write(w); err != nil {
								return err
							}
						}
						return nil
					}, ReadDAC: func(score.FeedbackRead) (score.DACFeedback, error) { return score.DACFeedback{}, nil }},
					CaptureLevels: func() (rootcontrol.PauseLevels, error) { return rootcontrol.CapturePauseLevels(chip.Snapshot()) },
					AcceptDispatch: func(id uint8) error {
						if id != 0x73 && id != 0xa7 {
							return fmt.Errorf("unexpected callable stress id %02x", id)
						}
						return nil
					},
					DispatchExternal: func(id uint8) error {
						if id != 0x73 {
							return fmt.Errorf("unexpected external dispatch %02x", id)
						}
						return nil
					},
				}
				if !callableStress {
					hooks.AcceptDispatch = nil
					hooks.DispatchExternal = nil
				}
				out, err := rootcontrol.AdvanceRules(&root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}, rootcontrol.RuleInput{Priorities: fixture.Priorities}, hooks)
				if err != nil {
					t.Fatalf("step%d: %v", index, err)
				}
				if root.Priority != frame.Root.Priority || root.Selected != frame.Root.Selected || root.ChoiceBits != frame.Root.ChoiceBits || root.Fade != (rootcontrol.FadeState{Steps: frame.Root.FadeSteps, Remaining: frame.Root.FadeRemaining}) || queue != frame.Root.Queue || pause != frame.AfterTickPause {
					t.Fatalf("step%d Root/world: got %+v queue%v pause%d; native %+v", index, root, queue, pause, frame.Root)
				}
				if got := orchestrationProjection(d.Snapshot()); got != frame.Score {
					for slot, v := range frame.Score.Voices {
						if got.Voices[slot] != v {
							t.Fatalf("step%d voice%d got%+v native%+v", index, slot, got.Voices[slot], v)
						}
					}
					t.Fatalf("step%d tempo got%+v native%+v", index, got, frame.Score)
				}
				if !reflect.DeepEqual(actual, frame.Requests) {
					for i := 0; i < len(actual) && i < len(frame.Requests); i++ {
						if !reflect.DeepEqual(actual[i], frame.Requests[i]) {
							t.Fatalf("step%d request%d Go%+v native%+v (count%d/%d)", index, i, actual[i], frame.Requests[i], len(actual), len(frame.Requests))
						}
					}
					t.Fatalf("step%d request count%d/%d", index, len(actual), len(frame.Requests))
				}
				if out.Route != rootcontrol.RuleNormal && root.PauseRestore == nil && pause == 255 {
					t.Fatal("physical mute lost pending restore payload")
				}
				// This World producer runs after Root's return in these source cases.
				if frame.WorldPauseRequest != nil {
					pause = *frame.WorldPauseRequest
				}
				if pause != frame.Root.AudioPause {
					t.Fatal("World post-tick pause producer differs", index, pause, frame.Root.AudioPause)
				}
				if d.Snapshot().Ticks != uint64(index+1) {
					t.Fatal("tick counted more than once")
				}
				if index == len(test.Frames)/2 { // Resume/checkpoint continuation uses only each unique owner's state.
					root, err = rootcontrol.RestoreState(root)
					if err != nil {
						t.Fatal(err)
					}
					d, err = score.Restore(catalog, d.Snapshot())
					if err != nil {
						t.Fatal(err)
					}
				}
				frames++
				requests += len(actual)
			}
		})
	}
	return frames, requests
}
func TestOrchestratorRetainedNative785FramesV3(t *testing.T) {
	frames, requests := replayOrchestrationCases(t, orchestrationOriginal(t), true, 0)
	if frames != 785 || requests != 7199 {
		t.Fatal("incomplete native replay", frames, requests)
	}
	t.Logf("%d retained native frames; %d ordered effective YM/PSG requests; unique-owner checkpoints", frames, requests)
}
func TestOrchestratorFightNative180FramesV3(t *testing.T) {
	fixture := loadOrchestrationFixture(t, "testdata/orchestrator-legal-v3.json.gz", 3)
	cases := []orchestrationCase{}
	for _, c := range fixture.Cases {
		if c.ID == "fight-mixed82-a6-clear04" || c.ID == "fight-mixed82-a6-pause" {
			cases = append(cases, c)
		}
	}
	if len(cases) != 2 {
		t.Fatal("legal score/control matrix changed")
	}
	fixture.Cases = cases
	frames, requests := replayOrchestrationCases(t, fixture, false, 0)
	if frames != 180 || requests < 1000 {
		t.Fatal("incomplete default139 native replay", frames, requests)
	}
	t.Logf("%d fight native frames; %d ordered effective YM/PSG requests; default139 profile without external dispatch service", frames, requests)
}
func TestOrchestratorFightActivePCM04TypedCheckpointV3(t *testing.T) {
	fixture := loadOrchestrationFixture(t, "testdata/orchestrator-legal-v3.json.gz", 3)
	cases := []orchestrationCase{}
	for _, c := range fixture.Cases {
		if c.ID == "fight-long1f-clear04" {
			cases = append(cases, c)
		}
	}
	if len(cases) != 1 {
		t.Fatal("legal PCM checkpoint matrix changed")
	}
	fixture.Cases = cases
	c := cases[0]
	if c.Frames[1].NativePCMModeFlags != 0xc1 || c.Frames[2].NativePCMModeFlags != 0xc1 || c.Frames[1].Root.ChoiceBits != 1 || c.Frames[2].InputQueue[0] != 4 {
		t.Fatal("native accepted-PCM/04 source precondition changed")
	}
	frames, requests := replayOrchestrationCases(t, fixture, false, 2)
	if frames != 148 || requests != 35 {
		t.Fatal("incomplete typed activePCM checkpoint replay", frames, requests)
	}
	t.Logf("%d rule frames from typed pre04 checkpoint, %d strict YM/PSG requests; native PCM active across04; ordinary1F acceptance/lifecycle remains outside this composition", frames, requests)
}

func TestOrchestratorFightCandidatePredicateMatchesSourceAudit(t *testing.T) {
	fixture := orchestrationOriginal(t)
	if len(fixture.FightCandidateIDs) != 139 {
		t.Fatal("fight source domain changed")
	}
	expected := [256]bool{}
	for _, id := range fixture.FightCandidateIDs {
		expected[id] = true
	}
	for id, want := range expected {
		if got := rootcontrol.IsFightCandidate(uint8(id)); got != want {
			t.Fatal("fight source domain differs", id, got, want)
		}
	}
}
