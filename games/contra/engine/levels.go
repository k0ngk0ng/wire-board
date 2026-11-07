package contra

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

type MapVariant uint8

const (
	RegularMap MapVariant = iota
	BaseBossMap
)

type PaletteState struct {
	CollisionLimits [3]uint8
	CycleIndices    [4]uint8
	PaletteIndices  [8]uint8
}

type AlternateAppearance struct {
	TriggerScreen uint8
	Mode          string
	HasCHR        bool
	CHRPreset     int
	Palette       PaletteState
}

type BaseBossDefinition struct {
	Available                                  bool
	SceneType                                  uint8
	MapIndex, CHRPreset                        int
	MapSetID, TilesetID                        string
	TriggerScreen, TriggerPhase, EnterAtScreen uint8
}

type StageDefinition struct {
	SceneType, ScrollType, CollisionCheck, StopScrollScreen uint8
	Rows, MapEntries, CHRPreset, ScrollSpanPixels           int
	Initial                                                 PaletteState
	Alternate                                               AlternateAppearance
	Boss                                                    BaseBossDefinition
}

// LevelResources contains only validated map/art definitions. A server may
// load the complete catalog; a client can install the current-stage payload.
// Unloaded stages remain unavailable rather than silently borrowing stage one.
type LevelResources struct {
	stages  map[int]resourceStage
	tiles   map[string]resourceTiles
	extra   map[string]resourceMaps
	presets map[int][8]uint8
	colors  map[int][3]uint8
}

type resourceMap struct {
	Index   int
	Encoded string
	AliasOf *int
	decoded []uint8
}
type resourceMaps struct {
	Rows    int
	Entries []resourceMap
}
type resourceTiles struct {
	ID                      string
	Supertiles              []GraphicsSupertile
	UpdateBase, UpdateCount int
	SourceAttributePage     uint8
}
type resourceStage struct {
	Stage, CHRPreset, ScrollSpanPixels                             int
	SceneType, ScrollType, InitialCollisionCheck, StopScrollScreen uint8
	TileSetID                                                      string
	InitialPaletteState                                            PaletteState
	Maps                                                           resourceMaps
	Alternate                                                      struct {
		TriggerScreen   uint8
		Mode            string
		CHRPreset       *int
		CollisionLimits [3]uint8
		CycleIndices    [4]uint8
		PaletteIndices  [8]uint8
	}
	BaseBoss *struct {
		SceneType           uint8
		MapIndex, CHRPreset int
		MapSetID, TileSetID string
		Trigger             struct{ Screen, PerspectivePhase uint8 }
		EnterAtScreen       uint8
	}
}

