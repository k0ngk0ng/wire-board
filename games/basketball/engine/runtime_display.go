package engine

import "fmt"

// The original dispatcher gives Scene 2 its own menu drawing routine. The
// platform keeps that selection in its room UI, so those menu coordinates must
// never be submitted to the court renderer. D1B5 supplies the next court's
// camera only after unentered Scene 1/phase 0 completes.
func runtimeHasCourtDisplaySource(s RuntimeState) bool {
	if s.Presentation != nil {
		return false
	}
	if s.Playing != nil {
		return true
	}
	var o *OpeningState
	if s.Opening != nil {
		o = s.Opening
	} else if s.Transition != nil {
		o = &s.Transition.Scene
	}
	if o == nil || o.Transition.Scene != 1 {
		return false
	}
	return o.Transition.Match.Phase != 0 || o.Transition.Match.Entered
}

func runtimeDisplaySource(s RuntimeState) DisplaySource {
	if s.Opening != nil {
		return DisplaySourceFromOpening(*s.Opening)
	}
	if s.Playing != nil {
		return DisplaySourceFromWorld(s.Playing.World, s.Playing.Context, s.Playing.Match)
	}
	if s.Transition != nil {
		return DisplaySourceFromOpening(s.Transition.Scene)
	}
	return DisplaySource{}
}
func observeRuntimeWorldDisplay(s *RuntimeState, step WorldLoopStep, m MatchState) error {
	next, err := ObserveDisplayMotionForCourt(s.Display, step.MotionSurfaceWrites, s.Config.Match.Court)
	if err != nil {
		return err
	}
	s.Display = next
	if step.StreamColumn {
		next, err = QueueDisplayColumn(s.Display, step.State.Camera.TileColumn, uint8(m.Phase))
		if err != nil {
			return err
		}
		s.Display = next
	}
	return nil
}
func resetRuntimeCourtDisplay(s *RuntimeState) error {
	next := ResetDisplayCourt(s.Display)
	next, err := InitializeOriginalDisplayHUD(next, s.Config.Match, *runtimeMatch(s))
	if err != nil {
		return err
	}
	s.Display = next
	return nil
}
func validateRuntimeDisplay(s DisplayState) error {
	if err := validateDisplayPaletteFade(s); err != nil {
		return err
	}
	if s.CourtEpoch == 0 || s.CourtEpoch > 1<<53-1 || s.CourtSerial > 1<<53-1 || len(s.CourtColumns) > 32 {
		return fmt.Errorf("invalid runtime display epoch or columns")
	}
	var seen [32]bool
	for _, c := range s.CourtColumns {
		if c.Column >= 64 || c.Phase >= 128 || seen[c.Column&31] {
			return fmt.Errorf("invalid display column history")
		}
		seen[c.Column&31] = true
	}
	if s.PendingColumn != nil && (s.PendingColumn.Column >= 64 || s.PendingColumn.Phase >= 128) {
		return fmt.Errorf("invalid pending display column")
	}
	for _, snapshot := range []BasketballRenderSnapshot{s.Submitted, s.Visible} {
		if len(snapshot.DrawOrder) > 13 {
			return fmt.Errorf("invalid display draw count")
		}
		for _, object := range snapshot.DrawOrder {
			if object < 0 || object >= 13 {
				return fmt.Errorf("invalid displayed object")
			}
		}
	}
	for _, snapshot := range []BasketballRenderSnapshot{s.Submitted, s.Visible} {
		if !validDisplayCourtPalette(snapshot.Court, snapshot.PaletteID) {
			return fmt.Errorf("invalid snapshot court palette")
		}
	}
	if s.PaletteID > 15 || s.PendingPaletteID > 15 {
		return fmt.Errorf("invalid display palette")
	}
	if s.HasVisible && !validDisplayCourtPalette(s.Visible.Court, s.PaletteID) {
		return fmt.Errorf("unavailable visible court palette")
	}
	court := s.Visible.Court
	if s.HasSubmission {
		court = s.Submitted.Court
	}
	if (s.HasVisible || s.HasSubmission) && !validDisplayCourtPalette(court, s.PendingPaletteID) {
		return fmt.Errorf("unavailable pending court palette")
	}
	if s.HUD.Clock.Seconds >= 60 || s.HUD.Clock.Minutes > 9 {
		return fmt.Errorf("invalid displayed clock")
	}
	return validateRuntimeCourtAnimationDisplay(s)
}

// BCC1/BCC3 installs the fourth sprite CHR page after result background loads.
// F049 subsequently retains it for raw phase 0x86. Court-ring history survives
// because the result HUD only replaces the original top six rows.
func initializeRuntimeResultDisplay(original DisplayState) DisplayState {
	s := original
	s.BallCHR = 13
	return s
}

func validDisplayCourtPalette(court, palette uint8) bool {
	if court > 7 {
		return false
	}
	if palette == 0 || palette == court+1 {
		return true
	}
	return court == 4 && (palette == 9 || palette == 10) || court == 6 && palette >= 11 && palette <= 15
}
