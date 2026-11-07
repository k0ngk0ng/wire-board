package contra

// Failure matrix precedes support recovery: a few verified player pixels hide
// the stone's top; an unrecognized/partial player must not supply a mask;
// transparent player holes and every unmasked stone pixel still matter;
// excessive coverage, insufficient details/colors/extent and ambiguous anchors
// fail closed. Only the support art gets this path, never generic enemy bodies.
import (
	"testing"
)

func TestBotPlayerCoveredStoneRealPNG(t *testing.T) {
	o := botVisionObservePNG(t, "testdata/bot-waterfall-player-covered-stone.png")
	if !o.Players[0].Visible {
		t.Fatal("fixture lacks a complete recognized player", o.Players, o.PlayerCandidates)
	}
	var stones []BotEnemy
	for _, e := range o.Enemies {
		if e.Kind == 16 && e.Sprite == 72 {
			stones = append(stones, e)
		}
	}
	if len(stones) != 1 || stones[0].X != 170 || stones[0].Y != 127 {
		t.Fatal("clear stone remainder lost behind feet", stones, o.Enemies)
	}
}

func supportTestArt(width, height int, label BotVisionLabel, gray bool) BotVisionTemplate {
	p := BotVisionTemplate{Width: width, Height: height, AnchorX: width / 2, AnchorY: height / 2, RGBA: make([]uint8, width*height*4), Label: label}
	seed := uint32(0x12ab34cd)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			seed ^= seed << 13
			seed ^= seed >> 17
			seed ^= seed << 5
			at := (y*width + x) * 4
			if gray {
				c := []uint8{92, 158, 224}[seed%3]
				p.RGBA[at], p.RGBA[at+1], p.RGBA[at+2] = c, c, c
			} else {
				p.RGBA[at], p.RGBA[at+1], p.RGBA[at+2] = 230, uint8(40+seed%100), uint8(130+seed%110)
			}
			p.RGBA[at+3] = 255
		}
	}
	return p
}
func TestBotPlayerCoveredSupportsRejectInsufficientEvidence(t *testing.T) {
	for _, mode := range []string{"valid", "no-player", "partial-player", "transparent-hole", "unmasked-conflict", "overcovered", "single-color", "dark-detail", "detail-loss", "narrow-details", "unmasked-dark-conflict", "generic-enemy", "ambiguous-anchor", "exact-existing", "intro", "continue", "interlude", "finished", "transition"} {
		t.Run(mode, func(t *testing.T) {
			frame := botVisionTestFrame()
			frame.hud.Seats = 1
			frame.hud.Players[1] = BotPublicPlayer{}
			stone := supportTestArt(24, 20, BotVisionLabel{Class: "enemy", Player: -1, Kind: 16, Sprite: 72}, true)
			player := supportTestArt(12, 28, BotVisionLabel{Class: "player", Player: 0, Sprite: 13}, false)
			player.RGBA[(25*12+3)*4+3] = 0
			px, py := 116, 76
			if mode == "overcovered" {
				player = supportTestArt(24, 28, player.Label, false)
				px, py = 110, 90
			}
			if mode == "exact-existing" {
				py = 70
			}
			if mode == "generic-enemy" {
				stone.Label.Kind, stone.Label.Sprite = 5, 62
			}
			if mode == "single-color" || mode == "dark-detail" {
				for at := 0; at < len(stone.RGBA); at += 4 {
					c := uint8(128)
					if mode == "dark-detail" {
						c = 16
					}
					stone.RGBA[at], stone.RGBA[at+1], stone.RGBA[at+2] = c, c, c
				}
			}

			if mode == "detail-loss" || mode == "narrow-details" {
				for y := 0; y < stone.Height; y++ {
					for x := 0; x < stone.Width; x++ {
						at := (y*stone.Width + x) * 4
						value := uint8(16)
						if mode == "detail-loss" && y < 4 && x >= 6 && x < 18 || mode == "narrow-details" && x >= 12 && x < 14 {
							value = uint8(100 + 100*((x+y)&1))
						}
						stone.RGBA[at], stone.RGBA[at+1], stone.RGBA[at+2] = value, value, value
					}
				}
			}
			if mode == "unmasked-dark-conflict" {
				at := (18*24 + 1) * 4
				stone.RGBA[at], stone.RGBA[at+1], stone.RGBA[at+2] = 16, 16, 16
			}
			botVisionPlace(&frame, stone, 110, 100)
			botVisionPlace(&frame, player, px, py)
			knowledge := BotVisionKnowledge{Sprites: []BotVisionTemplate{stone}}
			if mode != "no-player" {
				knowledge.Sprites = append(knowledge.Sprites, player)
			}
			corrupt := func(x, y int) {
				at := (y*256 + x) * 4
				frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2] = 3, 17, 239
			}
			switch mode {
			case "partial-player":
				corrupt(px+4, py+5)
			case "transparent-hole":
				corrupt(px+3, py+25)
			case "unmasked-conflict", "unmasked-dark-conflict":
				corrupt(111, 118)
			case "ambiguous-anchor":
				other := stone
				other.RGBA = append([]uint8(nil), stone.RGBA...)
				at := (1*24 + 7) * 4
				other.RGBA[at] ^= 37
				other.AnchorX++
				knowledge.Sprites = append(knowledge.Sprites, other)
			case "intro", "continue", "interlude", "finished", "transition":
				frame.hud.Phase = mode
			}
			vision, e := NewBotVision(knowledge)
			if e != nil {
				t.Fatal(e)
			}
			o, e := ObserveBotFrame(frame, vision)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "valid" || mode == "exact-existing" {
				if len(o.Enemies) != 1 || o.Enemies[0].X != 122 || o.Enemies[0].Y != 110 {
					t.Fatal("unique visible support not preserved", o.Enemies)
				}
			} else if len(o.Enemies) != 0 {
				t.Fatal("insufficient/forbidden evidence became an enemy", mode, o.Enemies)
			}
		})
	}
}

func TestBotPlayerCoveredStonePartialPNGHasNoMask(t *testing.T) {
	o := botVisionObservePNG(t, "testdata/bot-waterfall-partial-player-stone.png")
	if o.Players[0].Visible {
		t.Fatal("fixture unexpectedly supplies a full player", o.Players)
	}
	for _, e := range o.Enemies {
		if e.Kind == 16 && e.Sprite == 72 {
			t.Fatal("partial player was used to recover a stone", e)
		}
	}
}
