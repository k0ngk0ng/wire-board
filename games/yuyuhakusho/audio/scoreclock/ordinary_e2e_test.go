package scoreclock_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type ordinaryIO struct {
	*instrumentIO
	root            *rootcontrol.State
	publications    []pcmcommands.Input
	failPublication bool
}

func (e *ordinaryIO) OrdinaryDescriptor(id string) (pcmcommands.Descriptor, bool) {
	return e.r.OrdinaryDescriptor(id)
}
func (e *ordinaryIO) ReadArbitration(at uint64, field pcmcommands.ArbitrationField) (uint8, error) {
	value, err := e.r.ReadArbitration(at, field)
	e.trace = append(e.trace, access{Kind: "arbitration", At: at, Port: uint8(field), Value: value})
	return value, err
}
func (e *ordinaryIO) ReadPCMChoice(at uint64) (uint8, error) {
	value, err := e.root.ReadPCMChoice(at)
	e.trace = append(e.trace, access{Kind: "choice-read", At: at, Value: value})
	return value, err
}
func (e *ordinaryIO) WritePCMChoice(at uint64, value uint8) error {
	err := e.root.WritePCMChoice(at, value)
	e.trace = append(e.trace, access{Kind: "choice-write", At: at, Value: value})
	return err
}
func (e *ordinaryIO) PublishAcceptedAt(at uint64, input pcmcommands.Input) error {
	if e.failPublication {
		return errors.New("actual ordinary publication failed")
	}
	err := e.r.PublishAcceptedAt(at, input)
	if err == nil {
		e.publications = append(e.publications, input)
		e.trace = append(e.trace, access{Kind: "publish", At: at, Port: uint8(input.Destination), Value: map[bool]uint8{true: 1, false: 0}[input.StartDual]})
	}
	return err
}

type ordinaryRow struct {
	Input struct {
		ID, Shape       string
		Code            uint8
		Descriptor      pcmcommands.Descriptor
		ModeFlags       uint8    `json:"mode_flags"`
		SlotFlags       [2]uint8 `json:"slot_flags"`
		SlotBusy        [2]uint8 `json:"slot_busy"`
		Choice          uint8
		Origin, Refresh uint64
	}
	Events []instrumentNativeEvent
	Steps  [][]uint64
	Root   []struct {
		Kind          string
		At            uint64
		Before, Value uint8
	}
	Fields map[string]uint8
	End    scoreclock.Clock
	Choice uint8
	Equal  bool `json:"native_observer_state_equal"`
}

