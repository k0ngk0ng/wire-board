package presentation

import (
	"errors"
	"sort"
)

type Actor struct {
	ObjectID              uint16
	GraphicsSet, TileBank uint32
	Parameter             uint16
	ScreenX, ScreenY      uint16
	OffsetX, OffsetY      int16
	Attributes, Depth     uint16
	Layer, SkipTiles      uint8
	Cache                 [4]uint32
}

// TileSlots is a bounded current mapping of logical tile slots to immutable
// indexed tiles. It is not VRAM and contains no pixel or instruction image.
type TileSlots struct {
	Tiles   [2048]uint32
	Present [32]uint64
}

func (slots *TileSlots) Set(slot uint16, tile uint32) error {
	if slot >= 2048 {
		return errors.New("logical sprite tile slot is outside the bank")
	}
	slots.Tiles[slot] = tile
	slots.Present[slot/64] |= uint64(1) << (slot & 63)
	return nil
}

func (slots *TileSlots) Lookup(slot uint16) (uint32, bool) {
	if slot >= 2048 || slots.Present[slot/64]&(uint64(1)<<(slot&63)) == 0 {
		return 0, false
	}
	return slots.Tiles[slot], true
}

type Part struct {
	Owner                      uint16
	Layer, Index               uint8
	X, Y                       uint16
	WidthTiles, HeightTiles    uint8
	TileSlot                   uint16
	Palette                    uint8
	FlipX, FlipY, HighPriority bool
	SourceXor                  uint16 // Source variant flags retained for exact queue adapters.
}

type TileTransfer struct {
	Stamp, Bank uint32
	Destination uint16
	Tiles       []uint32
}

// Parts and Transfers are the current frame's transient presentation window.
// Only Slots and per-actor caches need to persist into the next frame.
type FrameState struct {
	Slots     TileSlots
	Count     uint16
	Flags     uint8
	Parts     []Part
	Transfers []TileTransfer
}

func BeginFrame(slots TileSlots, initialCount uint16) FrameState {
	return FrameState{Slots: slots, Count: initialCount, Flags: 0x80}
}

func DefaultTileSlots(catalog Catalog) (TileSlots, error) {
	var slots TileSlots
	for _, entry := range catalog.DefaultStaticSlots {
		if int(entry.Tile) >= len(catalog.Tiles) {
			return TileSlots{}, errors.New("default sprite tile is unavailable")
		}
		if err := slots.Set(entry.Slot, entry.Tile); err != nil {
			return TileSlots{}, err
		}
	}
	return slots, nil
}

// ComposeActor applies decoded frame data. Cache/skip/culling decisions precede
// global slot assignments. Hardware queue encoding is deliberately absent.
func ComposeActor(state FrameState, actor Actor, catalog Catalog) (FrameState, Actor, error) {
	x, y := actor.ScreenX+uint16(actor.OffsetX), actor.ScreenY+uint16(actor.OffsetY)
	if state.Flags&0x80 == 0 || x-64 == 0 || x-64 >= 448 || y-64 == 0 || y-64 >= 352 {
		return state, actor, nil
	}
	frame, exists := catalog.Lookup(FrameKey{actor.GraphicsSet, actor.Parameter})
	if !exists {
		return state, actor, errors.New("world sprite frame is unavailable")
	}
	if actor.SkipTiles == 0 {
		for i, transfer := range frame.Transfers {
			if actor.Cache[i] == transfer.Stamp {
				continue
			}
			count := int(uint8(transfer.Stamp))
			if count > 0 && (frame.PreloadedOnly || actor.TileBank != frame.TileBank || len(transfer.Tiles) != count) {
				return state, actor, errors.New("world sprite tile source is unavailable")
			}
			for _, tile := range transfer.Tiles {
				if int(tile) >= len(catalog.Tiles) {
					return state, actor, errors.New("world sprite indexed tile is unavailable")
				}
			}
		}
	}
	next, updated := state, actor
	next.Parts = append([]Part(nil), state.Parts...)
	next.Transfers = append([]TileTransfer(nil), state.Transfers...)
	if updated.SkipTiles == 0 {
		for i, transfer := range frame.Transfers {
			if updated.Cache[i] == transfer.Stamp {
				continue
			}
			updated.Cache[i] = transfer.Stamp
			destination := (uint16(uint8(transfer.Stamp>>8)) + updated.Attributes&0x7ff) & 2047
			for j, tile := range transfer.Tiles {
				_ = next.Slots.Set((destination+uint16(j))&2047, tile)
			}
			next.Transfers = append(next.Transfers, TileTransfer{Stamp: transfer.Stamp, Bank: actor.TileBank, Destination: destination, Tiles: append([]uint32(nil), transfer.Tiles...)})
		}
		for i := len(frame.Transfers); i < 4; i++ {
			updated.Cache[i] = 0
		}
	}
	for i, piece := range frame.Pieces {
		dx, dy := piece.CenterX, piece.CenterY
		if actor.Attributes&0x800 != 0 {
			dx = -dx
		}
		if actor.Attributes&0x1000 != 0 {
			dy = -dy
		}
		dx = int16(int32(dx) * int32(int16(actor.Depth)) >> 8)
		dy = int16(int32(dy) * int32(int16(actor.Depth)) >> 8)
		attributes := actor.Attributes + uint16(piece.FirstSlot)
		if piece.XorAttributes&0x6000 != 0 {
			attributes &^= 0x6000
		}
		attributes ^= piece.XorAttributes
		part := Part{Owner: actor.ObjectID, Layer: actor.Layer & 0xfc, Index: uint8(next.Count),
			X: x + uint16(dx) - uint16(piece.WidthTiles)*4, Y: y + uint16(dy) - uint16(piece.HeightTiles)*4,
			WidthTiles: piece.WidthTiles, HeightTiles: piece.HeightTiles, TileSlot: attributes & 0x7ff, Palette: uint8(attributes>>13) & 3,
			FlipX: attributes&0x800 != 0, FlipY: attributes&0x1000 != 0, HighPriority: attributes&0x8000 != 0, SourceXor: piece.XorAttributes}
		next.Count = next.Count&0xff00 | uint16(uint8(next.Count+1))
		next.Parts = append(next.Parts, part)
		if i == len(frame.Pieces)-1 {
			break
		}
		if next.Count >= 80 {
			next.Flags = 1
			break
		}
	}
	return next, updated, nil
}

func OrderedParts(state FrameState) []Part {
	parts := append([]Part(nil), state.Parts...)
	sort.SliceStable(parts, func(i, j int) bool { return parts[i].Layer < parts[j].Layer })
	return parts
}
