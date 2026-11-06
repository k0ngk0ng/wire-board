package scoreclock

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

// dispatchInit is invocation-local work on the parent's sole service. Catalog,
// Root and score remain their unique owners; this holds no copied voice state.
type dispatchInit struct {
	r         *requestTick
	s         *service
	plan      score.HeaderPlan
	stage     string
	voice     int
	scan      int8
	key       *KeyTransfer
	keySlot   int8
	keyPart   uint8
	keyCaller string
	patch     *trackEndTransfer
	secondary int
	fields    []DriverField
	done      bool
}

func (d *dispatchInit) field(kind string, cpu uint64, apply func() error, value func() uint8) error {
	return d.s.cost(cpu, func() error {
		if err := apply(); err != nil {
			return err
		}
		d.fields = append(d.fields, DriverField{Kind: kind, At: d.s.clock.Cycle, Value: value()})
		return nil
	})
}

func (d *dispatchInit) priorityClear(op score.Operation, apply func() error) error {
	if d.stage != "stop-priority-effect" || op.Kind != score.OpPriorityClear || op.Branch != score.BranchStopPrimary || apply == nil {
		return ErrRunningSequence
	}
	if err := d.field("priority-clear", 16, apply, func() uint8 { return d.r.root.Priority }); err != nil {
		return err
	}
	d.stage = "stop-ym"
	return nil
}

func (d *dispatchInit) headerEffect(op score.Operation, apply func() error) error {
	if apply == nil {
		return ErrRunningSequence
	}
	s := d.s
	switch op.Kind {
	case score.OpPrioritySave:
		if d.stage != "reset-save-effect" {
			return ErrRunningSequence
		}
		if err := d.field("priority-save", 12, apply, func() uint8 { return d.r.root.Priority }); err != nil {
			return err
		}
		if err := s.spend(8); err != nil {
			return err
		}
		d.stage = "reset-clear"
	case score.OpHeaderClear:
		if d.stage != "reset-clear-effect" {
			return ErrRunningSequence
		}
		if err := s.ordinaryCosts(8, 8); err != nil {
			return err
		}
		// The rules contract exposes this contiguous RAM clear as one bulk
		// effect; no callback or IRQ observes intermediate long stores here.
		for i := 0; i < 136; i++ {
			var effect func() error
			if i == 0 {
				effect = apply
			}
			if err := s.cost(20, effect); err != nil {
				return err
			}
			if err := s.pauseLoopEnd(i == 135); err != nil {
				return err
			}
		}
		d.stage = "reset-restore"
	case score.OpPriorityRestore:
		if d.stage != "reset-restore-effect" {
			return ErrRunningSequence
		}
		if err := s.spend(8); err != nil {
			return err
		}
		if err := d.field("priority-restore", 12, apply, func() uint8 { return d.r.root.Priority }); err != nil {
			return err
		}
		d.stage = "reset-selected"
	case score.OpSelectedReset:
		if d.stage != "reset-selected-effect" {
			return ErrRunningSequence
		}
		if err := d.field("header-selected", 16, apply, func() uint8 { return d.r.root.Selected }); err != nil {
			return err
		}
		d.stage = "reset-return"
	default:
		return ErrRunningSequence
	}
	return nil
}

