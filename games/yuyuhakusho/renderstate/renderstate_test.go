package renderstate_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/presentation"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
)

func raw(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(name)
	if e != nil {
		t.Fatal(e)
	}
	if len(b) > 2 && b[0] == 31 && b[1] == 139 {
		z, e := gzip.NewReader(bytes.NewReader(b))
		if e != nil {
			t.Fatal(e)
		}
		b, e = io.ReadAll(z)
		if e != nil {
			t.Fatal(e)
		}
		if e = z.Close(); e != nil {
			t.Fatal(e)
		}
	}
	return b
}
func jsonBytes(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func metadata(t *testing.T) []byte {
	t.Helper()
	var s, h, r map[string]json.RawMessage
	if e := json.Unmarshal(raw(t, "../data/world-sprites.json.gz"), &s); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(raw(t, "../data/hud-assets.json.gz"), &h); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(raw(t, "../data/chu-world-rules.json.gz"), &r); e != nil {
		t.Fatal(e)
	}
	var frames []map[string]json.RawMessage
	if e := json.Unmarshal(s["frames"], &frames); e != nil {
		t.Fatal(e)
	}
	for _, f := range frames {
		var pieces []json.RawMessage
		_ = json.Unmarshal(f["pieces"], &pieces)
		f["piece_count"] = json.RawMessage(fmt.Sprint(len(pieces)))
		delete(f, "pieces")
	}
	var tileBytes string
	_ = json.Unmarshal(s["tile_bytes"], &tileBytes)
	var glyphs []struct{ Code uint16 }
	var portraits []struct{ Parameter uint16 }
	_ = json.Unmarshal(h["glyphs"], &glyphs)
	_ = json.Unmarshal(h["portraits"], &portraits)
	var gc, pc []uint16
	for _, g := range glyphs {
		gc = append(gc, g.Code)
	}
	for _, p := range portraits {
		pc = append(pc, p.Parameter)
	}
	var start struct {
		Resources [8]uint32 `json:"arena_resources"`
	}
	_ = json.Unmarshal(r["match_start"], &start)
	value := map[string]any{"schema": 1, "rules_sha256": sha(raw(t, "../data/chu-world-rules.json.gz")), "sprites_sha256": sha(raw(t, "../data/world-sprites.json.gz")), "hud_sha256": sha(raw(t, "../data/hud-assets.json.gz")), "tile_count": len(tileBytes) / 64, "frames": frames, "default_static_slots": s["default_static_slots"], "arena_resources": start.Resources, "hud": map[string]any{"resource_list": h["resource_list"], "base_count": 16, "bar_count": 32, "glyph_codes": gc, "portrait_parameters": pc, "window_stripe": h["window_stripe"]}}
	return append(jsonBytes(value), '\n')
}

func libraries(t *testing.T) (renderstate.Library, presentation.Catalog, presentation.HUDCatalog) {
	t.Helper()
	l, e := renderstate.DecodeLibrary(metadata(t))
	if e != nil {
		t.Fatal(e)
	}
	s, e := presentation.DecodeCatalog(raw(t, "../data/world-sprites.json.gz"))
	if e != nil {
		t.Fatal(e)
	}
	h, e := presentation.DecodeHUDCatalog(raw(t, "../data/hud-assets.json.gz"))
	if e != nil {
		t.Fatal(e)
	}
	return l, s, h
}

func pixelHUD(t *testing.T, state renderstate.HUDState, catalog presentation.HUDCatalog) presentation.HUDState {
	t.Helper()
	out := presentation.HUDState{Cells: state.Cells, Rules: state.Rules, NextNameTile: state.NextNameTile, Slots: make(map[uint16]presentation.Tile)}
	for slot, ref := range state.Slots {
		switch ref.Kind {
		case renderstate.KindBase:
			out.Slots[slot] = catalog.Base[ref.Key]
		case renderstate.KindBar:
			out.Slots[slot] = catalog.Patterns[ref.Key]
		case renderstate.KindGlyph:
			out.Slots[slot] = catalog.Glyphs[ref.Key][ref.Index]
		case renderstate.KindPortrait:
			out.Slots[slot] = catalog.Portraits[ref.Key][ref.Index]
		default:
			t.Fatal("unknown HUD ref kind")
		}
	}
	return out
}

func TestRenderMetadataSourceAndValidation(t *testing.T) {
	l, _, _ := libraries(t)
	id := l.Identity()
	if id.RulesSHA256 != sha(raw(t, "../data/chu-world-rules.json.gz")) || id.SpriteCatalogSHA256 != sha(raw(t, "../data/world-sprites.json.gz")) || id.HUDCatalogSHA256 != sha(raw(t, "../data/hud-assets.json.gz")) || id.MetadataSHA256 != sha(metadata(t)) {
		t.Fatal("metadata identity lost typed sources")
	}
	var generated, independent any
	if e := json.Unmarshal(raw(t, "../data/renderstate-metadata.json.gz"), &generated); e != nil {
		t.Fatal(e)
	}
	_ = json.Unmarshal(metadata(t), &independent)
	if !reflect.DeepEqual(generated, independent) {
		t.Fatal("export differs from independently selected source metadata")
	}
	for _, kind := range []string{"schema", "pixel-payload", "missing-frames", "duplicate-frame", "piece-zero", "piece-large", "odd-parameter", "bad-tile", "short-transfer", "high-stamp", "duplicate-preload", "bad-preload", "missing-glyph", "duplicate-glyph", "bad-portrait", "bad-resource", "bad-sha"} {
		t.Run(kind, func(t *testing.T) {
			var m map[string]any
			_ = json.Unmarshal(metadata(t), &m)
			frames := m["frames"].([]any)
			f := frames[0].(map[string]any)
			switch kind {
			case "schema":
				m["schema"] = 2
			case "pixel-payload":
				m["tile_bytes"] = "0000"
			case "missing-frames":
				m["frames"] = frames[:1499]
			case "duplicate-frame":
				frames[1] = frames[0]
			case "piece-zero":
				f["piece_count"] = 0
			case "piece-large":
				f["piece_count"] = 81
			case "odd-parameter":
				f["key"].(map[string]any)["parameter"] = 1
			case "bad-tile":
				f["transfers"].([]any)[0].(map[string]any)["tiles"].([]any)[0] = float64(1 << 30)
			case "short-transfer":
				f["transfers"].([]any)[0].(map[string]any)["tiles"] = []any{}
			case "high-stamp":
				f["transfers"].([]any)[0].(map[string]any)["stamp"] = float64(1 << 31)
			case "duplicate-preload":
				a := m["default_static_slots"].([]any)
				a[1] = a[0]
			case "bad-preload":
				m["default_static_slots"].([]any)[0].(map[string]any)["slot"] = 2048
			case "missing-glyph":
				h := m["hud"].(map[string]any)
				h["glyph_codes"] = h["glyph_codes"].([]any)[1:]
			case "duplicate-glyph":
				a := m["hud"].(map[string]any)["glyph_codes"].([]any)
				a[1] = a[0]
			case "bad-portrait":
				m["hud"].(map[string]any)["portrait_parameters"].([]any)[0] = 1
			case "bad-resource":
				m["arena_resources"].([]any)[1] = 1
			case "bad-sha":
				m["sprites_sha256"] = "wrong"
			}
			got, e := renderstate.DecodeLibrary(jsonBytes(m))
			if e == nil || !reflect.DeepEqual(got, renderstate.Library{}) {
				t.Fatal("invalid metadata accepted or partial library returned", e)
			}
		})
	}
}

func worldActor(a presentation.Actor) engine.WorldObject {
	o := engine.WorldObject{ObjectID: a.ObjectID, Header: engine.MatchObjectHeader{Behavior: 0x12668, Size: 64, Stage: 2}, GraphicsSet: a.GraphicsSet, TilesBase: a.TileBank, GraphicParameter: a.Parameter, ScreenX: int16(a.ScreenX), ScreenY: int16(a.ScreenY), GraphicOffsetX: a.OffsetX, GraphicOffsetY: a.OffsetY, SkipGraphics: a.SkipTiles, GraphicCache: a.Cache}
	o.Body.Attributes, o.Body.Depth, o.Body.Layer = a.Attributes, a.Depth, a.Layer
	return o
}

func compareDrawSequence(t *testing.T, l renderstate.Library, c presentation.Catalog, actors []presentation.Actor, mutate bool) {
	t.Helper()
	state, e := renderstate.New(l)
	if e != nil {
		t.Fatal(e)
	}
	// Bounded dirty slot assignments are a valid inherited history for isolation.
	for i := uint16(0); i < 2048; i++ {
		if e = state.SpriteSlots.Set(i, uint32(i)%uint32(len(c.Tiles))); e != nil {
			t.Fatal(e)
		}
	}
	pframe := presentation.BeginFrame(presentation.TileSlots(state.SpriteSlots), 2)
	world := engine.ChuWorldState{Objects: make(engine.WorldObjects)}
	var events []engine.ChuWorldEvent
	for _, a := range actors {
		o := worldActor(a)
		world.Objects[o.ObjectID] = o
		d := o.DrawRequest()
		events = append(events, engine.ChuWorldEvent{Kind: "draw", ObjectID: o.ObjectID, Draw: &d})
	}
	if mutate {
		a := world.Objects[actors[0].ObjectID]
		a.GraphicParameter += 2
		a.GraphicCache = [4]uint32{0x123, 0x456, 0x789, 0xabc}
		world.Objects[a.ObjectID] = a
	}
	wantWorld := world
	wantWorld.Objects = make(engine.WorldObjects, len(world.Objects))
	for id, o := range world.Objects {
		wantWorld.Objects[id] = o
	}
	var wantErr error
	for _, event := range events {
		var updated presentation.Actor
		pframe, updated, wantErr = presentation.ComposeActor(pframe, presentation.Actor(*event.Draw), c)
		if wantErr != nil {
			break
		}
		if o := wantWorld.Objects[event.ObjectID]; o.DrawRequest() == *event.Draw {
			o.GraphicCache = updated.Cache
			wantWorld.Objects[o.ObjectID] = o
		}
	}
	beforeWorld, beforeState := jsonBytes(world), jsonBytes(state)
	got, next, e := renderstate.Advance(world, state, events, l)
	if wantErr != nil {
		if e == nil || !reflect.DeepEqual(got, world) || !reflect.DeepEqual(next, state) {
			t.Fatal("missing tile source did not fail atomically", e)
		}
	} else {
		if e != nil || !reflect.DeepEqual(got, wantWorld) || presentation.TileSlots(next.SpriteSlots) != pframe.Slots {
			t.Fatal("metadata differs from existing ordered composer", e, actors[0].GraphicsSet, actors[0].Parameter)
		}
	}
	if !bytes.Equal(beforeWorld, jsonBytes(world)) || !bytes.Equal(beforeState, jsonBytes(state)) {
		t.Fatal("metadata mutated caller-owned maps or arrays")
	}
}

func TestRenderAll1500FramesAndCacheBoundaries(t *testing.T) {
	l, c, _ := libraries(t)
	if len(c.Frames) != 1500 {
		t.Fatal("incomplete actual frame catalog")
	}
	cases := 0
	for _, f := range c.Frames {
		for _, kind := range []string{"cold", "warm", "skip", "culled-left", "culled-right", "culled-y", "wrapped-slot", "reused-slot"} {
			a := presentation.Actor{ObjectID: 0x8400, GraphicsSet: f.Key.GraphicsSet, TileBank: f.TileBank, Parameter: f.Key.Parameter, ScreenX: 160, ScreenY: 180, Depth: 256, Attributes: 0x6400}
			switch kind {
			case "warm":
				for i, tr := range f.Transfers {
					a.Cache[i] = tr.Stamp
				}
			case "skip":
				a.SkipTiles = 1
				a.Cache = [4]uint32{1, 2, 3, 4}
			case "culled-left":
				a.ScreenX = 64
			case "culled-right":
				a.ScreenX = 512
			case "culled-y":
				a.ScreenY = 64
			case "wrapped-slot":
				a.Attributes = 0x67f8
			}
			compareDrawSequence(t, l, c, []presentation.Actor{a}, kind == "reused-slot")
			cases++
		}
	}
	t.Logf("all actual frames=1500 cache/cull/skip/wrap/reuse comparisons=%d", cases)
}

func TestRenderBudgetAndLateFailure(t *testing.T) {
	l, c, _ := libraries(t)
	for _, f := range c.Frames {
		if f.PreloadedOnly {
			continue
		}
		var actors []presentation.Actor
		for i := 0; i < 110; i++ {
			actors = append(actors, presentation.Actor{ObjectID: 0x8000 + uint16(i)*64, GraphicsSet: f.Key.GraphicsSet, TileBank: f.TileBank, Parameter: f.Key.Parameter, ScreenX: 160, ScreenY: 180, Depth: 256, Attributes: uint16(i*16) & 2047})
		}
		compareDrawSequence(t, l, c, actors, false)
		if len(f.Pieces) > 3 {
			break
		}
	}
	s, _ := renderstate.New(l)
	f := c.Frames[0]
	o := worldActor(presentation.Actor{ObjectID: 0x8400, GraphicsSet: f.Key.GraphicsSet, TileBank: f.TileBank, Parameter: f.Key.Parameter, ScreenX: 160, ScreenY: 180, Depth: 256})
	w := engine.ChuWorldState{Objects: engine.WorldObjects{o.ObjectID: o}}
	d := o.DrawRequest()
	events := []engine.ChuWorldEvent{{Kind: "draw", ObjectID: o.ObjectID, Draw: &d}, {Kind: "hud", HUD: engine.FighterBarTransfer{Pattern: 32, TargetTile: 16}}}
	g, n, e := renderstate.Advance(w, s, events, l)
	if e == nil || !reflect.DeepEqual(g, w) || !reflect.DeepEqual(n, s) {
		t.Fatal("late HUD failure leaked earlier sprite/cache work")
	}
}

func TestRenderHUDOrderedRefsAndPixelAliases(t *testing.T) {
	l, _, h := libraries(t)
	s, _ := renderstate.New(l)
	old, e := presentation.NewHUD(h)
	if e != nil {
		t.Fatal(e)
	}
	w := engine.ChuWorldState{Objects: make(engine.WorldObjects)}
	var aliases = map[presentation.Tile]int{}
	for _, tile := range h.Base {
		aliases[tile]++
	}
	for _, tile := range h.Patterns {
		aliases[tile]++
	}
	for _, tiles := range h.Glyphs {
		for _, tile := range tiles {
			aliases[tile]++
		}
	}
	for _, tiles := range h.Portraits {
		for _, tile := range tiles {
			aliases[tile]++
		}
	}
	duplicateGroups := 0
	for _, n := range aliases {
		if n > 1 {
			duplicateGroups++
		}
	}
	if duplicateGroups != 4 {
		t.Fatal("source pixel aliases changed", duplicateGroups)
	}
	for code := range h.Glyphs {
		events := []engine.ChuWorldEvent{{Kind: "start", Start: engine.MatchStartEvent{Kind: "reset"}}, {Kind: "start", Start: engine.MatchStartEvent{Kind: "resource", Source: 0x21720}}, {Kind: "start", Start: engine.MatchStartEvent{Kind: "words", Offset: 256, Words: []uint16{0xe010, 0xe011, 0xe012}}}, {Kind: "hud", HUD: engine.FighterBarTransfer{Pattern: uint8(code % 32), TargetTile: 16}}, {Kind: "start", Start: engine.MatchStartEvent{Kind: "name", Glyphs: [3]uint16{code, 0, code}}}, {Kind: "start", Start: engine.MatchStartEvent{Kind: "portrait", Parameter: 2, Attributes: 0xe300, X: 144, Y: 144}}}
		old, e = presentation.ApplyHUDWorldEvents(old, events, w.Cells[:256], h)
		if e != nil {
			t.Fatal(e)
		}
		w, s, e = renderstate.Advance(w, s, events, l)
		if e != nil || !reflect.DeepEqual(pixelHUD(t, s.HUD, h), old) {
			t.Fatal("ordered HUD semantic refs differ from existing pixels", code, e)
		}
		for i := uint16(0); i < 4; i++ {
			if s.HUD.Slots[32+i] != (renderstate.TileRef{Kind: renderstate.KindGlyph, Key: code, Index: uint8(i)}) {
				t.Fatal("glyph provenance guessed from duplicate pixels", code, i, s.HUD.Slots[32+i])
			}
		}
	}
	for parameter := uint16(2); parameter <= 22; parameter += 2 {
		events := []engine.ChuWorldEvent{{Kind: "start", Start: engine.MatchStartEvent{Kind: "portrait", Parameter: parameter, Attributes: 0xe300, X: 144, Y: 144}}}
		old, e = presentation.ApplyHUDWorldEvents(old, events, w.Cells[:256], h)
		w, s, e = renderstate.Advance(w, s, events, l)
		if e != nil || !reflect.DeepEqual(pixelHUD(t, s.HUD, h), old) {
			t.Fatal("portrait refs differ", parameter, e)
		}
	}
	for pattern := uint8(0); pattern < 32; pattern++ {
		events := []engine.ChuWorldEvent{{Kind: "hud", HUD: engine.FighterBarTransfer{Pattern: pattern, TargetTile: 23}}}
		old, e = presentation.ApplyHUDWorldEvents(old, events, w.Cells[:256], h)
		w, s, e = renderstate.Advance(w, s, events, l)
		if e != nil || !reflect.DeepEqual(pixelHUD(t, s.HUD, h), old) {
			t.Fatal("bar refs differ", pattern, e)
		}
	}
	if e = renderstate.Validate(w, s, l); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"slot", "ref", "cursor", "missing-slot"} {
		var bad renderstate.State
		_ = json.Unmarshal(jsonBytes(s), &bad)
		switch kind {
		case "slot":
			bad.SpriteSlots.Tiles[0] = 1 << 30
		case "ref":
			bad.HUD.Slots[0] = renderstate.TileRef{Kind: renderstate.KindGlyph, Key: 0xffff}
		case "cursor":
			bad.HUD.NextNameTile = 31
		case "missing-slot":
			delete(bad.HUD.Slots, 0)
		}
		if renderstate.Validate(w, bad, l) == nil {
			t.Fatal("invalid bounded presentation checkpoint accepted", kind)
		}
	}
}

