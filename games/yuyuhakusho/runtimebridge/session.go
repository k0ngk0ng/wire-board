// Package runtimebridge combines independent rules and decoded client assets.
// It accepts no ROM, emulator state, CPU instructions or reference game frame.
package runtimebridge

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/presentation"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/resourceid"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/sessiontick"
	"strconv"
)

type Assets struct {
	rules      engine.ChuWorldLibrary
	sprites    presentation.Catalog
	arena      presentation.ArenaCatalog
	hud        presentation.HUDCatalog
	digests    [4]string
	arenaID    uint16
	aux        *presentation.ArenaAAuxCatalog
	auxSHA     string
	graphics   renderstate.Library
	descriptor resourceid.Descriptor
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

// snapshot owns only common rule state. Presentation is a mutually exclusive
// Session variant, never a legacy pixel state plus a metadata state together.
type snapshot struct {
	Schema         int                      `json:"schema"`
	AssetDigests   [4]string                `json:"asset_digests"`
	Tick           uint64                   `json:"tick"`
	Options        engine.ChuSessionOptions `json:"options"`
	World          engine.ChuWorldState     `json:"world"`
	ArenaAuxDigest string                   `json:"arena_aux_digest,omitempty"`
}
type legacyPresentation struct {
	Slots presentation.TileSlots
	HUD   presentation.HUDState
}
type Session struct {
	assets   Assets
	saved    snapshot
	legacy   *legacyPresentation
	computed *renderstate.State
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
	Frame          string       `json:"frame,omitempty"`
}
type Frame struct {
	View                                            View
	Pixels                                          []byte
	DrawRequests, SpritePixels, ArenaSpriteRequests int
	SoundRequests                                   []uint16
}

func validateNetworkOptions(options engine.ChuSessionOptions) error {
	n := len(options.Players)
	slots := []uint8{0, 3}
	if n == 3 {
		slots = []uint8{0, 1, 3}
	} else if n == 4 {
		slots = []uint8{0, 1, 2, 3}
	} else if n != 2 {
		return errors.New("unsupported network player count")
	}
	for seat, p := range options.Players {
		if p.Slot != slots[seat] || p.HumanChannel != uint8(seat) {
			return errors.New("network slot/controller projection differs from platform seats")
		}
	}
	return nil
}
func NewSession(options engine.ChuSessionOptions, assets Assets) (*Session, error) {
	if options.Arena != assets.arenaID {
		return nil, errors.New("selected arena assets are not loaded")
	}
	world, events, e := engine.NewChuSession(options, assets.rules)
	if e != nil {
		return nil, e
	}
	options.Players = append([]engine.ChuSessionPlayer(nil), options.Players...)
	s := &Session{assets: assets, saved: snapshot{Schema: 1, AssetDigests: assets.digests, Options: options, World: world, ArenaAuxDigest: assets.auxSHA}}
	if assets.graphics.Identity().MetadataSHA256 != "" {
		if e = validateNetworkOptions(options); e != nil {
			return nil, e
		}
		visual, e := renderstate.New(assets.graphics)
		if e != nil {
			return nil, e
		}
		world, visual, e = renderstate.Advance(world, visual, events, assets.graphics)
		if e != nil {
			return nil, e
		}
		s.saved.Schema = 2
		s.saved.World = world
		s.computed = &visual
	} else {
		slots, e := presentation.DefaultTileSlots(assets.sprites)
		if e != nil {
			return nil, e
		}
		hud, e := presentation.NewHUD(assets.hud)
		if e != nil {
			return nil, e
		}
		hud, e = presentation.ApplyHUDWorldEventsForArena(hud, events, world.Cells[:256], assets.hud, options.Arena)
		if e != nil {
			return nil, e
		}
		s.legacy = &legacyPresentation{slots, hud}
	}
	return s, nil
}
func (s *Session) Ready() error {
	resource, e := engine.ChuSessionResourceForArena(s.saved.Options.Arena, s.assets.rules)
	if e != nil {
		return e
	}
	world, e := engine.CompleteChuSessionResource(s.saved.World, resource, s.assets.rules)
	if e != nil {
		return e
	}
	s.saved.World = world
	return nil
}
func (s *Session) View() View {
	complete := s.legacy != nil && s.legacy.HUD.Slots != nil || s.computed != nil && s.computed.HUD.Slots != nil
	return sessionView(s.saved, complete)
}
func sessionView(saved snapshot, windowComplete bool) View {
	w := saved.World
	v := View{Tick: saved.Tick, LogicFrame: w.Clock.LogicFrame, Ready: w.ResourceBusy == 0, Paused: w.Clock.PauseFlags&0x80 != 0, Completed: w.MatchComplete, Winner: w.Match.WinnerCode, WindowComplete: windowComplete, Arena: saved.Options.Arena, PanOffset: w.Clock.PanOffset}
	if saved.Schema == 2 {
		v.Frame = strconv.FormatUint(saved.Tick, 10)
	}
	for _, player := range saved.Options.Players {
		o := w.Objects[0x8400+uint16(player.Slot)*0x400]
		computer, character := player.Computer, player.CharacterID
		if o.Header.Behavior == 0x12668 {
			computer = o.InputChannel == 255
			character = o.Tag
		}
		v.Players = append(v.Players, PlayerView{Slot: player.Slot, CharacterID: character, Computer: computer, InputChannel: o.InputChannel, Active: o.Header.Behavior == 0x12668, Health: o.Body.Health, Energy: int16(o.Energy >> 16), X: int16(o.Body.X >> 16), Y: int16(o.Body.Y >> 16), Pose: o.Body.Pose, Action: o.Body.ActionKind})
	}
	return v
}
func (s *Session) Digests() (string, string, error) {
	if s.computed == nil {
		return "", "", errors.New("pixel-free state is unavailable for legacy session")
	}
	rules, e := renderstate.RulesDigest(s.saved.World)
	if e != nil {
		return "", "", e
	}
	visual, e := renderstate.Digest(s.saved.World, *s.computed)
	return rules, visual, e
}

// Preview renders a detached first predicted frame, never the authoritative
// frame's guessed final-object picture. It advances no stored world or cursor.
func (s *Session) Preview() (Frame, error) {
	fork, e := s.Fork()
	if e != nil {
		return Frame{}, e
	}
	if !fork.View().Ready {
		if e = fork.Ready(); e != nil {
			return Frame{}, e
		}
	}
	var held [8]engine.Buttons
	for i, input := range fork.saved.World.Controllers {
		held[i] = input.Current
	}
	return fork.Step(held)
}
func (s *Session) Step(held [8]engine.Buttons) (Frame, error) {
	if s.computed != nil {
		for _, b := range held[4:] {
			if b != 0 {
				return Frame{}, errors.New("unoccupied network input channel")
			}
		}
		var input [4]engine.Buttons
		copy(input[:], held[:4])
		mask := uint8(0)
		for seat, p := range s.saved.Options.Players {
			if s.saved.World.Match.Controllers[p.Slot] == 0 {
				mask |= 1 << seat
			}
		}
		return s.StepControlled(input, mask)
	}
	if s.legacy == nil {
		return Frame{}, errors.New("session presentation variant is unavailable")
	}
	next := s.saved
	world, events, outcome, e := engine.StepChuSessionFrame(next.World, held, s.assets.rules)
	if e != nil {
		return Frame{}, e
	}
	if outcome.ResetRequested {
		return Frame{}, errors.New("session reset requested")
	}
	if !next.World.MatchComplete {
		if next.Tick == ^uint64(0) {
			return Frame{}, errors.New("session frame counter exhausted")
		}
		next.Tick++
	}
	next.World = world
	hud, e := presentation.ApplyHUDWorldEventsForArena(s.legacy.HUD, events, world.Cells[:256], s.assets.hud, next.Options.Arena)
	if e != nil {
		return Frame{}, e
	}
	world, frame, slots, e := renderPixels(next, s.legacy.Slots, hud, events, s.assets, true)
	if e != nil {
		return Frame{}, e
	}
	next.World = world
	frame.View = sessionView(next, true)
	s.saved = next
	s.legacy = &legacyPresentation{slots, hud}
	return frame, nil
}
func (s *Session) StepControlled(held [4]engine.Buttons, automated uint8) (Frame, error) {
	return s.stepControlled(held, automated, false)
}

// StepControlledWithSounds publishes this successful tick's actual producer
// requests. Media timing is frame based; no hardware scheduler is implied.
// Requests are never retained in a save, checkpoint or authority replay ring.
func (s *Session) StepControlledWithSounds(held [4]engine.Buttons, automated uint8) (Frame, error) {
	return s.stepControlled(held, automated, true)
}

func (s *Session) stepControlled(held [4]engine.Buttons, automated uint8, sounds bool) (Frame, error) {
	if s.computed == nil || s.legacy != nil {
		return Frame{}, errors.New("controlled replay requires sole pixel-free presentation state")
	}
	next := s.saved
	if !next.World.MatchComplete && next.Tick == ^uint64(0) {
		return Frame{}, errors.New("session frame counter exhausted")
	}
	prior := *s.computed
	var requests []uint16
	var sound engine.WorldSoundHook
	if sounds {
		sound = func(op engine.WorldSoundOperation, _ engine.WorldSoundOwners, apply func() error) error {
			if err := apply(); err != nil {
				return err
			}
			// The first operation identifies one producer request even when the
			// reference's unconsumed queue would reject it. Media playback owns
			// its bounded lanes independently; rule queue bytes stay unchanged.
			if op.Request.Sequence == 0 {
				requests = append(requests, uint16(op.Request.Code))
			}
			return nil
		}
	}
	world, visual, events, _, e := sessiontick.AdvanceObserved(next.World, prior, next.Options, held, automated, s.assets.rules, s.assets.graphics, sound)
	if e != nil {
		return Frame{}, e
	}
	if !next.World.MatchComplete {
		next.Tick++
	}
	next.World = world
	hud, e := presentation.MaterializeHUDRefs(visual.HUD, s.assets.hud)
	if e != nil {
		return Frame{}, e
	}
	_, frame, slots, e := renderPixels(next, presentation.TileSlots(prior.SpriteSlots), hud, events, s.assets, false)
	if e != nil {
		return Frame{}, e
	}
	if slots != presentation.TileSlots(visual.SpriteSlots) {
		return Frame{}, errors.New("temporary pixel slots differ from canonical shared presentation")
	}
	frame.View = sessionView(next, true)
	frame.SoundRequests = requests
	s.saved = next
	s.computed = &visual
	return frame, nil
}

// renderPixels has only disposable renderer views. Schema2 never writes rule
// caches here; the shared computational Advance is their sole commit path.
func renderPixels(next snapshot, slots presentation.TileSlots, hud presentation.HUDState, events []engine.ChuWorldEvent, assets Assets, legacyCache bool) (engine.ChuWorldState, Frame, presentation.TileSlots, error) {
	frame := presentation.BeginFrame(slots, 2)
	draws, arenaSprites := 0, 0
	world := next.World
	for _, event := range events {
		var e error
		if event.Kind == "arena-sprite" {
			if assets.arenaID != 10 || assets.aux == nil || event.ObjectID != 0x9e00 || event.RawArenaSprite == nil {
				return world, Frame{}, slots, errors.New("world arena sprite is missing selected callback-time data")
			}
			frame, e = presentation.ComposeArenaASprite(frame, *event.RawArenaSprite)
			if e != nil {
				return world, Frame{}, slots, e
			}
			draws++
			arenaSprites++
			continue
		}
		if event.Kind != "draw" {
			continue
		}
		if event.Draw == nil || event.Draw.ObjectID != event.ObjectID {
			return world, Frame{}, slots, errors.New("world draw is missing callback-time data")
		}
		actor := presentation.Actor(*event.Draw)
		frame, actor, e = presentation.ComposeActor(frame, actor, assets.sprites)
		if e != nil {
			return world, Frame{}, slots, fmt.Errorf("draw slot%04x set%x parameter%x: %w", actor.ObjectID, actor.GraphicsSet, actor.Parameter, e)
		}
		draws++
		if legacyCache {
			if object := world.Objects[actor.ObjectID]; object.DrawRequest() == *event.Draw {
				object.GraphicCache = actor.Cache
				world.Objects[actor.ObjectID] = object
			}
		}
	}
	palette := presentation.CompactPalette(world.Palette.Current)
	sprites, e := presentation.RasterizeSpritesForArena(frame, assets.sprites, assets.aux, palette, 320, 224, false)
	if e != nil {
		return world, Frame{}, slots, e
	}
	window, e := presentation.HUDWindow(hud)
	if e != nil {
		return world, Frame{}, slots, e
	}
	arenaFrame, e := sceneFrame(world, palette, &window)
	if e != nil {
		return world, Frame{}, slots, e
	}
	background, e := presentation.RasterizeArena(assets.arena, arenaFrame)
	if e != nil {
		return world, Frame{}, slots, e
	}
	screen, e := presentation.CompositeArenaSprites(background, sprites)
	if e != nil {
		return world, Frame{}, slots, e
	}
	result := Frame{Pixels: screen.RGBA().Pix, DrawRequests: draws, ArenaSpriteRequests: arenaSprites}
	for i, opaque := range sprites.Opaque {
		if opaque && background.Visible[i/320] {
			result.SpritePixels++
		}
	}
	return world, result, frame.Slots, nil
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
