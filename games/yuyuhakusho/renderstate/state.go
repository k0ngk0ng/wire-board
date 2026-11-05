package renderstate

import (
	"encoding/json"
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type SpriteSlots struct {
	Tiles   [2048]uint32
	Present [32]uint64
}

func (s *SpriteSlots) Set(slot uint16, tile uint32) error {
	if slot >= 2048 {
		return errors.New("sprite slot exceeds bounded bank")
	}
	s.Tiles[slot] = tile
	s.Present[slot/64] |= uint64(1) << (slot & 63)
	return nil
}
func (s SpriteSlots) Lookup(slot uint16) (uint32, bool) {
	if slot >= 2048 || s.Present[slot/64]&(uint64(1)<<(slot&63)) == 0 {
		return 0, false
	}
	return s.Tiles[slot], true
}

type TileKind uint8

const (
	KindBase TileKind = iota
	KindBar
	KindGlyph
	KindPortrait
)

type TileRef struct {
	Kind  TileKind `json:"kind"`
	Key   uint16   `json:"key"`
	Index uint8    `json:"index"`
}
type HUDState struct {
	Cells        [256]uint16
	Rules        [256]bool
	Slots        map[uint16]TileRef
	NextNameTile uint16
}
type State struct {
	SpriteSlots SpriteSlots `json:"sprite_slots"`
	HUD         HUDState    `json:"hud"`
}

func newHUD() HUDState {
	h := HUDState{Slots: make(map[uint16]TileRef), NextNameTile: 32}
	for i := uint16(0); i < 16; i++ {
		h.Slots[i] = TileRef{Kind: KindBase, Key: i}
	}
	return h
}
func New(l Library) (State, error) {
	if len(l.frames) != 1500 || l.tileCount == 0 {
		return State{}, errors.New("render metadata is unavailable")
	}
	s := State{HUD: newHUD()}
	for _, b := range l.defaults {
		_ = s.SpriteSlots.Set(b.Slot, b.Tile)
	}
	return s, nil
}

func validRef(r TileRef, l Library) bool {
	switch r.Kind {
	case KindBase:
		return r.Key < 16 && r.Index == 0
	case KindBar:
		return r.Key < 32 && r.Index == 0
	case KindGlyph:
		return l.glyphs[r.Key] && r.Index < 4
	case KindPortrait:
		return l.portraits[r.Key] && r.Index < 16
	}
	return false
}

// Validate checks a bounded current checkpoint, never reconstructs history.
func Validate(w engine.ChuWorldState, s State, l Library) error {
	if len(l.frames) != 1500 || s.HUD.Slots == nil || len(s.HUD.Slots) > 2048 || s.HUD.NextNameTile < 32 || s.HUD.NextNameTile > 2048 {
		return errors.New("render checkpoint is unavailable or unbounded")
	}
	if w.Match.Arena > 14 || w.Match.Arena&1 != 0 {
		return errors.New("unavailable checkpoint arena")
	}
	for id := range w.Objects {
		if _, e := engine.LookupWorldObject(w.Objects, id); e != nil {
			return e
		}
	}
	for i, tile := range s.SpriteSlots.Tiles {
		present := s.SpriteSlots.Present[i/64]&(uint64(1)<<uint(i&63)) != 0
		if present && tile >= l.tileCount || !present && tile != 0 {
			return errors.New("checkpoint contains an unavailable sprite tile")
		}
	}
	for slot, ref := range s.HUD.Slots {
		if slot >= 2048 || !validRef(ref, l) {
			return errors.New("checkpoint contains an unavailable HUD ref")
		}
	}
	for _, word := range s.HUD.Cells {
		if _, ok := s.HUD.Slots[word&2047]; !ok {
			return errors.New("HUD cell references an unavailable tile")
		}
	}
	return nil
}

// Advance applies ordered callback-time metadata. Cache lives only in World;
// slots/refs live only in State. Count and Flags are transient per-frame views.
func Advance(world engine.ChuWorldState, state State, events []engine.ChuWorldEvent, l Library) (engine.ChuWorldState, State, error) {
	originalWorld, originalState := world, state
	if e := Validate(world, state, l); e != nil {
		return originalWorld, originalState, e
	}
	world.Objects = make(engine.WorldObjects, len(originalWorld.Objects))
	for id, o := range originalWorld.Objects {
		world.Objects[id] = o
	}
	state.HUD.Slots = make(map[uint16]TileRef, len(originalState.HUD.Slots))
	for k, v := range originalState.HUD.Slots {
		state.HUD.Slots[k] = v
	}
	count, flags := uint16(2), uint8(0x80)
	for _, event := range events {
		if event.Kind == "hud" || event.Kind == "start" {
			if e := applyHUD(&state.HUD, event, l.resources[world.Match.Arena/2], l); e != nil {
				return originalWorld, originalState, e
			}
			continue
		}
		if event.Kind == "arena-sprite" {
			r := event.RawArenaSprite
			if world.Match.Arena != 10 || event.ObjectID != 0x9e00 || r == nil || r.SizeLink != 0xffc || (r.Tile != 0x5d0 && r.Tile != 0x5e0 && r.Tile != 0x5f0) {
				return originalWorld, originalState, errors.New("unavailable selected raw arena sprite")
			}
			if int8(uint8(count)) < 80 {
				count = count&0xff00 | uint16(uint8(count+1))
			}
			continue
		}
		if event.Kind != "draw" {
			continue
		}
		if event.Draw == nil || event.Draw.ObjectID != event.ObjectID {
			return originalWorld, originalState, errors.New("draw lacks callback-time identity")
		}
		a := *event.Draw
		x, y := a.ScreenX+uint16(a.OffsetX), a.ScreenY+uint16(a.OffsetY)
		if flags&0x80 == 0 || x-64 == 0 || x-64 >= 448 || y-64 == 0 || y-64 >= 352 {
			continue
		}
		f, ok := l.frames[frameKey{a.GraphicsSet, a.Parameter}]
		if !ok {
			return originalWorld, originalState, errors.New("draw frame metadata is unavailable")
		}
		cache := a.Cache
		if a.SkipTiles == 0 {
			for i, tr := range f.Transfers {
				if cache[i] == tr.Stamp {
					continue
				}
				if uint8(tr.Stamp) > 0 && (f.PreloadedOnly || a.TileBank != f.TileBank || len(tr.Tiles) != int(uint8(tr.Stamp))) {
					return originalWorld, originalState, errors.New("draw transfer source is unavailable")
				}
				cache[i] = tr.Stamp
				destination := (uint16(uint8(tr.Stamp>>8)) + a.Attributes&2047) & 2047
				for j, tile := range tr.Tiles {
					_ = state.SpriteSlots.Set((destination+uint16(j))&2047, tile)
				}
			}
			for i := len(f.Transfers); i < 4; i++ {
				cache[i] = 0
			}
		}
		if o, exists := world.Objects[event.ObjectID]; exists && o.DrawRequest() == a {
			o.GraphicCache = cache
			world.Objects[event.ObjectID] = o
		}
		for i := uint8(0); i < f.PieceCount; i++ {
			count = count&0xff00 | uint16(uint8(count+1))
			if i == f.PieceCount-1 {
				break
			}
			if count >= 80 {
				flags = 1
				break
			}
		}
	}
	for i, owned := range state.HUD.Rules {
		if owned {
			state.HUD.Cells[i] = world.Cells[i]
		}
	}
	if e := Validate(world, state, l); e != nil {
		return originalWorld, originalState, e
	}
	return world, state, nil
}

func applyHUD(h *HUDState, event engine.ChuWorldEvent, resource uint32, l Library) error {
	if event.Kind == "hud" {
		if event.HUD.Pattern >= 32 || event.HUD.TargetTile < 16 || event.HUD.TargetTile > 23 {
			return errors.New("unsupported HUD bar ref")
		}
		h.Slots[event.HUD.TargetTile] = TileRef{Kind: KindBar, Key: uint16(event.HUD.Pattern)}
		return nil
	}
	r := event.Start
	switch r.Kind {
	case "reset":
		*h = newHUD()
	case "resource":
		if r.Source != resource {
			return errors.New("HUD ref resource differs from selected arena")
		}
	case "words":
		start := int(r.Offset) / 2
		if r.Offset&1 != 0 || len(r.Words) == 0 || start+len(r.Words) > 256 {
			return errors.New("HUD words exceed owned cells")
		}
		for i, v := range r.Words {
			h.Cells[start+i], h.Rules[start+i] = v, true
		}
	case "name":
		start := int(r.Offset)/2 + 131
		if r.Offset&1 != 0 || start+69 >= 256 || int(h.NextNameTile)+12 > 2048 {
			return errors.New("HUD name position or allocation unavailable")
		}
		for gi, code := range r.Glyphs {
			if !l.glyphs[code] {
				return errors.New("HUD glyph ref unavailable")
			}
			for i := uint16(0); i < 4; i++ {
				h.Slots[h.NextNameTile+i] = TileRef{Kind: KindGlyph, Key: code, Index: uint8(i)}
			}
			for col := 0; col < 2; col++ {
				for row := 0; row < 2; row++ {
					index := start + gi*2 + col + row*64
					h.Cells[index], h.Rules[index] = 0xe000|h.NextNameTile+uint16(col*2+row), false
				}
			}
			h.NextNameTile += 4
		}
	case "portrait":
		if !l.portraits[r.Parameter] || r.Attributes & ^uint16(0x6000|0x30) != 0x8300 {
			return errors.New("HUD portrait ref or attributes unavailable")
		}
		x, y := (int(r.X)-144)&^7, (int(r.Y)-144)&^7
		col, row := x/8, y/8
		if x < 0 || y < 0 || col+4 > 64 || row+4 > 4 {
			return errors.New("HUD portrait exceeds cells")
		}
		base := r.Attributes & 2047
		for i := uint16(0); i < 16; i++ {
			h.Slots[base+i] = TileRef{Kind: KindPortrait, Key: r.Parameter, Index: uint8(i)}
		}
		for dx := 0; dx < 4; dx++ {
			for dy := 0; dy < 4; dy++ {
				index := (row+dy)*64 + col + dx
				h.Cells[index], h.Rules[index] = r.Attributes+uint16(dx*4+dy), false
			}
		}
	default:
		return errors.New("unsupported HUD metadata event")
	}
	return nil
}

// RulesDigest excludes only the established renderer-owned object caches.
func RulesDigest(w engine.ChuWorldState) (string, error) {
	if w.Objects != nil {
		objects := make(engine.WorldObjects, len(w.Objects))
		for id, o := range w.Objects {
			o.GraphicCache = [4]uint32{}
			objects[id] = o
		}
		w.Objects = objects
	}
	b, e := json.Marshal(w)
	if e != nil {
		return "", e
	}
	return digest(b), nil
}

// Digest covers the complementary persistent presentation owners.
func Digest(w engine.ChuWorldState, s State) (string, error) {
	caches := make(map[uint16][4]uint32, len(w.Objects))
	for id, o := range w.Objects {
		caches[id] = o.GraphicCache
	}
	b, e := json.Marshal(struct {
		Caches map[uint16][4]uint32
		State  State
	}{caches, s})
	if e != nil {
		return "", e
	}
	return digest(b), nil
}
