package scoreclock_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

func TestCommitModulationClockOriginalBranches(t *testing.T) {
	var fixture struct {
		Version int
		Cases   []struct {
			Input struct {
				ID, Kind, Family   string
				Tied, Enabled      bool
				ModulationEnabled  bool `json:"modulation_enabled"`
				DACSample          bool `json:"dac_sample"`
				Delay, Rate, Steps uint8
				Origin, Refresh    uint64
			}
			End   scoreclock.Clock
			Equal bool `json:"native_passive_state_equal"`
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-commit-modulation/native-commit-mod-full.json.gz"), &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Cases) != 188 {
		t.Fatal("original cleanup/modulation matrix incomplete")
	}
	for _, row := range fixture.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("passive native recording changed state")
			}
			input := row.Input
			clock := scoreclock.Clock{Cycle: input.Origin, Refresh: input.Refresh}
			var got scoreclock.Clock
			var err error
			switch input.Kind {
			case "commit":
				work := scoreclock.NoteCommitWork{Family: input.Family, Tied: input.Tied, ModulationEnabled: input.ModulationEnabled, DACSample: input.DACSample}
				result, failure := scoreclock.PriceNoteCommit(clock, work, scoreclock.Limits{})
				got, err = result.Clock, failure
				if result.NeedsDACPublication != input.DACSample {
					t.Fatal("DAC commit ended at wrong phase")
				}
			case "modulation":
				work := scoreclock.ModulationWork{Enabled: input.Enabled, Delay: input.Delay, Rate: input.Rate, Steps: input.Steps}
				got, err = scoreclock.PriceModulationWork(clock, work, scoreclock.Limits{})
			default:
				t.Fatal("unknown original profile")
			}
			if err != nil || got != row.End {
				t.Fatal("original per-operation clock differs", got, row.End, err)
			}
		})
	}
}

func TestCommitModulationClockValidationAndSharedBudget(t *testing.T) {
	for _, work := range []scoreclock.NoteCommitWork{
		{}, {Family: "unknown"}, {Family: "fm", DACSample: true},
		{Family: "psg", DACSample: true}, {Family: "dac", Tied: true},
		{Family: "dac", ModulationEnabled: true},
	} {
		result, err := scoreclock.PriceNoteCommit(scoreclock.Clock{}, work, scoreclock.Limits{})
		if !errors.Is(err, scoreclock.ErrNoteWorkUnavailable) || result != (scoreclock.NoteCommitResult{}) {
			t.Fatal("unknown or contradictory note-commit work priced", work, result, err)
		}
	}
	commit := scoreclock.NoteCommitWork{Family: "fm", ModulationEnabled: true}
	modulation := scoreclock.ModulationWork{Enabled: true, Rate: 1, Steps: 1}
	for _, limits := range []scoreclock.Limits{{MaxOperations: 1}, {MaxOperations: 12}} {
		result, err := scoreclock.PriceNoteCommit(scoreclock.Clock{}, commit, limits)
		if !errors.Is(err, scoreclock.ErrBudget) || result != (scoreclock.NoteCommitResult{}) {
			t.Fatal("cleanup reset body used a fresh budget or exposed partial success", result, err)
		}
		clock, err := scoreclock.PriceModulationWork(scoreclock.Clock{}, modulation, limits)
		if !errors.Is(err, scoreclock.ErrBudget) || clock != (scoreclock.Clock{}) {
			t.Fatal("modulation body used a fresh budget or exposed partial success", clock, err)
		}
	}
	for _, bad := range []struct {
		clock  scoreclock.Clock
		limits scoreclock.Limits
	}{
		{clock: scoreclock.Clock{Cycle: math.MaxUint64}},
		{limits: scoreclock.Limits{MaxOperations: 65537}},
	} {
		if result, err := scoreclock.PriceNoteCommit(bad.clock, commit, bad.limits); err == nil || result != (scoreclock.NoteCommitResult{}) {
			t.Fatal("invalid cleanup clock/limit accepted", result, err)
		}
		if result, err := scoreclock.PriceModulationWork(bad.clock, modulation, bad.limits); err == nil || result != (scoreclock.Clock{}) {
			t.Fatal("invalid modulation clock/limit accepted", result, err)
		}
	}
}
