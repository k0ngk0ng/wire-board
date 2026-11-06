package chips

import "errors"

// MaxResetDACBackstep is a bounded firmware-specific reset input profile. A
// completed single DAC store spans13 Z80 T-states (195 master cycles), so a
// reset requested inside that indivisible store retreats by at most194. It is
// not a maximum for arbitrary YM I/O, CPU instructions or physical hardware.
const MaxResetDACBackstep uint64 = 194

// warmYMReset implements the native chip reset domain, not cold construction
// or a whole audio reset. Register initialization leaves the shared address
// latch and the delayed keyscale circuit stage intact until the next refresh.
func warmYMReset(current YMState) YMState {
	next := ResetYMState()
	next.Address = current.Address
	for channel := range next.Channels {
		for operator := range next.Channels[channel].Operators {
			next.Channels[channel].Operators[operator].LatchedKeyScale = current.Channels[channel].Operators[operator].LatchedKeyScale
		}
	}
	return next
}

func (active *Interactive) resetYM(cycle uint64, completedDAC *uint64, independent bool) error {
	state := &active.state
	if state.InputCount >= MaxPendingEvents {
		return errors.New("reset input capacity reached before frame flush")
	}
	end, ok := plus(state.Time, FrameCycles)
	if !ok {
		return errors.New("reset frame end overflow")
	}
	if cycle < state.Time || cycle > end && cycle-end > MaxInputAhead {
		return errors.New("reset request outside current bounded frame")
	}
	if _, ok = plus(active.config.MasterOrigin, cycle); !ok {
		return errors.New("reset origin+request overflow")
	}
	frontier := state.Cursor
	if independent {
		frontier = active.YMInputCursor()
	}
	if completedDAC == nil {
		if cycle < frontier {
			return errors.New("ordinary YM reset moved input time backwards")
		}
	} else if *completedDAC != frontier || cycle > *completedDAC || *completedDAC-cycle > MaxResetDACBackstep {
		return errors.New("reset does not match bounded completed DAC frontier")
	}

	// Existing lookahead and current FIR are never rolled back. The original
	// request cycle reaches the FM synchronizer unchanged; when the chip has
	// already run beyond it, no additional samples are generated.
	next := active.state
	chip := YM{state: next.YM}
	edges, err := generateFM(active.config, &next, &chip, cycle)
	if err != nil {
		return err
	}
	plan, err := planFIR(active.config, &next, edges)
	if err != nil {
		return err
	}
	next.YM = warmYMReset(chip.state)
	next.BusyUntil = 0
	next.InputCount++
	if cycle > next.Cursor {
		next.Cursor = cycle
	}
	if independent || next.YMLag != 0 {
		ym := active.YMInputCursor()
		if cycle > ym {
			ym = cycle
		}
		setYMFrontier(&next, ym)
	}
	applyFIR(&next, plan)
	active.state = next
	return nil
}

// ResetYM applies an ordered native fm_reset at a nondecreasing raw cycle.
// It clears YM registers and busy after synchronizing FM, but preserves PSG,
// FIR/integrator, generated FM progress and already submitted output.
func (active *Interactive) ResetYM(cycle uint64) error {
	return active.resetYM(cycle, nil, false)
}

// ResetYMAfterDAC is the explicit exception for a Device reset request which
// was observed only after an actual DAC atomic store completed. The caller
// must obtain both original cycles from that causal Device transaction; it
// must not relabel arbitrary FM/PSG accesses as DAC completion. completedDAC
// must equal this instance's current frontier and request may retreat at most
// MaxResetDACBackstep. Ordinary writes, reads and queued inputs stay monotonic.
func (active *Interactive) ResetYMAfterDAC(request, completedDAC uint64) error {
	return active.resetYM(request, &completedDAC, false)
}
