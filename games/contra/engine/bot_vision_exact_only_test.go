package contra

// Before pruning impossible partial candidates: exact small/single-color art
// must stay recognizable, near-matches must stay rejected, and one physical
// bright anchor must not acquire two votes through an optimization.
import "testing"

func TestBotVisionExactOnlyPlayerArtwork(t *testing.T) {
	for _, size := range []int{8, 16} {
		art := BotVisionTemplate{Width: size, Height: size, Label: BotVisionLabel{Class: "player", Player: 0, Sprite: 115}, RGBA: make([]byte, size*size*4)}
		for i := 0; i < len(art.RGBA); i += 4 {
			art.RGBA[i], art.RGBA[i+3] = 200, 255
		}
		vision, err := NewBotVision(BotVisionKnowledge{Sprites: []BotVisionTemplate{art}})
		if err != nil {
			t.Fatal(err)
		}
		frame := botVisionTestFrame()
		botVisionPlace(&frame, art, 80, 90)
		for y := 20; y < 20+size; y++ {
			for x := 10; x < 10+size-1; x++ {
				frame.pixels[(y*256+x)*4] = 200
			}
		}
		observed, err := ObserveBotFrame(frame, vision)
		if err != nil {
			t.Fatal(err)
		}
		p := observed.Players[0]
		if !p.Visible || p.X != 80 || p.Y != 90 {
			t.Fatal("exact-only art changed", size, p)
		}
	}
}

func TestBotVisionSinglePhysicalAnchorDoesNotBecomeTwoVotes(t *testing.T) {
	art := BotVisionTemplate{Width: 3, Height: 1, Label: BotVisionLabel{Class: "player", Player: 0}, RGBA: []byte{0, 0, 0, 255, 200, 0, 0, 255, 0, 0, 0, 255}}
	vision, err := NewBotVision(BotVisionKnowledge{Sprites: []BotVisionTemplate{art}})
	if err != nil {
		t.Fatal(err)
	}
	frame := botVisionTestFrame()
	botVisionPlace(&frame, art, 80, 90)
	observed, err := ObserveBotFrame(frame, vision)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Players[0].Visible || len(observed.PlayerCandidates) != 0 {
		t.Fatal("one anchor became sufficient evidence")
	}
}
