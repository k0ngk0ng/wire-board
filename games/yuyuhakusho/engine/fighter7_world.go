package engine

func fighter7WorldEnvironment(state *ChuWorldState) ChuProjectileEnvironment {
	return ChuProjectileEnvironment{PauseFlags: state.Clock.PauseFlags, SlowFlag: state.Clock.SlowFlag, PaletteAttributes: state.Match.AttributeMask,
		ArenaLeft: state.Camera.Left, ArenaRight: state.Camera.Right, CameraX: state.Camera.X, CameraY: state.Camera.Y, Shake: int16(state.Clock.ScreenShake)}
}

func stepFighter7WorldFirstChild(state *ChuWorldState, object WorldObject, library AnimationLibrary) ([]ChuWorldEvent, error) {
	parent, err := LookupWorldObject(state.Objects, object.ObjectID-0x100)
	if err != nil {
		return nil, err
	}
	link, err := LookupWorldObject(state.Objects, object.ObjectID+0x100)
	if err != nil {
		return nil, err
	}
	context := Fighter7FirstChildContext{Parent: Fighter7ChildParent{ObjectID: parent.ObjectID, Pose: parent.Body.Pose},
		Link:       Fighter7ChildLink{ObjectID: link.ObjectID, VelocityX: link.Body.VelocityX, VerticalVelocity: link.Body.VerticalVelocity, Aux48: link.Aux48},
		Submission: Fighter7ChildSubmission{Buffers: state.Buffers, PendingSound: state.PendingSound, SoundQueue: state.SoundQueue}}
	next, context, out, err := StepFighter7FirstChild(object, context, fighter7WorldEnvironment(state), library)
	if err != nil {
		return nil, err
	}
	link.Body.VelocityX, link.Body.VerticalVelocity, link.Aux48 = context.Link.VelocityX, context.Link.VerticalVelocity, context.Link.Aux48
	state.Objects[object.ObjectID], state.Objects[link.ObjectID] = next, link
	state.Buffers, state.PendingSound, state.SoundQueue = context.Submission.Buffers, context.Submission.PendingSound, context.Submission.SoundQueue
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}

func stepFighter7WorldSecondChild(state *ChuWorldState, object WorldObject, library AnimationLibrary) ([]ChuWorldEvent, error) {
	parent, err := LookupWorldObject(state.Objects, object.ObjectID-0x200)
	if err != nil {
		return nil, err
	}
	pool, err := BuildWorldEffectPool(state.Objects, state.EffectCursor)
	if err != nil {
		return nil, err
	}
	context := Fighter7SecondChildContext{Parent: Fighter7ChildParent{ObjectID: parent.ObjectID, Pose: parent.Body.Pose}, Pool: pool,
		Submission: Fighter7ChildSubmission{Buffers: state.Buffers, PendingSound: state.PendingSound, SoundQueue: state.SoundQueue}}
	for i := range context.Fighters {
		id := uint16(0x8400 + i*0x400)
		o, err := LookupWorldObject(state.Objects, id)
		if err != nil {
			return nil, err
		}
		context.Fighters[i] = Fighter7HealingTarget{ObjectID: id, Size: o.Header.Size, Health: o.Body.Health, X: o.Body.X, Y: o.Body.Y}
	}
	for i := range context.EffectExtras {
		o, err := LookupWorldObject(state.Objects, uint16(0x9400+i*128))
		if err != nil {
			return nil, err
		}
		context.EffectExtras[i] = Fighter7ChildEffectExtra{VerticalVelocity: o.Body.VerticalVelocity, Delay: o.Body.Clock.Duration}
	}
	next, context, out, err := StepFighter7SecondChild(object, context, fighter7WorldEnvironment(state), library)
	if err != nil {
		return nil, err
	}
	objects, err := MergeWorldEffectPool(state.Objects, context.Pool)
	if err != nil {
		return nil, err
	}
	for _, target := range context.Fighters {
		o := objects[target.ObjectID]
		o.Body.Health = target.Health
		objects[target.ObjectID] = o
	}
	for i, extra := range context.EffectExtras {
		id := uint16(0x9400 + i*128)
		o := objects[id]
		o.Body.VerticalVelocity, o.Body.Clock.Duration = extra.VerticalVelocity, extra.Delay
		objects[id] = o
	}
	objects[object.ObjectID] = next
	state.Objects, state.EffectCursor = objects, context.Pool.Cursor
	state.Buffers, state.PendingSound, state.SoundQueue = context.Submission.Buffers, context.Submission.PendingSound, context.Submission.SoundQueue
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}

func stepFighter7WorldPan(state *ChuWorldState, object WorldObject) ([]ChuWorldEvent, error) {
	next, amplitude, out, err := StepFighter7Pan(object, state.Clock.PanAmplitude, fighter7WorldEnvironment(state))
	if err != nil {
		return nil, err
	}
	state.Objects[object.ObjectID], state.Clock.PanAmplitude = next, amplitude
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}
