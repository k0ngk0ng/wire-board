package engine

import "fmt"

func advancePostResultReward(r *PostResultVideoStep) error {
	s := &r.State
	t := &s.Scene.Transition
	switch s.Clock.Resume {
	case 0:
		if t.FirstTeamLost || t.Mode&128 != 0 || t.Match.Period >= 28 {
			return fmt.Errorf("reward video requires campaign victory")
		}
		if !s.Clock.OriginEntered {
			// Reuse the independently source-tested award transaction before yielding.
			entry, e := StepOriginalRewardPhase(s.Scene, s.Progress, uint8(s.Clock.Input[0].Pressed))
			if e != nil {
				return e
			}
			s.Progress = entry.Progress
			t.Match.Entered = true
			s.Clock.SavedPalette = s.Context.Ending.Actor.DisplayMask
			s.Clock.SavedFieldCHR = s.FieldCHR
			return beginPostResultBackground(r, 26, 1)
		}
		t.ResultTimer++
		if t.ResultTimer == 112 {
			return beginPostResultFade(r, 128, s.Context.Ending.FadeDelay, 6)
		}
		if t.ResultTimer == 40 {
			return beginPostResultMessage(r, 28, 3)
		}
		s.Clock.Resume = 5
	case 1:
		s.Context.Ending.Actor.DisplayMask = s.Clock.SavedPalette
		s.FieldCHR = s.Clock.SavedFieldCHR
		t.ResultTimer = 0
		s.Context.Ending.CHRPages[1], s.Context.Ending.CHRPages[2] = 59, 59
		// Reward source stores these pages; the next normal NMI commits them.
		return beginPostResultFade(r, 0, s.Context.Ending.FadeDelay, 2)
	case 2:
		s.Clock.Stage = "actors"
	case 3:
		return beginPostResultMessage(r, 30, 4)
	case 4:
		return beginPostResultMessage(r, 29, 5)
	case 5:
		var value uint8
		t.Random, value = NextRandom(t.Random)
		if value&31 == 0 {
			postResultSound(r, 50)
		}
		if s.Clock.Input[0].Pressed&(ButtonA|ButtonStart) != 0 {
			return beginPostResultFade(r, 128, s.Context.Ending.FadeDelay, 6)
		}
		s.Clock.Stage = "actors"
	case 6:
		t.Characters[0] = t.Match.Clock.ScoreLowCache[0]
		t.Characters[2] = t.Match.Clock.DisplayedSecond
		t.Scene, t.Match.Phase, t.Match.Entered = 2, 0, false
		s.Clock.Stage = "actors"
	default:
		return fmt.Errorf("unknown reward continuation%d", s.Clock.Resume)
	}
	return nil
}