func (d *dispatchInit) consume(op score.Operation, state score.State) error {
	if d.done || op.Position.Valid {
		return ErrRunningSequence
	}
	if d.patch != nil {
		if err := d.patch.Consume(op, state); err != nil {
			return err
		}
		if d.patch.phase == trackOwnerReturn {
			d.patch = nil
			d.stage = "stop-owner-return"
		}
		return nil
	}
	if d.key != nil {
		return d.consumeKey(op, state)
	}
	s := d.s
	switch d.stage {
	case "dispatch":
		branch := score.BranchPrimary
		if d.plan.Music {
			branch = score.BranchMusic
		}
		if !heldOperation(op, score.OpEnter, score.RoutineDispatch) || op.SoundID != d.plan.ID || op.Branch != branch {
			return ErrRunningSequence
		}
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.branch(true, true); err != nil {
			return err
		}
		if d.plan.Music {
			d.stage = "music-stop"
		} else {
			if err := s.ordinaryCosts(8, 8, 10, 18, 4, 4, 8, 8, 8, 4, 8, 4, 4); err != nil {
				return err
			}
			d.stage = "primary-header"
		}
	case "music-stop":
		if !heldOperation(op, score.OpEnter, score.RoutineStopPrimary) {
			return ErrRunningSequence
		}
		if err := s.spend(18); err != nil {
			return err
		}
		d.stage = "stop-priority"
	case "stop-priority":
		if op.Kind != score.OpPriorityClear || op.Branch != score.BranchStopPrimary {
			return ErrRunningSequence
		}
		d.stage = "stop-priority-effect"
	case "stop-ym", "reset-ym":
		if !heldWrite(op, score.OpChipWrite, score.Intent{Order: op.IntentOffset, Kind: "ym", Register: 0x27}) || op.VoiceSlot != -1 {
			return ErrRunningSequence
		}
		if err := s.ordinaryCosts(4, 4, 18, 10); err != nil {
			return err
		}
		if err := s.serveYM(Request{Register: 0x27}); err != nil {
			return err
		}
		if d.stage == "stop-ym" {
			if err := s.ordinaryCosts(8, 4); err != nil {
				return err
			}
			d.stage = "stop-scan"
		} else {
			d.stage = "reset-save"
		}
	case "stop-scan":
		if d.scan == 16 {
			if !heldOperation(op, score.OpLeave, score.RoutineStopPrimary) {
				return ErrRunningSequence
			}
			if err := s.spend(16); err != nil {
				return err
			}
			d.stage = "secondary-enter"
			return nil
		}
		if !heldOperation(op, score.OpVoiceScan, score.RoutineStopPrimary) || op.VoiceSlot != d.scan {
			return ErrRunningSequence
		}
		v := state.Voices[d.scan]
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.branch(!v.Active, true); err != nil {
			return err
		}
		if !v.Active {
			return d.stopNext()
		}
		channel, ok := heldChannel(v.Channel)
		if !ok {
			return ErrRunningProfile
		}
		if err := s.ordinaryCosts(16, 4, 12); err != nil {
			return err
		}
		if err := s.branch(channel >= 128, false); err != nil {
			return err
		}
		if err := s.spend(18); err != nil {
			return err
		}
		d.keySlot, d.keyCaller, d.stage = d.scan, "stop", "key-enter"
	case "key-enter", "music-key":
		if !heldOperation(op, score.OpEnter, score.RoutineKey) || op.VoiceSlot != d.keySlot || op.Branch != score.BranchKeyOff {
			return ErrRunningSequence
		}
		v := state.Voices[d.keySlot]
		channel, ok := heldChannel(v.Channel)
		if v.Channel == "" {
			channel, ok = 0, true
		}
		if v.Channel == "fm6" {
			channel, ok = 6, true
		}
		if !ok {
			return ErrRunningProfile
		}
		if d.stage == "music-key" {
			if err := s.spend(18); err != nil {
				return err
			}
		}
		var err error
		d.key, err = beginKey(s, KeyRequest{Channel: channel, NoteOff: v.NoteOff, Claimed: v.Claimed, Tied: v.Tie}, d.r.parent.chip)
		d.keyPart = 1
		return err
	case "stop-owner":
		if !heldOperation(op, score.OpEnter, score.RoutineOwnership) || op.VoiceSlot != d.scan {
			return ErrRunningSequence
		}
		return d.stopRestorePrefix(state)
	case "stop-restore":
		target := d.stopTarget(state)
		if !heldOperation(op, score.OpRestore, score.RoutineOwnership) || op.VoiceSlot != target || op.OtherSlot != d.scan {
			return ErrRunningSequence
		}
		channel, _ := heldChannel(state.Voices[d.scan].Channel)
		if err := s.ordinaryCosts(16, 16); err != nil {
			return err
		}
		if channel < 128 {
			if err := s.ordinaryCosts(12, 18); err != nil {
				return err
			}
			d.stage = "stop-patch"
		} else {
			if err := s.spend(16); err != nil {
				return err
			}
			noise := state.Voices[target].Channel == "psg-noise"
			if err := s.branch(!noise, false); err != nil {
				return err
			}
			if noise {
				d.stage = "stop-noise"
			} else {
				d.stage = "stop-owner-return"
			}
		}
	case "stop-patch":
		target := d.stopTarget(state)
		// Share the proved restored patch/pan service until its ownership-return
		// boundary. The stop caller supplies its distinct footer, not track-end.
		d.patch = &trackEndTransfer{s: s, catalog: d.r.library.Catalog(), chip: d.r.parent.chip, slot: d.scan, target: target, owner: state.Voices[d.scan].Owner, targetOwner: state.Voices[target].Owner, targetOwned: state.Voices[target].Owned, sequence: op.Sequence, intentOffset: op.IntentOffset, phase: trackPatchEnter}
		if err := d.patch.Consume(op, state); err != nil {
			return err
		}
	case "stop-noise":
		target := d.stopTarget(state)
		v := state.Voices[target]
		if !heldWrite(op, score.OpChipWrite, score.Intent{Order: op.IntentOffset, Kind: "psg", Value: v.NoiseControl}) || op.VoiceSlot != target {
			return ErrRunningSequence
		}
		if err := s.cost(24, func() error { return d.r.parent.chip.WritePSG(s.clock.Cycle, v.NoiseControl) }); err != nil {
			return err
		}
		d.stage = "stop-owner-return"
	case "stop-owner-return":
		if !heldOperation(op, score.OpLeave, score.RoutineOwnership) || op.VoiceSlot != d.scan {
			return ErrRunningSequence
		}
		channel, _ := heldChannel(state.Voices[d.scan].Channel)
		if channel < 128 {
			if err := s.ordinaryCosts(4, 10); err != nil {
				return err
			}
		}
		return d.stopNext()
	case "secondary-enter":
		if !heldOperation(op, score.OpEnter, score.RoutineStopSecondary) {
			return ErrRunningSequence
		}
		if err := s.ordinaryCosts(18, 8); err != nil {
			return err
		}
		d.scan = 16
		d.stage = "secondary-scan"
	case "secondary-scan":
		if !heldOperation(op, score.OpVoiceScan, score.RoutineStopSecondary) || op.VoiceSlot != d.scan || state.Voices[d.scan].Active {
			return ErrRunningProfile
		}
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.branch(true, false); err != nil {
			return err
		}
		d.scan++
		if d.scan == 17 {
			return s.spend(8)
		}
		d.stage = "secondary-return"
	case "secondary-return":
		if !heldOperation(op, score.OpLeave, score.RoutineStopSecondary) {
			return ErrRunningSequence
		}
		if err := s.spend(16); err != nil {
			return err
		}
		d.stage = "reset-enter"
	case "reset-enter":
		if !heldOperation(op, score.OpEnter, score.RoutineMusicReset) {
			return ErrRunningSequence
		}
		if err := s.spend(18); err != nil {
			return err
		}
		d.stage = "reset-ym"
	case "reset-save", "reset-clear", "reset-restore", "reset-selected":
		kind := map[string]score.OperationKind{"reset-save": score.OpPrioritySave, "reset-clear": score.OpHeaderClear, "reset-restore": score.OpPriorityRestore, "reset-selected": score.OpSelectedReset}[d.stage]
		if op.Kind != kind {
			return ErrRunningSequence
		}
		d.stage += "-effect"
	case "reset-return":
		if !heldOperation(op, score.OpLeave, score.RoutineMusicReset) {
			return ErrRunningSequence
		}
		if err := s.spend(16); err != nil {
			return err
		}
		d.stage = "music-header"
	case "music-header":
		if !heldOperation(op, score.OpParameter, score.RoutineInitialize) || op.EventKind != "music-header" || op.SoundID != d.plan.ID {
			return ErrRunningSequence
		}
		if err := s.ordinaryCosts(8, 8, 10, 18, 4, 8, 8, 16, 20, 20, 4, 4, 8, 4, 12); err != nil {
			return err
		}
		if err := s.branch(d.musicFM() == 0, false); err != nil {
			return err
		}
		if d.musicFM() == 0 {
			return ErrRunningProfile
		}
		if err := s.ordinaryCosts(4, 8, 8, 12, 4, 8, 8, 8); err != nil {
			return err
		}
		d.stage = "music-voice"
	case "primary-header":
		if !heldOperation(op, score.OpParameter, score.RoutineInitialize) || op.EventKind != "primary-header" || op.SoundID != d.plan.ID {
			return ErrRunningSequence
		}
		d.stage = "primary-claim"
	case "primary-claim":
		if d.voice >= len(d.plan.Voices) {
			return ErrRunningSequence
		}
		v := d.plan.Voices[d.voice]
		if !heldOperation(op, score.OpClaim, score.RoutineOwnership) || op.VoiceSlot != restoredMusicSlot(v.Slot) || op.OtherSlot != v.Slot {
			return ErrRunningSequence
		}
		channel, ok := heldChannel(v.Channel)
		if !ok {
			return ErrRunningProfile
		}
		if err := s.ordinaryCosts(4, 12, 4); err != nil {
			return err
		}
		if err := s.branch(channel >= 128, false); err != nil {
			return err
		}
		if channel < 128 {
			if err := s.ordinaryCosts(4, 10, 8, 18, 16, 10); err != nil {
				return err
			}
			d.stage = "primary-voice"
		} else {
			if err := s.ordinaryCosts(12, 18, 16, 8); err != nil {
				return err
			}
			if err := s.branch(channel != 192, false); err != nil {
				return err
			}
			if channel == 192 {
				d.stage = "primary-mute-first"
			} else {
				d.stage = "primary-voice"
			}
		}
	case "primary-mute-first", "primary-mute-second":
		value := uint8(223)
		first := d.stage == "primary-mute-first"
		if !first {
			value = 255
		}
		if !heldWrite(op, score.OpChipWrite, score.Intent{Order: op.IntentOffset, Kind: "psg", Value: value}) || op.VoiceSlot != d.plan.Voices[d.voice].Slot {
			return ErrRunningSequence
		}
		if first {
			if err := s.ordinaryCosts(4, 8); err != nil {
				return err
			}
		} else {
			if err := s.spend(10); err != nil {
				return err
			}
		}
		if err := s.cost(16, func() error { return d.r.parent.chip.WritePSG(s.clock.Cycle, value) }); err != nil {
			return err
		}
		if first {
			d.stage = "primary-mute-second"
		} else {
			d.stage = "primary-voice"
		}
	case "music-voice", "primary-voice":
		if d.voice >= len(d.plan.Voices) || !heldOperation(op, score.OpEnter, score.RoutineInitialize) || op.VoiceSlot != d.plan.Voices[d.voice].Slot {
			return ErrRunningSequence
		}
		d.stage += "-parameter"
	case "music-voice-parameter", "primary-voice-parameter":
		if !heldOperation(op, score.OpParameter, score.RoutineInitialize) || op.EventKind != "voice-header" || op.SoundID != d.plan.ID || op.VoiceSlot != d.plan.Voices[d.voice].Slot {
			return ErrRunningSequence
		}
		if d.plan.Music {
			if err := s.ordinaryCosts(8, 16, 12, 12); err != nil {
				return err
			}
			if d.voice < d.musicFM() {
				if err := s.spend(12); err != nil {
					return err
				}
			}
			if err := s.ordinaryCosts(12, 4, 8, 8, 16, 16); err != nil {
				return err
			}
			if d.voice >= d.musicFM() {
				if err := s.ordinaryCosts(16, 16); err != nil {
					return err
				}
			}
		} else {
			if err := s.ordinaryCosts(18, 4, 4); err != nil {
				return err
			}
			for i := 0; i < 12; i++ {
				if err := s.spend(20); err != nil {
					return err
				}
				if err := s.pauseLoopEnd(i == 11); err != nil {
					return err
				}
			}
			if err := s.ordinaryCosts(16, 12, 12, 4, 8, 8, 16, 16, 16, 12, 4); err != nil {
				return err
			}
			psg := d.plan.Voices[d.voice].Slot >= 13
			if err := s.branch(psg, false); err != nil {
				return err
			}
			if !psg {
				if err := s.spend(16); err != nil {
					return err
				}
			}
		}
		if d.plan.Music {
			d.stage = "music-voice-return"
		} else {
			d.stage = "primary-voice-return"
		}
	case "music-voice-return", "primary-voice-return":
		if !heldOperation(op, score.OpLeave, score.RoutineInitialize) || op.VoiceSlot != d.plan.Voices[d.voice].Slot {
			return ErrRunningSequence
		}
		if d.plan.Music {
			return d.nextMusicVoice()
		}
		last := d.voice == len(d.plan.Voices)-1
		if err := s.pauseLoopEnd(last); err != nil {
			return err
		}
		d.voice++
		if !last {
			d.stage = "primary-claim"
		} else {
			d.secondary = 0
			return d.nextSecondaryClaim(state)
		}
	case "secondary-claim":
		target, source := int8(16+d.secondary), int8(11+4*d.secondary)
		if !heldOperation(op, score.OpClaim, score.RoutineOwnership) || op.VoiceSlot != target || op.OtherSlot != source {
			return ErrRunningSequence
		}
		if err := s.spend(20); err != nil {
			return err
		}
		d.secondary++
		return d.nextSecondaryClaim(state)
	case "music-last-mute":
		if !heldWrite(op, score.OpChipWrite, score.Intent{Order: op.IntentOffset, Kind: "psg", Value: 255}) || op.VoiceSlot != 9 {
			return ErrRunningSequence
		}
		if err := s.cost(20, func() error { return d.r.parent.chip.WritePSG(s.clock.Cycle, 255) }); err != nil {
			return err
		}
		if err := s.ordinaryCosts(8, 16); err != nil {
			return err
		}
		d.stage = "dispatch-return"
	case "dispatch-return":
		if !heldOperation(op, score.OpLeave, score.RoutineDispatch) || op.SoundID != d.plan.ID {
			return ErrRunningSequence
		}
		d.done = true
	default:
		return fmt.Errorf("%w: dispatch stage %s operation %s/%s", ErrRunningSequence, d.stage, op.Kind, op.Routine)
	}
	return nil
}

