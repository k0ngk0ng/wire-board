package score

import "fmt"

// OperationKind is a closed semantic vocabulary, not source instructions.
type OperationKind string

const (
	OpEnter               OperationKind = "enter"
	OpLeave               OperationKind = "leave"
	OpDomain              OperationKind = "domain"
	OpVoiceScan           OperationKind = "voice-scan"
	OpEvent               OperationKind = "event"
	OpCounter             OperationKind = "counter"
	OpParameter           OperationKind = "parameter"
	OpNoteCleanup         OperationKind = "note-cleanup"
	OpClaim               OperationKind = "claim"
	OpRestore             OperationKind = "restore-forced-rest"
	OpPriorityClear       OperationKind = "priority-clear"
	OpPrioritySave        OperationKind = "priority-save"
	OpPriorityRestore     OperationKind = "priority-restore"
	OpQueueReplace        OperationKind = "queue-replace"
	OpChipWrite           OperationKind = "chip-write"
	OpPSGPitchWrite       OperationKind = "psg-pitch-write"
	OpFeedbackRead        OperationKind = "feedback-read"
	OpFeedbackResult      OperationKind = "feedback-result"
	OpParserUnavailable   OperationKind = "parser-unavailable"
	OpParserWork          OperationKind = "parser-work"
	OpResourceUnavailable OperationKind = "resource-work-unavailable"
	OpHeaderClear         OperationKind = "header-clear"
	OpSelectedReset       OperationKind = "selected-reset"
	OpScoreReset          OperationKind = "score-reset"
)

type Routine string

const (
	RoutineTick          Routine = "tick"
	RoutineTempo         Routine = "tempo"
	RoutineDispatch      Routine = "dispatch"
	RoutineStopPrimary   Routine = "stop-primary"
	RoutineStopSecondary Routine = "stop-secondary"
	RoutineMusicReset    Routine = "music-reset"
	RoutineInitialize    Routine = "initialize"
	RoutineVoice         Routine = "voice"
	RoutineParser        Routine = "parser"
	RoutineControl       Routine = "control"
	RoutineNote          Routine = "note"
	RoutineGate          Routine = "gate"
	RoutineKey           Routine = "key"
	RoutineModulation    Routine = "modulation"
	RoutinePitch         Routine = "pitch"
	RoutineVolume        Routine = "volume"
	RoutinePan           Routine = "pan"
	RoutinePatch         Routine = "patch"
	RoutineLevel         Routine = "level"
	RoutineOwnership     Routine = "ownership"
	RoutineDAC           Routine = "dac"
	RoutineMusicControl  Routine = "music-control"
)

type Branch string

const (
	BranchNormal            Branch = "normal"
	BranchMusic             Branch = "music"
	BranchPrimary           Branch = "primary"
	BranchSecondary         Branch = "secondary"
	BranchNoDispatch        Branch = "no-dispatch"
	BranchActive            Branch = "active"
	BranchInactive          Branch = "inactive"
	BranchDisabled          Branch = "disabled"
	BranchUnchanged         Branch = "unchanged"
	BranchReload            Branch = "reload"
	BranchWaiting           Branch = "waiting"
	BranchNoteBoundary      Branch = "note-boundary"
	BranchHeldNote          Branch = "held-note"
	BranchGateExpired       Branch = "gate-expired"
	BranchTied              Branch = "tied"
	BranchClaimed           Branch = "claimed"
	BranchNoteOff           Branch = "note-off"
	BranchRest              Branch = "rest"
	BranchPrevious          Branch = "previous"
	BranchChromatic         Branch = "chromatic"
	BranchDACNote           Branch = "dac-note"
	BranchEarlyCleanup      Branch = "early-cleanup"
	BranchFinalCleanup      Branch = "final-cleanup"
	BranchDelay             Branch = "delay"
	BranchRate              Branch = "rate"
	BranchReverse           Branch = "reverse"
	BranchStep              Branch = "step"
	BranchHold              Branch = "hold"
	BranchKeyOff            Branch = "key-off"
	BranchKeyOn             Branch = "key-on"
	BranchAccepted          Branch = "accepted"
	BranchRejected          Branch = "rejected"
	BranchBlocked           Branch = "blocked"
	BranchRedirect          Branch = "redirect"
	BranchYM                Branch = "ym"
	BranchStopPrimary       Branch = "stop-primary"
	BranchMusicReset        Branch = "music-reset"
	BranchPrimaryTrackStop  Branch = "primary-track-stop"
	BranchDACTrackStop      Branch = "dac-track-stop"
	BranchTrackStop         Branch = "track-stop"
	BranchInitialLoad       Branch = "initial-load"
	BranchOrdinaryLevel     Branch = "ordinary-level"
	BranchPatchSelection    Branch = "patch-selection"
	BranchEnvelope          Branch = "envelope"
	BranchPanAutomation     Branch = "pan-automation"
	BranchManualPan         Branch = "manual-pan"
	BranchStopMusicDACFlags Branch = "stop-music-dac-flags"
	BranchFadeMusicStep     Branch = "fade-music-step"
	BranchResetScore        Branch = "reset-score"
	BranchSuspended         Branch = "suspended"
)