func TestRenderDigestsAndNonzeroPanState(t *testing.T) {
	l, _, _ := libraries(t)
	s, _ := renderstate.New(l)
	w := engine.ChuWorldState{Objects: engine.WorldObjects{0x8400: {ObjectID: 0x8400}}, Stream: make([]byte, 1024)}
	d, e := renderstate.RulesDigest(w)
	if e != nil {
		t.Fatal(e)
	}
	p, _ := renderstate.Digest(w, s)
	o := w.Objects[0x8400]
	o.GraphicCache[0] = 123
	w.Objects[o.ObjectID] = o
	other, _ := renderstate.RulesDigest(w)
	pd, _ := renderstate.Digest(w, s)
	if d != other || p == pd {
		t.Fatal("rule/presentation cache ownership digest differs")
	}
	w.Stream[0] = 1
	other, _ = renderstate.RulesDigest(w)
	if d == other {
		t.Fatal("rules digest omitted canonical Stream")
	}
	if _, e = renderstate.New(renderstate.Library{}); e == nil {
		t.Fatal("empty metadata library accepted")
	}
}

func TestRenderArenaARawBudgetOrdering(t *testing.T) {
	l, c, _ := libraries(t)
	s, _ := renderstate.New(l)
	world := engine.ChuWorldState{Match: engine.ChuWorldMatch{Arena: 10}, Objects: make(engine.WorldObjects)}
	var one, many presentation.Frame
	for _, f := range c.Frames {
		if !f.PreloadedOnly && len(f.Pieces) == 1 {
			one = f
		}
		if !f.PreloadedOnly && len(f.Pieces) >= 4 {
			many = f
		}
	}
	frame := presentation.BeginFrame(presentation.TileSlots(s.SpriteSlots), 2)
	var events []engine.ChuWorldEvent
	addDraw := func(f presentation.Frame, id uint16) {
		o := worldActor(presentation.Actor{ObjectID: id, GraphicsSet: f.Key.GraphicsSet, TileBank: f.TileBank, Parameter: f.Key.Parameter, ScreenX: 160, ScreenY: 180, Depth: 256, Attributes: id & 2047})
		world.Objects[id] = o
		d := o.DrawRequest()
		events = append(events, engine.ChuWorldEvent{Kind: "draw", ObjectID: id, Draw: &d})
	}
	for i := 0; i < 126; i++ {
		addDraw(one, uint16(0x8000+i*64))
	}
	for i := 0; i < 6; i++ {
		r := engine.ArenaASprite{Y: 144, SizeLink: 0xffc, Tile: 0x5d0, X: 224}
		events = append(events, engine.ChuWorldEvent{Kind: "arena-sprite", ObjectID: 0x9e00, RawArenaSprite: &r})
	}
	addDraw(many, 0x9f80)
	addDraw(many, 0x9fc0)
	want := world
	want.Objects = make(engine.WorldObjects)
	for id, o := range world.Objects {
		want.Objects[id] = o
	}
	for _, e := range events {
		if e.RawArenaSprite != nil {
			var err error
			frame, err = presentation.ComposeArenaASprite(frame, *e.RawArenaSprite)
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		f, a, err := presentation.ComposeActor(frame, presentation.Actor(*e.Draw), c)
		if err != nil {
			t.Fatal(err)
		}
		frame = f
		if o := want.Objects[e.ObjectID]; o.DrawRequest() == *e.Draw {
			o.GraphicCache = a.Cache
			want.Objects[o.ObjectID] = o
		}
	}
	got, next, err := renderstate.Advance(world, s, events, l)
	if err != nil || !reflect.DeepEqual(got, want) || presentation.TileSlots(next.SpriteSlots) != frame.Slots {
		t.Fatal("raw arena signed budget or subsequent Flags order differs", err)
	}
	world.Match.Arena = 0
	g, n, err := renderstate.Advance(world, s, events, l)
	if err == nil || !reflect.DeepEqual(g, world) || !reflect.DeepEqual(n, s) {
		t.Fatal("unselected arena raw sprites accepted or partially committed")
	}
}
