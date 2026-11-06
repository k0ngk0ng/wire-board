package pcmcommands

import "errors"

// PanRequest reads the current request marker, independently of Busy. The
// caller owns the access latency; this method adds no clock or cached state.
func (d *Device) PanRequest(at uint64) (uint8, error) {
	if err := d.busTime(at); err != nil {
		return 0, err
	}
	if d.state.Held == nil {
		return 0, errors.New("PCM pan request requires the hardware bus")
	}
	return d.state.Global.StartMarker, nil
}

// WriteRestorePan changes only the canonical natural-end restore byte. It
// neither starts a sample nor changes its live routing or priority.
func (d *Device) WriteRestorePan(at uint64, value uint8) error {
	if err := d.busTime(at); err != nil {
		return err
	}
	if d.state.Held == nil {
		return errors.New("PCM restore pan requires the hardware bus")
	}
	d.state.Global.RestorePan = value
	return nil
}
