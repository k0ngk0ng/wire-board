package contra

import "errors"

// FlushTerminal drains presentation/audio before the host publishes Result and
// stops simulation timers. It never steps controls, entropy, physics or campaign
// decisions. Display blanking lasts at most the recovered five boundaries; a
// busy queue can defer palette submission to the next boundary.
func (s *Session) FlushTerminal() (SessionFrame, error) {
	if err := s.Validate(); err != nil {
		return SessionFrame{}, err
	}
	if s == nil || s.Version != 1 || s.Audio.Version != 1 || s.Audio.Epoch == "" || s.World.Campaign.Outcome == CampaignOngoing {
		return SessionFrame{}, errors.New("session has no final outcome")
	}
	if err := s.World.Campaign.Validate(&s.World.Active.Combat.World, true); err != nil {
		return SessionFrame{}, err
	}
	if s.Finalized {
		return SessionFrame{Duplicate: true, Finalized: true, Presentation: s.World.Active.Presentation.Snapshot(), Audio: AudioUpdate{View: s.Audio.View()}}, nil
	}
	if s.FinalFlushTicks != 0 || s.Audio.Tick != s.Random.Frames+s.PresentationTicks || s.World.Active.Presentation.Display.Busy > 5 || s.Revision == ^uint64(0) {
		return SessionFrame{}, errors.New("invalid terminal boundary state")
	}
	next := *s
	next.Audio = s.Audio.Clone()
	next.World.Active.Presentation.Queue = s.World.Active.Presentation.Queue.Clone()
	var events []AudioEvent
	for i := 0; i < 5; i++ {
		if next.Audio.Tick == ^uint64(0) {
			return SessionFrame{}, errors.New("audio clock exhausted")
		}
		if _, err := next.World.Active.Presentation.CommitWithScroll(next.World.Active.Combat.World.Scroll); err != nil {
			return SessionFrame{}, err
		}
		next.commitPublicLives()
		next.Audio.Context.EndPlayersAlive = next.World.Campaign.AliveAtExit
		update := next.Audio.Advance(next.World.Campaign.Paused)
		if len(update.UnsupportedIDs) != 0 {
			return SessionFrame{}, errors.New("unresolved terminal audio")
		}
		events = append(events, update.Events...)
		next.FinalFlushTicks++
		if next.World.Active.Presentation.Palette.RequestedColors == 0 && next.World.Active.Presentation.Display.Busy == 0 {
			next.Finalized = true
			next.Revision++
			*s = next
			return SessionFrame{Finalized: true, Presentation: s.World.Active.Presentation.Snapshot(), Audio: AudioUpdate{Events: events, View: s.Audio.View()}}, nil
		}
	}
	return SessionFrame{}, errors.New("terminal display did not settle")
}
