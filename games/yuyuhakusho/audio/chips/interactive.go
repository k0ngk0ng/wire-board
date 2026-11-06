package chips

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/dsp"
)

// InteractiveState is the single owner of an in-flight reference frame. FIR
// contains the current unread frame and18 extra cells; it replaces State.Tail.
// FMGenerated counts actual samples generated this frame, including discarded
// lookahead. The next FM grid point and sample count are derived, not stored.
// No original frame state, consumed input or returned read history is retained.
type InteractiveState struct {
	Time         uint64
	Cursor       uint64
	YMLag        uint64 `json:"YMLag,omitempty"`
	InputCount   uint16
	FMGenerated  uint16
	YM           YMState
	PSG          psg.State
	LastFMOutput [2]int32
	Integrator   [2]int32
	BusyUntil    uint64
	FIR          [2][]int32
}

type Interactive struct {
	config Config
	state  InteractiveState
}

func frameSamples(config Config, time uint64) (int, error) {
	absolute, ok := plus(config.MasterOrigin, time)
	if !ok {
		return 0, errors.New("interactive origin+frame start overflow")
	}
	return int((FrameCycles*rateFactor + phaseOffset(absolute)) >> 52), nil
}
func firstFMTick(config Config, time uint64) (uint64, error) {
	absolute, ok := plus(config.MasterOrigin, time)
	if !ok {
		return 0, errors.New("interactive origin+FM start overflow")
	}
	tick, ok := plus(time, (ChipPeriod-absolute%ChipPeriod)%ChipPeriod)
	if !ok {
		return 0, errors.New("interactive FM grid overflow")
	}
	return tick, nil
}
func cloneInteractive(state InteractiveState) InteractiveState {
	for side := range state.FIR {
		state.FIR[side] = append([]int32(nil), state.FIR[side]...)
	}
	return state
}

func validInteractive(config Config, state InteractiveState) error {
	if config.Profile != ReferenceMixedHQ44100 || state.Time%FrameCycles != 0 || state.InputCount > MaxPendingEvents {
		return errors.New("invalid interactive profile/frame/input count")
	}
	end, ok := plus(state.Time, FrameCycles)
	if !ok {
		return errors.New("interactive frame end overflow")
	}
	if state.Cursor < state.Time || (state.Cursor > end && state.Cursor-end > MaxInputAhead) {
		return errors.New("interactive raw cursor out of bounds")
	}
	if state.YMLag > state.Cursor-state.Time || state.YMLag != 0 && state.Cursor != state.PSG.Cursor {
		return errors.New("invalid independent YM frontier")
	}
	if _, ok = plus(config.MasterOrigin, state.Cursor); !ok {
		return errors.New("interactive origin+cursor overflow")
	}
	if err := validYM(state.YM); err != nil {
		return err
	}
	if state.PSG.Cursor < state.Time || state.PSG.Cursor > state.Cursor {
		return errors.New("interactive PSG cursor mismatch")
	}
	if _, err := psg.New(psg.IntegratedHQ150Stereo, state.PSG); err != nil {
		return err
	}
	if state.BusyUntil != 0 {
		if state.BusyUntil <= state.Time || (state.BusyUntil > end && state.BusyUntil-end > MaxInputAhead+33*BusyClockPeriod-1) {
			return errors.New("invalid interactive busy deadline")
		}
		if _, ok = plus(config.MasterOrigin, state.BusyUntil); !ok {
			return errors.New("interactive origin+busy overflow")
		}
	}
	for _, value := range state.LastFMOutput {
		if value < -60000 || value > 60000 {
			return errors.New("invalid interactive prior FM output")
		}
	}
	samples, err := frameSamples(config, state.Time)
	if err != nil {
		return err
	}
	for _, side := range state.FIR {
		if len(side) != samples+18 {
			return errors.New("invalid interactive FIR extent")
		}
	}
	first, err := firstFMTick(config, state.Time)
	if err != nil {
		return err
	}
	if state.FMGenerated != 0 {
		last, ok := plus(first, uint64(state.FMGenerated-1)*ChipPeriod)
		if !ok || last >= state.Cursor-state.YMLag {
			return errors.New("interactive FM progress exceeds actual raw input")
		}
	}
	// The resource counter also identifies a newly opened frame. All carries
	// reside in the first18 FIR cells, with no duplicated persisted tail.
	if state.InputCount == 0 {
		if state.FMGenerated != 0 || state.Cursor-state.Time > MaxInputAhead {
			return errors.New("invalid unopened interactive frame progress")
		}
		for _, side := range state.FIR {
			for _, value := range side[18:] {
				if value != 0 {
					return errors.New("unopened interactive frame has extra FIR data")
				}
			}
		}
	}
	return nil
}

