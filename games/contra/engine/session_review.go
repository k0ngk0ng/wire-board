package contra

import "errors"

// BattleReview retains one actual applied battlefield, never a reconstructed or
// predicted image. It is presentation-only persisted metadata: rules, entropy,
// controls and audio continue to their original terminal state independently.
type BattleReview struct {
	Version      uint8          `json:"version"`
	Frame        uint64         `json:"frame,string"`
	Revision     uint64         `json:"revision,string"`
	Level        uint8          `json:"level"`
	Routine      uint8          `json:"routine"`
	ExitSequence uint8          `json:"exitSequence"`
	Lives        [2]uint8       `json:"lives"`
	Presentation PresentedFrame `json:"presentation"`
}

func (s *Session) battleReviewCandidate() bool {
	if s.World.Campaign.Outcome != CampaignOngoing || s.World.Active.Combat.World.GameRoutine != 5 {
		return false
	}
	switch s.World.Active.Combat.World.LevelRoutine {
	case 4, 8, 11:
		// Routine 11 is the actual battlefield's 96-frame game-over delay;
		// the continue/title screen is only installed by the later transition.
		return true
	case 9:
		return s.World.Campaign.EndSequence < 3
	default:
		return false
	}
}

func (s *Session) retainBattleReview(frame, revision uint64, level, routine, exit uint8) {
	view := s.World.Active.Presentation.Snapshot()
	if frame == 0 || view.Display.Busy != 0 || view.Display.AppliedMask&0x18 == 0 {
		return
	}
	visible := false
	for _, color := range view.Palette {
		if color&15 < 13 {
			visible = true
			break
		}
	}
	if !visible {
		return
	}
	// Arrays have value ownership; Snapshot contains no pending writes. Replacing
	// the pointer leaves previously published/checkpointed Session copies intact.
	s.Review = &BattleReview{Version: 1, Frame: frame, Revision: revision, Level: level, Routine: routine, ExitSequence: exit, Lives: s.PresentedLives, Presentation: view}
}

func (s *Session) validateBattleReview() error {
	r := s.Review
	if r == nil {
		return nil
	} // Older records have no captured review; never invent one.
	if r.Version != 1 || r.Frame == 0 || r.Frame > s.Random.Frames || r.Revision == 0 || r.Revision > s.Revision || r.Level >= 8 || len(r.Presentation.Writes) != 0 || r.Presentation.Display.Busy != 0 || r.Presentation.Display.AppliedMask&0x18 == 0 {
		return errors.New("invalid retained battlefield boundary")
	}
	if r.Routine != 4 && r.Routine != 8 && r.Routine != 11 && (r.Routine != 9 || r.ExitSequence >= 3) {
		return errors.New("retained presentation is not a battlefield")
	}
	for _, page := range r.Presentation.Patterns {
		if page >= 128 {
			return errors.New("invalid retained battlefield graphics")
		}
	}
	for _, color := range r.Presentation.Palette {
		if color >= 64 {
			return errors.New("invalid retained battlefield palette")
		}
	}
	return nil
}
