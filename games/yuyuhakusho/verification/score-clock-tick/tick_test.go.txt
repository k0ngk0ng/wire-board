package scoreclock_test

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

//go:embed testdata/tick-native-boundaries.json.gz
var tickNative []byte

type fieldAccess struct {
	Kind  string `json:"kind"`
	At    uint64 `json:"at"`
	Value uint8  `json:"value"`
}
type tickFields struct {
	pause    uint8
	priority uint8
	trace    []fieldAccess
	sequence []uint8
	failure  bool
}

func (f *tickFields) ReadPause(at uint64) (uint8, error) {
	if f.failure {
		return 0, errors.New("declared field failure")
	}
	value := f.pause
	if len(f.sequence) > 0 {
		value = f.sequence[0]
		f.sequence = f.sequence[1:]
	}
	f.trace = append(f.trace, fieldAccess{Kind: "pause-read", At: at, Value: value})
	return value, nil
}
func (f *tickFields) WritePause(at uint64, value uint8) error {
	f.pause = value
	f.trace = append(f.trace, fieldAccess{Kind: "pause-write", At: at, Value: value})
	return nil
}
func (f *tickFields) WritePriority(at uint64, value uint8) error {
	f.priority = value
	f.trace = append(f.trace, fieldAccess{Kind: "priority-write", At: at, Value: value})
	return nil
}

type tickFixture struct {
	Cases []struct {
		Input struct {
			Kind            string `json:"kind"`
			Pause           uint8  `json:"pause"`
			Origin, Refresh uint64
		} `json:"input"`
		Route  scoreclock.GateRoute `json:"route"`
		End    scoreclock.Clock     `json:"end"`
		Domain *uint64              `json:"domain_cleared_at"`
		Trace  []fieldAccess        `json:"trace"`
	} `json:"cases"`
	Natural []struct {
		Frame           int
		Origin, Refresh uint64
		Pause           uint8
		WriteAt         uint64 `json:"write_at"`
		Value           uint8
	} `json:"natural_pause_stores"`
}

func loadTick(t *testing.T) tickFixture {
	t.Helper()
	z, err := gzip.NewReader(bytes.NewReader(tickNative))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var f tickFixture
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) != 290 || len(f.Natural) != 2 {
		t.Fatal("native tick corpus coverage changed")
	}
	return f
}

func TestNativeFixedTickBoundaries(t *testing.T) {
	f := loadTick(t)
	reads, writes := 0, 0
	for i, row := range f.Cases {
		t.Run(fmt.Sprintf("%s-%03d", row.Input.Kind, i), func(t *testing.T) {
			owner := &tickFields{pause: row.Input.Pause, priority: 0x5a}
			clock := scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}
			var end scoreclock.Clock
			var err error
			switch row.Input.Kind {
			case "gate":
				var gate scoreclock.GateResult
				gate, err = scoreclock.EnterTick(clock, owner, scoreclock.Limits{})
				end = gate.Clock
				if err == nil && (gate.Route != row.Route || row.Domain == nil || gate.DomainClearedAt != *row.Domain) {
					t.Fatalf("gate route/domain differs %+v, native route=%d domain=%v", gate, row.Route, row.Domain)
				}
			case "return":
				end, err = scoreclock.FinishTick(clock, scoreclock.Limits{})
			case "priority":
				end, err = scoreclock.ClearPrimaryPriority(clock, owner, scoreclock.Limits{})
			default:
				t.Fatal(row.Input.Kind)
			}
			if err != nil {
				t.Fatal(err)
			}
			if end != row.End {
				t.Fatalf("end clock %+v native %+v", end, row.End)
			}
			if len(owner.trace) != len(row.Trace) {
				t.Fatalf("field count %+v native %+v", owner.trace, row.Trace)
			}
			for k, value := range owner.trace {
				if value != row.Trace[k] {
					t.Fatalf("field%d %+v native %+v", k, value, row.Trace[k])
				}
				if value.Kind == "pause-read" {
					reads++
				} else {
					writes++
				}
			}
		})
	}
	t.Logf("290 original fixed boundaries, %d immediate pause reads, %d writes", reads, writes)
}

