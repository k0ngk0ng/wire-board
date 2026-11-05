package presentation

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// ComposeArenaASprite consumes one callback-time submission. It shares the
// ordered part list and byte-sized budget, but never uses an actor's culling,
// cache, transfer or display flag gates. The auxiliary bank remains immutable.
func ComposeArenaASprite(state FrameState, sprite engine.ArenaASprite) (FrameState, error) {
	if sprite.SizeLink != 0x0ffc || (sprite.Tile != 0x5d0 && sprite.Tile != 0x5e0 && sprite.Tile != 0x5f0) {
		return state, errors.New("unsupported arena A sprite descriptor")
	}
	if int8(uint8(state.Count)) >= 80 {
		return state, nil
	}
	next := state
	next.Parts = append(append([]Part(nil), state.Parts...), Part{
		Owner: 0x9e00, Layer: 0xfc, Index: uint8(state.Count), X: sprite.X, Y: sprite.Y,
		WidthTiles: 4, HeightTiles: 4, TileSlot: sprite.Tile - 0x5d0, ArenaAux: true,
	})
	next.Count = state.Count&0xff00 | uint16(uint8(state.Count+1))
	return next, nil
}
