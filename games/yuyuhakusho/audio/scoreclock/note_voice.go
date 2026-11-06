package scoreclock

import (
	"errors"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

var ErrNoteVoiceSequence = errors.New("invalid new-note voice sequence")
var ErrNoteVoiceUnavailable = errors.New("new-note voice work unavailable")

type notePhase uint8

const (
	noteWait notePhase = iota + 1
	noteBoundary
	notePrelude
	noteControlEnter
	noteControlParameter
	noteControlSetup
	noteControlPan
	noteControlPanWrite
	noteControlNoise
	noteControlReturn
	noteInstrumentEnter
	noteInstrumentResource
	noteInstrumentClaimed
	noteInstrumentBody
	noteInstrumentPan
	noteInstrumentReturn
	noteEnter
	noteKey
	notePitch
	notePitchParameter
	noteEarlyCommit
	noteCleanup
	noteCleanupSetup
	noteCleanupTied
	noteDuration
	noteCounter
	noteFinalCommit
	noteReturn
	noteDACRead
	noteFeedbackResult
	noteDACWrite
	noteTail
	noteVoiceReturn
	noteComplete
	noteTrackEnd
	noteControlQueue
	noteQueuePending
)

// NoteVoiceTransfer consumes one actual new-note voice body. Parser metadata
// describes bounded work only; the existing score driver remains its only
// event/counter owner. All nested costs and real effects share one service.
type NoteVoiceTransfer struct {
	s                                *service
	catalog                          *score.Catalog
	chip                             KeyGateChip
	slot                             int8
	owner                            score.Owner
	family                           string
	sequence                         uint32
	intentOffset                     int
	phase                            notePhase
	position                         score.EventPosition
	positionSet                      bool
	token, action, flow, form, probe string
	duration                         uint16
	pitchReturn, cleanupFinal, setup bool
	key                              *KeyTransfer
	keyPart                          uint8
	keyRest                          bool
	controlNext                      string
	noise                            uint8
	instrument                       *InstrumentTransfer
	instrumentMode                   string
	instrumentFinished               bool
	pan                              *PanRouting
	music                            *MusicPublication
	read                             score.FeedbackRead
	feedback                         score.DACFeedback
	readReady, readUsed, readControl bool
	tail                             *HeldVoiceTransfer
	done, running                    bool
	failure                          error
	allowStop, canonicalDomain       bool
	queueOp                          score.Operation
}

func noteVoiceFamily(channel string) (string, bool) {
	if channel == "dac" {
		return "dac", true
	}
	code, ok := heldChannel(channel)
	if !ok {
		return "", false
	}
	if code >= 128 {
		return "psg", true
	}
	return "fm", true
}

func BeginNoteVoice(clock Clock, op score.Operation, v score.VoiceState, catalog *score.Catalog, bus Bus, chip KeyGateChip, limits Limits) (*NoteVoiceTransfer, error) {
	if catalog == nil || !heldOperation(op, score.OpEnter, score.RoutineVoice) || op.Branch != score.BranchActive || op.OtherSlot != -1 || op.Position.Valid || !v.Active || !v.Owned || !observedVoiceMatches(op, v) {
		return nil, ErrNoteVoiceSequence
	}
	family, ok := noteVoiceFamily(v.Channel)
	if !ok || v.NoteWait != 1 {
		return nil, ErrNoteVoiceUnavailable
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return nil, err
	}
	return beginNoteVoiceOnService(s, op, v, catalog, chip, family)
}

func beginNoteVoiceOnService(s *service, op score.Operation, v score.VoiceState, catalog *score.Catalog, chip KeyGateChip, family string) (*NoteVoiceTransfer, error) {
	var err error
	if err = s.spend(16); err != nil {
		return nil, err
	}
	if err = s.branch(false, family == "dac"); err != nil {
		return nil, err
	}
	return &NoteVoiceTransfer{s: s, catalog: catalog, chip: chip, slot: op.VoiceSlot, owner: v.Owner, family: family, sequence: op.Sequence + 1, intentOffset: op.IntentOffset, phase: noteWait}, nil
}

func (t *NoteVoiceTransfer) fail(err error) error { t.failure = err; return err }
func (t *NoteVoiceTransfer) Clock() Clock         { return t.s.clock }

func (t *NoteVoiceTransfer) Consume(op score.Operation, v score.VoiceState) error {
	if t == nil {
		return ErrNoteVoiceSequence
	}
	family, ok := noteVoiceFamily(v.Channel)
	if t.done || t.running || t.failure != nil || t.phase == noteComplete || op.Sequence != t.sequence || op.VoiceSlot != t.slot || op.OtherSlot != -1 || op.IntentOffset != t.intentOffset || v.Owner != t.owner || !v.Active || !v.Owned || !ok || family != t.family || !observedVoiceMatches(op, v) {
		return t.fail(ErrNoteVoiceSequence)
	}
	if t.positionSet && t.phase != notePrelude && op.Position != t.position {
		return t.fail(ErrNoteVoiceSequence)
	}
	t.running = true
	err := t.consume(op, v)
	t.running = false
	if err != nil {
		return t.fail(fmt.Errorf("new-note phase %d: %w", t.phase, err))
	}
	if t.failure != nil {
		return t.failure
	}
	t.sequence++
	if op.Kind == score.OpChipWrite || op.Kind == score.OpPSGPitchWrite || op.Kind == score.OpQueueReplace {
		t.intentOffset += int(op.WriteCount)
	}
	return nil
}

func (t *NoteVoiceTransfer) consume(op score.Operation, v score.VoiceState) error {
	s := t.s
	switch t.phase {
	case noteWait:
		if !heldCounter(op, score.RoutineVoice, score.CounterNoteWait, 1, 0, score.BranchStep) || v.NoteWait != 1 {
			return ErrNoteVoiceSequence
		}
		t.phase = noteBoundary
	case noteBoundary:
		if !heldOperation(op, score.OpParameter, score.RoutineVoice) || op.Branch != score.BranchNoteBoundary || v.NoteWait != 0 {
			return ErrNoteVoiceSequence
		}
		if t.family == "dac" {
			if err := s.ordinaryCosts(16, 16); err != nil {
				return err
			}
		} else {
			if err := s.ordinaryCosts(16, 18, 16, 16); err != nil {
				return err
			}
		}
		t.phase = notePrelude
	case notePrelude:
		return t.consumePrelude(op)
	case noteControlEnter, noteControlParameter, noteControlSetup, noteControlPan, noteControlPanWrite, noteControlNoise, noteControlReturn, noteInstrumentEnter, noteInstrumentResource, noteInstrumentClaimed, noteInstrumentBody, noteInstrumentPan, noteInstrumentReturn, noteControlQueue:
		return t.consumeControl(op, v)
	case noteEnter:
		if !heldOperation(op, score.OpEnter, score.RoutineNote) || op.Branch != score.BranchNoteBoundary {
			return ErrNoteVoiceSequence
		}
		if t.family == "fm" {
			if err := s.spend(18); err != nil {
				return err
			}
			channel, _ := heldChannel(v.Channel)
			key, err := beginKey(s, KeyRequest{Channel: channel, NoteOff: v.NoteOff, Claimed: v.Claimed, Tied: v.Tie}, t.chip)
			if err != nil {
				return err
			}
			t.key, t.keyPart, t.keyRest, t.phase = key, 0, false, noteKey
		} else {
			t.phase = notePitch
		}
	case noteKey:
		return t.consumeKey(op, v)
	case notePitch:
		if t.token == "duration" {
			if !heldOperation(op, score.OpParameter, score.RoutineNote) || op.Branch != score.BranchPrevious || op.Duration != t.duration {
				return ErrNoteVoiceSequence
			}
			if err := s.spend(4); err != nil {
				return err
			}
			if err := s.branch(true, false); err != nil {
				return err
			}
			t.form = "previous"
			t.phase = noteDuration
			return nil
		}
		if !t.work(op, score.WorkPitchResolve, score.SitePitch) || op.Work.Family != t.family || op.Work.Form == "" {
			return ErrNoteVoiceSequence
		}
		t.form = op.Work.Form
		if err := s.spend(4); err != nil {
			return err
		}
		if err := s.branch(false, false); err != nil {
			return err
		}
		if t.family != "dac" {
			if err := s.spend(18); err != nil {
				return err
			}
		}
		if t.form == "chromatic" {
			t.phase = notePitchParameter
			return nil
		}
		if err := s.pitchWork(PitchWork{Family: t.family, Form: t.form}); err != nil {
			return err
		}
		if t.family == "dac" {
			t.phase = noteDuration
		} else {
			t.phase = noteEarlyCommit
		}
	case notePitchParameter:
		if !heldOperation(op, score.OpParameter, score.RoutineNote) || op.Branch != score.BranchChromatic || op.Duration != t.duration {
			return ErrNoteVoiceSequence
		}
		if err := s.pitchWork(PitchWork{Family: t.family, Form: t.form, Index: op.PitchIndex}); err != nil {
			return err
		}
		if t.family == "fm" {
			t.pitchReturn = true
			t.phase = noteDuration
		} else {
			t.phase = noteEarlyCommit
		}
	case noteEarlyCommit:
		if !t.work(op, score.WorkNoteCommit, score.SiteEarlyCommit) || op.Work.Phase != "after-pitch" {
			return ErrNoteVoiceSequence
		}
		if t.family == "psg" {
			cost := uint64(10)
			if t.form == "rest" {
				cost = 18
			}
			if err := s.spend(cost); err != nil {
				return err
			}
		}
		return t.beginCleanup(v, false)
	case noteCleanup:
		branch := score.BranchEarlyCleanup
		if t.cleanupFinal {
			branch = score.BranchFinalCleanup
		}
		if !heldOperation(op, score.OpNoteCleanup, score.RoutineNote) || op.Branch != branch {
			return ErrNoteVoiceSequence
		}
		if t.setup {
			t.phase = noteCleanupSetup
		} else if v.Tie {
			t.phase = noteCleanupTied
		} else {
			return t.afterCleanup(v)
		}
	case noteCleanupSetup:
		if !heldOperation(op, score.OpParameter, score.RoutineModulation) || op.Branch != score.BranchReload || op.EventKind != "definition-setup" || op.Iterations != 4 {
			return ErrNoteVoiceSequence
		}
		return t.afterCleanup(v)
	case noteCleanupTied:
		if !heldOperation(op, score.OpParameter, score.RoutineNote) || op.Branch != score.BranchTied {
			return ErrNoteVoiceSequence
		}
		return t.afterCleanup(v)
	case noteDuration:
		return t.consumeDuration(op)
	case noteCounter:
		if !heldCounter(op, score.RoutineNote, score.CounterNoteWait, uint16(v.NoteWait), uint16(uint8(t.duration)), score.BranchReload) || op.Duration != t.duration {
			return ErrNoteVoiceSequence
		}
		t.phase = noteFinalCommit
	case noteFinalCommit:
		if !t.work(op, score.WorkNoteCommit, score.SiteFinalCommit) || op.Work.Phase != "final" {
			return ErrNoteVoiceSequence
		}
		if t.family == "dac" {
			if err := s.noteCommit(NoteCommitWork{Family: "dac", DACSample: v.DACDescriptor != ""}); err != nil {
				return err
			}
			t.phase = noteReturn
			if v.DACDescriptor != "" {
				t.phase = noteDACRead
			}
			return nil
		}
		if t.probe != "reuse" {
			if err := s.spend(10); err != nil {
				return err
			}
		}
		return t.beginCleanup(v, true)
	case noteReturn:
		if !heldOperation(op, score.OpLeave, score.RoutineNote) || op.Branch != score.BranchNoteBoundary {
			return ErrNoteVoiceSequence
		}
		if t.family == "dac" {
			if t.music != nil {
				if _, err := t.music.Finish(); err != nil {
					return err
				}
				t.music = nil
			}
			t.phase = noteVoiceReturn
		} else {
			if err := s.spend(18); err != nil {
				return err
			}
			t.phase = noteTail
		}
	case noteTail:
		if t.tail == nil {
			channel, ok := heldChannel(v.Channel)
			if !ok {
				return ErrNoteVoiceUnavailable
			}
			t.tail = &HeldVoiceTransfer{s: s, catalog: t.catalog, chip: t.chip, slot: t.slot, owner: t.owner, channel: channel, channelName: v.Channel, position: op.Position, sequence: op.Sequence, intentOffset: op.IntentOffset, phase: heldPitchEnter, onNote: true}
		}
		if err := t.tail.Consume(op, v); err != nil {
			return err
		}
		if t.tail.phase == heldComplete {
			t.phase = noteComplete
		}
	case noteDACRead:
		if !heldOperation(op, score.OpFeedbackRead, score.RoutineDAC) || op.Read.Kind != score.ReadBlockedPriority || op.Read.DescriptorID != v.DACDescriptor || op.Read.TrackPan != v.Pan || op.Read.WritePan != v.Pan {
			return ErrNoteVoiceSequence
		}
		bus, ok := s.bus.(MusicBus)
		if !ok {
			return ErrNoteVoiceUnavailable
		}
		publication, err := beginMusicOnService(s, MusicRequest{DescriptorID: v.DACDescriptor, Pan: v.Pan}, bus, false)
		if err != nil {
			return err
		}
		t.music, t.read, t.feedback, t.readReady, t.readUsed, t.readControl = publication, op.Read, publication.Feedback(), true, false, false
		t.phase = noteFeedbackResult
	case noteFeedbackResult:
		return t.consumeFeedback(op)
	case noteDACWrite:
		input, ok := t.music.ExpectedInput()
		intent := score.Intent{Order: t.intentOffset, Kind: "dac", DescriptorID: input.DescriptorID, Pan: &score.Stereo{Left: v.Pan.Left, Right: v.Pan.Right}}
		if !ok || !heldWrite(op, score.OpChipWrite, intent) {
			return ErrNoteVoiceSequence
		}
		if err := t.music.Apply(input); err != nil {
			return err
		}
		t.phase = noteReturn
	case noteVoiceReturn:
		if !heldOperation(op, score.OpLeave, score.RoutineVoice) || op.Branch != score.BranchActive {
			return ErrNoteVoiceSequence
		}
		t.phase = noteComplete
	default:
		return ErrNoteVoiceSequence
	}
	return nil
}

func (t *NoteVoiceTransfer) work(op score.Operation, kind score.ParserWorkKind, site score.ParserWorkSite) bool {
	return heldOperation(op, score.OpParserWork, score.RoutineParser) && op.Position == t.position && op.Work.Kind == kind && op.WorkSite == site
}

func (t *NoteVoiceTransfer) consumePrelude(op score.Operation) error {
	if !op.Position.Valid || op.Position.Owner != t.owner {
		return ErrNoteVoiceSequence
	}
	if !t.positionSet {
		t.position, t.positionSet = op.Position, true
	} else if op.Position != t.position {
		return ErrNoteVoiceSequence
	}
	if heldOperation(op, score.OpEvent, score.RoutineParser) {
		if t.flow != "" {
			return ErrNoteVoiceSequence
		}
		if t.token == "control" {
			if op.EventKind != t.action {
				return ErrNoteVoiceSequence
			}
			t.phase = noteControlEnter
			if t.action == "track-end" && t.allowStop {
				t.phase = noteTrackEnd
			}
			return nil
		}
		if (t.token != "pitch" && t.token != "duration") || op.EventKind != "note" || op.Duration < 1 || op.Duration > 256 {
			return ErrNoteVoiceSequence
		}
		t.duration, t.phase = op.Duration, noteEnter
		return nil
	}
	if !heldOperation(op, score.OpParserWork, score.RoutineParser) || op.WorkSite != score.SitePrelude {
		return ErrNoteVoiceUnavailable
	}
	w := op.Work
	switch w.Kind {
	case score.WorkTokenRead:
		if t.flow != "" {
			return ErrNoteVoiceSequence
		}
		t.token = w.Form
	case score.WorkControlDispatch:
		if t.token != "control" || t.flow != "" {
			return ErrNoteVoiceSequence
		}
		switch w.Action {
		case "jump", "loop", "call", "return":
			t.flow = w.Action
		default:
			t.action = w.Action
		}
		if (w.Action == "track-end" || w.Action == "enqueue-replace-first") && !t.allowStop {
			return ErrNoteVoiceUnavailable
		}
	case score.WorkParameterRead:
		// The selected fixed control prefix owns these reads exactly once.
		if t.action == "" || t.flow != "" {
			return ErrNoteVoiceSequence
		}
		return nil
	case score.WorkLoop, score.WorkJump, score.WorkCall, score.WorkReturn:
		if t.flow != string(w.Kind) {
			return ErrNoteVoiceSequence
		}
		if err := t.s.parserWork(w); err != nil {
			return err
		}
		if err := t.s.spend(10); err != nil {
			return err
		}
		t.flow = ""
		return nil
	default:
		return ErrNoteVoiceUnavailable
	}
	return t.s.parserWork(w)
}

func (t *NoteVoiceTransfer) beginCleanup(v score.VoiceState, final bool) error {
	if err := t.s.noteCommit(NoteCommitWork{Family: t.family, Tied: v.Tie, ModulationEnabled: v.PitchControl.ModulationEnabled}); err != nil {
		return err
	}
	t.cleanupFinal, t.setup, t.phase = final, !v.Tie && v.PitchControl.ModulationEnabled, noteCleanup
	return nil
}
func (t *NoteVoiceTransfer) afterCleanup(v score.VoiceState) error {
	if t.cleanupFinal {
		t.phase = noteReturn
		return nil
	}
	if t.family == "psg" && t.form == "rest" {
		if err := t.s.spend(10); err != nil {
			return err
		}
		channel, _ := heldChannel(v.Channel)
		key, err := beginKey(t.s, KeyRequest{Channel: channel, NoteOff: v.NoteOff, Claimed: v.Claimed, Tied: v.Tie}, t.chip)
		if err != nil {
			return err
		}
		t.key, t.keyPart, t.keyRest, t.phase = key, 0, true, noteKey
		return nil
	}
	t.phase = noteDuration
	return nil
}

func (t *NoteVoiceTransfer) consumeKey(op score.Operation, v score.VoiceState) error {
	switch t.keyPart {
	case 0:
		if !heldOperation(op, score.OpEnter, score.RoutineKey) || op.Branch != score.BranchKeyOff {
			return ErrNoteVoiceSequence
		}
		t.keyPart = 1
	case 1:
		if effect, ok := t.key.ExpectedEffect(); ok {
			if !heldWrite(op, score.OpChipWrite, keyIntent(effect, t.intentOffset)) {
				return ErrNoteVoiceSequence
			}
			if err := t.key.Apply(effect); err != nil {
				return err
			}
		} else {
			branch := score.BranchTied
			if v.Claimed {
				branch = score.BranchClaimed
			}
			if !heldOperation(op, score.OpParameter, score.RoutineKey) || op.Branch != branch {
				return ErrNoteVoiceSequence
			}
		}
		t.keyPart = 2
	case 2:
		if !heldOperation(op, score.OpLeave, score.RoutineKey) || op.Branch != score.BranchKeyOff {
			return ErrNoteVoiceSequence
		}
		if _, err := t.key.Finish(); err != nil {
			return err
		}
		t.key = nil
		t.phase = notePitch
		if t.keyRest {
			t.phase = noteDuration
		}
	default:
		return ErrNoteVoiceSequence
	}
	return nil
}

func (t *NoteVoiceTransfer) consumeDuration(op score.Operation) error {
	if !heldOperation(op, score.OpParserWork, score.RoutineParser) || op.WorkSite != score.SiteDuration || op.Position != t.position {
		return ErrNoteVoiceSequence
	}
	if t.pitchReturn {
		if err := t.s.spend(16); err != nil {
			return err
		}
		t.pitchReturn = false
	}
	switch op.Work.Kind {
	case score.WorkDurationProbe:
		if t.form == "previous" || t.probe != "" {
			return ErrNoteVoiceSequence
		}
		if err := t.s.durationProbe(DurationProbeWork{Family: t.family, Outcome: op.Work.Outcome}); err != nil {
			return err
		}
		t.probe = op.Work.Outcome
		if t.probe == "reuse" {
			t.phase = noteCounter
		}
	case score.WorkDurationScale:
		if t.form != "previous" && t.probe != "take" {
			return ErrNoteVoiceSequence
		}
		if err := t.s.spend(18); err != nil {
			return err
		}
		if err := t.s.parserWork(op.Work); err != nil {
			return err
		}
		t.phase = noteCounter
	default:
		return ErrNoteVoiceUnavailable
	}
	return nil
}

func (t *NoteVoiceTransfer) ReadDAC(read score.FeedbackRead) (score.DACFeedback, error) {
	if t == nil {
		return score.DACFeedback{}, ErrNoteVoiceSequence
	}
	if t.done || t.failure != nil || t.running || !t.readReady || t.readUsed || read != t.read {
		return score.DACFeedback{}, t.fail(ErrNoteVoiceSequence)
	}
	t.readUsed = true
	return t.feedback, nil
}

func (t *NoteVoiceTransfer) consumeFeedback(op score.Operation) error {
	if !heldOperation(op, score.OpFeedbackResult, score.RoutineDAC) || !t.readReady || !t.readUsed || op.Read != t.read {
		return ErrNoteVoiceSequence
	}
	branch := score.BranchAccepted
	if t.readControl {
		branch = score.BranchYM
		if t.feedback.PanRedirect {
			branch = score.BranchRedirect
		}
	} else if t.feedback.Blocked {
		branch = score.BranchBlocked
	} else if _, ok := t.music.ExpectedInput(); !ok {
		branch = score.BranchRejected
	}
	if op.Branch != branch {
		return ErrNoteVoiceSequence
	}
	t.readReady, t.readUsed = false, false
	if t.readControl {
		t.phase = noteControlPanWrite
	} else {
		t.phase = noteReturn
		if _, ok := t.music.ExpectedInput(); ok {
			t.phase = noteDACWrite
		}
	}
	return nil
}

func (t *NoteVoiceTransfer) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrNoteVoiceSequence
	}
	if t.done || t.running || t.failure != nil || t.phase != noteComplete {
		return Clock{}, t.fail(ErrNoteVoiceSequence)
	}
	if t.tail != nil {
		if _, err := t.tail.Finish(); err != nil {
			return Clock{}, t.fail(err)
		}
	}
	t.done = true
	return t.s.clock, nil
}