func NewInteractive(config Config, committed State) (*Interactive, error) {
	if len(committed.Pending) != 0 {
		return nil, errors.New("consume queued inputs before switching to interactive mode")
	}
	if err := validBase(config, committed); err != nil {
		return nil, err
	}
	samples, err := frameSamples(config, committed.Time)
	if err != nil {
		return nil, err
	}
	cursor, ok := plus(committed.Time, uint64(committed.InputAhead))
	if !ok {
		return nil, errors.New("interactive initial raw cursor overflow")
	}
	state := InteractiveState{Time: committed.Time, Cursor: cursor, YM: committed.YM, PSG: committed.PSG, LastFMOutput: committed.LastFMOutput, Integrator: committed.Integrator, BusyUntil: committed.BusyUntil}
	for side := range state.FIR {
		state.FIR[side] = make([]int32, samples+18)
		copy(state.FIR[side], committed.Tail[side][:])
	}
	if err = validInteractive(config, state); err != nil {
		return nil, err
	}
	return &Interactive{config: config, state: state}, nil
}
func RestoreInteractive(config Config, state InteractiveState) (*Interactive, error) {
	if err := validInteractive(config, state); err != nil {
		return nil, err
	}
	return &Interactive{config: config, state: cloneInteractive(state)}, nil
}
func (active *Interactive) Snapshot() InteractiveState { return cloneInteractive(active.state) }

// CommittedState exports the old frame-boundary form only before the first
// operation of the next frame. It returns a value; no tail copy is kept inside
// the instance. In-flight FIR data cannot silently be squeezed into18 cells.
func (active *Interactive) CommittedState() (State, error) {
	state := active.state
	if state.InputCount != 0 || state.YMLag != 0 {
		return State{}, errors.New("interactive frame must be flushed before exporting committed state")
	}
	result := State{Time: state.Time, YM: state.YM, PSG: state.PSG, LastFMOutput: state.LastFMOutput, Integrator: state.Integrator, InputAhead: uint32(state.Cursor - state.Time), BusyUntil: state.BusyUntil}
	for side := range result.Tail {
		copy(result.Tail[side][:], state.FIR[side][:18])
	}
	if err := validBase(active.config, result); err != nil {
		return State{}, err
	}
	return result, nil
}

type firPlan struct {
	position, phase int
	fraction        int64
	delta           [2]int32
}

func planFIR(config Config, state *InteractiveState, edges []psg.Edge) ([]firPlan, error) {
	end, ok := plus(state.Time, FrameCycles)
	if !ok {
		return nil, errors.New("interactive FIR frame overflow")
	}
	absolute, ok := plus(config.MasterOrigin, state.Time)
	if !ok {
		return nil, errors.New("interactive FIR origin overflow")
	}
	offset := phaseOffset(absolute)
	plan := make([]firPlan, 0, len(edges))
	for _, edge := range edges {
		if edge.Cycle < state.Time || (edge.Cycle > end && edge.Cycle-end > MaxInputAhead) {
			return nil, errors.New("interactive edge outside bounded FIR frame")
		}
		fixed := uint32(((edge.Cycle-state.Time)*rateFactor + offset) >> 32)
		position := int(fixed >> 20)
		if position+16 > len(state.FIR[0]) || position+16 > len(state.FIR[1]) {
			return nil, errors.New("interactive edge exceeds FIR buffer")
		}
		plan = append(plan, firPlan{position: position, phase: int((fixed >> 15) & 31), fraction: int64(fixed & 32767), delta: edge.Delta})
	}
	return plan, nil
}

// Every potentially failing calculation finishes before this mutates the
// owned FIR. Individual deltas stay separate, preserving integer interpolation.
func applyFIR(state *InteractiveState, plan []firPlan) {
	kernel := dsp.StepKernel()
	for _, edge := range plan {
		for side := range state.FIR {
			part := int32(int64(edge.delta[side]) * edge.fraction >> 15)
			main := edge.delta[side] - part
			for tap := 0; tap < 8; tap++ {
				state.FIR[side][edge.position+tap] += int32(int64(kernel[edge.phase][tap])*int64(main) + int64(kernel[edge.phase+1][tap])*int64(part))
				state.FIR[side][edge.position+8+tap] += int32(int64(kernel[32-edge.phase][7-tap])*int64(main) + int64(kernel[31-edge.phase][7-tap])*int64(part))
			}
		}
	}
}

