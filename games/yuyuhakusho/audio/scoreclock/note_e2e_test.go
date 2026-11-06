package scoreclock_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

func TestNoteClockOriginalPitchAndProbeBoundaries(t *testing.T) {
	var f struct {
		Version int
		Cases   []struct {
			Input struct {
				ID, Kind, Family, Form, Outcome string
				Index                           uint8
				Origin, Refresh                 uint64
			}
			End   scoreclock.Clock
			Equal bool `json:"native_passive_state_equal"`
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-note/native-note-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 1064 {
		t.Fatal("native note profile incomplete")
	}
	for _, row := range f.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("passive native trace changed state")
			}
			clock := scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}
			var got scoreclock.Clock
			var err error
			if row.Input.Kind == "pitch" {
				request := scoreclock.PitchWork{Family: row.Input.Family, Form: row.Input.Form, Index: row.Input.Index}
				before := request
				got, err = scoreclock.PricePitchWork(clock, request, scoreclock.Limits{})
				if before != request {
					t.Fatal("pitch input mutated")
				}
			} else {
				request := scoreclock.DurationProbeWork{Family: row.Input.Family, Outcome: row.Input.Outcome}
				before := request
				got, err = scoreclock.PriceDurationProbe(clock, request, scoreclock.Limits{})
				if before != request {
					t.Fatal("probe input mutated")
				}
			}
			if err != nil || got != row.End {
				t.Fatalf("clock%+v native%+v err%v", got, row.End, err)
			}
		})
	}
}

func TestNoteClockOracleMatchesExistingPitchResources(t *testing.T) {
	var f struct {
		Cases []struct {
			Input struct {
				Kind, Family, Form, Outcome string
				Index                       uint8
			}
			Word uint16 `json:"pitch_word"`
			Off  bool   `json:"note_off"`
			Wait uint8
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-note/native-note-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	var data struct {
		Tables struct {
			FM       []uint16 `json:"fm_fnums"`
			PSG      []uint16 `json:"psg_dividers"`
			Extended []struct {
				Index   int
				Divider uint16
			} `json:"psg_extended_dividers"`
		} `json:"pitch_tables"`
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../data/audio-score.json.gz"), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Tables.FM) != 12 || len(data.Tables.PSG) != 70 {
		t.Fatal("existing pitch resource shape changed")
	}
	psg := map[int]uint16{}
	for i, v := range data.Tables.PSG {
		psg[i] = v
	}
	for _, v := range data.Tables.Extended {
		psg[v.Index] = v.Divider
	}
	fm, knownPSG, rest, dac, probe := 0, 0, 0, 0, 0
	for _, row := range f.Cases {
		request := row.Input
		if request.Kind == "probe" {
			if request.Family == "dac" && request.Outcome == "reuse" && row.Wait != 0x23 {
				t.Fatal("source DAC did not reuse old duration")
			}
			probe++
			continue
		}
		if request.Form == "rest" {
			switch request.Family {
			case "fm":
				if row.Word != 0 || !row.Off {
					t.Fatal("source FM rest differs")
				}
			case "psg":
				if row.Word != 0xffff || !row.Off {
					t.Fatal("source PSG rest differs")
				}
			case "dac":
				if row.Word != 0x8000 {
					t.Fatal("source DAC rest marker differs")
				}
			}
			rest++
			continue
		}
		i := int(request.Index)
		switch request.Family {
		case "fm":
			want := data.Tables.FM[i%12] | uint16((i/12)&7)<<11
			if row.Word != want {
				t.Fatal("existing FM arithmetic differs", i, row.Word, want)
			}
			fm++
		case "psg":
			if want, ok := psg[i]; ok {
				if row.Word != want {
					t.Fatal("existing PSG table differs", i, row.Word, want)
				}
				knownPSG++
			}
		case "dac":
			if row.Word != 0x8100 {
				t.Fatal("source DAC marker differs")
			}
			dac++
		}
	}
	if fm != 512 || knownPSG != len(psg)*4 || rest != 12 || dac != 4 || probe != 24 {
		t.Fatal("note resource coverage incomplete", fm, knownPSG, rest, dac, probe)
	}
}

func TestNoteClockRejectsUnknownInputsAndBoundedFailure(t *testing.T) {
	for _, request := range []scoreclock.PitchWork{{Family: "unknown", Form: "rest"}, {Family: "fm", Form: "dac"}, {Family: "fm", Form: "rest", Index: 1}, {Family: "fm", Form: "chromatic", Index: 128}, {Family: "psg", Form: "dac"}, {Family: "dac", Form: "chromatic"}} {
		got, err := scoreclock.PricePitchWork(scoreclock.Clock{}, request, scoreclock.Limits{})
		if !errors.Is(err, scoreclock.ErrNoteWorkUnavailable) || got != (scoreclock.Clock{}) {
			t.Fatal("unknown pitch profile accepted", request, got, err)
		}
	}
	for _, request := range []scoreclock.DurationProbeWork{{Family: "unknown", Outcome: "take"}, {Family: "fm", Outcome: "unknown"}} {
		got, err := scoreclock.PriceDurationProbe(scoreclock.Clock{}, request, scoreclock.Limits{})
		if !errors.Is(err, scoreclock.ErrNoteWorkUnavailable) || got != (scoreclock.Clock{}) {
			t.Fatal("unknown probe accepted", request, got, err)
		}
	}
	request := scoreclock.PitchWork{Family: "fm", Form: "chromatic"}
	if got, err := scoreclock.PricePitchWork(scoreclock.Clock{}, request, scoreclock.Limits{MaxOperations: 1}); !errors.Is(err, scoreclock.ErrBudget) || got != (scoreclock.Clock{}) {
		t.Fatal("pitch budget not enforced", got, err)
	}
	for _, clock := range []scoreclock.Clock{{Cycle: math.MaxUint64}, {Cycle: math.MaxUint64 - 1000, Refresh: math.MaxUint64 - 100}} {
		if got, err := scoreclock.PricePitchWork(clock, request, scoreclock.Limits{}); err == nil || got != (scoreclock.Clock{}) {
			t.Fatal("long note arithmetic overflow accepted", got, err)
		}
	}
	if got, err := scoreclock.PriceDurationProbe(scoreclock.Clock{}, scoreclock.DurationProbeWork{Family: "psg", Outcome: "reuse"}, scoreclock.Limits{MaxOperations: 1}); !errors.Is(err, scoreclock.ErrBudget) || got != (scoreclock.Clock{}) {
		t.Fatal("probe budget not enforced", got, err)
	}
	if got, err := scoreclock.PricePitchWork(scoreclock.Clock{}, request, scoreclock.Limits{MaxOperations: 65537}); err == nil || got != (scoreclock.Clock{}) {
		t.Fatal("invalid cap accepted")
	}
}
