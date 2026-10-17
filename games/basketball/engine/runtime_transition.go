package engine

import "fmt"

// This adapter copies semantic fields into the independently verified period
// transition component. It does not reconstruct RAM or load a fixture.
func runtimeOpeningFromPlaying(p RuntimePlayingState, c RuntimeConfig) (OpeningState, error) {
	o, err := NewOriginalMatch(c.Match)
	if err != nil {
		return o, err
	}
	t := &o.Transition
	w := p.World.Preparation.Actions
	prep := p.World.Preparation
	t.Match = p.Match
	t.Random = w.Random
	t.Scores = prep.Scores
	t.Kinds = worldObjectKinds(w, p.Context.ObjectKinds)
	t.PlayerControls = p.Context.Actions.Controls
	t.ResultExitFlag = p.ResultExitFlag
	t.TeamWinBits = p.TeamWinBits
	t.CameraScroll = [4]uint16{p.World.Camera.DisplayedX, p.World.DisplayedCameraY, p.World.Camera.X, p.World.Camera.Y}
	t.DisplayFlags = prep.DisplayFlags
	t.EndCounter = p.World.Camera.Shake
	t.TrackedObject = p.Context.TrackedObject
	t.SpecialHolder = w.SpecialHolder
	t.AuxiliaryLinks = [2]int8{w.Auxiliary[0].ObjectLink, w.Auxiliary[1].ObjectLink}
	t.LastShooter = uint8(w.LastShooter)
	t.ResultControllers = [2]int8{int8(prep.PreviousBallAction), int8(prep.ObservedBallAction)}
	for i, a := range w.Actors {
		q := a.Player
		t.Objects[i] = PeriodObject{Body: q.Body, Action: uint8(q.Action), Entered: q.Entered, Request: q.Request, Pose: q.Pose, Animation: q.Animation, Airborne: q.Airborne, FacingLeft: q.FacingLeft}
		t.Endurance[i] = a.Endurance
		t.ShotCycle[i] = a.ShotCycle
		t.Passages[i] = q.NewYorkPassage
		t.AIModes[i] = uint8(prep.TeamAI.Players[i].Mode)
		if prep.TeamAI.Players[i].Entered {
			t.AIModes[i] |= 128
		}
		t.AIPlans[i] = uint8(prep.TeamAI.Players[i].Plan)
		o.Links[i] = w.PlayerLinks[i]
		o.InputDelays[i] = uint8(p.World.Input.Memory[i].LastAttackTap)
		o.InputDelays[i+4] = uint8(p.World.Input.Memory[i].LastTap)
		o.InputDelays[i+8] = uint8(p.World.Input.Memory[i].PendingButtons)
	}
	for i, a := range w.Auxiliary {
		t.Objects[i+4] = PeriodObject{Body: a.Body, Action: a.Action, Entered: a.Entered, Request: a.Request, Pose: a.Pose, Animation: a.Animation, Airborne: a.Airborne, FacingLeft: a.FacingLeft}
		t.Owners[i] = uint8(a.Owner)
		o.Links[i+4] = a.Attachment
		t.Passages[i+4] = a.NewYorkPassage
		t.TeamAI[i] = prep.TeamAI.Teams[i].Mode
		if prep.TeamAI.Teams[i].Entered {
			t.TeamAI[i] |= 128
		}
	}
	setOpeningBall(&o, w.Ball)
	t.Owners[2] = uint8(w.Holder)
	o.Links[6] = w.BallLink
	o.ExcludedCatcher = w.ExcludedCatcher
	for i, h := range w.Hoops {
		t.Objects[i+7] = PeriodObject{Body: Body{Position: h.Position, Velocity: h.Velocity, Gravity: h.Gravity}, Action: uint8(h.Action), Entered: h.Entered, Request: ActionRequest{Action: Action(h.Request.Action), Pending: h.Request.Pending}, Pose: h.Pose, Animation: h.Animation, Airborne: h.Airborne, FacingLeft: h.FacingLeft}
		t.Owners[i+3] = h.Owner
		t.HoopDurability[i] = h.Durability
		o.Links[i+7] = p.Context.HoopLinks[i]
	}
	t.Passages[6], t.Passages[7] = prep.CourtScoreFlags[0], prep.CourtScoreFlags[1]
	o.MountFlags = w.MountFlags
	o.DetachedCount = w.DetachedCount
	o.ScoreSoundDelay = prep.ScoreSoundDelay
	o.TacticCache = p.TacticCache
	o.Projection = p.World.Projection
	return o, nil
}

