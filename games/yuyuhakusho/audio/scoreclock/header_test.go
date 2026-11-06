package scoreclock_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/bits"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type headerTempo struct {
	Enabled         bool   `json:"enabled"`
	Reload          bool   `json:"reload"`
	ActiveMusicMask uint16 `json:"active_music_mask"`
}

func (w headerTempo) work() scoreclock.TempoWork {
	return scoreclock.TempoWork{Enabled: w.Enabled, Reload: w.Reload, ActiveMusicMask: w.ActiveMusicMask}
}

func TestHeaderNativeRoutineAndCombinedStops(t *testing.T) {
	raw, err := os.ReadFile("testdata/header-native-calls.json.gz")
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
				ID      string      `json:"id"`
				Kind    string      `json:"kind"`
				Origin  uint64      `json:"origin"`
				Refresh uint64      `json:"refresh"`
				Tempo   headerTempo `json:"tempo"`
			} `json:"input"`
			Boundaries []scoreclock.Clock `json:"boundaries"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 170 {
		t.Fatal("native header matrix changed", len(fixture.Cases))
	}
	counts := map[string]int{}
	boundaries := 0
	masks := map[uint16]bool{}
	for _, row := range fixture.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			clock := scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}
			var got []scoreclock.Clock
			var err error
			switch row.Input.Kind {
			case "control":
				clock, err = scoreclock.CallControlIdle(clock, scoreclock.ControlWork{}, scoreclock.Limits{})
				got = append(got, clock)
			case "tempo":
				clock, err = scoreclock.CallTempo(clock, row.Input.Tempo.work(), scoreclock.Limits{})
				got = append(got, clock)
			case "fade":
				clock, err = scoreclock.CallFadeIdle(clock, 0, scoreclock.Limits{})
				got = append(got, clock)
			case "combined":
				clock, err = scoreclock.CallControlIdle(clock, scoreclock.ControlWork{}, scoreclock.Limits{})
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, clock)
				clock, err = scoreclock.CallTempo(clock, row.Input.Tempo.work(), scoreclock.Limits{})
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, clock)
				clock, err = scoreclock.CallFadeIdle(clock, 0, scoreclock.Limits{})
				got = append(got, clock)
			default:
				t.Fatal("unknown native header profile", row.Input.Kind)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, row.Boundaries) {
				t.Fatalf("header stops differ\nGo %+v\nnative %+v", got, row.Boundaries)
			}
			counts[row.Input.Kind]++
			boundaries += len(got)
			if row.Input.Tempo.Reload {
				masks[row.Input.Tempo.ActiveMusicMask] = true
			}
		})
	}
	if counts["control"] != 5 || counts["fade"] != 5 || counts["tempo"] != 80 || counts["combined"] != 80 || boundaries != 330 || len(masks) != 14 {
		t.Fatal("header coverage changed", counts, boundaries, len(masks))
	}
	for slot := 0; slot < 10; slot++ {
		if !masks[1<<slot] {
			t.Fatal("native reload missing slot position", slot)
		}
	}
	t.Logf("170 native header profiles, 330 original routine stops, 14 reload masks")
}

func TestHeaderCallerAndReturnAreIncludedExactlyOnce(t *testing.T) {
	origin := scoreclock.Clock{Cycle: 1, Refresh: 8192}
	control, err := scoreclock.CallControlIdle(origin, scoreclock.ControlWork{}, scoreclock.Limits{})
	if err != nil || control != (scoreclock.Clock{Cycle: 939, Refresh: 8192}) {
		t.Fatal("control caller/callee/return cost", control, err)
	}
	fade, err := scoreclock.CallFadeIdle(origin, 0, scoreclock.Limits{})
	if err != nil || fade != (scoreclock.Clock{Cycle: 421, Refresh: 8192}) {
		t.Fatal("fade caller/callee/return cost", fade, err)
	}
	for _, row := range []struct {
		work  scoreclock.TempoWork
		after uint64
	}{
		{scoreclock.TempoWork{}, 393},
		{scoreclock.TempoWork{Enabled: true}, 561},
		{scoreclock.TempoWork{Enabled: true, Reload: true}, 3347},
		{scoreclock.TempoWork{Enabled: true, Reload: true, ActiveMusicMask: 0x3ff}, 4327},
	} {
		got, err := scoreclock.CallTempo(origin, row.work, scoreclock.Limits{})
		if err != nil || got != (scoreclock.Clock{Cycle: row.after, Refresh: 8192}) {
			t.Fatal("tempo caller/callee/return cost", row, got, err)
		}
	}
}

func TestHeaderUnsupportedControlAndFadeProfiles(t *testing.T) {
	for value := 1; value < 256; value++ {
		for _, work := range []scoreclock.ControlWork{{Mode: uint8(value)}, {AuxMode: uint8(value)}} {
			got, err := scoreclock.CallControlIdle(scoreclock.Clock{}, work, scoreclock.Limits{})
			if !errors.Is(err, scoreclock.ErrHeaderUnsupported) || got != (scoreclock.Clock{}) {
				t.Fatal("non-idle control accepted", work, got, err)
			}
		}
		got, err := scoreclock.CallFadeIdle(scoreclock.Clock{}, uint8(value), scoreclock.Limits{})
		if !errors.Is(err, scoreclock.ErrHeaderUnsupported) || got != (scoreclock.Clock{}) {
			t.Fatal("non-idle fade accepted", value, got, err)
		}
	}
}

func TestHeaderRejectsInconsistentTempoWork(t *testing.T) {
	for _, work := range []scoreclock.TempoWork{
		{Reload: true}, {ActiveMusicMask: 1}, {Enabled: true, ActiveMusicMask: 1},
		{Enabled: true, Reload: true, ActiveMusicMask: 0x400},
		{Enabled: true, Reload: true, ActiveMusicMask: 0xffff},
	} {
		got, err := scoreclock.CallTempo(scoreclock.Clock{}, work, scoreclock.Limits{})
		if err == nil || errors.Is(err, scoreclock.ErrHeaderUnsupported) || got != (scoreclock.Clock{}) {
			t.Fatal("inconsistent temporary tempo work accepted", work, got, err)
		}
	}
}

func TestHeaderFixedOperationBudgets(t *testing.T) {
	origin := scoreclock.Clock{Cycle: 1, Refresh: 8192}
	type call func(scoreclock.Limits) (scoreclock.Clock, error)
	rows := []struct {
		name       string
		operations uint32
		run        call
	}{
		{"control", 10, func(l scoreclock.Limits) (scoreclock.Clock, error) {
			return scoreclock.CallControlIdle(origin, scoreclock.ControlWork{}, l)
		}},
		{"fade", 5, func(l scoreclock.Limits) (scoreclock.Clock, error) { return scoreclock.CallFadeIdle(origin, 0, l) }},
		{"disabled", 4, func(l scoreclock.Limits) (scoreclock.Clock, error) {
			return scoreclock.CallTempo(origin, scoreclock.TempoWork{}, l)
		}},
		{"step", 6, func(l scoreclock.Limits) (scoreclock.Clock, error) {
			return scoreclock.CallTempo(origin, scoreclock.TempoWork{Enabled: true}, l)
		}},
	}
	for _, mask := range []uint16{0, 1, 0x200, 0x155, 0x2aa, 0x3ff} {
		mask := mask
		rows = append(rows, struct {
			name       string
			operations uint32
			run        call
		}{"reload", uint32(50 + bits.OnesCount16(mask)), func(l scoreclock.Limits) (scoreclock.Clock, error) {
			return scoreclock.CallTempo(origin, scoreclock.TempoWork{Enabled: true, Reload: true, ActiveMusicMask: mask}, l)
		}})
	}
	for _, row := range rows {
		for budget := uint32(1); budget < row.operations; budget++ {
			got, err := row.run(scoreclock.Limits{MaxOperations: budget})
			if !errors.Is(err, scoreclock.ErrBudget) || got != (scoreclock.Clock{}) {
				t.Fatal("fixed header budget crossed wrong boundary", row.name, budget, got, err)
			}
		}
		if _, err := row.run(scoreclock.Limits{MaxOperations: row.operations}); err != nil {
			t.Fatal("complete fixed header budget failed", row.name, row.operations, err)
		}
	}
}

func TestHeaderInvalidClockLimitAndLateOverflow(t *testing.T) {
	functions := []func(scoreclock.Clock, scoreclock.Limits) (scoreclock.Clock, error){
		func(c scoreclock.Clock, l scoreclock.Limits) (scoreclock.Clock, error) {
			return scoreclock.CallControlIdle(c, scoreclock.ControlWork{}, l)
		},
		func(c scoreclock.Clock, l scoreclock.Limits) (scoreclock.Clock, error) {
			return scoreclock.CallFadeIdle(c, 0, l)
		},
		func(c scoreclock.Clock, l scoreclock.Limits) (scoreclock.Clock, error) {
			return scoreclock.CallTempo(c, scoreclock.TempoWork{Enabled: true, Reload: true, ActiveMusicMask: 0x3ff}, l)
		},
	}
	for _, run := range functions {
		for _, row := range []struct {
			clock  scoreclock.Clock
			limits scoreclock.Limits
		}{
			{clock: scoreclock.Clock{Cycle: math.MaxUint64}},
			{limits: scoreclock.Limits{MaxOperations: 65537}},
			{clock: scoreclock.Clock{Cycle: math.MaxUint64 - 900, Refresh: math.MaxUint64 - 790}},
		} {
			got, err := run(row.clock, row.limits)
			if err == nil || got != (scoreclock.Clock{}) {
				t.Fatal("invalid/overflowing header clock accepted", row, got, err)
			}
		}
	}
	got, err := scoreclock.CallTempo(scoreclock.Clock{Cycle: math.MaxUint64 - 2000, Refresh: math.MaxUint64}, scoreclock.TempoWork{Enabled: true, Reload: true, ActiveMusicMask: 0x3ff}, scoreclock.Limits{})
	if err == nil || got != (scoreclock.Clock{}) {
		t.Fatal("later tempo cycle overflow wrapped", got, err)
	}
}
