package scoreclock_test

import (
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"reflect"
	"sort"
	"testing"
)

type requestField struct {
	Kind    string
	At      uint64
	Address uint32
	Value   uint8
}
type requestNative struct {
	Input struct {
		ID, Kind        string
		Code            uint8
		Origin, Refresh uint64
		Before          struct {
			Priority, Selected uint8
			Queue              [4]uint8
		}
	}
	Fields []requestField
	Events []instrumentNativeEvent
	End    scoreclock.Clock
	After  struct {
		Priority, Selected uint8
		Queue              [4]uint8
		Choice             uint8
	}
	Equal bool `json:"native_observer_state_equal"`
}

func requestNatives(t *testing.T) ([]requestNative, engine.SoundPriorityTable) {
	t.Helper()
	var f struct {
		Version    int
		Priorities engine.SoundPriorityTable
		Cases      []requestNative
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-request-queue/native-request-queue-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 409 {
		t.Fatal("request source scope missing")
	}
	return f.Cases, f.Priorities
}
func TestRequestQueueActualFieldTimesAndCanonicalOwners(t *testing.T) {
	rows, table := requestNatives(t)
	for _, row := range rows {
		if row.Input.Kind != "queue" {
			continue
		}
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("queue observer changed source")
			}
			root := rootcontrol.InitialState()
			root.Priority, root.Selected = row.Input.Before.Priority, row.Input.Before.Selected
			queue := row.Input.Before.Queue
			result, err := scoreclock.ConsumeQueue(scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}, &root, &queue, table, scoreclock.Limits{})
			if err != nil || result.Clock != row.End || root.Priority != row.After.Priority || root.Selected != row.After.Selected || queue != row.After.Queue {
				t.Fatal("canonical timed queue differs", result.Clock, row.End, err)
			}
			if len(result.Fields) != len(row.Fields) {
				t.Fatal("canonical field count differs", len(result.Fields), len(row.Fields))
			}
			for i, field := range result.Fields {
				want := row.Fields[i]
				slot := int8(-1)
				if want.Address >= 0xf80a && want.Address <= 0xf80d {
					slot = int8(want.Address - 0xf80a)
				}
				if field.Kind != want.Kind || field.At != want.At || field.Value != want.Value || field.Slot != slot {
					t.Fatal("actual queue field time/order differs", i, field, want)
				}
			}
		})
	}
}
func TestRequestQueueOrdinaryWholeDriverDispatch(t *testing.T) {
	rows, table := requestNatives(t)
	l := observedEnvelopeLibrary(t)
	for _, row := range rows {
		if row.Input.Kind != "ordinary" {
			continue
		}
		t.Run(row.Input.ID, func(t *testing.T) {
			d, err := score.New(l.Catalog())
			if err != nil {
				t.Fatal(err)
			}
			root := rootcontrol.InitialState()
			root.Priority, root.Selected = row.Input.Before.Priority, row.Input.Before.Selected
			queue := row.Input.Before.Queue
			pause := uint8(0)
			ordinaryRow := ordinaryRow{}
			ordinaryRow.Input.Origin = row.Input.Origin
			ordinaryRow.Input.Refresh = row.Input.Refresh
			e, _ := newOrdinaryIO(t, ordinaryRow)
			e.root = &root
			result, err := scoreclock.AdvanceRequestTick(scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}, &root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}, rootcontrol.RuleInput{Priorities: table}, l, e, e, scoreclock.Limits{})
			if err != nil || result.Clock != row.End || root.Priority != row.After.Priority || root.Selected != row.After.Selected || queue != row.After.Queue || root.ChoiceBits != row.After.Choice {
				t.Fatal("complete ordinary request dispatch differs", result.Clock, row.End, err)
			}
			want := instrumentAccesses(t, row.Events)
			var stores []instrumentNativeEvent
			for _, event := range row.Events {
				if event.Kind == 25 {
					field := uint8(0)
					switch event.Address & 65535 {
					case 0x1ffc:
						field = 0
					case 0x1f87:
						field = 1
					case 0x1fa7:
						field = 2
					case 0x1f86:
						field = 3
					case 0x1fa6:
						field = 4
					}
					want = append(want, access{Kind: "arbitration", At: event.After, Port: field, Value: uint8(event.Value)})
				}
				if event.Kind == 26 {
					stores = append(stores, event)
				}
			}
			for _, field := range row.Fields {
				if field.Kind == "choice-read" || field.Kind == "choice-write" {
					want = append(want, access{Kind: field.Kind, At: field.At, Value: field.Value})
				}
			}
			if len(e.publications) > 1 {
				t.Fatal("ordinary publication repeated")
			}
			if len(e.publications) == 1 {
				p := e.publications[0]
				if len(stores) == 0 || p.Begin != stores[0].After || p.End != stores[len(stores)-1].After {
					t.Fatal("ordinary publication interval differs")
				}
				want = append(want, access{Kind: "publish", At: p.End, Port: uint8(p.Destination), Value: map[bool]uint8{true: 1}[p.StartDual]})
			} else if len(stores) != 0 {
				t.Fatal("source publication missing")
			}
			sort.SliceStable(want, func(i, j int) bool { return want[i].At < want[j].At })
			if !reflect.DeepEqual(e.trace, want) {
				for i := 0; i < len(e.trace) && i < len(want); i++ {
					if e.trace[i] != want[i] {
						t.Fatalf("request access%d got%+v want%+v", i, e.trace[i], want[i])
					}
				}
				t.Fatal("request access count differs", len(e.trace), len(want))
			}
		})
	}
}
func TestRequestQueueWholeBudgetAndUnpricedDispatch(t *testing.T) {
	l := observedEnvelopeLibrary(t)
	_, table := requestNatives(t)
	for _, code := range []uint8{130, 162, 1, 4, 5} {
		d, _ := score.New(l.Catalog())
		root := rootcontrol.InitialState()
		queue := [4]uint8{code}
		pause := uint8(0)
		row := ordinaryRow{}
		e, _ := newOrdinaryIO(t, row)
		e.root = &root
		before := d.Snapshot()
		_, err := scoreclock.AdvanceRequestTick(scoreclock.Clock{}, &root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}, rootcontrol.RuleInput{Priorities: table}, l, e, e, scoreclock.Limits{})
		if err == nil || d.Snapshot() != before {
			t.Fatal("unpriced dispatch accepted/committed", code, err)
		}
	}
	d, _ := score.New(l.Catalog())
	root := rootcontrol.InitialState()
	queue := [4]uint8{6}
	pause := uint8(0)
	row := ordinaryRow{}
	e, _ := newOrdinaryIO(t, row)
	e.root = &root
	before := d.Snapshot()
	_, err := scoreclock.AdvanceRequestTick(scoreclock.Clock{}, &root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}, rootcontrol.RuleInput{Priorities: table}, l, e, e, scoreclock.Limits{MaxOperations: 100})
	if !errors.Is(err, scoreclock.ErrBudget) || d.Snapshot() != before {
		t.Fatal("request/queue/dispatch reset whole budget", err)
	}
}
