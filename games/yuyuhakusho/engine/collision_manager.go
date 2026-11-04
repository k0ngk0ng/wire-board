package engine

// CollisionActor is the union needed by the recovered collision consumers.
// ThrowDelay is the separate +c8 exclusion timer; StatusDelay is +86.
type CollisionActor struct {
	AttackContactActor
	ThrowDelay uint8
}

type CollisionManagerState struct {
	Actors           map[uint16]CollisionActor
	Buffers          [4]AnimationShapeBuffer
	Throws           ThrowContactBuffer
	Effects          EffectPool
	EffectParameters [16]uint16
	PendingSound     uint8
	SoundQueue       [4]uint8
	PauseFlags       uint8
	SlowTicks        uint16
	SlowFlag         uint8
	ComboFlags       uint8
	ArenaLeft        int16
	ArenaRight       int16
}

// ResolveCollisionManager recovers 0x1edac in original consumer order. The
// caller supplies the preceding actor-update round's shapes. This function
// does not advance actors or submit new shapes; the world scheduler must call
// it before those updates, as the original manager object does.
func ResolveCollisionManager(state CollisionManagerState) (CollisionManagerState, error) {
	if state.PauseFlags&0x80 != 0 {
		return state, nil
	}
	original := state
	state.Actors = make(map[uint16]CollisionActor, len(original.Actors))
	for id, actor := range original.Actors {
		state.Actors[id] = actor
	}
	throws := ThrowContactState{Actors: make(map[uint16]ThrowContactActor, len(state.Actors)), Throws: state.Throws, Hurts: state.Buffers[2], Left: state.ArenaLeft, Right: state.ArenaRight}
	for id, a := range state.Actors {
		throws.Actors[id] = ThrowContactActor{X: a.X, Y: a.Y, Ground: a.Ground, Team: a.Team, Delay: a.ThrowDelay, EnabledStages: a.ActionFlags,
			FrameFlags: a.FrameFlags, ContactFlags: a.ContactFlags, TargetMask: a.TargetMask, Facing: a.Facing, InputFlags: a.RetainedHeld,
			Layer: a.Layer, Pose: a.Pose, Action: a.ActionKind, FrameIndex: a.FrameIndex, FrameDuration: a.FrameDuration, FrameTicks: a.FrameTicks,
			Auxiliary60: a.Completed, AttackFlags: a.AttackFlags, RetainedPress: a.Auxiliary77, CancelTicks: a.CancelTicks}
	}
	throws, err := ResolveThrowContacts(throws)
	if err != nil {
		return original, err
	}
	for id, t := range throws.Actors {
		a := state.Actors[id]
		a.X, a.Y, a.Ground, a.Team, a.ThrowDelay = t.X, t.Y, t.Ground, t.Team, t.Delay
		a.ActionFlags, a.FrameFlags, a.ContactFlags, a.TargetMask, a.Facing = t.EnabledStages, t.FrameFlags, t.ContactFlags, t.TargetMask, t.Facing
		a.RetainedHeld, a.Layer, a.Pose, a.ActionKind = t.InputFlags, t.Layer, t.Pose, t.Action
		a.FrameIndex, a.FrameDuration, a.FrameTicks, a.Completed = t.FrameIndex, t.FrameDuration, t.FrameTicks, t.Auxiliary60
		a.AttackFlags, a.Auxiliary77, a.CancelTicks = t.AttackFlags, t.RetainedPress, t.CancelTicks
		state.Actors[id] = a
	}
	clash := ClashState{Actors: make(map[uint16]ClashActor, len(state.Actors)), Buffer: state.Buffers[1], Effects: state.Effects, PendingSound: state.PendingSound}
	for id, a := range state.Actors {
		clash.Actors[id] = ClashActor{Team: a.Team, Ground: a.Ground, Y: a.Y, TargetBit: a.TargetBit, TargetMask: a.TargetMask,
			AttackValue: a.AttackValue, AttackFlags: a.AttackFlags, FrameTicks: a.FrameTicks, ContactFlags: a.ContactFlags,
			HitStopTicks: a.HitStopTicks, InputRetention: a.InputRetention, CancelTicks: a.CancelTicks, Residual: a.Residual,
			Shake: a.Shake, Attributes: a.Attributes, Layer: a.Layer, Depth: a.Depth}
	}
	clash, err = ResolveClashes(clash)
	if err != nil {
		return original, err
	}
	state.Effects, state.PendingSound = clash.Effects, clash.PendingSound
	for id, c := range clash.Actors {
		a := state.Actors[id]
		a.TargetMask, a.FrameTicks, a.ContactFlags = c.TargetMask, c.FrameTicks, c.ContactFlags
		a.HitStopTicks, a.InputRetention, a.CancelTicks, a.Residual, a.Shake = c.HitStopTicks, c.InputRetention, c.CancelTicks, c.Residual, c.Shake
		state.Actors[id] = a
	}
	attack := AttackContactState{Actors: make(map[uint16]AttackContactActor, len(state.Actors)), Attack: state.Buffers[0], Hurt: state.Buffers[2],
		Effects: state.Effects, EffectParameters: state.EffectParameters, PendingSound: state.PendingSound,
		SlowTicks: state.SlowTicks, ComboFlags: state.ComboFlags, ArenaLeft: state.ArenaLeft, ArenaRight: state.ArenaRight}
	for id, a := range state.Actors {
		attack.Actors[id] = a.AttackContactActor
	}
	attack, err = ResolveAttackContacts(attack)
	if err != nil {
		return original, err
	}
	state.Effects, state.EffectParameters, state.PendingSound = attack.Effects, attack.EffectParameters, attack.PendingSound
	state.SlowTicks, state.ComboFlags = attack.SlowTicks, attack.ComboFlags
	for id, a := range attack.Actors {
		actor := state.Actors[id]
		actor.AttackContactActor = a
		state.Actors[id] = actor
	}
	body := BodySeparationState{Actors: make(map[uint16]BodySeparationActor, len(state.Actors)), Buffer: state.Buffers[3]}
	for id, a := range state.Actors {
		body.Actors[id] = BodySeparationActor{X: a.X, Ground: a.Ground, Team: a.Team, ContactFlags: a.ContactFlags}
	}
	body, err = SeparateBodies(body)
	if err != nil {
		return original, err
	}
	state.Buffers[3] = body.Buffer
	for id, b := range body.Actors {
		a := state.Actors[id]
		a.X, a.ContactFlags = b.X, b.ContactFlags
		state.Actors[id] = a
	}
	for i := range state.Buffers {
		state.Buffers[i].Offset = 0
	}
	state.Throws.Offset = 0
	if state.PendingSound != 0 {
		request := state.PendingSound
		state.PendingSound = 0
		for i, value := range state.SoundQueue {
			if value < 128 || value == request {
				state.SoundQueue[i] = request
				break
			}
		}
	}
	if state.SlowTicks != 0 {
		state.SlowTicks--
	}
	state.SlowFlag = 0
	if state.SlowTicks&2 != 0 {
		state.SlowFlag = 1
	}
	return state, nil
}
