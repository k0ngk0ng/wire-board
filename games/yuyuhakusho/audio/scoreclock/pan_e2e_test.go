package scoreclock_test

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sort"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type panIO struct {
	*instrumentIO
	failWrite bool
	denyGrant bool
}

func (e *panIO) PanRequest(at uint64) (uint8, error) {
	v, err := e.r.PanRequest(at)
	e.trace = append(e.trace, access{Kind: "pan-request", At: at, Value: v})
	return v, err
}
func (e *panIO) WriteRestorePan(at uint64, value uint8) error {
	if e.failWrite {
		return errors.New("detached pan write failed")
	}
	err := e.r.WriteRestorePan(at, value)
	e.trace = append(e.trace, access{Kind: "restore-pan", At: at, Value: value})
	return err
}
func (e *panIO) BusGranted(at uint64) (bool, error) {
	if e.denyGrant {
		return false, nil
	}
	return e.instrumentIO.BusGranted(at)
}

func panValue(bits uint8) score.Pan {
	return score.Pan{Left: bits&128 != 0, Right: bits&64 != 0, AMS: bits >> 4 & 3, FMS: bits & 7}
}

func newPanMixed(t *testing.T, marker, busy uint8) (*panIO, mixed.Config) {
	t.Helper()
	owner, cfg, _ := newPauseMixed(t)
	if _, err := owner.r.RequestBus(0, true); err != nil {
		t.Fatal(err)
	}
	state := owner.r.Snapshot()
	state.PCM.Global.StartMarker = marker
	state.PCM.Global.Busy = busy
	state.PCM.Global.RestorePan = 64
	r, err := mixed.Restore(cfg, state)
	if err != nil {
		t.Fatal(err)
	}
	return &panIO{instrumentIO: &instrumentIO{r: r}}, cfg
}

