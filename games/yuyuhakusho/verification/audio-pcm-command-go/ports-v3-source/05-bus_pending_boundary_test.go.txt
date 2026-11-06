package pcmcommands_test

import (
	pcmc "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"math"
	"testing"
)

func TestPendingBusEffectRejectsWrappedResumeOutsideReferenceFrame(t *testing.T) {
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
	state := d.Snapshot()
	found := false
	for i := range state.PendingLedger {
		event := &state.PendingLedger[i]
		if event.Bus != nil {
			event.Bus.At = math.MaxUint32
			event.Bus.Z80After = 44
			event.Bus.M68KDelay = ((event.Bus.At%7 + 72) / 7) * 7
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no original bank input")
	}
	if x, e := pcmc.Restore(cfg, state); e == nil || x != nil {
		t.Fatal("wrapped out-of-frame pending resume accepted")
	}
}
