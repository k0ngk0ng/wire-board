package pcmcommands

import (
	"reflect"
	"testing"
)

func singleFixture(t *testing.T) (Config, State) {
	t.Helper()
	for _, c := range nativeCases(t) {
		if c.Name == "idle-to-single" {
			cfg := nativeConfig(t, c)
			d, e := New(cfg)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = d.AdvanceFrame(FramePlan{Inputs: c.Inputs[:1]}); e != nil {
				t.Fatal(e)
			}
			return cfg, d.Snapshot()
		}
	}
	t.Fatal("single fixture absent")
	return Config{}, State{}
}
func TestRestoreRequiresLivePhaseContextsAndExactAtomicBounds(t *testing.T) {
	cfg, valid := singleFixture(t)
	checks := []struct {
		name   string
		mutate func(*State)
	}{
		{"single context absent", func(s *State) { s.Phase = "s-high"; s.Part = 0; s.Single = nil }},
		{"inactive part exceeds exact table", func(s *State) {
			s.Phase = "d-inactive"
			s.Global.ModeFlags = 1
			s.Work.PrefetchSlot = 0
			s.Work.DecodeSlot = 0
			s.Part = 12
		}},
		{"empty delay", func(s *State) { s.Phase = "s-delay-high"; s.Part = 0; s.Work.Count = 0 }},
		{"nibble before any byte", func(s *State) { s.Phase = "s-decode-low"; s.Part = 0; s.Single.Consumed = 0 }},
		{"dual mode absent", func(s *State) { s.Phase = "d-refill-read"; s.Part = 0; s.Global.ModeFlags = 0 }},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			s := clone(valid)
			c.mutate(&s)
			if d, e := Restore(cfg, s); e == nil || d != nil {
				t.Fatal("invalid live audio phase accepted")
			}
		})
	}
}
func TestOpaqueMetadataHybridPublicationIsRejectedAtomically(t *testing.T) {
	var c nativeCase
	for _, x := range nativeCases(t) {
		if x.Name == "single-to-dual" {
			c = x
		}
	}
	cfg := nativeConfig(t, c)
	d, e := New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	second := c.Inputs[1]
	second.Begin = 2000
	second.End = 4000
	before := d.Snapshot()
	if _, e = d.AdvanceFrame(FramePlan{Inputs: []Input{c.Inputs[0], second}}); e == nil || !reflect.DeepEqual(before, d.Snapshot()) {
		t.Fatal("hybrid single header/bank publication accepted or partially committed")
	}
	// Use an actual valid dual state to isolate the same-slot transfer boundary.
	for _, x := range nativeCases(t) {
		if x.Name == "dual-replace-a" {
			c = x
		}
	}
	cfg = nativeConfig(t, c)
	d, e = New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.AdvanceFrame(FramePlan{Inputs: c.Inputs[:1]}); e != nil {
		t.Fatal(e)
	}
	state := d.Snapshot()
	state.Phase = "d-transfer"
	state.Part = 13
	state.Work.PrefetchSlot = 0
	state.Work.Source = c.Inputs[0].DescriptorID
	d, e = Restore(cfg, state)
	if e != nil {
		t.Fatal(e)
	}
	before = d.Snapshot()
	second = c.Inputs[1]
	second.Begin = FrameCycles
	second.End = FrameCycles + 2000
	if _, e = d.AdvanceFrame(FramePlan{Inputs: []Input{second}}); e == nil || !reflect.DeepEqual(before, d.Snapshot()) {
		t.Fatal("hybrid same-slot metadata accepted or partially committed")
	}
}
func TestSameSlotPartialDescriptorReplacementHasActualRejection(t *testing.T) {
	var c nativeCase
	for _, x := range nativeCases(t) {
		if x.Name == "dual-replace-a" {
			c = x
		}
	}
	cfg := nativeConfig(t, c)
	d, e := New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.AdvanceFrame(FramePlan{Inputs: c.Inputs[:1]}); e != nil {
		t.Fatal(e)
	}
	state := d.Snapshot()
	state.Phase = "d-transfer"
	state.Part = 13
	state.Work.PrefetchSlot = 0
	state.Work.Source = c.Inputs[0].DescriptorID
	d, e = Restore(cfg, state)
	if e != nil {
		t.Fatal(e)
	}
	before := d.Snapshot()
	second := c.Inputs[1]
	second.Begin = FrameCycles
	second.End = FrameCycles + 2000
	if _, e = d.AdvanceFrame(FramePlan{Inputs: []Input{second}}); e == nil || !reflect.DeepEqual(before, d.Snapshot()) {
		t.Fatal("hybrid same-slot metadata accepted or partially committed")
	}
}
