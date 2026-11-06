package pcmcommands

import "errors"

// DMAWaiting reads the one hardware wait latch. The external VDP owns the
// transfer and its deadline; no duplicate DMA timer is kept in the player.
func (d *Device) DMAWaiting() bool { return d.state.WaitFlag }

// RunDMAUntil is for the real VDP batch coordinator. Its zero-or-one input is
// the latest global ROM-bus DMA state, including when the background player's
// cursor precedes the host write. It is not an input history. The unscheduled
// FramePlan and RunUntil surfaces retain their closed Remaining==0 contract.
func (d *Device) RunDMAUntil(at uint64, dma []DMAInterval) (RunResult, error) {
	if err := d.busTime(at); err != nil {
		return RunResult{}, err
	}
	if len(dma) > 1 {
		return RunResult{}, errors.New("multiple current VDP DMA states")
	}
	if len(dma) == 1 {
		v := dma[0]
		if v.Begin != 0 || v.Mode > 1 || uint64(v.End) > FrameCycles || v.Remaining > 65536 {
			return RunResult{}, errors.New("invalid current VDP ROM-bus DMA state")
		}
	}
	return d.runUntil(at, dma)
}

// AdvanceDMA applies a real VDP update before the next main/background batch.
// An already waiting player's cursor is assigned, preserving the completed ROM
// operation and its phase. Completion clears only that wait latch. No sample,
// phase, publication, frame or synthesis work is replayed here.
func (d *Device) AdvanceDMA(end uint64, remaining uint32) error {
	if err := d.busTime(end); err != nil {
		return err
	}
	if remaining > 65536 {
		return errors.New("invalid VDP DMA remainder")
	}
	if d.state.WaitFlag {
		d.state.Time = end
		if d.state.Held != nil {
			d.state.Held.Grant = end
		}
	}
	if remaining == 0 {
		d.state.WaitFlag = false
	}
	return nil
}
