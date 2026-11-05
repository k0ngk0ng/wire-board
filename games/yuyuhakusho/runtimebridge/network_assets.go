package runtimebridge

import (
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/resourceid"
)

// AssetBytes are the exact verified decompressed JSON bytes, including any
// final newline. A JS object parse/stringify is not an identity-preserving load.
type AssetBytes struct{ Rules, Sprites, Arena, HUD, ArenaAux, RenderMetadata, Descriptor []byte }

func DecodeNetworkAssets(raw AssetBytes, selected uint16) (Assets, error) {
	descriptor, e := resourceid.Decode(raw.Descriptor)
	if e != nil {
		return Assets{}, e
	}
	arenaID := "arena-default"
	if selected != 0 {
		arenaID = fmt.Sprintf("arena-%x", selected)
	}
	for _, input := range []struct {
		id    string
		bytes []byte
	}{{"rules", raw.Rules}, {"sprites", raw.Sprites}, {"hud", raw.HUD}, {"render_meta", raw.RenderMetadata}, {arenaID, raw.Arena}} {
		if e = descriptor.VerifyJSON(input.id, input.bytes); e != nil {
			return Assets{}, fmt.Errorf("%s: %w", input.id, e)
		}
	}
	if selected == 10 {
		if e = descriptor.VerifyJSON("arena-a-aux", raw.ArenaAux); e != nil {
			return Assets{}, e
		}
	}
	graphics, e := renderstate.DecodeLibrary(raw.RenderMetadata)
	if e != nil {
		return Assets{}, e
	}
	a, e := DecodeAssetsForArena(raw.Rules, raw.Sprites, raw.Arena, raw.HUD, raw.ArenaAux, selected)
	if e != nil {
		return Assets{}, e
	}
	identity := graphics.Identity()
	if identity.RulesSHA256 != a.digests[0] || identity.SpriteCatalogSHA256 != a.digests[1] || identity.HUDCatalogSHA256 != a.digests[3] {
		return Assets{}, errors.New("computational metadata and exact client resources differ")
	}
	a.graphics = graphics
	a.descriptor = descriptor
	return a, nil
}
func (a Assets) RenderIdentity() renderstate.Identity { return a.graphics.Identity() }
func (a Assets) ResourceDescriptorSHA256() string     { return a.descriptor.Digest() }
