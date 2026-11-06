package pcmcommands

import "errors"

type ArbitrationField uint8

const (
	ArbitrationMode ArbitrationField = iota
	ArbitrationAFlags
	ArbitrationBFlags
	ArbitrationABusy
	ArbitrationBBusy
)

func (d *Device) OrdinaryDescriptor(id string) (Descriptor, bool) {
	descriptor, ok := d.library.descriptors[id]
	if !ok || descriptor.Protocol != OrdinaryProtocol {
		return Descriptor{}, false
	}
	return descriptor, true
}

// ReadArbitration exposes only the five semantic bytes owned by this player.
// The caller accounts for latency and holds the bus over its ordered reads.
func (d *Device) ReadArbitration(at uint64, field ArbitrationField) (uint8, error) {
	if err := d.busTime(at); err != nil {
		return 0, err
	}
	if d.state.Held == nil {
		return 0, errors.New("PCM arbitration read requires the hardware bus")
	}
	switch field {
	case ArbitrationMode:
		return d.state.Global.ModeFlags, nil
	case ArbitrationAFlags:
		return d.state.Voices[0].Flags, nil
	case ArbitrationBFlags:
		return d.state.Voices[1].Flags, nil
	case ArbitrationABusy:
		return d.state.Voices[0].Busy, nil
	case ArbitrationBBusy:
		return d.state.Voices[1].Busy, nil
	}
	return 0, errors.New("unknown PCM arbitration field")
}
