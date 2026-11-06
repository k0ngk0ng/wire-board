package rootcontrol

import (
	"errors"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// RuleOwners refers only to caller-owned detached transactions. A failure can
// leave these detached owners changed; the caller must discard all of them.
// No World queue, pause, score, PCM, chip or clock is owned by this coordinator.
type RuleOwners struct {
	Score      *score.Driver
	Queue      *[4]uint8
	AudioPause *uint8
}

// RuleIdleProjection validates a transient profile boundary. The audited-zero
// fields are never retained in State or copied from another owner's state.
type RuleIdleProjection struct {
	ControlMode, FMDelta, PSGDelta, DuckPhase, Signal, FM3Mode uint8
	FM3Offsets                                                 [8]uint16
}

type RuleInput struct {
	Priorities  engine.SoundPriorityTable
	Idle        RuleIdleProjection
	RequireCost bool
}

type RulePhase string

const (
	RuleGate        RulePhase = "gate"
	RuleControlIdle RulePhase = "control-idle"
	RuleTempo       RulePhase = "tempo"
	RuleFade        RulePhase = "fade"
	RuleQueue       RulePhase = "queue"
	RuleDispatch    RulePhase = "dispatch"
	RuleVoices      RulePhase = "voices"
	RuleReturn      RulePhase = "return"
)

type RuleOperationKind string

const (
	RuleEnterPhase    RuleOperationKind = "enter-phase"
	RulePauseRead     RuleOperationKind = "pause-read"
	RulePauseWrite    RuleOperationKind = "pause-write"
	RulePauseCapture  RuleOperationKind = "pause-capture"
	RulePauseCommit   RuleOperationKind = "pause-payload-commit"
	RulePauseRelease  RuleOperationKind = "pause-payload-release"
	RuleFadeWrite     RuleOperationKind = "fade-write"
	RuleConsumeQueue  RuleOperationKind = "consume-queue"
	RuleSelectedWrite RuleOperationKind = "selected-write"
	RuleDispatchCode  RuleOperationKind = "dispatch-code"
	RuleBulkClear     RuleOperationKind = "rules-bulk-clear"
	RuleChipWrite     RuleOperationKind = "chip-write"
)

// RuleOperation is a temporary semantic effect, observed before its mutation.
// Bulk counts are source provenance, never a claim of per-long progress or
// pricing. A consumer needing that granularity must reject this revision.
type RuleOperation struct {
	Version                   uint16
	Sequence                  uint32
	Kind                      RuleOperationKind
	Phase                     RulePhase
	Value, ReadIndex, SoundID uint8
	FadeWrite                 FadeWrite
	Write                     score.Intent
	HeaderLongs, VoiceLongs   uint16
	MusicOnly                 bool
}

// Every hook runs synchronously at its actual stage. Operation consumes Root
// chip writes; Score.Operation consumes score chip writes. Neither set of
// writes should be replayed from the returned diagnostic score output.
// AcceptDispatch is only an explicit callable-stress extension for the proven
// idle ordinary/primary domain; it cannot authorize private 7F duck behavior.
type RuleHooks struct {
	Operation func(RuleOperation) error
	Score     score.ObservedHooks
	// WorkingScore receives the actual transaction state before this effect.
	// The detached value is an observation, never another mutable score owner.
	WorkingScore   func(score.Operation, score.State) error
	PriorityClear  func(op score.Operation, apply func() error) error
	QueueTest      func(read func() bool) (bool, error)
	QueueOperation engine.SoundQueueHook
	SelectedWrite  func(op RuleOperation, apply func() error) error
	// HeaderEffect times each actual Root-owned music reset effect once.
	HeaderEffect func(op score.Operation, apply func() error) error
	// QueueReplace may time the real canonical queue mutation. It must call
	// apply exactly once synchronously on success and never retain it.
	QueueReplace     func(op score.Operation, apply func() error) error
	CaptureLevels    func() (PauseLevels, error)
	DispatchExternal func(uint8) error
	AcceptDispatch   func(uint8) error
}

type RuleRoute uint8

const (
	RuleNormal RuleRoute = iota + 1
	RulePauseHold
	RulePauseStart
	RuleResume
)

// RuleOutput contains observations only. Score.Intents and the compatibility
// ClearPrimaryPriority flag have already been handled at their actual nodes;
// applying either again at tick end would duplicate or reorder effects.
type RuleOutput struct {
	Route RuleRoute
	Score score.Output
}

var (
	ErrRuleUnsupported     = errors.New("unsupported Root rules profile")
	ErrRuleCostUnavailable = errors.New("Root rules master-cycle cost unavailable")
)

// IsFightCandidate is the exact bounded 139-ID source-audited producer domain.
// It is immutable classification, not another mutable World request queue.
func IsFightCandidate(id uint8) bool {
	switch {
	case id == 1 || id == 4 || id == 5 || id == 0xf0:
		return true
	case id >= 6 && id <= 0x7e:
		return id != 0x12 && id != 0x1b && id != 0x38 && id != 0x73
	case id >= 0xa2 && id <= 0xa6 || id == 0xa9 || id == 0xaa:
		return true
	default:
		return musicCode(id)
	}
}

func musicCode(id uint8) bool {
	return id >= 0x82 && id <= 0x88 || id == 0x8a || id == 0x8d || id == 0x8f || id == 0x95
}
func scoreCode(id uint8) bool { return musicCode(id) || id >= 0xa0 && id <= 0xab }
func idleCallable(id uint8) bool {
	return IsFightCandidate(id) || id >= 6 && id <= 0x7e || id >= 0xa0 && id <= 0xab
}

type ruleTick struct {
	root     *State
	owners   RuleOwners
	hooks    RuleHooks
	phase    RulePhase
	sequence uint32
	tx       *score.TickTransaction
	// Only the original music-reset save/restore interval owns this latch.
	savedPriority uint8
	prioritySaved bool
}

func (r *ruleTick) emit(op RuleOperation) error {
	if r.sequence >= 1024 {
		return fmt.Errorf("%w: Root operation budget exhausted", ErrRuleUnsupported)
	}
	op.Version, op.Sequence, op.Phase = 1, r.sequence, r.phase
	r.sequence++
	if err := r.hooks.Operation(op); err != nil {
		return fmt.Errorf("Root rules %s: %w", op.Kind, err)
	}
	return nil
}
func (r *ruleTick) enter(phase RulePhase) error {
	r.phase = phase
	return r.emit(RuleOperation{Kind: RuleEnterPhase})
}
func (r *ruleTick) readPause(index uint8) (uint8, error) {
	if err := r.emit(RuleOperation{Kind: RulePauseRead, ReadIndex: index}); err != nil {
		return 0, err
	}
	return *r.owners.AudioPause, nil
}
func (r *ruleTick) writePause(value uint8) error {
	if err := r.emit(RuleOperation{Kind: RulePauseWrite, Value: value}); err != nil {
		return err
	}
	*r.owners.AudioPause = value
	return nil
}
func (r *ruleTick) selected(value uint8) error {
	if err := r.emit(RuleOperation{Kind: RuleSelectedWrite, Value: value}); err != nil {
		return err
	}
	if r.hooks.SelectedWrite == nil {
		r.root.Selected = value
		return nil
	}
	active, called, violated := true, false, false
	apply := func() error {
		if !active || called {
			violated = true
			return ErrRuleUnsupported
		}
		called = true
		r.root.Selected = value
		return nil
	}
	err := r.hooks.SelectedWrite(RuleOperation{Version: 1, Kind: RuleSelectedWrite, Phase: r.phase, Value: value}, apply)
	active = false
	if err != nil {
		return err
	}
	if !called || violated {
		return ErrRuleUnsupported
	}
	return nil
}
func (r *ruleTick) write(intent score.Intent) error {
	return r.emit(RuleOperation{Kind: RuleChipWrite, Write: intent})
}
func (r *ruleTick) levelWrites(writes [24]LevelWrite) error {
	for _, w := range writes {
		if err := r.write(score.Intent{Kind: "ym", Bank: w.Bank, Register: w.Register, Value: w.Value}); err != nil {
			return err
		}
	}
	return nil
}
func (r *ruleTick) mutePSG() error {
	for _, value := range [4]uint8{0x9f, 0xbf, 0xdf, 0xff} {
		if err := r.write(score.Intent{Kind: "psg", Value: value}); err != nil {
			return err
		}
	}
	return nil
}

func (r *ruleTick) gate() (RuleRoute, error) {
	if err := r.enter(RuleGate); err != nil {
		return 0, err
	}
	pause, err := r.readPause(1)
	if err != nil {
		return 0, err
	}
	if pause == 0 {
		if r.root.PauseRestore != nil {
			return 0, fmt.Errorf("%w: pending pause levels on normal route", ErrRuleUnsupported)
		}
		return RuleNormal, nil
	}
	pause, err = r.readPause(2)
	if err != nil {
		return 0, err
	}
	if pause == 255 {
		if r.root.PauseRestore == nil {
			return 0, fmt.Errorf("%w: hold lacks pending pause levels", ErrRuleUnsupported)
		}
		return RulePauseHold, nil
	}
	pause, err = r.readPause(3)
	if err != nil {
		return 0, err
	}
	if pause&128 != 0 {
		if r.root.PauseRestore == nil {
			return 0, fmt.Errorf("%w: resume lacks pending pause levels", ErrRuleUnsupported)
		}
		if err = r.writePause(0); err != nil {
			return 0, err
		}
		return RuleResume, nil
	}
	if r.root.PauseRestore != nil {
		return 0, fmt.Errorf("%w: new pause still owns pending levels", ErrRuleUnsupported)
	}
	if err = r.writePause(255); err != nil {
		return 0, err
	}
	return RulePauseStart, nil
}

func (r *ruleTick) pause(route RuleRoute) error {
	switch route {
	case RulePauseHold:
		return nil
	case RulePauseStart:
		if r.hooks.CaptureLevels == nil {
			return fmt.Errorf("%w: missing live FM level capture", ErrRuleUnsupported)
		}
		if err := r.emit(RuleOperation{Kind: RulePauseCapture}); err != nil {
			return err
		}
		saved, err := r.hooks.CaptureLevels()
		if err != nil {
			return err
		}
		if _, err = PauseRestoreWrites(saved); err != nil {
			return err
		}
		if err = r.levelWrites(PauseMuteWrites()); err != nil {
			return err
		}
		if err = r.mutePSG(); err != nil {
			return err
		}
		if err = r.emit(RuleOperation{Kind: RulePauseCommit}); err != nil {
			return err
		}
		r.root.PauseRestore = &saved
	case RuleResume:
		writes, err := PauseRestoreWrites(*r.root.PauseRestore)
		if err != nil {
			return err
		}
		if err = r.levelWrites(writes); err != nil {
			return err
		}
		if err = r.emit(RuleOperation{Kind: RulePauseRelease}); err != nil {
			return err
		}
		r.root.PauseRestore = nil
	default:
		return fmt.Errorf("%w: invalid suspended route", ErrRuleUnsupported)
	}
	return nil
}

// AdvanceRules performs one complete rules transaction. It deliberately
// exposes no cycle/cost result. Caller-owned detached hooks and fields are
// committed together by the outer coordinator only after success.
func AdvanceRules(root *State, owners RuleOwners, input RuleInput, hooks RuleHooks) (RuleOutput, error) {
	if input.RequireCost {
		return RuleOutput{}, ErrRuleCostUnavailable
	}
	if root == nil || owners.Score == nil || owners.Queue == nil || owners.AudioPause == nil || hooks.Operation == nil || hooks.Score.Operation == nil {
		return RuleOutput{}, fmt.Errorf("%w: missing unique owner or operation callback", ErrRuleUnsupported)
	}
	if input.Idle != (RuleIdleProjection{}) || root.ChoiceBits > 1 {
		return RuleOutput{}, ErrRuleUnsupported
	}
	if root.PauseRestore != nil {
		if _, err := PauseRestoreWrites(*root.PauseRestore); err != nil {
			return RuleOutput{}, err
		}
	}
	// Acquire the score lease before the first callback. Gate still precedes
	// every advancing phase; Begin only establishes this tick's transaction.
	r := ruleTick{root: root, owners: owners, hooks: hooks, phase: RuleGate}
	tx, err := owners.Score.BeginTickObserved(r.scoreHooks())
	if err != nil {
		return RuleOutput{}, err
	}
	r.tx = tx
	defer tx.Abort()
	route, err := r.gate()
	if err != nil {
		return RuleOutput{}, err
	}
	if route != RuleNormal {
		if err = r.pause(route); err != nil {
			return RuleOutput{}, err
		}
		if err = r.enter(RuleReturn); err != nil {
			return RuleOutput{}, err
		}
		out, err := tx.FinishSuspended()
		if err != nil {
			return RuleOutput{}, err
		}
		return RuleOutput{Route: route, Score: out}, nil
	}
	if err = r.normal(tx, input.Priorities); err != nil {
		return RuleOutput{}, err
	}
	out, err := tx.Finish()
	if err != nil {
		return RuleOutput{}, err
	}
	return RuleOutput{Route: route, Score: out}, nil
}
