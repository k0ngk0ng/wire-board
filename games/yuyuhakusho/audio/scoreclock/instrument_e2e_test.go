package scoreclock_test

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

func instrumentEffective(reg, value uint8) uint8 {
	switch reg & 0xf0 {
	case 0x30:
		return value & 0x7f
	case 0x40:
		return value & 0x7f
	case 0x50:
		return value & 0xdf
	case 0x60:
		return value & 0x9f
	case 0x70:
		return value & 0x1f
	case 0xb0:
		if reg&15 < 4 {
			return value & 0x3f
		}
	}
	return value
}

type instrumentIO struct {
	r     *mixed.Runtime
	trace []access
}

func (e *instrumentIO) RequestBus(at uint64, on bool) (uint64, error) {
	after, err := e.r.RequestBus(at, on)
	e.trace = append(e.trace, access{Kind: "request", At: at, After: after, Asserted: on})
	return after, err
}
func (e *instrumentIO) BusGranted(at uint64) (bool, error) {
	v, err := e.r.BusGranted(at)
	e.trace = append(e.trace, access{Kind: "ack", At: at, Granted: v})
	return v, err
}
func (e *instrumentIO) Mailbox(at uint64) (uint8, error) {
	v, err := e.r.Mailbox(at)
	e.trace = append(e.trace, access{Kind: "mailbox", At: at, Value: v})
	return v, err
}
func (e *instrumentIO) ReadStatus(at uint64, p uint8) (uint8, error) {
	v, err := e.r.ReadStatus(at, p)
	e.trace = append(e.trace, access{Kind: "status", At: at, Port: p, Value: v})
	return v, err
}
func (e *instrumentIO) WritePort(at uint64, p, v uint8) error {
	err := e.r.WritePort(at, p, v)
	e.trace = append(e.trace, access{Kind: "write", At: at, Port: p, Value: v})
	return err
}

type instrumentNativeEvent struct {
	Kind, PC                     uint32
	At, After                    uint64
	Width, Address, Value, Owner uint32
}

func instrumentAccesses(t *testing.T, events []instrumentNativeEvent) []access {
	t.Helper()
	var out []access
	var request *instrumentNativeEvent
	var register uint8
	for _, e := range events {
		if e.Owner != 1 {
			continue
		}
		switch e.Kind {
		case 4:
			v := e
			request = &v
		case 5:
			if request == nil {
				t.Fatal("native request missing")
			}
			out = append(out, access{Kind: "request", At: request.At, After: e.After, Asserted: e.Value&map[bool]uint32{true: 256, false: 1}[e.Width == 2] != 0})
			request = nil
		case 6:
			out = append(out, access{Kind: "ack", At: e.At, Granted: e.Value&1 == 0})
		case 3:
			out = append(out, access{Kind: "mailbox", At: e.After, Value: uint8(e.Value)})
		case 1:
			out = append(out, access{Kind: "status", At: e.At, Port: uint8(e.Address), Value: uint8(e.Value)})
		case 2:
			value := uint8(e.Value)
			if e.Address%2 == 0 {
				register = value
			} else {
				value = instrumentEffective(register, value)
			}
			out = append(out, access{Kind: "write", At: e.At, Port: uint8(e.Address), Value: value})
		}
	}
	return out
}

func TestInstrumentClockOriginalCallerAndActualMixed(t *testing.T) {
	var f struct {
		Version int
		Cases   []struct {
			Input struct {
				ID, Mode, Domain      string
				Channel, Base, Loaded uint8
				Claimed               bool
				Work                  score.InstrumentWork
				Patch                 param.FMPatch
				Origin, Refresh       uint64
				LoadedAlgorithm       uint8 `json:"loaded_algorithm"`
			}
			Events   []instrumentNativeEvent
			Requests []struct {
				At                    uint64
				Refresh               uint64
				Bank, Register, Value uint8
			}
			End   scoreclock.Clock
			Equal bool `json:"native_recorder_state_equal"`
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-instrument/native-instrument-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 220 {
		t.Fatal("native instrument matrix incomplete")
	}
	for _, row := range f.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("native observer changed state")
			}
			initial, _, _ := newPauseMixed(t)
			if _, err := initial.r.RequestBus(0, true); err != nil {
				t.Fatal(err)
			}
			e := &instrumentIO{r: initial.r}
			input := row.Input
			work := scoreclock.InstrumentRequest{Mode: input.Mode, Domain: input.Domain, Channel: input.Channel, Claimed: input.Claimed, Work: input.Work, Patch: input.Patch, BaseLevel: input.Base, LoadedAlgorithm: input.LoadedAlgorithm}
			transfer, err := scoreclock.BeginInstrumentTransfer(scoreclock.Clock{Cycle: input.Origin, Refresh: input.Refresh}, work, e, e, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			expected := transfer.ExpectedWrites()
			if len(expected) != len(row.Requests) {
				t.Fatal("typed writes differ from original count")
			}
			for i, request := range expected {
				native := row.Requests[i]
				want := scoreclock.Request{Bank: native.Bank, Register: native.Register, Value: instrumentEffective(native.Register, native.Value)}
				if request != want {
					t.Fatal("typed original request differs", i, request, want)
				}
				if err = transfer.WriteNext(request); err != nil {
					t.Fatal(err)
				}
			}
			end, err := transfer.Finish()
			if err != nil || end != row.End {
				t.Fatal("caller final clock differs", end, row.End, err)
			}
			want := instrumentAccesses(t, row.Events)
			if !reflect.DeepEqual(e.trace, want) {
				n := 0
				for n < len(e.trace) && n < len(want) && e.trace[n] == want[n] {
					n++
				}
				t.Fatalf("actual mixed IO differs at%d Go%v Native%v", n, e.trace[max(0, n-1):min(len(e.trace), n+2)], want[max(0, n-1):min(len(want), n+2)])
			}
			if _, err = transfer.Finish(); !errors.Is(err, scoreclock.ErrInstrumentSequence) {
				t.Fatal("finished transfer reused")
			}
		})
	}
}

