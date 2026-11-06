package scoreclock

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

type PauseLevelSource interface {
	ReadPauseLevels(at uint64) ([6][4]uint8, error)
}

// driverControl borrows the actual Root/score/chip owners. Every child uses
// the parent's service; all fields here are discarded after this invocation.
type driverControl struct {
	r          *requestTick
	s          *service
	stage      string
	route      rootcontrol.RuleRoute
	pauseIndex uint8
	command    uint8
	clearPhase rootcontrol.RulePhase
	action     score.MusicControlAction
	scan       int8
	keyIndex   int
	levels     *levelScan
	psg        *mutePSGScan
	level      *InstrumentTransfer
	volume     *VolumeTransfer
	fields     []DriverField
}

func (d *driverControl) field(kind string, cpu uint64, apply func() error, value func() uint8) error {
	return d.s.cost(cpu, func() error {
		if err := apply(); err != nil {
			return err
		}
		d.fields = append(d.fields, DriverField{Kind: kind, At: d.s.clock.Cycle, Value: value()})
		return nil
	})
}
func (d *driverControl) pauseRead(op rootcontrol.RuleOperation, read func() uint8) (uint8, error) {
	if op.ReadIndex != d.pauseIndex+1 || read == nil || op.Phase != rootcontrol.RuleGate {
		return 0, ErrRunningSequence
	}
	d.pauseIndex++
	cpu := uint64(12)
	if op.ReadIndex == 2 {
		cpu = 16
	}
	var value uint8
	if err := d.s.cost(cpu, func() error {
		value = read()
		if op.ReadIndex != 1 || value != 0 {
			d.fields = append(d.fields, DriverField{Kind: "pause-read", At: d.s.clock.Cycle, Value: value})
		}
		return nil
	}); err != nil {
		return 0, err
	}
	s := d.s
	switch op.ReadIndex {
	case 1:
		if err := s.branch(value != 0, true); err != nil {
			return 0, err
		}
		if value == 0 {
			d.route = rootcontrol.RuleNormal
			d.stage = "idle"
		} else {
			d.stage = "pause-read-more"
		}
	case 2:
		if err := s.branch(value != 255, true); err != nil {
			return 0, err
		}
		if value == 255 {
			if err := s.spend(16); err != nil {
				return 0, err
			}
			d.route = rootcontrol.RulePauseHold
			d.stage = "pause-done"
		}
	case 3:
		resume := value&128 != 0
		if err := s.branch(resume, false); err != nil {
			return 0, err
		}
		d.route = rootcontrol.RulePauseStart
		if resume {
			d.route = rootcontrol.RuleResume
		}
		d.stage = "pause-write"
	default:
		return 0, ErrRunningSequence
	}
	return value, nil
}
func (d *driverControl) pauseWrite(op rootcontrol.RuleOperation, apply func() error) error {
	value := uint8(255)
	if d.route == rootcontrol.RuleResume {
		value = 0
	}
	if d.stage != "pause-write" || op.Value != value {
		return ErrRunningSequence
	}
	if err := d.field("pause-write", 16, apply, func() uint8 { return value }); err != nil {
		return err
	}
	if d.route == rootcontrol.RulePauseStart {
		d.stage = "pause-capture"
		return nil
	}
	if d.route != rootcontrol.RuleResume || d.r.root.PauseRestore == nil {
		return ErrRunningSequence
	}
	if err := d.s.ordinaryCosts(14, 20); err != nil {
		return err
	}
	if err := d.s.request(20, false); err != nil {
		return err
	}
	if err := d.s.ordinaryCosts(16, 8); err != nil {
		return err
	}
	writes, err := rootcontrol.PauseRestoreWrites(*d.r.root.PauseRestore)
	if err != nil {
		return err
	}
	d.levels, err = beginLevelScan(d.s, writes, true)
	d.stage = "resume-levels"
	return err
}
func (d *driverControl) capture() (rootcontrol.PauseLevels, error) {
	if d.stage != "pause-capture-ready" {
		return rootcontrol.PauseLevels{}, ErrRunningSequence
	}
	source, ok := d.r.parent.s.chip.(PauseLevelSource)
	if !ok {
		return rootcontrol.PauseLevels{}, ErrRunningProfile
	}
	levels, err := source.ReadPauseLevels(d.s.clock.Cycle)
	if err != nil {
		return rootcontrol.PauseLevels{}, err
	}
	saved := rootcontrol.PauseLevels{Levels: levels}
	if _, err = rootcontrol.PauseRestoreWrites(saved); err != nil {
		return rootcontrol.PauseLevels{}, err
	}
	for i := 0; i < 8; i++ {
		if err = d.s.spend(20); err != nil {
			return rootcontrol.PauseLevels{}, err
		}
	}
	if err = d.s.spend(18); err != nil {
		return rootcontrol.PauseLevels{}, err
	}
	d.levels, err = beginLevelScan(d.s, rootcontrol.PauseMuteWrites(), false)
	if err != nil {
		return rootcontrol.PauseLevels{}, err
	}
	d.stage = "pause-levels"
	return saved, nil
}
func (d *driverControl) suspended() bool { return d.route != rootcontrol.RuleNormal }
func (d *driverControl) finishPaused() error {
	if d.stage != "pause-done" {
		return ErrRunningSequence
	}
	if d.route == rootcontrol.RuleResume {
		if d.r.root.PauseRestore != nil {
			return ErrRunningSequence
		}
	} else if d.r.root.PauseRestore == nil {
		return ErrRunningSequence
	}
	return nil
}

