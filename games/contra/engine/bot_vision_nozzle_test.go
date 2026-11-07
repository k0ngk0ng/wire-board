package contra

import "testing"

// Before nozzle knowledge: a dormant visible nozzle is absent until its flame
// extends far enough to match the flame recipe. The emitter artwork establishes
// position and direction only, never whether it is active or safe to approach.
// Unrelated jungle artwork must not acquire this family.
func TestBuiltinBotVisionDormantEnergyNozzles(t *testing.T) {
	o := botVisionObservePNG(t, "testdata/bot-energy-nozzles.png")
	for _, y := range []int{88, 136, 184} {
		found := false
		for _, e := range o.Enemies {
			if e.Family == "machine-flame-nozzle-left" && e.X == 140 && e.Y == y {
				found = true
			}
		}
		if !found {
			t.Errorf("visible dormant nozzle missing at 140,%d: %+v", y, o.Enemies)
		}
	}
}
func TestBuiltinBotVisionNozzleDoesNotAliasJungleArtwork(t *testing.T) {
	for _, name := range []string{"bot-powerup-no-player.png", "bot-npc-head-no-player.png"} {
		o := botVisionObservePNG(t, "testdata/"+name)
		for _, e := range o.Enemies {
			if e.Family == "machine-flame-nozzle-left" {
				t.Fatal("jungle art became a nozzle", name, e)
			}
		}
	}
}
