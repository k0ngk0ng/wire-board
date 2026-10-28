package contra

import "errors"

// Validate checks the saved-session envelope without executing rules. The
// transport additionally binds it to the expected room epoch and engine build.
func (s *Session) Validate() error {
	if s == nil || s.Version != 1 || s.Audio.Version != 1 || s.Audio.Epoch == "" {
		return errors.New("invalid Contra session version or epoch")
	}
	if err := s.Random.adapter(s.World.Active.Random).Validate(); err != nil {
		return err
	}
	if err := s.World.Campaign.Validate(&s.World.Active.Combat.World, true); err != nil {
		return err
	}
	if s.FinalFlushTicks > 5 || s.Finalized != (s.FinalFlushTicks != 0) || s.Finalized && s.World.Campaign.Outcome == CampaignOngoing {
		return errors.New("inconsistent terminal presentation state")
	}
	if s.PresentationTicks > ^uint64(0)-s.Random.Frames {
		return errors.New("session clock overflow")
	}
	clock := s.Random.Frames + s.PresentationTicks
	if uint64(s.FinalFlushTicks) > ^uint64(0)-clock || s.Audio.Tick != clock+uint64(s.FinalFlushTicks) || s.World.Campaign.HasTick != (s.Random.Frames != 0) || s.World.Campaign.HasTick && s.World.Campaign.LastTick != s.Random.Frames || s.Revision < clock {
		return errors.New("inconsistent Contra session clocks")
	}
	if s.ResumeRequested && (!s.World.Campaign.Paused || s.World.Campaign.Outcome != CampaignOngoing) {
		return errors.New("invalid pending resume request")
	}
	p := &s.World.Active.Presentation
	if _, err := p.Queue.Encode(); err != nil {
		return err
	}
	used := int(p.Queue.Used) + int(p.DiscardedGraphicsBytes)
	if p.Terminated {
		used++
	}
	if used > GraphicsBufferCapacity || p.Display.Busy > 5 {
		return errors.New("invalid pending presentation bounds")
	}
	if p.Palette.RequestedColors != 0 && p.Palette.RequestedColors != 16 && p.Palette.RequestedColors != 32 {
		return errors.New("invalid pending palette length")
	}
	if s.Finalized && (p.Terminated || p.Queue.Used != 0 || p.DiscardedGraphicsBytes != 0 || p.Display.Busy != 0 || p.Palette.RequestedColors != 0 || len(s.Audio.Pending) != 0) {
		return errors.New("final session has unsubmitted work")
	}
	return s.validateBattleReview()
}
