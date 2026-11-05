package yuyuhakusho

import (
	"crypto/rand"
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/data"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/resourceid"
)

type moduleResources struct {
	rules         engine.ChuWorldLibrary
	graphics      renderstate.Library
	descriptorSHA string
	nonce         [32]byte
	err           error
}

var resources = loadResources()

func loadResources() moduleResources {
	var r moduleResources
	rules, meta, descriptor, err := data.RuntimeResources()
	if err != nil {
		r.err = err
		return r
	}
	identity, err := resourceid.Decode(descriptor)
	if err != nil {
		r.err = err
		return r
	}
	if err = identity.VerifyJSON("rules", rules); err != nil {
		r.err = err
		return r
	}
	if err = identity.VerifyJSON("render_meta", meta); err != nil {
		r.err = err
		return r
	}
	r.rules, err = engine.DecodeChuWorldRules(rules)
	if err != nil {
		r.err = err
		return r
	}
	r.graphics, err = renderstate.DecodeLibrary(meta)
	if err != nil {
		r.err = err
		return r
	}
	id := r.graphics.Identity()
	sprite, ok := identity.Lookup("sprites")
	if !ok || sprite.RawJSONSHA256 != id.SpriteCatalogSHA256 {
		r.err = errors.New("render sprite identity mismatch")
		return r
	}
	hud, ok := identity.Lookup("hud")
	if !ok || hud.RawJSONSHA256 != id.HUDCatalogSHA256 {
		r.err = errors.New("render HUD identity mismatch")
		return r
	}
	rule, ok := identity.Lookup("rules")
	if !ok || rule.RawJSONSHA256 != id.RulesSHA256 {
		r.err = errors.New("render rule identity mismatch")
		return r
	}
	r.descriptorSHA = identity.Digest()
	_, r.err = rand.Read(r.nonce[:])
	return r
}
