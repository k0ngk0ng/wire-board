package scoreclock_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type controlNative struct {
	Input struct {
		ID              string
		Work            scoreclock.ControlParameterWork
		Next            string
		Origin, Refresh uint64
	}
	Steps  [][]uint64
	Writes []struct {
		At    uint64
		Value uint8
	}
	Queue []struct {
		At    uint64
		Value uint8
	}
	End   scoreclock.Clock
	Equal bool `json:"native_observer_state_equal"`
}

func controlNatives(t *testing.T) []controlNative {
	t.Helper()
	var f struct {
		Version int
		Cases   []controlNative
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-control/native-control-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 208 {
		t.Fatal("formal control/native scope incomplete")
	}
	return f.Cases
}

type controlPSG struct {
	r      *mixed.Runtime
	writes []access
}

func (p *controlPSG) WritePSG(at uint64, value uint8) error {
	err := p.r.WritePSG(at, value)
	if err == nil {
		p.writes = append(p.writes, access{Kind: "psg", At: at, Value: value})
	}
	return err
}

func TestControlClockOriginalBodiesAndContinuations(t *testing.T) {
	for _, row := range controlNatives(t) {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("recorder changed original handler")
			}
			origin := scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}
			result, err := scoreclock.PriceControlWork(origin, row.Input.Work, scoreclock.Limits{})
			want := row.End
			if row.Input.Next == "queue" {
				want = origin
			}
			if row.Input.Next == "psg-noise" {
				for _, s := range row.Steps {
					if s[0] == 0x391a2 {
						want = scoreclock.Clock{Cycle: s[1], Refresh: s[2]}
						break
					}
				}
			}
			if err != nil || result.Clock != want || result.Next != row.Input.Next {
				t.Fatal("original control boundary differs", result, want, row.Input.Next, err)
			}
			if row.Input.Work.Action == "psg-noise" {
				owner, _, _ := newPauseMixed(t)
				psg := &controlPSG{r: owner.r}
				end, err := scoreclock.ServeNoiseControl(origin, row.Input.Work, psg, scoreclock.Limits{})
				if err != nil || end != row.End || len(psg.writes) != len(row.Writes) {
					t.Fatal("complete original noise control differs", end, row.End, err)
				}
				for i, w := range row.Writes {
					if psg.writes[i].At != w.At || psg.writes[i].Value != w.Value {
						t.Fatal("noise port timing/value differs")
					}
				}
			}
			if row.Input.Work.Action == "enqueue-replace-first" {
				if len(row.Queue) != 1 {
					t.Fatal("native queue mutation absent")
				}
				value := uint8(19)
				calls := 0
				var at uint64
				op := score.Operation{Version: 1, Kind: score.OpQueueReplace, WriteCount: 1, Writes: [2]score.Intent{{Kind: "queue", Action: "replace-first", SoundID: row.Queue[0].Value}}}
				end, err := scoreclock.ServeQueueControl(origin, op, func(point uint64) error { calls++; at = point; value = op.Writes[0].SoundID; return nil }, scoreclock.Limits{})
				if err != nil || end != row.End || calls != 1 || value != row.Queue[0].Value || at != row.Queue[0].At {
					t.Fatal("source queue write/return differs", end, row.End, calls, at, err)
				}
			}
		})
	}
}

type controlFormalEvent struct {
	Kind           string
	ImmediateFM    bool `json:"immediate_fm"`
	Definition     *param.PanDefinition
	Enabled, White bool
	Clock          uint8
}
type controlFormal struct {
	Score  uint8
	Voice  int
	Cycle  bool
	Index  int
	Family string
	Event  controlFormalEvent
}

