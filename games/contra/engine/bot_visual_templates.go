package contra

import (
	"crypto/sha256"
	"errors"
)

// A recipe describes fixed artwork knowledge, never a live entity. The caller
// explicitly chooses a bounded set of legal visual variants; this constructor
// does not enumerate every sprite/palette/preset combination or label enemies.
type BotVisualSpriteRecipe struct {
	Mapping          [8]uint8
	Palette          [32]uint8
	Code, Attributes uint8
	Label            BotVisionLabel
}
type botTemplateCatalog struct {
	source BotVisualCatalog
	rgb    []uint8
	pages  map[uint8][]uint8
}

func (c *botTemplateCatalog) RGB() ([]uint8, error) { return c.rgb, nil }
func (c *botTemplateCatalog) CHRPage(id uint8) ([]uint8, error) {
	if p, ok := c.pages[id]; ok {
		return p, nil
	}
	p, e := c.source.CHRPage(id)
	if e != nil {
		return nil, e
	}
	if id >= 128 || len(p) != 4096 {
		return nil, errors.New("invalid template page")
	}
	for _, v := range p {
		if v > 3 {
			return nil, errors.New("invalid template pixel")
		}
	}
	copy := append([]uint8(nil), p...)
	c.pages[id] = copy
	return copy, nil
}

// BuildBotVisualSpriteTemplates uses the authenticated static sprite composer
// and the same final-pixel provider as the Canvas contract. Two distinct plain
// backdrops distinguish transparent pixels without relying on palette indices.
// Native actor anchors are retained after trimming. No World or slot is read.
func BuildBotVisualSpriteTemplates(catalog BotVisualCatalog, recipes []BotVisualSpriteRecipe) ([]BotVisionTemplate, error) {
	if catalog == nil {
		return nil, errors.New("missing visual template catalog")
	}
	rgb, e := catalog.RGB()
	if e != nil {
		return nil, e
	}
	if len(rgb) != 192 {
		return nil, errors.New("invalid visual template RGB")
	}
	local := &botTemplateCatalog{source: catalog, rgb: append([]uint8(nil), rgb...), pages: map[uint8][]uint8{}}
	first, second := uint8(0), uint8(0)
	for i := 1; i < 64; i++ {
		if rgb[i*3] != rgb[0] || rgb[i*3+1] != rgb[1] || rgb[i*3+2] != rgb[2] {
			second = uint8(i)
			break
		}
	}
	if second == first {
		return nil, errors.New("visual palette cannot identify artwork")
	}
	descriptors, e := spriteCompositionData()
	if e != nil {
		return nil, e
	}
	memo := map[[32]byte]BotVisionTemplate{}
	var result []BotVisionTemplate
	for _, recipe := range recipes {
		if recipe.Code == 0 || len(descriptors.Frames[int(recipe.Code)-1].Parts) == 0 {
			continue
		}
		state, e := NewSpriteCompositionState()
		if e != nil {
			return nil, e
		}
		input := SpriteCompositionInput{}
		input.Objects[0] = LogicalSprite{Code: recipe.Code, X: 128, Y: 112, Attributes: recipe.Attributes}
		if _, e = state.Compose(input); e != nil {
			return nil, e
		}
		view := PresentedFrame{OAM: state.OAM, Patterns: recipe.Mapping, Palette: recipe.Palette, Scroll: CollisionScroll{NameTable: 0xb0}, Display: DisplayState{AppliedMask: 0x14}}
		signature, e := botTemplateSignature(view, local)
		if e != nil {
			return nil, e
		}
		if cached, ok := memo[signature]; ok {
			if cached.Width != 0 {
				cached.Label = recipe.Label
				cached.RGBA = append([]uint8(nil), cached.RGBA...)
				result = append(result, cached)
			}
			continue
		}

		view.Palette[0] = first
		a, e := NewBotVisualFrame(view, BotPublicHUD{Seats: 1}, local, 0, 0)
		if e != nil {
			return nil, e
		}
		view.Palette[0] = second
		b, e := NewBotVisualFrame(view, BotPublicHUD{Seats: 1}, local, 0, 0)
		if e != nil {
			return nil, e
		}
		left, top, right, bottom, count := 256, 240, -1, -1, 0
		same := func(i int) bool {
			at := i * 4
			return a.pixels[at] == b.pixels[at] && a.pixels[at+1] == b.pixels[at+1] && a.pixels[at+2] == b.pixels[at+2]
		}
		for i := 0; i < 61440; i++ {
			if !same(i) {
				continue
			}
			x, y := i%256, i/256
			left = min(left, x)
			top = min(top, y)
			right = max(right, x)
			bottom = max(bottom, y)
			count++
		}
		if count < 3 {
			memo[signature] = BotVisionTemplate{}
			continue
		}
		width, height := right-left+1, bottom-top+1
		if width > 128 || height > 128 {
			return nil, errors.New("visual recipe exceeds identifiable template bounds")
		}
		template := BotVisionTemplate{Width: width, Height: height, AnchorX: 128 - left, AnchorY: 112 - top, RGBA: make([]uint8, width*height*4), Label: recipe.Label}
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				src := (top+y)*256 + left + x
				if !same(src) {
					continue
				}
				copy(template.RGBA[(y*width+x)*4:(y*width+x)*4+4], a.pixels[src*4:src*4+4])
			}
		}
		memo[signature] = template
		result = append(result, template)
	}
	return result, nil
}

// The key describes actual ordered sprite coverage and decoded RGB, not page
// numbers, raw tile IDs, palette aliases, sprite code or semantic labels. Empty
// visible OAM entries remain: they still consume the scanline sprite budget.
func botTemplateSignature(view PresentedFrame, catalog *botTemplateCatalog) ([32]byte, error) {
	r := botVisualRaster{frame: view}
	copy(r.rgb[:], catalog.rgb)
	for slot, id := range view.Patterns {
		p, e := catalog.CHRPage(id)
		if e != nil {
			return [32]byte{}, e
		}
		r.pages[slot] = (*[4096]uint8)(p)
	}
	key := make([]uint8, 0, 4096)
	for slot := 0; slot < 64; slot++ {
		at := slot * 4
		top := int(view.OAM[at]) + 1
		if top >= 240 {
			continue
		}
		key = append(key, view.OAM[at], view.OAM[at+3])
		for row := 0; row < 16; row++ {
			for col := 0; col < 8; col++ {
				x, y := int(view.OAM[at+3])+col, top+row
				if x >= 256 || y >= 240 {
					key = append(key, 0, 0, 0, 0)
					continue
				}
				v := r.spritePixel(slot, y, col, 0xb0)
				if v == 0 {
					key = append(key, 0, 0, 0, 0)
					continue
				}
				color := view.Palette[16+int(view.OAM[at+2]&3)*4+int(v)] & 63
				key = append(key, 1, r.rgb[int(color)*3], r.rgb[int(color)*3+1], r.rgb[int(color)*3+2])
			}
		}
	}
	return sha256.Sum256(key), nil
}
