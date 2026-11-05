package engine

import "errors"

// ArenaASprite is the callback-time descriptor passed to 14d0. SizeLink retains
// the original size byte and layer-chain selector; the presentation consumer
// owns the hardware sprite budget and final link numbers.
type ArenaASprite struct {
	Y, SizeLink, Tile, X uint16
}

type ArenaAChildView struct {
	PauseFlags, SlowFlag uint8
	CameraY, Shake, Pan  int16
}

// StepArenaAChild recovers 21258. Gating preserves its two cached coordinates
// but does not suppress the six descriptor submissions. Canonical object
// words retain their shared long owners, including the parent's +1a word.
func StepArenaAChild(object, parent WorldObject, view ArenaAChildView) (WorldObject, []ArenaASprite, error) {
	original := object
	if view.PauseFlags&128 == 0 && view.SlowFlag == 0 {
		if object.Header.Stage == 0 {
			object.TilesBase = object.TilesBase&0xffff0000 | 0x0ffc
			object.GraphicOffsetX = -1
			object.Header.Stage += 2
		}
		if parent.ObjectID != uint16(object.Aux18) {
			return original, nil, errors.New("arena A child parent is unavailable")
		}
		x := uint16(parent.ScreenX) + uint16(view.Pan)
		object.Aux18 = int32(uint32(x)<<16 | uint32(object.Aux18)&0xffff)
		y := uint16(view.CameraY) - uint16(view.Shake)
		object.GraphicsSet = uint32(y)<<16 | object.GraphicsSet&0xffff
	}
	if object.GraphicOffsetX != -1 {
		return original, nil, errors.New("arena A child descriptor termination is unavailable")
	}
	tiles := [6]uint16{0x5d0, 0x5d0, 0x5e0, 0x5e0, 0x5f0, 0x5f0}
	ys := [6]uint16{144, 176, 144, 176, 144, 176}
	xs := [6]uint16{224, 224, 256, 256, 496, 496}
	requests := make([]ArenaASprite, 0, 6)
	for i := range tiles {
		y := -uint16(object.GraphicsSet>>16) + ys[i]
		x := uint16(uint32(object.Aux18)>>16) + xs[i]
		object.TilesBase = uint32(y)<<16 | object.TilesBase&0xffff
		object.Body.Attributes = tiles[i]
		object.Body.Layer, object.SkipGraphics = uint8(x>>8), uint8(x)
		requests = append(requests, ArenaASprite{Y: y, SizeLink: uint16(object.TilesBase), Tile: tiles[i], X: x})
	}
	return object, requests, nil
}
