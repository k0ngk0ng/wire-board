package mixed

// PanRequest reads the sole PCM owner's current request marker under the
// caller's existing bus hold. No feedback snapshot is retained here.
func (r *Runtime) PanRequest(at uint64) (uint8, error) { return r.pcm.PanRequest(at) }

func (r *Runtime) WriteRestorePan(at uint64, value uint8) error {
	return r.atomic(func(next *Runtime) error { return next.pcm.WriteRestorePan(at, value) })
}
