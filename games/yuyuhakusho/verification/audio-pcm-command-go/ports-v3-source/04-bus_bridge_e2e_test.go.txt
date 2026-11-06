package pcmcommands_test

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"io"
	"reflect"
	"testing"

	pcmc "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
)

//go:embed testdata/original-command-bus-mailboxes.json.gz
var nativeMailboxBytes []byte

func nativeMailboxes(t *testing.T) map[string]struct {
	Values       []uint8
	SourceSHA256 string
} {
	t.Helper()
	r, e := gzip.NewReader(bytes.NewReader(nativeMailboxBytes))
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
		Cases   map[string]struct {
			Values       []uint8
			SourceSHA256 string
		}
	}
	if e = json.Unmarshal(raw, &b); e != nil || b.Version != 1 {
		t.Fatal(e)
	}
	return b.Cases
}
func TestSegmentedBusContract(t *testing.T) {
	var d *pcmc.Device
	_ = d.RunUntil
	_ = d.RequestBus
	_ = d.RequestBusWithDMA
	_ = d.BusGranted
	_ = d.Mailbox
	_ = d.PublishAccepted
	_ = d.DrainLedger
	_ = d.FlushFrame
	var s pcmc.State
	_ = s.PendingLedger
	var h pcmc.HeldWindow
	_ = h.Open
}
func TestSegmentedRealDeviceAgainstOriginalBusWindows(t *testing.T) {
	total, streams := 0, 0
	mailboxes := nativeMailboxes(t)
	for _, c := range controlCases(t) {
		reset := false
		for _, in := range c.Inputs {
			if in.Kind == pcmc.HardReset {
				reset = true
			}
		}
		if reset {
			continue
		}
		streams++
		t.Run(c.Name, func(t *testing.T) {
			cfg := controlConfig(t, c)
			d, e := pcmc.New(cfg)
			if e != nil {
				t.Fatal(e)
			}
			var held *pcmc.Input
			cursor, transaction := 0, 0
			for f, native := range c.Frames {
				var ledger []pcmc.Event
				drain := func() { ledger = append(ledger, d.DrainLedger()...) }
				var dma []pcmc.DMAInterval
				for _, v := range c.DMA {
					if v.Frame == uint64(f) {
						dma = append(dma, v.Interval)
					}
				}
				end := (uint64(f) + 1) * pcmc.FrameCycles
				closeHold := func() {
					if held.Kind != pcmc.Hold {
						if e = d.PublishAccepted(*held); e != nil {
							t.Fatal(e)
						}
					}
					after, e := d.RequestBusWithDMA(held.End, false, dma)
					if e != nil || after != held.End {
						t.Fatal("native release main cursor", e, after, held.End)
					}
					expected := c.Transactions[transaction]
					if d.Snapshot().Time != expected.Resume {
						t.Fatalf("native resume actual%d native%d", d.Snapshot().Time, expected.Resume)
					}
					transaction++
					held = nil
					drain()
				}
				if held != nil && held.End < end {
					closeHold()
				}
				for cursor < len(c.Inputs) && c.Inputs[cursor].Begin < end {
					in := c.Inputs[cursor]
					cursor++
					after, e := d.RequestBusWithDMA(in.Begin, true, dma)
					if e != nil {
						t.Fatal(e)
					}
					expected := c.Transactions[transaction]
					if d.Snapshot().Time != expected.Grant {
						t.Fatalf("native grant actual%d native%d", d.Snapshot().Time, expected.Grant)
					}
					events := d.DrainLedger()
					var steal uint64
					for _, v := range events {
						if v.Bus != nil {
							steal += uint64(v.Bus.M68KDelay)
						}
					}
					if after != in.Begin+steal {
						t.Fatalf("main cursor included old effects: after%d requested%d newsteal%d", after, in.Begin, steal)
					}
					ledger = append(ledger, events...)
					granted, e := d.BusGranted(after)
					if e != nil || !granted {
						t.Fatal("real bus ACK", e)
					}
					mailbox, e := d.Mailbox(after)
					if e != nil || mailbox != mailboxes[c.Name].Values[transaction] {
						t.Fatal("real mailbox mismatch", e)
					}
					frontier := d.Snapshot().Time
					again, e := d.RequestBus(after, true)
					if e != nil || again != after || d.Snapshot().Time != frontier || len(d.DrainLedger()) != 0 {
						t.Fatal("repeated assert advanced/refcounted")
					}
					held = &in
					if held.End < end {
						closeHold()
					}
				}
				if held == nil {
					if _, e = d.RunUntil(end, dma); e != nil {
						t.Fatal(e)
					}
				}
				drain()
				// A serialized partial/frame boundary owns the exact remaining hardware
				// level and pending work; the source plan belongs to this test environment.
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
					t.Fatal("segmented restore", f, e)
				}
				flushed, e := d.FlushFrame()
				if e != nil {
					t.Fatal("explicit flush", f, e)
				}
				ledger = append(ledger, flushed.Ledger...)
				got := make([]controlEvent, len(ledger))
				for i, v := range ledger {
					got[i] = controlEvent{Cycle: v.Cycle, Kind: string(v.Kind), Value: v.Value, PayloadID: v.PayloadID, Offset: v.Offset, Bus: v.Bus, Register: v.Register}
				}
				if !reflect.DeepEqual(got, native.Events) {
					for i := 0; i < len(got) && i < len(native.Events); i++ {
						if !reflect.DeepEqual(got[i], native.Events[i]) {
							t.Fatalf("segmented original frame%d event%d actual=%+v native=%+v", f, i, got[i], native.Events[i])
						}
					}
					t.Fatalf("segmented original count frame%d actual%d native%d", f, len(got), len(native.Events))
				}
				if flushed.Status != native.Status {
					t.Fatalf("segmented native status frame%d actual=%+v native=%+v", f, flushed.Status, native.Status)
				}
				total += len(got)
			}
			if held != nil || transaction != len(c.Transactions) {
				t.Fatal("incomplete original bus transactions")
			}
		})
	}
	if streams != 17 {
		t.Fatal("non-reset bridge matrix incomplete", streams)
	}
	t.Logf("seventeen original streams, %d real segmented events", total)
}
func TestSegmentedDrainOwnershipAndRejectedCalls(t *testing.T) {
	var c controlCase
	for _, v := range controlCases(t) {
		if v.Name == "idle-to-single" {
			c = v
		}
	}
	cfg := controlConfig(t, c)
	d, e := pcmc.New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	before := d.Snapshot()
	if e = d.PublishAccepted(c.Inputs[0]); e == nil || !reflect.DeepEqual(before, d.Snapshot()) {
		t.Fatal("unheld publication accepted")
	}
	if _, e = d.FlushFrame(); e == nil || !reflect.DeepEqual(before, d.Snapshot()) {
		t.Fatal("early flush mutated")
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
	if _, e = d.RunUntil(70000); e != nil {
		t.Fatal(e)
	}
	s := d.Snapshot()
	if len(s.PendingLedger) == 0 {
		t.Fatal("real outputs disappeared")
	}
	s.PendingLedger[0].Value ^= 255
	if reflect.DeepEqual(s, d.Snapshot()) {
		t.Fatal("pending ledger aliases snapshot")
	}
	events := d.DrainLedger()
	if len(events) == 0 || len(d.DrainLedger()) != 0 || len(d.Snapshot().PendingLedger) != 0 {
		t.Fatal("drain did not transfer and clear")
	}
	frontier := d.Snapshot().Time
	at := frontier - 1
	after, e := d.RequestBus(at, true)
	if e != nil || after != at || d.Snapshot().Time != frontier {
		t.Fatal("atomic overrun was refused or replayed")
	}
	if _, e = d.RequestBus(at+100, false); e != nil {
		t.Fatal(e)
	}
	frontier = d.Snapshot().Time
	if _, e = d.RequestBus(at+200, false); e != nil || d.Snapshot().Time != frontier {
		t.Fatal("unheld release advanced")
	}
}
