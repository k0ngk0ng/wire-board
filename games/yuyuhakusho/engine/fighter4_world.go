package engine

import "errors"

func fighter4WorldEnvironment(state *ChuWorldState) ChuAttachmentWorld {
	return ChuAttachmentWorld{PauseFlags: state.Clock.PauseFlags, SlowFlag: state.Clock.SlowFlag, AttributeMask: state.Match.AttributeMask,
		CameraX: state.Camera.X, CameraY: state.Camera.Y, Shake: int16(state.Clock.ScreenShake)}
}

func stepFighter4WorldDragonHead(state *ChuWorldState, object WorldObject, library Fighter4DragonLibrary) ([]ChuWorldEvent, error) {
	pool, err := BuildWorldEffectPool(state.Objects, state.EffectCursor)
	if err != nil {
		return nil, err
	}
	view := Fighter4DragonHeadState{Visual: fighter1WorldVisual(state, object), Direction: object.Aux48, EffectiveAngle: object.Body.VelocityX,
		WavePhase: uint16(object.Body.Gravity >> 16), Age: uint16(object.Body.VerticalVelocity >> 16), SoundClock: uint16(object.Energy >> 16),
		DragonBank: object.DragonBank, Depth: object.Body.Depth, Ground: int16(object.Body.Ground >> 16), Layer: object.Body.Layer,
		SkipGraphics: object.SkipGraphics, Held: uint8(object.Controls.Held), ContactFlags: object.ContactFlags, Pool: pool}
	for i := range view.EffectExtras {
		effect := state.Objects[0x9400+uint16(i)*128]
		view.EffectExtras[i] = Fighter4DragonTrailExtra{Owner: uint16(effect.Aux48 >> 16), Bank: uint16(effect.Body.Gravity >> 16)}
	}
	next, history, out, err := StepFighter4DragonHead(view, state.DragonHistory, ChuProjectileEnvironment{PauseFlags: state.Clock.PauseFlags, SlowFlag: state.Clock.SlowFlag,
		PaletteAttributes: state.Match.AttributeMask, ArenaLeft: state.Camera.Left, ArenaRight: state.Camera.Right, CameraX: state.Camera.X, CameraY: state.Camera.Y, Shake: int16(state.Clock.ScreenShake)}, library)
	if err != nil {
		return nil, err
	}
	objects, err := MergeWorldEffectPool(state.Objects, next.Pool)
	if err != nil {
		return nil, err
	}
	object, err = mergeFighter1WorldVisual(object, next.Visual, out.ClearedBytes)
	if err != nil {
		return nil, err
	}
	object.Aux48, object.Body.VelocityX, object.DragonBank = next.Direction, next.EffectiveAngle, next.DragonBank
	object.Body.Gravity = worldObjectHighWord(object.Body.Gravity, next.WavePhase)
	object.Body.VerticalVelocity = worldObjectHighWord(object.Body.VerticalVelocity, next.Age)
	object.Energy = worldObjectHighWord(object.Energy, next.SoundClock)
	object.Body.Ground = worldObjectHighWord(object.Body.Ground, uint16(next.Ground))
	object.Body.Depth, object.Body.Layer, object.SkipGraphics = next.Depth, next.Layer, next.SkipGraphics
	for i, extra := range next.EffectExtras {
		id := uint16(0x9400 + i*128)
		effect := objects[id]
		effect.Aux48 = worldObjectHighWord(effect.Aux48, extra.Owner)
		effect.Body.Gravity = worldObjectHighWord(effect.Body.Gravity, extra.Bank)
		objects[id] = effect
	}
	objects[object.ObjectID] = object
	state.Objects, state.DragonHistory, state.EffectCursor = objects, history, next.Pool.Cursor
	state.Buffers, state.PendingSound, state.SoundQueue = next.Visual.Buffers, next.Visual.PendingSound, next.Visual.SoundQueue
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}

