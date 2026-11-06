package scoreclock_test

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/session"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type soundIRQNative struct {
	Input struct {
		ID, Kind        string
		Before          score.State
		Root            rootcontrol.State
		Scheduler       engine.SoundSchedulerState
		Queue           [4]uint8
		Pause           uint8
		Levels          [6][4]uint8
		Origin, Refresh uint64
	}
	Fields []struct {
		Kind  string
		At    uint64
		Value uint8
	}
	SchedulerFields []struct {
		Kind  string
		At    uint64
		Value uint16
	} `json:"scheduler_fields"`
	RequestFields []struct {
		Kind  string
		At    uint64
		Slot  int8
		Value uint8
	} `json:"request_fields"`
	ChoiceFields []struct {
		Kind  string
		At    uint64
		Value uint8
	} `json:"choice_fields"`
	Entries []scoreclock.Clock
	Events  []instrumentNativeEvent
	PSG     []struct {
		At    uint64
		Value uint8
	}
	End            scoreclock.Clock
	SchedulerAfter engine.SoundSchedulerState `json:"scheduler_after"`
	AudioRAM       string                     `json:"audio_ram"`
	Equal          bool                       `json:"native_observer_state_equal"`
}

func soundIRQNatives(t *testing.T) ([]soundIRQNative, engine.SoundPriorityTable) {
	t.Helper()
	var f struct {
		Version    int
		Priorities engine.SoundPriorityTable
		Cases      []soundIRQNative
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/sound-irq-runtime/native-sound-irq-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 122 {
		t.Fatal("whole original sound IRQ scope missing")
	}
	return f.Cases, f.Priorities
}

type soundIRQIO struct {
	*driverControlIO
	root         *rootcontrol.State
	publications []pcmcommands.Input
}

func (e *soundIRQIO) OrdinaryDescriptor(id string) (pcmcommands.Descriptor, bool) {
	return e.r.OrdinaryDescriptor(id)
}
func (e *soundIRQIO) ReadArbitration(at uint64, field pcmcommands.ArbitrationField) (uint8, error) {
	v, err := e.r.ReadArbitration(at, field)
	e.trace = append(e.trace, access{Kind: "arbitration", At: at, Port: uint8(field), Value: v})
	return v, err
}
func (e *soundIRQIO) ReadPCMChoice(at uint64) (uint8, error) {
	v, err := e.root.ReadPCMChoice(at)
	e.trace = append(e.trace, access{Kind: "choice-read", At: at, Value: v})
	return v, err
}
func (e *soundIRQIO) WritePCMChoice(at uint64, v uint8) error {
	err := e.root.WritePCMChoice(at, v)
	e.trace = append(e.trace, access{Kind: "choice-write", At: at, Value: v})
	return err
}
func (e *soundIRQIO) PublishAcceptedAt(at uint64, in pcmcommands.Input) error {
	if in.Kind != pcmcommands.Ordinary {
		return e.musicIO.PublishAcceptedAt(at, in)
	}
	err := e.r.PublishAcceptedAt(at, in)
	if err == nil {
		e.publications = append(e.publications, in)
		e.trace = append(e.trace, access{Kind: "publish", At: at, Port: uint8(in.Destination), Value: map[bool]uint8{true: 1}[in.StartDual]})
	}
	return err
}

func TestSoundIRQWholeOriginalAndJointState(t *testing.T) {
	l := observedEnvelopeLibrary(t)
	rows, priorities := soundIRQNatives(t)
	for _, row := range rows {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("IRQ observer changed original state")
			}
			d, err := score.Restore(l.Catalog(), row.Input.Before)
			if err != nil {
				t.Fatal(err)
			}
			root, _ := rootcontrol.RestoreState(row.Input.Root)
			scheduler := row.Input.Scheduler
			queue, pause := row.Input.Queue, row.Input.Pause
			e := &soundIRQIO{driverControlIO: newDriverControlIO(t, l, row.Input.Levels), root: &root}
			initialMixed := e.r.Snapshot()
			clock := scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}
			owners := rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}
			input := rootcontrol.RuleInput{Priorities: priorities}
			var result scoreclock.SoundIRQResult
			if row.Input.Kind == "interrupt" {
				result, err = scoreclock.AdvanceSoundInterrupt(clock, &scheduler, &root, owners, input, l, e, e, scoreclock.Limits{})
			} else {
				result, err = scoreclock.AdvanceSoundCatchup(clock, &scheduler, &root, owners, input, l, e, e, scoreclock.Limits{})
			}
			if err != nil || result.Clock != row.End || scheduler != row.SchedulerAfter || !reflect.DeepEqual(result.Entries, row.Entries) {
				t.Fatal("whole sound IRQ clock/state/entry differs", result.Clock, row.End, scheduler, row.SchedulerAfter, err)
			}
			want := noteAccesses(t, noteNative{Events: row.Events, PSG: row.PSG})
			var stores []instrumentNativeEvent
			for _, ev := range row.Events {
				if ev.Kind == 25 {
					field := map[uint32]uint8{0x1ffc: 0, 0x1f87: 1, 0x1fa7: 2, 0x1f86: 3, 0x1fa6: 4}[ev.Address&65535]
					want = append(want, access{Kind: "arbitration", At: ev.After, Port: field, Value: uint8(ev.Value)})
				}
				if ev.Kind == 26 {
					stores = append(stores, ev)
				}
			}
			for _, field := range row.ChoiceFields {
				want = append(want, access{Kind: field.Kind, At: field.At, Value: field.Value})
			}
			if len(e.publications) > 1 {
				t.Fatal("ordinary publication repeated in IRQ")
			}
			if len(e.publications) == 1 {
				p := e.publications[0]
				if len(stores) == 0 || p.Begin != stores[0].After || p.End != stores[len(stores)-1].After {
					t.Fatal("IRQ ordinary publication interval differs")
				}
				want = append(want, access{Kind: "publish", At: p.End, Port: uint8(p.Destination), Value: map[bool]uint8{true: 1}[p.StartDual]})
			} else if len(stores) != 0 {
				t.Fatal("IRQ lost source ordinary publication")
			}
			sort.SliceStable(want, func(i, j int) bool { return want[i].At < want[j].At })
			if !reflect.DeepEqual(e.trace, want) {
				for i := 0; i < len(e.trace) && i < len(want); i++ {
					if e.trace[i] != want[i] {
						t.Fatalf("IRQ access%d got%+v want%+v", i, e.trace[i], want[i])
					}
				}
				t.Fatal("IRQ access count differs", len(e.trace), len(want))
			}
			if len(result.SchedulerFields) != len(row.SchedulerFields) || len(result.RequestFields) != len(row.RequestFields) {
				t.Fatal("IRQ owner field count differs")
			}
			for i, field := range result.SchedulerFields {
				v := row.SchedulerFields[i]
				if field.Kind != v.Kind || field.At != v.At || field.Value != v.Value {
					t.Fatal("scheduler field time/order differs", i, field, v)
				}
			}
			for i, field := range result.RequestFields {
				v := row.RequestFields[i]
				if field.Kind != v.Kind || field.At != v.At || field.Slot != v.Slot || field.Value != v.Value {
					t.Fatal("request live field time/order differs", i, field, v)
				}
			}
			var fields []scoreclock.DriverField
			for _, driver := range result.Drivers {
				fields = append(fields, driver.Fields...)
			}
			if len(fields) != len(row.Fields) {
				t.Fatal("IRQ driver field count differs", len(fields), len(row.Fields))
			}
			for i, field := range fields {
				v := row.Fields[i]
				if field.Kind != v.Kind || field.At != v.At || field.Value != v.Value {
					t.Fatal("IRQ driver field time/order differs", i, field, v)
				}
			}
			ram, _ := hex.DecodeString(row.AudioRAM)
			if len(ram) != 928 || root.Priority != ram[0] || root.Selected != ram[9] || queue != ([4]uint8{ram[10], ram[11], ram[12], ram[13]}) || pause != ram[7] || root.Fade.Steps != ram[4] || root.Fade.Remaining != ram[6] {
				t.Fatal("IRQ native Root final fields differ")
			}
			// Actual full mixed owner, restored through JSON, must reach the same
			// canonical result without adding World queue/pause to the audio state.
			state := session.State{Version: 1, ResourceID: l.Identity(), Root: row.Input.Root, Score: row.Input.Before, Scheduler: row.Input.Scheduler, Mixed: initialMixed}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			var restored session.State
			if err = json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			cfg := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}, PCMOrigin: 0}
			runtime, err := session.Restore(l, cfg, restored)
			if err != nil {
				t.Fatal(err)
			}
			q, p := row.Input.Queue, row.Input.Pause
			var actual scoreclock.SoundIRQResult
			if row.Input.Kind == "interrupt" {
				actual, err = runtime.Interrupt(clock, &q, &p, priorities, scoreclock.Limits{})
			} else {
				actual, err = runtime.Catchup(clock, &q, &p, priorities, scoreclock.Limits{})
			}
			after := runtime.Snapshot()
			if err != nil || actual.Clock != result.Clock || !reflect.DeepEqual(after.Root, root) || after.Score != d.Snapshot() || after.Scheduler != scheduler || !reflect.DeepEqual(after.Mixed, e.r.Snapshot()) || q != queue || p != pause {
				t.Fatal("joint runtime differs/duplicated owners", err)
			}
		})
	}
}

