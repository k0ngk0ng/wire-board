package contra

// Before partial-character matching: mutually occluding co-op figures must be
// localized only from their remaining visible colored pixels, as candidates
// when their whole artwork is not visible; hidden/gray
// frames cannot reveal seat identity; overlapping roles must not suppress each
// other merely because their rectangles overlap. No rule position is supplied.
import "testing"

func TestBotVisionOverlappingCartridgePlayers(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	vision, e := NewBuiltinBotVision(catalog, bundle)
	if e != nil {
		t.Fatal(e)
	}
	res, e := bundle.Stage(0)
	if e != nil {
		t.Fatal(e)
	}
	info, e := res.Stage(0)
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewSpriteCompositionState()
	if e != nil {
		t.Fatal(e)
	}
	input := SpriteCompositionInput{Level: 0, Flicker: true}
	input.Objects[0] = LogicalSprite{Code: 15, X: 156, Y: 100}
	input.Objects[1] = LogicalSprite{Code: 15, X: 143, Y: 100, Attributes: 5}
	if _, e = s.Compose(input); e != nil {
		t.Fatal(e)
	}
	p := PresentedFrame{OAM: s.OAM, Scroll: CollisionScroll{NameTable: 0xb0}, Display: DisplayState{AppliedMask: 20}}
	p.Patterns, e = res.CHRPreset(info.CHRPreset)
	if e != nil {
		t.Fatal(e)
	}
	p.Palette, e = res.Palette(info.Initial.PaletteIndices)
	if e != nil {
		t.Fatal(e)
	}
	hud := BotPublicHUD{Seats: 2, Players: [2]BotPublicPlayer{{Occupied: true}, {Occupied: true}}}
	frame, e := NewBotVisualFrame(p, hud, catalog, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	observed, e := ObserveBotFrame(frame, vision)
	if e != nil {
		t.Fatal(e)
	}
	for i, x := range []int{156, 143} {
		got := observed.Players[i]
		located := got.Visible && absBot(got.X-x) <= 3 && absBot(got.Y-100) <= 3
		for _, candidate := range observed.PlayerCandidates {
			p := candidate.Player
			located = located || candidate.Mask&(1<<uint(i)) != 0 && absBot(p.X-x) <= 3 && absBot(p.Y-100) <= 3
		}
		if !located {
			t.Fatalf("occluded role%d lost its visible geometric candidate: %+v %+v", i, got, observed.PlayerCandidates)
		}
	}
	if observed.Players[1].Visible {
		t.Fatal("partly hidden role was promoted to an unambiguous full body")
	}
	p.Display.AppliedMask = 0
	frame, e = NewBotVisualFrame(p, hud, catalog, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	none, e := ObserveBotFrame(frame, vision)
	if e != nil {
		t.Fatal(e)
	}
	if none.Players[0].Visible || none.Players[1].Visible || len(none.Enemies) != 0 {
		t.Fatal("uniform hidden pixels became recognizable objects")
	}
}
