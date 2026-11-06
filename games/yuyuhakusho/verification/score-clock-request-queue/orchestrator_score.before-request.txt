package rootcontrol

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func (r *ruleTick) scoreHooks() score.ObservedHooks {
	hooks := r.hooks.Score
	hooks.Operation = func(op score.Operation) error {
		if r.hooks.WorkingScore != nil {
			state := r.owners.Score.Snapshot()
			if r.tx != nil {
				var err error
				state, err = r.tx.Snapshot()
				if err != nil {
					return err
				}
			}
			if err := r.hooks.WorkingScore(op, state); err != nil {
				return err
			}
		}
		if r.hooks.Score.Operation != nil {
			if err := r.hooks.Score.Operation(op); err != nil {
				return err
			}
		}
		switch op.Kind {
		case score.OpPrioritySave:
			if r.prioritySaved {
				return fmt.Errorf("%w: nested music priority save", ErrRuleUnsupported)
			}
			r.savedPriority, r.prioritySaved = r.root.Priority, true
		case score.OpPriorityClear:
			if err := r.applyPriorityClear(op); err != nil {
				return err
			}
		case score.OpHeaderClear:
			if !r.prioritySaved || op.Routine != score.RoutineMusicReset || op.Iterations != 16 {
				return fmt.Errorf("%w: unknown score header clear", ErrRuleUnsupported)
			}
			if err := r.emit(RuleOperation{Kind: RuleBulkClear, HeaderLongs: 16, VoiceLongs: 120, MusicOnly: true}); err != nil {
				return err
			}
			r.clearHeader()
		case score.OpPriorityRestore:
			if !r.prioritySaved {
				return fmt.Errorf("%w: music priority restore without save", ErrRuleUnsupported)
			}
			r.root.Priority, r.prioritySaved = r.savedPriority, false
		case score.OpSelectedReset:
			if op.Routine != score.RoutineMusicReset || op.SoundID != 255 || r.prioritySaved {
				return fmt.Errorf("%w: unknown score selected reset", ErrRuleUnsupported)
			}
			r.root.Selected = 255
		case score.OpQueueReplace:
			if op.WriteCount != 1 || op.Writes[0].Kind != "queue" || op.Writes[0].Action != "replace-first" {
				return fmt.Errorf("%w: unknown score queue effect", ErrRuleUnsupported)
			}
			return r.applyQueueReplace(op)
		}
		return nil
	}
	return hooks
}

// clearHeader is wholly inside one explicit rules bulk effect. These source
// groups order the semantic assignments; no callback observes invented
// intermediate score long-store progress. Tempo and voices stay score-owned.
func (r *ruleTick) clearHeader() {
	r.root.Priority = 0
	r.root.Fade.Steps, r.root.Fade.Remaining = 0, 0
	*r.owners.AudioPause = 0
	r.root.PauseRestore = nil
	r.root.Selected = 0
	r.owners.Queue[0], r.owners.Queue[1] = 0, 0
	r.owners.Queue[2], r.owners.Queue[3] = 0, 0
	r.root.ChoiceBits = 0
}

func (r *ruleTick) fullClear(tx *score.TickTransaction) error {
	if err := r.write(score.Intent{Kind: "ym", Register: 0x27}); err != nil {
		return err
	}
	if err := r.emit(RuleOperation{Kind: RuleBulkClear, HeaderLongs: 16, VoiceLongs: 216}); err != nil {
		return err
	}
	r.clearHeader()
	if err := tx.ApplyMusicControl(score.ActionResetScore); err != nil {
		return err
	}
	if err := r.selected(255); err != nil {
		return err
	}
	for _, value := range [6]uint8{2, 6, 1, 5, 0, 4} {
		if err := r.write(score.Intent{Kind: "ym", Register: 0x28, Value: value}); err != nil {
			return err
		}
	}
	if err := r.levelWrites(PauseMuteWrites()); err != nil {
		return err
	}
	return r.mutePSG()
}

