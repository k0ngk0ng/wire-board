package engine

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

// OriginalRewardPhaseStep stops at BE63, before its three actor updates.
// Progress is the live reward inventory; the immutable confirmed match result
// stays with the caller. The entry latch prevents a second award after restore.
type OriginalRewardPhaseStep struct {
	State                                OpeningState
	Progress                             campaign.Progress
	SoundIDs, Messages, Backgrounds      []uint8
	CHRPages                             [2]uint8
	ChangeCHR, FadeIn, FadeOut, Complete bool
}

func StepOriginalRewardPhase(original OpeningState, progress campaign.Progress, pressed uint8) (OriginalRewardPhaseStep, error) {
	r := OriginalRewardPhaseStep{State: original, Progress: progress}
	t := &r.State.Transition
	if t.Scene != 1 || t.Match.Phase != 7 || t.Mode&128 != 0 || t.FirstTeamLost || t.Match.Period >= 28 {
		return r, fmt.Errorf("reward requires original campaign victory phase")
	}
	if err := progress.Validate(); err != nil {
		return r, err
	}
	if progress.Mode != t.Mode || progress.Period != t.Match.Period || progress.WinBits != t.TeamWinBits {
		return r, fmt.Errorf("reward progress differs from confirmed match")
	}
	if !t.Match.Entered {
		item, err := campaign.RewardItem(progress.RewardSet, t.Teams[1], progress.Period)
		if err != nil {
			return r, err
		}
		t.Match.Entered = true
		for i, held := range r.Progress.Inventory {
			if held == campaign.Empty {
				r.Progress.Inventory[i] = item
				break
			}
		}
		r.Backgrounds = []uint8{26}
		t.ResultTimer = 0
		r.ChangeCHR, r.CHRPages, r.FadeIn = true, [2]uint8{0x3b, 0x3b}, true
		return r, nil
	}
	t.ResultTimer++
	if t.ResultTimer != 112 {
		if t.ResultTimer == 40 {
			r.Messages = []uint8{28, 30, 29}
			// 06F7 is the in-game overtime flag and the menu message selector.
			t.Match.OvertimeFlags = 29
		}
		var value uint8
		t.Random, value = NextRandom(t.Random)
		if value&31 == 0 {
			r.SoundIDs = []uint8{50}
		}
		if pressed&0x90 == 0 {
			return r, nil
		}
	}
	r.FadeOut, r.Complete = true, true
	// The result temporarily borrowed character slots for reward performers.
	t.Characters[0] = t.Match.Clock.ScoreLowCache[0]
	t.Characters[2] = t.Match.Clock.DisplayedSecond
	t.Scene, t.Match.Phase, t.Match.Entered = 2, 0, false
	return r, nil
}
