package contra

import "testing"

// Visible moving-platform art can establish support even where background
// terrain is unknown. Do not fabricate a background tile to enable climbing.
func TestBotVisibleMovingPlatformAllowsGroundedClimb(t *testing.T) {
	o := BotObservation{Tick: 2, Actor: 0, Seats: 1, Phase: "battle", Stage: 2, StageKnown: true, SceneKnown: true, Vertical: true, GridKnown: true}
	o.Players[0] = BotPlayer{X: 100, Y: 100, Sprite: 13, Alive: true, Visible: true}
	o.Enemies = []BotEnemy{{ID: 0, X: 100, Y: 128, Kind: 16, Sprite: 72}}
	for x := 9; x <= 10; x++ {
		o.Terrain[5][x] = TerrainPlatform
		o.TerrainKnown[5][x] = true
	}
	b := Bot{LastTick: 1, LastX: 100, LastY: 100, Stage: 2, StageKnown: true, SceneKnown: true, Phase: "battle", GridKnown: true}
	if pad := b.Decide(o); pad&ButtonA == 0 {
		t.Fatal("visible support is discarded because background is unknown", pad)
	}
	if o.terrainKnownAt(100, 116) {
		t.Fatal("dynamic support was injected into terrain")
	}
}
