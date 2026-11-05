package runtimebridge

import (
	"errors"
	"maps"
	"slices"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/presentation"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/sessiontick"
)

// Fork copies every mutable owner without encoding or parsing JSON. Immutable
// validated assets are shared; neither presentation variant gains a new owner.
// The structural and reference checks are the same as an in-memory restore.
func (s *Session) Fork() (*Session, error) {
	if s == nil {
		return nil, errors.New("session is unavailable")
	}
	if err := validateCommon(s.saved, s.assets); err != nil {
		return nil, err
	}
	if s.saved.Schema == 2 && s.computed != nil && s.legacy == nil {
		if s.assets.graphics.Identity().MetadataSHA256 == "" || s.assets.descriptor.Digest() == "" {
			return nil, errors.New("schema2 assets are not loaded")
		}
		if err := validateNetworkOptions(s.saved.Options); err != nil {
			return nil, err
		}
		if err := renderstate.Validate(s.saved.World, *s.computed, s.assets.graphics); err != nil {
			return nil, err
		}
	} else if s.saved.Schema == 1 && s.legacy != nil && s.computed == nil {
		for slot, tile := range s.legacy.Slots.Tiles {
			if _, present := s.legacy.Slots.Lookup(uint16(slot)); present && int(tile) >= len(s.assets.sprites.Tiles) {
				return nil, errors.New("legacy session references unavailable indexed tile")
			}
		}
		if _, err := presentation.HUDWindow(s.legacy.HUD); err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("session must have exactly one persistent presentation owner")
	}
	next := *s
	next.saved.Options.Players = slices.Clone(s.saved.Options.Players)
	next.saved.World.Objects = maps.Clone(s.saved.World.Objects)
	next.saved.World.Stream = slices.Clone(s.saved.World.Stream)
	next.saved.World.Throws.Records = slices.Clone(s.saved.World.Throws.Records)
	next.saved.World.Registration.Owners = slices.Clone(s.saved.World.Registration.Owners)
	if s.computed != nil {
		visual := *s.computed
		visual.HUD.Slots = maps.Clone(s.computed.HUD.Slots)
		next.computed = &visual
	} else {
		visual := *s.legacy
		visual.HUD.Slots = maps.Clone(s.legacy.HUD.Slots)
		next.legacy = &visual
	}
	return &next, nil
}

// AdvanceControlled advances every canonical rule/cache/slot/HUD operation but
// omits disposable RGBA production. Use StepControlled on the actual final tick
// when pixels are needed: callback-time draws cannot be inferred afterwards.
func (s *Session) AdvanceControlled(held [4]engine.Buttons, automated uint8) (View, error) {
	if s == nil || s.computed == nil || s.legacy != nil {
		return View{}, errors.New("controlled replay requires sole pixel-free presentation state")
	}
	next := s.saved
	if !next.World.MatchComplete && next.Tick == ^uint64(0) {
		return View{}, errors.New("session frame counter exhausted")
	}
	world, visual, _, _, err := sessiontick.Advance(next.World, *s.computed, next.Options, held, automated, s.assets.rules, s.assets.graphics)
	if err != nil {
		return View{}, err
	}
	// Preserve renderability checks without materializing HUD pixels or raster
	// planes. Typed HUD refs were already validated by the shared advance.
	if _, err = sceneFrame(world, presentation.CompactPalette(world.Palette.Current), nil); err != nil {
		return View{}, err
	}
	if !next.World.MatchComplete {
		next.Tick++
	}
	next.World = world
	s.saved = next
	s.computed = &visual
	return sessionView(next, true), nil
}
