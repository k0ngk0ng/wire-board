package engine

// 9BA4 changes the prop before EEFF. Keep this as an explicit two-stage
// adapter so a court device sees the detached/released object immediately.
func advanceWorldAuxiliarySetDown(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	ctx := AuxiliaryCarrierContext{Actor: actor, Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor), Owners: [2]int8{s.Auxiliary[0].Owner, s.Auxiliary[1].Owner}, Attachment: s.PlayerLinks[actor]}
	if ctx.Attachment&128 != 0 {
		support, err := worldAuxiliarySupport(*s, c, ctx.Attachment&15)
		if err != nil {
			return err
		}
		ctx.Support = support
	}
	prepared, err := PrepareAuxiliarySetDown(a.Player, ctx)
	if err != nil {
		return err
	}
	a.Player = prepared.Player
	if command := prepared.Command; command != nil {
		object := &s.Auxiliary[command.Object-4]
		object.Action, object.Entered = command.Action, false
		if command.ClearAttachment {
			object.Attachment = 0
		}
		if command.ReleaseOwner {
			object.Owner = -1
		}
	}
	completed, err := CompleteAuxiliarySetDown(prepared, ctx, court)
	if err != nil {
		return err
	}
	a.Player = completed.Player
	s.Ball = ApplyBallCommand(s.Ball, completed.Ball)
	if completed.ChangeAttachment {
		s.PlayerLinks[actor] = completed.Attachment
	}
	return nil
}
