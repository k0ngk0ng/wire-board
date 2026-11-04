package engine

// WorldDraw is a callback-time value, independent of later slot mutations.
// It contains only the inputs of the indexed graphics composer.
type WorldDraw struct {
	ObjectID              uint16
	GraphicsSet, TileBank uint32
	Parameter             uint16
	ScreenX, ScreenY      uint16
	OffsetX, OffsetY      int16
	Attributes, Depth     uint16
	Layer, SkipTiles      uint8
	Cache                 [4]uint32
}

func (object WorldObject) DrawRequest() WorldDraw {
	return WorldDraw{ObjectID: object.ObjectID, GraphicsSet: object.GraphicsSet, TileBank: object.TilesBase,
		Parameter: object.GraphicParameter, ScreenX: uint16(object.ScreenX), ScreenY: uint16(object.ScreenY),
		OffsetX: object.GraphicOffsetX, OffsetY: object.GraphicOffsetY, Attributes: object.Body.Attributes,
		Depth: object.Body.Depth, Layer: object.Body.Layer, SkipTiles: object.SkipGraphics, Cache: object.GraphicCache}
}
