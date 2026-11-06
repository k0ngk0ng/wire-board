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

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type fadeClockAfter struct {
	Steps      uint8 `json:"steps"`
	Remaining  uint8 `json:"remaining"`
	DACFlags   uint8 `json:"DAC_flags"`
	Priority   uint8 `json:"priority"`
	Selected   uint8 `json:"selected"`
	ChoiceBits uint8 `json:"choice_bits"`
}
type fadeClockNative struct {
	Input struct {
		ID        string `json:"id"`
		Kind      string `json:"kind"`
		Command   uint8  `json:"command"`
		Steps     uint8  `json:"steps"`
		Remaining uint8  `json:"remaining"`
		DACFlags  uint8  `json:"DAC_flags"`
		Origin    uint64 `json:"origin"`
		Refresh   uint64 `json:"refresh"`
	} `json:"input"`
	Next    string           `json:"next"`
	End     scoreclock.Clock `json:"end"`
	After   fadeClockAfter   `json:"after"`
	Effects []struct {
		At    uint64 `json:"at"`
		Field string `json:"field"`
		Value uint8  `json:"value"`
	} `json:"effects"`
}

func loadFadeClocks(t *testing.T) []fadeClockNative {
	t.Helper()
	raw, err := os.ReadFile("testdata/fade-clock-native.json.gz")
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
		Cases []fadeClockNative `json:"cases"`
	}
	if err = json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 91 {
		t.Fatal("native fade clock matrix changed", len(fixture.Cases))
	}
	return fixture.Cases
}
func nativeFadeClockResult(t *testing.T, row fadeClockNative) scoreclock.FadeClockResult {
	t.Helper()
	want := scoreclock.FadeClockResult{Clock: row.End, EffectCount: uint8(len(row.Effects))}
	switch row.Next {
	case "Done":
		want.Next = scoreclock.FadeDone
	case "LevelScan":
		want.Next = scoreclock.FadeLevelScan
	case "FullClear":
		want.Next = scoreclock.FadeFullClear
	default:
		t.Fatal("unknown native fade boundary", row.Next)
	}
	if len(row.Effects) > 3 {
		t.Fatal("unbounded native fade effects")
	}
	for index, effect := range row.Effects {
		var field scoreclock.FadeEffectField
		switch effect.Field {
		case "remaining":
			field = scoreclock.FadeRemainingStore
		case "steps":
			field = scoreclock.FadeStepsStore
		case "DAC-flags":
			field = scoreclock.FadeDACFlagsClear
		default:
			t.Fatal("unknown native fade effect", effect.Field)
		}
		want.Effects[index] = scoreclock.FadeTimedEffect{At: effect.At, Field: field, Value: effect.Value}
	}
	return want
}
func runFadeClock(row fadeClockNative, limits scoreclock.Limits) (scoreclock.FadeClockResult, error) {
	clock := scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}
	if row.Input.Kind == "start" {
		return scoreclock.ServeFadeStart(clock, row.Input.Command, limits)
	}
	return scoreclock.BeginFadeHeader(clock, scoreclock.FadeHeaderWork{Steps: row.Input.Steps, Remaining: row.Input.Remaining}, limits)
}