func (d *dispatchInit) stopNext() error {
	if err := d.s.spend(12); err != nil {
		return err
	}
	if err := d.s.pauseLoopEnd(d.scan == 15); err != nil {
		return err
	}
	d.scan++
	d.stage = "stop-scan"
	return nil
}

func (d *dispatchInit) stopTarget(state score.State) int8 {
	channel, _ := heldChannel(state.Voices[d.scan].Channel)
	if channel == 192 || channel == 224 {
		return 17
	}
	return restoredMusicSlot(d.scan)
}

func (d *dispatchInit) stopRestorePrefix(state score.State) error {
	s := d.s
	channel, _ := heldChannel(state.Voices[d.scan].Channel)
	if channel < 128 {
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.branch(channel != 4, false); err != nil {
			return err
		}
		if channel == 4 {
			if state.Voices[16].Active {
				return ErrRunningProfile
			}
			if err := s.spend(12); err != nil {
				return err
			}
			if err := s.branch(true, false); err != nil {
				return err
			}
		}
		if err := s.ordinaryCosts(4, 10, 8, 4, 18, 16); err != nil {
			return err
		}
	} else {
		if err := s.ordinaryCosts(8, 8); err != nil {
			return err
		}
		if err := s.branch(channel == 224, false); err != nil {
			return err
		}
		if channel != 224 {
			if err := s.spend(8); err != nil {
				return err
			}
			if err := s.branch(channel == 192, false); err != nil {
				return err
			}
			if channel != 192 {
				if err := s.ordinaryCosts(12, 8, 18); err != nil {
					return err
				}
			}
		}
	}
	d.stage = "stop-restore"
	return nil
}

