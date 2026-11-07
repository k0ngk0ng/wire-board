package contra

// Test first: bases need fixed floors, shock, shared cleared-screen gates,
// preserved transition anchors/fractions, forced-jump guards and scene-specific
// animation/sounds. Vertical stages may consume Y fractions in both camera
// calls. Boss aim consults actor zero's jump flag. No outdoor fallbacks or
// recorded expected fire outputs are accepted by this integrated replay.
import (
	"reflect"
	"testing"
)

func TestOriginalPlayerBaseBossAndWaterfallModes(t *testing.T) {
	d := loadPlayerOracle(t, "original-player-modes.json.gz")
	checked := 0
	scenes := map[uint8]int{}
	vertical := 0
	advancing := 0
	for _, scenario := range d.Scenarios {
		windows := playerOracleWindows(t, scenario.Maps)
		for _, row := range scenario.Steps {
			values := oraclePlayerValues(d, row.Before.Player)
			peerValues := oraclePlayerValues(d, row.Before.Peer)
			player, peer := playerFromOracle(d, row.Before.Player), playerFromOracle(d, row.Before.Peer)
			world := worldFromOracle(d, row.Before.World)
			window := windows[row.Before.Map]
			input := PlayerInput{Held: Buttons(values["held"]), Pressed: Buttons(values["pressed"])}
			peerInput := PlayerInput{Held: Buttons(peerValues["held"]), Pressed: Buttons(peerValues["pressed"])}
			pool := originalWeaponPool(t, row.BeforePool)
			sounds, err := player.Step(&PlayerStepContext{Actor: row.Actor, Peer: &peer, PeerInput: peerInput, World: &world, Terrain: &window, Input: input, ScreenCleared: row.Before.ScreenCleared, Fire: pool.Fire})
			if err != nil {
				t.Fatalf("%s %s frame%d actor%d: %v", scenario.Name, row.Kind, row.Frame, row.Actor, err)
			}
			want := oraclePlayerValues(d, row.After.Player)
			for field, got := range playerAsOracle(player, input) {
				if got != want[field] {
					t.Fatalf("%s %s frame%d actor%d %s got%d want%d before=%v", scenario.Name, row.Kind, row.Frame, row.Actor, field, got, want[field], values)
				}
			}
			if peer != playerFromOracle(d, row.After.Peer) || world != worldFromOracle(d, row.After.World) {
				t.Fatalf("%s %s frame%d actor%d peer/world differs: peer=%+v wanted=%+v world=%+v wanted=%+v", scenario.Name, row.Kind, row.Frame, row.Actor, peer, playerFromOracle(d, row.After.Peer), world, worldFromOracle(d, row.After.World))
			}
			if pool != originalWeaponPool(t, row.AfterPool) {
				t.Fatalf("%s frame%d projectile pool differs", scenario.Name, row.Frame)
			}
			if !reflect.DeepEqual(append([]uint8{}, sounds...), append([]uint8{}, row.Sounds...)) {
				t.Fatalf("%s frame%d sounds %v want %v", scenario.Name, row.Frame, sounds, row.Sounds)
			}
			scenes[world.SceneType]++
			vertical += int(world.ScrollType)
			if values["indoorAdvance"] != 0 {
				advancing++
			}
			checked++
		}
	}
	if checked < 7000 || scenes[1] == 0 || scenes[128] == 0 || vertical < 100 || advancing == 0 {
		t.Fatalf("incomplete mode coverage n=%d scenes=%v vertical=%d advancing=%d", checked, scenes, vertical, advancing)
	}
	t.Logf("%d original calls: scenes=%v vertical=%d advancing=%d; all player/peer/world/pool/sound fields match", checked, scenes, vertical, advancing)
}