func panAccesses(t *testing.T, events []instrumentNativeEvent) []access {
	t.Helper()
	out := instrumentAccesses(t, events)
	for _, e := range events {
		if e.Owner != 1 {
			continue
		}
		if e.Kind == 17 || e.Kind == 18 {
			kind := "pan-request"
			if e.Kind == 18 {
				kind = "restore-pan"
			}
			out = append(out, access{Kind: kind, At: e.After, Value: uint8(e.Value)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

func TestPanClockOriginalRoutingAndActualMixed(t *testing.T) {
	var f struct {
		Version int
		Cases   []struct {
			Input struct {
				ID, Entry       string
				Channel, Marker uint8
				Pan             uint8
				Claimed         bool
				Origin, Refresh uint64
			}
			Events     []instrumentNativeEvent
			End        scoreclock.Clock
			RestorePan uint8 `json:"restore_pan"`
			Marker     uint8
			Equal      bool `json:"native_observer_state_equal"`
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-pan/native-pan-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 108 {
		t.Fatal("original pan matrix incomplete")
	}
	for _, row := range f.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("observer changed original state")
			}
			e, _ := newPanMixed(t, row.Input.Marker, 0)
			input := row.Input
			req := scoreclock.PanRequest{Entry: input.Entry, Channel: input.Channel, Claimed: input.Claimed, Pan: panValue(input.Pan)}
			tx, err := scoreclock.BeginPanRouting(scoreclock.Clock{Cycle: input.Origin, Refresh: input.Refresh}, req, e, e, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range e.trace {
				if a.Kind == "write" || a.Kind == "restore-pan" || a.Kind == "status" {
					t.Fatal("feedback phase emitted a chip or pan effect")
				}
			}
			if e.r.Snapshot().PCM.Global.RestorePan != 64 {
				t.Fatal("feedback phase changed canonical restore pan")
			}
			effect, hasEffect := tx.ExpectedEffect()
			if hasEffect == input.Claimed {
				t.Fatal("claimed routing emitted an effect, or normal routing lost one")
			}
			if hasEffect {
				want := scoreclock.PanEffect{Kind: "ym", Write: scoreclock.Request{Bank: input.Channel >> 2, Register: 0xb4 + input.Channel&3, Value: input.Pan}}
				if input.Channel == 6 && input.Marker != 0 {
					want = scoreclock.PanEffect{Kind: "dac-pan", RestorePan: input.Pan}
				}
				if effect != want {
					t.Fatal("typed pan effect differs from original", effect, want)
				}
				if err = tx.Apply(effect); err != nil {
					t.Fatal(err)
				}
			}
			end, err := tx.Finish()
			if err != nil || end != row.End {
				t.Fatal("original pan final clock differs", end, row.End, err)
			}
			state := e.r.Snapshot()
			if state.PCM.Global.RestorePan != row.RestorePan || state.PCM.Global.StartMarker != row.Marker || state.PCM.Held != nil && !input.Claimed {
				t.Fatal("canonical PCM pan/marker/bus state differs")
			}
			want := panAccesses(t, row.Events)
			if !reflect.DeepEqual(e.trace, want) {
				n := 0
				for n < len(e.trace) && n < len(want) && e.trace[n] == want[n] {
					n++
				}
				t.Fatalf("actual mixed pan IO differs at%d Go%v Native%v", n, e.trace[max(0, n-1):min(len(e.trace), n+2)], want[max(0, n-1):min(len(want), n+2)])
			}
			if _, err = tx.Finish(); !errors.Is(err, scoreclock.ErrPanSequence) {
				t.Fatal("finished pan routing reused")
			}
		})
	}
}

func TestPanClockStagedIntentAndDetachedFailures(t *testing.T) {
	for _, marker := range []uint8{0, 1, 128, 255} {
		t.Run(string(rune('a'+marker%26)), func(t *testing.T) {
			left, cfg := newPanMixed(t, marker, 0)
			before := left.r.Snapshot()
			copy, err := mixed.Restore(cfg, before)
			if err != nil {
				t.Fatal(err)
			}
			right := &panIO{instrumentIO: &instrumentIO{r: copy}}
			req := scoreclock.PanRequest{Entry: "instrument", Channel: 6, Pan: panValue(181)}
			tx, err := scoreclock.BeginPanRouting(scoreclock.Clock{}, req, left, left, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			effect, ok := tx.ExpectedEffect()
			if !ok {
				t.Fatal("missing normal effect")
			}
			clone, _ := tx.ExpectedEffect()
			clone.RestorePan++
			if err = tx.Apply(effect); err != nil {
				t.Fatal(err)
			}
			end, err := tx.Finish()
			if err != nil {
				t.Fatal(err)
			}
			complete, err := scoreclock.ServePan(scoreclock.Clock{}, req, right, right, scoreclock.Limits{})
			if err != nil || complete != end || !reflect.DeepEqual(left.trace, right.trace) || !reflect.DeepEqual(left.r.Snapshot(), right.r.Snapshot()) {
				t.Fatal("complete and staged pan diverged", err)
			}
			committed := left.r.Snapshot()
			failed, err := mixed.Restore(cfg, before)
			if err != nil {
				t.Fatal(err)
			}
			e := &panIO{instrumentIO: &instrumentIO{r: failed}}
			bad, err := scoreclock.BeginPanRouting(scoreclock.Clock{}, req, e, e, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = bad.Finish(); !errors.Is(err, scoreclock.ErrPanSequence) {
				t.Fatal("premature Finish accepted")
			}
			if err = bad.Apply(effect); !errors.Is(err, scoreclock.ErrPanSequence) {
				t.Fatal("poisoned pan routing continued")
			}
			if !reflect.DeepEqual(committed, left.r.Snapshot()) || !reflect.DeepEqual(committed, right.r.Snapshot()) {
				t.Fatal("detached failure changed committed branch")
			}
		})
	}
}

func TestPanClockBusOwnershipAndFullByte(t *testing.T) {
	owner, cfg, _ := newPauseMixed(t)
	original := owner.r.Snapshot()
	if _, err := owner.r.PanRequest(0); err == nil {
		t.Fatal("unheld marker read accepted")
	}
	if err := owner.r.WriteRestorePan(0, 181); err == nil {
		t.Fatal("unheld pan write accepted")
	}
	if !reflect.DeepEqual(original, owner.r.Snapshot()) {
		t.Fatal("rejected unheld access changed state")
	}
	e, _ := newPanMixed(t, 1, 9)
	before := e.r.Snapshot()
	if got, err := e.r.PanRequest(0); err != nil || got != 1 {
		t.Fatal("marker confused with busy mailbox", got, err)
	}
	for _, at := range []uint64{pcmcommands.FrameCycles + 1, math.MaxUint64} {
		if _, err := e.r.PanRequest(at); err == nil {
			t.Fatal("marker read outside unflushed frame accepted")
		}
		if err := e.r.WriteRestorePan(at, 181); err == nil {
			t.Fatal("pan write outside unflushed frame accepted")
		}
	}
	if !reflect.DeepEqual(before, e.r.Snapshot()) {
		t.Fatal("invalid access partially committed")
	}
	if err := e.r.WriteRestorePan(0, 181); err != nil {
		t.Fatal(err)
	}
	want := before
	want.PCM.Global.RestorePan = 181
	if !reflect.DeepEqual(want, e.r.Snapshot()) {
		t.Fatal("pan write changed another owner or lost AMS/FMS")
	}
	device, err := pcmcommands.Restore(cfg.PCM, before.PCM)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := device.PanRequest(0); err != nil || got != 1 {
		t.Fatal("direct owner marker differs", got, err)
	}
	if err = device.WriteRestorePan(0, 181); err != nil || !reflect.DeepEqual(want.PCM, device.Snapshot()) {
		t.Fatal("direct canonical owner write differs", err)
	}
	req := scoreclock.PanRequest{Entry: "voice", Channel: 6, Pan: panValue(181)}
	if _, err = scoreclock.ServePan(scoreclock.Clock{}, req, e, e, scoreclock.Limits{}); err != nil {
		t.Fatal("pan path incorrectly waited for busy mailbox", err)
	}
	for _, access := range e.trace {
		if access.Kind == "mailbox" || access.Kind == "write" {
			t.Fatal("redirect path ran a YM or mailbox service")
		}
	}
}

func TestPanClockValidationBudgetAndPoisoning(t *testing.T) {
	req := scoreclock.PanRequest{Entry: "voice", Channel: 6, Pan: panValue(181)}
	for _, change := range []func(*scoreclock.PanRequest){
		func(r *scoreclock.PanRequest) { r.Entry = "unknown" },
		func(r *scoreclock.PanRequest) { r.Channel = 3 },
		func(r *scoreclock.PanRequest) { r.Pan.AMS = 4 },
		func(r *scoreclock.PanRequest) { r.Pan.FMS = 8 },
		func(r *scoreclock.PanRequest) { r.Entry = "instrument"; r.Claimed = true },
	} {
		e, _ := newPanMixed(t, 1, 0)
		bad := req
		change(&bad)
		if tx, err := scoreclock.BeginPanRouting(scoreclock.Clock{}, bad, e, e, scoreclock.Limits{}); err == nil || tx != nil || len(e.trace) != 0 {
			t.Fatal("invalid pan input touched external owners")
		}
	}
	e, cfg := newPanMixed(t, 1, 0)
	before := e.r.Snapshot()
	if _, err := scoreclock.BeginPanRouting(scoreclock.Clock{Cycle: math.MaxUint64}, req, e, e, scoreclock.Limits{}); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, err := scoreclock.BeginPanRouting(scoreclock.Clock{}, req, e, e, scoreclock.Limits{MaxOperations: 65537}); err == nil {
		t.Fatal("oversized operation limit accepted")
	}
	if _, err := scoreclock.BeginPanRouting(scoreclock.Clock{}, req, nil, e, scoreclock.Limits{}); err == nil {
		t.Fatal("missing bus accepted")
	}
	if _, err := scoreclock.BeginPanRouting(scoreclock.Clock{}, req, e, nil, scoreclock.Limits{}); err == nil {
		t.Fatal("missing chip accepted")
	}
	for _, scenario := range []string{"wrong-intent", "external-write", "repeat-apply", "budget", "grant-budget"} {
		t.Run(scenario, func(t *testing.T) {
			branch, err := mixed.Restore(cfg, before)
			if err != nil {
				t.Fatal(err)
			}
			x := &panIO{instrumentIO: &instrumentIO{r: branch}}
			if scenario == "budget" || scenario == "grant-budget" {
				x.denyGrant = scenario == "grant-budget"
				localReq := req
				if scenario == "budget" {
					localReq.Channel = 0 // nested YM must share the prefix budget
				}
				if _, err = scoreclock.ServePan(scoreclock.Clock{}, localReq, x, x, scoreclock.Limits{MaxOperations: 24}); !errors.Is(err, scoreclock.ErrBudget) {
					t.Fatal("nested or acquisition budget reset", err)
				}
			} else {
				tx, err := scoreclock.BeginPanRouting(scoreclock.Clock{}, req, x, x, scoreclock.Limits{})
				if err != nil {
					t.Fatal(err)
				}
				effect, _ := tx.ExpectedEffect()
				switch scenario {
				case "wrong-intent":
					effect.RestorePan++
				case "external-write":
					x.failWrite = true
				case "repeat-apply":
					if err = tx.Apply(effect); err != nil {
						t.Fatal(err)
					}
				}
				if err = tx.Apply(effect); err == nil {
					t.Fatal("invalid/repeated/failed effect accepted")
				}
				if _, err = tx.Finish(); !errors.Is(err, scoreclock.ErrPanSequence) {
					t.Fatal("failed pan routing continued")
				}
			}
			if !reflect.DeepEqual(before, e.r.Snapshot()) {
				t.Fatal("failed detached operation mutated original owner")
			}
		})
	}
}