func (r *ruleTick) fade(tx *score.TickTransaction, result FadeResult) error {
	for _, w := range result.Writes[:result.WriteCount] {
		if err := r.emit(RuleOperation{Kind: RuleFadeWrite, FadeWrite: w}); err != nil {
			return err
		}
		switch w.Counter {
		case FadeRemainingCounter:
			r.root.Fade.Remaining = w.Value
		case FadeStepsCounter:
			r.root.Fade.Steps = w.Value
		default:
			return fmt.Errorf("%w: unknown fade counter", ErrRuleUnsupported)
		}
	}
	switch result.Action {
	case FadeNoAction:
		return nil
	case StopDACFlags:
		return tx.ApplyMusicControl(score.ActionStopMusicDACFlags)
	case FadeMusicStep:
		return tx.ApplyMusicControl(score.ActionFadeMusicStep)
	case RequestFullClear:
		return r.fullClear(tx)
	default:
		return fmt.Errorf("%w: unknown fade action", ErrRuleUnsupported)
	}
}

func (r *ruleTick) dispatch(tx *score.TickTransaction) error {
	id := r.root.Selected
	if id == 0 || id == 255 {
		return tx.Dispatch(0)
	}
	if !IsFightCandidate(id) {
		if !idleCallable(id) || r.hooks.AcceptDispatch == nil {
			return fmt.Errorf("%w: dispatch %02x outside fight candidates", ErrRuleUnsupported, id)
		}
		if err := r.hooks.AcceptDispatch(id); err != nil {
			return err
		}
	}
	if err := r.emit(RuleOperation{Kind: RuleDispatchCode, SoundID: id}); err != nil {
		return err
	}
	if err := r.selected(255); err != nil {
		return err
	}
	if scoreCode(id) {
		return tx.Dispatch(id)
	}
	switch id {
	case 1, 5:
		result, err := StartFade(r.root.Fade, id)
		if err != nil {
			return err
		}
		if err = r.fade(tx, result); err != nil {
			return err
		}
	case 4:
		if err := r.fullClear(tx); err != nil {
			return err
		}
	default:
		if r.hooks.DispatchExternal == nil {
			return fmt.Errorf("%w: missing external dispatch %02x", ErrRuleUnsupported, id)
		}
		if err := r.hooks.DispatchExternal(id); err != nil {
			return err
		}
	}
	return tx.Dispatch(0)
}

func (r *ruleTick) normal(tx *score.TickTransaction, priorities engine.SoundPriorityTable) error {
	if err := r.enter(RuleControlIdle); err != nil {
		return err
	}
	if err := r.enter(RuleTempo); err != nil {
		return err
	}
	if err := tx.AdvanceTempo(); err != nil {
		return err
	}
	if err := r.enter(RuleFade); err != nil {
		return err
	}
	if err := r.fade(tx, AdvanceFade(r.root.Fade)); err != nil {
		return err
	}
	if err := r.enter(RuleQueue); err != nil {
		return err
	}
	if err := r.emit(RuleOperation{Kind: RuleConsumeQueue}); err != nil {
		return err
	}
	view := engine.ConsumeSoundQueue(engine.SoundPriorityState{CurrentPriority: r.root.Priority, SelectedSound: r.root.Selected, Queue: *r.owners.Queue}, priorities)
	r.root.Priority, r.root.Selected, *r.owners.Queue = view.CurrentPriority, view.SelectedSound, view.Queue
	if err := r.enter(RuleDispatch); err != nil {
		return err
	}
	if err := r.dispatch(tx); err != nil {
		return err
	}
	if err := r.enter(RuleVoices); err != nil {
		return err
	}
	if err := tx.AdvanceVoices(); err != nil {
		return err
	}
	if r.prioritySaved {
		return fmt.Errorf("%w: unfinished music priority save", ErrRuleUnsupported)
	}
	return r.enter(RuleReturn)
}
