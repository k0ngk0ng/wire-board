package engine

import "fmt"

// 9852/9C4E copies the rider's motion into the support, moves support first,
// then attaches the rider. Only afterward are support motion and rider motion
// exchanged. Keep this order visible to the live court state.
func advanceWorldPlayerAssist(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	originalAction := a.Player.Action
	ctx := PlayerAssistContext{Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor), CarriedBody: s.CarriedPlayer == int8(actor), Attachment: s.PlayerLinks[actor]}
	prepared, e := preparePlayerAssist(PlayerAssistState{Player: a.Player, LinkedActor: a.LinkedActor}, ctx)
	if e != nil {
		return e
	}
	a.Player, a.LinkedActor = prepared.State.Player, prepared.State.LinkedActor
	if prepared.ChangeFirstEndurance {
		s.Actors[0].Endurance = prepared.FirstEndurance
	}
	if prepared.ChangeTarget {
		s.Actors[prepared.Target].Player.Action, s.Actors[prepared.Target].Player.Entered = prepared.TargetAction, false
	}
	if originalAction == 9 && ctx.CarriedBody {
		support := ctx.Attachment & 15
		if support >= 13 {
			return fmt.Errorf("assist support %d outside original objects", support)
		}
		f := worldFloorState(*s, c)
		f.Objects[support].Body.Velocity = f.Objects[actor].Body.Velocity
		f.Objects[support].Body.Gravity = f.Objects[actor].Body.Gravity
		for _, object := range []uint8{support, actor} {
			binding, e := originalWorldMotionCourt(result, c, object)
			if e != nil {
				return e
			}
			moved, e := binding.originalMotion.run(f)
			if e != nil {
				return e
			}
			f = moved.State
			storeWorldFloorState(s, f, 13)
		}
		f.Objects[actor].Body.Velocity = f.Objects[support].Body.Velocity
		f.Objects[actor].Body.Gravity = f.Objects[support].Body.Gravity
		f.Objects[support].Body.Velocity = Velocity{}
		f.Objects[support].Body.Gravity = 0
		storeWorldFloorState(s, f, 13)
		return nil
	}
	// Court1's ordinary actor dispatcher uses the flat motion path. Attached
	// release must still execute original EEFF so the support remains authoritative.
	if ctx.Attachment&128 != 0 && court.originalMotion == nil {
		court, e = originalWorldMotionCourt(result, c, actor)
		if e != nil {
			return e
		}
	}
	ball, e := advancePlayerOnCourt(&a.Player, playerTerrainContext{Computer: ctx.Computer, HasBall: ctx.HasBall}, court)
	if e != nil {
		return e
	}
	s.Ball = ApplyBallCommand(s.Ball, ball)
	if originalAction == 9 && a.Player.Airborne && a.Player.Body.Gravity == 0 {
		a.Player.Body.Velocity.Height = signed16(a.Player.Body.Velocity.Height + 0x3c0)
		a.Player.Body.Gravity = 0x1e0
	}
	return nil
}
