package contra

// Before marker knowledge: only rendered menu words/cursor and stage digits may
// disclose phase/choice/stage. No source routine or selection byte is passed.
// A continue heading without a visible cursor must keep choice unknown; HUD
// reserve-life icons must not become a playable character position.
import "testing"

func visualTestWritePackets(t *testing.T, p *WorldPresentation, id uint8) {
	t.Helper()
	packets, e := sceneTextPackets(id, false, false)
	if e != nil {
		t.Fatal(e)
	}
	q := GraphicsQueue{Mode: GraphicsText}
	for _, packet := range packets {
		if e = q.AppendText(packet); e != nil {
			t.Fatal(e)
		}
	}
	if e = p.ApplyImmediate(q.Writes()); e != nil {
		t.Fatal(e)
	}
}
func TestBuiltinBotVisionRenderedContinueChoice(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	vision, e := NewBuiltinBotVision(catalog, bundle)
	if e != nil {
		t.Fatal(e)
	}
	p := WorldPresentation{Display: DisplayState{AppliedMask: 30}, AppliedScroll: CollisionScroll{NameTable: 0xb0}}
	p.Patterns.Applied = campaignScenePresets[10]
	for i := 0; i < 256; i += 4 {
		p.AppliedOAM[i] = 244
	}
	visualTestWritePackets(t, &p, 6)
	visualTestWritePackets(t, &p, 13)
	visualTestWritePackets(t, &p, 14)
	hud := BotPublicHUD{Seats: 1, Players: [2]BotPublicPlayer{{Occupied: true}}}
	frame, e := NewBotVisualFrame(p.Snapshot(), hud, catalog, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	seen, e := ObserveBotFrame(frame, vision)
	if e != nil {
		t.Fatal(e)
	}
	if seen.Phase != "continue" || seen.ChoiceKnown {
		t.Fatal("menu phase or hidden selection differs", seen.Phase, seen.ChoiceKnown)
	}
	for choice, y := range []uint8{162, 178} {
		sprites, e := NewSpriteCompositionState()
		if e != nil {
			t.Fatal(e)
		}
		input := SpriteCompositionInput{}
		input.Objects[0] = LogicalSprite{Code: 170, X: 82, Y: y}
		if _, e = sprites.Compose(input); e != nil {
			t.Fatal(e)
		}
		p.AppliedOAM = sprites.OAM
		frame, e = NewBotVisualFrame(p.Snapshot(), hud, catalog, 0, 2)
		if e != nil {
			t.Fatal(e)
		}
		seen, e = ObserveBotFrame(frame, vision)
		if e != nil {
			t.Fatal(e)
		}
		if seen.Phase != "continue" || !seen.ChoiceKnown || seen.Choice != uint8(choice) {
			t.Fatal("visible cursor not recovered", choice, seen.Choice, seen.ChoiceKnown)
		}
	}
}
func TestBuiltinBotVisionRenderedStageTitle(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	vision, e := NewBuiltinBotVision(catalog, bundle)
	if e != nil {
		t.Fatal(e)
	}
	for stage := uint8(0); stage < 8; stage++ {
		p := WorldPresentation{Display: DisplayState{AppliedMask: 10}, AppliedScroll: CollisionScroll{NameTable: 0xb0}}
		p.Patterns.Applied = campaignScenePresets[10]
		p.Palette.Applied = campaignSceneIntroPalette
		if e = p.ApplyImmediate(sceneBackground(14)); e != nil {
			t.Fatal(e)
		}
		packets, e := sceneTextPackets(12, false, false)
		if e != nil {
			t.Fatal(e)
		}
		packets[0].Values[len(packets[0].Values)-3] = stage + 2
		q := GraphicsQueue{Mode: GraphicsText}
		for _, packet := range packets {
			if e = q.AppendText(packet); e != nil {
				t.Fatal(e)
			}
		}
		if e = p.ApplyImmediate(q.Writes()); e != nil {
			t.Fatal(e)
		}
		frame, e := NewBotVisualFrame(p.Snapshot(), BotPublicHUD{Seats: 1, Players: [2]BotPublicPlayer{{Occupied: true}}}, catalog, 0, 1)
		if e != nil {
			t.Fatal(e)
		}
		seen, e := ObserveBotFrame(frame, vision)
		if e != nil {
			t.Fatal(e)
		}
		if seen.Phase != "intro" || !seen.StageKnown || seen.Stage != stage {
			t.Fatal("visible stage label not read", stage, seen.Phase, seen.StageKnown, seen.Stage)
		}
	}
}
func TestBuiltinBotVisionLifeHUDIsNotAvatar(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	vision, e := NewBuiltinBotVision(catalog, bundle)
	if e != nil {
		t.Fatal(e)
	}
	res, _ := bundle.Stage(0)
	info, _ := res.Stage(0)
	mapping, _ := res.CHRPreset(info.CHRPreset)
	palette, _ := res.Palette(info.Initial.PaletteIndices)
	sprites, e := NewSpriteCompositionState()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = sprites.Compose(SpriteCompositionInput{Level: 0, Flicker: true, HUD: true, TwoPlayers: true, Lives: [2]uint8{2, 2}}); e != nil {
		t.Fatal(e)
	}
	p := PresentedFrame{OAM: sprites.OAM, Patterns: mapping, Palette: palette, Scroll: CollisionScroll{NameTable: 0xb0}, Display: DisplayState{AppliedMask: 20}}
	frame, e := NewBotVisualFrame(p, BotPublicHUD{Seats: 2, Players: [2]BotPublicPlayer{{Occupied: true}, {Occupied: true}}}, catalog, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	seen, e := ObserveBotFrame(frame, vision)
	if e != nil {
		t.Fatal(e)
	}
	if seen.Players[0].Visible || seen.Players[1].Visible || len(seen.PlayerCandidates) != 0 {
		t.Fatal("reserve HUD became a body position", seen.Players, seen.PlayerCandidates)
	}
}

// Individual seat HUD messages reuse the global menu's glyphs. Their visible
// location must not stop the surviving teammate or erase its gameplay view.
func TestBuiltinBotVisionSeatGameOverIsNotGlobalMenu(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	vision, err := NewBuiltinBotVision(catalog, bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, textID := range []uint8{15, 16} {
		p := WorldPresentation{Display: DisplayState{AppliedMask: 10}, AppliedScroll: CollisionScroll{NameTable: 0xb0}}
		p.Patterns.Applied = campaignScenePresets[10]
		visualTestWritePackets(t, &p, 6)
		visualTestWritePackets(t, &p, textID)
		frame, err := NewBotVisualFrame(p.Snapshot(), BotPublicHUD{Seats: 2, Players: [2]BotPublicPlayer{{Occupied: true}, {Occupied: true}}}, catalog, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		observed, err := ObserveBotFrame(frame, vision)
		if err != nil {
			t.Fatal(err)
		}
		if observed.Phase == "game-over" || observed.Phase == "continue" {
			t.Fatal("individual visible HUD message became whole-team game over", textID, observed.Phase)
		}
		seat := int(textID - 15)
		if !observed.Players[seat].AliveKnown || observed.Players[seat].Alive {
			t.Fatal("visible seat result was not attributed to its displayed seat", textID, observed.Players)
		}
	}
}
