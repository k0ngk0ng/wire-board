package contra

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// DecodeScreen expands the original map's literal/repeat/row-copy format. It
// returns the consumed source length because consecutive screens share a bank.
// This is map data decoding only; no cartridge program is present or executed.
func DecodeScreen(encoded []uint8, rows int) ([]uint8, int, error) {
	if rows < 1 || rows > 8 {
		return nil, 0, errors.New("invalid map row count")
	}
	count, cursor := rows*8, 0
	indices := make([]uint8, 0, count)
	for len(indices) < count {
		if cursor >= len(encoded) {
			return nil, cursor, errors.New("truncated map stream")
		}
		code := encoded[cursor]
		cursor++
		switch {
		case code < 0x80:
			indices = append(indices, code)
		case code < 0xf0:
			repeat := int(code & 0x7f)
			if repeat == 0 || len(indices)+repeat > count || cursor >= len(encoded) {
				return nil, cursor, errors.New("invalid map repeat")
			}
			value := encoded[cursor]
			cursor++
			for range repeat {
				indices = append(indices, value)
			}
		default:
			start := int(code&15) * 8
			if start+8 > len(indices) || len(indices)+8 > count {
				return nil, cursor, errors.New("invalid map row reference")
			}
			indices = append(indices, indices[start:start+8]...)
		}
	}
	return indices, cursor, nil
}

type LevelMetadata struct {
	DistinctScreens, LookaheadAlias                int
	AutomaticScrollScreen, AlternateGraphicsScreen int
}

// JungleMetadata describes the original first-stage header, including the
// lookahead reference that must not be counted as another playable screen.
func JungleMetadata() LevelMetadata {
	return LevelMetadata{DistinctScreens: 13, LookaheadAlias: 13, AutomaticScrollScreen: 11, AlternateGraphicsScreen: 11}
}

// ExpandedScreen is a fresh mutable screen instance. The world may apply the
// original bridge/gate changes without modifying another match's source data.
type ExpandedScreen struct {
	SupertileIDs []uint8
	Tiles        []uint8
	Attributes   []uint8
	Collision    [64]uint8
}

type supertileDefinition struct {
	Tiles     [16]uint8 `json:"tiles"`
	Attribute uint8     `json:"attribute"`
}

type jungleDefinition struct {
	Schema                                         int
	ScreenRows, DistinctScreens, LookaheadAlias    int
	AutomaticScrollScreen, AlternateGraphicsScreen int
	CollisionLimits                                [3]uint8
	Screens                                        []struct{ Encoded string }
	Supertiles                                     []supertileDefinition
}

// The embedded data is a 10 KiB export of named map definitions, not a ROM.
// Source ranges and generation instructions live beside it in jungle-source.json.
//
//go:embed leveldata/jungle.json
var jungleJSON []byte

var jungle = sync.OnceValues(func() (*jungleDefinition, error) {
	var layout jungleDefinition
	if err := json.Unmarshal(jungleJSON, &layout); err != nil {
		return nil, err
	}
	meta := JungleMetadata()
	if layout.Schema != 1 || layout.ScreenRows != 7 || len(layout.Supertiles) != 108 ||
		layout.DistinctScreens != meta.DistinctScreens || layout.LookaheadAlias != meta.LookaheadAlias ||
		len(layout.Screens) != meta.LookaheadAlias+1 || layout.AutomaticScrollScreen != meta.AutomaticScrollScreen ||
		layout.AlternateGraphicsScreen != meta.AlternateGraphicsScreen {
		return nil, errors.New("invalid jungle map definition")
	}
	return &layout, nil
})

// JungleScreen decodes a source-bound first-stage screen. It does not infer
// camera progression, destructible terrain or Boss completion from the index.
func JungleScreen(index int) (ExpandedScreen, error) {
	layout, err := jungle()
	if err != nil {
		return ExpandedScreen{}, err
	}
	if index < 0 || index >= len(layout.Screens) {
		return ExpandedScreen{}, errors.New("unknown jungle screen")
	}
	encoded, err := hex.DecodeString(layout.Screens[index].Encoded)
	if err != nil {
		return ExpandedScreen{}, err
	}
	indices, used, err := DecodeScreen(encoded, layout.ScreenRows)
	if err != nil {
		return ExpandedScreen{}, err
	}
	if used != len(encoded) {
		return ExpandedScreen{}, errors.New("unconsumed jungle screen data")
	}
	screen := ExpandedScreen{SupertileIDs: indices, Tiles: make([]uint8, layout.ScreenRows*128), Attributes: make([]uint8, len(indices))}
	for cell, id := range indices {
		if int(id) >= len(layout.Supertiles) {
			return ExpandedScreen{}, fmt.Errorf("unknown supertile %d", id)
		}
		tile := layout.Supertiles[id]
		row, column := cell/8, cell%8
		screen.Attributes[cell] = tile.Attribute
		for y := range 4 {
			start := (row*4+y)*32 + column*4
			copy(screen.Tiles[start:start+4], tile.Tiles[y*4:y*4+4])
		}
		for y := range 2 {
			for x := range 2 {
				code := collisionBits(tile.Tiles[y*8+x*2], layout.CollisionLimits)
				cy, cx := row*2+y, column*2+x
				screen.Collision[cy*4+cx/4] |= code << (6 - 2*(cx%4))
			}
		}
	}
	return screen, nil
}

func collisionBits(tile uint8, limits [3]uint8) uint8 {
	switch {
	case tile == 0:
		return 0
	case tile < limits[0]:
		return 1
	case tile < limits[1]:
		return 0
	case tile < limits[2]:
		return 2
	default:
		return 3
	}
}
