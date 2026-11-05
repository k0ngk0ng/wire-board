package presentation

import (
	"encoding/hex"
	"encoding/json"
	"errors"
)

// ArenaCell refers to a local immutable tile, never a hardware tile address.
type ArenaCell struct {
	Tile  uint16 `json:"tile"`
	Flags uint16 `json:"flags"`
}

// IndexedPlane holds the normal-mode 64-by-32 map used by the default arena.
// A separately constructed window uses the same format and its own tile bank.
type IndexedPlane struct {
	Columns, Rows int
	Tiles         []Tile
	Cells         []ArenaCell
}

type ArenaCatalog struct {
	Planes   [2]IndexedPlane // A, B. The immutable tile bank is shared.
	Palette0 [16]uint16      // Initial compact colors; not the current world colors.
	identity ArenaIdentity
}

// ArenaRow supplies actual display inputs for one visible or blank row. These
// are presentation values, not emulated video registers or an interrupt image.
type ArenaRow struct {
	Horizontal, Vertical [2]uint16
	Palette              [64]uint16 // Compact three-bit R/G/B channels.
	Backdrop             uint8
	Visible, Limited     bool
	Window               bool // Replace plane A with the separate unscrolled window.
}

type ArenaFrame struct {
	Width, Height int
	Rows          []ArenaRow
	Window        *IndexedPlane
}

// ArenaRaster keeps source opacity distinct from the backdrop. Visible gates
// sprite composition by row; Sources is 0=backdrop, 1=A, 2=B, 3=window.
type ArenaRaster struct {
	Raster
	Visible []bool
	Sources []uint8
}

func validIndexedPlane(plane IndexedPlane) bool {
	if plane.Columns != 64 || plane.Rows != 32 || len(plane.Cells) != 64*32 || len(plane.Tiles) == 0 || len(plane.Tiles) > 65536 {
		return false
	}
	for _, cell := range plane.Cells {
		if int(cell.Tile) >= len(plane.Tiles) || cell.Flags&0x7ff != 0 {
			return false
		}
	}
	return true
}

// DecodeArenaCatalog validates the compact default A/B asset. Dynamic colors,
// scrolling, visible rows and HUD/window material remain explicit frame inputs.
func DecodeArenaCatalog(raw []byte) (ArenaCatalog, error) {
	return DecodeArenaCatalogForArena(raw, 0)
}

func decodeArenaCatalogForIdentity(raw []byte, identity ArenaIdentity) (ArenaCatalog, error) {
	var source struct {
		Schema        int    `json:"schema"`
		ROMSHA        string `json:"rom_sha256"`
		ArenaOffset   uint16 `json:"arena_offset"`
		SceneBehavior uint32 `json:"scene_behavior"`
		Binding       struct {
			ArenaOffset   uint16 `json:"arena_offset"`
			SceneBehavior uint32 `json:"scene_behavior"`
			ResourceList  uint32 `json:"resource_list"`
		} `json:"resource_binding"`
		Tiles   []string `json:"packed_4bpp_tiles"`
		Palette []uint16 `json:"initial_scene_palette_0"`
		Planes  []struct {
			Name          string `json:"name"`
			Columns, Rows int
			Cells         []ArenaCell
		}
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return ArenaCatalog{}, err
	}
	if source.Schema != 1 || source.ROMSHA != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || source.ArenaOffset != identity.ArenaOffset || source.SceneBehavior != identity.SceneBehavior || source.Binding.ArenaOffset != identity.ArenaOffset || source.Binding.SceneBehavior != identity.SceneBehavior || source.Binding.ResourceList != identity.ResourceList || len(source.Planes) != 2 || len(source.Tiles) == 0 || len(source.Tiles) > 65536 || len(source.Palette) != 16 {
		if identity.ArenaOffset == 0 {
			return ArenaCatalog{}, errors.New("invalid default indexed arena identity")
		}
		return ArenaCatalog{}, errors.New("invalid selected indexed arena identity")
	}
	tiles := make([]Tile, len(source.Tiles))
	for i, encoded := range source.Tiles {
		packed, err := hex.DecodeString(encoded)
		if err != nil || len(packed) != 32 {
			return ArenaCatalog{}, errors.New("invalid indexed arena tile")
		}
		copy(tiles[i][:], packed)
	}
	catalog := ArenaCatalog{identity: identity}
	for i, plane := range source.Planes {
		catalog.Planes[i] = IndexedPlane{Columns: plane.Columns, Rows: plane.Rows, Tiles: tiles, Cells: plane.Cells}
		if plane.Name != [2]string{"A", "B"}[i] || !validIndexedPlane(catalog.Planes[i]) {
			return ArenaCatalog{}, errors.New("invalid indexed arena plane")
		}
	}
	for i, color := range source.Palette {
		if color > 511 {
			return ArenaCatalog{}, errors.New("invalid initial arena color")
		}
		catalog.Palette0[i] = color
	}
	return catalog, nil
}