func generateFM(config Config, state *InteractiveState, chip *YM, to uint64) ([]psg.Edge, error) {
	first, err := firstFMTick(config, state.Time)
	if err != nil {
		return nil, err
	}
	tick, ok := plus(first, uint64(state.FMGenerated)*ChipPeriod)
	if !ok {
		return nil, errors.New("interactive next FM tick overflow")
	}
	if to <= tick {
		return nil, nil
	}
	count := (to-tick-1)/ChipPeriod + 1
	if count > MaxYMBlockSamples || uint64(state.FMGenerated)+count > 65535 {
		return nil, errors.New("interactive FM sample cap exceeded")
	}
	generated, err := chip.Generate(int(count))
	if err != nil {
		return nil, err
	}
	end, ok := plus(state.Time, FrameCycles)
	if !ok {
		return nil, errors.New("interactive FM frame overflow")
	}
	edges := make([]psg.Edge, 0, count)
	for i := uint64(0); i < count; i++ {
		if tick < end {
			current := [2]int32{generated[i*2], generated[i*2+1]}
			delta := [2]int32{current[0] - state.LastFMOutput[0], current[1] - state.LastFMOutput[1]}
			if delta != [2]int32{} {
				edges = append(edges, psg.Edge{Cycle: tick, Delta: delta})
			}
			state.LastFMOutput = current
		}
		tick, ok = plus(tick, ChipPeriod)
		if !ok {
			return nil, errors.New("interactive FM tick overflow")
		}
	}
	state.FMGenerated += uint16(count)
	return edges, nil
}

func (active *Interactive) validateEvent(event Event) error {
	return active.validateEventOnLanes(event, false)
}
func (active *Interactive) validateEventOnLanes(event Event, independent bool) error {
	state := &active.state
	if state.InputCount >= MaxPendingEvents {
		return errors.New("interactive input capacity reached before frame flush")
	}
	end, ok := plus(state.Time, FrameCycles)
	if !ok {
		return errors.New("interactive frame end overflow")
	}
	frontier := state.Cursor
	if independent {
		frontier = active.YMInputCursor()
		if event.Kind == PSGByteWrite {
			frontier = state.PSG.Cursor
		}
	}
	if event.Cycle < state.Time || event.Cycle < frontier || (event.Cycle > end && event.Cycle-end > MaxInputAhead) {
		return errors.New("interactive input time out of order/bounds")
	}
	if _, ok = plus(active.config.MasterOrigin, event.Cycle); !ok {
		return errors.New("interactive origin+input overflow")
	}
	switch event.Kind {
	case YMPortWrite:
		if event.Port > 3 {
			return errors.New("invalid interactive YM port")
		}
	case PSGByteWrite:
		if event.Port != 0 {
			return errors.New("invalid interactive PSG port")
		}
	case YMStatusRead:
		if event.Port > 3 || event.Value != 0 {
			return errors.New("invalid interactive status port/value")
		}
	default:
		return errors.New("invalid interactive event kind")
	}
	return nil
}

func (active *Interactive) event(event Event) (uint8, error) {
	return active.eventOnLanes(event, false)
}
func (active *Interactive) eventOnLanes(event Event, independent bool) (uint8, error) {
	if err := active.validateEventOnLanes(event, independent); err != nil {
		return 0, err
	}
	next := active.state // FIR slices remain owned by active until all plans pass.
	ymFrontier := active.YMInputCursor()
	chip := YM{state: next.YM}
	var edges []psg.Edge
	var err error
	value := uint8(0)
	switch event.Kind {
	case YMPortWrite:
		if event.Port&1 != 0 {
			local := event.Cycle - next.Time
			deadline := (local+BusyClockPeriod-1)/BusyClockPeriod*BusyClockPeriod + 32*BusyClockPeriod
			busy, ok := plus(next.Time, deadline)
			if !ok {
				return 0, errors.New("interactive busy deadline overflow")
			}
			if _, ok = plus(active.config.MasterOrigin, busy); !ok {
				return 0, errors.New("interactive origin+busy deadline overflow")
			}
			next.BusyUntil = busy
			edges, err = generateFM(active.config, &next, &chip, event.Cycle)
			if err != nil {
				return 0, err
			}
		}
		if err = chip.WritePort(event.Port, event.Value); err != nil {
			return 0, err
		}
	case PSGByteWrite:
		psgChip, err := psg.New(psg.IntegratedHQ150Stereo, next.PSG)
		if err != nil {
			return 0, err
		}
		edges, err = psgChip.Write(psg.Event{Cycle: event.Cycle, Data: event.Value})
		if err != nil {
			return 0, err
		}
		next.PSG = psgChip.Snapshot()
	case YMStatusRead:
		if event.Port == 0 {
			edges, err = generateFM(active.config, &next, &chip, event.Cycle)
			if err != nil {
				return 0, err
			}
			value = chip.state.Status
			if event.Cycle < next.BusyUntil {
				value |= 128
			}
		}
	}
	plan, err := planFIR(active.config, &next, edges)
	if err != nil {
		return 0, err
	}
	next.YM = chip.state
	if independent || next.YMLag != 0 {
		if event.Kind != PSGByteWrite {
			ymFrontier = event.Cycle
		}
		setYMFrontier(&next, ymFrontier)
	} else {
		next.Cursor = event.Cycle
	}
	next.InputCount++
	applyFIR(&next, plan)
	active.state = next
	return value, nil
}

