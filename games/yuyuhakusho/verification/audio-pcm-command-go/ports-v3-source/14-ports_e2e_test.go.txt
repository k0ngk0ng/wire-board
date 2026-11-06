package pcmcommands_test

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"io"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	pcmc "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
)

//go:embed testdata/original-command-ports-v3.json.gz
var originalPorts []byte

type physicalEvent struct {
	controlEvent
	Port uint8
}
type physicalFrame struct {
	Events    []physicalEvent
	Status    pcmc.Status
	YMAddress uint16
}
type physicalCase struct {
	Name         string
	Payloads     map[string]string
	Descriptors  []pcmc.Descriptor
	Inputs       []pcmc.Input
	Transactions []pcmc.Transaction
	Frames       []physicalFrame
	DMA          []struct {
		Frame    uint64
		Interval pcmc.DMAInterval
	}
	SourceSHA256 string
}

func physicalCases(t *testing.T) []physicalCase {
	t.Helper()
	r, e := gzip.NewReader(bytes.NewReader(originalPorts))
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	raw, e := io.ReadAll(r)
	if e != nil {
		t.Fatal(e)
	}
	var b struct {
		Version int
		Cases   []physicalCase
	}
	if e = json.Unmarshal(raw, &b); e != nil || b.Version != 3 || len(b.Cases) != 23 {
		t.Fatal("original port matrix", e)
	}
	return b.Cases
}
func physicalConfig(t *testing.T, c physicalCase) pcmc.Config {
	t.Helper()
	cfg := controlConfig(t, controlCase{Name: c.Name, Payloads: c.Payloads, Descriptors: c.Descriptors, SourceSHA256: c.SourceSHA256})
	cfg.IncludeYMAddresses = true
	return cfg
}
func TestActualYMAddressPortAndIntermediateNativeLatch(t *testing.T) {
	count, frames := 0, 0
	for _, c := range physicalCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			cfg := physicalConfig(t, c)
			d, e := pcmc.New(cfg)
			if e != nil {
				t.Fatal(e)
			}
			ym, e := chips.NewYM(chips.ResetYMState())
			if e != nil {
				t.Fatal(e)
			}
			var transactions []pcmc.Transaction
			for f, native := range c.Frames {
				raw, e := json.Marshal(d.Snapshot())
				if e != nil {
					t.Fatal(e)
				}
				var checkpoint pcmc.State
				if e = json.Unmarshal(raw, &checkpoint); e != nil {
					t.Fatal(e)
				}
				d, e = pcmc.Restore(cfg, checkpoint)
				if e != nil {
					t.Fatal(e)
				}
				plan := pcmc.FramePlan{}
				for _, in := range c.Inputs {
					if in.Begin/pcmc.FrameCycles == uint64(f) {
						plan.Inputs = append(plan.Inputs, in)
					}
				}
				for _, v := range c.DMA {
					if v.Frame == uint64(f) {
						plan.DMA = append(plan.DMA, v.Interval)
					}
				}
				frame, e := d.AdvanceFrame(plan)
				if e != nil {
					t.Fatal(e)
				}
				actual := make([]physicalEvent, len(frame.Ledger))
				for i, event := range frame.Ledger {
					actual[i] = physicalEvent{controlEvent: controlEvent{Cycle: event.Cycle, Kind: string(event.Kind), Value: event.Value, PayloadID: event.PayloadID, Offset: event.Offset, Bus: event.Bus, Register: event.Register}, Port: event.Port}
				}
				if !reflect.DeepEqual(actual, native.Events) {
					for i := 0; i < len(actual) && i < len(native.Events); i++ {
						if !reflect.DeepEqual(actual[i], native.Events[i]) {
							t.Fatalf("original physical port frame%d event%d actual=%+v native=%+v", f, i, actual[i], native.Events[i])
						}
					}
					t.Fatalf("original physical event count frame%d actual%d native%d", f, len(actual), len(native.Events))
				}
				if frame.Status != native.Status {
					t.Fatalf("original physical status frame%d differs", f)
				}
				transactions = append(transactions, frame.Transactions...)
				for _, event := range frame.Ledger {
					switch event.Kind {
					case pcmc.YMAddress, pcmc.DAC, pcmc.Pan, pcmc.Enable, pcmc.YMWrite:
						e = ym.WritePort(event.Port, event.Value)
					case pcmc.Reset:
						// This projection checks only the actual shared address latch. Native
						// warm reset preserves it. Full reset/FIR is an independent chip proof.
						reset := chips.ResetYMState()
						reset.Address = ym.Snapshot().Address
						ym, e = chips.NewYM(reset)
					}
					if e != nil {
						t.Fatal(e)
					}
				}
				if actual := ym.Snapshot().Address; actual != native.YMAddress {
					t.Fatalf("original shared address frame%d actual=%x native=%x", f, actual, native.YMAddress)
				}
				count += len(actual)
				frames++
			}
			if !reflect.DeepEqual(transactions, c.Transactions) {
				t.Fatal("physical-view original transactions differ")
			}
		})
	}
	t.Logf("original %d physical events, %d frame address latches", count, frames)
}
func TestPhysicalOutputViewIdentityAndPendingAddressOwnership(t *testing.T) {
	var c physicalCase
	for _, x := range physicalCases(t) {
		if x.Name == "idle-to-single" {
			c = x
		}
	}
	cfg := physicalConfig(t, c)
	d, e := pcmc.New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	s := d.Snapshot()
	other := cfg
	other.IncludeYMAddresses = false
	if _, e = pcmc.Restore(other, s); e == nil {
		t.Fatal("output view changed checkpoint meaning")
	}
	if _, e = d.RequestBus(0, true); e != nil {
		t.Fatal(e)
	}
	if e = d.PublishAccepted(c.Inputs[0]); e != nil {
		t.Fatal(e)
	}
	if _, e = d.RequestBus(0, false); e != nil {
		t.Fatal(e)
	}
	if _, e = d.RunUntil(71280); e != nil {
		t.Fatal(e)
	}
	state := d.Snapshot()
	found := false
	for i := range state.PendingLedger {
		if state.PendingLedger[i].Kind == pcmc.YMAddress {
			state.PendingLedger[i].Port = 1
			found = true
			break
		}
	}
	if !found {
		t.Fatal("actual address before data was absent")
	}
	if _, e = pcmc.Restore(cfg, state); e == nil {
		t.Fatal("data port accepted for pending address")
	}
}