func (d *dispatchInit) consumeKey(op score.Operation, state score.State) error {
	if op.VoiceSlot != d.keySlot {
		return ErrRunningSequence
	}
	if d.keyPart == 1 {
		effect, ok := d.key.ExpectedEffect()
		if ok {
			if !heldWrite(op, score.OpChipWrite, keyIntent(effect, op.IntentOffset)) {
				return ErrRunningSequence
			}
			if err := d.key.Apply(effect); err != nil {
				return err
			}
		} else {
			if !heldOperation(op, score.OpParameter, score.RoutineKey) {
				return ErrRunningSequence
			}
		}
		d.keyPart = 2
		return nil
	}
	if d.keyPart != 2 || !heldOperation(op, score.OpLeave, score.RoutineKey) {
		return ErrRunningSequence
	}
	if _, err := d.key.Finish(); err != nil {
		return err
	}
	d.key = nil
	if d.keyCaller == "stop" {
		d.stage = "stop-owner"
		return nil
	}
	if err := d.s.spend(8); err != nil {
		return err
	}
	last := d.keySlot == 6 || d.keySlot == 9
	if err := d.s.pauseLoopEnd(last); err != nil {
		return err
	}
	d.keySlot++
	if d.keySlot == 7 {
		if err := d.s.spend(4); err != nil {
			return err
		}
	}
	if d.keySlot <= 9 {
		d.stage = "music-key"
		return nil
	}
	if err := d.s.spend(16); err != nil {
		return err
	}
	if err := d.s.branch(state.Voices[9].Claimed, false); err != nil {
		return err
	}
	if state.Voices[9].Claimed {
		return ErrRunningProfile
	}
	d.stage = "music-last-mute"
	return nil
}

