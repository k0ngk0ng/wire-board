package scoreclock_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type trackProjection struct {
	Active          bool
	NoteOff         bool `json:"note_off"`
	Claimed, Tie    bool
	Channel         uint8
	NoteWait        uint8  `json:"note_wait"`
	GateRemaining   uint8  `json:"gate_remaining"`
	BasePitch       uint16 `json:"base_pitch"`
	LoadedAlgorithm uint8  `json:"loaded_algorithm"`
	Noise           uint8
	Pan             score.Pan
}
type trackNative struct {
	Input struct {
		ID              string
		Slot, Target    int
		Before, Entry   score.State
		Origin, Refresh uint64
	}
	Events []instrumentNativeEvent
	PSG    []struct {
		At    uint64
		Value uint8
	}
	Stores []struct {
		Kind  string
		At    uint64
		Value uint8
	}
	End             scoreclock.Clock
	Stop            trackProjection
	Target          *trackProjection
	Priority, Queue uint8
	Stack           uint32 `json:"stack_pointer"`
	Equal           bool   `json:"native_observer_state_equal"`
}

func trackNatives(t *testing.T) []trackNative {
	t.Helper()
	var f struct {
		Version int
		Cases   []trackNative
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-track-end/native-track-end-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 119 {
		t.Fatal("track-end original scope incomplete")
	}
	return f.Cases
}

func trackAssertProjection(t *testing.T, v score.VoiceState, n trackProjection) {
	t.Helper()
	if v.Active != n.Active || v.NoteOff != n.NoteOff || v.Claimed != n.Claimed || v.Tie != n.Tie || v.NoteWait != n.NoteWait || v.GateRemaining != n.GateRemaining || v.Channel != "dac" && v.BasePitch != n.BasePitch || v.Pan != n.Pan {
		t.Fatal("track-end voice projection differs", v, n)
	}
	if v.Channel == "fm1" || v.Channel == "fm2" || v.Channel == "fm3" || v.Channel == "fm4" || v.Channel == "fm5" {
		if v.LoadedAlgorithm != n.LoadedAlgorithm {
			t.Fatal("restored algorithm differs")
		}
	}
	if v.Channel == "psg-noise" && v.NoiseControl != n.Noise {
		t.Fatal("restored noise latch differs")
	}
}

func TestTrackEndCompleteOriginalBodyAndActualRootEffects(t *testing.T) {
	l := observedEnvelopeLibrary(t)
	c := l.Catalog()
	for _, row := range trackNatives(t) {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("track observer changed source state")
			}
			d, err := score.Restore(c, row.Input.Before)
			if err != nil {
				t.Fatal(err)
			}
			legacy, err := score.Restore(c, row.Input.Before)
			if err != nil {
				t.Fatal(err)
			}
			e := newNoteVoiceIO(t, l, 0, 0)
			root := rootcontrol.InitialState()
			root.Priority = 63
			queue := [4]uint8{}
			pause := uint8(0)
			var voice *scoreclock.VoiceTransfer
			var end scoreclock.Clock
			closed := false
			var stores []access
			hooks := rootcontrol.RuleHooks{Operation: func(rootcontrol.RuleOperation) error { return nil }, Score: score.ObservedHooks{Parser: l.Parser(), Operation: func(score.Operation) error { return nil }}, WorkingScore: func(op score.Operation, state score.State) error {
				if closed {
					return nil
				}
				if voice == nil {
					if op.Kind != score.OpEnter || op.Routine != score.RoutineVoice || op.VoiceSlot != int8(row.Input.Slot) {
						return nil
					}
					if state != row.Input.Entry {
						return errors.New("sibling state differs at actual voice entry")
					}
					var failure error
					voice, failure = scoreclock.BeginVoice(scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}, op, state, c, e, e, scoreclock.Limits{})
					return failure
				}
				if failure := voice.Consume(op, state); failure != nil {
					return failure
				}
				if op.Kind == score.OpLeave && op.Routine == score.RoutineVoice && op.VoiceSlot == int8(row.Input.Slot) {
					var failure error
					end, failure = voice.Finish()
					closed = true
					return failure
				}
				return nil
			}, QueueReplace: func(op score.Operation, apply func() error) error {
				if voice == nil {
					return errors.New("queue without active voice")
				}
				return voice.QueueReplace(op, func() error {
					stores = append(stores, access{Kind: "queue", At: voice.Clock().Cycle, Value: op.Writes[0].SoundID})
					return apply()
				})
			}, PriorityClear: func(op score.Operation, apply func() error) error {
				if voice == nil {
					return errors.New("priority without active voice")
				}
				return voice.PriorityClear(op, func() error {
					stores = append(stores, access{Kind: "priority", At: voice.Clock().Cycle})
					return apply()
				})
			}}
			out, err := rootcontrol.AdvanceRules(&root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}, rootcontrol.RuleInput{}, hooks)
			if err != nil || !closed || end != row.End {
				t.Fatal("actual Root track-end failed or clock differs", end, row.End, err)
			}
			lr := rootcontrol.InitialState()
			lr.Priority = 63
			lq := [4]uint8{}
			lp := uint8(0)
			want, failure := rootcontrol.AdvanceRules(&lr, rootcontrol.RuleOwners{Score: legacy, Queue: &lq, AudioPause: &lp}, rootcontrol.RuleInput{}, rootcontrol.RuleHooks{Operation: func(rootcontrol.RuleOperation) error { return nil }, Score: score.ObservedHooks{Parser: l.Parser(), Operation: func(score.Operation) error { return nil }}})
			if failure != nil || !reflect.DeepEqual(out, want) || root != lr || queue != lq || d.Snapshot() != legacy.Snapshot() {
				t.Fatal("timed track-end changed canonical owners", failure)
			}
			if root.Priority != row.Priority || queue[0] != row.Queue {
				t.Fatal("real Root effects differ from original", root.Priority, queue[0], row.Priority, row.Queue)
			}
			var expectedStores []access
			for _, store := range row.Stores {
				expectedStores = append(expectedStores, access{Kind: store.Kind, At: store.At, Value: store.Value})
			}
			if !reflect.DeepEqual(stores, expectedStores) {
				t.Fatal("Root store order/time differs", stores, expectedStores)
			}
			trace := frequencyPanAccesses(t, row.Events, row.PSG)
			for _, event := range row.Events {
				if event.Kind == 17 {
					trace = append(trace, access{Kind: "pan-request", At: event.After, Value: uint8(event.Value)})
				}
				if event.Kind == 18 {
					trace = append(trace, access{Kind: "restore-pan", At: event.After, Value: uint8(event.Value)})
				}
			}
			sort.SliceStable(trace, func(i, j int) bool { return trace[i].At < trace[j].At })
			if !reflect.DeepEqual(e.trace, trace) {
				for i := 0; i < len(e.trace) && i < len(trace); i++ {
					if e.trace[i] != trace[i] {
						t.Fatalf("restore access%d got%+v want%+v", i, e.trace[i], trace[i])
					}
				}
				t.Fatal("restore access count differs", len(e.trace), len(trace))
			}
			trackAssertProjection(t, d.Snapshot().Voices[row.Input.Slot], row.Stop)
			if row.Target != nil {
				trackAssertProjection(t, d.Snapshot().Voices[row.Input.Target], *row.Target)
			}
			if voice.EndsDriverTick() != (row.Input.Slot == 0) || row.Stack != uint32(0xffff04+map[bool]int{true: 4}[row.Input.Slot == 0]) {
				t.Fatal("DAC early-return outcome differs")
			}
			if _, err = voice.Finish(); err == nil {
				t.Fatal("terminated voice reused")
			}
		})
	}
}

