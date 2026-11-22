package cs16

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
)

func TestClassicCorpseProjectionPreservesVisibleDeathWithoutLeakingLivingEnemies(t *testing.T) {
	state := observerFixture()
	state.World.Frame = 200
	state.World.Players[0].Alive = true
	enemy := &state.World.Players[1]
	enemy.Alive, enemy.X = false, 10*engine.Unit
	enemy.DeathAt, enemy.DeathHitgroup, enemy.DeathVX = 190, 1, 80*engine.Unit
	view := projection(state, 0)
	if !view.Players[1].Visible || view.Players[1].X == nil || view.Players[1].DeathAt != 190 || view.Players[1].DeathHitgroup != 1 || view.Players[1].DeathVX != 80*engine.Unit {
		t.Fatal("visible enemy corpse lost authoritative death pose", view.Players[1])
	}
	if view.Players[1].Inventory != nil || view.Players[1].Money != nil {
		t.Fatal("corpse revealed private inventory")
	}
	enemy.X = 150 * engine.Unit
	view = projection(state, 0)
	if view.Players[1].Visible || view.Players[1].X != nil || view.Players[1].DeathAt != 0 {
		t.Fatal("hidden corpse leaked its position or pose", view.Players[1])
	}
	enemy.Alive = true
	if projection(state, 0).Players[1].Visible {
		t.Fatal("corpse projection exposed a living enemy through a wall")
	}
}

func TestClassicHitgroupsAndPrivateHudReachOnlyAuthorizedProjection(t *testing.T) {
	state := observerFixture()
	state.World.Players[0].Alive = true
	state.World.Players[0].Flashlight, state.World.Players[0].FlashlightCharge, state.World.Players[0].Stamina = true, 91, 500
	state.World.Events = []engine.Event{{ID: 1, Kind: "hit", Seat: 0, Target: 1, Value: 27, Hitgroup: 2}}
	view := projection(state, 0)
	if len(view.Events) != 1 || view.Events[0].Hitgroup != 2 || view.Events[0].Value != 27 || view.Players[0].FlashlightCharge == nil || *view.Players[0].FlashlightCharge != 91 || view.Players[0].Stamina != 500 {
		t.Fatal("authoritative feedback was lost", view)
	}
	if projection(state, 1).Players[0].FlashlightCharge != nil {
		t.Fatal("enemy view leaked private battery charge")
	}
	raw, err := json.Marshal(projection(observerFixture(), 0))
	if err != nil || strings.Contains(string(raw), "deathAt") || strings.Contains(string(raw), "deathVX") {
		t.Fatal("inactive death metadata enlarged the ordinary projection", err)
	}
}

func TestClassicBombSoundUsesAudiblePositionWithoutLeakingDistantHiddenSite(t *testing.T) {
	state := observerFixture()
	state.World.Bomb = engine.Bomb{Planted: true, X: 10 * engine.Unit, Y: 100 * engine.Unit, Defuser: -1, PickupAt: 99}
	near := projection(state, 1)
	if near.Bomb.X == 0 || near.Bomb.PickupAt != 0 {
		t.Fatal("nearby audible bomb lost spatial sound or leaked private timer", near.Bomb)
	}
	state.World.Bomb.Y = 2000 * engine.Unit
	far := projection(state, 1)
	if far.Bomb.X != 0 || far.Bomb.Y != 0 || far.Bomb.Z != 0 {
		t.Fatal("distant unseen bomb position leaked", far.Bomb)
	}
}

func TestClassicPublicDropsExcludeUnusedPrivatePhysics(t *testing.T) {
	state := observerFixture()
	state.World.Players[0].Alive = true
	state.World.Drops = []engine.Drop{{ID: 9, X: 10 * engine.Unit, Weapon: engine.WeaponState{ID: "ak47"}, Moving: true, VX: 123, VY: 456, VZ: 789}}
	view := projection(state, 0)
	if len(view.Drops) != 1 || view.Drops[0].Moving || view.Drops[0].VX != 0 || view.Drops[0].VY != 0 || view.Drops[0].VZ != 0 {
		t.Fatal("drop projection transmitted unused simulation metadata", view.Drops)
	}
}

func TestClassicSpatialEventsDoNotUseAnUnsetTargetAsSeatZero(t *testing.T) {
	state := observerFixture()
	state.World.Players[0].Alive = true
	for _, kind := range []string{"step", "land", "flashlight", "knife_air", "knife_wall", "reload", "pickup"} {
		event := engine.Event{Kind: kind, Seat: 1, X: 5000 * engine.Unit}
		if eventVisible(state, 0, event) {
			t.Fatal("untargeted hidden sound leaked to seat zero", kind)
		}
	}
	if !eventVisible(state, 0, engine.Event{Kind: "hit", Seat: 1, Target: 0}) {
		t.Fatal("actual victim lost own hit feedback")
	}
}
