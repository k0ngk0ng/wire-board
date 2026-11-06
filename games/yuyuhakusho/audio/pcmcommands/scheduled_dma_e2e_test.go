package pcmcommands_test

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/dsp"
	pcmc "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
)

type scheduledDMAEvent struct {
	controlEvent
	Port uint8
}
type scheduledDMACase struct {
	controlCase
	Actions    []struct{ Gate, IsGate bool }
	Physical   [][]scheduledDMAEvent
	Waiting    []bool
	Frontiers  []uint64
	MainDelays []uint64
	Steps      []struct {
		Kind                                     string
		Frame, Index                             int
		At, End, Frontier, FrontierBefore, Delay uint64
		Remaining                                uint32
		Waiting, WaitingBefore                   bool
		DMA                                      pcmc.DMAInterval
	}
}

func scheduledDMACases(t *testing.T) []scheduledDMACase {
	t.Helper()
	packed, err := os.ReadFile("../../verification/world-clock/portable-scheduled-dma.json.gz")
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
	var fixture struct {
		Version int
		Cases   []json.RawMessage
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 4 || len(fixture.Cases) != 10 {
		t.Fatal("full actual scheduled DMA matrix missing")
	}
	var rows []scheduledDMACase
	for _, raw := range fixture.Cases {
		var base controlCase
		var extra struct {
			Inputs       []struct{ Gate, IsGate bool }
			Transactions []struct{ MainDelay uint64 }
			Frames       []struct {
				Events     []scheduledDMAEvent
				DMAWaiting bool
				Frontier   uint64
			}
			Steps json.RawMessage
		}
		if err := json.Unmarshal(raw, &base); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &extra); err != nil {
			t.Fatal(err)
		}
		c := scheduledDMACase{controlCase: base, Actions: extra.Inputs}
		if err := json.Unmarshal(extra.Steps, &c.Steps); err != nil {
			t.Fatal(err)
		}
		for _, row := range extra.Frames {
			c.Physical = append(c.Physical, row.Events)
			c.Waiting = append(c.Waiting, row.DMAWaiting)
			c.Frontiers = append(c.Frontiers, row.Frontier)
		}
		for _, tr := range extra.Transactions {
			c.MainDelays = append(c.MainDelays, tr.MainDelay)
		}
		rows = append(rows, c)
	}
	return rows
}