func (active *Interactive) WritePort(cycle uint64, port, value uint8) error {
	_, err := active.event(Event{Cycle: cycle, Kind: YMPortWrite, Port: port, Value: value})
	return err
}
func (active *Interactive) WritePSG(cycle uint64, value uint8) error {
	_, err := active.event(Event{Cycle: cycle, Kind: PSGByteWrite, Value: value})
	return err
}
func (active *Interactive) ReadStatus(cycle uint64, port uint8) (uint8, error) {
	return active.event(Event{Cycle: cycle, Kind: YMStatusRead, Port: port})
}

// FrameBoundary is the sole reference-frame flush. It never waits for a busy
// loop to finish and never replays already returned reads. A caller may flush
// after a bounded raw overrun, then continue at the same or a later raw cycle.
func (active *Interactive) FrameBoundary() (Frame, error) {
	ymFrontier := active.YMInputCursor()
	end, ok := plus(active.state.Time, FrameCycles)
	if !ok {
		return Frame{}, errors.New("interactive frame end overflow")
	}
	if _, ok = plus(active.config.MasterOrigin, end); !ok {
		return Frame{}, errors.New("interactive absolute frame end overflow")
	}
	next := cloneInteractive(active.state) // One bounded per-frame atomic scratch.
	psgChip, err := psg.New(psg.IntegratedHQ150Stereo, next.PSG)
	if err != nil {
		return Frame{}, err
	}
	edges, err := psgChip.FrameBoundary(end)
	if err != nil {
		return Frame{}, err
	}
	chip := YM{state: next.YM}
	fmEdges, err := generateFM(active.config, &next, &chip, end)
	if err != nil {
		return Frame{}, err
	}
	edges = append(edges, fmEdges...)
	plan, err := planFIR(active.config, &next, edges)
	if err != nil {
		return Frame{}, err
	}
	applyFIR(&next, plan)
	samples := len(next.FIR[0]) - 18
	pcm := make([]int16, samples*2)
	for i := 0; i < samples; i++ {
		for side := range next.FIR {
			value := next.Integrator[side] >> 15
			next.Integrator[side] += next.FIR[side][i]
			if value > 32767 {
				value = 32767
			} else if value < -32768 {
				value = -32768
			}
			pcm[i*2+side] = int16(value)
			next.Integrator[side] -= value << 6
		}
	}
	count, err := frameSamples(active.config, end)
	if err != nil {
		return Frame{}, err
	}
	var nextFIR [2][]int32
	for side := range nextFIR {
		nextFIR[side] = make([]int32, count+18)
		copy(nextFIR[side], next.FIR[side][samples:samples+18])
	}
	next.Time = end
	next.FIR = nextFIR
	next.InputCount = 0
	next.FMGenerated = 0
	if next.Cursor < end {
		next.Cursor = end
	}
	if next.BusyUntil <= end {
		next.BusyUntil = 0
	}
	next.YM = chip.state
	next.PSG = psgChip.Snapshot()
	if ymFrontier < end {
		ymFrontier = end
	}
	setYMFrontier(&next, ymFrontier)
	if err = validInteractive(active.config, next); err != nil {
		return Frame{}, err
	}
	active.state = next
	return Frame{PCM: pcm}, nil
}