func DecodeLevelResources(raw []byte) (*LevelResources, error) {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return nil, errors.New("invalid level resource size")
	}
	var data struct {
		Schema       int
		RomSHA256    string
		Stages       []resourceStage
		TileSets     []resourceTiles
		ExtraMapSets map[string]resourceMaps
		CHRPresets   []struct {
			ID    int
			Pages [8]uint8
		}
		PaletteColors []struct {
			ID     int
			Colors [3]uint8
		}
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data.Schema != 1 || data.RomSHA256 != "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe" || len(data.Stages) == 0 || len(data.Stages) > 8 {
		return nil, errors.New("level resource identity differs")
	}
	r := &LevelResources{stages: map[int]resourceStage{}, tiles: map[string]resourceTiles{}, extra: map[string]resourceMaps{}, presets: map[int][8]uint8{}, colors: map[int][3]uint8{}}
	for _, t := range data.TileSets {
		if t.ID == "" || len(t.Supertiles) == 0 || len(t.Supertiles) > 128 || t.UpdateBase < 0 || t.UpdateCount < 0 || t.UpdateBase+t.UpdateCount != len(t.Supertiles) {
			return nil, errors.New("invalid level tileset")
		}
		if _, ok := r.tiles[t.ID]; ok {
			return nil, errors.New("duplicate tileset")
		}
		r.tiles[t.ID] = t
	}
	for _, p := range data.CHRPresets {
		if p.ID < 0 || p.ID > 255 {
			return nil, errors.New("invalid CHR preset")
		}
		for _, page := range p.Pages {
			if page >= 128 {
				return nil, errors.New("CHR page outside source")
			}
		}
		if _, ok := r.presets[p.ID]; ok {
			return nil, errors.New("duplicate CHR preset")
		}
		r.presets[p.ID] = p.Pages
	}
	for _, p := range data.PaletteColors {
		if p.ID < 0 || p.ID > 255 {
			return nil, errors.New("invalid palette ID")
		}
		for _, color := range p.Colors {
			if color >= 64 {
				return nil, errors.New("invalid palette color")
			}
		}
		if _, ok := r.colors[p.ID]; ok {
			return nil, errors.New("duplicate palette ID")
		}
		r.colors[p.ID] = p.Colors
	}
	for _, stage := range data.Stages {
		level := stage.Stage - 1
		if level < 0 || level >= 8 || stage.SceneType > 1 || stage.ScrollType > 1 {
			return nil, errors.New("invalid level definition")
		}
		if _, ok := r.stages[level]; ok {
			return nil, errors.New("duplicate stage")
		}
		tileSet, ok := r.tiles[stage.TileSetID]
		if !ok {
			return nil, errors.New("missing stage tileset")
		}
		if err := decodeResourceMaps(&stage.Maps, tileSet); err != nil {
			return nil, err
		}
		if stage.ScrollType == 1 && stage.Maps.Rows != 8 || stage.ScrollType == 0 && stage.Maps.Rows != 7 {
			return nil, errors.New("map rows do not match stage")
		}
		if _, err := r.CHRPreset(stage.CHRPreset); err != nil {
			return nil, err
		}
		if _, err := r.Palette(stage.InitialPaletteState.PaletteIndices); err != nil {
			return nil, err
		}
		if _, err := r.Palette(stage.Alternate.PaletteIndices); err != nil {
			return nil, err
		}
		if stage.Alternate.CHRPreset != nil {
			if _, err := r.CHRPreset(*stage.Alternate.CHRPreset); err != nil {
				return nil, err
			}
		}
		if boss := stage.BaseBoss; boss != nil {
			set, ok := r.tiles[boss.TileSetID]
			if !ok {
				return nil, errors.New("missing Boss tileset")
			}
			maps, ok := data.ExtraMapSets[boss.MapSetID]
			if !ok {
				return nil, errors.New("missing Boss map")
			}
			if err := decodeResourceMaps(&maps, set); err != nil {
				return nil, err
			}
			if _, err := findResourceMap(maps, boss.MapIndex); err != nil {
				return nil, err
			}
			if boss.SceneType != 0x80 {
				return nil, errors.New("invalid Boss scene type")
			}
			if _, err := r.CHRPreset(boss.CHRPreset); err != nil {
				return nil, err
			}
			r.extra[boss.MapSetID] = maps
		}
		r.stages[level] = stage
	}
	return r, nil
}