type CounterKind string

const (
	CounterNone       CounterKind = ""
	CounterTempo      CounterKind = "tempo-remaining"
	CounterNoteWait   CounterKind = "note-wait"
	CounterGate       CounterKind = "gate-remaining"
	CounterModDelay   CounterKind = "modulation-delay"
	CounterModRate    CounterKind = "modulation-rate"
	CounterModSteps   CounterKind = "modulation-steps"
	CounterModOffset  CounterKind = "modulation-offset"
	CounterPanStep    CounterKind = "pan-step"
	CounterPanElapsed CounterKind = "pan-elapsed"
	CounterBaseLevel  CounterKind = "base-level"
)

type EventEntry string

const (
	EntryContinue   EventEntry = "continue"
	EntryFromPrefix EventEntry = "from-prefix"
	EntryCycleWrap  EventEntry = "cycle-wrap"
)

// EventPosition addresses immutable logical data. Entry is derived at the
// transition, never stored as another cycle counter or source return stack.
type EventPosition struct {
	Valid bool
	Owner Owner
	Cycle bool
	Index int
	Entry EventEntry
}

type VoiceFlags struct {
	Active, NoteOff, Claimed, Tie, ModulationEnabled bool
}

type TempoObservation struct {
	Enabled, Reload   bool
	ActiveMusicMask   uint16
	Period, Remaining uint8
}

// Operation is one temporary semantic stage, before its effect. Writes form a
// single ordered group (at most a PSG pitch pair). It contains no time or PC.
// Counter Before/After are values of the named semantic counter, not registers.
type Operation struct {
	Version       uint16
	Sequence      uint32
	Kind          OperationKind
	Routine       Routine
	Branch        Branch
	VoiceSlot     int8
	OtherSlot     int8
	Position      EventPosition
	IntentOffset  int
	Writes        [2]Intent
	WriteCount    uint8
	Counter       CounterKind
	Before, After uint16
	Iterations    uint16
	EventKind     string
	SoundID       uint8
	Channel       string
	Flags         VoiceFlags
	Transpose     int8
	BaseLevel     uint8
	BasePitch     uint16
	GateTicks     uint8
	PitchIndex    uint8
	Duration      uint16
	DefinitionID  string
	Read          FeedbackRead
	Tempo         TempoObservation
	Work          ParserWork
	WorkSite      ParserWorkSite
	Action        MusicControlAction
}

type FeedbackKind string

const (
	ReadBlockedPriority FeedbackKind = "blocked-priority"
	ReadPanRedirect     FeedbackKind = "pan-redirect"
)

// FeedbackRead is issued at its actual semantic stage. The blocked-priority
// provider may skip the priority read when Blocked is true, as the source does.
type FeedbackRead struct {
	Kind               FeedbackKind
	VoiceSlot          int8
	Position           EventPosition
	IntentOffset       int
	DescriptorID       string
	DescriptorPriority uint8
	Pan                Stereo
}

type ObservedInput struct{ SoundID uint8 }

const MaxObservedOperations uint32 = 262144

// ObservedHooks run synchronously on caller-owned detached bus/chip/PCM owners.
// Failure rolls back only score. It cannot undo effects in an external hook.
// The caller commits every detached owner together after a successful return.
type ObservedHooks struct {
	Operation     func(Operation) error
	ReadDAC       func(FeedbackRead) (DACFeedback, error)
	MaxOperations uint32
	Parser        *ParserMetadata
}

func (d *Driver) AdvanceObserved(input ObservedInput, hooks ObservedHooks) (Output, error) {
	if hooks.Operation == nil {
		return Output{}, fmt.Errorf("observed operation callback is required")
	}
	if hooks.MaxOperations > MaxObservedOperations {
		return Output{}, fmt.Errorf("operation budget exceeds profile")
	}
	if hooks.Parser != nil && hooks.Parser.catalogSHA != d.catalog.jsonSHA {
		return Output{}, fmt.Errorf("parser metadata belongs to another catalog")
	}
	return d.advance(input.SoundID, DACFeedback{}, &hooks)
}

