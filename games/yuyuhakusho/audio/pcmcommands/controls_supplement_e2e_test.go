package pcmcommands_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"io"
	"os"
	"reflect"
	"testing"

	pcmc "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
)

var supplementalVector = flag.String("native-controls-supplement-vector", "testdata/original-command-controls-supplement.json.gz", "independent reset-domain supplemental originals")

func TestIndependentResetChipTickAndCompleteDualContinuation(t *testing.T) {
	raw, e := os.ReadFile(*supplementalVector)
	if e != nil {
		t.Fatal(e)
	}
	reader, e := gzip.NewReader(bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	raw, e = io.ReadAll(reader)
	if e != nil {
		t.Fatal(e)
	}
	var body struct {
		Version int
		Cases   []controlCase
	}
	if e = json.Unmarshal(raw, &body); e != nil || body.Version != 2 || len(body.Cases) != 2 {
		t.Fatal("supplemental original matrix", e)
	}
	for _, c := range body.Cases {
		t.Run(c.Name, func(t *testing.T) {
			cfg := controlConfig(t, c)
			d, e := pcmc.New(cfg)
			if e != nil {
				t.Fatal(e)
			}
			var transactions []pcmc.Transaction
			reverse := false
			for f, expected := range c.Frames {
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
				frame, e := d.AdvanceFrame(plan)
				if e != nil {
					t.Fatal(e)
				}
				actual := make([]controlEvent, len(frame.Ledger))
				for i, event := range frame.Ledger {
					actual[i] = controlEvent{Cycle: event.Cycle, Kind: string(event.Kind), Value: event.Value, PayloadID: event.PayloadID, Offset: event.Offset, Bus: event.Bus, Register: event.Register}
					if i > 0 && event.Cycle < frame.Ledger[i-1].Cycle {
						reverse = true
					}
				}
				if !reflect.DeepEqual(actual, expected.Events) {
					for i := 0; i < len(actual) && i < len(expected.Events); i++ {
						if !reflect.DeepEqual(actual[i], expected.Events[i]) {
							t.Fatalf("supplement original frame%d event%d actual=%+v native=%+v", f, i, actual[i], expected.Events[i])
						}
					}
					t.Fatal("supplement event count")
				}
				if frame.Status != expected.Status {
					t.Fatalf("supplement status frame%d actual=%+v native=%+v", f, frame.Status, expected.Status)
				}
				transactions = append(transactions, frame.Transactions...)
			}
			if !reflect.DeepEqual(transactions, c.Transactions) {
				t.Fatal("supplement original transactions differ")
			}
			if c.Name == "dual-hard-reset-complete" && d.Status().ModeFlags != 0 {
				t.Fatal("post-reset dual did not reach original natural end")
			}
			if c.Name == "single-reset-crossing-chip-tick" && !reverse {
				t.Fatal("chip-tick raw-time reversal was hidden")
			}
			t.Logf("independent original %d reset-domain frames", len(c.Frames))
		})
	}
}
