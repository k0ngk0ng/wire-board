// Package runtimebridge combines independent rules and decoded client assets.
// It accepts no ROM, emulator state, CPU instructions or reference game frame.
package runtimebridge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/presentation"
)

type Assets struct {
	rules   engine.ChuWorldLibrary
	sprites presentation.Catalog
	arena   presentation.ArenaCatalog
	hud     presentation.HUDCatalog
	digests [4]string
	arenaID uint16
	aux     *presentation.ArenaAAuxCatalog
	auxSHA  string
}

func DecodeAssets(rules, sprites, arena, hud []byte) (Assets, error) {
	return DecodeAssetsForArena(rules, sprites, arena, hud, nil, 0)
}

// DecodeAssetsForArena binds the selected immutable background and optional
// auxiliary material to the same rule resource before a match can be prepared.
func DecodeAssetsForArena(rules, sprites, arena, hud, auxiliary []byte, selected uint16) (Assets, error) {
	var a Assets
	var err error
	if a.rules, err = engine.DecodeChuWorldRules(rules); err != nil {
		return Assets{}, fmt.Errorf("world rules: %w", err)
	}
	if a.sprites, err = presentation.DecodeCatalog(sprites); err != nil {
		return Assets{}, fmt.Errorf("sprite assets: %w", err)
	}
	if a.arena, err = presentation.DecodeArenaCatalogForArena(arena, selected); err != nil {
		return Assets{}, fmt.Errorf("arena assets: %w", err)
	}
	if a.hud, err = presentation.DecodeHUDCatalog(hud); err != nil {
		return Assets{}, fmt.Errorf("HUD assets: %w", err)
	}
	resource, err := engine.ChuSessionResourceForArena(selected, a.rules)
	if err != nil || resource.Source != a.arena.Identity().ResourceList {
		return Assets{}, errors.New("arena rule and media resource identities differ")
	}
	var nativePalette [64]uint16
	copy(nativePalette[:16], resource.Palette0[:])
	compact := presentation.CompactPalette(nativePalette)
	for i, color := range a.arena.Palette0 {
		if compact[i] != color {
			return Assets{}, errors.New("arena rule and media initial colors differ")
		}
	}
	if selected == 10 {
		aux, decodeErr := presentation.DecodeArenaAAuxCatalog(auxiliary)
		if decodeErr != nil {
			return Assets{}, fmt.Errorf("arena auxiliary assets: %w", decodeErr)
		}
		a.aux = &aux
		sum := sha256.Sum256(auxiliary)
		a.auxSHA = hex.EncodeToString(sum[:])
	} else if len(auxiliary) != 0 {
		return Assets{}, errors.New("selected arena does not use auxiliary assets")
	}
	a.arenaID = selected
	for i, raw := range [][]byte{rules, sprites, arena, hud} {
		sum := sha256.Sum256(raw)
		a.digests[i] = hex.EncodeToString(sum[:])
	}
	return a, nil
}

type snapshot struct {
	Schema         int                      `json:"schema"`
	AssetDigests   [4]string                `json:"asset_digests"`
	Tick           uint64                   `json:"tick"`
	Options        engine.ChuSessionOptions `json:"options"`
	World          engine.ChuWorldState     `json:"world"`
	Slots          presentation.TileSlots   `json:"tile_slots"`
	HUD            presentation.HUDState    `json:"hud"`
	ArenaAuxDigest string                   `json:"arena_aux_digest,omitempty"`
}

type Session struct {
	assets Assets
	saved  snapshot
}

type PlayerView struct {
	Slot         uint8 `json:"slot"`
	CharacterID  uint8 `json:"character_id"`
	Computer     bool  `json:"computer"`
	InputChannel uint8 `json:"input_channel"`
	Active       bool  `json:"active"`
	Health       int16 `json:"health"`
	Energy       int16 `json:"energy"`
	X, Y         int16
	Pose, Action uint8
}

type View struct {
	Tick           uint64       `json:"tick"`
	LogicFrame     uint16       `json:"logic_frame"`
	Ready          bool         `json:"ready"`
	Paused         bool         `json:"paused"`
	Completed      bool         `json:"completed"`
	Winner         int16        `json:"winner"`
	WindowComplete bool         `json:"window_complete"`
	Players        []PlayerView `json:"players"`
	Arena          uint16       `json:"arena,omitempty"`
	PanOffset      uint16       `json:"pan_offset,omitempty"`
}

type Frame struct {
	View                View
	Pixels              []byte
	DrawRequests        int
	SpritePixels        int
	ArenaSpriteRequests int
}

