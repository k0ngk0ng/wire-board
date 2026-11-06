package scoreclock_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type psgAccess struct {
	At    uint64 `json:"at"`
	Value uint8  `json:"value"`
}
type psgOwner struct {
	chip    *chips.Interactive
	trace   []psgAccess
	failAt  int
	failure error
}

func (o *psgOwner) WritePSG(at uint64, value uint8) error {
	o.trace = append(o.trace, psgAccess{At: at, Value: value})
	if o.failAt == len(o.trace) {
		return o.failure
	}
	if o.chip != nil {
		return o.chip.WritePSG(at, value)
	}
	return nil
}
func newPSGOwner(t *testing.T) *psgOwner {
	t.Helper()
	chip, err := chips.NewInteractive(chips.Config{Profile: chips.ReferenceMixedHQ44100}, chips.State{YM: chips.ResetYMState(), PSG: psg.ResetState()})
	if err != nil {
		t.Fatal(err)
	}
	return &psgOwner{chip: chip}
}
func psgKind(t *testing.T, kind string) scoreclock.PSGKind {
	t.Helper()
	switch kind {
	case "single":
		return scoreclock.PSGSingle
	case "pitch-pair":
		return scoreclock.PSGPitchPair
	case "mute-all":
		return scoreclock.PSGMuteAll
	default:
		t.Fatalf("unexpected native leaf %q", kind)
		return 0
	}
}

