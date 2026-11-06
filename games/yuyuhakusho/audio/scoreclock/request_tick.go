package scoreclock

import (
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/resourcebridge"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

type RequestResult struct {
	Clock       Clock
	Output      rootcontrol.RuleOutput
	QueueFields []QueueField
}
type DriverField struct {
	Kind  string
	At    uint64
	Value uint8
}
type DriverResult struct {
	RequestResult
	Fields []DriverField
}
type requestTick struct {
	parent         *runningTick
	root           *rootcontrol.State
	queue          *[4]uint8
	library        *resourcebridge.Library
	bus            OrdinaryBus
	qclock         *queueClock
	selected       uint8
	selectedStored bool
	allowScores    bool
	init           *dispatchInit
	control        *driverControl
}

// AdvanceRequestTick extends the same running parent with live queue fields
// and ordinary PCM dispatch. Other selected commands remain explicitly unpriced.
func AdvanceRequestTick(clock Clock, root *rootcontrol.State, owners rootcontrol.RuleOwners, input rootcontrol.RuleInput, library *resourcebridge.Library, bus OrdinaryBus, chip Chip, limits Limits) (RequestResult, error) {
	result, err := advanceRequestTick(clock, root, owners, input, library, bus, chip, limits, false)
	return result.RequestResult, err
}

// AdvanceDriverTick composes the fight driver, including catalog dispatch,
// 01/05/04, active fade and pause. World IRQ/request production stays external.
func AdvanceDriverTick(clock Clock, root *rootcontrol.State, owners rootcontrol.RuleOwners, input rootcontrol.RuleInput, library *resourcebridge.Library, bus Bus, chip Chip, limits Limits) (DriverResult, error) {
	return advanceRequestTick(clock, root, owners, input, library, bus, chip, limits, true)
}

func advanceRequestTick(clock Clock, root *rootcontrol.State, owners rootcontrol.RuleOwners, input rootcontrol.RuleInput, library *resourcebridge.Library, bus Bus, chip Chip, limits Limits, allowScores bool) (DriverResult, error) {
	if root == nil || owners.Score == nil || owners.Queue == nil || owners.AudioPause == nil || library == nil || bus == nil || chip == nil || input.Idle != (rootcontrol.RuleIdleProjection{}) {
		return DriverResult{}, ErrRunningProfile
	}
	if !allowScores && (root.Selected == 0 || root.Fade.Steps != 0 || root.PauseRestore != nil || *owners.AudioPause != 0) {
		return DriverResult{}, ErrRunningProfile
	}
	keyChip, _ := chip.(KeyGateChip)
	if keyChip == nil {
		for _, v := range owners.Score.Snapshot().Voices {
			if v.Active {
				return DriverResult{}, ErrRunningProfile
			}
		}
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return DriverResult{}, err
	}
	if limits.MaxOperations == 0 {
		s.limit = 65536
	}
	parent := &runningTick{s: s, catalog: library.Catalog(), chip: keyChip}
	ordinaryBus, _ := bus.(OrdinaryBus)
	r := &requestTick{parent: parent, root: root, queue: owners.Queue, library: library, bus: ordinaryBus, allowScores: allowScores}
	if allowScores {
		r.control = &driverControl{r: r, s: s, stage: "idle", route: rootcontrol.RuleNormal}
	}
	parent.request = r
	r.qclock = &queueClock{s: s, root: root, queue: owners.Queue}
	hooks := rootcontrol.RuleHooks{Operation: parent.rootOperation, WorkingScore: parent.scoreOperation, Score: score.ObservedHooks{Parser: library.Parser(), Operation: func(score.Operation) error { return nil }, ReadDAC: func(read score.FeedbackRead) (score.DACFeedback, error) {
		if parent.voice == nil {
			return score.DACFeedback{}, ErrRunningSequence
		}
		return parent.voice.ReadDAC(read)
	}}, HeaderEffect: func(op score.Operation, apply func() error) error {
		if r.init == nil {
			return ErrRunningSequence
		}
		return r.init.headerEffect(op, apply)
	}, QueueTest: r.queueTest, QueueOperation: r.qclock.operation, SelectedWrite: r.selectedWrite, DispatchExternal: r.dispatch, AcceptDispatch: func(code uint8) error {
		if allowScores {
			if _, ok := library.Catalog().HeaderPlan(code); ok {
				return nil
			}
		}
		if _, ok := library.OrdinaryDescriptor(code); !ok {
			return ErrRunningProfile
		}
		return nil
	}, QueueReplace: func(op score.Operation, apply func() error) error {
		if parent.voice == nil {
			return ErrRunningSequence
		}
		return parent.voice.QueueReplace(op, apply)
	}, PriorityClear: func(op score.Operation, apply func() error) error {
		if parent.voice == nil {
			if r.init != nil {
				return r.init.priorityClear(op, apply)
			}
			return ErrRunningSequence
		}
		return parent.voice.PriorityClear(op, apply)
	}}
	if r.control != nil {
		hooks.ReadPause = r.control.pauseRead
		hooks.PauseWrite = r.control.pauseWrite
		hooks.FadeWrite = r.control.fadeWrite
		hooks.BulkClear = r.control.bulk
		hooks.CaptureLevels = r.control.capture
	}
	semantic := input
	semantic.RequireCost = false
	out, err := rootcontrol.AdvanceRules(root, owners, semantic, hooks)
	if err != nil {
		return DriverResult{}, err
	}
	if !parent.finished || !parent.scoreClosed || parent.voice != nil {
		return DriverResult{}, ErrRunningSequence
	}
	result := DriverResult{RequestResult: RequestResult{Clock: s.clock, Output: out, QueueFields: r.qclock.fields}}
	if r.control != nil {
		result.Fields = append(result.Fields, r.control.fields...)
	}
	if r.init != nil {
		result.Fields = append(result.Fields, r.init.fields...)
	}
	return result, nil
}

func (r *requestTick) queueTest(read func() bool) (bool, error) {
	if read == nil {
		return false, ErrRunningSequence
	}
	present := false
	s := r.parent.s
	if err := s.cost(16, func() error { present = read(); return nil }); err != nil {
		return false, err
	}
	if err := s.branch(!present, false); err != nil {
		return false, err
	}
	if present {
		if err := s.ordinaryCosts(18, 8, 8); err != nil {
			return false, err
		}
	}
	return present, nil
}

func (r *requestTick) beginDispatch() error {
	s := r.parent.s
	if err := s.ordinaryCosts(18, 4); err != nil {
		return err
	}
	if err := s.cost(12, func() error { r.selected = r.root.Selected; return nil }); err != nil {
		return err
	}
	if r.selected == 255 {
		// The semantic no-dispatch case already retains FF; price the real same-value
		// store and fixed return without creating a second mutable selected owner.
		if err := s.ordinaryCosts(16, 4); err != nil {
			return err
		}
		if err := s.branch(false, true); err != nil {
			return err
		}
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.branch(true, false); err != nil {
			return err
		}
		return s.spend(16)
	}
	_, ordinary := r.library.OrdinaryDescriptor(r.selected)
	_, scoreHeader := r.library.Catalog().HeaderPlan(r.selected)
	control := r.control != nil && (r.selected == 1 || r.selected == 4 || r.selected == 5)
	if !ordinary && !control && !(r.allowScores && scoreHeader && r.parent.chip != nil) {
		return ErrRunningProfile
	}
	return nil
}

func (r *requestTick) selectedWrite(op rootcontrol.RuleOperation, apply func() error) error {
	if r.control != nil && r.control.stage == "clear-selected" {
		return r.control.selected(op, apply)
	}
	if r.selected == 255 || r.selectedStored || op.Kind != rootcontrol.RuleSelectedWrite || op.Value != 255 || apply == nil {
		return ErrRunningSequence
	}
	r.selectedStored = true
	s := r.parent.s
	if err := s.cost(16, apply); err != nil {
		return err
	}
	if err := s.spend(4); err != nil {
		return err
	}
	if err := s.branch(false, true); err != nil {
		return err
	}
	if err := s.spend(8); err != nil {
		return err
	}
	if err := s.branch(false, false); err != nil {
		return err
	}
	compare := func(value uint8) error {
		if err := s.spend(8); err != nil {
			return err
		}
		return s.branch(r.selected < value, true)
	}
	if err := compare(1); err != nil {
		return err
	}
	if err := compare(6); err != nil {
		return err
	}
	if r.selected < 6 {
		if r.control == nil {
			return ErrRunningProfile
		}
		return r.control.beginDispatch(r.selected)
	}
	if err := compare(128); err != nil {
		return err
	}
	if r.selected < 128 {
		if err := compare(128); err != nil {
			return err
		}
		if err := s.spend(4); err != nil {
			return err
		}
	} else {
		for _, threshold := range []uint8{129, 160, 172, 240, 241} {
			if err := compare(threshold); err != nil {
				return err
			}
			if r.selected < threshold {
				if r.allowScores && (threshold == 160 || threshold == 172) {
					plan, ok := r.library.Catalog().HeaderPlan(r.selected)
					if !ok {
						return ErrRunningProfile
					}
					r.init = &dispatchInit{r: r, s: s, plan: plan, stage: "dispatch", scan: 10}
					return nil
				}
				if threshold != 241 || r.selected != 240 {
					return ErrRunningProfile
				}
				break
			}
		}
		if err := compare(241); err != nil {
			return err
		}
		if err := s.ordinaryCosts(8, 8, 10); err != nil {
			return err
		}
	}
	return s.ordinaryCosts(8, 12, 12, 12)
}

func (r *requestTick) dispatch(code uint8) error {
	if !r.selectedStored || code != r.selected || r.bus == nil {
		return ErrRunningSequence
	}
	// A combined Root/PCM port may observe the actual choice owner at these
	// accesses. It must delegate to the same Root passed to this call.
	choice := PCMChoiceField(r.root)
	if port, ok := r.bus.(PCMChoiceField); ok {
		choice = port
	}
	transfer, err := beginOrdinaryOnService(r.parent.s, code, r.library, choice, r.bus)
	if err != nil {
		return err
	}
	if input, ok := transfer.ExpectedInput(); ok {
		if err = transfer.Apply(input); err != nil {
			return err
		}
	}
	_, err = transfer.Finish()
	return err
}
