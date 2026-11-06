package scoreclock_test

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sort"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type keyIO struct{ *instrumentIO }

func (e *keyIO) WritePSG(at uint64, value uint8) error {
	err := e.r.WritePSG(at, value)
	if err == nil {
		e.trace = append(e.trace, access{Kind: "psg", At: at, Value: value})
	}
	return err
}

func TestKeyGateOriginalBranchesAndActualOwners(t *testing.T) {
	var f struct {
		Version int
		Cases   []struct {
			Input struct {
				ID, Kind           string
				Channel, Remaining uint8
				On, Claimed, Tied  bool
				NoteOff            bool `json:"note_off"`
				Origin, Refresh    uint64
			}
			Events []instrumentNativeEvent
			PSG    []struct {
				At    uint64
				Value uint8
			}
			End   scoreclock.Clock
			Stack uint32 `json:"stack_pointer"`
			Equal bool   `json:"native_observer_state_equal"`
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-key-gate/native-key-gate-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 288 {
		t.Fatal("native key/gate scope incomplete")
	}
	for _, row := range f.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("observer changed native key/gate state")
			}
			owner, _, _ := newPauseMixed(t)
			if _, err := owner.r.RequestBus(0, true); err != nil {
				t.Fatal(err)
			}
			e := &keyIO{&instrumentIO{r: owner.r}}
			in := row.Input
			clock := scoreclock.Clock{Cycle: in.Origin, Refresh: in.Refresh}
			var end scoreclock.Clock
			var err error
			if in.Kind == "key" {
				tx, failure := scoreclock.BeginKeyTransfer(clock, scoreclock.KeyRequest{On: in.On, Channel: in.Channel, NoteOff: in.NoteOff, Claimed: in.Claimed, Tied: in.Tied}, e, e, scoreclock.Limits{})
				if failure != nil {
					t.Fatal(failure)
				}
				if len(e.trace) != 0 {
					t.Fatal("key guard emitted early effect")
				}
				if effect, ok := tx.ExpectedEffect(); ok {
					if err = tx.Apply(effect); err != nil {
						t.Fatal(err)
					}
				}
				end, err = tx.Finish()
				if _, failure = tx.Finish(); !errors.Is(failure, scoreclock.ErrKeySequence) {
					t.Fatal("key transfer reused")
				}
			} else {
				tx, failure := scoreclock.BeginGateTransfer(clock, scoreclock.GateRequest{Channel: in.Channel, Remaining: in.Remaining, Claimed: in.Claimed, Tied: in.Tied}, e, e, scoreclock.Limits{})
				if failure != nil {
					t.Fatal(failure)
				}
				if tx.Expired() != (in.Remaining == 1) || len(e.trace) != 0 {
					t.Fatal("gate outcome/early effect differs")
				}
				if effect, ok := tx.ExpectedEffect(); ok {
					if err = tx.Apply(effect); err != nil {
						t.Fatal(err)
					}
				}
				end, err = tx.Finish()
				wantSP := uint32(0xffff04)
				if tx.Expired() {
					wantSP += 4
				}
				if row.Stack != wantSP {
					t.Fatal("source return-skip semantics not proven", row.Stack, wantSP)
				}
				if _, failure = tx.Finish(); !errors.Is(failure, scoreclock.ErrGateSequence) {
					t.Fatal("gate transfer reused")
				}
			}
			if err != nil || end != row.End {
				t.Fatal("original key/gate clock differs", end, row.End, err)
			}
			want := instrumentAccesses(t, row.Events)
			for _, w := range row.PSG {
				want = append(want, access{Kind: "psg", At: w.At, Value: w.Value})
			}
			sort.SliceStable(want, func(i, j int) bool { return want[i].At < want[j].At })
			if !reflect.DeepEqual(e.trace, want) {
				t.Fatal("actual mixed key/gate IO differs", e.trace, want)
			}
		})
	}
}

func TestKeyGateValidationAndSharedBudget(t *testing.T) {
	owner, _, _ := newPauseMixed(t)
	e := &keyIO{&instrumentIO{r: owner.r}}
	for _, req := range []scoreclock.KeyRequest{{Channel: 3}, {On: true, Channel: 128}} {
		if tx, err := scoreclock.BeginKeyTransfer(scoreclock.Clock{}, req, e, e, scoreclock.Limits{}); err == nil || tx != nil {
			t.Fatal("unsupported key request accepted")
		}
	}
	if _, err := scoreclock.BeginKeyTransfer(scoreclock.Clock{Cycle: math.MaxUint64}, scoreclock.KeyRequest{}, e, e, scoreclock.Limits{}); err == nil {
		t.Fatal("key clock overflow accepted")
	}
	before := owner.r.Snapshot()
	tx, err := scoreclock.BeginKeyTransfer(scoreclock.Clock{}, scoreclock.KeyRequest{On: true}, e, e, scoreclock.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Finish(); !errors.Is(err, scoreclock.ErrKeySequence) {
		t.Fatal("early key finish accepted")
	}
	effect, _ := tx.ExpectedEffect()
	if err = tx.Apply(effect); !errors.Is(err, scoreclock.ErrKeySequence) {
		t.Fatal("poisoned key continued")
	}
	if !reflect.DeepEqual(before, owner.r.Snapshot()) {
		t.Fatal("rejected key emitted hardware effect")
	}
	gate, err := scoreclock.BeginGateTransfer(scoreclock.Clock{}, scoreclock.GateRequest{Remaining: 1}, e, e, scoreclock.Limits{MaxOperations: 20})
	if err != nil {
		t.Fatal(err)
	}
	effect, ok := gate.ExpectedEffect()
	if !ok {
		t.Fatal("expiring FM gate must request keyoff")
	}
	if err = gate.Apply(effect); !errors.Is(err, scoreclock.ErrBudget) {
		t.Fatal("nested key/YM reset gate budget", err)
	}
	if _, err = gate.Finish(); !errors.Is(err, scoreclock.ErrGateSequence) {
		t.Fatal("failed gate continued")
	}
}
