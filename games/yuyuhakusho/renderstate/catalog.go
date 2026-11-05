// Package renderstate maintains bounded pixel-free presentation metadata.
// It imports no client catalog, renderer, media, VM, or filesystem facility.
package renderstate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

type Identity struct {
	RulesSHA256         string `json:"rules_sha256"`
	SpriteCatalogSHA256 string `json:"sprite_catalog_sha256"`
	HUDCatalogSHA256    string `json:"hud_catalog_sha256"`
	MetadataSHA256      string `json:"metadata_sha256"`
}
type frameKey struct {
	GraphicsSet uint32 `json:"graphics_set"`
	Parameter   uint16 `json:"parameter"`
}
type transfer struct {
	Stamp uint32   `json:"stamp"`
	Tiles []uint32 `json:"tiles"`
}
type frame struct {
	Key           frameKey   `json:"key"`
	TileBank      uint32     `json:"tile_bank"`
	PreloadedOnly bool       `json:"preloaded_only"`
	Transfers     []transfer `json:"transfers"`
	PieceCount    uint8      `json:"piece_count"`
}
type binding struct {
	Slot uint16 `json:"slot"`
	Tile uint32 `json:"tile"`
}
type Library struct {
	identity  Identity
	frames    map[frameKey]frame
	tileCount uint32
	defaults  []binding
	glyphs    map[uint16]bool
	portraits map[uint16]bool
	resources [8]uint32
}

func (l Library) Identity() Identity { return l.identity }
func digest(raw []byte) string       { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }
func validSHA(s string) bool {
	if len(s) != 64 {
		return false
	}
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && hex.EncodeToString(b) == s
}

// DecodeLibrary validates immutable computational metadata once before play.
func DecodeLibrary(raw []byte) (Library, error) {
	var empty Library
	if len(raw) == 0 || len(raw) > 4<<20 {
		return empty, errors.New("invalid render metadata size")
	}
	var data struct {
		Schema     int       `json:"schema"`
		RulesSHA   string    `json:"rules_sha256"`
		SpritesSHA string    `json:"sprites_sha256"`
		HUDSHA     string    `json:"hud_sha256"`
		TileCount  uint32    `json:"tile_count"`
		Frames     []frame   `json:"frames"`
		Defaults   []binding `json:"default_static_slots"`
		Resources  []uint32  `json:"arena_resources"`
		HUD        struct {
			Resource  uint32   `json:"resource_list"`
			BaseCount int      `json:"base_count"`
			BarCount  int      `json:"bar_count"`
			Glyphs    []uint16 `json:"glyph_codes"`
			Portraits []uint16 `json:"portrait_parameters"`
			Stripe    struct {
				Row        int    `json:"row"`
				Columns    int    `json:"columns"`
				Attributes uint16 `json:"attributes"`
			} `json:"window_stripe"`
		} `json:"hud"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&data); e != nil {
		return empty, e
	}
	if d.Decode(new(any)) != io.EOF {
		return empty, errors.New("trailing render metadata")
	}
	if data.Schema != 1 || !validSHA(data.RulesSHA) || !validSHA(data.SpritesSHA) || !validSHA(data.HUDSHA) || data.TileCount == 0 || data.TileCount > 1<<20 || len(data.Frames) != 1500 || len(data.Defaults) != 320 || len(data.Resources) != 8 || data.HUD.Resource != 0x21720 || data.HUD.BaseCount != 16 || data.HUD.BarCount != 32 || len(data.HUD.Glyphs) != 20 || len(data.HUD.Portraits) != 11 || data.HUD.Stripe.Row != 1 || data.HUD.Stripe.Columns != 40 || data.HUD.Stripe.Attributes != 0xe00d {
		return empty, errors.New("incomplete render metadata identity or domains")
	}
	l := Library{identity: Identity{data.RulesSHA, data.SpritesSHA, data.HUDSHA, digest(raw)}, frames: make(map[frameKey]frame), tileCount: data.TileCount, defaults: data.Defaults, glyphs: make(map[uint16]bool), portraits: make(map[uint16]bool)}
	for _, f := range data.Frames {
		if f.Key.GraphicsSet == 0 || f.Key.Parameter&1 != 0 || f.PieceCount == 0 || f.PieceCount > 80 || len(f.Transfers) == 0 || len(f.Transfers) > 4 {
			return empty, errors.New("invalid render frame")
		}
		if _, ok := l.frames[f.Key]; ok {
			return empty, errors.New("duplicate render frame")
		}
		for _, tr := range f.Transfers {
			if tr.Stamp&0x80000000 != 0 || !f.PreloadedOnly && len(tr.Tiles) != int(uint8(tr.Stamp)) || f.PreloadedOnly && len(tr.Tiles) != 0 {
				return empty, errors.New("invalid render transfer")
			}
			for _, tile := range tr.Tiles {
				if tile >= data.TileCount {
					return empty, errors.New("unavailable render tile identity")
				}
			}
		}
		l.frames[f.Key] = f
	}
	var used [2048]bool
	for _, b := range data.Defaults {
		if b.Slot >= 2048 || b.Tile >= data.TileCount || used[b.Slot] {
			return empty, errors.New("invalid or duplicate preload binding")
		}
		used[b.Slot] = true
	}
	for _, code := range data.HUD.Glyphs {
		if l.glyphs[code] {
			return empty, errors.New("duplicate HUD glyph domain")
		}
		l.glyphs[code] = true
	}
	if !l.glyphs[0] {
		return empty, errors.New("missing HUD blank glyph")
	}
	for _, code := range data.HUD.Portraits {
		if code < 2 || code > 22 || code&1 != 0 || l.portraits[code] {
			return empty, errors.New("invalid HUD portrait domain")
		}
		l.portraits[code] = true
	}
	expected := [8]uint32{0x21720, 0x2175c, 0x21798, 0x217cc, 0x21800, 0x21834, 0x21870, 0x218a4}
	for i, v := range data.Resources {
		if v != expected[i] {
			return empty, errors.New("invalid selected arena metadata resource")
		}
		l.resources[i] = v
	}
	return l, nil
}