func indexedPlanePixel(plane IndexedPlane, x, y int) (uint8, uint16) {
	x, y = x&511, y&255
	cell := plane.Cells[(y/8)*64+x/8]
	u, v := x&7, y&7
	if cell.Flags&0x800 != 0 {
		u = 7 - u
	}
	if cell.Flags&0x1000 != 0 {
		v = 7 - v
	}
	value := plane.Tiles[cell.Tile][v*4+u/2] >> uint(4*(1-u&1)) & 15
	return value, cell.Flags
}

// RasterizeArena independently selects the normal-mode A/B/window pixel for
// every row. A zero tile index is transparent even when a palette is selected;
// a nonzero index whose color is black remains opaque and keeps its priority.
func RasterizeArena(catalog ArenaCatalog, frame ArenaFrame) (ArenaRaster, error) {
	if frame.Width < 1 || frame.Width > 512 || frame.Height < 1 || frame.Height > 256 || len(frame.Rows) != frame.Height || !validIndexedPlane(catalog.Planes[0]) || !validIndexedPlane(catalog.Planes[1]) {
		return ArenaRaster{}, errors.New("invalid independent arena geometry or planes")
	}
	if frame.Window != nil && !validIndexedPlane(*frame.Window) {
		return ArenaRaster{}, errors.New("invalid independent window plane")
	}
	for _, row := range frame.Rows {
		if row.Backdrop >= 64 || row.Window && frame.Window == nil {
			return ArenaRaster{}, errors.New("unavailable arena row backdrop or window")
		}
		for _, color := range row.Palette {
			if color > 511 {
				return ArenaRaster{}, errors.New("invalid current arena row color")
			}
		}
	}
	n := frame.Width * frame.Height
	out := ArenaRaster{Raster: Raster{Width: frame.Width, Height: frame.Height, Colors: make([]uint16, n), Opaque: make([]bool, n), HighPriority: make([]bool, n), Owners: make([]uint16, n)}, Visible: make([]bool, frame.Height), Sources: make([]uint8, n)}
	for y, row := range frame.Rows {
		out.Visible[y] = row.Visible
		for x := 0; x < frame.Width; x++ {
			pos := y*frame.Width + x
			index := row.Backdrop
			if row.Visible {
				a, af := indexedPlanePixel(catalog.Planes[0], x-int(row.Horizontal[0]), y+int(row.Vertical[0]))
				b, bf := indexedPlanePixel(catalog.Planes[1], x-int(row.Horizontal[1]), y+int(row.Vertical[1]))
				if row.Window {
					a, af = indexedPlanePixel(*frame.Window, x, y)
				}
				if b != 0 {
					index, out.Sources[pos] = uint8(bf>>9&0x30)|b, 2
					out.Opaque[pos], out.HighPriority[pos] = true, bf&0x8000 != 0
				}
				if a != 0 && (b == 0 || af&0x8000 != 0 || bf&0x8000 == 0) {
					index, out.Sources[pos] = uint8(af>>9&0x30)|a, 1
					if row.Window {
						out.Sources[pos] = 3
					}
					out.Opaque[pos], out.HighPriority[pos] = true, af&0x8000 != 0
				}
			}
			out.Colors[pos] = color565(row.Palette[index], row.Limited)
		}
	}
	return out, nil
}

func validRaster(raster Raster) bool {
	n := raster.Width * raster.Height
	return raster.Width >= 1 && raster.Width <= 8192 && raster.Height >= 1 && raster.Height <= 8192 && len(raster.Colors) == n && len(raster.Opaque) == n && len(raster.HighPriority) == n && len(raster.Owners) == n
}

// CompositeArenaSprites merges the already ordered sprite plane with the
// selected background. The completed screen is opaque, including blank rows
// and backdrop colors. Source buffers are never mutated.
func CompositeArenaSprites(background ArenaRaster, sprites Raster) (Raster, error) {
	if !validRaster(background.Raster) || !validRaster(sprites) || background.Width != sprites.Width || background.Height != sprites.Height || len(background.Visible) != background.Height || len(background.Sources) != len(background.Colors) {
		return Raster{}, errors.New("incompatible or incomplete arena and sprite rasters")
	}
	out := Raster{Width: background.Width, Height: background.Height, Colors: append([]uint16(nil), background.Colors...), Opaque: make([]bool, len(background.Colors)), HighPriority: append([]bool(nil), background.HighPriority...), Owners: append([]uint16(nil), background.Owners...)}
	for i := range out.Colors {
		out.Opaque[i] = true
		if background.Visible[i/out.Width] && sprites.Opaque[i] && (sprites.HighPriority[i] || !background.Opaque[i] || !background.HighPriority[i]) {
			out.Colors[i], out.HighPriority[i], out.Owners[i] = sprites.Colors[i], sprites.HighPriority[i], sprites.Owners[i]
		}
	}
	return out, nil
}
