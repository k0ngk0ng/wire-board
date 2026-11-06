package pcmcommands

import "errors"

// MusicDescriptor returns detached immutable metadata from the existing
// library. It neither duplicates payloads nor consults a playback snapshot.
func (d *Device) MusicDescriptor(id string) (Descriptor, bool) {
	descriptor, ok := d.library.descriptors[id]
	if !ok || descriptor.Protocol != MusicProtocol {
		return Descriptor{}, false
	}
	return descriptor, true
}

// MusicFlags reads the actual mode/priority byte under the hardware hold.
// The main-clock caller accounts for access latency and masks priority bits.
func (d *Device) MusicFlags(at uint64) (uint8, error) {
	if err := d.busTime(at); err != nil {
		return 0, err
	}
	if d.state.Held == nil {
		return 0, errors.New("PCM music flags require the hardware bus")
	}
	return d.state.Global.ModeFlags, nil
}

// PublishAcceptedAt binds a held metadata group to its actual last store.
// All metadata stores must belong to this unflushed frame and current open
// hold; the original canonical publication and overlap guards remain intact.
func (d *Device) PublishAcceptedAt(at uint64, input Input) error {
	if input.End != at {
		return errors.New("PCM publication completion differs from actual access")
	}
	if err := d.busTime(input.Begin); err != nil {
		return err
	}
	if err := d.busTime(at); err != nil {
		return err
	}
	if d.state.Held == nil || !d.state.Held.Open || input.Begin < d.state.Held.Input.Begin {
		return errors.New("PCM timed publication requires its current open hardware hold")
	}
	return d.PublishAccepted(input)
}
