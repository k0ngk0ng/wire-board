package chips_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
)

func TestNativeIndependentChipLanesFullStateAndPCM(t *testing.T) {
	raw, err := os.ReadFile("../../verification/fmpsg-independent-lanes/native.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err = io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Cases []struct {
			Name       string
			Config     chips.Config
			Initial    chips.State
			Operations []struct {
				Kind                string
				Cycle, CompletedDAC uint64
				Port, Value, Read   uint8
				Snapshot            *chips.InteractiveState
				Committed           *chips.State
				PCM                 string
			}
		}
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) != 8 {
		t.Fatal("independent native scope missing")
	}
	var backwardsAcrossDomains, frames int
	for _, row := range f.Cases {
		t.Run(row.Name, func(t *testing.T) {
			active, err := chips.NewInteractive(row.Config, row.Initial)
			if err != nil {
				t.Fatal(err)
			}
			for index, op := range row.Operations {
				before := active.Snapshot()
				if op.Kind != "frame" && op.Cycle < before.Cursor {
					backwardsAcrossDomains++
				}
				switch op.Kind {
				case "write":
					err = active.WritePortOnLane(op.Cycle, op.Port, op.Value)
				case "psg":
					err = active.WritePSGOnLane(op.Cycle, op.Value)
				case "read":
					var value uint8
					value, err = active.ReadStatusOnLane(op.Cycle, op.Port)
					if value != op.Read {
						t.Fatal("independent native read differs", index, value, op.Read)
					}
				case "reset":
					err = active.ResetYMOnLane(op.Cycle)
				case "after-dac":
					err = active.ResetYMAfterDACOnLane(op.Cycle, op.CompletedDAC)
				case "frame":
					var frame chips.Frame
					frame, err = active.FrameBoundary()
					if err == nil {
						equalPCM(t, frame.PCM, op.PCM)
						committed, e := active.CommittedState()
						if op.Committed == nil {
							if e == nil {
								t.Fatal("legacy boundary discarded independent frontier")
							}
						} else if e != nil || !reflect.DeepEqual(committed, *op.Committed) {
							t.Fatal("independent boundary differs", index, e)
						}
						frames++
					}
				default:
					t.Fatal("unknown independent native operation")
				}
				if err != nil {
					t.Fatalf("op%d %s: %v", index, op.Kind, err)
				}
				after := active.Snapshot()
				if op.Snapshot == nil || !reflect.DeepEqual(after, *op.Snapshot) {
					a, _ := json.Marshal(after)
					b, _ := json.Marshal(op.Snapshot)
					t.Fatalf("op%d %s full state differs\nGo=%s\nNative=%s", index, op.Kind, a, b)
				}
				if active.YMInputCursor() != after.Cursor-after.YMLag {
					t.Fatal("YM frontier is not canonical relative lag")
				}
				encoded, err := json.Marshal(after)
				if err != nil {
					t.Fatal(err)
				}
				var restored chips.InteractiveState
				if err = json.Unmarshal(encoded, &restored); err != nil {
					t.Fatal(err)
				}
				active, err = chips.RestoreInteractive(row.Config, restored)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	if backwardsAcrossDomains < 10 || frames != 15 {
		t.Fatal("cross-domain/reset/frame scope missing", backwardsAcrossDomains, frames)
	}
}

func TestIndependentLanesKeepSameDomainAndLegacyGuardsAtomic(t *testing.T) {
	cfg := chips.Config{Profile: chips.ReferenceMixedHQ44100}
	active, err := chips.NewInteractive(cfg, mixedReset())
	if err != nil {
		t.Fatal(err)
	}
	if err = active.WritePortOnLane(1000, 0, 0x28); err != nil {
		t.Fatal(err)
	}
	if err = active.WritePSGOnLane(5000, 0x9f); err != nil {
		t.Fatal(err)
	}
	before := active.Snapshot()
	if before.Cursor != 5000 || before.YMLag != 4000 || active.YMInputCursor() != 1000 {
		t.Fatal("lost independent YM frontier")
	}
	for _, bad := range []func() error{
		func() error { return active.WritePort(4999, 0, 0x28) }, func() error { _, e := active.ReadStatus(4999, 0); return e }, func() error { return active.ResetYM(4999) },
		func() error { return active.WritePortOnLane(999, 0, 0x28) }, func() error { _, e := active.ReadStatusOnLane(999, 0); return e }, func() error { return active.WritePSGOnLane(4999, 0x9f) }, func() error { return active.ResetYMOnLane(999) },
		func() error { return active.ResetYMAfterDACOnLane(1000, 1001) }, func() error { return active.ResetYMAfterDACOnLane(805, 1000) }, func() error { return active.WritePortOnLane(chips.FrameCycles+chips.MaxInputAhead+1, 0, 0x28) },
	} {
		if err = bad(); err == nil {
			t.Fatal("same-domain/legacy invalid input accepted")
		}
		if !reflect.DeepEqual(active.Snapshot(), before) {
			t.Fatal("failed lane input mutated authoritative state")
		}
	}
	for _, mode := range []string{"underflow", "aggregate", "rendered"} {
		bad := active.Snapshot()
		switch mode {
		case "underflow":
			bad.YMLag = bad.Cursor + 1
		case "aggregate":
			bad.Cursor++
		case "rendered":
			bad.FMGenerated = 2
		}
		if _, err = chips.RestoreInteractive(cfg, bad); err == nil {
			t.Fatal("invalid independent checkpoint accepted", mode)
		}
	}
	copy := active.Snapshot()
	copy.FIR[0][0]++
	copy.YMLag = 0
	if !reflect.DeepEqual(active.Snapshot(), before) {
		t.Fatal("snapshot aliases independent owner")
	}
}