func TestInstrumentClockStagedAndCompleteEquivalent(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Input struct {
				Mode, Domain  string
				Channel, Base uint8
				Work          score.InstrumentWork
				Patch         param.FMPatch
				Loaded        uint8 `json:"loaded_algorithm"`
			}
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-instrument/native-instrument-full.json.gz"), &fixture); err != nil {
		t.Fatal(err)
	}
	input := fixture.Cases[0].Input
	left, cfg, _ := newPauseMixed(t)
	if _, err := left.r.RequestBus(0, true); err != nil {
		t.Fatal(err)
	}
	before := left.r.Snapshot()
	copy, err := mixed.Restore(cfg, before)
	if err != nil {
		t.Fatal(err)
	}
	a := &instrumentIO{r: left.r}
	b := &instrumentIO{r: copy}
	request := scoreclock.InstrumentRequest{Mode: input.Mode, Domain: input.Domain, Channel: input.Channel, Work: input.Work, Patch: input.Patch, BaseLevel: input.Base, LoadedAlgorithm: input.Loaded}
	tx, err := scoreclock.BeginInstrumentTransfer(scoreclock.Clock{}, request, a, a, scoreclock.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	writes := tx.ExpectedWrites()
	clone := tx.ExpectedWrites()
	clone[0].Value++
	for _, w := range writes {
		if err = tx.WriteNext(w); err != nil {
			t.Fatal(err)
		}
	}
	end, err := tx.Finish()
	if err != nil {
		t.Fatal(err)
	}
	complete, err := scoreclock.ServeInstrument(scoreclock.Clock{}, request, b, b, scoreclock.Limits{})
	if err != nil || complete != end || !reflect.DeepEqual(a.trace, b.trace) || !reflect.DeepEqual(a.r.Snapshot(), b.r.Snapshot()) {
		t.Fatal("staged and complete operations diverge", err)
	}
	failed, err := mixed.Restore(cfg, before)
	if err != nil {
		t.Fatal(err)
	}
	e := &instrumentIO{r: failed}
	bad, err := scoreclock.BeginInstrumentTransfer(scoreclock.Clock{}, request, e, e, scoreclock.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = bad.Finish(); !errors.Is(err, scoreclock.ErrInstrumentSequence) {
		t.Fatal("premature Finish accepted")
	}
	if !reflect.DeepEqual(before, failed.Snapshot()) {
		t.Fatal("premature Finish emitted hardware writes")
	}
}

func TestInstrumentClockValidationBudgetAndPoisoning(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Input struct {
				Mode, Domain  string
				Channel, Base uint8
				Work          score.InstrumentWork
				Patch         param.FMPatch
				Loaded        uint8 `json:"loaded_algorithm"`
			}
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-instrument/native-instrument-full.json.gz"), &fixture); err != nil {
		t.Fatal(err)
	}
	input := fixture.Cases[0].Input
	request := scoreclock.InstrumentRequest{Mode: input.Mode, Domain: input.Domain, Channel: input.Channel, Work: input.Work, Patch: input.Patch, BaseLevel: input.Base, LoadedAlgorithm: input.Loaded}
	for _, change := range []func(*scoreclock.InstrumentRequest){func(w *scoreclock.InstrumentRequest) { w.Mode = "unknown" }, func(w *scoreclock.InstrumentRequest) { w.Domain = "unknown" }, func(w *scoreclock.InstrumentRequest) { w.Channel = 3 }, func(w *scoreclock.InstrumentRequest) { w.Work.PatchID = "other" }, func(w *scoreclock.InstrumentRequest) { w.LoadedAlgorithm = 8 }} {
		owner, _, _ := newPauseMixed(t)
		e := &instrumentIO{r: owner.r}
		bad := request
		change(&bad)
		got, err := scoreclock.BeginInstrumentTransfer(scoreclock.Clock{}, bad, e, e, scoreclock.Limits{})
		if err == nil || got != nil || len(e.trace) != 0 {
			t.Fatal("invalid transfer touched hardware")
		}
	}
	owner, cfg, _ := newPauseMixed(t)
	before := owner.r.Snapshot()
	branch, err := mixed.Restore(cfg, before)
	if err != nil {
		t.Fatal(err)
	}
	e := &instrumentIO{r: branch}
	if _, err = scoreclock.ServeInstrument(scoreclock.Clock{}, request, e, e, scoreclock.Limits{MaxOperations: 96}); !errors.Is(err, scoreclock.ErrBudget) {
		t.Fatal("nested operations reset budget", err)
	}
	if !reflect.DeepEqual(before, owner.r.Snapshot()) {
		t.Fatal("failed branch mutated original")
	}
	e = &instrumentIO{r: owner.r}
	tx, err := scoreclock.BeginInstrumentTransfer(scoreclock.Clock{}, request, e, e, scoreclock.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	wrong := tx.ExpectedWrites()[0]
	wrong.Value++
	if err = tx.WriteNext(wrong); !errors.Is(err, scoreclock.ErrInstrumentSequence) {
		t.Fatal("wrong next intent accepted")
	}
	if _, err = tx.Finish(); !errors.Is(err, scoreclock.ErrInstrumentSequence) {
		t.Fatal("poisoned transfer continued")
	}
	if _, err = scoreclock.BeginInstrumentTransfer(scoreclock.Clock{Cycle: math.MaxUint64}, request, e, e, scoreclock.Limits{}); err == nil {
		t.Fatal("overflow accepted")
	}
}
