package contra

import "errors"

// RequestResume is a control transaction, not a physics tick. The host must
// authorize it through Apply before changing its paused envelope. The next
// ordinary frame consumes the request after the original paused visual prefix.
func (s *Session) RequestResume() error {
	if err := s.Validate(); err != nil {
		return err
	}
	if !s.World.Campaign.Paused || s.World.Campaign.Outcome != CampaignOngoing || s.World.Active.Combat.World.LevelRoutine != 4 {
		return errors.New("campaign is not awaiting resume")
	}
	if s.ResumeRequested {
		return nil
	}
	if s.Revision == ^uint64(0) {
		return errors.New("session revision exhausted")
	}
	s.ResumeRequested = true
	s.Revision++
	return nil
}

// PresentPaused is the opt-in host presentation timer. It consumes no pads,
// entropy or campaign work, and cannot authorize resume. Only native display
// counters, sprite projections, CHR/palette output and audio advance.
func (s *Session) PresentPaused(resources *LevelResources) (SessionFrame, error) {
	if err := s.Validate(); err != nil {
		return SessionFrame{}, err
	}
	if resources == nil || !s.World.Campaign.Paused || s.ResumeRequested || s.World.Campaign.Outcome != CampaignOngoing || s.World.Active.Combat.World.LevelRoutine != 4 {
		return SessionFrame{}, errors.New("session has no paused presentation tick")
	}
	if s.Revision == ^uint64(0) || s.Audio.Tick == ^uint64(0) || s.PresentationTicks == ^uint64(0) {
		return SessionFrame{}, errors.New("presentation clock exhausted")
	}
	next := *s
	next.Audio = s.Audio.Clone()
	next.World.Active.Presentation.Queue = s.World.Active.Presentation.Queue.Clone()
	if _, err := next.World.Active.Presentation.CommitWithScroll(next.World.Active.Combat.World.Scroll); err != nil {
		return SessionFrame{}, err
	}
	next.commitPublicLives()
	next.Audio.Context.EndPlayersAlive = next.World.Campaign.AliveAtExit
	audio := next.Audio.Advance(true)
	if len(audio.UnsupportedIDs) != 0 {
		return SessionFrame{}, errors.New("unresolved paused audio")
	}
	next.World.Active.Combat.World.Frame++
	next.World.Active.projectPausedSprites()
	if err := next.World.Active.animate(resources); err != nil {
		return SessionFrame{}, err
	}
	if err := next.World.Active.compose(); err != nil {
		return SessionFrame{}, err
	}
	next.PresentationTicks++
	if s.battleReviewCandidate() {
		next.retainBattleReview(next.Random.Frames, next.Revision+1, s.World.Active.Combat.World.Level, s.World.Active.Combat.World.LevelRoutine, s.World.Campaign.EndSequence)
	}
	next.Revision++
	*s = next
	return SessionFrame{Presentation: s.World.Active.Presentation.Snapshot(), Audio: audio}, nil
}
