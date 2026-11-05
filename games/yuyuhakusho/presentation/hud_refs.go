package presentation

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
)

// MaterializeHUDRefs creates a disposable pixel view from exact typed refs.
// The returned map is never persisted alongside the canonical render state.
func MaterializeHUDRefs(state renderstate.HUDState, catalog HUDCatalog) (HUDState, error) {
	if state.Slots == nil || len(state.Slots) > 2048 || state.NextNameTile < 32 || state.NextNameTile > 2048 {
		return HUDState{}, errors.New("HUD ref state is unavailable")
	}
	out := HUDState{Cells: state.Cells, Rules: state.Rules, NextNameTile: state.NextNameTile, Slots: make(map[uint16]Tile, len(state.Slots))}
	for slot, ref := range state.Slots {
		if slot >= 2048 {
			return HUDState{}, errors.New("HUD ref slot exceeds logical bank")
		}
		var tile Tile
		switch ref.Kind {
		case renderstate.KindBase:
			if ref.Key >= 16 || ref.Index != 0 {
				return HUDState{}, errors.New("invalid HUD base ref")
			}
			tile = catalog.Base[ref.Key]
		case renderstate.KindBar:
			if ref.Key >= 32 || ref.Index != 0 {
				return HUDState{}, errors.New("invalid HUD bar ref")
			}
			tile = catalog.Patterns[ref.Key]
		case renderstate.KindGlyph:
			tiles, ok := catalog.Glyphs[ref.Key]
			if !ok || ref.Index >= 4 {
				return HUDState{}, errors.New("invalid HUD glyph ref")
			}
			tile = tiles[ref.Index]
		case renderstate.KindPortrait:
			tiles, ok := catalog.Portraits[ref.Key]
			if !ok || ref.Index >= 16 {
				return HUDState{}, errors.New("invalid HUD portrait ref")
			}
			tile = tiles[ref.Index]
		default:
			return HUDState{}, errors.New("unknown HUD ref kind")
		}
		out.Slots[slot] = tile
	}
	for _, word := range state.Cells {
		if _, ok := out.Slots[word&2047]; !ok {
			return HUDState{}, errors.New("HUD ref cell is unavailable")
		}
	}
	return out, nil
}