func NewSession(options engine.ChuSessionOptions, assets Assets) (*Session, error) {
	if options.Arena != assets.arenaID {
		return nil, errors.New("selected arena assets are not loaded")
	}
	world, events, err := engine.NewChuSession(options, assets.rules)
	if err != nil {
		return nil, err
	}
	slots, err := presentation.DefaultTileSlots(assets.sprites)
	if err != nil {
		return nil, err
	}
	hud, err := presentation.NewHUD(assets.hud)
	if err != nil {
		return nil, err
	}
	hud, err = presentation.ApplyHUDWorldEventsForArena(hud, events, world.Cells[:256], assets.hud, options.Arena)
	if err != nil {
		return nil, err
	}
	options.Players = append([]engine.ChuSessionPlayer(nil), options.Players...)
	return &Session{assets: assets, saved: snapshot{Schema: 1, AssetDigests: assets.digests, Options: options, World: world, Slots: slots, HUD: hud, ArenaAuxDigest: assets.auxSHA}}, nil
}

func (s *Session) Ready() error {
	resource, err := engine.ChuSessionResourceForArena(s.saved.Options.Arena, s.assets.rules)
	if err != nil {
		return err
	}
	world, err := engine.CompleteChuSessionResource(s.saved.World, resource, s.assets.rules)
	if err != nil {
		return err
	}
	s.saved.World = world
	return nil
}

func (s *Session) View() View { return sessionView(s.saved) }

func sessionView(saved snapshot) View {
	w := saved.World
	v := View{Tick: saved.Tick, LogicFrame: w.Clock.LogicFrame, Ready: w.ResourceBusy == 0,
		Paused: w.Clock.PauseFlags&0x80 != 0, Completed: w.MatchComplete, Winner: w.Match.WinnerCode, WindowComplete: saved.HUD.Slots != nil, Arena: saved.Options.Arena, PanOffset: w.Clock.PanOffset}
	for _, player := range saved.Options.Players {
		o := w.Objects[0x8400+uint16(player.Slot)*0x400]
		computer := player.Computer
		character := player.CharacterID
		if o.Header.Behavior == 0x12668 {
			computer = o.InputChannel == 255
			character = o.Tag
		}
		v.Players = append(v.Players, PlayerView{Slot: player.Slot, CharacterID: character, Computer: computer, InputChannel: o.InputChannel,
			Active: o.Header.Behavior == 0x12668, Health: o.Body.Health, Energy: int16(o.Energy >> 16),
			X: int16(o.Body.X >> 16), Y: int16(o.Body.Y >> 16), Pose: o.Body.Pose, Action: o.Body.ActionKind})
	}
	return v
}

func (s *Session) Save() ([]byte, error) { return json.Marshal(s.saved) }

func RestoreSession(raw []byte, assets Assets) (*Session, error) {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return nil, errors.New("invalid independent session save size")
	}
	var saved snapshot
	if err := json.Unmarshal(raw, &saved); err != nil {
		return nil, err
	}
	if saved.Schema != 1 || saved.AssetDigests != assets.digests || saved.ArenaAuxDigest != assets.auxSHA || saved.Options.Arena != assets.arenaID || saved.World.Match.Arena != saved.Options.Arena || len(saved.World.Objects) > 128 || len(saved.World.Stream) != 1024 {
		return nil, errors.New("session save does not match the loaded assets or world format")
	}
	// Validate configuration without replacing any current rule state.
	if _, _, err := engine.NewChuSession(saved.Options, assets.rules); err != nil {
		return nil, err
	}
	for id := range saved.World.Objects {
		if _, err := engine.LookupWorldObject(saved.World.Objects, id); err != nil {
			return nil, err
		}
	}
	for slot, tile := range saved.Slots.Tiles {
		if _, present := saved.Slots.Lookup(uint16(slot)); present && int(tile) >= len(assets.sprites.Tiles) {
			return nil, errors.New("session save references an unavailable indexed tile")
		}
	}
	if _, err := presentation.HUDWindow(saved.HUD); err != nil {
		return nil, err
	}
	return &Session{assets: assets, saved: saved}, nil
}

