package engine

import "fmt"

func displayPose(p Pose) uint8 {
	v := p.Frame
	if p.Mirrored {
		v |= 128
	}
	return v
}
func DisplaySourceFromOpening(s OpeningState) DisplaySource {
	t := s.Transition
	r := DisplaySource{Court: t.Court, Projection: s.Projection, Camera: RenderCamera{X: t.CameraScroll[2], Y: t.CameraScroll[3]}, Teams: t.Teams, Characters: t.Characters, Phase: uint8(t.Match.Phase), Period: t.Match.Period, Controls: t.PlayerControls, DisplayFlags: t.DisplayFlags, DetachedCount: s.DetachedCount, LastShooter: t.LastShooter}
	if t.Match.Entered {
		r.Phase |= 128
	}
	if t.Match.Phase == 6 && t.Match.Entered {
		winner := uint8(0)
		if t.FirstTeamLost {
			winner = 1
		}
		r.Result = &RenderResult{Court: t.Court, Digits: t.Match.DisplayedScores, WinningTeam: winner}
	}
	for i, o := range t.Objects {
		r.Objects[i] = RenderObject{ID: uint8(i), Kind: t.Kinds[i], Pose: displayPose(o.Pose), Position: o.Body.Position, Airborne: o.Airborne, Action: o.Action}
	}
	return r
}
func DisplaySourceFromWorld(s WorldLoopState, c WorldLoopContext, m MatchState) DisplaySource {
	c.ObjectKinds = worldObjectKinds(s.Preparation.Actions, c.ObjectKinds)
	a := s.Preparation.Actions
	r := DisplaySource{Court: c.Actions.CourtID, Projection: s.Projection, Camera: RenderCamera{X: s.Camera.X, Y: s.Camera.Y}, Teams: c.Actions.Teams, Characters: c.Actions.Characters, Phase: uint8(m.Phase), Period: m.Period, Controls: c.Actions.Controls, DisplayFlags: s.Preparation.DisplayFlags, DetachedCount: a.DetachedCount, LastShooter: uint8(a.LastShooter)}
	if m.Entered {
		r.Phase |= 128
	}
	positions, poses := worldLoopPositions(a), worldLoopPoses(a)
	for i := range r.Objects {
		r.Objects[i] = RenderObject{ID: uint8(i), Kind: c.ObjectKinds[i], Pose: poses[i], Position: positions[i]}
	}
	for i, p := range a.Actors {
		r.Objects[i].Action = uint8(p.Player.Action)
		r.Objects[i].Airborne = p.Player.Airborne
	}
	for i, o := range a.Auxiliary {
		r.Objects[i+4].Action = o.Action
		r.Objects[i+4].Airborne = o.Airborne
	}
	r.Objects[6].Action = uint8(a.Ball.Action)
	r.Objects[6].Airborne = a.Ball.Airborne
	for i, h := range a.Hoops {
		r.Objects[i+7].Action = uint8(h.Action)
		r.Objects[i+7].Airborne = h.Airborne
	}
	return r
}

// ObserveDisplayMotion updates only the explicitly emitted real terrain reads.
// The original reads the old floor point, before EF5F commits its candidate XY.
func ObserveDisplayMotion(original DisplayState, writes []MotionSurfaceWrite) (DisplayState, error) {
	return ObserveDisplayMotionForCourt(original, writes, 1)
}
func ObserveDisplayMotionForCourt(original DisplayState, writes []MotionSurfaceWrite, court uint8) (DisplayState, error) {
	s := original
	for _, w := range writes {
		if w.Object >= 13 {
			return original, fmt.Errorf("invalid terrain display object %d", w.Object)
		}
		cell, e := OriginalCourtSurface(court, w.Origin.X, w.Origin.Depth)
		if e != nil {
			return original, e
		}
		s.SurfaceCodes[w.Object] = uint8(cell) & 248
	}
	return s, nil
}
