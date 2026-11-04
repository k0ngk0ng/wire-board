package presentation

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// HUDCatalog contains only immutable indexed artwork extracted from original
// resources. No captured window, CPU image or decompression program is retained.
type HUDCatalog struct {
	Base      [16]Tile
	Patterns  [32]Tile
	Glyphs    map[uint16][4]Tile
	Portraits map[uint16][16]Tile
}

func DecodeHUDCatalog(raw []byte) (HUDCatalog, error) {
	var source struct {
		Schema   int      `json:"schema"`
		ROMSHA   string   `json:"rom_sha256"`
		Resource uint32   `json:"resource_list"`
		Base     []string `json:"base_tiles"`
		Patterns []string `json:"bar_tiles"`
		Glyphs   []struct {
			Code  uint16   `json:"code"`
			Tiles []string `json:"tiles"`
		} `json:"glyphs"`
		Portraits []struct {
			Parameter uint16   `json:"parameter"`
			Tiles     []string `json:"tiles"`
		} `json:"portraits"`
		Stripe struct {
			Row, Columns int
			Attributes   uint16
		} `json:"window_stripe"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return HUDCatalog{}, err
	}
	if source.Schema != 1 || source.ROMSHA != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || source.Resource != 0x21720 || len(source.Base) != 16 || len(source.Patterns) != 32 || len(source.Glyphs) == 0 || len(source.Glyphs) > 64 || len(source.Portraits) != 11 || source.Stripe.Row != 1 || source.Stripe.Columns != 40 || source.Stripe.Attributes != 0xe00d {
		return HUDCatalog{}, errors.New("invalid default HUD asset identity")
	}
	decode := func(encoded string) (Tile, error) {
		var tile Tile
		b, err := hex.DecodeString(encoded)
		if err != nil || len(b) != 32 {
			return tile, errors.New("invalid indexed HUD tile")
		}
		copy(tile[:], b)
		return tile, nil
	}
	catalog := HUDCatalog{Glyphs: make(map[uint16][4]Tile), Portraits: make(map[uint16][16]Tile)}
	for i, encoded := range source.Base {
		tile, err := decode(encoded)
		if err != nil {
			return HUDCatalog{}, err
		}
		catalog.Base[i] = tile
	}
	for i, encoded := range source.Patterns {
		tile, err := decode(encoded)
		if err != nil {
			return HUDCatalog{}, err
		}
		catalog.Patterns[i] = tile
	}
	for _, glyph := range source.Glyphs {
		if _, ok := catalog.Glyphs[glyph.Code]; ok || len(glyph.Tiles) != 4 {
			return HUDCatalog{}, errors.New("invalid or duplicate HUD glyph")
		}
		var tiles [4]Tile
		for i, encoded := range glyph.Tiles {
			tile, err := decode(encoded)
			if err != nil {
				return HUDCatalog{}, err
			}
			tiles[i] = tile
		}
		catalog.Glyphs[glyph.Code] = tiles
	}
	for _, portrait := range source.Portraits {
		if _, ok := catalog.Portraits[portrait.Parameter]; ok || portrait.Parameter < 2 || portrait.Parameter > 22 || portrait.Parameter&1 != 0 || len(portrait.Tiles) != 16 {
			return HUDCatalog{}, errors.New("invalid or duplicate HUD portrait")
		}
		var tiles [16]Tile
		for i, encoded := range portrait.Tiles {
			tile, err := decode(encoded)
			if err != nil {
				return HUDCatalog{}, err
			}
			tiles[i] = tile
		}
		catalog.Portraits[portrait.Parameter] = tiles
	}
	if _, ok := catalog.Glyphs[0]; !ok {
		return HUDCatalog{}, errors.New("HUD blank name glyph unavailable")
	}
	return catalog, nil
}

// HUDState owns presentation cells, indexed tile assignments and the name
// allocation cursor. Rules marks the cells written by canonical rule helpers;
// names/portraits clear ownership wherever they overwrite a template cell.
type HUDState struct {
	Cells        [256]uint16
	Rules        [256]bool
	Slots        map[uint16]Tile
	NextNameTile uint16
}

func NewHUD(catalog HUDCatalog) (HUDState, error) {
	if len(catalog.Glyphs) == 0 || len(catalog.Portraits) != 11 {
		return HUDState{}, errors.New("HUD indexed assets unavailable")
	}
	state := HUDState{Slots: make(map[uint16]Tile), NextNameTile: 32}
	for i, tile := range catalog.Base {
		state.Slots[uint16(i)] = tile
	}
	return state, nil
}

// ApplyHUDWorldEvents consumes the actual ordered start/bar events. Canonical
// cells are an optional256-word E600 view; they update only rule-owned cells.
// The adapter never writes the engine world or derives life/energy values.
func ApplyHUDWorldEvents(state HUDState, events []engine.ChuWorldEvent, cells []uint16, catalog HUDCatalog) (HUDState, error) {
	original := state
	if len(cells) != 0 && len(cells) != 256 {
		return original, errors.New("HUD rules require the256-word E600 cell view")
	}
	if state.Slots == nil || state.NextNameTile < 32 || state.NextNameTile > 2048 {
		return original, errors.New("HUD presentation state is unavailable")
	}
	state.Slots = make(map[uint16]Tile, len(original.Slots)+16)
	for slot, tile := range original.Slots {
		if slot >= 2048 {
			return original, errors.New("HUD tile slot exceeds logical bank")
		}
		state.Slots[slot] = tile
	}
	for _, event := range events {
		if event.Kind == "hud" {
			if event.HUD.Pattern >= 32 || event.HUD.TargetTile < 16 || event.HUD.TargetTile > 23 {
				return original, errors.New("unsupported HUD bar assignment")
			}
			state.Slots[event.HUD.TargetTile] = catalog.Patterns[event.HUD.Pattern]
			continue
		}
		if event.Kind != "start" {
			continue
		}
		request := event.Start
		switch request.Kind {
		case "reset":
			var err error
			state, err = NewHUD(catalog)
			if err != nil {
				return original, err
			}
		case "resource":
			if request.Source != 0x21720 {
				return original, errors.New("HUD resource does not match default arena")
			}
		case "words":
			start := int(request.Offset) / 2
			if request.Offset&1 != 0 || len(request.Words) == 0 || start+len(request.Words) > len(state.Cells) {
				return original, errors.New("HUD word request exceeds visible shared cells")
			}
			for i, word := range request.Words {
				state.Cells[start+i], state.Rules[start+i] = word, true
			}
		case "name":
			start := int(request.Offset)/2 + 131
			if request.Offset&1 != 0 || start+69 >= len(state.Cells) || int(state.NextNameTile)+12 > 2048 {
				return original, errors.New("HUD name position or tile allocation unavailable")
			}
			for glyphIndex, code := range request.Glyphs {
				tiles, ok := catalog.Glyphs[code]
				if !ok {
					return original, errors.New("HUD name glyph unavailable")
				}
				for i, tile := range tiles {
					state.Slots[state.NextNameTile+uint16(i)] = tile
				}
				for col := 0; col < 2; col++ {
					for row := 0; row < 2; row++ {
						index := start + glyphIndex*2 + col + row*64
						state.Cells[index], state.Rules[index] = 0xe000|state.NextNameTile+uint16(col*2+row), false
					}
				}
				state.NextNameTile += 4
			}
		case "portrait":
			tiles, ok := catalog.Portraits[request.Parameter]
			if !ok || request.Attributes & ^uint16(0x6000|0x30) != 0x8300 {
				return original, errors.New("HUD portrait asset or attributes unavailable")
			}
			x, y := (int(request.X)-144)&^7, (int(request.Y)-144)&^7
			col, row := x/8, y/8
			if x < 0 || y < 0 || col+4 > 64 || row+4 > 4 {
				return original, errors.New("HUD portrait exceeds shared cell rectangle")
			}
			base := request.Attributes & 0x7ff
			for i, tile := range tiles {
				state.Slots[base+uint16(i)] = tile
			}
			for dx := 0; dx < 4; dx++ {
				for dy := 0; dy < 4; dy++ {
					index := (row+dy)*64 + col + dx
					state.Cells[index], state.Rules[index] = request.Attributes+uint16(dx*4+dy), false
				}
			}
		default:
			return original, errors.New("unsupported HUD startup event")
		}
	}
	if len(cells) != 0 {
		for i, owned := range state.Rules {
			if owned {
				state.Cells[i] = cells[i]
			}
		}
	}
	return state, nil
}

// HUDWindow constructs a normal indexed window from the fixed ROM-backed
// stripe, independently maintained HUD cells and current sparse tile map.
func HUDWindow(state HUDState) (IndexedPlane, error) {
	if state.Slots == nil {
		return IndexedPlane{}, errors.New("HUD tile state unavailable")
	}
	logical := make([]uint16, 0, len(state.Slots))
	for slot := range state.Slots {
		if slot >= 2048 {
			return IndexedPlane{}, errors.New("HUD logical tile outside bank")
		}
		logical = append(logical, slot)
	}
	sort.Slice(logical, func(i, j int) bool { return logical[i] < logical[j] })
	plane := IndexedPlane{Columns: 64, Rows: 32, Cells: make([]ArenaCell, 64*32)}
	indices := make(map[Tile]uint16)
	slots := make(map[uint16]uint16, len(logical))
	for _, slot := range logical {
		tile := state.Slots[slot]
		index, ok := indices[tile]
		if !ok {
			index = uint16(len(plane.Tiles))
			indices[tile] = index
			plane.Tiles = append(plane.Tiles, tile)
		}
		slots[slot] = index
	}
	for i := range plane.Cells {
		word := uint16(0)
		if i >= 64 && i < 104 {
			word = 0xe00d
		}
		if i >= 128 && i < 384 {
			word = state.Cells[i-128]
		}
		tile, ok := slots[word&0x7ff]
		if !ok {
			return IndexedPlane{}, errors.New("HUD window references an unavailable indexed tile")
		}
		plane.Cells[i] = ArenaCell{Tile: tile, Flags: word & 0xf800}
	}
	return plane, nil
}
