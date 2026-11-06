package mixed

// SetBankBlocked writes the actual PCM owner's bank-transfer gate. It neither
// runs the player nor changes the synthesizer's frontier.
func (r *Runtime) SetBankBlocked(at uint64, blocked bool) error {
	return r.atomic(func(next *Runtime) error { return next.pcm.SetBankBlocked(at, blocked) })
}