func (d *dispatchInit) musicFM() int {
	n := 0
	for _, v := range d.plan.Voices {
		if v.Slot < 7 {
			n++
		}
	}
	return n
}

func (d *dispatchInit) nextMusicVoice() error {
	s := d.s
	fm := d.musicFM()
	last := d.voice == fm-1 || d.voice == len(d.plan.Voices)-1
	if err := s.spend(8); err != nil {
		return err
	}
	if err := s.pauseLoopEnd(last); err != nil {
		return err
	}
	d.voice++
	if d.voice == fm {
		if err := s.ordinaryCosts(4, 12); err != nil {
			return err
		}
		empty := fm == len(d.plan.Voices)
		if err := s.branch(empty, false); err != nil {
			return err
		}
		if !empty {
			if err := s.ordinaryCosts(4, 8, 8); err != nil {
				return err
			}
		}
	}
	if d.voice < len(d.plan.Voices) {
		d.stage = "music-voice"
		return nil
	}
	// Primary and secondary stops precede the reset, so these post-load claim
	// scans are the audited inactive profile, with no invented claim effects.
	if err := s.ordinaryCosts(8, 4); err != nil {
		return err
	}
	for i := 0; i < 6; i++ {
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.branch(true, true); err != nil {
			return err
		}
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.pauseLoopEnd(i == 5); err != nil {
			return err
		}
	}
	for i := 0; i < 2; i++ {
		if err := s.spend(12); err != nil {
			return err
		}
		if err := s.branch(true, false); err != nil {
			return err
		}
	}
	if err := s.ordinaryCosts(8, 4); err != nil {
		return err
	}
	d.keySlot, d.keyCaller, d.stage = 1, "music", "music-key"
	return nil
}

func (d *dispatchInit) nextSecondaryClaim(state score.State) error {
	for d.secondary < 2 {
		active := state.Voices[11+4*d.secondary].Active
		if err := d.s.spend(12); err != nil {
			return err
		}
		if err := d.s.branch(!active, false); err != nil {
			return err
		}
		if active {
			d.stage = "secondary-claim"
			return nil
		}
		d.secondary++
	}
	if err := d.s.spend(16); err != nil {
		return err
	}
	d.stage = "dispatch-return"
	return nil
}