func stepFighter4WorldDragonTrail(state *ChuWorldState, object WorldObject) ([]ChuWorldEvent, error) {
	view := Fighter4DragonTrailState{Header: object.Header, Draw: object.DrawRequest(), WorldX: object.Body.X, WorldY: object.Body.Y,
		ScreenOffsetX: object.ScreenOffsetX, ScreenOffsetY: object.ScreenOffsetY, Angle: uint16(object.Body.VelocityX >> 16),
		Age: uint16(object.Body.VerticalVelocity >> 16), Bank: uint16(object.Body.Gravity >> 16)}
	next, out, err := StepFighter4DragonTrail(view, state.DragonHistory, fighter4WorldEnvironment(state))
	if err != nil {
		return nil, err
	}
	object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
	if err != nil {
		return nil, err
	}
	object.Header, object.GraphicsSet, object.TilesBase, object.GraphicParameter = next.Header, next.Draw.GraphicsSet, next.Draw.TileBank, next.Draw.Parameter
	object.ScreenX, object.ScreenY, object.GraphicOffsetX, object.GraphicOffsetY = int16(next.Draw.ScreenX), int16(next.Draw.ScreenY), next.Draw.OffsetX, next.Draw.OffsetY
	object.Body.Attributes, object.Body.Depth, object.Body.Layer = next.Draw.Attributes, next.Draw.Depth, next.Draw.Layer
	object.SkipGraphics, object.GraphicCache = next.Draw.SkipTiles, next.Draw.Cache
	object.Body.X, object.Body.Y, object.ScreenOffsetX, object.ScreenOffsetY = next.WorldX, next.WorldY, next.ScreenOffsetX, next.ScreenOffsetY
	object.Body.VelocityX = worldObjectHighWord(object.Body.VelocityX, next.Angle)
	object.Body.VerticalVelocity = worldObjectHighWord(object.Body.VerticalVelocity, next.Age)
	object.Body.Gravity = worldObjectHighWord(object.Body.Gravity, next.Bank)
	state.Objects[object.ObjectID] = object
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}

func stepFighter4WorldBoost(state *ChuWorldState, object WorldObject, library AnimationLibrary) ([]ChuWorldEvent, error) {
	if object.ObjectID < 0x8500 || object.ObjectID > 0x9100 || (object.ObjectID-0x8500)%0x400 != 0 {
		return nil, errors.New("source4 boost parent identity is unavailable")
	}
	parent, err := LookupWorldObject(state.Objects, object.ObjectID-256)
	if err != nil {
		return nil, err
	}
	view := Fighter4BoostState{Visual: fighter1WorldVisual(state, object), Depth: object.Body.Depth, Layer: object.Body.Layer}
	parentView := Fighter4BoostParent{Body: parent.Body, Energy: parent.Energy, AttackValue: parent.AttackValue, MovementSource: parent.MovementSource, StatusDelay: parent.StatusDelay, StunTicks: parent.StunTicks}
	next, parentNext, out, err := StepFighter4Boost(view, parentView, fighter4WorldEnvironment(state), library)
	if err != nil {
		return nil, err
	}
	object, err = mergeFighter1WorldVisual(object, next.Visual, out.ClearedBytes)
	if err != nil {
		return nil, err
	}
	object.Body.Depth, object.Body.Layer = next.Depth, next.Layer
	parent.Body, parent.Energy, parent.AttackValue = parentNext.Body, parentNext.Energy, parentNext.AttackValue
	parent.MovementSource, parent.StatusDelay, parent.StunTicks = parentNext.MovementSource, parentNext.StatusDelay, parentNext.StunTicks
	state.Objects[parent.ObjectID], state.Objects[object.ObjectID] = parent, object
	state.Buffers, state.PendingSound, state.SoundQueue = next.Visual.Buffers, next.Visual.PendingSound, next.Visual.SoundQueue
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}