func ordinaryRows(t *testing.T) []ordinaryRow {
	t.Helper()
	var f struct {
		Version int
		Shapes  []string
		Cases   []ordinaryRow
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-ordinary/native-ordinary-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 140 || len(f.Shapes) != 22 {
		t.Fatal("original ordinary resource/branch scope incomplete")
	}
	return f.Cases
}

func newOrdinaryIO(t *testing.T, row ordinaryRow) (*ordinaryIO, mixed.Config) {
	t.Helper()
	cfg := musicConfig(t)
	r, err := mixed.New(cfg, chips.State{YM: chips.ResetYMState(), PSG: psg.ResetState()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.RequestBus(0, true); err != nil {
		t.Fatal(err)
	}
	state := r.Snapshot()
	// The pinned quiet checkpoint preserves these two idle bytes; rejected
	// original calls prove both values unchanged by any publication.
	state.PCM.Global.ModeFlags = row.Input.ModeFlags
	state.PCM.Global.OrdinaryPan, state.PCM.Global.LatestFlags = 192, 1
	for i := range state.PCM.Voices {
		state.PCM.Voices[i].Busy = row.Input.SlotBusy[i]
		state.PCM.Voices[i].Flags = row.Input.SlotFlags[i]
	}
	r, err = mixed.Restore(cfg, state)
	if err != nil {
		t.Fatal(err)
	}
	root := rootcontrol.InitialState()
	root.ChoiceBits = row.Input.Choice
	return &ordinaryIO{instrumentIO: &instrumentIO{r: r}, root: &root}, cfg
}

func originalOrdinaryInput(row ordinaryRow) (pcmcommands.Input, bool) {
	input := pcmcommands.Input{Kind: pcmcommands.Ordinary, DescriptorID: row.Input.Descriptor.ID}
	accepted := false
	for _, s := range row.Steps {
		switch s[0] {
		case 0x37dd8:
			input.Destination = pcmcommands.Single
			accepted = true
		case 0x37f1e:
			input.Destination = pcmcommands.SlotA
			accepted = true
		case 0x37fce:
			input.Destination = pcmcommands.SlotB
			accepted = true
		case 0x37f48, 0x37ff8:
			input.StartDual = true
		}
	}
	return input, accepted
}

func TestOrdinaryPublicationOriginalResourcesAndActualOwners(t *testing.T) {
	library := observedPCMLibrary(t)
	codes, shapes := map[uint8]bool{}, map[string]bool{}
	for _, row := range ordinaryRows(t) {
		codes[row.Input.Code], shapes[row.Input.Shape] = true, true
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("native observer changed original state")
			}
			e, _ := newOrdinaryIO(t, row)
			d, ok := library.OrdinaryDescriptor(row.Input.Code)
			actual, found := e.r.OrdinaryDescriptor(d.ID)
			if !ok || !found || d != row.Input.Descriptor || actual != d {
				t.Fatal("real library/device metadata differs from source descriptor", d, row.Input.Descriptor)
			}
			tx, err := scoreclock.BeginOrdinaryPublication(scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}, row.Input.Code, library, e, e, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if len(e.publications) != 0 || e.root.ChoiceBits != row.Input.Choice {
				t.Fatal("arbitration prematurely published or changed choice")
			}
			expected, accepted := tx.ExpectedInput()
			wantInput, wantAccepted := originalOrdinaryInput(row)
			if accepted != wantAccepted || accepted && expected != wantInput {
				t.Fatal("canonical selection differs from original", expected, wantInput)
			}
			var stores []instrumentNativeEvent
			want := instrumentAccesses(t, row.Events)
			fields := map[uint32]pcmcommands.ArbitrationField{0x1ffc: pcmcommands.ArbitrationMode, 0x1f87: pcmcommands.ArbitrationAFlags, 0x1fa7: pcmcommands.ArbitrationBFlags, 0x1f86: pcmcommands.ArbitrationABusy, 0x1fa6: pcmcommands.ArbitrationBBusy}
			for _, event := range row.Events {
				if event.Kind == 25 {
					want = append(want, access{Kind: "arbitration", At: event.After, Port: uint8(fields[event.Address&0xffff]), Value: uint8(event.Value)})
				}
				if event.Kind == 26 {
					stores = append(stores, event)
				}
			}
			for _, event := range row.Root {
				want = append(want, access{Kind: event.Kind, At: event.At, Value: event.Value})
			}
			if accepted {
				if err = tx.Apply(expected); err != nil {
					t.Fatal(err)
				}
				wantInput.Begin, wantInput.End = stores[0].After, stores[len(stores)-1].After
				if !reflect.DeepEqual(e.publications, []pcmcommands.Input{wantInput}) {
					t.Fatal("actual atomic metadata boundaries differ", e.publications, wantInput)
				}
				want = append(want, access{Kind: "publish", At: wantInput.End, Port: uint8(wantInput.Destination), Value: map[bool]uint8{true: 1, false: 0}[wantInput.StartDual]})
			}
			end, err := tx.Finish()
			if err != nil || end != row.End {
				t.Fatal("complete original ordinary clock differs", end, row.End, err)
			}
			sort.SliceStable(want, func(i, j int) bool { return want[i].At < want[j].At })
			if !reflect.DeepEqual(e.trace, want) {
				t.Fatal("actual bus/field/choice/publication trace differs", e.trace, want)
			}
			state := e.r.Snapshot()
			g := state.PCM.Global
			got := map[string]uint8{"8188": g.ModeFlags, "8189": g.StartMarker, "8190": g.RateRequest, "8187": g.LatestFlags, "8186": g.OrdinaryPan, "8070": state.PCM.Voices[0].Busy, "8071": state.PCM.Voices[0].Flags, "8102": state.PCM.Voices[1].Busy, "8103": state.PCM.Voices[1].Flags}
			if !reflect.DeepEqual(got, row.Fields) || e.root.ChoiceBits != row.Choice || state.PCM.Held != nil {
				t.Fatal("canonical actual PCM/Root result differs", got, row.Fields, e.root.ChoiceBits, row.Choice)
			}
			if _, err = tx.Finish(); !errors.Is(err, scoreclock.ErrOrdinarySequence) {
				t.Fatal("finished ordinary phase reused")
			}
		})
	}
	if len(codes) != 122 || len(shapes) != 22 {
		t.Fatal("formal resource or branch coverage incomplete", len(codes), len(shapes))
	}
}

