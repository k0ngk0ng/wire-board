package pcmcommands_test

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/dsp"
	pcmc "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"io"
	"os"
	"reflect"
	"testing"
)

type transferGateCase struct {
	controlCase
	Actions []struct{ Gate, IsGate bool }
	Blocked []bool
}

func transferGateCases(t *testing.T) []transferGateCase {
	t.Helper()
	packed, err := os.ReadFile("../../verification/world-irq-gate/portable-transfer-gate.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Version int
		Cases   []json.RawMessage
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Version != 3 || len(data.Cases) != 8 {
		t.Fatal("original gate matrix missing")
	}
	var cases []transferGateCase
	for _, raw := range data.Cases {
		var base controlCase
		var extra struct {
			Inputs []struct{ Gate, IsGate bool }
			Frames []struct{ BankBlocked bool }
		}
		if err := json.Unmarshal(raw, &base); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &extra); err != nil {
			t.Fatal(err)
		}
		c := transferGateCase{controlCase: base, Actions: extra.Inputs}
		for _, row := range extra.Frames {
			c.Blocked = append(c.Blocked, row.BankBlocked)
		}
		cases = append(cases, c)
	}
	return cases
}
func TestPCMTransferGateWholeOriginalStreams(t *testing.T) {
	frames, events := 0, 0
	for _, c := range transferGateCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			cfg := controlConfig(t, c.controlCase)
			d, err := pcmc.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			renderer, err := dsp.New(dsp.Config{Profile: dsp.ReferenceFrameDiscreteHQ44100, MasterOrigin: c.MasterOrigin}, c.InitialDSP)
			if err != nil {
				t.Fatal(err)
			}
			cursor := 0
			for f, row := range c.Frames {
				var ledger []pcmc.Event
				drain := func() { ledger = append(ledger, d.DrainLedger()...) }
				end := uint64(f+1) * pcmc.FrameCycles
				for cursor < len(c.Inputs) && c.Inputs[cursor].Begin < end {
					input := c.Inputs[cursor]
					action := c.Actions[cursor]
					tr := c.Transactions[cursor]
					cursor++
					if _, err := d.RequestBus(input.Begin, true); err != nil {
						t.Fatal(err)
					}
					drain()
					if d.Snapshot().Time != tr.Grant {
						t.Fatal("original bus grant differs", f, d.Snapshot().Time, tr.Grant)
					}
					if action.IsGate {
						if err := d.SetBankBlocked(input.End, action.Gate); err != nil {
							t.Fatal(err)
						}
					} else {
						if err := d.PublishAccepted(input); err != nil {
							t.Fatal(err)
						}
					}
					if _, err := d.RequestBus(input.End, false); err != nil {
						t.Fatal(err)
					}
					drain()
					if d.Snapshot().Time != tr.Resume {
						t.Fatal("original bus resume differs", f, d.Snapshot().Time, tr.Resume)
					}
				}
				if _, err := d.RunUntil(end); err != nil {
					t.Fatal(err)
				}
				drain()
				raw, _ := json.Marshal(d.Snapshot())
				var saved pcmc.State
				if err := json.Unmarshal(raw, &saved); err != nil {
					t.Fatal(err)
				}
				restored, err := pcmc.Restore(cfg, saved)
				if err != nil {
					t.Fatal("partial gate restore", f, err)
				}
				if !reflect.DeepEqual(restored.Snapshot(), d.Snapshot()) {
					t.Fatal("gate restore changed owner", f)
				}
				d = restored
				flushed, err := d.FlushFrame()
				if err != nil {
					t.Fatal(err)
				}
				ledger = append(ledger, flushed.Ledger...)
				got := make([]controlEvent, len(ledger))
				for i, event := range ledger {
					got[i] = controlEvent{Cycle: event.Cycle, Kind: string(event.Kind), Value: event.Value, PayloadID: event.PayloadID, Offset: event.Offset, Bus: event.Bus, Register: event.Register}
				}
				if !reflect.DeepEqual(got, row.Events) {
					for i := 0; i < len(got) && i < len(row.Events); i++ {
						if !reflect.DeepEqual(got[i], row.Events[i]) {
							t.Fatalf("frame %d event %d actual=%+v native=%+v", f, i, got[i], row.Events[i])
						}
					}
					t.Fatal("complete original ledger length differs", f, len(got), len(row.Events))
				}
				for _, event := range ledger {
					switch event.Kind {
					case pcmc.DAC:
						err = renderer.Queue([]dsp.Event{{Cycle: event.Cycle, Kind: dsp.DACData, Value: event.Value}})
					case pcmc.Pan:
						err = renderer.Queue([]dsp.Event{{Cycle: event.Cycle, Kind: dsp.DACPan, Value: event.Value & 192}})
					case pcmc.Enable:
						err = renderer.Queue([]dsp.Event{{Cycle: event.Cycle, Kind: dsp.DACEnable, Value: event.Value}})
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				pcm, err := renderer.FrameBoundary()
				if err != nil {
					t.Fatal(err)
				}
				want, err := hex.DecodeString(row.PCMHex)
				if err != nil {
					t.Fatal(err)
				}
				actual := make([]byte, len(pcm)*2)
				for i, value := range pcm {
					actual[i*2] = byte(value)
					actual[i*2+1] = byte(uint16(value) >> 8)
				}
				if !bytes.Equal(actual, want) || !reflect.DeepEqual(renderer.Snapshot(), row.DSP) || d.Snapshot().Global.BankBlocked != c.Blocked[f] {
					t.Fatal("original PCM/DSP/wait state differs", f)
				}
				frames++
				events += len(ledger)
			}
		})
	}
	if frames != 312 || events != 78546 {
		t.Fatal("original gate coverage differs", frames, events)
	}
}
func TestPCMTransferGateHeldOwnerAndBoot(t *testing.T) {
	c := transferGateCases(t)[0]
	d, err := pcmc.New(controlConfig(t, c.controlCase))
	if err != nil {
		t.Fatal(err)
	}
	initial := d.Snapshot()
	if err := d.SetBankBlocked(0, true); err == nil || !reflect.DeepEqual(d.Snapshot(), initial) {
		t.Fatal("gate changed without physical bus")
	}
	if _, err := d.RequestBus(0, true); err != nil {
		t.Fatal(err)
	}
	frontier := d.Snapshot().Time
	if err := d.SetBankBlocked(1000, true); err != nil || d.Snapshot().Time != frontier || !d.Snapshot().Global.BankBlocked {
		t.Fatal("gate write advanced background", err)
	}
	fork := d.Fork()
	if err := fork.SetBankBlocked(1000, false); err != nil || !d.Snapshot().Global.BankBlocked {
		t.Fatal("fork leaked wait owner", err)
	}
	if _, err := d.RequestBus(2000, false); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AdvanceFrame(pcmc.FramePlan{Inputs: []pcmc.Input{{Kind: pcmc.HardReset, Begin: 10000, End: 11000}}}); err != nil || d.Snapshot().Global.BankBlocked {
		t.Fatal("boot did not clear wait flag", err)
	}
}