func TestFadeClockNativeLocalBoundariesAndTimedWrites(t *testing.T) {
	effects, starts, headers := 0, 0, 0
	routes := map[string]int{}
	for _, row := range loadFadeClocks(t) {
		t.Run(row.Input.ID, func(t *testing.T) {
			got, err := runFadeClock(row, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			want := nativeFadeClockResult(t, row)
			if got != want {
				t.Fatalf("fade clock boundary/effect differs\nGo %+v\nnative %+v", got, want)
			}
			after := fadeClockAfter{Steps: row.Input.Steps, Remaining: row.Input.Remaining, DACFlags: row.Input.DACFlags, Priority: 0x55, Selected: 0xa6, ChoiceBits: 0xfe}
			previous := row.Input.Origin
			for _, effect := range got.Effects[:got.EffectCount] {
				if effect.At < previous || effect.At > got.Clock.Cycle {
					t.Fatal("effect outside ordered local interval", effect, previous, got.Clock)
				}
				previous = effect.At
				switch effect.Field {
				case scoreclock.FadeRemainingStore:
					after.Remaining = effect.Value
				case scoreclock.FadeStepsStore:
					after.Steps = effect.Value
				case scoreclock.FadeDACFlagsClear:
					after.DACFlags = effect.Value
				default:
					t.Fatal("invalid emitted effect", effect)
				}
			}
			if after != row.After {
				t.Fatal("clock effects changed other Root fields or lost native values", after, row.After)
			}
			effects += int(got.EffectCount)
			routes[row.Next]++
			if row.Input.Kind == "start" {
				starts++
			} else {
				headers++
			}
		})
	}
	if starts != 28 || headers != 63 || effects != 154 || routes["Done"] != 63 || routes["LevelScan"] != 21 || routes["FullClear"] != 7 {
		t.Fatal("fade clock coverage changed", starts, headers, effects, routes)
	}
	t.Logf("91 native local profiles,154 precise RAM effects,63 completed/21 LevelScan/7 FullClear boundaries")
}

func TestFadeClockAgreesWithUniquePureFadeOwner(t *testing.T) {
	for _, row := range loadFadeClocks(t) {
		t.Run(row.Input.ID, func(t *testing.T) {
			got, err := runFadeClock(row, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			state := rootcontrol.FadeState{Steps: row.Input.Steps, Remaining: row.Input.Remaining}
			var pure rootcontrol.FadeResult
			if row.Input.Kind == "start" {
				pure, err = rootcontrol.StartFade(state, row.Input.Command)
				if err != nil {
					t.Fatal(err)
				}
				if pure.Action != rootcontrol.StopDACFlags || got.Next != scoreclock.FadeDone || got.EffectCount != pure.WriteCount+1 || got.Effects[pure.WriteCount].Field != scoreclock.FadeDACFlagsClear || got.Effects[pure.WriteCount].Value != 0 {
					t.Fatal("start action/timed DAC clear contract differs", got, pure)
				}
			} else {
				pure = rootcontrol.AdvanceFade(state)
				next := scoreclock.FadeDone
				switch pure.Action {
				case rootcontrol.FadeNoAction:
				case rootcontrol.FadeMusicStep:
					next = scoreclock.FadeLevelScan
				case rootcontrol.RequestFullClear:
					next = scoreclock.FadeFullClear
				default:
					t.Fatal("unexpected owner fade action", pure.Action)
				}
				if got.Next != next || got.EffectCount != pure.WriteCount {
					t.Fatal("header stop differs from actual owner decision", got, pure)
				}
			}
			for index, write := range pure.Writes[:pure.WriteCount] {
				field := scoreclock.FadeStepsStore
				if write.Counter == rootcontrol.FadeRemainingCounter {
					field = scoreclock.FadeRemainingStore
				} else if write.Counter != rootcontrol.FadeStepsCounter {
					t.Fatal("unknown pure owner effect", write)
				}
				if got.Effects[index].Field != field || got.Effects[index].Value != write.Value {
					t.Fatal("clock invented a different counter effect", got, pure)
				}
			}
			if pure.State.Steps != row.After.Steps || pure.State.Remaining != row.After.Remaining {
				t.Fatal("owner and native counter states differ", pure, row.After)
			}
		})
	}
}

func TestFadeClockFixedCallerAndUnfinishedBoundaryContract(t *testing.T) {
	origin := scoreclock.Clock{Cycle: 1, Refresh: 4096}
	start, err := scoreclock.ServeFadeStart(origin, 1, scoreclock.Limits{})
	if err != nil || start.Clock != (scoreclock.Clock{Cycle: 449, Refresh: 4096}) || start.Next != scoreclock.FadeDone || start.EffectCount != 3 || start.Effects != ([3]scoreclock.FadeTimedEffect{{At: 1, Field: scoreclock.FadeRemainingStore, Value: 3}, {At: 113, Field: scoreclock.FadeStepsStore, Value: 40}, {At: 225, Field: scoreclock.FadeDACFlagsClear, Value: 0}}) {
		t.Fatal("start included caller or omitted16cpu CLR/RTS", start, err)
	}
	for _, row := range []struct {
		work    scoreclock.FadeHeaderWork
		clock   uint64
		next    scoreclock.FadeNext
		effects []scoreclock.FadeTimedEffect
	}{
		{scoreclock.FadeHeaderWork{Remaining: 255}, 421, scoreclock.FadeDone, nil},
		{scoreclock.FadeHeaderWork{Steps: 1, Remaining: 1}, 659, scoreclock.FadeDone, []scoreclock.FadeTimedEffect{{At: 435, Field: scoreclock.FadeRemainingStore, Value: 0}}},
		{scoreclock.FadeHeaderWork{Steps: 2}, 841, scoreclock.FadeLevelScan, []scoreclock.FadeTimedEffect{{At: 449, Field: scoreclock.FadeStepsStore, Value: 1}, {At: 645, Field: scoreclock.FadeRemainingStore, Value: 3}}},
		{scoreclock.FadeHeaderWork{Steps: 1}, 631, scoreclock.FadeFullClear, []scoreclock.FadeTimedEffect{{At: 449, Field: scoreclock.FadeStepsStore, Value: 0}}},
	} {
		got, err := scoreclock.BeginFadeHeader(origin, row.work, scoreclock.Limits{})
		if err != nil || got.Clock != (scoreclock.Clock{Cycle: row.clock, Refresh: 4096}) || got.Next != row.next || int(got.EffectCount) != len(row.effects) {
			t.Fatal("header finished or priced the wrong boundary", row, got, err)
		}
		if len(row.effects) != 0 && !reflect.DeepEqual(got.Effects[:got.EffectCount], row.effects) {
			t.Fatal("header moved writes to completion", got, row.effects)
		}
	}
}

func TestFadeClockRejectsUnknownStartAndInvalidBounds(t *testing.T) {
	for command := 0; command < 256; command++ {
		if command == 1 || command == 5 {
			continue
		}
		got, err := scoreclock.ServeFadeStart(scoreclock.Clock{}, uint8(command), scoreclock.Limits{})
		if !errors.Is(err, scoreclock.ErrFadeClockUnsupported) || got != (scoreclock.FadeClockResult{}) {
			t.Fatal("unknown local start program accepted", command, got, err)
		}
	}
	for _, run := range []func(scoreclock.Clock, scoreclock.Limits) (scoreclock.FadeClockResult, error){
		func(c scoreclock.Clock, l scoreclock.Limits) (scoreclock.FadeClockResult, error) {
			return scoreclock.ServeFadeStart(c, 1, l)
		},
		func(c scoreclock.Clock, l scoreclock.Limits) (scoreclock.FadeClockResult, error) {
			return scoreclock.BeginFadeHeader(c, scoreclock.FadeHeaderWork{Steps: 2}, l)
		},
	} {
		for _, row := range []struct {
			clock  scoreclock.Clock
			limits scoreclock.Limits
		}{
			{clock: scoreclock.Clock{Cycle: math.MaxUint64}},
			{limits: scoreclock.Limits{MaxOperations: 65537}},
			{clock: scoreclock.Clock{Cycle: math.MaxUint64 - 900, Refresh: math.MaxUint64 - 790}},
		} {
			got, err := run(row.clock, row.limits)
			if err == nil || got != (scoreclock.FadeClockResult{}) {
				t.Fatal("invalid clock/limit leaked partial timed effects", row, got, err)
			}
		}
	}
	for _, run := range []func() (scoreclock.FadeClockResult, error){
		func() (scoreclock.FadeClockResult, error) {
			return scoreclock.ServeFadeStart(scoreclock.Clock{Cycle: math.MaxUint64 - 1000, Refresh: math.MaxUint64 - 800}, 1, scoreclock.Limits{})
		},
		func() (scoreclock.FadeClockResult, error) {
			return scoreclock.BeginFadeHeader(scoreclock.Clock{Cycle: math.MaxUint64 - 1100, Refresh: math.MaxUint64 - 600}, scoreclock.FadeHeaderWork{Steps: 2}, scoreclock.Limits{})
		},
	} {
		got, err := run()
		if err == nil || got != (scoreclock.FadeClockResult{}) {
			t.Fatal("late refresh overflow returned partial plan", got, err)
		}
	}
}

func TestFadeClockOperationBudgetsDiscardPartialEffects(t *testing.T) {
	origin := scoreclock.Clock{Cycle: 1, Refresh: 4096}
	type run func(scoreclock.Limits) (scoreclock.FadeClockResult, error)
	for _, row := range []struct {
		operations uint32
		invoke     run
	}{
		{4, func(l scoreclock.Limits) (scoreclock.FadeClockResult, error) {
			return scoreclock.ServeFadeStart(origin, 1, l)
		}},
		{5, func(l scoreclock.Limits) (scoreclock.FadeClockResult, error) {
			return scoreclock.BeginFadeHeader(origin, scoreclock.FadeHeaderWork{}, l)
		}},
		{8, func(l scoreclock.Limits) (scoreclock.FadeClockResult, error) {
			return scoreclock.BeginFadeHeader(origin, scoreclock.FadeHeaderWork{Steps: 1, Remaining: 1}, l)
		}},
		{8, func(l scoreclock.Limits) (scoreclock.FadeClockResult, error) {
			return scoreclock.BeginFadeHeader(origin, scoreclock.FadeHeaderWork{Steps: 1}, l)
		}},
		{11, func(l scoreclock.Limits) (scoreclock.FadeClockResult, error) {
			return scoreclock.BeginFadeHeader(origin, scoreclock.FadeHeaderWork{Steps: 2}, l)
		}},
	} {
		for budget := uint32(1); budget < row.operations; budget++ {
			got, err := row.invoke(scoreclock.Limits{MaxOperations: budget})
			if !errors.Is(err, scoreclock.ErrBudget) || got != (scoreclock.FadeClockResult{}) {
				t.Fatal("budget emitted partial timed work", row.operations, budget, got, err)
			}
		}
		if _, err := row.invoke(scoreclock.Limits{MaxOperations: row.operations}); err != nil {
			t.Fatal("exact local budget failed", row.operations, err)
		}
	}
}

func TestFadeClockWorkAndEffectsAreBoundedTransientValues(t *testing.T) {
	work := reflect.TypeOf(scoreclock.FadeHeaderWork{})
	if work.NumField() != 2 {
		t.Fatal("timing work copied another owner", work)
	}
	for _, name := range []string{"Steps", "Remaining"} {
		field, ok := work.FieldByName(name)
		if !ok || field.Type.Kind() != reflect.Uint8 {
			t.Fatal("timing work has unknown profile or persistent state", name, work)
		}
	}
	result := reflect.TypeOf(scoreclock.FadeClockResult{})
	if result.NumField() != 4 {
		t.Fatal("timing result acquired shadow state", result)
	}
	for _, name := range []string{"Clock", "Next", "Effects", "EffectCount"} {
		if _, ok := result.FieldByName(name); !ok {
			t.Fatal("timing result contract changed", name)
		}
	}
	effects, _ := result.FieldByName("Effects")
	if effects.Type.Kind() != reflect.Array || effects.Type.Len() != 3 || reflect.TypeOf(scoreclock.FadeTimedEffect{}).NumField() != 3 {
		t.Fatal("timed effects became an unbounded program or state copy", effects.Type)
	}
}
