package contra

// Failure cases before sharing classification: duplicate pixel matching for
// co-op seats; seat one inherits seat zero's controller decision; manual seats
// advance bot memory; a one-bot seat-one room cannot use canonical observation.
import (
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"testing"
)

func TestContraSharedObservationCopiesMutableCandidates(t *testing.T) {
	shared := engine.BotObservation{Actor: 0, Seats: 2, Enemies: []engine.BotEnemy{{X: 12}}, PlayerCandidates: []engine.BotPlayerCandidate{{Mask: 3}}}
	left, right := policyObservation(shared, 0), policyObservation(shared, 1)
	left.Enemies[0].X = 200
	left.PlayerCandidates[0].Mask = 1
	left.Players[0].X = 200
	left.TerrainKnown[0][0] = true
	if right.Actor != 1 || shared.Actor != 0 || right.Enemies[0].X != 12 || shared.Enemies[0].X != 12 || right.PlayerCandidates[0].Mask != 3 || shared.PlayerCandidates[0].Mask != 3 || right.Players[0].X != 0 || right.TerrainKnown[0][0] {
		t.Fatal("one seat changed another seat's public observation")
	}
}

func TestContraModuleSharedClassificationPreservesSeatPolicies(t *testing.T) {
	for _, flags := range [][2]bool{{true, true}, {false, true}, {true, false}} {
		calls := 0
		m, _ := visualModule(t, func(f engine.BotVisualFrame) (engine.BotObservation, error) {
			calls++
			if f.Actor() != 0 {
				t.Fatal("classifier received noncanonical actor")
			}
			o, err := boundaryObserver(f)
			o.Phase = "intro"
			o.Players[0].X, o.Players[1].X = 32, 96
			return o, err
		})
		_, c, g := initialWithModule(t, m, 2)
		c.Automated = []bool{flags[0], flags[1]}
		g = ticks(t, m, &c, g, 17)
		s := saved(t, g)
		if calls != 1 {
			t.Fatal("duplicate shared classification", calls)
		}
		for actor, automated := range flags {
			if !automated {
				if s.Bots[actor].LastTick != 0 {
					t.Fatal("manual bot memory advanced")
				}
				continue
			}
			wantX := 32 + actor*64
			wantPad := engine.Buttons(0)
			if actor == 0 {
				wantPad = engine.ButtonStart
			}
			if s.Bots[actor].LastTick != 1 || s.Bots[actor].LastX != wantX || s.Inputs[actor].Applied != wantPad {
				t.Fatal("shared observation changed seat-specific policy", actor, s.Bots[actor])
			}
		}
	}
}
