package pcmcommands

// Fork borrows the authenticated immutable library and detaches the canonical
// dynamic state. Per-call frame/plan/error scratch is deliberately not carried
// across the branch, just as Snapshot followed by Restore does not carry it.
func (d *Device) Fork() *Device {
	return &Device{library: d.library, state: clone(d.state)}
}