func TestPSGNativeFixedLeavesWithInteractiveOwner(t *testing.T) {
	raw, err := os.ReadFile("testdata/psg-native-services.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
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
				ID      string `json:"id"`
				Kind    string `json:"kind"`
				Value   uint8  `json:"value"`
				Data    uint8  `json:"data"`
				Origin  uint64 `json:"origin"`
				Refresh uint64 `json:"refresh"`
			} `json:"input"`
			Events []psgAccess      `json:"events"`
			End    scoreclock.Clock `json:"end"`
			PSG    struct {
				Latch       uint8     `json:"latch"`
				ToneDivider [3]uint16 `json:"tone_divider"`
				Level       [4]uint16 `json:"level"`
			} `json:"psg"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 138 {
		t.Fatal("native PSG matrix changed", len(fixture.Cases))
	}
	accesses, stores, singles, pairs, mutes := 0, 0, 0, 0, 0
	for _, row := range fixture.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			owner := newPSGOwner(t)
			origin := scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}
			var got scoreclock.Clock
			var err error
			if row.Input.Kind == "store" {
				got, err = scoreclock.StorePSG(origin, row.Input.Value, owner, scoreclock.Limits{})
				stores++
			} else {
				got, err = scoreclock.ServePSG(origin, scoreclock.PSGRequest{Kind: psgKind(t, row.Input.Kind), Value: row.Input.Value, Data: row.Input.Data}, owner, scoreclock.Limits{})
				switch row.Input.Kind {
				case "single":
					singles++
				case "pitch-pair":
					pairs++
				case "mute-all":
					mutes++
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != row.End {
				t.Fatalf("end clock %+v, native %+v", got, row.End)
			}
			if !reflect.DeepEqual(owner.trace, row.Events) {
				t.Fatalf("ordered PSG writes differ\nGo %+v\nnative %+v", owner.trace, row.Events)
			}
			expected := newPSGOwner(t)
			for _, event := range row.Events {
				if err = expected.chip.WritePSG(event.At, event.Value); err != nil {
					t.Fatal(err)
				}
			}
			state := owner.chip.Snapshot()
			if !reflect.DeepEqual(state, expected.chip.Snapshot()) {
				t.Fatal("service duplicated, replaced or unexpectedly advanced the external owner")
			}
			if state.PSG.Latch != row.PSG.Latch || state.PSG.ToneDivider != row.PSG.ToneDivider || state.PSG.Level != row.PSG.Level {
				t.Fatalf("chip semantic state differs from native: Go %+v native %+v", state.PSG, row.PSG)
			}
			accesses += len(row.Events)
		})
	}
	if stores != 48 || singles != 48 || pairs != 36 || mutes != 6 || accesses != 192 {
		t.Fatal("PSG case coverage changed", stores, singles, pairs, mutes, accesses)
	}
	t.Logf("138 native PSG operations, %d writes; raw store has no return, leaves include one return", accesses)
}

func TestPSGRawStoreAndLeafHaveDistinctCallerBoundary(t *testing.T) {
	clock := scoreclock.Clock{Cycle: 17, Refresh: 4096}
	raw := newPSGOwner(t)
	got, err := scoreclock.StorePSG(clock, 0x9f, raw.chip, scoreclock.Limits{})
	if err != nil || got != (scoreclock.Clock{Cycle: 129, Refresh: 4096}) {
		t.Fatal("raw store included caller or return cost", got, err)
	}
	leaf := newPSGOwner(t)
	after, err := scoreclock.ServePSG(clock, scoreclock.PSGRequest{Kind: scoreclock.PSGSingle, Value: 0x9f}, leaf.chip, scoreclock.Limits{})
	if err != nil || after != (scoreclock.Clock{Cycle: 241, Refresh: 4096}) || !reflect.DeepEqual(raw.chip.Snapshot(), leaf.chip.Snapshot()) {
		t.Fatal("single leaf did not add exactly its own return", after, err)
	}
}

func TestPSGRejectsInvalidInputBeforeExternalIO(t *testing.T) {
	for _, request := range []scoreclock.PSGRequest{
		{}, {Kind: 255},
		{Kind: scoreclock.PSGSingle, Value: 0x9f, Data: 1},
		{Kind: scoreclock.PSGPitchPair, Value: 0x7f},
		{Kind: scoreclock.PSGPitchPair, Value: 0x90},
		{Kind: scoreclock.PSGPitchPair, Value: 0xe0},
		{Kind: scoreclock.PSGPitchPair, Value: 0x80, Data: 64},
		{Kind: scoreclock.PSGMuteAll, Value: 1},
		{Kind: scoreclock.PSGMuteAll, Data: 1},
	} {
		owner := newPSGOwner(t)
		before := owner.chip.Snapshot()
		got, err := scoreclock.ServePSG(scoreclock.Clock{}, request, owner, scoreclock.Limits{})
		if err == nil || got != (scoreclock.Clock{}) || len(owner.trace) != 0 || !reflect.DeepEqual(before, owner.chip.Snapshot()) {
			t.Fatal("invalid PSG payload touched IO", request, got, err, owner.trace)
		}
	}
	for _, raw := range []bool{false, true} {
		for _, row := range []struct {
			clock    scoreclock.Clock
			limit    scoreclock.Limits
			nilOwner bool
		}{
			{nilOwner: true},
			{clock: scoreclock.Clock{Cycle: math.MaxUint64}},
			{limit: scoreclock.Limits{MaxOperations: 65537}},
		} {
			owner := newPSGOwner(t)
			var chip scoreclock.PSGChip = owner
			if row.nilOwner {
				chip = nil
			}
			var got scoreclock.Clock
			var err error
			if raw {
				got, err = scoreclock.StorePSG(row.clock, 0x9f, chip, row.limit)
			} else {
				got, err = scoreclock.ServePSG(row.clock, scoreclock.PSGRequest{Kind: scoreclock.PSGSingle, Value: 0x9f}, chip, row.limit)
			}
			if err == nil || got != (scoreclock.Clock{}) || len(owner.trace) != 0 {
				t.Fatal("invalid PSG clock/owner/limit touched IO", raw, row, got, err, owner.trace)
			}
		}
	}
}

func TestPSGFixedOperationBudgetStopsWithoutRollback(t *testing.T) {
	for _, row := range []struct {
		request scoreclock.PSGRequest
		budget  uint32
		writes  int
	}{
		{scoreclock.PSGRequest{Kind: scoreclock.PSGSingle, Value: 0x9f}, 1, 1},
		{scoreclock.PSGRequest{Kind: scoreclock.PSGPitchPair, Value: 0x80, Data: 63}, 1, 1},
		{scoreclock.PSGRequest{Kind: scoreclock.PSGPitchPair, Value: 0x80, Data: 63}, 2, 2},
		{scoreclock.PSGRequest{Kind: scoreclock.PSGMuteAll}, 1, 0},
		{scoreclock.PSGRequest{Kind: scoreclock.PSGMuteAll}, 3, 2},
		{scoreclock.PSGRequest{Kind: scoreclock.PSGMuteAll}, 5, 4},
	} {
		owner := newPSGOwner(t)
		got, err := scoreclock.ServePSG(scoreclock.Clock{}, row.request, owner, scoreclock.Limits{MaxOperations: row.budget})
		if !errors.Is(err, scoreclock.ErrBudget) || got != (scoreclock.Clock{}) || len(owner.trace) != row.writes {
			t.Fatal("PSG budget failed at wrong semantic boundary", row, got, err, owner.trace)
		}
	}
	owner := newPSGOwner(t)
	if _, err := scoreclock.StorePSG(scoreclock.Clock{}, 0x9f, owner.chip, scoreclock.Limits{MaxOperations: 1}); err != nil {
		t.Fatal("single raw store exceeded one operation", err)
	}
}

func TestPSGExternalFailurePropagatesWithoutLaterWrites(t *testing.T) {
	sentinel := errors.New("declared PSG owner failure")
	for _, row := range []struct {
		raw     bool
		request scoreclock.PSGRequest
		failAt  int
	}{
		{raw: true, failAt: 1},
		{request: scoreclock.PSGRequest{Kind: scoreclock.PSGSingle, Value: 0x9f}, failAt: 1},
		{request: scoreclock.PSGRequest{Kind: scoreclock.PSGPitchPair, Value: 0x80, Data: 63}, failAt: 2},
		{request: scoreclock.PSGRequest{Kind: scoreclock.PSGMuteAll}, failAt: 3},
	} {
		owner := &psgOwner{failAt: row.failAt, failure: sentinel}
		var got scoreclock.Clock
		var err error
		if row.raw {
			got, err = scoreclock.StorePSG(scoreclock.Clock{}, 0x9f, owner, scoreclock.Limits{})
		} else {
			got, err = scoreclock.ServePSG(scoreclock.Clock{}, row.request, owner, scoreclock.Limits{})
		}
		if !errors.Is(err, sentinel) || got != (scoreclock.Clock{}) || len(owner.trace) != row.failAt {
			t.Fatal("PSG error hidden or later writes continued", row, got, err, owner.trace)
		}
	}
}

func TestPSGInteractiveOwnerRejectsBackwardAndUnflushedInput(t *testing.T) {
	for _, cycle := range []uint64{99, chips.FrameCycles + chips.MaxInputAhead + 1} {
		owner := newPSGOwner(t)
		if err := owner.chip.WritePSG(100, 0x90); err != nil {
			t.Fatal(err)
		}
		before := owner.chip.Snapshot()
		got, err := scoreclock.StorePSG(scoreclock.Clock{Cycle: cycle, Refresh: math.MaxUint64}, 0x9f, owner.chip, scoreclock.Limits{})
		if err == nil || got != (scoreclock.Clock{}) || !reflect.DeepEqual(before, owner.chip.Snapshot()) {
			t.Fatal("service hid chip time/frame rejection", cycle, got, err)
		}
	}
}

func TestPSGRefreshOverflowStopsAfterPriorExternalWrite(t *testing.T) {
	owner := &psgOwner{}
	got, err := scoreclock.ServePSG(scoreclock.Clock{Cycle: math.MaxUint64 - 900, Refresh: math.MaxUint64 - 790}, scoreclock.PSGRequest{Kind: scoreclock.PSGSingle, Value: 0x9f}, owner, scoreclock.Limits{})
	if err == nil || got != (scoreclock.Clock{}) || len(owner.trace) != 1 || owner.trace[0].At != math.MaxUint64-900 {
		t.Fatal("late refresh overflow wrapped or hid prior IO", got, err, owner.trace)
	}
}
