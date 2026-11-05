package presentation

import (
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func TestArenaAuxSpriteSubmissionOrderAndBudget(t *testing.T) {
	for _, initial := range []uint16{2, 79, 80, 127, 128, 255, 0x1234} {
		state := FrameState{Count: initial, Flags: 0, Parts: []Part{{Owner: 0x8400, Layer: 0, Index: 1}}}
		before := state
		before.Parts = append([]Part(nil), state.Parts...)
		raw := engine.ArenaASprite{Y: 160, SizeLink: 0x0ffc, Tile: 0x5e0, X: 192}
		got, err := ComposeArenaASprite(state, raw)
		if err != nil || !reflect.DeepEqual(state, before) || got.Flags != state.Flags || got.Slots != state.Slots {
			t.Fatal("raw submission changed caller, flags or persistent slots", initial, err)
		}
		if int8(uint8(initial)) >= 80 {
			if !reflect.DeepEqual(got, state) {
				t.Fatal("raw over-budget submission changed state", initial)
			}
			continue
		}
		want := Part{Owner: 0x9e00, Layer: 0xfc, Index: uint8(initial), X: 192, Y: 160, WidthTiles: 4, HeightTiles: 4, TileSlot: 16, ArenaAux: true}
		if len(got.Parts) != 2 || got.Parts[1] != want || got.Count != initial&0xff00|uint16(uint8(initial+1)) {
			t.Fatal("raw descriptor lost immediate geometry, layer or byte count", initial, got)
		}
	}
	state := BeginFrame(TileSlots{}, 79)
	for _, tile := range []uint16{0x5d0, 0x5d0, 0x5e0, 0x5e0, 0x5f0, 0x5f0} {
		var err error
		state, err = ComposeArenaASprite(state, engine.ArenaASprite{Y: 128, SizeLink: 0x0ffc, Tile: tile, X: 128})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(state.Parts) != 1 || state.Count != 80 || state.Parts[0].Index != 79 || state.Flags != 0x80 {
		t.Fatal("six raw calls did not share the original sprite budget", state)
	}
	for _, raw := range []engine.ArenaASprite{{SizeLink: 0x0ffc, Tile: 0x5d1}, {SizeLink: 0x0ffc, Tile: 0x85d0}, {SizeLink: 0x0ff8, Tile: 0x5d0}, {SizeLink: 0x0efc, Tile: 0x5d0}} {
		got, err := ComposeArenaASprite(state, raw)
		if err == nil || !reflect.DeepEqual(got, state) {
			t.Fatal("unknown arena submission was not rejected atomically", raw, err)
		}
	}
}

func TestArenaAuxSpriteIndependentTileBank(t *testing.T) {
	var aux ArenaAAuxCatalog
	for i := range aux.Tiles {
		for j := range aux.Tiles[i] {
			value := byte(i%15 + 1)
			aux.Tiles[i][j] = value<<4 | value
		}
	}
	var palette [64]uint16
	for i := range palette {
		palette[i] = uint16(i)
	}
	var tile Tile
	for i := range tile {
		tile[i] = 0x44
	}
	catalog := Catalog{Tiles: []Tile{tile}}
	state := BeginFrame(TileSlots{}, 2)
	_ = state.Slots.Set(0, 0)
	for group, tile := range []uint16{0x5d0, 0x5e0, 0x5f0} {
		var err error
		state, err = ComposeArenaASprite(state, engine.ArenaASprite{Y: 128, SizeLink: 0x0ffc, Tile: tile, X: uint16(128 + group*32)})
		if err != nil {
			t.Fatal(err)
		}
	}
	raster, err := RasterizeSpritesForArena(state, catalog, &aux, palette, 96, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 32; y++ {
		for x := 0; x < 96; x++ {
			local := (x/32)*16 + ((x%32)/8)*4 + y/8
			index := y*96 + x
			if !raster.Opaque[index] || raster.Owners[index] != 0x9e00 || raster.HighPriority[index] || raster.Colors[index] != color565(palette[local%15+1], false) {
				t.Fatal("auxiliary bank lost column-major local tiles, geometry or palette", x, y, local)
			}
		}
	}
	if _, err := RasterizeSprites(state, catalog, palette, 96, 32, false); err == nil {
		t.Fatal("legacy rasterizer accepted an unavailable auxiliary bank")
	}
	state.Parts = append(state.Parts, Part{Owner: 0x8400, Layer: 0, Index: 8, X: 128, Y: 128, WidthTiles: 1, HeightTiles: 1, TileSlot: 0, Palette: 1, HighPriority: true})
	raster, err = RasterizeSpritesForArena(state, catalog, &aux, palette, 96, 32, false)
	if err != nil || raster.Owners[0] != 0x8400 || raster.Colors[0] != color565(palette[20], false) || !raster.HighPriority[0] || raster.Owners[8] != 0x9e00 {
		t.Fatal("normal and auxiliary banks lost common stable layer ordering", err)
	}
	state.Parts[0].TileSlot = 33
	if _, err := RasterizeSpritesForArena(state, catalog, &aux, palette, 96, 32, false); err == nil {
		t.Fatal("auxiliary block crosses the immutable tile bank")
	}
}
