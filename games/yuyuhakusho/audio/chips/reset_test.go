package chips_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
)

func TestNativeOrderedYMResetAndFullState(t *testing.T) {
	raw, err := os.ReadFile("../../verification/fmpsg-reset/native.json.gz")
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
	if err = json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 7 {
		t.Fatal("reset native fixture coverage changed")
	}
	var resets, retreat194 int
	var preservedKSR bool
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			active, err := chips.NewInteractive(row.Config, row.Initial)
			if err != nil {
				t.Fatal(err)
			}
			for index, op := range row.Operations {
				before := active.Snapshot()
				switch op.Kind {
				case "write":
					err = active.WritePort(op.Cycle, op.Port, op.Value)
				case "psg":
					err = active.WritePSG(op.Cycle, op.Value)
				case "read":
					var value uint8
					value, err = active.ReadStatus(op.Cycle, op.Port)
					if value != op.Read {
						t.Fatalf("op%d reset read Go%x Native%x", index, value, op.Read)
					}
				case "reset":
					err = active.ResetYM(op.Cycle)
					resets++
				case "after-dac":
					err = active.ResetYMAfterDAC(op.Cycle, op.CompletedDAC)
					resets++
					if op.CompletedDAC-op.Cycle == 194 {
						retreat194++
					}
				case "frame":
					var frame chips.Frame
					frame, err = active.FrameBoundary()
					if err == nil {
						equalPCM(t, frame.PCM, op.PCM)
						committed, e := active.CommittedState()
						if e != nil || op.Committed == nil || !reflect.DeepEqual(committed, *op.Committed) {
							t.Fatalf("op%d committed reset state differs: %v", index, e)
						}
					}
				default:
					t.Fatal("unknown native reset input")
				}
				if err != nil {
					t.Fatalf("op%d %s: %v", index, op.Kind, err)
				}
				after := active.Snapshot()
				if op.Snapshot != nil && !reflect.DeepEqual(after, *op.Snapshot) {
					a, _ := json.Marshal(after)
					b, _ := json.Marshal(op.Snapshot)
					t.Fatalf("op%d %s state differs\nGo=%s\nNative=%s", index, op.Kind, a, b)
				}
				if op.Kind == "reset" || op.Kind == "after-dac" {
					if after.BusyUntil != 0 || after.YM.Address != before.YM.Address || after.YM.DACEnabled || after.YM.DACValue != 128 || !reflect.DeepEqual(after.PSG, before.PSG) || after.Integrator != before.Integrator {
						t.Fatal("warm reset scope differs")
					}
					for c := range after.YM.Channels {
						for o := range after.YM.Channels[c].Operators {
							if after.YM.Channels[c].Operators[o].LatchedKeyScale != 0 {
								preservedKSR = true
							}
						}
					}
				}
				if op.Kind == "after-dac" {
					if after.Cursor != before.Cursor || after.FMGenerated != before.FMGenerated || after.LastFMOutput != before.LastFMOutput || !reflect.DeepEqual(after.FIR, before.FIR) {
						t.Fatal("backdated reset rewound completed FM/FIR frontier")
					}
				}
				active, err = chips.RestoreInteractive(row.Config, after)
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	if resets < 10 || retreat194 < 2 || !preservedKSR {
		t.Fatalf("reset coverage incomplete: resets%d maxretreat%d ksr%v", resets, retreat194, preservedKSR)
	}
}

func TestResetDomainsAreBoundedAndAtomic(t *testing.T) {
	if chips.MaxResetDACBackstep != 194 {
		t.Fatal("bounded reset profile changed")
	}
	config := chips.Config{Profile: chips.ReferenceMixedHQ44100}
	initial := mixedReset()
	active, err := chips.NewInteractive(config, initial)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range [][2]uint8{{0, 0x2a}, {1, 115}, {0, 0x2b}, {1, 128}, {0, 0x2a}} {
		if err = active.WritePort(0, w[0], w[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err = active.WritePort(1202, 1, 123); err != nil {
		t.Fatal(err)
	}
	before := active.Snapshot()
	for _, bad := range []func() error{
		func() error { return active.ResetYM(1201) }, func() error { return active.ResetYMAfterDAC(1008, 1203) },
		func() error { return active.ResetYMAfterDAC(1007, 1202) }, func() error { return active.ResetYMAfterDAC(1203, 1202) },
		func() error { return active.ResetYM(chips.FrameCycles + chips.MaxInputAhead + 1) },
	} {
		if err = bad(); err == nil {
			t.Fatal("out of domain reset accepted")
		}
		if !reflect.DeepEqual(before, active.Snapshot()) {
			t.Fatal("invalid reset partially committed chip/FIR/frontier")
		}
	}
	if err = active.ResetYMAfterDAC(1008, 1202); err != nil {
		t.Fatal(err)
	}
	reset := active.Snapshot()
	if reset.Cursor != 1202 {
		t.Fatal("reset changed completed frontier")
	}
	if err = active.WritePort(1201, 0, 0x2a); err == nil {
		t.Fatal("ordinary write acquired reset backstep privilege")
	}
	if !reflect.DeepEqual(reset, active.Snapshot()) {
		t.Fatal("rejected ordinary write changed reset state")
	}
	if err = active.ResetYM(1202); err != nil {
		t.Fatal(err)
	}
	queued, err := chips.NewStream(config, initial)
	if err != nil {
		t.Fatal(err)
	}
	if err = queued.Queue([]chips.Event{{Kind: 4}}); err == nil {
		t.Fatal("legacy Queue silently expanded into reset domain")
	}
	limited, err := chips.NewInteractive(config, initial)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < chips.MaxPendingEvents; i++ {
		if err = limited.ResetYM(0); err != nil {
			t.Fatal(err)
		}
	}
	full := limited.Snapshot()
	if err = limited.ResetYM(0); err == nil {
		t.Fatal("reset bypassed input cap")
	}
	if !reflect.DeepEqual(full, limited.Snapshot()) {
		t.Fatal("capacity reset mutated state")
	}
	if _, err = limited.FrameBoundary(); err != nil {
		t.Fatal(err)
	}
	flushed := limited.Snapshot()
	if err = limited.ResetYMAfterDAC(chips.FrameCycles-1, chips.FrameCycles); err == nil {
		t.Fatal("reset reached into an already flushed frame")
	}
	if !reflect.DeepEqual(flushed, limited.Snapshot()) {
		t.Fatal("past-frame reset mutated state")
	}
	late, err := chips.NewInteractive(chips.Config{Profile: chips.ReferenceMixedHQ44100, MasterOrigin: math.MaxUint64 - 100}, initial)
	if err != nil {
		t.Fatal(err)
	}
	old := late.Snapshot()
	if err = late.ResetYM(101); err == nil {
		t.Fatal("reset origin overflow accepted")
	}
	if !reflect.DeepEqual(old, late.Snapshot()) {
		t.Fatal("overflow reset changed state")
	}
}