func TestSoundIRQJointFailureAndResourceBinding(t *testing.T) {
	l := observedEnvelopeLibrary(t)
	rows, priorities := soundIRQNatives(t)
	var row soundIRQNative
	for _, v := range rows {
		if strings.HasSuffix(v.Input.ID, "-catchup-3") {
			row = v
			break
		}
	}
	if row.Input.ID == "" {
		t.Fatal("outer budget witness missing")
	}
	e := newDriverControlIO(t, l, row.Input.Levels)
	cfg := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	state := session.State{Version: 1, ResourceID: l.Identity(), Root: row.Input.Root, Score: row.Input.Before, Scheduler: row.Input.Scheduler, Mixed: e.r.Snapshot()}
	for _, mode := range []string{"identity", "root", "score", "mixed"} {
		bad := state
		switch mode {
		case "identity":
			bad.ResourceID = "unknown"
		case "root":
			bad.Root.ChoiceBits = 2
		case "score":
			bad.Score.Voices[0].Active = true
		case "mixed":
			bad.Mixed.Chips.Time++
		}
		if _, err := session.Restore(l, cfg, bad); err == nil {
			t.Fatal("joint restore accepted mismatched owner", mode)
		}
	}
	runtime, err := session.Restore(l, cfg, state)
	if err != nil {
		t.Fatal(err)
	}
	before := runtime.Snapshot()
	queue, pause := row.Input.Queue, row.Input.Pause
	oldQ, oldP := queue, pause
	_, err = runtime.Catchup(scoreclock.Clock{}, &queue, &pause, priorities, scoreclock.Limits{MaxOperations: 300})
	if !errors.Is(err, scoreclock.ErrBudget) || !reflect.DeepEqual(runtime.Snapshot(), before) || queue != oldQ || pause != oldP {
		t.Fatal("outer budget reset/partial joint commit", err)
	}
	if _, err = runtime.Interrupt(scoreclock.Clock{}, &queue, &pause, priorities, scoreclock.Limits{}); err != nil {
		t.Fatal("failed joint call retained a lease", err)
	}
}
