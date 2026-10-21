package basketball

import "fmt"

// Frames and input packets remain in memory. Commit before publishing a new
// phase, period, campaign round or an accepted preparation revision.
func (s State) checkpointKey() string {
	var period uint8
	switch {
	case s.Runtime.Playing != nil:
		period = s.Runtime.Playing.Match.Period
	case s.Runtime.Transition != nil:
		period = s.Runtime.Transition.Scene.Transition.Match.Period
	case s.Runtime.Opening != nil:
		period = s.Runtime.Opening.Transition.Match.Period
	case s.Runtime.Presentation != nil:
		period = s.Runtime.Presentation.PendingResult.Progress.Period
	}
	var round, revision, preparation uint64
	if s.Campaign != nil {
		round, revision = s.Campaign.Round, s.Campaign.Revision
	}
	if s.Runtime.CampaignPreparation != nil {
		preparation = s.Runtime.CampaignPreparation.Revision
	}
	return fmt.Sprintf("%s/%s/%d/%d/%d/%d/%d", s.Epoch, s.phase(), period, round, revision, preparation, s.PresentationConfirmAt)
}
