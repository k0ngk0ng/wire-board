package engine

// Failure inventory before implementation: enter a tunnel after diagonal travel
// has already left the raised floor; miss a legitimate entrance; fail to apply
// the old floor's depth travel attribute; change fractional positions, possession,
// action requests or other objects. Exercise both sides and both travel axes.
import (
	"fmt"
	"reflect"
	"testing"
)

func TestOriginalNewYorkCornerCarrierCalls(t *testing.T) {
	var rows []struct {
		worldActionFixture
		Actor            uint8
		X, Depth, VX, VZ int
	}
	readPropAttackFixture(t, "testdata/new-york-corner.json.gz", &rows)
	if len(rows) != 480 {
		t.Fatal("incomplete original corner calls", len(rows))
	}
	for i, r := range rows {
		t.Run(fmt.Sprintf("%d-x%d-z%d-v%d-%d", i, r.X, r.Depth, r.VX, r.VZ), func(t *testing.T) {
			a, c := r.Before.Actors[r.Actor], r.context(t)
			got, err := StepCarrierAction(Carrier{Player: a.Player, ShotCycle: a.ShotCycle}, CarrierContext{HasBall: r.Before.Holder == int8(r.Actor), Computer: c.Controls[r.Actor]&128 != 0, BallAction: r.Before.Ball.Action}, c.Profiles[r.Actor], NewYorkGroundCourt())
			if err != nil {
				t.Fatal(err)
			}
			world := r.Before
			world.Actors[r.Actor].Player, world.Actors[r.Actor].ShotCycle = got.Carrier.Player, got.Carrier.ShotCycle
			world.Ball = ApplyBallCommand(world.Ball, got.Ball)
			if got.BindBall {
				world.Holder = int8(r.Actor)
			}
			if world != r.After {
				t.Fatal(firstWorldLoopDifference(reflect.ValueOf(world), reflect.ValueOf(r.After), "world"))
			}
		})
	}
}