func TestNaturalPauseStoreTimes(t *testing.T) {
	for _, row := range loadTick(t).Natural {
		owner := &tickFields{pause: row.Pause}
		gate, err := scoreclock.EnterTick(scoreclock.Clock{Cycle: row.Origin, Refresh: row.Refresh}, owner, scoreclock.Limits{})
		if err != nil {
			t.Fatal(err)
		}
		if gate.Route != scoreclock.TickPauseStart && gate.Route != scoreclock.TickResume {
			t.Fatal("natural transition routed elsewhere", gate)
		}
		writes := 0
		for _, e := range owner.trace {
			if e.Kind == "pause-write" {
				writes++
				if e.At != row.WriteAt || e.Value != row.Value {
					t.Fatal("natural pause field-store changed", row, e)
				}
			}
		}
		if writes != 1 {
			t.Fatal("natural transition missing field-store")
		}
	}
}

func TestTickPauseReadsAreNotAnEntrySnapshot(t *testing.T) {
	for _, row := range []struct {
		read    []uint8
		route   scoreclock.GateRoute
		cycle   uint64
		written bool
		value   uint8
	}{{[]uint8{1, 255}, scoreclock.TickPauseHold, 588, false, 0}, {[]uint8{1, 1, 128}, scoreclock.TickResume, 728, true, 0}} {
		owner := &tickFields{pause: 1, sequence: append([]uint8(nil), row.read...)}
		got, err := scoreclock.EnterTick(scoreclock.Clock{}, owner, scoreclock.Limits{})
		if err != nil || got.Route != row.route || got.Clock.Cycle != row.cycle {
			t.Fatal("did not query current field at each semantic read", got, err)
		}
		writes := 0
		for _, e := range owner.trace {
			if e.Kind == "pause-write" {
				writes++
				if e.Value != row.value {
					t.Fatal("wrong pause value")
				}
			}
		}
		if (writes != 0) != row.written {
			t.Fatal("wrong pause write count", owner.trace)
		}
	}
}

func TestTickInvalidBudgetAndFieldFailure(t *testing.T) {
	for _, row := range []struct {
		clock  scoreclock.Clock
		limits scoreclock.Limits
	}{{scoreclock.Clock{Cycle: math.MaxUint64}, scoreclock.Limits{}}, {scoreclock.Clock{}, scoreclock.Limits{MaxOperations: 65537}}} {
		owner := &tickFields{pause: 1}
		got, err := scoreclock.EnterTick(row.clock, owner, row.limits)
		if err == nil || !reflect.DeepEqual(got, scoreclock.GateResult{}) || len(owner.trace) != 0 || owner.pause != 1 {
			t.Fatal("invalid gate touched external fields", got, err)
		}
	}
	owner := &tickFields{pause: 1}
	if _, err := scoreclock.EnterTick(scoreclock.Clock{}, owner, scoreclock.Limits{MaxOperations: 1}); !errors.Is(err, scoreclock.ErrBudget) || len(owner.trace) != 0 {
		t.Fatal("gate operation budget ignored", err)
	}
	bad := &tickFields{pause: 1, failure: true}
	if got, err := scoreclock.EnterTick(scoreclock.Clock{}, bad, scoreclock.Limits{}); err == nil || !reflect.DeepEqual(got, scoreclock.GateResult{}) {
		t.Fatal("external field failure ignored")
	}
	if _, err := scoreclock.EnterTick(scoreclock.Clock{}, nil, scoreclock.Limits{}); err == nil {
		t.Fatal("nil owner accepted")
	}
	if _, err := scoreclock.ClearPrimaryPriority(scoreclock.Clock{}, nil, scoreclock.Limits{}); err == nil {
		t.Fatal("nil priority owner accepted")
	}
}
