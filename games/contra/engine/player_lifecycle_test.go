package contra

// Failure cases fixed before lifecycle implementation: premature life loss;
// offscreen corpse never finishing; spawn above a hole/solid; clearing preserved
// special/boost state; wrong donor seat, last-life or indoor-advance eligibility;
// held A vs new A; single-player or wrong-stage borrow; duplicate hits.
import "testing"

func TestOriginalOutdoorPlayerLifecycle(t *testing.T) {
	counts := map[string]int{}
	borrowSuccess := 0
	for _, scope := range []string{"natural", "borrow-controlled"} {
		d := loadPlayerOracle(t, "original-player-lifecycle-"+scope+".json.gz")
		for _, scenario := range d.Scenarios {
			windows := playerOracleWindows(t, scenario.Maps)
			for _, record := range scenario.Steps {
				player, peer := playerFromOracle(d, record.Before.Player), playerFromOracle(d, record.Before.Peer)
				world := worldFromOracle(d, record.Before.World)
				window := windows[record.Before.Map]
				pInput := oraclePlayerValues(d, record.Before.Player)
				otherInput := oraclePlayerValues(d, record.Before.Peer)
				input := PlayerInput{Held: Buttons(pInput["held"]), Pressed: Buttons(pInput["pressed"])}
				peerInput := PlayerInput{Held: Buttons(otherInput["held"]), Pressed: Buttons(otherInput["pressed"])}
				switch record.Kind {
				case "borrow":
					players := [2]Player{player, peer}
					inputs := [2]PlayerInput{input, peerInput}
					if BorrowLife(&players, inputs, &world) {
						borrowSuccess++
					}
					player, peer = players[0], players[1]
				case "hit":
					sounds := player.Hit()
					if len(sounds) != 1 || sounds[0] != 0x72 {
						t.Fatal("hit sound differs")
					}
				case "spawn", "dying":
					pool := WeaponSystem{}
					_, err := player.Step(&PlayerStepContext{Actor: record.Actor, Peer: &peer, PeerInput: peerInput, World: &world, Terrain: &window, Input: input, Fire: pool.Fire})
					if err != nil {
						t.Fatalf("%s %s frame%d actor%d: %v", scope, record.Kind, record.Frame, record.Actor, err)
					}
				default:
					t.Fatalf("unknown lifecycle oracle %q", record.Kind)
				}
				want := oraclePlayerValues(d, record.After.Player)
				for field, got := range playerAsOracle(player, input) {
					if got != want[field] {
						t.Fatalf("%s %s frame%d actor%d %s got%d want%d before=%v", scope, record.Kind, record.Frame, record.Actor, field, got, want[field], pInput)
					}
				}
				if peer != playerFromOracle(d, record.After.Peer) || world != worldFromOracle(d, record.After.World) {
					t.Fatalf("%s %s frame%d peer/world differs", scope, record.Kind, record.Frame)
				}
				counts[record.Kind]++
			}
		}
	}
	if counts["spawn"] < 4 || counts["dying"] < 297 || counts["hit"] < 3 || counts["borrow"] < 4529 || borrowSuccess != 10 {
		t.Fatalf("missing lifecycle coverage: %v successes%d", counts, borrowSuccess)
	}
	t.Logf("original lifecycle calls %v, successful controlled borrows %d", counts, borrowSuccess)
}

func TestPlayerHitDoesNotRepeatOutsideNormalState(t *testing.T) {
	for _, state := range []uint8{0, 2, 3} {
		p := Player{State: state, Lives: 2, SpecialSpriteTimer: 19}
		before := p
		if sounds := p.Hit(); len(sounds) != 0 || p != before {
			t.Fatalf("hit changed inactive state %d", state)
		}
	}
}
