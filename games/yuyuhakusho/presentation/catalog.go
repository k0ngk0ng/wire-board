// Package presentation composes decoded indexed graphics independently of the
// reference machine. It accepts no ROM, CPU state, DMA queue or SAT buffer.
package presentation

import (
	"encoding/hex"
	"encoding/json"
	"errors"
)

type Tile [32]byte

type FrameKey struct {
	GraphicsSet uint32 `json:"graphics_set"`
	Parameter   uint16 `json:"parameter"`
}

// Stamp is an immutable transfer identity retained by an actor's four-entry
// cache. Its source/destination fields describe asset data, never instructions.
type Transfer struct {
	Stamp uint32   `json:"stamp"`
	Tiles []uint32 `json:"tiles"`
}

type Piece struct {
	CenterX, CenterY        int16
	WidthTiles, HeightTiles uint8
	FirstSlot               uint8
	XorAttributes           uint16
}

func (p *Piece) UnmarshalJSON(raw []byte) error {
	var v struct {
		CenterX       int16  `json:"center_x"`
		CenterY       int16  `json:"center_y"`
		WidthTiles    uint8  `json:"width_tiles"`
		HeightTiles   uint8  `json:"height_tiles"`
		FirstSlot     uint8  `json:"first_slot"`
		XorAttributes uint16 `json:"xor_attributes"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	*p = Piece{v.CenterX, v.CenterY, v.WidthTiles, v.HeightTiles, v.FirstSlot, v.XorAttributes}
	return nil
}

type Frame struct {
	Key           FrameKey   `json:"key"`
	TileBank      uint32     `json:"tile_bank"`
	PreloadedOnly bool       `json:"preloaded_only"`
	Transfers     []Transfer `json:"transfers"`
	Pieces        []Piece    `json:"pieces"`
}

type SlotBinding struct {
	Slot uint16 `json:"slot"`
	Tile uint32 `json:"tile"`
}

type CharacterPalette struct {
	CharacterID uint8         `json:"character_id"`
	GraphicsSet uint32        `json:"graphics_set"`
	Variants    [4][10]uint16 `json:"variants"`
}

// Catalog is immutable after decoding. The shared tile bank contains packed
// 4bpp indices; world palette colors and the current slot map remain separate.
type Catalog struct {
	Tiles              []Tile
	Frames             []Frame
	DefaultStaticSlots []SlotBinding
	Characters         []CharacterPalette
	SharedPalettes     [8][10]uint16
	lookup             map[FrameKey]int
	characters         [11]int
}

func DecodeCatalog(raw []byte) (Catalog, error) {
	var data struct {
		Schema     int    `json:"schema"`
		ROMSHA     string `json:"rom_sha256"`
		TileFormat string `json:"tile_format"`
		TileBytes  string `json:"tile_bytes"`
		Frames     []Frame
		Slots      []SlotBinding `json:"default_static_slots"`
		Characters []struct {
			CharacterID uint8      `json:"character_id"`
			GraphicsSet uint32     `json:"graphics_set"`
			Variants    [][]uint16 `json:"variants"`
		}
		Shared struct{ Variants [][]uint16 } `json:"shared_palettes"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return Catalog{}, err
	}
	tiles, err := hex.DecodeString(data.TileBytes)
	if err != nil || len(tiles) == 0 || len(tiles)%32 != 0 || data.Schema != 1 || data.TileFormat != "md-4bpp-row-major-high-nibble-first" || data.ROMSHA != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" {
		return Catalog{}, errors.New("invalid indexed world sprite catalog")
	}
	c := Catalog{Tiles: make([]Tile, len(tiles)/32), Frames: data.Frames, DefaultStaticSlots: data.Slots, lookup: make(map[FrameKey]int, len(data.Frames))}
	for i := range c.Tiles {
		copy(c.Tiles[i][:], tiles[i*32:(i+1)*32])
	}
	for i, frame := range c.Frames {
		if frame.Key.GraphicsSet == 0 || frame.Key.Parameter&1 != 0 || len(frame.Transfers) == 0 || len(frame.Transfers) > 4 || len(frame.Pieces) == 0 || len(frame.Pieces) > 80 {
			return Catalog{}, errors.New("invalid world sprite frame")
		}
		if _, exists := c.lookup[frame.Key]; exists {
			return Catalog{}, errors.New("duplicate world sprite key")
		}
		for _, transfer := range frame.Transfers {
			if transfer.Stamp&0x80000000 != 0 || (!frame.PreloadedOnly && len(transfer.Tiles) != int(uint8(transfer.Stamp))) || frame.PreloadedOnly && len(transfer.Tiles) != 0 {
				return Catalog{}, errors.New("invalid world tile assignment")
			}
			for _, tile := range transfer.Tiles {
				if int(tile) >= len(c.Tiles) {
					return Catalog{}, errors.New("unavailable tile in world frame")
				}
			}
		}
		for _, piece := range frame.Pieces {
			if piece.CenterX < -64 || piece.CenterX > 63 || piece.CenterY < -64 || piece.CenterY > 63 || piece.WidthTiles < 1 || piece.WidthTiles > 4 || piece.HeightTiles < 1 || piece.HeightTiles > 4 || piece.XorAttributes&^uint16(0x7800) != 0 {
				return Catalog{}, errors.New("invalid world sprite piece")
			}
		}
		c.lookup[frame.Key] = i
	}
	var slots [2048]bool
	for _, slot := range c.DefaultStaticSlots {
		if slot.Slot >= 2048 || slots[slot.Slot] || int(slot.Tile) >= len(c.Tiles) {
			return Catalog{}, errors.New("invalid default preloaded slot")
		}
		slots[slot.Slot] = true
	}
	if len(data.Characters) != 11 || len(data.Shared.Variants) != 8 || len(c.Frames) == 0 {
		return Catalog{}, errors.New("incomplete world palette/frame catalog")
	}
	var seen [11]bool
	for _, source := range data.Characters {
		if source.CharacterID >= 11 || seen[source.CharacterID] || len(source.Variants) != 4 {
			return Catalog{}, errors.New("invalid character palette catalog")
		}
		if _, exists := c.lookup[FrameKey{source.GraphicsSet, 2}]; !exists {
			return Catalog{}, errors.New("character palette graphics are unavailable")
		}
		seen[source.CharacterID] = true
		entry := CharacterPalette{CharacterID: source.CharacterID, GraphicsSet: source.GraphicsSet}
		for i, colors := range source.Variants {
			if len(colors) != 10 {
				return Catalog{}, errors.New("invalid ten-color character tail")
			}
			copy(entry.Variants[i][:], colors)
		}
		c.characters[source.CharacterID] = len(c.Characters)
		c.Characters = append(c.Characters, entry)
	}
	for i, colors := range data.Shared.Variants {
		if len(colors) != 10 {
			return Catalog{}, errors.New("invalid shared palette tail")
		}
		copy(c.SharedPalettes[i][:], colors)
	}
	return c, nil
}

func (c Catalog) Lookup(key FrameKey) (Frame, bool) {
	i, exists := c.lookup[key]
	if !exists || i >= len(c.Frames) {
		return Frame{}, false
	}
	return c.Frames[i], true
}

// ApplyPalette changes only entries6..15 of one world slot, preserving the
// public first six colors and every other slot exactly as supplied.
func ApplyPalette(colors [64]uint16, character, selector, slot uint8, catalog Catalog) ([64]uint16, error) {
	if character >= 11 || selector >= 12 || slot >= 4 || len(catalog.Characters) != 11 {
		return colors, errors.New("unavailable character palette selector")
	}
	var tail [10]uint16
	if selector < 4 {
		tail = catalog.Characters[catalog.characters[character]].Variants[selector]
	} else {
		tail = catalog.SharedPalettes[selector-4]
	}
	copy(colors[int(slot)*16+6:int(slot+1)*16], tail[:])
	return colors, nil
}

func CompactPalette(words [64]uint16) [64]uint16 {
	var compact [64]uint16
	for i, word := range words {
		compact[i] = (word>>1)&7 | ((word>>5)&7)<<3 | ((word>>9)&7)<<6
	}
	return compact
}