func (d *driverControl) beginFade() error {
	if d.stage != "idle" {
		return ErrRunningSequence
	}
	s := d.s
	if err := s.ordinaryCosts(18, 4); err != nil {
		return err
	}
	var steps uint8
	if err := s.cost(12, func() error { steps = d.r.root.Fade.Steps; return nil }); err != nil {
		return err
	}
	if err := s.branch(steps == 0, false); err != nil {
		return err
	}
	if steps == 0 {
		return s.spend(16)
	}
	var remaining uint8
	if err := s.cost(12, func() error { remaining = d.r.root.Fade.Remaining; return nil }); err != nil {
		return err
	}
	if err := s.branch(remaining == 0, false); err != nil {
		return err
	}
	if remaining != 0 {
		d.stage = "fade-timer"
	} else {
		d.stage = "fade-steps"
	}
	return nil
}
func (d *driverControl) fadeWrite(op rootcontrol.RuleOperation, apply func() error) error {
	w := op.FadeWrite
	kind := "fade-remaining"
	if w.Counter == rootcontrol.FadeStepsCounter {
		kind = "fade-steps"
	}
	valid := false
	switch d.stage {
	case "fade-timer":
		valid = w.Counter == rootcontrol.FadeRemainingCounter && w.Value == d.r.root.Fade.Remaining-1
	case "fade-steps":
		valid = w.Counter == rootcontrol.FadeStepsCounter && w.Value == d.r.root.Fade.Steps-1
	case "fade-reload":
		valid = w.Counter == rootcontrol.FadeRemainingCounter && w.Value == 3
	case "start-remaining":
		value := uint8(3)
		if d.command == 5 {
			value = 32
		}
		valid = w.Counter == rootcontrol.FadeRemainingCounter && w.Value == value
	case "start-steps":
		value := uint8(40)
		if d.command == 5 {
			value = 50
		}
		valid = w.Counter == rootcontrol.FadeStepsCounter && w.Value == value
	}
	if !valid {
		return ErrRunningSequence
	}
	if err := d.field(kind, 16, apply, func() uint8 {
		if w.Counter == rootcontrol.FadeStepsCounter {
			return d.r.root.Fade.Steps
		}
		return d.r.root.Fade.Remaining
	}); err != nil {
		return err
	}
	switch d.stage {
	case "fade-timer":
		d.stage = "idle"
		return d.s.spend(16)
	case "fade-steps":
		terminal := d.r.root.Fade.Steps == 0
		if err := d.s.branch(terminal, true); err != nil {
			return err
		}
		if terminal {
			d.stage = "clear-ym"
			d.clearPhase = rootcontrol.RuleFade
		} else {
			d.stage = "fade-reload"
		}
	case "fade-reload":
		if err := d.s.ordinaryCosts(8, 4); err != nil {
			return err
		}
		d.stage = "fade-action"
		d.scan = 1
	case "start-remaining":
		d.stage = "start-steps"
	case "start-steps":
		d.stage = "start-dac"
	}
	return nil
}
func (d *driverControl) beginDispatch(code uint8) error {
	if d.stage != "idle" || code != 1 && code != 5 && code != 4 {
		return ErrRunningProfile
	}
	// Fixed repeat classification, table selection and branch into the leaf.
	if err := d.s.spend(8); err != nil {
		return err
	}
	if err := d.s.branch(true, true); err != nil {
		return err
	}
	if err := d.s.ordinaryCosts(4, 10, 14, 10); err != nil {
		return err
	}
	d.command = code
	if code == 4 {
		d.stage = "clear-ym"
		d.clearPhase = rootcontrol.RuleDispatch
	} else {
		d.stage = "start-remaining"
	}
	return nil
}
func (d *driverControl) bulk(op rootcontrol.RuleOperation, apply func() error) error {
	if d.stage != "clear-bulk" || op.MusicOnly || op.HeaderLongs != 16 || op.VoiceLongs != 216 {
		return ErrRunningSequence
	}
	s := d.s
	if err := s.ordinaryCosts(8, 8); err != nil {
		return err
	}
	for i := 0; i < 232; i++ {
		var effect func() error
		if i == 0 {
			effect = apply
		}
		if err := s.cost(20, effect); err != nil {
			return err
		}
		if err := s.pauseLoopEnd(i == 231); err != nil {
			return err
		}
	}
	d.stage = "clear-score"
	return nil
}
func (d *driverControl) selected(op rootcontrol.RuleOperation, apply func() error) error {
	if d.stage != "clear-selected" || op.Value != 255 {
		return ErrRunningSequence
	}
	if err := d.field("clear-selected", 16, apply, func() uint8 { return d.r.root.Selected }); err != nil {
		return err
	}
	if err := d.s.ordinaryCosts(18, 4, 4); err != nil {
		return err
	}
	d.keyIndex = 0
	d.stage = "clear-keys"
	return nil
}

