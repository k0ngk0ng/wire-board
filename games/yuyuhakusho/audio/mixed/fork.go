package mixed

// Fork detaches dynamic owners while borrowing their authenticated immutable
// resource library. It avoids rehashing sample payloads at every outer call.
func (r *Runtime) Fork() (*Runtime, error) { return r.fork() }
