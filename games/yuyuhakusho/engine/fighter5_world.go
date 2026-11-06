package engine

import "errors"

func stepFighter5WorldProjectile(state *ChuWorldState, object WorldObject, library AnimationLibrary, soundPort *worldSoundPort) ([]ChuWorldEvent, error) {
	pool, err := BuildWorldEffectPool(state.Objects, state.EffectCursor)
	if err != nil {
		return nil, err
	}
	parentID := object.DragonBank // The source5 parent view of canonical +fe.
	if object.Header.Behavior == 0x146ba {
		if object.ObjectID < 0x200 {
			return nil, errors.New("source5 follower parent is unavailable")
		}
		parentID = object.ObjectID - 0x200
	}
	parentObject, err := LookupWorldObject(state.Objects, parentID)
	if err != nil {
		return nil, err
	}
	parent := Fighter5ProjectileParent{ObjectID: parentID, X: parentObject.Body.X, Y: parentObject.Body.Y, Pose: parentObject.Body.Pose,
		Held: parentObject.Controls.Held, HitStopTicks: parentObject.HitStopTicks, RetainTicks: parentObject.Controls.RetainTicks}
	view := Fighter5ProjectileState{Visual: fighter1WorldVisual(state, object), VelocityX: object.Body.VelocityX, VerticalVelocity: object.Body.VerticalVelocity,
		Ground: object.Body.Ground, Aux48: object.Aux48, Gravity: object.Body.Gravity, Depth: object.Body.Depth, Owner: object.DragonBank,
		Layer: object.Body.Layer, Team: object.Body.Team, ActionKind: object.Body.ActionKind, ContactFlags: object.ContactFlags, HitStopTicks: object.HitStopTicks, Pool: pool}
	next, parentNext, out, err := stepFighter5ProjectileWithSound(view, parent, ChuProjectileEnvironment{PauseFlags: state.Clock.PauseFlags, SlowFlag: state.Clock.SlowFlag,
		PaletteAttributes: state.Match.AttributeMask, ArenaLeft: state.Camera.Left, ArenaRight: state.Camera.Right, CameraX: state.Camera.X, CameraY: state.Camera.Y, Shake: int16(state.Clock.ScreenShake)}, library, soundPort)
	if err != nil {
		return nil, err
	}
	object, err = mergeFighter1WorldVisual(object, next.Visual, out.ClearedBytes)
	if err != nil {
		return nil, err
	}
	objects, err := MergeWorldEffectPool(state.Objects, next.Pool)
	if err != nil {
		return nil, err
	}
	object.Body.VelocityX, object.Body.VerticalVelocity, object.Body.Ground = next.VelocityX, next.VerticalVelocity, next.Ground
	object.Aux48, object.Body.Gravity, object.Body.Depth = next.Aux48, next.Gravity, next.Depth
	object.Body.Layer, object.Body.Team, object.Body.ActionKind = next.Layer, next.Team, next.ActionKind
	object.ContactFlags, object.HitStopTicks, object.DragonBank = next.ContactFlags, next.HitStopTicks, next.Owner
	if parentNext != parent {
		parentObject.Body.X, parentObject.Body.Y, parentObject.Body.Pose = parentNext.X, parentNext.Y, parentNext.Pose
		parentObject.Controls.Held, parentObject.HitStopTicks, parentObject.Controls.RetainTicks = parentNext.Held, parentNext.HitStopTicks, parentNext.RetainTicks
		objects[parentID] = parentObject
	}
	objects[object.ObjectID] = object
	state.Objects, state.EffectCursor = objects, next.Pool.Cursor
	state.Buffers, state.PendingSound, state.SoundQueue = next.Visual.Buffers, next.Visual.PendingSound, next.Visual.SoundQueue
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}
