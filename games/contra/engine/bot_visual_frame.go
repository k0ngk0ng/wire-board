package contra

import "errors"

// BotVisualCatalog supplies immutable, source-verified visual assets. The page
// layout is the same 64x64 indexed atlas used by RuntimeRenderer. A host may
// fetch/cache these pages from its verified asset bundle; no ROM is executed.
type BotVisualCatalog interface {
	CHRPage(id uint8) ([]uint8, error)
	RGB() ([]uint8, error)
}
type BotPublicNumber struct {
	Known bool
	Value uint8
}
type BotPublicBool struct {
	Known bool
	Value bool
}
type BotPublicPlayer struct {
	Occupied      bool
	Lives, Weapon BotPublicNumber
	Alive         BotPublicBool
}

// HUD must come from the same actual public UI as human players. Unknown fields
// are canonicalized to zero and never filled from rules by this provider.
type BotPublicHUD struct {
	Seats                int
	Phase                string
	Stage, Scene, Choice BotPublicNumber
	Paused               BotPublicBool
	Players              [2]BotPublicPlayer
}

// BotVisualFrame is a frozen, final-pixel classifier input. It deliberately
// retains no OAM, offscreen nametable, palette index, collision, entity slot,
// velocity, HP or RNG field. Those cannot become strategy-side shortcuts.
type BotVisualFrame struct {
	tick   uint64
	actor  int
	hud    BotPublicHUD
	pixels []uint8
}

func (f BotVisualFrame) Tick() uint64      { return f.tick }
func (f BotVisualFrame) Actor() int        { return f.actor }
func (f BotVisualFrame) HUD() BotPublicHUD { return f.hud }
func (f BotVisualFrame) Pixels() []uint8   { return append([]uint8(nil), f.pixels...) }

// ForActor reuses immutable pixels for the other seat without another render or
// asset load. Pixels() still returns a separate caller-owned copy.
func (f BotVisualFrame) ForActor(actor int) (BotVisualFrame, error) {
	if actor < 0 || actor >= f.hud.Seats || len(f.pixels) != 256*240*4 {
		return BotVisualFrame{}, errors.New("invalid visual-frame seat")
	}
	f.actor = actor
	return f, nil
}
func NewBotVisualFrame(p PresentedFrame, hud BotPublicHUD, catalog BotVisualCatalog, actor int, tick uint64) (BotVisualFrame, error) {
	if catalog == nil || hud.Seats < 1 || hud.Seats > 2 || actor < 0 || actor >= hud.Seats || p.Scroll.Y >= 240 {
		return BotVisualFrame{}, errors.New("invalid public visual frame")
	}
	number := func(v BotPublicNumber) BotPublicNumber {
		if !v.Known {
			return BotPublicNumber{}
		}
		return v
	}
	boolean := func(v BotPublicBool) BotPublicBool {
		if !v.Known {
			return BotPublicBool{}
		}
		return v
	}
	hud.Stage, hud.Scene, hud.Choice = number(hud.Stage), number(hud.Scene), number(hud.Choice)
	hud.Paused = boolean(hud.Paused)
	for i := range hud.Players {
		v := &hud.Players[i]
		if i >= hud.Seats || !v.Occupied {
			*v = BotPublicPlayer{}
			continue
		}
		v.Lives, v.Weapon = number(v.Lives), number(v.Weapon)
		v.Alive = boolean(v.Alive)
	}
	rgb, e := catalog.RGB()
	if e != nil {
		return BotVisualFrame{}, e
	}
	if len(rgb) != 192 {
		return BotVisualFrame{}, errors.New("invalid visual RGB palette")
	}
	var r botVisualRaster
	copy(r.rgb[:], rgb)
	r.frame = p
	r.frame.Writes = nil
	// Load at most the eight mapped pages, only once per unique mapping. Freeze
	// caller buffers so a concurrent resource cache cannot mutate this frame.
	loaded := map[uint8]*[4096]uint8{}
	for slot, id := range p.Patterns {
		if id >= 128 {
			return BotVisualFrame{}, errors.New("invalid visual CHR page")
		}
		page := loaded[id]
		if page == nil {
			data, e := catalog.CHRPage(id)
			if e != nil {
				return BotVisualFrame{}, e
			}
			if len(data) != 4096 {
				return BotVisualFrame{}, errors.New("invalid visual CHR dimensions")
			}
			page = new([4096]uint8)
			for i, v := range data {
				if v > 3 {
					return BotVisualFrame{}, errors.New("invalid visual CHR pixel")
				}
				page[i] = v
			}
			loaded[id] = page
		}
		r.pages[slot] = page
	}
	pixels, e := r.render()
	if e != nil {
		return BotVisualFrame{}, e
	}
	return BotVisualFrame{tick: tick, actor: actor, hud: hud, pixels: pixels}, nil
}
