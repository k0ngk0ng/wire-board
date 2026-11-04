package engine

import "errors"

// AsMatchOverlay exposes the original first64-byte overlay fields. Counter
// shares the high word of VelocityX, while Timer/Data share Aux18's two words.
func (object WorldObject) AsMatchOverlay() MatchOverlayObject {
	return MatchOverlayObject{Header: object.Header, ScreenX: object.ScreenX, ScreenY: object.ScreenY,
		Counter: uint16(uint32(object.Body.VelocityX) >> 16), Timer: uint16(uint32(object.Aux18) >> 16), Data: uint16(object.Aux18),
		GraphicsSet: object.GraphicsSet, TilesBase: object.TilesBase, Attributes: object.Body.Attributes, Layer: object.Body.Layer,
		SkipGraphics: object.SkipGraphics, GraphicOffsetX: object.GraphicOffsetX, GraphicOffsetY: object.GraphicOffsetY,
		GraphicCache: object.GraphicCache, Parameter: object.GraphicParameter, Depth: object.Body.Depth}
}

func (object WorldObject) MergeMatchOverlay(overlay MatchOverlayObject) WorldObject {
	object.Header, object.ScreenX, object.ScreenY = overlay.Header, overlay.ScreenX, overlay.ScreenY
	object.Body.VelocityX = worldObjectHighWord(object.Body.VelocityX, overlay.Counter)
	object.Aux18 = int32(uint32(overlay.Timer)<<16 | uint32(overlay.Data))
	object.GraphicsSet, object.TilesBase = overlay.GraphicsSet, overlay.TilesBase
	object.Body.Attributes, object.Body.Layer, object.SkipGraphics = overlay.Attributes, overlay.Layer, overlay.SkipGraphics
	object.GraphicOffsetX, object.GraphicOffsetY, object.GraphicCache = overlay.GraphicOffsetX, overlay.GraphicOffsetY, overlay.GraphicCache
	object.GraphicParameter, object.Body.Depth = overlay.Parameter, overlay.Depth
	return object
}

// AsArenaSky follows the actual sky decoder: Phase is the complete +48 long,
// Sequence the +50 long, and the screen fraction the independent +0a word.
// It does not reinterpret Phase as the overlay's high-word timer or owner ID.
func (object WorldObject) AsArenaSky(kind ArenaSkyKind) ArenaSkyState {
	return ArenaSkyState{Kind: kind, ObjectID: object.ObjectID, Behavior: object.Header.Behavior, Size: object.Header.Size, Stage: object.Header.Stage,
		ScreenX: object.ScreenX, ScreenY: object.ScreenY, ScreenXFraction: object.Aux0A,
		GraphicsSet: object.GraphicsSet, TilesBase: object.TilesBase, Attributes: object.Body.Attributes, Layer: object.Body.Layer,
		GraphicOffsetX: object.GraphicOffsetX, GraphicOffsetY: object.GraphicOffsetY, Cache: object.GraphicCache,
		Parameter: object.GraphicParameter, Depth: object.Body.Depth, X: object.Body.X, Y: object.Body.Y,
		Phase: uint32(object.Aux48), Sequence: object.AnimationSet, Index: object.Body.Clock.FrameIndex,
		Ticks: object.Body.Clock.Ticks, Flags: object.Body.Clock.Flags}
}

func (object WorldObject) MergeArenaSky(sky ArenaSkyState) (WorldObject, error) {
	if sky.ObjectID != object.ObjectID {
		return object, errors.New("arena sky object identity mismatch")
	}
	object.Header = MatchObjectHeader{Behavior: sky.Behavior, Size: sky.Size, Stage: sky.Stage}
	object.ScreenX, object.ScreenY, object.Aux0A = sky.ScreenX, sky.ScreenY, sky.ScreenXFraction
	object.GraphicsSet, object.TilesBase, object.Body.Attributes, object.Body.Layer = sky.GraphicsSet, sky.TilesBase, sky.Attributes, sky.Layer
	object.GraphicOffsetX, object.GraphicOffsetY, object.GraphicCache = sky.GraphicOffsetX, sky.GraphicOffsetY, sky.Cache
	object.GraphicParameter, object.Body.Depth, object.Body.X, object.Body.Y = sky.Parameter, sky.Depth, sky.X, sky.Y
	object.Aux48, object.AnimationSet = int32(sky.Phase), sky.Sequence
	object.Body.Clock.FrameIndex, object.Body.Clock.Ticks, object.Body.Clock.Flags = sky.Index, sky.Ticks, sky.Flags
	return object, nil
}

// Registration is shared world state and is filled separately by the caller.
// Cursor is a high-word view of AnimationSet, not a second stored value.
func (object WorldObject) AsMatchShadow() MatchShadowState {
	return MatchShadowState{Object: object.AsMatchOverlay(), X: object.Body.X, Y: object.Body.Y,
		Cursor: uint16(object.AnimationSet >> 16), ScreenOffsetX: object.ScreenOffsetX, ScreenOffsetY: object.ScreenOffsetY}
}

func (object WorldObject) MergeMatchShadow(shadow MatchShadowState) WorldObject {
	object = object.MergeMatchOverlay(shadow.Object)
	object.Body.X, object.Body.Y = shadow.X, shadow.Y
	object.AnimationSet = uint32(worldObjectHighWord(int32(object.AnimationSet), shadow.Cursor))
	object.ScreenOffsetX, object.ScreenOffsetY = shadow.ScreenOffsetX, shadow.ScreenOffsetY
	return object
}
