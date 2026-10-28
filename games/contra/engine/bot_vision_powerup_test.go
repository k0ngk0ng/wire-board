package contra

// Before correcting the recipe/recognition failure: a publicly drawn powerup
// box must not become a player during respawn flashing. The regression carries
// final RGBA only, not a World, OAM, or logical sprite identifiers.
import (
	"image/png"
	"os"
	"testing"
)

func TestBuiltinBotVisionPowerupDoesNotBecomeAvatar(t *testing.T) {
	observed := botVisionObservePNG(t, "testdata/bot-powerup-no-player.png")
	if observed.Players[0].Visible || len(observed.PlayerCandidates) > 0 {
		t.Fatal("stationary powerup became the flashing player's position", observed.Players, observed.PlayerCandidates)
	}
}

// Head artwork is not necessarily a player's head: the larger visible soldier
// can reuse the same pixels. Occupancy alone cannot turn it into this seat.
func TestBuiltinBotVisionSoldierHeadDoesNotEstablishPlayer(t *testing.T) {
	observed := botVisionObservePNG(t, "testdata/bot-npc-head-no-player.png")
	if observed.Players[0].Visible {
		t.Fatal("single-seat occupancy converted an NPC head into the player's location", observed.Players[0])
	}
}

// A partly matching standing pose must not override a complete submerged-head
// appearance and invent a certain body/seat from the overlapping pixels.
func TestBuiltinBotVisionPartialStandingPoseRemainsCandidate(t *testing.T) {
	observed := botVisionObservePNGSeats(t, "testdata/bot-coop-water-head-partial.png", 2)
	if observed.Players[0].Visible {
		t.Fatal("partial standing artwork became a definite body over the water head", observed.Players[0])
	}
	head := false
	for _, c := range observed.PlayerCandidates {
		if c.Mask&1 != 0 && c.Player.Sprite >= 24 && c.Player.Sprite <= 29 && absBot(c.Player.X-168) <= 4 && absBot(c.Player.Y-212) <= 4 {
			head = true
		}
	}
	if !head {
		t.Fatal("visible submerged head candidate was discarded", observed.PlayerCandidates)
	}
}

func botVisionObservePNG(t *testing.T, path string) BotObservation {
	t.Helper()
	return botVisionObservePNGSeats(t, path, 1)
}

func botVisionObservePNGSeats(t *testing.T, path string, seats int) BotObservation {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	picture, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	frame := botVisionTestFrame()
	frame.hud = BotPublicHUD{Seats: seats, Players: [2]BotPublicPlayer{{Occupied: true}, {Occupied: seats == 2}}}
	for y := 0; y < 240; y++ {
		for x := 0; x < 256; x++ {
			r, g, b, a := picture.At(x, y).RGBA()
			at := (y*256 + x) * 4
			frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2], frame.pixels[at+3] = uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)
		}
	}
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	vision, err := NewBuiltinBotVision(botBuiltinTestCatalog(t), bundle)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := ObserveBotFrame(frame, vision)
	if err != nil {
		t.Fatal(err)
	}
	return observed
}