func (t *tick) fail(err error) {
	if t.err == nil {
		t.err = err
	}
}
func (t *tick) operation(op Operation) {
	if t.hooks == nil || t.err != nil {
		return
	}
	limit := t.hooks.MaxOperations
	if limit == 0 {
		limit = MaxObservedOperations
	}
	if t.operations >= limit {
		t.fail(fmt.Errorf("observed operation budget exhausted"))
		return
	}
	op.Version = 1
	op.Sequence = t.operations
	op.IntentOffset = len(t.out.Intents)
	if !op.Position.Valid && op.VoiceSlot >= 0 {
		op.Position = t.position
	}
	if op.EventKind == "" && op.VoiceSlot >= 0 {
		op.EventKind = t.eventKind
	}
	if op.VoiceSlot >= 0 && int(op.VoiceSlot) < len(t.driver.state.Voices) {
		v := t.driver.state.Voices[op.VoiceSlot]
		op.Channel = v.Channel
		op.Flags = VoiceFlags{v.Active, v.NoteOff, v.Claimed, v.Tie, v.PitchControl.ModulationEnabled}
		op.Transpose = v.Transpose
		op.BaseLevel = v.BaseLevel
		op.BasePitch = v.BasePitch
		op.GateTicks = v.GateTicks
	}
	for i := 0; i < int(op.WriteCount); i++ {
		op.Writes[i] = cloneIntent(op.Writes[i])
	}
	t.operations++
	if err := t.hooks.Operation(op); err != nil {
		t.fail(fmt.Errorf("observed %s: %w", op.Kind, err))
	}
}

func cloneIntent(intent Intent) Intent {
	if intent.Pan != nil {
		value := *intent.Pan
		intent.Pan = &value
	}
	return intent
}
func (t *tick) enter(routine Routine, slot int, branch Branch) func() {
	op := Operation{Kind: OpEnter, Routine: routine, VoiceSlot: int8(slot), OtherSlot: -1, Branch: branch}
	if routine == RoutineTempo {
		state := t.driver.state
		op.Tempo = TempoObservation{Enabled: state.TempoPeriod != 0, Reload: state.TempoPeriod != 0 && state.TempoRemaining == 1, Period: state.TempoPeriod, Remaining: state.TempoRemaining}
		if op.Tempo.Reload {
			for i := 0; i < 10; i++ {
				if state.Voices[i].Active {
					op.Tempo.ActiveMusicMask |= 1 << i
				}
			}
		}
	}
	t.operation(op)
	return func() {
		t.operation(Operation{Kind: OpLeave, Routine: routine, VoiceSlot: int8(slot), OtherSlot: -1, Branch: branch})
	}
}
func (t *tick) priority(kind OperationKind, slot int, branch Branch) {
	t.operation(Operation{Kind: kind, Routine: RoutineOwnership, VoiceSlot: int8(slot), OtherSlot: -1, Branch: branch})
	if t.err == nil && kind == OpPriorityClear {
		t.out.ClearPrimaryPriority = true
	}
}
func (t *tick) readDAC(request FeedbackRead) (DACFeedback, error) {
	if t.err != nil {
		return DACFeedback{}, t.err
	}
	if t.hooks == nil {
		return t.feedback, nil
	}
	request.Position = t.position
	request.IntentOffset = len(t.out.Intents)
	t.operation(Operation{Kind: OpFeedbackRead, Routine: RoutineDAC, VoiceSlot: request.VoiceSlot, OtherSlot: -1, Read: request})
	if t.err != nil {
		return DACFeedback{}, t.err
	}
	if t.hooks.ReadDAC == nil {
		t.fail(fmt.Errorf("staged DAC feedback callback is required"))
		return DACFeedback{}, t.err
	}
	value, err := t.hooks.ReadDAC(request)
	if err != nil {
		t.fail(fmt.Errorf("staged DAC feedback: %w", err))
		return DACFeedback{}, t.err
	}
	if request.Kind == ReadBlockedPriority && !value.Blocked && value.Priority&0x3f != 0 {
		t.fail(fmt.Errorf("invalid staged DAC priority"))
		return DACFeedback{}, t.err
	}
	branch := BranchUnchanged
	if request.Kind == ReadPanRedirect {
		if value.PanRedirect {
			branch = BranchRedirect
		} else {
			branch = BranchYM
		}
	} else if value.Blocked {
		branch = BranchBlocked
	} else if request.DescriptorPriority >= value.Priority {
		branch = BranchAccepted
	} else {
		branch = BranchRejected
	}
	t.operation(Operation{Kind: OpFeedbackResult, Routine: RoutineDAC, VoiceSlot: request.VoiceSlot, OtherSlot: -1, Read: request, Branch: branch})
	return value, t.err
}
