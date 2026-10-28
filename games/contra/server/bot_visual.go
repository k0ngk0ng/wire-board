package contra

import (
	"errors"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
)

// BotObserver is constructed with immutable global visual knowledge before the
// module is registered. Its only per-frame input is final pixels and actual HUD;
// it cannot receive World, entity slots, collision state or a hidden stage key.
// It is called once per rule frame with canonical Actor=0 whenever any seat is
// automated. Every observation field describes the shared public screen. The
// host supplies each policy a separate value with its authenticated actor ID.
type BotObserver func(engine.BotVisualFrame) (engine.BotObservation, error)

func policyObservation(shared engine.BotObservation, actor int) engine.BotObservation {
	view := shared
	view.Actor = actor
	view.Enemies = append([]engine.BotEnemy(nil), shared.Enemies...)
	view.PlayerCandidates = append([]engine.BotPlayerCandidate(nil), shared.PlayerCandidates...)
	return view
}

type BotVisualResources struct {
	id      string
	catalog engine.BotVisualCatalog
}

func (r *BotVisualResources) ID() string {
	if r == nil {
		return ""
	}
	return r.id
}

// Catalog returns an immutable memory reader. Every returned byte slice is owned
// by the caller, so setup-time BotVision construction cannot alter host assets.
func (r *BotVisualResources) Catalog() engine.BotVisualCatalog {
	if r == nil {
		return nil
	}
	return r.catalog
}

type frozenVisualCatalog struct {
	rgb   [192]uint8
	pages map[uint8][4096]uint8
}

func (c *frozenVisualCatalog) RGB() ([]uint8, error) { return append([]uint8(nil), c.rgb[:]...), nil }
func (c *frozenVisualCatalog) CHRPage(id uint8) ([]uint8, error) {
	p, ok := c.pages[id]
	if !ok {
		return nil, errors.New("visual CHR page unavailable")
	}
	return append([]uint8(nil), p[:]...), nil
}

// NewBotVisualResources is called before ModuleWithConfig. The caller reads its
// verified published bundle before this pure constructor; all declared pages are
// validated eagerly here. Module callbacks can only read frozen in-memory data.
func NewBotVisualResources(manifest engine.BotVisualCatalogManifest, rgb []uint8, pages map[uint8][]uint8) (*BotVisualResources, error) {
	id, e := engine.BotVisualCatalogIdentity(manifest)
	if e != nil {
		return nil, e
	}
	if len(pages) != len(manifest.Pages) {
		return nil, errors.New("visual payload set differs from manifest")
	}
	checked, e := engine.NewVerifiedBotVisualCatalog(manifest, rgb, func(id uint8) ([]uint8, error) {
		p, ok := pages[id]
		if !ok {
			return nil, errors.New("missing declared visual page")
		}
		return p, nil
	})
	if e != nil {
		return nil, e
	}
	memory := &frozenVisualCatalog{pages: make(map[uint8][4096]uint8, len(manifest.Pages))}
	colors, e := checked.RGB()
	if e != nil {
		return nil, e
	}
	copy(memory.rgb[:], colors)
	for page := range manifest.Pages {
		data, e := checked.CHRPage(page)
		if e != nil {
			return nil, e
		}
		var owned [4096]uint8
		copy(owned[:], data)
		memory.pages[page] = owned
	}
	return &BotVisualResources{id: id, catalog: memory}, nil
}

// publicBotHUD mirrors Board.tsx and module.tsx, not the extra diagnostic fields
// present in the runtime snapshot. Lives includes a displayed zero. Everything
// available only inside the original Canvas must be inferred from pixels.
func publicBotHUD(snapshot runtime.Snapshot, players int) engine.BotPublicHUD {
	hud := engine.BotPublicHUD{Seats: players, Paused: engine.BotPublicBool{Known: true, Value: snapshot.Paused}}
	for i := 0; i < players; i++ {
		hud.Players[i] = engine.BotPublicPlayer{Occupied: true, Lives: engine.BotPublicNumber{Known: true, Value: snapshot.Players[i].Lives}}
	}
	return hud
}
func (h *host) botVisualFrame(s *State) (engine.BotVisualFrame, error) {
	snapshot, e := runtime.View(s.Session, s.BundleID, h.build)
	if e != nil {
		return engine.BotVisualFrame{}, e
	}
	return engine.NewBotVisualFrame(snapshot.Presentation, publicBotHUD(snapshot, s.Players), h.visuals.catalog, 0, s.Session.Random.Frames+1)
}
