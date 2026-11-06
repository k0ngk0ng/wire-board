package pcmcommands

// Boot is the typed reset initialization sequence. Its fixed atomic latency
// partitions have no firmware bytes, addresses or generic instruction effects.
var bootProfile = []uint16{4, 4, 4, 10, 7, 13, 20, 20, 10, 10, 6, 10, 20, 20, 10, 10, 6, 10, 20, 20, 6, 10, 6, 4, 7, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13}

func (d *Device) controlWritten(kind Kind, value uint8, register uint16, ticks uint64) {
	d.frame.Ledger = append(d.frame.Ledger, Event{Cycle: d.state.Time, StartCycle: d.state.Time - ticks*15, Kind: kind, Value: value, Register: register, Port: 1 + 2*uint8(register>>8)})
}
func (d *Device) stepControl() bool {
	if d.state.Phase != "boot" {
		return false
	}
	s := &d.state
	g := &s.Global
	d.profile(bootProfile, func(part uint16) {
		switch part {
		case 9:
			d.addressWritten(2, 0xb6, 10)
		case 15:
			d.addressWritten(0, 0x28, 10)
		case 21:
			d.addressWritten(0, 0x2b, 10)
		case 5:
			g.Busy = 1
		case 11:
			d.written(Pan, 0, 10)
		case 17:
			d.controlWritten(YMWrite, 6, 0x28, 10)
		case 24:
			d.written(Enable, 0, 7)
		case 26:
			g.RateRequest = 0
		case 27:
			g.StartMarker = 0
		case 28:
			g.ModeFlags = 0
		case 29:
			g.OrdinaryPan = 0
		case 30:
			g.MusicPan = 0
		case 31:
			g.RestorePan = 0
		case 32:
			s.Voices[0].Busy = 0
		case 33:
			s.Voices[1].Busy = 0
		case 34:
			s.Voices[0].Flags = 0
		case 35:
			s.Voices[1].Flags = 0
		case 36:
			g.Busy = 0
		}
	}, func() { s.Work = Work{}; d.next("idle") })
	return true
}

// Address is an output at an existing semantic write boundary, not a saved
// chip latch. The chip owner alone interprets the shared address/data ports.
func (d *Device) addressWritten(port, value uint8, ticks uint64) {
	if !d.library.includeYMAddresses {
		return
	}
	d.frame.Ledger = append(d.frame.Ledger, Event{Cycle: d.state.Time, StartCycle: d.state.Time - ticks*15, Kind: YMAddress, Port: port, Value: value})
}
