package scoreclock

import (
	"errors"
	"math"
)

// FadeHeaderWork is an invocation-local projection of the unique Root fade
// owner. It is stable for this SR2700, no-extra-IRQ/no-external-stall profile.
// It is not another persisted counter state or a live shared-RAM view.
type FadeHeaderWork struct{ Steps, Remaining uint8 }

type FadeNext uint8

const (
	FadeDone FadeNext = iota + 1
	FadeLevelScan
	FadeFullClear
)

type FadeEffectField uint8

const (
	FadeRemainingStore FadeEffectField = iota + 1
	FadeStepsStore
	FadeDACFlagsClear
)

// FadeTimedEffect names an actual semantic write point. Field is never an
// original address, opcode or caller-supplied instruction.
type FadeTimedEffect struct {
	At    uint64
	Field FadeEffectField
	Value uint8
}

// FadeClockResult is temporary bounded work for the detached orchestrator.
// LevelScan and FullClear are unfinished semantic boundaries, not zero-cost
// completions. Apply effects in order once, then continue from Clock/Next.
type FadeClockResult struct {
	Clock       Clock
	Next        FadeNext
	Effects     [3]FadeTimedEffect
	EffectCount uint8
}

var ErrFadeClockUnsupported = errors.New("unsupported fixed fade-start command")

func fadeClockService(clock Clock, limits Limits) (*service, error) {
	if limits.MaxOperations > 65536 || clock.Cycle > math.MaxUint64-896 {
		return nil, errors.New("invalid fixed fade clock or operation limit")
	}
	limit := limits.MaxOperations
	if limit == 0 {
		limit = 4096
	}
	return &service{clock: clock, limit: limit}, nil
}

func fadeClockStore(s *service, result *FadeClockResult, field FadeEffectField, value uint8) error {
	// Each of these fixed absolute-word byte stores has a16CPU source cost.
	// Main RAM writes have no YM/ZRAM seven-master access delay.
	return s.cost(16, func() error {
		result.Effects[result.EffectCount] = FadeTimedEffect{At: s.clock.Cycle, Field: field, Value: value}
		result.EffectCount++
		return nil
	})
}

// ServeFadeStart performs the fixed01/05 start leaf, including one return but
// excluding dispatcher/table/caller costs. It only reports timed effects; the
// Root and score owners apply each effect once at its returned phase.
func ServeFadeStart(clock Clock, command uint8, limits Limits) (FadeClockResult, error) {
	var remaining, steps uint8
	switch command {
	case 1:
		remaining, steps = 3, 40
	case 5:
		remaining, steps = 32, 50
	default:
		return FadeClockResult{}, ErrFadeClockUnsupported
	}
	s, err := fadeClockService(clock, limits)
	if err != nil {
		return FadeClockResult{}, err
	}
	result := FadeClockResult{Next: FadeDone}
	if err = fadeClockStore(s, &result, FadeRemainingStore, remaining); err != nil {
		return FadeClockResult{}, err
	}
	if err = fadeClockStore(s, &result, FadeStepsStore, steps); err != nil {
		return FadeClockResult{}, err
	}
	if err = fadeClockStore(s, &result, FadeDACFlagsClear, 0); err != nil {
		return FadeClockResult{}, err
	}
	if err = s.spend(16); err != nil {
		return FadeClockResult{}, err
	} // return
	result.Clock = s.clock
	return result, nil
}

// BeginFadeHeader begins at the original PC-relative caller and stops after
// an idle/timer return or before the dynamic level scan/full04 clear. Work is
// stable for this local profile; no IRQ, bus wait or other owner is advanced.
// It does not apply Root/score writes or persist a clock/counter shadow.
func BeginFadeHeader(clock Clock, work FadeHeaderWork, limits Limits) (FadeClockResult, error) {
	s, err := fadeClockService(clock, limits)
	if err != nil {
		return FadeClockResult{}, err
	}
	result := FadeClockResult{}
	if err = s.spend(18); err != nil {
		return FadeClockResult{}, err
	} // original caller
	if err = s.spend(4); err != nil {
		return FadeClockResult{}, err
	} // clear temporary
	if err = s.spend(12); err != nil {
		return FadeClockResult{}, err
	} // read steps
	if err = s.branch(work.Steps == 0, false); err != nil {
		return FadeClockResult{}, err
	}
	if work.Steps == 0 {
		if err = s.spend(16); err != nil {
			return FadeClockResult{}, err
		}
		result.Clock, result.Next = s.clock, FadeDone
		return result, nil
	}
	if err = s.spend(12); err != nil {
		return FadeClockResult{}, err
	} // read remaining
	if err = s.branch(work.Remaining == 0, false); err != nil {
		return FadeClockResult{}, err
	}
	if work.Remaining != 0 {
		if err = fadeClockStore(s, &result, FadeRemainingStore, work.Remaining-1); err != nil {
			return FadeClockResult{}, err
		}
		if err = s.spend(16); err != nil {
			return FadeClockResult{}, err
		}
		result.Clock, result.Next = s.clock, FadeDone
		return result, nil
	}
	steps := work.Steps - 1
	if err = fadeClockStore(s, &result, FadeStepsStore, steps); err != nil {
		return FadeClockResult{}, err
	}
	if err = s.branch(steps == 0, true); err != nil {
		return FadeClockResult{}, err
	}
	if steps == 0 {
		result.Clock, result.Next = s.clock, FadeFullClear
		return result, nil
	}
	if err = fadeClockStore(s, &result, FadeRemainingStore, 3); err != nil {
		return FadeClockResult{}, err
	}
	if err = s.spend(8); err != nil {
		return FadeClockResult{}, err
	} // select FM music rows
	if err = s.spend(4); err != nil {
		return FadeClockResult{}, err
	} // fixed scan setup
	result.Clock, result.Next = s.clock, FadeLevelScan
	return result, nil
}