func (d *driverControl) rootOperation(op rootcontrol.RuleOperation) (bool, error) {
	s := d.s
	switch op.Kind {
	case rootcontrol.RulePauseRead:
		if op.Phase != rootcontrol.RuleGate {
			return true, ErrRunningSequence
		}
		if op.ReadIndex == 1 {
			d.r.parent.pauseRead = true
		}
		return true, nil
	case rootcontrol.RulePauseWrite:
		return true, nil
	case rootcontrol.RuleFadeWrite:
		return true, nil
	case rootcontrol.RuleBulkClear:
		if op.MusicOnly {
			return false, nil
		}
		return true, nil
	case rootcontrol.RuleSelectedWrite:
		if d.stage == "clear-selected" {
			return true, nil
		}
		return false, nil
	case rootcontrol.RulePauseCapture:
		if d.stage != "pause-capture" || d.route != rootcontrol.RulePauseStart {
			return true, ErrRunningSequence
		}
		if _, ok := s.chip.(PauseLevelSource); !ok {
			return true, ErrRunningProfile
		}
		if err := s.ordinaryCosts(14, 20); err != nil {
			return true, err
		}
		if err := s.acquire(); err != nil {
			return true, err
		}
		if err := s.ordinaryCosts(16, 8); err != nil {
			return true, err
		}
		d.stage = "pause-capture-ready"
		return true, nil
	case rootcontrol.RulePauseCommit:
		if d.route != rootcontrol.RulePauseStart || d.stage != "pause-muted" {
			return true, ErrRunningSequence
		}
		d.stage = "pause-done"
		return true, nil
	case rootcontrol.RulePauseRelease:
		if d.route != rootcontrol.RuleResume || d.stage != "resume-levels" || d.levels == nil || !d.levels.done {
			return true, ErrRunningSequence
		}
		d.stage = "pause-done"
		return true, nil
	case rootcontrol.RuleChipWrite:
		return true, d.chipWrite(op.Write)
	}
	return false, nil
}
func (d *driverControl) chipWrite(write score.Intent) error {
	s := d.s
	switch d.stage {
	case "clear-ym":
		if write.Kind != "ym" || write.Bank != 0 || write.Register != 0x27 || write.Value != 0 {
			return ErrRunningSequence
		}
		if err := s.ordinaryCosts(4, 4, 18, 10); err != nil {
			return err
		}
		if err := s.serveYM(Request{Register: 0x27}); err != nil {
			return err
		}
		d.stage = "clear-bulk"
	case "clear-keys":
		values := [6]uint8{2, 6, 1, 5, 0, 4}
		if d.keyIndex >= 6 || write.Kind != "ym" || write.Bank != 0 || write.Register != 0x28 || write.Value != values[d.keyIndex] {
			return ErrRunningSequence
		}
		if err := s.ordinaryCosts(4, 18); err != nil {
			return err
		}
		if err := s.serveYM(Request{Register: 0x28, Value: write.Value}); err != nil {
			return err
		}
		d.keyIndex++
		if d.keyIndex%2 == 0 {
			if err := s.pauseLoopEnd(d.keyIndex == 6); err != nil {
				return err
			}
		}
		if d.keyIndex == 6 {
			var err error
			d.levels, err = beginLevelScan(s, rootcontrol.PauseMuteWrites(), false)
			if err != nil {
				return err
			}
			d.stage = "clear-levels"
		}
	case "clear-levels", "pause-levels", "resume-levels":
		if write.Kind != "ym" || d.levels == nil {
			return ErrRunningSequence
		}
		if err := d.levels.next(rootcontrol.LevelWrite{Bank: write.Bank, Register: write.Register, Value: write.Value}); err != nil {
			return err
		}
		if d.levels.done {
			if d.stage == "clear-levels" {
				d.stage = "clear-psg-prefix"
			} else if d.stage == "pause-levels" {
				d.stage = "pause-psg-prefix"
			}
		}
	case "clear-psg-prefix", "pause-psg-prefix", "clear-psg", "pause-psg":
		if d.stage == "pause-psg-prefix" {
			if err := s.spend(8); err != nil {
				return err
			}
			for i := 0; i < 8; i++ {
				if err := s.spend(20); err != nil {
					return err
				}
			}
		}
		if d.stage == "clear-psg-prefix" || d.stage == "pause-psg-prefix" {
			pause := d.stage == "pause-psg-prefix"
			if err := s.spend(10); err != nil {
				return err
			}
			var err error
			d.psg, err = beginMutePSG(s, d.r.parent.chip)
			if err != nil {
				return err
			}
			d.stage = "clear-psg"
			if pause {
				d.stage = "pause-psg"
			}
		}
		if write.Kind != "psg" || d.psg == nil {
			return ErrRunningSequence
		}
		if err := d.psg.next(write.Value); err != nil {
			return err
		}
		if d.psg.done {
			if d.stage == "pause-psg" {
				d.stage = "pause-muted"
			} else if d.clearPhase == rootcontrol.RuleFade {
				d.stage = "idle"
			} else {
				d.stage = "dispatch-done"
			}
		}
	default:
		return fmt.Errorf("%w: Root chip write in %s", ErrRunningSequence, d.stage)
	}
	return nil
}