func TestTrackEndRootPriorityAndQueueLeasesRejectMisuse(t *testing.T) {
	l := observedEnvelopeLibrary(t)
	c := l.Catalog()
	var row trackNative
	for _, candidate := range trackNatives(t) {
		if candidate.Input.Before.Voices[candidate.Input.Slot].Owner.ScoreID == 0xab && candidate.Input.Slot == 15 {
			row = candidate
			break
		}
	}
	if row.Input.ID == "" {
		t.Fatal("queue/priority witness missing")
	}
	for _, which := range []string{"queue", "priority"} {
		for _, mode := range []string{"missing", "twice", "expired"} {
			d, err := score.Restore(c, row.Input.Before)
			if err != nil {
				t.Fatal(err)
			}
			root := rootcontrol.InitialState()
			root.Priority = 63
			queue := [4]uint8{}
			pause := uint8(0)
			var escaped func() error
			h := rootcontrol.RuleHooks{Operation: func(rootcontrol.RuleOperation) error { return nil }, Score: score.ObservedHooks{Parser: l.Parser(), Operation: func(score.Operation) error { return nil }}}
			lease := func(op score.Operation, apply func() error) error {
				if mode == "missing" {
					return nil
				}
				if err := apply(); err != nil {
					return err
				}
				if mode == "twice" {
					return apply()
				}
				escaped = apply
				return nil
			}
			if which == "queue" {
				h.QueueReplace = lease
			} else {
				h.PriorityClear = lease
			}
			_, err = rootcontrol.AdvanceRules(&root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}, rootcontrol.RuleInput{}, h)
			if mode == "expired" {
				if err != nil || escaped == nil || escaped() == nil {
					t.Fatal("expired Root effect remained usable", which, err)
				}
			} else if err == nil || d.Snapshot() != row.Input.Before {
				t.Fatal("Root effect misuse accepted or score committed", which, mode, err)
			}
		}
	}
}