func controlFormals(t *testing.T) []controlFormal {
	t.Helper()
	var data struct {
		Scores []struct {
			ID     uint8
			Voices []struct {
				ID       int
				Channel  string
				Timeline struct{ Prefix, Cycle []controlFormalEvent }
			}
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../data/audio-score.json.gz"), &data); err != nil {
		t.Fatal(err)
	}
	var result []controlFormal
	seen := map[string]bool{}
	for _, s := range data.Scores {
		for _, v := range s.Voices {
			family := "fm"
			if v.Channel == "dac" {
				family = "dac"
			} else if len(v.Channel) >= 3 && v.Channel[:3] == "psg" {
				family = "psg"
			}
			for section, events := range [][]controlFormalEvent{v.Timeline.Prefix, v.Timeline.Cycle} {
				for index, e := range events {
					if e.Kind == "note" || e.Kind == "track-end" {
						continue
					}
					disabled := e.Definition != nil && e.Definition.Mode == param.PanDisabled
					key := fmt.Sprintf("%s/%s/%t/%t/%t", e.Kind, family, e.ImmediateFM, disabled, e.Enabled)
					if seen[key] {
						continue
					}
					seen[key] = true
					result = append(result, controlFormal{s.ID, v.ID, section == 1, index, family, e})
				}
			}
		}
	}
	return result
}
func controlFormalState(t *testing.T, c *score.Catalog, f controlFormal) (score.State, int) {
	t.Helper()
	d, err := score.New(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Advance(score.Input{SoundID: f.Score}); err != nil {
		t.Fatal(err)
	}
	s := d.Snapshot()
	slot := -1
	for i := range s.Voices {
		v := &s.Voices[i]
		v.Active = false
		if v.Owned && v.Owner == (score.Owner{ScoreID: f.Score, VoiceID: f.Voice}) {
			slot = i
		}
	}
	if slot < 0 {
		t.Fatal("formal control owner absent", f.Score, f.Voice)
	}
	s.TempoPeriod, s.TempoRemaining = 0, 0
	v := &s.Voices[slot]
	v.Active, v.Cycle, v.EventIndex, v.NoteWait = true, f.Cycle, f.Index, 1
	return s, slot
}

func TestControlClockActualObservedProjection(t *testing.T) {
	library := observedPCMLibrary(t)
	forms := controlFormals(t)
	if len(forms) != 25 {
		t.Fatal("formal live control shapes changed", len(forms))
	}
	for _, f := range forms {
		t.Run(fmt.Sprintf("%s-%s-%d", f.Event.Kind, f.Family, f.Score), func(t *testing.T) {
			before, slot := controlFormalState(t, library.Catalog(), f)
			d, err := score.Restore(library.Catalog(), before)
			if err != nil {
				t.Fatal(err)
			}
			stop := errors.New("stop after actual readonly control projection")
			calls := 0
			_, err = d.AdvanceObserved(score.ObservedInput{}, score.ObservedHooks{Operation: func(op score.Operation) error {
				if op.Kind != score.OpParameter || op.Routine != score.RoutineControl {
					return nil
				}
				calls++
				want := score.ControlProjection{ImmediateFM: f.Event.ImmediateFM, PanDisabled: f.Event.Definition != nil && f.Event.Definition.Mode == param.PanDisabled, Enabled: f.Event.Enabled}
				if f.Event.Kind == "psg-noise" {
					want.Noise = 224 | f.Event.Clock
					if f.Event.White {
						want.Noise |= 4
					}
				}
				if op.Control != want || op.VoiceSlot != int8(slot) || op.EventKind != f.Event.Kind {
					return errors.New("actual immutable control projection differs")
				}
				result, err := scoreclock.PriceObservedControl(scoreclock.Clock{}, op, scoreclock.Limits{})
				work := scoreclock.ControlParameterWork{Action: f.Event.Kind, Family: f.Family, ImmediateFM: want.ImmediateFM, PanDisabled: want.PanDisabled, Enabled: want.Enabled, Noise: want.Noise, Claimed: op.Flags.Claimed}
				plain, failure := scoreclock.PriceControlWork(scoreclock.Clock{}, work, scoreclock.Limits{})
				if err != nil || failure != nil || plain != result {
					return errors.New("actual control projection selected wrong clock path")
				}
				return stop
			}})
			if !errors.Is(err, stop) || calls != 1 || d.Snapshot() != before {
				t.Fatal("actual control projection mutated score or was not reached", calls, err)
			}
		})
	}
}

func TestControlClockActualRootQueueEffectIsSingleAndTimed(t *testing.T) {
	library := observedPCMLibrary(t)
	var form controlFormal
	for _, f := range controlFormals(t) {
		if f.Event.Kind == "enqueue-replace-first" {
			form = f
			break
		}
	}
	var native controlNative
	for _, r := range controlNatives(t) {
		if r.Input.Work.Action == form.Event.Kind && !r.Input.Work.Claimed {
			native = r
			break
		}
	}
	if len(native.Queue) != 1 {
		t.Fatal("original queue profile absent")
	}
	for _, scenario := range []string{"success", "omitted", "repeated", "error-after-write"} {
		t.Run(scenario, func(t *testing.T) {
			before, _ := controlFormalState(t, library.Catalog(), form)
			d, err := score.Restore(library.Catalog(), before)
			if err != nil {
				t.Fatal(err)
			}
			root := rootcontrol.InitialState()
			authorityQueue := [4]uint8{}
			queue := authorityQueue
			pause := uint8(0)
			calls, effects := 0, 0
			var late func() error
			var at uint64
			var end scoreclock.Clock
			out, err := rootcontrol.AdvanceRules(&root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}, rootcontrol.RuleInput{}, rootcontrol.RuleHooks{
				Operation: func(rootcontrol.RuleOperation) error { return nil }, Score: score.ObservedHooks{Operation: func(score.Operation) error { return nil }, ReadDAC: func(score.FeedbackRead) (score.DACFeedback, error) { return score.DACFeedback{}, nil }},
				QueueReplace: func(op score.Operation, apply func() error) error {
					calls++
					late = apply
					if queue[0] != 0 {
						return errors.New("queue already changed before actual timed store")
					}
					if scenario == "omitted" {
						return nil
					}
					var err error
					end, err = scoreclock.ServeQueueControl(scoreclock.Clock{Cycle: native.Input.Origin, Refresh: native.Input.Refresh}, op, func(point uint64) error { effects++; at = point; return apply() }, scoreclock.Limits{})
					if err != nil {
						return err
					}
					if scenario == "repeated" {
						_ = apply()
						return nil
					}
					if scenario == "error-after-write" {
						return errors.New("discard detached Root/score/queue branch")
					}
					return nil
				},
			})
			if calls != 1 {
				t.Fatal("queue callback not at actual source event", calls, err)
			}
			if scenario == "success" {
				if err != nil || effects != 1 || end != native.End || at != native.Queue[0].At || queue[0] != native.Queue[0].Value || len(out.Score.Intents) == 0 {
					t.Fatal("actual canonical queue effect/clock differs", effects, end, native.End, at, queue, err)
				}
			} else if err == nil || d.Snapshot() != before {
				t.Fatal("failed queue callback committed score", err)
			}
			saved := queue
			if err := late(); err == nil || queue != saved {
				t.Fatal("expired queue effect remained callable", err)
			}
			if authorityQueue != ([4]uint8{}) {
				t.Fatal("detached queue failure changed original owner")
			}
		})
	}
}

func TestControlClockValidationAndSharedBudget(t *testing.T) {
	for _, w := range []scoreclock.ControlParameterWork{{}, {Action: "track-end", Family: "fm"}, {Action: "detune", Family: "unknown"}, {Action: "detune", Family: "fm", ImmediateFM: true}, {Action: "tie", Family: "fm", PanDisabled: true}, {Action: "tempo", Family: "fm", Enabled: true}, {Action: "psg-noise", Family: "fm", Noise: 224}, {Action: "psg-noise", Family: "psg", Noise: 128}} {
		if r, err := scoreclock.PriceControlWork(scoreclock.Clock{}, w, scoreclock.Limits{}); !errors.Is(err, scoreclock.ErrControlWorkUnavailable) || r != (scoreclock.ControlResult{}) {
			t.Fatal("unpriced or contradictory control accepted", w, r, err)
		}
	}
	noise := scoreclock.ControlParameterWork{Action: "psg-noise", Family: "psg", Noise: 229}
	owner, cfg, _ := newPauseMixed(t)
	before := owner.r.Snapshot()
	branch, err := mixed.Restore(cfg, before)
	if err != nil {
		t.Fatal(err)
	}
	if end, err := scoreclock.ServeNoiseControl(scoreclock.Clock{}, noise, &controlPSG{r: branch}, scoreclock.Limits{MaxOperations: 5}); !errors.Is(err, scoreclock.ErrBudget) || end != (scoreclock.Clock{}) {
		t.Fatal("noise write/return reset prefix budget", end, err)
	}
	if !reflect.DeepEqual(before, owner.r.Snapshot()) {
		t.Fatal("failed detached noise mutated original owner")
	}
	for _, clock := range []scoreclock.Clock{{Cycle: math.MaxUint64}} {
		if r, err := scoreclock.PriceControlWork(clock, noise, scoreclock.Limits{}); err == nil || r != (scoreclock.ControlResult{}) {
			t.Fatal("overflow accepted")
		}
	}
	if _, err := scoreclock.ServeNoiseControl(scoreclock.Clock{}, noise, nil, scoreclock.Limits{}); err == nil {
		t.Fatal("missing PSG owner accepted")
	}
	if _, err := scoreclock.PriceObservedControl(scoreclock.Clock{}, score.Operation{Version: 1, Kind: score.OpChipWrite}, scoreclock.Limits{}); err == nil {
		t.Fatal("foreign observed operation accepted")
	}
	if _, err := scoreclock.ServeQueueControl(scoreclock.Clock{}, score.Operation{}, func(uint64) error { return nil }, scoreclock.Limits{}); err == nil {
		t.Fatal("foreign queue effect accepted")
	}
}
