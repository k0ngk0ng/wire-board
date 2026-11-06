package scoreclock

import (
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

var ErrRunningProfile = errors.New("unsupported running driver boundary")
var ErrRunningSequence = errors.New("invalid running driver sequence")

type RunningResult struct {
	Clock  Clock
	Output rootcontrol.RuleOutput
}

// AdvanceRunningTick times a complete already-dispatched running driver call.
// Queue consumption with pending inputs, dispatch/header, non-idle fade/control,
// pause and extra IRQ/world work remain explicit unsupported boundaries.
// All passed owners must be detached and committed together by the caller.
func AdvanceRunningTick(clock Clock, root *rootcontrol.State, owners rootcontrol.RuleOwners, input rootcontrol.RuleInput, c *score.Catalog, parser *score.ParserMetadata, bus Bus, chip KeyGateChip, limits Limits) (RunningResult, error) {
	if root == nil || owners.Score == nil || owners.Queue == nil || owners.AudioPause == nil || c == nil || parser == nil || root.Selected != 255 || root.Fade.Steps != 0 || root.PauseRestore != nil || *owners.AudioPause != 0 || *owners.Queue != ([4]uint8{}) || input.Idle != (rootcontrol.RuleIdleProjection{}) {
		return RunningResult{}, ErrRunningProfile
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return RunningResult{}, err
	}
	if limits.MaxOperations == 0 {
		s.limit = 65536
	}
	r := &runningTick{s: s, catalog: c, chip: chip}
	h := rootcontrol.RuleHooks{Operation: r.rootOperation, WorkingScore: r.scoreOperation, Score: score.ObservedHooks{Parser: parser, Operation: func(score.Operation) error { return nil }, ReadDAC: func(read score.FeedbackRead) (score.DACFeedback, error) {
		if r.voice == nil {
			return score.DACFeedback{}, ErrRunningSequence
		}
		return r.voice.ReadDAC(read)
	}}, QueueReplace: func(op score.Operation, apply func() error) error {
		if r.voice == nil {
			return ErrRunningSequence
		}
		return r.voice.QueueReplace(op, apply)
	}, PriorityClear: func(op score.Operation, apply func() error) error {
		if r.voice == nil {
			return ErrRunningSequence
		}
		return r.voice.PriorityClear(op, apply)
	}}
	semanticInput := input
	semanticInput.RequireCost = false
	out, err := rootcontrol.AdvanceRules(root, owners, semanticInput, h)
	if err != nil {
		return RunningResult{}, err
	}
	if !r.finished || !r.scoreClosed || r.voice != nil {
		return RunningResult{}, ErrRunningSequence
	}
	return RunningResult{Clock: s.clock, Output: out}, nil
}

type runningTick struct {
	s                           *service
	catalog                     *score.Catalog
	chip                        KeyGateChip
	rootSequence, scoreSequence uint32
	phase                       rootcontrol.RulePhase
	phaseIndex                  uint8
	pauseRead, queueConsumed    bool
	tempoEntered, tempoClosed   bool
	scoreEntered, scoreClosed   bool
	scan                        int8
	domainReady                 bool
	activeReady                 bool
	voice                       *VoiceTransfer
	endsTick, finished          bool
}

func (r *runningTick) rootOperation(op rootcontrol.RuleOperation) error {
	if op.Version != 1 || op.Sequence != r.rootSequence || r.finished {
		return ErrRunningSequence
	}
	r.rootSequence++
	if op.Kind == rootcontrol.RuleEnterPhase {
		phases := [8]rootcontrol.RulePhase{rootcontrol.RuleGate, rootcontrol.RuleControlIdle, rootcontrol.RuleTempo, rootcontrol.RuleFade, rootcontrol.RuleQueue, rootcontrol.RuleDispatch, rootcontrol.RuleVoices, rootcontrol.RuleReturn}
		if int(r.phaseIndex) >= len(phases) || op.Phase != phases[r.phaseIndex] {
			return ErrRunningSequence
		}
		r.phaseIndex++
		r.phase = op.Phase
		switch op.Phase {
		case rootcontrol.RuleGate:
			return r.s.spend(16)
		case rootcontrol.RuleControlIdle:
			if !r.pauseRead {
				return ErrRunningSequence
			}
			if err := r.s.spend(18); err != nil {
				return err
			}
			_, err := controlIdleBody(r.s, ControlWork{})
			return err
		case rootcontrol.RuleTempo:
			return r.s.spend(18)
		case rootcontrol.RuleFade:
			if !r.tempoClosed {
				return ErrRunningSequence
			}
			if err := r.s.spend(18); err != nil {
				return err
			}
			_, err := fadeIdleBody(r.s)
			return err
		case rootcontrol.RuleQueue:
			if err := r.s.spend(16); err != nil {
				return err
			}
			return r.s.branch(true, false)
		case rootcontrol.RuleDispatch:
			if !r.queueConsumed {
				return ErrRunningSequence
			}
			if err := r.s.ordinaryCosts(18, 4, 12, 16, 4); err != nil {
				return err
			}
			if err := r.s.branch(false, true); err != nil {
				return err
			}
			if err := r.s.spend(8); err != nil {
				return err
			}
			if err := r.s.branch(true, false); err != nil {
				return err
			}
			return r.s.spend(16)
		case rootcontrol.RuleVoices:
			return r.s.spend(8)
		case rootcontrol.RuleReturn:
			if r.voice != nil || !r.endsTick && r.scan != 18 {
				return ErrRunningSequence
			}
			if !r.endsTick {
				if err := r.s.ordinaryCosts(14, 16); err != nil {
					return err
				}
			}
			r.finished = true
			return nil
		}
	}
	if op.Phase != r.phase {
		return ErrRunningSequence
	}
	switch op.Kind {
	case rootcontrol.RulePauseRead:
		if r.phase != rootcontrol.RuleGate || op.ReadIndex != 1 || r.pauseRead {
			return ErrRunningSequence
		}
		r.pauseRead = true
		if err := r.s.spend(12); err != nil {
			return err
		}
		return r.s.branch(false, true)
	case rootcontrol.RuleConsumeQueue:
		if r.phase != rootcontrol.RuleQueue || r.queueConsumed {
			return ErrRunningSequence
		}
		r.queueConsumed = true
		return nil
	default:
		return ErrRunningProfile
	}
}

func (r *runningTick) scoreOperation(op score.Operation, state score.State) error {
	if op.Version != 1 || op.Sequence != r.scoreSequence || r.scoreClosed {
		return ErrRunningSequence
	}
	r.scoreSequence++
	if r.voice != nil {
		if err := r.voice.Consume(op, state); err != nil {
			return err
		}
		if heldOperation(op, score.OpLeave, score.RoutineVoice) && op.VoiceSlot == r.voice.slot {
			if _, err := r.voice.Finish(); err != nil {
				return err
			}
			r.endsTick = r.voice.EndsDriverTick()
			r.voice = nil
			r.activeReady = false
		}
		return nil
	}
	if heldOperation(op, score.OpEnter, score.RoutineTick) {
		if r.scoreEntered || op.VoiceSlot != -1 {
			return ErrRunningSequence
		}
		r.scoreEntered = true
		return nil
	}
	if heldOperation(op, score.OpLeave, score.RoutineTick) {
		if !r.finished || op.VoiceSlot != -1 {
			return ErrRunningSequence
		}
		r.scoreClosed = true
		return nil
	}
	if op.Routine == score.RoutineTempo {
		return r.tempoOperation(op, state)
	}
	if heldOperation(op, score.OpParameter, score.RoutineDispatch) && op.Branch == score.BranchNoDispatch && r.phase == rootcontrol.RuleDispatch {
		return nil
	}
	if r.phase != rootcontrol.RuleVoices || r.endsTick {
		return ErrRunningSequence
	}
	switch op.Kind {
	case score.OpDomain:
		if r.domainReady || r.activeReady {
			return ErrRunningSequence
		}
		branch := score.BranchMusic
		switch r.scan {
		case 0:
		case 10:
			branch = score.BranchPrimary
		case 16:
			branch = score.BranchSecondary
		default:
			return ErrRunningSequence
		}
		if op.Branch != branch || op.VoiceSlot != -1 || op.OtherSlot != -1 {
			return ErrRunningSequence
		}
		r.domainReady = true
		return nil
	case score.OpVoiceScan:
		if op.VoiceSlot != r.scan || op.OtherSlot != -1 || r.activeReady || r.scan >= 18 || !observedProjectionMatches(op, state.Voices[r.scan]) {
			return ErrRunningSequence
		}
		if (r.scan == 0 || r.scan == 10 || r.scan == 16) && !r.domainReady {
			return ErrRunningSequence
		}
		active := state.Voices[r.scan].Active
		branch := score.BranchInactive
		if active {
			branch = score.BranchActive
		}
		if op.Branch != branch {
			return ErrRunningSequence
		}
		if err := r.scanPrefix(r.scan, active, state.Voices[r.scan]); err != nil {
			return err
		}
		r.activeReady = active
		r.scan++
		r.domainReady = false
		return nil
	case score.OpEnter:
		if op.Routine != score.RoutineVoice || !r.activeReady || op.VoiceSlot != r.scan-1 {
			return ErrRunningSequence
		}
		voice, err := beginVoiceOnService(r.s, op, state, r.catalog, r.chip)
		if err != nil {
			return err
		}
		r.voice = voice
		return nil
	default:
		return fmt.Errorf("%w: unpriced score operation %s/%s", ErrRunningProfile, op.Kind, op.Routine)
	}
}

func (r *runningTick) tempoOperation(op score.Operation, state score.State) error {
	if r.phase != rootcontrol.RuleTempo || r.tempoClosed {
		return ErrRunningSequence
	}
	if op.Kind == score.OpEnter {
		if r.tempoEntered {
			return ErrRunningSequence
		}
		work := TempoWork{Enabled: op.Tempo.Enabled, Reload: op.Tempo.Reload, ActiveMusicMask: op.Tempo.ActiveMusicMask}
		if !validTempoWork(work) || op.Tempo.Period != state.TempoPeriod || op.Tempo.Remaining != state.TempoRemaining {
			return ErrRunningSequence
		}
		r.tempoEntered = true
		_, err := tempoBody(r.s, work)
		return err
	}
	if !r.tempoEntered {
		return ErrRunningSequence
	}
	switch op.Kind {
	case score.OpLeave:
		r.tempoClosed = true
		return nil
	case score.OpParameter:
		if op.Branch != score.BranchDisabled || state.TempoPeriod != 0 {
			return ErrRunningSequence
		}
		return nil
	case score.OpVoiceScan:
		if op.VoiceSlot < 0 || op.VoiceSlot >= 10 || op.Iterations != 10 {
			return ErrRunningSequence
		}
		return nil
	case score.OpCounter:
		if op.Counter == score.CounterTempo {
			if op.Branch == score.BranchStep && op.Before == uint16(state.TempoRemaining) && op.After == uint16(state.TempoRemaining-1) {
				return nil
			}
			if op.Branch == score.BranchReload && op.After == uint16(state.TempoPeriod) {
				return nil
			}
		} else if op.Counter == score.CounterNoteWait && op.VoiceSlot >= 0 && op.VoiceSlot < 10 && op.Branch == score.BranchReload {
			v := state.Voices[op.VoiceSlot]
			if v.Active && op.Before == uint16(v.NoteWait) && op.After == uint16(v.NoteWait+1) {
				return nil
			}
		}
	}
	return ErrRunningSequence
}

func (r *runningTick) scanPrefix(slot int8, active bool, v score.VoiceState) error {
	s := r.s
	switch slot {
	case 0:
	case 1:
		if err := s.ordinaryCosts(16, 4, 12); err != nil {
			return err
		}
	case 7, 13:
		if err := s.ordinaryCosts(14, 4, 12); err != nil {
			return err
		}
	case 10, 16:
		if err := s.ordinaryCosts(14, 16, 4, 12); err != nil {
			return err
		}
	default:
		if err := s.ordinaryCosts(10, 12); err != nil {
			return err
		}
	}
	if err := s.spend(8); err != nil {
		return err
	}
	if err := s.branch(!active, false); err != nil {
		return err
	}
	if !active {
		return nil
	}
	if slot >= 16 {
		// Active secondary data are outside the restored accepted catalog.
		return ErrRunningProfile
	}
	return s.spend(18)
}
