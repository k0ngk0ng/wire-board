package scoreclock_test

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"math"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

//go:embed testdata/native-services.json.gz
var native []byte

type access struct {
	Kind     string `json:"kind"`
	At       uint64 `json:"at"`
	After    uint64 `json:"after"`
	Port     uint8  `json:"port"`
	Value    uint8  `json:"value"`
	Asserted bool   `json:"asserted"`
	Granted  bool   `json:"granted"`
}
type environment struct {
	chip           *chips.Interactive
	trace          []access
	ack            bool
	mailbox        uint8
	requestAdvance uint64
	fail           bool
}

func (e *environment) RequestBus(at uint64, asserted bool) (uint64, error) {
	if e.fail {
		return 0, errors.New("declared external bus failure")
	}
	after := at + e.requestAdvance
	e.trace = append(e.trace, access{Kind: "request", At: at, After: after, Asserted: asserted})
	return after, nil
}
func (e *environment) BusGranted(at uint64) (bool, error) {
	e.trace = append(e.trace, access{Kind: "ack", At: at, Granted: e.ack})
	return e.ack, nil
}
func (e *environment) Mailbox(at uint64) (uint8, error) {
	e.trace = append(e.trace, access{Kind: "mailbox", At: at, Value: e.mailbox})
	return e.mailbox, nil
}
func (e *environment) ReadStatus(at uint64, port uint8) (uint8, error) {
	value, err := e.chip.ReadStatus(at, port)
	e.trace = append(e.trace, access{Kind: "status", At: at, Port: port, Value: value})
	return value, err
}
func (e *environment) WritePort(at uint64, port, value uint8) error {
	err := e.chip.WritePort(at, port, value)
	e.trace = append(e.trace, access{Kind: "write", At: at, Port: port, Value: value})
	return err
}
func newEnvironment(t *testing.T) *environment {
	t.Helper()
	chip, err := chips.NewInteractive(chips.Config{Profile: chips.ReferenceMixedHQ44100}, chips.State{YM: chips.ResetYMState(), PSG: psg.ResetState()})
	if err != nil {
		t.Fatal(err)
	}
	return &environment{chip: chip, ack: true}
}

func TestNativeCanonicalYMServiceWithInteractiveChips(t *testing.T) {
	z, err := gzip.NewReader(bytes.NewReader(native))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	body, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Input struct {
				ID                    string `json:"id"`
				Bank, Register, Value uint8
				Origin                uint64 `json:"origin"`
				Refresh               uint64 `json:"refresh"`
				SeedBusy              bool   `json:"seed_busy"`
			} `json:"input"`
			Events []access                        `json:"events"`
			Cache  *scoreclock.LevelEffect         `json:"cache"`
			End    struct{ Cycle, Refresh uint64 } `json:"end"`
		}
	}
	if err = json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 48 {
		t.Fatal("native service matrix changed", len(fixture.Cases))
	}
	accesses, busyPolls := 0, 0
	for _, row := range fixture.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			e := newEnvironment(t)
			if row.Input.SeedBusy {
				if err = e.chip.WritePort(row.Input.Origin, 0, 0x2b); err != nil {
					t.Fatal(err)
				}
				if err = e.chip.WritePort(row.Input.Origin, 1, 0); err != nil {
					t.Fatal(err)
				}
			}
			got, err := scoreclock.ServeYM(scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}, scoreclock.Request{Bank: row.Input.Bank, Register: row.Input.Register, Value: row.Input.Value}, e, e, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if got.Clock.Cycle != row.End.Cycle || got.Clock.Refresh != row.End.Refresh {
				t.Fatalf("end clock %+v, native %+v", got.Clock, row.End)
			}
			if !reflect.DeepEqual(e.trace, row.Events) {
				t.Fatalf("ordered accesses differ\nGo %+v\nnative %+v", e.trace, row.Events)
			}
			if !reflect.DeepEqual(got.Level, row.Cache) {
				t.Fatalf("level cache semantic effect %+v, native %+v", got.Level, row.Cache)
			}
			for _, a := range e.trace {
				if a.Kind == "status" && a.Value&0x80 != 0 {
					busyPolls++
				}
			}
			accesses += len(e.trace)
		})
	}
	t.Logf("48 canonical YM services, %d accesses, %d busy polls", accesses, busyPolls)
}

func TestServiceRejectsInvalidBeforeExternalIO(t *testing.T) {
	for _, row := range []struct {
		clock   scoreclock.Clock
		request scoreclock.Request
		limits  scoreclock.Limits
	}{
		{scoreclock.Clock{}, scoreclock.Request{Bank: 2}, scoreclock.Limits{}},
		{scoreclock.Clock{Cycle: math.MaxUint64}, scoreclock.Request{}, scoreclock.Limits{}},
		{scoreclock.Clock{}, scoreclock.Request{}, scoreclock.Limits{MaxOperations: 65537}},
	} {
		e := newEnvironment(t)
		before := e.chip.Snapshot()
		got, err := scoreclock.ServeYM(row.clock, row.request, e, e, row.limits)
		if err == nil || len(e.trace) != 0 || !reflect.DeepEqual(before, e.chip.Snapshot()) || !reflect.DeepEqual(got, scoreclock.Result{}) {
			t.Fatal("invalid request touched external state", row, got, err, e.trace)
		}
	}
}

func TestUnreadyBusAndMailboxHaveFiniteBudget(t *testing.T) {
	for _, row := range []struct {
		ack     bool
		mailbox uint8
	}{{false, 0}, {true, 1}} {
		e := newEnvironment(t)
		e.ack = row.ack
		e.mailbox = row.mailbox
		_, err := scoreclock.ServeYM(scoreclock.Clock{}, scoreclock.Request{Register: 0x28}, e, e, scoreclock.Limits{MaxOperations: 64})
		if !errors.Is(err, scoreclock.ErrBudget) {
			t.Fatal("unready external owner did not stop at declared cost budget", err)
		}
		if len(e.trace) > 64 {
			t.Fatal("unbounded external operations", len(e.trace))
		}
		for _, a := range e.trace {
			if a.Kind == "write" {
				t.Fatal("unready bus/mailbox wrote chip", a)
			}
		}
	}
}

func TestExternalBusFailurePropagates(t *testing.T) {
	e := newEnvironment(t)
	e.fail = true
	_, err := scoreclock.ServeYM(scoreclock.Clock{}, scoreclock.Request{Register: 0x28}, e, e, scoreclock.Limits{})
	if err == nil {
		t.Fatal("external failure hidden")
	}
	if len(e.trace) != 0 {
		t.Fatal("failed bus wrote chip", e.trace)
	}
}