func TestOrdinaryPublicationRootDispatchUsesCanonicalChoice(t *testing.T) {
	library := observedPCMLibrary(t)
	for _, code := range []uint8{6, 11, 12, 0xf0, 0x73} {
		t.Run(fmt.Sprintf("code%02x", code), func(t *testing.T) {
			var row ordinaryRow
			for _, r := range ordinaryRows(t) {
				if r.Input.Code == code && r.Input.ModeFlags == 0 && r.Input.SlotBusy == [2]uint8{} && r.Input.SlotFlags == [2]uint8{} {
					row = r
					break
				}
			}
			if row.Input.Descriptor.ID == "" {
				t.Fatal("original resource closure profile absent")
			}
			e, _ := newOrdinaryIO(t, row)
			e.root.Selected = code
			d, err := score.New(library.Catalog())
			if err != nil {
				t.Fatal(err)
			}
			queue := [4]uint8{}
			pause := uint8(0)
			calls := 0
			var end scoreclock.Clock
			out, err := rootcontrol.AdvanceRules(e.root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}, rootcontrol.RuleInput{}, rootcontrol.RuleHooks{
				Operation: func(rootcontrol.RuleOperation) error { return nil },
				Score:     score.ObservedHooks{Operation: func(score.Operation) error { return nil }},
				AcceptDispatch: func(id uint8) error {
					if id != code {
						return errors.New("foreign private callable")
					}
					return nil
				},
				DispatchExternal: func(id uint8) error {
					calls++
					if id != code {
						return errors.New("wrong actual Root dispatch")
					}
					result, err := scoreclock.ServeOrdinaryPublication(scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}, id, library, e, e, scoreclock.Limits{})
					end = result.Clock
					return err
				},
			})
			if err != nil || calls != 1 || end != row.End || e.root.Selected != 255 || e.root.ChoiceBits != row.Choice || len(e.publications) != 1 || len(out.Score.Intents) != 0 {
				t.Fatal("actual Root external dispatch failed/replayed", calls, end, row.End, err)
			}
		})
	}
}

func TestOrdinaryPublicationPreservesSampledModeAndDetachedFailures(t *testing.T) {
	library := observedPCMLibrary(t)
	row := ordinaryRows(t)[0]
	for _, scenario := range []string{"sampled-mode", "wrong-input", "early-finish", "duplicate", "external-error", "budget"} {
		t.Run(scenario, func(t *testing.T) {
			original, cfg := newOrdinaryIO(t, row)
			before := original.r.Snapshot()
			rootBefore := *original.root
			r, err := mixed.Restore(cfg, before)
			if err != nil {
				t.Fatal(err)
			}
			rootCopy := rootBefore
			e := &ordinaryIO{instrumentIO: &instrumentIO{r: r}, root: &rootCopy}
			if scenario == "budget" {
				result, err := scoreclock.ServeOrdinaryPublication(scoreclock.Clock{}, row.Input.Code, library, e, e, scoreclock.Limits{MaxOperations: 32})
				if !errors.Is(err, scoreclock.ErrBudget) || result != (scoreclock.OrdinaryResult{}) {
					t.Fatal("nested ordinary work reset budget", result, err)
				}
			} else {
				tx, err := scoreclock.BeginOrdinaryPublication(scoreclock.Clock{}, row.Input.Code, library, e, e, scoreclock.Limits{})
				if err != nil {
					t.Fatal(err)
				}
				input, ok := tx.ExpectedInput()
				if !ok || !input.StartDual {
					t.Fatal("counterexample must sample legacy mode")
				}
				switch scenario {
				case "sampled-mode":
					current := e.r.Snapshot()
					if current.PCM.Held != nil {
						t.Fatal("arbitration failed to release first hold")
					}
					current.PCM.Global.ModeFlags = 1
					e.r, err = mixed.Restore(cfg, current)
					if err != nil {
						t.Fatal(err)
					}
					if err = tx.Apply(input); err != nil {
						t.Fatal(err)
					}
					if _, err = tx.Finish(); err != nil || len(e.publications) != 1 || !e.publications[0].StartDual || e.r.Snapshot().PCM.Global.RateRequest != 128 {
						t.Fatal("publication resampled later mode", err)
					}
				case "wrong-input":
					input.StartDual = false
				case "early-finish":
					if _, err = tx.Finish(); !errors.Is(err, scoreclock.ErrOrdinarySequence) {
						t.Fatal("early finish accepted")
					}
				case "duplicate":
					if err = tx.Apply(input); err != nil {
						t.Fatal(err)
					}
				case "external-error":
					e.failPublication = true
				}
				if scenario != "sampled-mode" {
					if err = tx.Apply(input); err == nil {
						t.Fatal("wrong/repeated/failed ordinary effect accepted")
					}
					if _, err = tx.Finish(); !errors.Is(err, scoreclock.ErrOrdinarySequence) {
						t.Fatal("failed ordinary phase continued")
					}
				}
			}
			if !reflect.DeepEqual(before, original.r.Snapshot()) || *original.root != rootBefore {
				t.Fatal("failed detached branch mutated original owners")
			}
		})
	}
}

