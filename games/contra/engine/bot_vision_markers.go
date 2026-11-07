package contra

import "errors"

// Text recipes are fixed public artwork. Their meaning is established only by
// matching the rendered words/digits and cursor geometry, not by a routine byte.
func botBuiltinMarkerTemplates(catalog BotVisualCatalog, bundle *LevelBundle) ([]BotVisionTemplate, error) {
	palettePackets, e := sceneTextPackets(6, false, false)
	if e != nil {
		return nil, e
	}
	if len(palettePackets) != 1 || len(palettePackets[0].Values) != 32 {
		return nil, errors.New("missing public menu palette")
	}
	var palette [32]uint8
	copy(palette[:], palettePackets[0].Values)
	for i := 16; i < 32; i += 4 {
		palette[i-16] = palette[i]
	}
	mapping := campaignScenePresets[10]
	menu := PresentedFrame{Patterns: mapping, Palette: palette, Scroll: CollisionScroll{NameTable: 0xb0}, Display: DisplayState{AppliedMask: 10}}
	var out []BotVisionTemplate
	add := func(base PresentedFrame, packet GraphicsTextPacket, label BotVisionLabel) error {
		if packet.Kind != 1 || packet.Address < 0x2000 || packet.Address >= 0x3000 || len(packet.Values) == 0 || len(packet.Values) > 16 {
			return errors.New("unsupported public text artwork")
		}
		for i, v := range packet.Values {
			base.Background[(int(packet.Address)-0x2000+i)&0x7ff] = v
		}
		image, e := NewBotVisualFrame(base, BotPublicHUD{Seats: 1}, catalog, 0, 0)
		if e != nil {
			return e
		}
		x, y := int(packet.Address&31)*8, int(packet.Address&0x3ff)/32*8
		width, height := len(packet.Values)*8, 8
		if x+width > 256 || y+height > 240 {
			return errors.New("public text rectangle crosses screen")
		}
		template := BotVisionTemplate{Width: width, Height: height, RGBA: make([]byte, width*height*4), Label: label}
		pixels := image.Pixels()
		for row := 0; row < height; row++ {
			copy(template.RGBA[row*width*4:], pixels[((y+row)*256+x)*4:((y+row)*256+x+width)*4])
		}
		out = append(out, template)
		return nil
	}
	over, e := sceneTextPackets(13, false, false)
	if e != nil {
		return nil, e
	}
	for _, packet := range over {
		if e = add(menu, packet, BotVisionLabel{Class: "game-over-word", Player: -1}); e != nil {
			return nil, e
		}
	}
	choices, e := sceneTextPackets(14, false, false)
	if e != nil {
		return nil, e
	}
	for i, packet := range choices {
		class := "continue-word"
		if i != 0 {
			class = "end-word"
		}
		if e = add(menu, packet, BotVisionLabel{Class: class, Player: -1}); e != nil {
			return nil, e
		}
	}
	cursor, e := BuildBotVisualSpriteTemplates(catalog, []BotVisualSpriteRecipe{{Mapping: mapping, Palette: palette, Code: 170, Label: BotVisionLabel{Class: "menu-cursor", Player: -1}}})
	if e != nil {
		return nil, e
	}
	out = append(out, cursor...)
	intro := PresentedFrame{Patterns: mapping, Palette: campaignSceneIntroPalette, Scroll: CollisionScroll{NameTable: 0xb0}, Display: DisplayState{AppliedMask: 10}}
	for _, write := range sceneBackground(14) {
		for i, v := range write.Values {
			intro.Background[(int(write.Address)-0x2000+i*int(write.Step))&0x7ff] = v
		}
	}
	for stage := uint8(0); stage < 8; stage++ {
		packets, e := sceneTextPackets(12, false, false)
		if e != nil {
			return nil, e
		}
		packets[0].Values[len(packets[0].Values)-3] = stage + 2
		res, e := bundle.Stage(stage)
		if e != nil {
			return nil, e
		}
		info, e := res.Stage(int(stage))
		if e != nil {
			return nil, e
		}
		for _, packet := range packets {
			if e = add(intro, packet, BotVisionLabel{Class: "stage-title", Player: -1, Phase: "intro", Stage: BotPublicNumber{Known: true, Value: stage}, Scene: BotPublicNumber{Known: true, Value: info.SceneType}}); e != nil {
				return nil, e
			}
		}
	}
	return out, nil
}

func applyBotVisionMarkers(matches []botVisionMatch, hud BotPublicHUD, out *BotObservation) {
	defer func() {
		if out.Phase == "intro" || out.Phase == "continue" || out.Phase == "game-over" || out.Phase == "finished" {
			for i := range out.Players {
				p := &out.Players[i]
				p.Visible = false
				p.X, p.Y, p.Sprite, p.Appearance = 0, 0, 0, 0
			}
			out.PlayerCandidates = nil
		}
	}()
	over, offer := false, false
	var words, cursors []botVisionMatch
	for _, m := range matches {
		switch m.pattern.label.Class {
		case "game-over-word":
			// The per-seat HUD uses the same glyphs near the top edge. Only
			// the centered original menu word establishes a global phase.
			over = over || m.x == 80 && m.y == 136
			for seat, x := range [2]int{8, 176} {
				if seat < out.Seats && m.x == x && m.y == 40 {
					p := &out.Players[seat]
					p.GameOverKnown, p.GameOver = true, true
					p.AliveKnown, p.Alive = true, false
					if out.Phase == "" {
						out.Phase = "battle"
					}
				}
			}
		case "continue-word":
			offer = true
			words = append(words, m)
		case "end-word":
			words = append(words, m)
		case "menu-cursor":
			cursors = append(cursors, m)
		}
	}
	if !over {
		return
	}
	if hud.Phase == "" {
		out.Phase = "game-over"
		if offer {
			out.Phase = "continue"
		}
	}
	if !offer || hud.Choice.Known {
		return
	}
	for _, cursor := range cursors {
		for _, word := range words {
			if cursor.x >= word.x-24 && cursor.x <= word.x-4 && absBot(cursor.y-word.y-2) <= 4 {
				out.ChoiceKnown = true
				out.Choice = 0
				if word.pattern.label.Class == "end-word" {
					out.Choice = 1
				}
				return
			}
		}
	}
}