func decodeResourceMaps(m *resourceMaps, t resourceTiles) error {
	if m.Rows != 7 && m.Rows != 8 || len(m.Entries) == 0 || len(m.Entries) > 128 {
		return errors.New("invalid map set")
	}
	seen := map[int]bool{}
	for i := range m.Entries {
		e := &m.Entries[i]
		if e.Index < 0 || e.Index >= 128 || seen[e.Index] {
			return errors.New("invalid map index")
		}
		seen[e.Index] = true
		encoded, err := hex.DecodeString(e.Encoded)
		if err != nil {
			return err
		}
		values, n, err := DecodeScreen(encoded, m.Rows)
		if err != nil {
			return err
		}
		if n != len(encoded) {
			return errors.New("trailing map data")
		}
		for _, id := range values {
			if int(id) >= len(t.Supertiles) {
				return errors.New("map references missing supertile")
			}
		}
		e.decoded = values
	}
	for _, e := range m.Entries {
		if e.AliasOf != nil {
			other, err := findResourceMap(*m, *e.AliasOf)
			if err != nil {
				return err
			}
			if other.Encoded != e.Encoded {
				return errors.New("map alias differs from target")
			}
		}
	}
	return nil
}
func findResourceMap(m resourceMaps, index int) (resourceMap, error) {
	for _, e := range m.Entries {
		if e.Index == index {
			return e, nil
		}
	}
	return resourceMap{}, errors.New("map is not loaded")
}
func (r *LevelResources) Stage(level int) (StageDefinition, error) {
	s, ok := r.stages[level]
	if !ok {
		return StageDefinition{}, errors.New("stage resources are not loaded")
	}
	a := s.Alternate
	info := StageDefinition{SceneType: s.SceneType, ScrollType: s.ScrollType, CollisionCheck: s.InitialCollisionCheck, StopScrollScreen: s.StopScrollScreen, Rows: s.Maps.Rows, MapEntries: len(s.Maps.Entries), CHRPreset: s.CHRPreset, ScrollSpanPixels: s.ScrollSpanPixels, Initial: s.InitialPaletteState,
		Alternate: AlternateAppearance{TriggerScreen: a.TriggerScreen, Mode: a.Mode, Palette: PaletteState{CollisionLimits: a.CollisionLimits, CycleIndices: a.CycleIndices, PaletteIndices: a.PaletteIndices}}}
	if a.CHRPreset != nil {
		info.Alternate.HasCHR = true
		info.Alternate.CHRPreset = *a.CHRPreset
	}
	if b := s.BaseBoss; b != nil {
		info.Boss = BaseBossDefinition{Available: true, SceneType: b.SceneType, MapIndex: b.MapIndex, CHRPreset: b.CHRPreset, MapSetID: b.MapSetID, TilesetID: b.TileSetID, TriggerScreen: b.Trigger.Screen, TriggerPhase: b.Trigger.PerspectivePhase, EnterAtScreen: b.EnterAtScreen}
	}
	return info, nil
}
func (r *LevelResources) mapSet(level int, variant MapVariant) (resourceMaps, resourceTiles, error) {
	s, ok := r.stages[level]
	if !ok {
		return resourceMaps{}, resourceTiles{}, errors.New("stage resources are not loaded")
	}
	switch variant {
	case RegularMap:
		return s.Maps, r.tiles[s.TileSetID], nil
	case BaseBossMap:
		if s.BaseBoss != nil {
			return r.extra[s.BaseBoss.MapSetID], r.tiles[s.BaseBoss.TileSetID], nil
		}
	}
	return resourceMaps{}, resourceTiles{}, errors.New("map variant is not loaded")
}
func (r *LevelResources) LoadMap(level int, variant MapVariant, index, offset int, target *[128]uint8) error {
	if target == nil || offset != 0 && offset != 64 {
		return errors.New("invalid map destination")
	}
	m, _, err := r.mapSet(level, variant)
	if err != nil {
		return err
	}
	entry, err := findResourceMap(m, index)
	if err != nil {
		return err
	}
	copy(target[offset:offset+len(entry.decoded)], entry.decoded)
	return nil
}
func (r *LevelResources) Screen(level int, variant MapVariant, index int, alternate bool) (ExpandedScreen, error) {
	m, t, err := r.mapSet(level, variant)
	if err != nil {
		return ExpandedScreen{}, err
	}
	e, err := findResourceMap(m, index)
	if err != nil {
		return ExpandedScreen{}, err
	}
	s := r.stages[level]
	limits := s.InitialPaletteState.CollisionLimits
	if alternate {
		limits = s.Alternate.CollisionLimits
	}
	screen := ExpandedScreen{SupertileIDs: append([]uint8(nil), e.decoded...), Tiles: make([]uint8, m.Rows*128), Attributes: make([]uint8, len(e.decoded))}
	for cell, id := range e.decoded {
		tile := t.Supertiles[id]
		row, col := cell/8, cell%8
		screen.Attributes[cell] = tile.Attribute
		for y := range 4 {
			start := (row*4+y)*32 + col*4
			copy(screen.Tiles[start:start+4], tile.Tiles[y*4:y*4+4])
		}
		for y := range 2 {
			for x := range 2 {
				cy, cx := row*2+y, col*2+x
				screen.Collision[cy*4+cx/4] |= collisionBits(tile.Tiles[y*8+x*2], limits) << (6 - 2*(cx%4))
			}
		}
	}
	return screen, nil
}
func (r *LevelResources) Graphics(level int, variant MapVariant) (GraphicsTileset, error) {
	_, t, err := r.mapSet(level, variant)
	if err != nil {
		return GraphicsTileset{}, err
	}
	set := GraphicsTileset{SourceAttributePage: t.SourceAttributePage, BaseAttributes: make([]uint8, len(t.Supertiles)), Updates: make([]GraphicsSupertile, t.UpdateCount)}
	for i, s := range t.Supertiles {
		set.BaseAttributes[i] = s.Attribute
	}
	copy(set.Updates, t.Supertiles[t.UpdateBase:])
	return set, nil
}
func (r *LevelResources) CHRPreset(id int) ([8]uint8, error) {
	v, ok := r.presets[id]
	if !ok {
		return [8]uint8{}, fmt.Errorf("CHR preset %d is not loaded", id)
	}
	return v, nil
}
func (r *LevelResources) Palette(indices [8]uint8) ([32]uint8, error) {
	var out [32]uint8
	for i, id := range indices {
		colors, ok := r.colors[int(id)]
		if !ok {
			return out, errors.New("palette color group is not loaded")
		}
		out[i*4] = 0x0f
		copy(out[i*4+1:i*4+4], colors[:])
	}
	return out, nil
}