func TestOrdinaryPublicationActualAPIValidation(t *testing.T) {
	library := observedPCMLibrary(t)
	row := ordinaryRows(t)[0]
	e, _ := newOrdinaryIO(t, row)
	before := e.r.Snapshot()
	for _, code := range []uint8{0, 1, 5, 0x7f, 0x80, 0xff} {
		if _, ok := library.OrdinaryDescriptor(code); ok {
			t.Fatal("invalid/diagnostic code exposed as ordinary", code)
		}
		if tx, err := scoreclock.BeginOrdinaryPublication(scoreclock.Clock{}, code, library, e, e, scoreclock.Limits{}); err == nil || tx != nil || len(e.trace) != 0 {
			t.Fatal("invalid code touched hardware", code)
		}
	}
	for _, field := range []pcmcommands.ArbitrationField{pcmcommands.ArbitrationField(99)} {
		if _, err := e.r.ReadArbitration(0, field); err == nil {
			t.Fatal("unknown arbitration field accepted")
		}
	}
	if _, err := e.r.ReadArbitration(pcmcommands.FrameCycles+1, pcmcommands.ArbitrationMode); err == nil {
		t.Fatal("out of frame arbitration read")
	}
	if !reflect.DeepEqual(before, e.r.Snapshot()) {
		t.Fatal("rejected arbitration access changed its owner")
	}
	if _, err := e.r.RequestBus(0, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.r.ReadArbitration(0, pcmcommands.ArbitrationMode); err == nil {
		t.Fatal("unheld arbitration read")
	}
	if _, err := scoreclock.BeginOrdinaryPublication(scoreclock.Clock{Cycle: math.MaxUint64}, row.Input.Code, library, e, e, scoreclock.Limits{}); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, err := scoreclock.BeginOrdinaryPublication(scoreclock.Clock{}, row.Input.Code, library, e, e, scoreclock.Limits{MaxOperations: 65537}); err == nil {
		t.Fatal("oversized budget accepted")
	}
	if _, err := scoreclock.BeginOrdinaryPublication(scoreclock.Clock{}, row.Input.Code, nil, e, e, scoreclock.Limits{}); err == nil {
		t.Fatal("missing resource library accepted")
	}
	if _, err := scoreclock.BeginOrdinaryPublication(scoreclock.Clock{}, row.Input.Code, library, nil, e, scoreclock.Limits{}); err == nil {
		t.Fatal("missing canonical choice owner accepted")
	}
	if _, err := scoreclock.BeginOrdinaryPublication(scoreclock.Clock{}, row.Input.Code, library, e, nil, scoreclock.Limits{}); err == nil {
		t.Fatal("missing actual bus accepted")
	}
	d, ok := e.r.OrdinaryDescriptor(row.Input.Descriptor.ID)
	d.Flags++
	if again, found := e.r.OrdinaryDescriptor(row.Input.Descriptor.ID); !ok || !found || again != row.Input.Descriptor {
		t.Fatal("metadata return aliases immutable owner")
	}
	if _, found := e.r.OrdinaryDescriptor("missing"); found {
		t.Fatal("unknown ordinary resource exposed")
	}
	root := rootcontrol.InitialState()
	root.ChoiceBits = 2
	if _, err := root.ReadPCMChoice(0); err == nil {
		t.Fatal("unsupported choice bits read")
	}
	if err := root.WritePCMChoice(0, 1); err == nil || root.ChoiceBits != 2 {
		t.Fatal("unsupported owner bits silently cleared")
	}
	root.ChoiceBits = 0
	if err := root.WritePCMChoice(0, 2); err == nil || root.ChoiceBits != 0 {
		t.Fatal("unsupported choice write accepted")
	}
}
