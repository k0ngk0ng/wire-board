package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"reflect"
)

type arenaResource struct {
	Kind         uint16 `json:"kind"`
	Destination  uint16 `json:"destination"`
	Source       uint32 `json:"source"`
	Length       uint16 `json:"length,omitempty"`
	EncodedBytes uint16 `json:"encoded_bytes,omitempty"`
	EncodedSHA   string `json:"encoded_sha256,omitempty"`
}

type arenaBinding struct {
	ArenaOffset   uint16 `json:"arena_offset"`
	SceneBehavior uint32 `json:"scene_behavior"`
	ResourceList  uint32 `json:"resource_list"`
	ListSHA       string `json:"list_sha256"`
	Resources     []arenaResource
}

type arenaCell struct {
	Tile  uint16 `json:"tile"`
	Flags uint16 `json:"flags"` // Palette, flips and priority; no hardware tile number.
}

type arenaPlaneAsset struct {
	Name    string      `json:"name"`
	Columns int         `json:"columns"`
	Rows    int         `json:"rows"`
	Cells   []arenaCell `json:"cells"`
}

type arenaCatalog struct {
	Schema        int               `json:"schema"`
	ROMSHA        string            `json:"rom_sha256"`
	ArenaOffset   uint16            `json:"arena_offset"`
	SceneBehavior uint32            `json:"scene_behavior"`
	Binding       arenaBinding      `json:"resource_binding"`
	Tiles         []string          `json:"packed_4bpp_tiles"`
	Planes        []arenaPlaneAsset `json:"planes"`
	Palette0      []uint16          `json:"initial_scene_palette_0"`
	RuntimeInputs []string          `json:"runtime_inputs"`
	Scope         string            `json:"scope"`
}

func compactArenaColor(word uint16) uint16 {
	return word>>1&7 | word>>2&0x38 | word>>3&0x1c0
}

// buildArenaCatalog stores indexed 4bpp material and complete static A/B name
// maps. It omits the captured character-specific window and runtime raster
// arrays, which must come from the independent HUD/scene state rather than a
// prerecorded video or a VRAM image.
func buildArenaCatalog(f arenaReference) (arenaCatalog, error) {
	c := arenaCatalog{Schema: 1, ROMSHA: f.Identity.ROMSHA, ArenaOffset: 0, SceneBehavior: 0x20918,
		RuntimeInputs: []string{"horizontal scroll rows for A/B", "vertical scroll for A/B", "visible/blank rows", "current palette including raster backdrop changes", "separate window/HUD plane"},
		Scope:         "Default arena0 static A/B indexed material only; scene updates, camera and HUD are separate"}
	if len(f.Frames) == 0 || len(f.Bindings) != 8 || len(f.PaletteWords) != 16 {
		return c, errors.New("missing original arena source bindings")
	}
	c.Binding = f.Bindings[0]
	for _, value := range f.PaletteWords {
		c.Palette0 = append(c.Palette0, compactArenaColor(value))
	}
	frame := f.Frames[0]
	if len(frame.Registers) != 32 {
		return c, errors.New("missing original arena registers")
	}
	vram, err := arenaBytes(frame.ActiveVRAM, 65536)
	if err != nil {
		return c, err
	}
	tiles := make(map[string]uint16)
	for i, base := range []int{int(frame.Registers[2]&0x38) << 10, int(frame.Registers[4]&7) << 13} {
		p := arenaPlaneAsset{Name: []string{"A", "B"}[i], Columns: 64, Rows: 32}
		for cell := 0; cell < 64*32; cell++ {
			attribute := binary.BigEndian.Uint16(vram[base+cell*2:])
			start := int(attribute&0x7ff) * 32
			packed := hex.EncodeToString(vram[start : start+32])
			id, ok := tiles[packed]
			if !ok {
				id = uint16(len(c.Tiles))
				tiles[packed] = id
				c.Tiles = append(c.Tiles, packed)
			}
			p.Cells = append(p.Cells, arenaCell{Tile: id, Flags: attribute & 0xf800})
		}
		c.Planes = append(c.Planes, p)
	}
	return c, verifyArenaCatalog(c, f)
}

func verifyArenaCatalog(c arenaCatalog, f arenaReference) error {
	if c.Schema != 1 || c.ROMSHA != f.Identity.ROMSHA || c.ArenaOffset != 0 || c.SceneBehavior != 0x20918 || len(f.Bindings) != 8 || !reflect.DeepEqual(c.Binding, f.Bindings[0]) || len(c.Planes) != 2 || len(c.Tiles) == 0 || len(c.Palette0) != 16 || len(f.PaletteWords) != 16 {
		return errors.New("invalid compact arena identity")
	}
	observed := false
	for _, load := range f.ObservedLoads {
		if load.ResourceList == c.Binding.ResourceList {
			observed = true
		}
	}
	if !observed || c.Binding.ArenaOffset != 0 || c.Binding.SceneBehavior != c.SceneBehavior || c.Binding.ResourceList != 0x21720 {
		return errors.New("arena resource list was not observed in original loading")
	}
	for i, word := range f.PaletteWords {
		if c.Palette0[i] != compactArenaColor(word) {
			return errors.New("scene palette does not match original resource words")
		}
	}
	bank := make([][]byte, len(c.Tiles))
	used := make([]bool, len(bank))
	for i, encoded := range c.Tiles {
		var err error
		bank[i], err = arenaBytes(encoded, 32)
		if err != nil {
			return err
		}
	}
	initialPaletteObserved := false
	for _, frame := range f.Frames {
		if frame.ArenaOffset != c.ArenaOffset || frame.ActiveVRAMChanged || len(frame.Registers) != 32 {
			return errors.New("compact arena only covers the fixed original static planes")
		}
		vram, err := arenaBytes(frame.ActiveVRAM, 65536)
		if err != nil {
			return err
		}
		ram, err := arenaBytes(frame.RAM, 65536)
		if err != nil {
			return err
		}
		paletteMatches := true
		for i, word := range f.PaletteWords {
			if binary.BigEndian.Uint16(ram[0xe080+i*2:]) != word {
				paletteMatches = false
			}
		}
		initialPaletteObserved = initialPaletteObserved || paletteMatches
		for i, plane := range c.Planes {
			if plane.Name != []string{"A", "B"}[i] || plane.Columns != 64 || plane.Rows != 32 || len(plane.Cells) != 64*32 {
				return errors.New("invalid arena name-map geometry")
			}
			base := int(frame.Registers[2]&0x38) << 10
			if i == 1 {
				base = int(frame.Registers[4]&7) << 13
			}
			for cell, data := range plane.Cells {
				attribute := binary.BigEndian.Uint16(vram[base+cell*2:])
				if int(data.Tile) >= len(bank) || data.Flags != attribute&0xf800 || data.Flags&0x7ff != 0 {
					return errors.New("arena name-map cell differs from original")
				}
				used[data.Tile] = true
				start := int(attribute&0x7ff) * 32
				if !reflect.DeepEqual(bank[data.Tile], vram[start:start+32]) {
					return errors.New("indexed arena tile differs from original loaded graphics")
				}
			}
		}
	}
	if !initialPaletteObserved {
		return errors.New("initial scene palette was never observed in loaded original state")
	}
	for _, present := range used {
		if !present {
			return errors.New("unreferenced extra material in compact arena bank")
		}
	}
	return nil
}