func TestScheduledDMAActualWholePCMPortsDSPAndRestore(t *testing.T) {
	frames, events, latches, waitingProgress := 0, 0, 0, 0
	for _, c := range scheduledDMACases(t) {
		t.Run(c.Name, func(t *testing.T) {
			cfg := controlConfig(t, c.controlCase)
			cfg.IncludeYMAddresses = true
			d, err := pcmc.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			renderer, err := dsp.New(dsp.Config{Profile: dsp.ReferenceFrameDiscreteHQ44100, MasterOrigin: c.MasterOrigin}, c.InitialDSP)
			if err != nil {
				t.Fatal(err)
			}
			var ledger []pcmc.Event
			for index, step := range c.Steps {
				switch step.Kind {
				case "input":
					in, action, tr := c.Inputs[step.Index], c.Actions[step.Index], c.Transactions[step.Index]
					after, err := d.RequestBus(in.Begin, true)
					if err != nil || after != in.Begin+c.MainDelays[step.Index] || d.Snapshot().Time != tr.Grant {
						t.Fatal("scheduled original grant differs", index, err, after, d.Snapshot().Time, tr)
					}
					if action.IsGate {
						err = d.SetBankBlocked(in.End, action.Gate)
					} else {
						err = d.PublishAccepted(in)
					}
					if err != nil {
						t.Fatal(err)
					}
					if _, err = d.RequestBus(in.End, false); err != nil || d.Snapshot().Time != tr.Resume {
						t.Fatal("scheduled original release differs", index, err)
					}
				case "dma":
					if d.Snapshot().Time != step.FrontierBefore || d.Snapshot().WaitFlag != step.WaitingBefore {
						t.Fatal("original pre-DMA state differs", index, d.Snapshot().Time, step.FrontierBefore)
					}
					if err := d.AdvanceDMA(step.End, step.Remaining); err != nil || d.Snapshot().Time != step.Frontier || d.Snapshot().WaitFlag != step.Waiting {
						t.Fatal("original DMA continuation differs", index, err, d.Snapshot().Time, step.Frontier)
					}
					if step.WaitingBefore {
						waitingProgress++
					}
				case "run":
					result, err := d.RunDMAUntil(step.At, []pcmc.DMAInterval{step.DMA})
					if err != nil || result.Frontier != step.Frontier || result.M68KDelay != step.Delay || d.Snapshot().WaitFlag != step.Waiting {
						t.Fatal("original background DMA batch differs", index, err, result, step)
					}
					if step.Waiting && step.At+1000 <= uint64(step.Frame+1)*pcmc.FrameCycles {
						before := d.Snapshot()
						probe, err := d.RunDMAUntil(step.At+1000, []pcmc.DMAInterval{step.DMA})
						if err != nil || probe.M68KDelay != 0 || probe.EventCount != 0 || !reflect.DeepEqual(before, d.Snapshot()) {
							t.Fatal("waiting player executed before real VDP continuation", err)
						}
					}
				case "frame":
					row := c.Frames[step.Frame]
					got, err := d.FlushFrame()
					if err != nil {
						t.Fatal(err)
					}
					ledger = append(ledger, got.Ledger...)
					actual := make([]scheduledDMAEvent, len(ledger))
					for i, event := range ledger {
						actual[i] = scheduledDMAEvent{controlEvent{Cycle: event.Cycle, Kind: string(event.Kind), Value: event.Value, PayloadID: event.PayloadID, Offset: event.Offset, Bus: event.Bus, Register: event.Register}, event.Port}
						if event.Bus != nil && event.Bus.SetWaitFlag {
							latches++
						}
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
					if !reflect.DeepEqual(actual, c.Physical[step.Frame]) {
						for i := 0; i < len(actual) && i < len(c.Physical[step.Frame]); i++ {
							if !reflect.DeepEqual(actual[i], c.Physical[step.Frame][i]) {
								t.Fatal("actual full DMA ledger differs", step.Frame, i, actual[i], c.Physical[step.Frame][i])
							}
						}
						t.Fatal("actual full DMA ledger length differs", step.Frame, len(actual), len(c.Physical[step.Frame]))
					}
					pcm, err := renderer.FrameBoundary()
					if err != nil {
						t.Fatal(err)
					}
					want, err := hex.DecodeString(row.PCMHex)
					if err != nil {
						t.Fatal(err)
					}
					data := make([]byte, len(pcm)*2)
					for i, value := range pcm {
						data[i*2] = byte(value)
						data[i*2+1] = byte(uint16(value) >> 8)
					}
					if !bytes.Equal(data, want) || !reflect.DeepEqual(renderer.Snapshot(), row.DSP) || got.Status != row.Status || d.Snapshot().WaitFlag != c.Waiting[step.Frame] || d.Snapshot().Time != c.Frontiers[step.Frame] {
						t.Fatal("original PCM/DSP/player frame differs", step.Frame)
					}
					frames++
					events += len(ledger)
					ledger = nil
				default:
					t.Fatal("unknown scheduled DMA action", step.Kind)
				}
				// Restore while a real wait and undrained SetWaitFlag event can still
				// be present, as well as after every completed batch and frame.
				raw, err := json.Marshal(d.Snapshot())
				if err != nil {
					t.Fatal(err)
				}
				var checkpoint pcmc.State
				if err := json.Unmarshal(raw, &checkpoint); err != nil {
					t.Fatal(err)
				}
				restored, err := pcmc.Restore(cfg, checkpoint)
				if err != nil || !reflect.DeepEqual(restored.Snapshot(), d.Snapshot()) {
					t.Fatal("live scheduled DMA state cannot restore", index, err)
				}
				d = restored
				ledger = append(ledger, d.DrainLedger()...)
			}
		})
	}
	if frames != 546 || events != 263798 || latches != 2 || waitingProgress != 5 {
		t.Fatal("native scheduled DMA coverage differs", frames, events, latches, waitingProgress)
	}
}