func (d *driverControl) handlesScore(op score.Operation) bool {
	return d.action != 0 || op.Routine == score.RoutineMusicControl
}
func (d *driverControl) scoreOperation(op score.Operation, state score.State) error {
	s := d.s
	if op.Kind == score.OpEnter && op.Routine == score.RoutineMusicControl {
		if d.action != 0 {
			return ErrRunningSequence
		}
		switch op.Branch {
		case score.BranchStopMusicDACFlags:
			if d.stage != "start-dac" {
				return ErrRunningSequence
			}
			d.action = score.ActionStopMusicDACFlags
		case score.BranchResetScore:
			if d.stage != "clear-score" {
				return ErrRunningSequence
			}
			d.action = score.ActionResetScore
		case score.BranchFadeMusicStep:
			if d.stage != "fade-action" {
				return ErrRunningSequence
			}
			d.action = score.ActionFadeMusicStep
			d.stage = "fade-scan"
		default:
			return ErrRunningSequence
		}
		return nil
	}
	if d.action == score.ActionStopMusicDACFlags {
		if op.Kind == score.OpParameter && op.Action == d.action && op.VoiceSlot == 0 && d.stage == "start-dac" {
			if err := s.spend(16); err != nil {
				return err
			}
			d.stage = "start-return"
			return nil
		}
		if op.Kind == score.OpLeave && op.Routine == score.RoutineMusicControl && d.stage == "start-return" {
			if err := s.spend(16); err != nil {
				return err
			}
			d.action = 0
			d.stage = "dispatch-done"
			return nil
		}
		return ErrRunningSequence
	}
	if d.action == score.ActionResetScore {
		if op.Kind == score.OpScoreReset && op.Iterations == 18 && d.stage == "clear-score" {
			d.stage = "clear-score-return"
			return nil
		}
		if op.Kind == score.OpLeave && op.Routine == score.RoutineMusicControl && d.stage == "clear-score-return" {
			d.action = 0
			d.stage = "clear-selected"
			return nil
		}
		return ErrRunningSequence
	}
	if d.action != score.ActionFadeMusicStep {
		return ErrRunningSequence
	}
	if op.Kind == score.OpLeave && op.Routine == score.RoutineMusicControl {
		if d.scan != 10 || d.stage != "fade-scan" {
			return ErrRunningSequence
		}
		if err := s.spend(16); err != nil {
			return err
		}
		d.action = 0
		d.stage = "idle"
		return nil
	}
	if d.scan < 1 || d.scan > 9 || op.VoiceSlot != d.scan {
		return ErrRunningSequence
	}
	v := state.Voices[d.scan]
	switch d.stage {
	case "fade-scan":
		if !heldOperation(op, score.OpVoiceScan, score.RoutineMusicControl) || !observedProjectionMatches(op, v) {
			return ErrRunningSequence
		}
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.branch(!v.Active, false); err != nil {
			return err
		}
		if !v.Active {
			return d.fadeNext()
		}
		d.stage = "fade-counter"
	case "fade-counter":
		next := v.BaseLevel + 1
		if !heldOperation(op, score.OpCounter, score.RoutineMusicControl) || op.Counter != score.CounterBaseLevel || op.Before != uint16(v.BaseLevel) || op.After != uint16(next) {
			return ErrRunningSequence
		}
		if err := s.spend(16); err != nil {
			return err
		}
		if d.scan < 7 {
			if err := s.branch(next < 128, false); err != nil {
				return err
			}
			if next >= 128 {
				if err := s.ordinaryCosts(16, 10); err != nil {
					return err
				}
				return d.fadeNext()
			}
			if err := s.spend(18); err != nil {
				return err
			}
			d.stage = "fade-level-enter"
		} else {
			if err := s.spend(16); err != nil {
				return err
			}
			if err := s.branch(next < 16, false); err != nil {
				return err
			}
			if next >= 16 {
				if err := s.ordinaryCosts(16, 10); err != nil {
					return err
				}
				return d.fadeNext()
			}
			if err := s.ordinaryCosts(12, 18); err != nil {
				return err
			}
			channel, ok := heldChannel(v.Channel)
			if !ok {
				return ErrRunningProfile
			}
			var err error
			d.volume, err = beginBaseVolume(s, EnvelopeVolumeRequest{BaseLevel: next, Channel: channel, NoteOff: v.NoteOff, Claimed: v.Claimed, Tied: v.Tie, GateTicks: v.GateTicks, GateRemaining: v.GateRemaining}, d.r.parent.chip, next)
			if err != nil {
				return err
			}
			if _, ok := d.volume.ExpectedWrite(); !ok {
				if _, err = d.volume.Finish(); err != nil {
					return err
				}
				d.volume = nil
				return d.fadeNext()
			}
			d.stage = "fade-volume"
		}
	case "fade-volume":
		value, _ := d.volume.ExpectedWrite()
		if !heldWrite(op, score.OpChipWrite, score.Intent{Order: op.IntentOffset, Kind: "psg", Value: value}) {
			return ErrRunningSequence
		}
		if err := d.volume.Write(value); err != nil {
			return err
		}
		if _, err := d.volume.Finish(); err != nil {
			return err
		}
		d.volume = nil
		return d.fadeNext()
	case "fade-level-enter":
		if !heldOperation(op, score.OpEnter, score.RoutineLevel) {
			return ErrRunningSequence
		}
		work, patch, err := d.r.library.Catalog().InstrumentParameters(v.Tone)
		if err != nil {
			return err
		}
		channel, ok := heldChannel(v.Channel)
		if !ok {
			return ErrRunningProfile
		}
		d.level, err = beginInstrumentTransfer(s, InstrumentRequest{Mode: "level", Domain: "music", Channel: channel, Claimed: v.Claimed, Work: work, Patch: patch, BaseLevel: v.BaseLevel, LoadedAlgorithm: v.LoadedAlgorithm})
		if err != nil {
			return err
		}
		d.stage = "fade-level-resource"
	case "fade-level-resource":
		if v.Claimed {
			if !heldOperation(op, score.OpParameter, score.RoutineLevel) || op.Branch != score.BranchClaimed {
				return ErrRunningSequence
			}
			d.stage = "fade-level-return"
			return nil
		}
		if !heldOperation(op, score.OpResourceUnavailable, score.RoutineLevel) || op.DefinitionID != d.level.request.Work.ID || op.Iterations != 4 {
			return ErrRunningSequence
		}
		d.stage = "fade-level-writes"
		if len(d.level.writes) == 0 {
			d.stage = "fade-level-return"
		}
	case "fade-level-writes":
		write := d.level.writes[d.level.cursor]
		if !heldWrite(op, score.OpChipWrite, keyIntent(KeyEffect{Kind: "ym", YM: write}, op.IntentOffset)) {
			return ErrRunningSequence
		}
		if err := d.level.WriteNext(write); err != nil {
			return err
		}
		if d.level.cursor == len(d.level.writes) {
			d.stage = "fade-level-return"
		}
	case "fade-level-return":
		if !heldOperation(op, score.OpLeave, score.RoutineLevel) {
			return ErrRunningSequence
		}
		if _, err := d.level.Finish(); err != nil {
			return err
		}
		d.level = nil
		return d.fadeNext()
	default:
		return fmt.Errorf("%w: fade stage %s operation %s/%s", ErrRunningSequence, d.stage, op.Kind, op.Routine)
	}
	return nil
}
func (d *driverControl) fadeNext() error {
	if err := d.s.spend(12); err != nil {
		return err
	}
	if err := d.s.pauseLoopEnd(d.scan == 6 || d.scan == 9); err != nil {
		return err
	}
	d.scan++
	if d.scan == 7 {
		if err := d.s.spend(4); err != nil {
			return err
		}
	}
	d.stage = "fade-scan"
	return nil
}