// The original between-period reset does not zero whole actors or all AI/input
// memory. Start with preserved fields and overwrite only initialization-owned
// fields supplied by the completed scene. This is important for stale timers,
// cached zones and action parameters that the next real input phase handles.
func mergeRuntimeCarriedWorld(fresh, carry WorldLoopState) WorldLoopState {
	result := carry
	a := &result.Preparation.Actions
	n := fresh.Preparation.Actions
	for i := range a.Actors {
		heading := a.Actors[i].Player.Heading
		a.Actors[i].Player = n.Actors[i].Player
		a.Actors[i].Player.Heading = heading
		a.Actors[i].ShotCycle = n.Actors[i].ShotCycle
		a.Actors[i].Endurance = n.Actors[i].Endurance
	}
	a.Auxiliary = n.Auxiliary
	a.Ball = n.Ball
	for i := range a.Hoops {
		lock := a.Hoops[i].ScoreLock
		a.Hoops[i] = n.Hoops[i]
		a.Hoops[i].ScoreLock = lock
	}
	a.PlayerLinks, a.BallLink = n.PlayerLinks, n.BallLink
	a.Holder, a.ExcludedCatcher, a.SpecialHolder = n.Holder, n.ExcludedCatcher, n.SpecialHolder
	a.Random = n.Random
	a.MountFlags, a.DetachedCount = n.MountFlags, n.DetachedCount
	a.LastShooter = n.LastShooter
	for i := range result.Preparation.TeamAI.Teams {
		result.Preparation.TeamAI.Teams[i].Mode = fresh.Preparation.TeamAI.Teams[i].Mode
		result.Preparation.TeamAI.Teams[i].Entered = fresh.Preparation.TeamAI.Teams[i].Entered
	}
	for i := range result.Preparation.TeamAI.Players {
		p := &result.Preparation.TeamAI.Players[i]
		q := fresh.Preparation.TeamAI.Players[i]
		p.Mode, p.Entered, p.Plan = q.Mode, q.Entered, q.Plan
	}
	result.Preparation.Scores = fresh.Preparation.Scores
	result.Preparation.ScoreSoundDelay = fresh.Preparation.ScoreSoundDelay
	result.Preparation.PreviousBallAction = fresh.Preparation.PreviousBallAction
	result.Preparation.ObservedBallAction = fresh.Preparation.ObservedBallAction
	result.Preparation.DisplayFlags = fresh.Preparation.DisplayFlags
	result.Preparation.CourtScoreFlags = fresh.Preparation.CourtScoreFlags
	for i := range result.Input.Memory {
		result.Input.Memory[i].LastAttackTap = fresh.Input.Memory[i].LastAttackTap
		result.Input.Memory[i].LastTap = fresh.Input.Memory[i].LastTap
		result.Input.Memory[i].PendingButtons = fresh.Input.Memory[i].PendingButtons
	}
	result.Camera, result.Projection = fresh.Camera, fresh.Projection
	result.LaunchColumn = fresh.LaunchColumn
	result.LogicFrame = fresh.LogicFrame
	result.DisplayedCameraY = fresh.DisplayedCameraY
	result.Started = false
	result.Updates = 0
	return result
}

// Result destinations deliberately retain the original mutated selections.
// Validate those exact mutations instead of relaxing all configuration checks.
func validateRuntimeOutcome(s RuntimeState) error {
	if s.Outcome == nil {
		return nil
	}
	out := s.Outcome
	if s.Transition == nil || out.WinningTeam > 1 {
		return fmt.Errorf("invalid runtime result stage")
	}
	t := s.Transition.Scene.Transition
	if out.Scores != t.Scores || out.WinningTeam != func() uint8 {
		if t.FirstTeamLost {
			return 1
		}
		return 0
	}() {
		return fmt.Errorf("outcome differs from original result")
	}
	if !out.Confirmed {
		if t.Match.Phase != 6 || !t.Match.Entered || out.NextScene != 0 || out.NextPhase != 0 {
			return fmt.Errorf("invalid unconfirmed result")
		}
		return nil
	}
	if out.NextScene != t.Scene || out.NextPhase != t.Match.Phase || t.Match.Entered || s.AwaitingNextPeriod {
		return fmt.Errorf("invalid result destination")
	}
	expected := s.Config.Match
	switch {
	case expected.Mode&128 != 0:
		if t.Scene != 2 || t.Match.Phase != 2 {
			return fmt.Errorf("invalid versus result destination")
		}
		expected.ControllerAssignments = [4]int8{-1, -1, -1, -1}
		if t.ResultExitFlag < 0 {
			expected.Mode = 129
		}
	case t.FirstTeamLost || t.Match.Period == 28:
		if t.Scene != 7 || t.Match.Phase != 0 {
			return fmt.Errorf("invalid campaign ending destination")
		}
	default:
		if t.Scene != 1 || t.Match.Phase != 7 {
			return fmt.Errorf("invalid reward destination")
		}
		expected.Characters[0], expected.Characters[2] = 0, t.Teams[1]
	}
	if t.Court != expected.Court || t.Teams != expected.Teams || t.Mode != expected.Mode || t.Characters != expected.Characters || t.ControllerAssignments != expected.ControllerAssignments || s.Transition.Scene.Equipment != expected.Equipment {
		return fmt.Errorf("invalid result configuration mutations")
	}
	grades, err := runtimeConfiguredGrades(s.Config)
	if err != nil {
		return err
	}
	if s.Transition.Scene.Grades != grades {
		return fmt.Errorf("result changed character grades")
	}
	return nil
}
