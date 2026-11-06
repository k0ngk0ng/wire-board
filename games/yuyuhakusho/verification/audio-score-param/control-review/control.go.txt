package score

import "fmt"

type MusicControlAction uint8

const (
	ActionStopMusicDACFlags MusicControlAction = iota + 1
	ActionFadeMusicStep
	ActionResetScore
)

func validMusicControl(action MusicControlAction) bool {
	return action >= ActionStopMusicDACFlags && action <= ActionResetScore
}

// ApplyMusicControlObserved applies one score-owned action with no driver tick,
// tempo or normal voice advancement. Root fields and 04 hardware IO are separate.
func (d *Driver) ApplyMusicControlObserved(action MusicControlAction, hooks ObservedHooks) (Output, error) {
	if !validMusicControl(action) {
		return Output{}, fmt.Errorf("unsupported music control action")
	}
	if err := d.validateObservedHooks(hooks); err != nil {
		return Output{}, err
	}
	tx, err := d.beginTransaction(&hooks, DACFeedback{}, stageAction)
	if err != nil {
		return Output{}, err
	}
	if err = tx.checkResult(tx.engine.applyMusicControl(action)); err != nil {
		return Output{}, err
	}
	d.state = tx.working.state
	d.lease = nil
	tx.stage = stageClosed
	return tx.engine.out, nil
}

func (t *tick) applyMusicControl(action MusicControlAction) error {
	if !validMusicControl(action) {
		return fmt.Errorf("unsupported music control action")
	}
	branch := BranchStopMusicDACFlags
	if action == ActionFadeMusicStep {
		branch = BranchFadeMusicStep
	} else if action == ActionResetScore {
		branch = BranchResetScore
	}
	leave := t.enter(RoutineMusicControl, -1, branch)
	switch action {
	case ActionStopMusicDACFlags:
		t.operation(Operation{Kind: OpParameter, Routine: RoutineMusicControl, VoiceSlot: 0, OtherSlot: -1, Branch: branch, Action: action})
		if t.err == nil {
			v := &t.driver.state.Voices[0]
			v.Active = false
			v.NoteOff = false
			v.Claimed = false
			v.Tie = false
		}
	case ActionFadeMusicStep:
		for i := 1; i <= 9; i++ {
			t.operation(Operation{Kind: OpVoiceScan, Routine: RoutineMusicControl, VoiceSlot: int8(i), OtherSlot: -1, Branch: branch, Action: action})
			if t.err != nil {
				break
			}
			v := &t.driver.state.Voices[i]
			if !v.Active {
				continue
			}
			next := v.BaseLevel + 1
			t.operation(Operation{Kind: OpCounter, Routine: RoutineMusicControl, VoiceSlot: int8(i), OtherSlot: -1, Branch: branch, Action: action, Counter: CounterBaseLevel, Before: uint16(v.BaseLevel), After: uint16(next)})
			if t.err != nil {
				break
			}
			v.BaseLevel = next
			if !isPSG(i) {
				if int8(next) < 0 {
					v.Active = false
				} else if err := t.level(i); err != nil {
					t.fail(err)
				}
			} else if next >= 16 {
				v.Active = false
			} else {
				t.baseVolume(i)
			}
			if t.err != nil {
				break
			}
		}
	case ActionResetScore:
		t.operation(Operation{Kind: OpScoreReset, Routine: RoutineMusicControl, VoiceSlot: -1, OtherSlot: -1, Branch: branch, Action: action, Iterations: 18})
		if t.err == nil {
			t.driver.state = State{Ticks: t.driver.state.Ticks}
		}
	}
	leave()
	return t.err
}

// baseVolume is the original immediate attenuation service, not envelope.Next.
func (t *tick) baseVolume(i int) {
	v := &t.driver.state.Voices[i]
	if v.NoteOff || v.Claimed || v.Tie && v.GateTicks != 0 && v.GateRemaining == 0 {
		return
	}
	t.psgFor(i, (v.BaseLevel|channelCode(v.Channel))+0x10)
}
