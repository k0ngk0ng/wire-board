package chips

// YMInputCursor is the YM domain's actual accepted frontier. YMLag stores
// only the irreducible distance hidden by a later PSG cursor, not a history
// or another copy of the aggregate clock. Legacy snapshots have zero lag.
func (active *Interactive) YMInputCursor() uint64 { return active.state.Cursor - active.state.YMLag }

func setYMFrontier(state *InteractiveState, ym uint64) {
	state.Cursor, state.YMLag = ym, 0
	if state.PSG.Cursor > ym {
		state.Cursor = state.PSG.Cursor
		state.YMLag = state.Cursor - ym
	}
}

// OnLane inputs retain producer order while each physical chip remains
// monotonic. The original public methods keep their aggregate-order guard.
func (active *Interactive) WritePortOnLane(cycle uint64, port, value uint8) error {
	_, err := active.eventOnLanes(Event{Cycle: cycle, Kind: YMPortWrite, Port: port, Value: value}, true)
	return err
}
func (active *Interactive) WritePSGOnLane(cycle uint64, value uint8) error {
	_, err := active.eventOnLanes(Event{Cycle: cycle, Kind: PSGByteWrite, Value: value}, true)
	return err
}
func (active *Interactive) ReadStatusOnLane(cycle uint64, port uint8) (uint8, error) {
	return active.eventOnLanes(Event{Cycle: cycle, Kind: YMStatusRead, Port: port}, true)
}
func (active *Interactive) ResetYMOnLane(cycle uint64) error { return active.resetYM(cycle, nil, true) }
func (active *Interactive) ResetYMAfterDACOnLane(request, completedDAC uint64) error {
	return active.resetYM(request, &completedDAC, true)
}
