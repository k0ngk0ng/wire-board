package pcmcommands

import "errors"

// SetBankBlocked publishes the main world's transfer gate while the actual
// hardware bus is held. It adds no time, resets no stream, and leaves each
// phase to sample the gate at its own original read boundary.
func (d *Device) SetBankBlocked(at uint64, blocked bool) error {
	if err := d.busTime(at); err != nil {
		return err
	}
	if d.state.Held == nil || !d.state.Held.Open {
		return errors.New("PCM transfer gate requires an open hardware bus")
	}
	d.state.Global.BankBlocked = blocked
	return nil
}
