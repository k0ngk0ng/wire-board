package runtimebridge

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/presentation"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
)

// Checkpoint crosses the browser as an opaque exact JSON string. Present words
// may exceed2^53 and cannot survive JSON.parse followed by JSON.stringify.
type Checkpoint struct {
	Version                  int                      `json:"version"`
	Epoch                    string                   `json:"epoch"`
	Frame                    uint64                   `json:"frame,string"`
	Options                  engine.ChuSessionOptions `json:"options"`
	World                    engine.ChuWorldState     `json:"world"`
	Presentation             renderstate.State        `json:"presentation"`
	Identity                 renderstate.Identity     `json:"identity"`
	ResourceDescriptorSHA256 string                   `json:"resourceDescriptorSHA256"`
}
type legacyWire struct {
	Schema         int                      `json:"schema"`
	AssetDigests   [4]string                `json:"asset_digests"`
	Tick           uint64                   `json:"tick"`
	Options        engine.ChuSessionOptions `json:"options"`
	World          engine.ChuWorldState     `json:"world"`
	Slots          presentation.TileSlots   `json:"tile_slots"`
	HUD            presentation.HUDState    `json:"hud"`
	ArenaAuxDigest string                   `json:"arena_aux_digest,omitempty"`
}
type computedWire struct {
	snapshot
	RenderMetadataDigest     string            `json:"render_metadata_digest"`
	ResourceDescriptorSHA256 string            `json:"resource_descriptor_sha256"`
	Presentation             renderstate.State `json:"presentation"`
}

func (s *Session) Save() ([]byte, error) {
	if s.legacy != nil && s.computed == nil && s.saved.Schema == 1 {
		return json.Marshal(legacyWire{s.saved.Schema, s.saved.AssetDigests, s.saved.Tick, s.saved.Options, s.saved.World, s.legacy.Slots, s.legacy.HUD, s.saved.ArenaAuxDigest})
	}
	if s.computed != nil && s.legacy == nil && s.saved.Schema == 2 {
		return json.Marshal(computedWire{s.saved, s.assets.graphics.Identity().MetadataSHA256, s.assets.descriptor.Digest(), *s.computed})
	}
	return nil, errors.New("session must have exactly one persistent presentation owner")
}
func validateCommon(saved snapshot, assets Assets) error {
	if saved.AssetDigests != assets.digests || saved.ArenaAuxDigest != assets.auxSHA || saved.Options.Arena != assets.arenaID || saved.World.Match.Arena != saved.Options.Arena || len(saved.World.Objects) > 128 || len(saved.World.Stream) != 1024 {
		return errors.New("session save does not match loaded assets or world format")
	}
	if _, _, e := engine.NewChuSession(saved.Options, assets.rules); e != nil {
		return e
	}
	for id := range saved.World.Objects {
		if _, e := engine.LookupWorldObject(saved.World.Objects, id); e != nil {
			return e
		}
	}
	return nil
}
func RestoreSession(raw []byte, assets Assets) (*Session, error) {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return nil, errors.New("invalid independent session save size")
	}
	var common snapshot
	if e := json.Unmarshal(raw, &common); e != nil {
		return nil, e
	}
	if common.Schema != 1 && common.Schema != 2 {
		return nil, errors.New("unsupported independent session schema")
	}
	if e := validateCommon(common, assets); e != nil {
		return nil, e
	}
	if common.Schema == 1 {
		var old legacyWire
		if e := json.Unmarshal(raw, &old); e != nil {
			return nil, e
		}
		for slot, tile := range old.Slots.Tiles {
			if _, present := old.Slots.Lookup(uint16(slot)); present && int(tile) >= len(assets.sprites.Tiles) {
				return nil, errors.New("legacy session references unavailable indexed tile")
			}
		}
		if _, e := presentation.HUDWindow(old.HUD); e != nil {
			return nil, e
		}
		return &Session{assets: assets, saved: common, legacy: &legacyPresentation{old.Slots, old.HUD}}, nil
	}
	if assets.graphics.Identity().MetadataSHA256 == "" || assets.descriptor.Digest() == "" {
		return nil, errors.New("schema2 assets are not loaded")
	}
	var present map[string]json.RawMessage
	if e := json.Unmarshal(raw, &present); e != nil {
		return nil, e
	}
	if present["tile_slots"] != nil || present["hud"] != nil {
		return nil, errors.New("schema2 cannot carry legacy presentation owners")
	}
	var current computedWire
	if e := json.Unmarshal(raw, &current); e != nil {
		return nil, e
	}
	if current.RenderMetadataDigest != assets.graphics.Identity().MetadataSHA256 || current.ResourceDescriptorSHA256 != assets.descriptor.Digest() {
		return nil, errors.New("schema2 resource identity differs")
	}
	if e := validateNetworkOptions(common.Options); e != nil {
		return nil, e
	}
	if e := renderstate.Validate(common.World, current.Presentation, assets.graphics); e != nil {
		return nil, e
	}
	return &Session{assets: assets, saved: common, computed: &current.Presentation}, nil
}
func validEpoch(epoch string) bool {
	if len(epoch) != 24 {
		return false
	}
	raw, e := hex.DecodeString(epoch)
	return e == nil && hex.EncodeToString(raw) == epoch
}
func (s *Session) Checkpoint(epoch string) ([]byte, error) {
	if s.computed == nil || s.legacy != nil || !validEpoch(epoch) {
		return nil, errors.New("checkpoint requires network state and a valid epoch")
	}
	return json.Marshal(Checkpoint{1, epoch, s.saved.Tick, s.saved.Options, s.saved.World, *s.computed, s.assets.graphics.Identity(), s.assets.descriptor.Digest()})
}
func RestoreCheckpoint(raw []byte, assets Assets) (*Session, error) {
	if len(raw) == 0 || len(raw) > 1<<20 || assets.graphics.Identity().MetadataSHA256 == "" || assets.descriptor.Digest() == "" {
		return nil, errors.New("checkpoint or network assets unavailable")
	}
	var checkpoint Checkpoint
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&checkpoint); e != nil {
		return nil, e
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, errors.New("trailing checkpoint JSON")
	}
	if checkpoint.Version != 1 || !validEpoch(checkpoint.Epoch) || checkpoint.Identity != assets.graphics.Identity() || checkpoint.ResourceDescriptorSHA256 != assets.descriptor.Digest() {
		return nil, errors.New("checkpoint identity or epoch differs")
	}
	saved := snapshot{Schema: 2, AssetDigests: assets.digests, Tick: checkpoint.Frame, Options: checkpoint.Options, World: checkpoint.World, ArenaAuxDigest: assets.auxSHA}
	if e := validateCommon(saved, assets); e != nil {
		return nil, e
	}
	if e := validateNetworkOptions(saved.Options); e != nil {
		return nil, e
	}
	if e := renderstate.Validate(saved.World, checkpoint.Presentation, assets.graphics); e != nil {
		return nil, e
	}
	// The decoded map/slices are caller-detached. No pixel or last-draw history
	// is inferred from current objects; Preview will explicitly predictF+1.
	return &Session{assets: assets, saved: saved, computed: &checkpoint.Presentation}, nil
}
