package cs16

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
)

func TestExplosionAudibleEventKeepsHiddenThrowerPrivateE2E(t *testing.T) {
	for _, delta := range []int{-1, 0, 1} {
		state := observerFixture()
		state.World.Players[0].Alive = true
		event := engine.Event{ID: 1, Kind: "explode", Seat: 1, Target: -1, Weapon: "hegrenade", X: 2048*engine.Unit + delta}
		state.World.Events = []engine.Event{event}
		view := projection(state, 0)
		if view.Players[1].Visible || view.Players[1].X != nil || view.Players[1].Inventory != nil {
			t.Fatal("audible explosion exposed hidden thrower", view.Players[1])
		}
		if (len(view.Events) == 1) != (delta <= 0) {
			t.Fatal("real explode event did not use the registered audible boundary", delta, view.Events)
		}
		if len(view.Events) == 1 && (view.Events[0].X == nil || *view.Events[0].X != event.X || view.Events[0].Global) {
			t.Fatal("spatial explosion metadata changed", view.Events)
		}
	}
}
