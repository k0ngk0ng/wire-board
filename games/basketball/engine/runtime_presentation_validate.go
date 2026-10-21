package engine

import "fmt"

func validateRuntimePresentation(s RuntimeState) error {
	p := s.Presentation
	if p == nil || s.Config.Campaign == nil || s.Outcome == nil || !s.Outcome.Confirmed || s.Outcome.WinningTeam != 0 || s.AwaitingNextPeriod || s.CampaignPreparation != nil {
		return fmt.Errorf("invalid campaign presentation owner")
	}
	if p.PendingResult.NextStage != CampaignPrepare && p.PendingResult.NextStage != CampaignComplete {
		return fmt.Errorf("unsupported presentation destination")
	}
	var t MatchTransitionState
	if p.Video == nil {
		if p.Complete || s.Transition == nil || s.WaitVideoFrames == 0 || s.FramesUntilLogic != 0 || s.Display.PaletteFade == nil || !s.Display.PaletteFade.Out || uint16(s.Display.PaletteFade.Frame)+s.WaitVideoFrames != 8 {
			return fmt.Errorf("invalid pending result fade")
		}
		legacy := s
		legacy.Presentation = nil
		if e := validateRuntimeOutcome(legacy); e != nil {
			return e
		}
		t = s.Transition.Scene.Transition
		if t.Random != p.PendingResult.Random {
			return fmt.Errorf("pending result random changed before handoff")
		}
	} else {
		v := p.Video
		t = v.Scene.Transition
		if s.Transition != nil || s.Carry != nil || s.Opening != nil || s.Playing != nil || s.WaitVideoFrames != 0 || s.FramesUntilLogic != 0 || s.LogicFrame != v.LogicByte || v.Clock.VideoTicks > s.VideoFrames {
			return fmt.Errorf("invalid presentation stage/counter ownership")
		}
		terminal := v.Clock.Stage == PostResultHandoff || v.Clock.Stage == PostResultReset
		if terminal != p.Complete {
			return fmt.Errorf("presentation completion differs from continuation")
		}
		if v.Resource.Kind == "" {
			switch v.Clock.Stage {
			case PostResultOuterThreshold:
				if v.Clock.Counter >= postResultThreshold(t.Scene) {
					return fmt.Errorf("invalid outer threshold wait")
				}
			case PostResultOuterChange:
				if v.Clock.Counter != v.Clock.ObservedCounter {
					return fmt.Errorf("invalid outer counter-change wait")
				}
			case PostResultHandoff:
				if t.Scene != 2 || t.Match.Phase != 0 || t.Match.Entered || p.PendingResult.NextStage != CampaignPrepare {
					return fmt.Errorf("invalid preparation handoff")
				}
			case PostResultReset:
				if t.Scene != 7 || t.Match.Phase != 7 || !t.Match.Entered || t.Match.Period != 0 || p.PendingResult.NextStage != CampaignComplete {
					return fmt.Errorf("invalid ending reset")
				}
			default:
				return fmt.Errorf("unrestored stable presentation continuation %q", v.Clock.Stage)
			}
		} else {
			if v.Clock.Stage != PostResultPhase || p.Complete {
				return fmt.Errorf("resource wait outside phase continuation")
			}
			if e := validateRuntimePresentationResource(*v); e != nil {
				return e
			}
		}
		if p.PendingResult.NextStage == CampaignPrepare {
			if !(t.Scene == 1 && t.Match.Phase == 7) && !(t.Scene == 2 && t.Match.Phase == 0) {
				return fmt.Errorf("reward presentation changed destination")
			}
		} else if t.Scene != 7 || t.Match.Phase > 10 {
			return fmt.Errorf("ending presentation changed scene")
		}
		if v.Clock.Stage != PostResultReset && t.Match.Period != p.PendingResult.Progress.Period {
			return fmt.Errorf("presentation period changed")
		}
		desired := p.PendingResult.Progress
		if t.Scene == 1 && t.Match.Phase == 7 && !t.Match.Entered {
			desired = *s.Config.Campaign
			desired.Period = p.PendingResult.Progress.Period
			desired.WinBits = p.PendingResult.Progress.WinBits
		}
		if v.Progress != desired {
			return fmt.Errorf("presentation inventory differs from source entry stage")
		}
		if v.Scene.Equipment != s.Config.Match.Equipment {
			return fmt.Errorf("presentation changed equipment")
		}
		grades, e := runtimeConfiguredGrades(s.Config)
		if e != nil {
			return e
		}
		if v.Scene.Grades != grades {
			return fmt.Errorf("presentation changed character grades")
		}
		for i := 0; i < 4; i++ {
			if postResultQueue(v, i).Length > 32 {
				return fmt.Errorf("invalid presentation upload length")
			}
		}
	}
	if t.Scores != s.Outcome.Scores || t.FirstTeamLost || t.Court != s.Config.Match.Court || t.Mode != s.Config.Match.Mode || t.Teams != s.Config.Match.Teams || t.ControllerAssignments != s.Config.Match.ControllerAssignments {
		return fmt.Errorf("presentation changed immutable match identity")
	}
	expectedScene, expectedPhase := uint8(1), MatchPhase(7)
	if s.Config.Campaign.Period+2 == 28 {
		expectedScene, expectedPhase = 7, 0
	}
	if s.Outcome.NextScene != expectedScene || s.Outcome.NextPhase != expectedPhase {
		return fmt.Errorf("immutable initial destination changed")
	}
	originalResult := t
	originalResult.Match.Period = s.Config.Campaign.Period + 2
	originalResult.TeamWinBits = s.Config.Campaign.WinBits
	originalResult.TeamWinBits[(s.Config.Campaign.Period>>1)&1] |= 1 << (s.Config.Match.Teams[1] - 1)
	originalResult.Random = p.PendingResult.Random
	expected, e := campaignResultFromConfirmed(s.Config, originalResult, *s.Outcome)
	if e != nil {
		return e
	}
	if expected != p.PendingResult || t.TeamWinBits != expected.Progress.WinBits {
		return fmt.Errorf("immutable expected settlement changed")
	}
	return nil
}
func validateRuntimePresentationResource(v PostResultVideoState) error {
	q := v.Resource
	switch q.Kind {
	case PostResultFade:
		if q.Level >= 4 || q.Flags != v.Context.Ending.FadeMode || q.Delay != v.Context.Ending.FadeDelay {
			return fmt.Errorf("invalid fade continuation")
		}
		level := uint8(16 + q.Level*16)
		if q.Flags&128 != 0 {
			level = 48 - q.Level*16
		}
		if q.Delay == 0 {
			level = 64
		}
		if v.PaletteBuildLevel != level {
			return fmt.Errorf("fade level differs from cursor")
		}
		switch q.Stage {
		case PostResultPaletteCommit:
			if v.TailUploads[1].Address != 0x3f00 {
				return fmt.Errorf("fade palette is not pending")
			}
		case PostResultPaletteDelay:
			if v.Clock.Counter >= q.Delay || v.TailUploads[1].Address&0x8000 == 0 {
				return fmt.Errorf("invalid committed fade delay")
			}
		default:
			return fmt.Errorf("invalid fade wait")
		}
	case PostResultMessage:
		if q.Stage != PostResultCounterChange || q.ID >= 31 || q.Counter != v.Clock.Counter || q.ID != v.Scene.Transition.Match.OvertimeFlags {
			return fmt.Errorf("invalid message wait")
		}
	case PostResultBackground:
		asset, e := OriginalPresentationBackground(q.ID)
		if e != nil {
			return e
		}
		switch q.Stage {
		case PostResultWaitEntry, PostResultWaitExit:
			if q.Counter != v.Clock.Counter {
				return fmt.Errorf("invalid background counter wait")
			}
		case PostResultBlankEnter, PostResultBlankExit:
			if v.Context.Ending.MachineControl != 0 || v.Control&128 == 0 {
				return fmt.Errorf("invalid blanking handoff")
			}
		case PostResultWork:
			if q.RemainingCycles == 0 || q.RemainingCycles > asset.WorkCycles || v.Control&128 != 0 || v.Mask&0x18 != 0 {
				return fmt.Errorf("invalid disabled resource work")
			}
		default:
			return fmt.Errorf("invalid background continuation")
		}
	default:
		return fmt.Errorf("unknown presentation resource")
	}
	return nil
}