// Step commits the next rule state, renderer caches and slot assignments only
// after every draw and background input has been validated and rasterized.
func (s *Session) Step(held [8]engine.Buttons) (Frame, error) {
	next := s.saved
	world, events, outcome, err := engine.StepChuSessionFrame(next.World, held, s.assets.rules)
	if err != nil {
		return Frame{}, err
	}
	if outcome.ResetRequested {
		return Frame{}, errors.New("session reset requested")
	}
	if !next.World.MatchComplete {
		next.Tick++
	}
	next.World = world
	next.HUD, err = presentation.ApplyHUDWorldEventsForArena(next.HUD, events, next.World.Cells[:256], s.assets.hud, next.Options.Arena)
	if err != nil {
		return Frame{}, err
	}
	frame := presentation.BeginFrame(next.Slots, 2)
	draws, arenaSprites := 0, 0
	for _, event := range events {
		if event.Kind == "arena-sprite" {
			if s.assets.arenaID != 10 || s.assets.aux == nil || event.ObjectID != 0x9e00 || event.RawArenaSprite == nil {
				return Frame{}, errors.New("world arena sprite is missing selected callback-time data")
			}
			frame, err = presentation.ComposeArenaASprite(frame, *event.RawArenaSprite)
			if err != nil {
				return Frame{}, err
			}
			draws++
			arenaSprites++
			continue
		}
		if event.Kind != "draw" {
			continue
		}
		if event.Draw == nil || event.Draw.ObjectID != event.ObjectID {
			return Frame{}, errors.New("world draw is missing callback-time data")
		}
		actor := presentation.Actor(*event.Draw)
		frame, actor, err = presentation.ComposeActor(frame, actor, s.assets.sprites)
		if err != nil {
			return Frame{}, fmt.Errorf("draw slot%04x set%x parameter%x: %w", actor.ObjectID, actor.GraphicsSet, actor.Parameter, err)
		}
		draws++
		// A later callback may have cleared or reused this logical slot. Its
		// newer rule-owned fields must not be overwritten by the earlier draw.
		if object := next.World.Objects[actor.ObjectID]; object.DrawRequest() == *event.Draw {
			object.GraphicCache = actor.Cache
			next.World.Objects[actor.ObjectID] = object
		}
	}
	palette := presentation.CompactPalette(next.World.Palette.Current)
	sprites, err := presentation.RasterizeSpritesForArena(frame, s.assets.sprites, s.assets.aux, palette, 320, 224, false)
	if err != nil {
		return Frame{}, err
	}
	window, err := presentation.HUDWindow(next.HUD)
	if err != nil {
		return Frame{}, err
	}
	arenaFrame, err := sceneFrame(next.World, palette, &window)
	if err != nil {
		return Frame{}, err
	}
	background, err := presentation.RasterizeArena(s.assets.arena, arenaFrame)
	if err != nil {
		return Frame{}, err
	}
	screen, err := presentation.CompositeArenaSprites(background, sprites)
	if err != nil {
		return Frame{}, err
	}
	result := Frame{View: sessionView(next), Pixels: screen.RGBA().Pix, DrawRequests: draws, ArenaSpriteRequests: arenaSprites}
	for i, opaque := range sprites.Opaque {
		if opaque && background.Visible[i/320] {
			result.SpritePixels++
		}
	}
	next.Slots = frame.Slots
	s.saved = next
	return result, nil
}

// sceneFrame resolves the selected arena's owned scroll sources and uses the
// separately reconstructed text/portrait window and observed display profile.
func sceneFrame(world engine.ChuWorldState, palette [64]uint16, window *presentation.IndexedPlane) (presentation.ArenaFrame, error) {
	var horizontal, vertical [224][2]uint16
	pan := [2][224]int16{world.PanRows(0), world.PanRows(1)}
	for y := range horizontal {
		vertical[y] = [2]uint16{uint16(world.Vertical[0]), uint16(world.Vertical[1])}
		for plane, source := range world.ScrollSources {
			if source == -0xa000 || source == -0x9e00 {
				if plane == 0 && source == -0xa000 {
					horizontal[y][plane] = uint16(pan[0][y])
				} else if plane == 1 && source == -0x9e00 {
					horizontal[y][plane] = uint16(pan[1][y])
				} else {
					return presentation.ArenaFrame{}, errors.New("unsupported arena feedback source")
				}
				continue
			}
			if source < -0x1a00 || source > -0x15c0 || source&1 != 0 {
				return presentation.ArenaFrame{}, errors.New("arena scroll row is outside the owned cell bank")
			}
			horizontal[y][plane] = world.Cells[int(source+0x1a00)/2+y]
		}
	}
	rows, err := presentation.ArenaRowsForArena(world.Match.Arena, world.BackdropColor&63, palette, horizontal, vertical)
	if err != nil {
		return presentation.ArenaFrame{}, err
	}
	return presentation.ArenaFrame{Width: 320, Height: 224, Rows: rows[:], Window: window}, nil
}
