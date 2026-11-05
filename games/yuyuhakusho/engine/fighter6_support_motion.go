package engine

import "errors"

// StepFighter6SupportMotion reuses the recovered shared entries in source6's
// actual state view. It stops before15afc; animation/tail/FX scheduling is external.
func StepFighter6SupportMotion(state Fighter6MotionState, parameters JumpParameters) (Fighter6MotionState, error) {
	before := state
	var err error
	switch state.Body.ActionKind {
	case 0x0e:
		var view Fighter0MotionState
		view, err = StepFighter0Motion(Fighter0MotionState{ChuSpecialMotionState: state.ChuSpecialMotionState}, parameters)
		state.ChuSpecialMotionState = view.ChuSpecialMotionState
	case 0x0f:
		state.ChuSpecialMotionState, err = StepChuSpecialMotion(state.ChuSpecialMotionState, parameters)
	case 0x11:
		var view Fighter10MotionState
		view, err = StepFighter10Motion(Fighter10MotionState{ObjectID: state.ObjectID, Body: state.Body, Controls: state.Controls, Aux48: state.AlternateVelocity, ContactFlags: state.ContactFlags, Pool: state.Pool}, parameters)
		state.Body, state.Controls, state.AlternateVelocity, state.ContactFlags, state.Pool = view.Body, view.Controls, view.Aux48, view.ContactFlags, view.Pool
	case 0x13:
		var view Fighter3MotionState
		view, err = StepFighter3Motion(Fighter3MotionState{ChuSpecialMotionState: state.ChuSpecialMotionState}, parameters)
		state.ChuSpecialMotionState = view.ChuSpecialMotionState
	case 0x37:
		if state.Body.Clock.Completed == 1 {
			state.Body.Clock.Completed = 0
			state.Body.ExtraJump = 0
			state.Body.GroundActionState = ResetActionState(state.Body.GroundActionState)
			state.Body.Gravity, state.Body.VelocityX, state.Body.VerticalVelocity = parameters.Gravity, 0, 0
			state.Body.ActionKind, state.Body.Pose, state.Body.EnabledStages = 2, 7, 0x14
		}
	default:
		err = errors.New("unsupported source6 support motion")
	}
	if err != nil {
		return before, err
	}
	return state, nil
}

// enterFighter6Exit recovers18682 through its tail-called action reset. This
// enters0c/2b and reloads the word at7e; it does not enter ordinary air motion.
func enterFighter6Exit(body *FighterBodyState) {
	body.ActionKind, body.Pose, body.EnabledStages = 0x0c, 0x2b, 0
	body.Auxiliary7E = body.Auxiliary80
	body.GroundActionState = ResetActionState(body.GroundActionState)
}
