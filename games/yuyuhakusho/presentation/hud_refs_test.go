package presentation

import (
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
	"reflect"
	"testing"
)

func refHUDState(c HUDCatalog) renderstate.HUDState {
	s := renderstate.HUDState{Slots: make(map[uint16]renderstate.TileRef), NextNameTile: 32}
	for i := uint16(0); i < 16; i++ {
		s.Slots[i] = renderstate.TileRef{Kind: renderstate.KindBase, Key: i}
	}
	return s
}
func TestHUDRefMaterializationDomainsAndCopies(t *testing.T) {
	c := loadHUD(t)
	s := refHUDState(c)
	slot := uint16(32)
	for key := range c.Glyphs {
		for i := uint8(0); i < 4; i++ {
			s.Slots[slot] = renderstate.TileRef{Kind: renderstate.KindGlyph, Key: key, Index: i}
			slot++
		}
	}
	for key := range c.Portraits {
		for i := uint8(0); i < 16; i++ {
			s.Slots[slot] = renderstate.TileRef{Kind: renderstate.KindPortrait, Key: key, Index: i}
			slot++
		}
	}
	for key := uint16(0); key < 32; key++ {
		s.Slots[slot] = renderstate.TileRef{Kind: renderstate.KindBar, Key: key}
		slot++
	}
	before := s
	before.Slots = make(map[uint16]renderstate.TileRef)
	for k, v := range s.Slots {
		before.Slots[k] = v
	}
	p, e := MaterializeHUDRefs(s, c)
	if e != nil || !reflect.DeepEqual(before, s) {
		t.Fatal("materialization mutated owner", e)
	}
	for logical, r := range s.Slots {
		var want Tile
		switch r.Kind {
		case renderstate.KindBase:
			want = c.Base[r.Key]
		case renderstate.KindBar:
			want = c.Patterns[r.Key]
		case renderstate.KindGlyph:
			want = c.Glyphs[r.Key][r.Index]
		case renderstate.KindPortrait:
			want = c.Portraits[r.Key][r.Index]
		}
		if p.Slots[logical] != want {
			t.Fatal("ref identity changed", logical, r)
		}
	}
	p.Slots[0] = Tile{9}
	p.Cells[0] = 123
	if s.Slots[0] != before.Slots[0] || s.Cells[0] != 0 {
		t.Fatal("temporary pixels alias canonical refs")
	}
}
func TestHUDRefMaterializationRejectsMissingOrAliasedDomains(t *testing.T) {
	c := loadHUD(t)
	for _, ref := range []renderstate.TileRef{{Kind: 4}, {Kind: renderstate.KindBase, Key: 16}, {Kind: renderstate.KindBase, Index: 1}, {Kind: renderstate.KindBar, Key: 32}, {Kind: renderstate.KindBar, Index: 1}, {Kind: renderstate.KindGlyph, Key: 65535}, {Kind: renderstate.KindGlyph, Key: 0, Index: 4}, {Kind: renderstate.KindPortrait, Key: 1}, {Kind: renderstate.KindPortrait, Key: 2, Index: 16}} {
		s := refHUDState(c)
		s.Slots[0] = ref
		out, e := MaterializeHUDRefs(s, c)
		if e == nil || out.Slots != nil {
			t.Fatal("invalid materialization emitted partial pixels", ref, e)
		}
	}
	for _, kind := range []string{"nil", "cursor", "slot", "cell"} {
		s := refHUDState(c)
		switch kind {
		case "nil":
			s.Slots = nil
		case "cursor":
			s.NextNameTile = 31
		case "slot":
			s.Slots[2048] = renderstate.TileRef{}
		case "cell":
			s.Cells[0] = 2047
		}
		if _, e := MaterializeHUDRefs(s, c); e == nil {
			t.Fatal("unowned HUD ref accepted", kind)
		}
	}
}
