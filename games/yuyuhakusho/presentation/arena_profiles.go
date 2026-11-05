package presentation

import (
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// ArenaIdentity binds a decoded catalog to one of the eight recovered scenes.
// It is returned by value and is not part of a saved presentation payload.
type ArenaIdentity struct {
	ArenaOffset   uint16
	SceneBehavior uint32
	ResourceList  uint32
}

func (catalog ArenaCatalog) Identity() ArenaIdentity {
	return catalog.identity
}

func arenaIdentity(arena uint16) (ArenaIdentity, error) {
	if arena > 14 || arena&1 != 0 {
		return ArenaIdentity{}, errors.New("unavailable indexed arena selection")
	}
	bindings := [8][2]uint32{
		{0x20918, 0x21720}, {0x20ae0, 0x2175c},
		{0x20cb2, 0x21798}, {0x20e28, 0x217cc},
		{0x20fb4, 0x21800}, {0x21126, 0x21834},
		{0x212f8, 0x21870}, {0x21412, 0x218a4},
	}
	binding := bindings[arena/2]
	return ArenaIdentity{ArenaOffset: arena, SceneBehavior: binding[0], ResourceList: binding[1]}, nil
}

// DecodeArenaCatalogForArena accepts only the ROM-bound identity of the
// explicitly selected recovered scene. The legacy entry remains arena0-only.
func DecodeArenaCatalogForArena(raw []byte, arena uint16) (ArenaCatalog, error) {
	identity, err := arenaIdentity(arena)
	if err != nil {
		return ArenaCatalog{}, err
	}
	return decodeArenaCatalogForIdentity(raw, identity)
}

// ArenaAAuxCatalog preserves the original sequential48 tiles separately from
// the deduplicated background. Its three4-by-4 groups start at local0/16/32.
type ArenaAAuxCatalog struct {
	Tiles [48]Tile
}

func DecodeArenaAAuxCatalog(raw []byte) (ArenaAAuxCatalog, error) {
	var source struct {
		Schema   int      `json:"schema"`
		ROMSHA   string   `json:"rom_sha256"`
		Arena    uint16   `json:"arena_offset"`
		Behavior uint32   `json:"scene_behavior"`
		Callback uint32   `json:"callback"`
		Resource uint32   `json:"resource_source"`
		Tiles    []string `json:"packed_4bpp_tiles"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return ArenaAAuxCatalog{}, err
	}
	if source.Schema != 1 || source.ROMSHA != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || source.Arena != 10 || source.Behavior != 0x21126 || source.Callback != 0x21258 || source.Resource != 0x297056 || len(source.Tiles) != 48 {
		return ArenaAAuxCatalog{}, errors.New("invalid arena auxiliary asset identity")
	}
	var catalog ArenaAAuxCatalog
	for i, encoded := range source.Tiles {
		packed, err := hex.DecodeString(encoded)
		if err != nil || len(packed) != 32 {
			return ArenaAAuxCatalog{}, errors.New("invalid arena auxiliary indexed tile")
		}
		copy(catalog.Tiles[i][:], packed)
	}
	return catalog, nil
}

// ArenaRowsForArena uses the observed normal-mode display shared by all eight
// recovered arenas. Current scrolling, palette and backdrop remain explicit.
// ID4 may advance unused blank-row colors9..11 before the frame-end palette;
// this profile preserves visible colors and final pixels, not those unused phases.
func ArenaRowsForArena(arena uint16, backdrop uint8, palette [64]uint16, horizontal, vertical [224][2]uint16) ([224]ArenaRow, error) {
	if _, err := arenaIdentity(arena); err != nil {
		return [224]ArenaRow{}, err
	}
	if backdrop >= 64 {
		return [224]ArenaRow{}, errors.New("invalid indexed arena backdrop")
	}
	rows, err := DefaultArenaRows(palette, horizontal, vertical)
	if err != nil {
		return [224]ArenaRow{}, err
	}
	for y := range rows {
		rows[y].Backdrop = backdrop
	}
	return rows, nil
}

// ApplyHUDWorldEventsForArena keeps shared HUD artwork bound to the selected
// scene's resource event. It leaves the legacy default-only entry unchanged.
func ApplyHUDWorldEventsForArena(state HUDState, events []engine.ChuWorldEvent, cells []uint16, catalog HUDCatalog, arena uint16) (HUDState, error) {
	identity, err := arenaIdentity(arena)
	if err != nil {
		return state, err
	}
	return applyHUDWorldEvents(state, events, cells, catalog, identity.ResourceList)
}
