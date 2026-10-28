package contra

// Defined before recording/importing complete pre-call projectile pools:
// firearms must see post-aim/pre-motion coordinates; real occupancy and preserved
// inactive-slot positions must survive; expected recoil/timer/pool snapshots must
// never be injected as a callback result; water entry/exit must remain a rule step.
import "testing"

func TestOriginalOutdoorPlayerAndWeaponIntegration(t *testing.T) {
	d := loadPlayerOracle(t, "original-player-complete.json.gz")
	checked := 0
	for _, scenario := range d.Scenarios {
		windows := playerOracleWindows(t, scenario.Maps)
		for _, step := range scenario.Steps {
			before := oraclePlayerValues(d, step.Before.Player)
			player, peer := playerFromOracle(d, step.Before.Player), playerFromOracle(d, step.Before.Peer)
			world := worldFromOracle(d, step.Before.World)
			window := windows[step.Before.Map]
			input := PlayerInput{Held: Buttons(before["held"]), Pressed: Buttons(before["pressed"])}
			pool := originalWeaponPool(t, step.BeforePool)
			_, err := player.StepOutdoor(&PlayerStepContext{Actor: step.Actor, Peer: &peer, World: &world, Terrain: &window, Input: input, Fire: pool.Fire})
			if err != nil {
				t.Fatalf("%s frame%d actor%d: %v", scenario.Name, step.Frame, step.Actor, err)
			}
			want := oraclePlayerValues(d, step.After.Player)
			for field, got := range playerAsOracle(player, input) {
				if got != want[field] {
					t.Fatalf("%s frame%d actor%d %s got %d want %d", scenario.Name, step.Frame, step.Actor, field, got, want[field])
				}
			}
			if pool != originalWeaponPool(t, step.AfterPool) {
				t.Fatalf("%s frame%d actor%d projectile pool differs", scenario.Name, step.Frame, step.Actor)
			}
			if peer != playerFromOracle(d, step.After.Peer) || world != worldFromOracle(d, step.After.World) {
				t.Fatalf("%s frame%d peer/world differs", scenario.Name, step.Frame)
			}
			checked++
		}
	}
	if checked < 2425 {
		t.Fatalf("incomplete integration coverage: %d", checked)
	}
	t.Logf("%d original complete player+weapon calls match all fields and the entire projectile pool", checked)
}
