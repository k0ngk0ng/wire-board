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

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

func parserClockUnpack(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
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
	return body
}

func TestParserClockOriginalStructuralPaths(t *testing.T) {
	var f struct {
		Version int
		Cases   []struct {
			Input struct {
				ID, Kind        string
				Work            score.ParserWork
				Origin, Refresh uint64
			}
			End   scoreclock.Clock
			Equal bool `json:"native_passive_state_equal"`
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-parser/native-parser-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 148 {
		t.Fatal("native parser profile incomplete")
	}
	for _, row := range f.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("native passive observation changed result")
			}
			clock := scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}
			var got scoreclock.Clock
			var err error
			before := row.Input.Work
			if row.Input.Kind == "backedge" {
				got, err = scoreclock.FinishParserControl(clock, scoreclock.Limits{})
			} else {
				got, err = scoreclock.PriceParserWork(clock, row.Input.Work, scoreclock.Limits{})
			}
			if err != nil || got != row.End || row.Input.Work != before {
				t.Fatalf("clock%+v native%+v err%v", got, row.End, err)
			}
		})
	}
}

func TestParserClockActualMetadataSupportedAndUnpricedFamilies(t *testing.T) {
	raw := parserClockUnpack(t, "../../data/audio-score.json.gz")
	catalog, err := score.DecodeCatalog(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	raw = parserClockUnpack(t, "../../data/audio-score-parser-work.json.gz")
	metadata, err := score.DecodeParserMetadata(catalog, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Records []struct {
			Key struct {
				ScoreID uint8 `json:"score_id"`
				VoiceID int   `json:"voice_id"`
				Section string
				Index   int `json:"event_index"`
			}
			Variants []struct {
				Entries []score.EventEntry
				Work    []score.ParserWork
			}
		}
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Records) != 39045 {
		t.Fatal("actual metadata scope differs")
	}
	supported, unpriced, variants := 0, 0, 0
	for _, record := range f.Records {
		for _, variant := range record.Variants {
			if len(variant.Entries) == 0 {
				t.Fatal("missing real logical entry")
			}
			position := score.EventPosition{Valid: true, Owner: score.Owner{ScoreID: record.Key.ScoreID, VoiceID: record.Key.VoiceID}, Cycle: record.Key.Section == "cycle", Index: record.Key.Index, Entry: variant.Entries[0]}
			work, err := metadata.Lookup(position)
			if err != nil || !reflect.DeepEqual(work, variant.Work) {
				t.Fatal("bound producer work differs", position, err)
			}
			for _, entry := range variant.Entries[1:] {
				position.Entry = entry
				again, err := metadata.Lookup(position)
				if err != nil || !reflect.DeepEqual(work, again) {
					t.Fatal("logical entry variant changed")
				}
			}
			for _, item := range work {
				clock := scoreclock.Clock{Cycle: 997, Refresh: 1024}
				got, err := scoreclock.PriceParserWork(clock, item, scoreclock.Limits{})
				switch item.Kind {
				case score.WorkTokenRead, score.WorkControlDispatch, score.WorkLoop, score.WorkJump, score.WorkCall, score.WorkReturn, score.WorkDurationScale:
					if err != nil || got.Cycle <= clock.Cycle {
						t.Fatal("actual structural Work not priced", item, err)
					}
					supported++
				default:
					if !errors.Is(err, scoreclock.ErrParserWorkUnavailable) || got != (scoreclock.Clock{}) {
						t.Fatal("unpriced live body silently accepted", item, got, err)
					}
					unpriced++
				}
			}
			variants++
		}
	}
	if variants != 39053 || supported != 63728 || unpriced != 116103 {
		t.Fatal("actual Work coverage incomplete", variants, supported, unpriced)
	}
}

func TestParserClockInvalidWorkBudgetAndOverflow(t *testing.T) {
	for _, work := range []score.ParserWork{{Kind: "unknown"}, {Kind: score.WorkTokenRead, Form: "unknown"}, {Kind: score.WorkTokenRead, Form: "control", Action: "jump"}, {Kind: score.WorkControlDispatch, Action: "unknown"}, {Kind: score.WorkJump, First: true}, {Kind: score.WorkLoop, Role: "source-address"}, {Kind: score.WorkDurationScale, Additions: 2}, {Kind: score.WorkParameterRead, Role: "instrument-selection"}, {Kind: score.WorkPitchResolve, Form: "chromatic", Family: "fm"}, {Kind: score.WorkDurationProbe, Outcome: "take"}, {Kind: score.WorkNoteCommit, Phase: "final"}} {
		before := work
		got, err := scoreclock.PriceParserWork(scoreclock.Clock{}, work, scoreclock.Limits{})
		if !errors.Is(err, scoreclock.ErrParserWorkUnavailable) || got != (scoreclock.Clock{}) || work != before {
			t.Fatal("unsupported work accepted", work, got, err)
		}
	}
	work := score.ParserWork{Kind: score.WorkLoop, First: true, Taken: true}
	if got, err := scoreclock.PriceParserWork(scoreclock.Clock{}, work, scoreclock.Limits{MaxOperations: 1}); !errors.Is(err, scoreclock.ErrBudget) || got != (scoreclock.Clock{}) {
		t.Fatal("budget not enforced", got, err)
	}
	for _, clock := range []scoreclock.Clock{{Cycle: math.MaxUint64}, {Cycle: math.MaxUint64 - 895}} {
		if got, err := scoreclock.PriceParserWork(clock, work, scoreclock.Limits{}); err == nil || got != (scoreclock.Clock{}) {
			t.Fatal("overflow accepted")
		}
	}
	if got, err := scoreclock.PriceParserWork(scoreclock.Clock{}, work, scoreclock.Limits{MaxOperations: 65537}); err == nil || got != (scoreclock.Clock{}) {
		t.Fatal("invalid operation cap accepted")
	}
	if got, err := scoreclock.FinishParserControl(scoreclock.Clock{Cycle: math.MaxUint64}, scoreclock.Limits{}); err == nil || got != (scoreclock.Clock{}) {
		t.Fatal("backedge overflow accepted")
	}
}
