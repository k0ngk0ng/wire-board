package engine

// observeRuntimeCourtAnimation is the D352 tail, after object projection and
// column preparation. CA58 increments $0300 once per main-loop update; this is
// World.LogicFrame, not the count of NMI/video ticks. Court 0 stores its water
// frame in the same authoritative $06D7 slot used by New York's first passage.
// Court2's side, triggers and cooldowns use $06D7..$06DB. These are aliases of
// the existing canonical court slots, not a second persistent animation copy.
// Court3 shares its gate trigger and countdown with actor0/1 (06D7/06D8).
func observeRuntimeCourtAnimation(s *RuntimeState, w *WorldLoopState, c WorldLoopContext, m MatchState) ([]uint8, error) {
	if c.Actions.CourtID != 0 && c.Actions.CourtID != 2 && c.Actions.CourtID != 3 && c.Actions.CourtID != 4 && c.Actions.CourtID != 5 {
		return nil, nil
	}
	a := &w.Preparation.Actions
	var state CourtAnimationState
	switch c.Actions.CourtID {
	case 0:
		state.WaterFrame = a.Actors[0].Player.NewYorkPassage
	case 2, 5:
		state.AlternatingSide = a.Actors[0].Player.NewYorkPassage
		state.SwitchFlags = [2]uint8{a.Actors[1].Player.NewYorkPassage, a.Actors[2].Player.NewYorkPassage}
		state.SwitchTicks = [2]uint8{a.Actors[3].Player.NewYorkPassage, a.Auxiliary[0].NewYorkPassage}
	case 3:
		state.GateFlags = a.Actors[0].Player.NewYorkPassage
		state.GateTicks = a.Actors[1].Player.NewYorkPassage
	case 4:
		state.PaletteFrame = a.Actors[0].Player.NewYorkPassage
		state.PaletteReverse = a.Actors[1].Player.NewYorkPassage != 0
	}
	step, err := StepCourtAnimation(state, CourtAnimationContext{Court: c.Actions.CourtID, Active: m.Phase != 0 && m.Phase != MatchPaused, VideoFrame: w.LogicFrame, CameraX: uint8(w.Camera.DisplayedX), Random: a.Random, Scores: w.Preparation.Scores})
	if err != nil {
		return nil, err
	}
	display, err := QueueDisplayPatches(s.Display, step.Patches)
	if err != nil {
		return nil, err
	}
	if step.Palette {
		display, err = QueueDisplayPalette(display, step.PaletteID)
		if err != nil {
			return nil, err
		}
	}
	switch c.Actions.CourtID {
	case 0:
		a.Actors[0].Player.NewYorkPassage = step.State.WaterFrame
	case 2, 5:
		a.Actors[0].Player.NewYorkPassage = step.State.AlternatingSide
		a.Actors[1].Player.NewYorkPassage = step.State.SwitchFlags[0]
		a.Actors[2].Player.NewYorkPassage = step.State.SwitchFlags[1]
		a.Actors[3].Player.NewYorkPassage = step.State.SwitchTicks[0]
		a.Auxiliary[0].NewYorkPassage = step.State.SwitchTicks[1]
	case 3:
		a.Actors[0].Player.NewYorkPassage = step.State.GateFlags
		a.Actors[1].Player.NewYorkPassage = step.State.GateTicks
	case 4:
		a.Actors[0].Player.NewYorkPassage = step.State.PaletteFrame
		// Direction is shared with device movement; preserve its actual byte.
	}
	a.Random = step.Random
	w.Preparation.Scores = step.Scores
	s.Display = display
	return step.Sounds, nil
}
