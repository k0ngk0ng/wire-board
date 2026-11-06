package pcmcommands

import (
	"errors"
	"math"
	"strings"
)

func (d *Device) step() {
	if d.state.Work.PrefetchSlot > 1 || d.state.Work.DecodeSlot > 1 {
		d.err = errors.New("invalid PCM slot phase")
		return
	}
	if d.stepSingle() || d.stepDual() || d.stepControl() {
		return
	}
	d.err = errors.New("unknown PCM phase")
}
func (d *Device) validateState(s State) error {
	if s.Version != 1 || s.Identity != d.library.identity {
		return errors.New("PCM checkpoint identity differs")
	}
	if s.Frame > (math.MaxUint64-d.library.origin)/FrameCycles-1 {
		return errors.New("PCM checkpoint frame overflows")
	}
	origin := d.library.origin + s.Frame*FrameCycles
	upper := uint64(math.MaxUint64)
	if origin <= math.MaxUint64-FrameCycles-1024 {
		upper = origin + FrameCycles + 1024
	}
	if s.Time < d.library.origin || s.Time > upper || s.Held == nil && s.Time < origin {
		return errors.New("PCM checkpoint frontier outside frame")
	}
	count := 0
	if ticks, ok := singleProfiles[s.Phase]; ok {
		count = len(ticks)
	}
	if ticks, ok := dualProfiles[s.Phase]; ok {
		count = len(ticks)
	}
	if s.Phase == "boot" {
		count = len(bootProfile)
	}
	if count == 0 {
		switch s.Phase {
		case "s-bank-serial":
			count = 3
		case "s-header-start", "s-header-size":
			count = 2
		case "s-preload-copy":
			count = 4
		case "s-delay-high", "s-delay-low", "s-tail-walk", "s-copy-body", "s-dummy", "d-decode-read", "d-decode-branch", "d-active-highlead", "d-active-hi", "d-end-branch", "d-end-disable", "d-refill-read", "d-refill-branch", "d-copy-data", "d-copy-pe", "d-notcopy-positive-jump":
			count = 1
		case "d-inactive":
			if s.Work.DecodeSlot == s.Work.PrefetchSlot {
				count = 12
			} else {
				count = 11
			}
		case "d-copy-shift":
			count = 2
		}
	}
	if count == 0 || int(s.Part) >= count || s.Work.PrefetchSlot > 1 || s.Work.DecodeSlot > 1 || s.Work.Count > 256 {
		return errors.New("PCM checkpoint audio phase invalid")
	}
	needsSingle := strings.HasPrefix(s.Phase, "s-")
	switch s.Phase {
	case "s-init", "s-pan-pad", "s-bank-setup", "s-bank-serial", "s-prime-start", "s-header-start", "s-preload-setup":
		needsSingle = false
	}
	if needsSingle && s.Single == nil {
		return errors.New("PCM single phase lacks stream context")
	}
	if strings.HasPrefix(s.Phase, "d-") && s.Phase != "d-start" && s.Global.ModeFlags&1 == 0 {
		return errors.New("PCM dual phase lacks mode context")
	}
	if s.Phase == "s-delay-high" || s.Phase == "s-delay-low" {
		if s.Work.Count == 0 {
			return errors.New("PCM delay has no remaining iterations")
		}
	}
	if s.Phase == "s-tail-walk" && (s.Work.Count == 0 || s.Work.Count > 9) {
		return errors.New("PCM tail walk is outside phase")
	}
	if s.Phase == "s-copy-body" && (s.Work.Count == 0 || s.Work.Count > 2) {
		return errors.New("PCM copy count is outside phase")
	}
	if s.Phase == "s-bank-serial" && s.Work.Count > 7 || s.Phase == "d-copy-shift" && s.Work.Count > 7 || s.Phase == "d-copy-data" && (s.Work.Count == 0 || s.Work.Count > 4) {
		return errors.New("PCM bank loop is outside phase")
	}
	if s.Single != nil {
		consumedRequired := s.Phase == "s-decode-low" || s.Phase == "s-delay-high" || s.Phase == "s-high" && s.Part >= 7
		if consumedRequired && s.Single.Consumed == 0 {
			return errors.New("PCM nibble has no preceding encoded byte")
		}
	}
	exists := func(id string) bool {
		if id == "" {
			return true
		}
		_, ok := d.library.descriptors[id]
		return ok
	}
	if !exists(s.Global.SingleDescriptor) || !exists(s.Work.Source) {
		return errors.New("PCM checkpoint missing descriptor")
	}
	if s.Global.Busy > 128 {
		return errors.New("PCM checkpoint control byte invalid")
	}
	if s.Single != nil {
		v := s.Single
		if v.Descriptor == "" || !exists(v.Descriptor) {
			return errors.New("PCM checkpoint single resource invalid")
		}
		n := d.length(v.Descriptor)
		maxFetched := n + 1
		if maxFetched < 128 {
			maxFetched = 128
		}
		if v.Consumed > n || v.Fetched > maxFetched || v.DummyCursor > 10*v.Consumed+4 || v.Rate >= 128 {
			return errors.New("PCM checkpoint single cursors invalid")
		}
	}
	for _, v := range s.Voices {
		if !exists(v.Descriptor) || !exists(v.Request) || v.Busy != 0 && v.Busy != 64 && v.Busy != 128 || v.Control != 0 && v.Control != 1 && v.Control != 64 {
			return errors.New("PCM checkpoint voice control invalid")
		}
		if v.Descriptor != "" {
			n := d.length(v.Descriptor)
			if v.Consumed > n || v.Fetched > n+3 || v.Produced > n {
				return errors.New("PCM checkpoint voice cursor invalid")
			}
		} else if v.Consumed != 0 || v.Fetched != 0 || v.Produced != 0 || v.Enabled {
			return errors.New("PCM checkpoint empty voice cursor invalid")
		}
	}
	if s.Held != nil {
		if e := d.validateInput(s.Held.Input); e != nil {
			return e
		}
		if (!s.Held.Open && s.Held.Input.End < origin) || s.Held.Grant < d.library.origin || s.Held.Grant != s.Time || s.Held.Grant < s.Held.Input.Begin || s.Held.Open && s.Held.Input.Kind != Hold {
			return errors.New("PCM checkpoint hold interval invalid")
		}
	}
	if e := d.validatePending(s); e != nil {
		return e
	}
	if d.err != nil {
		return d.err
	}
	return nil
}
