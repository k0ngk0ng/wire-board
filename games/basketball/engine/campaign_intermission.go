package engine

import "fmt"

// commitOriginalCampaignRoster is bank3 B2EF..B32D, before the four C033 actor
// updates. The original pending roster occupies the same bytes as the match
// HUD cache. Grade calculation must follow that commit, not preview characters.
func commitOriginalCampaignRoster(original OpeningState) (OpeningState, error) {
	if original.Transition.Scene != 6 {
		return original, fmt.Errorf("campaign roster commit requires the confirmed travel scene")
	}
	s := original
	t := &s.Transition
	t.Characters = [4]uint8{t.Match.Clock.ScoreLowCache[0], t.Match.Clock.ScoreLowCache[1], t.Match.Clock.DisplayedSecond, t.Match.Clock.HUDTurn}
	graded, err := InitializeOpeningGrades(s)
	if err != nil {
		return original, err
	}
	s = graded.State
	t = &s.Transition
	for i := 0; i < 6; i++ {
		o := &t.Objects[i]
		periodSetAction(o, 0)
		o.Body.Velocity.X = Fixed(uint8(o.Body.Velocity.X))
		o.Body.Position.X &= 65535
		t.Passages[i] = 0
	}
	t.Scene = 1
	t.Match.Phase, t.Match.Entered = 0, false
	t.EndActorFlags = [4]uint8{}
	t.SelectedController = 0
	t.ResultControllers = [2]int8{}
	t.LastShooter = 0
	s.Projection.Projected[6].Y = int16(uint16(s.Projection.Projected[6].Y)&0xff00 | 0xf0)
	return s, nil
}

// confirmCampaignIntermission applies committed platform choices without
// simulating the intervening menu waits. The caller has already advanced the
// outer-loop RNG; source member confirmation performs one additional draw.
// Both configuration and scene are returned together, with no writes on error.
func confirmCampaignIntermission(c RuntimeConfig, original OpeningState) (RuntimeConfig, OpeningState, error) {
	if c.Campaign == nil || original.Transition.Scene != 2 || original.Transition.Match.Phase != 4 || original.Transition.Mode != c.Match.Mode {
		return c, original, fmt.Errorf("campaign confirmation requires pending member selection")
	}
	if err := validateRuntimeConfig(c); err != nil {
		return c, original, err
	}
	roster, err := SelectOriginalCampaignRoster(c.Match.Mode, [2]uint8{c.Match.Characters[0], c.Match.Characters[1]}, original.Transition.Random)
	if err != nil {
		return c, original, err
	}
	s := original
	t := &s.Transition
	// 89AC's default menu cleanup is broader than B2EF's six-object reset.
	t.Kinds[5] = 0
	t.Passages = [9]uint8{}
	t.Random = roster.Random
	t.Match.Clock.ScoreLowCache = [2]uint8{roster.Characters[0], roster.Characters[1]}
	t.Match.Clock.DisplayedSecond, t.Match.Clock.HUDTurn = roster.Characters[2], roster.Characters[3]
	// The platform has already committed the current tactics. Its preparation
	// replaces original menu/travel waits, reaching the B2EF commit boundary.
	t.Scene, t.Match.Phase, t.Match.Entered = 6, 3, true
	s, err = commitOriginalCampaignRoster(s)
	if err != nil {
		return c, original, err
	}
	for i := uint8(0); i < 4; i++ {
		// Commit sets action0, whose original tail has no sound requests.
		if _, err = stepOriginalResultActor(&s, i); err != nil {
			return c, original, err
		}
	}
	updated := c
	updated.Match.Characters = roster.Characters
	return updated, s, nil
}
