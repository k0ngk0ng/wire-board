package cs16

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
)

func TestHitEffectProjectionIsOptionalCompactAndDoesNotRevealArmorE2E(t *testing.T) {
	for _, fx := range []int{1, 2} {
		state := observerFixture()
		state.World.Players[0].Alive = true
		event := engine.Event{ID: 1, Kind: "hit", Seat: 1, Target: 0, Weapon: "ak47", Value: 17, Hitgroup: 1, Life: 1, FX: fx}
		state.World.Events = []engine.Event{event}
		view := projection(state, 0)
		if len(view.Events) != 1 || view.Players[1].Visible || view.Players[1].Armor != nil || view.Players[1].Helmet != nil {
			t.Fatal("hit effect changed hidden enemy equipment visibility", view)
		}
		raw, err := json.Marshal(view.Events[0])
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(raw, &fields); err != nil || string(fields["fx"]) != string(rune('0'+fx)) {
			t.Fatal("authoritative impact effect missing from projection", string(raw), err)
		}
		for _, key := range []string{"armor", "helmet", "inventory", "armorRemainder"} {
			if fields[key] != nil {
				t.Fatal("cosmetic event disclosed private equipment", key)
			}
		}
		event.FX = 0
		plain, err := json.Marshal(projectedEvent(event))
		if err != nil || len(raw)-len(plain) != 7 {
			t.Fatal("impact metadata exceeded its seven-byte event increment", len(raw), len(plain), err)
		}
		var omitted map[string]json.RawMessage
		if err = json.Unmarshal(plain, &omitted); err != nil || omitted["fx"] != nil {
			t.Fatal("ordinary event gained empty impact metadata", string(plain), err)
		}
	}
}

func TestHiddenVictimHitKeepsDamageWithoutExactPositionOrEffectE2E(t *testing.T) {
	state := observerFixture()
	state.World.Players[0].Alive = true
	state.World.Events = []engine.Event{{ID: 1, Kind: "hit", Seat: 0, Target: 1, Weapon: "ak47", X: state.World.Players[1].X, Value: 17, Hitgroup: 2, FX: 1}}
	view := projection(state, 0)
	if len(view.Events) != 1 || view.Players[1].Visible {
		t.Fatal("hidden victim fixture or owner damage feedback changed", view)
	}
	event := view.Events[0]
	if event.Value != 17 || event.Hitgroup != 2 || event.X != nil || event.Y != nil || event.Z != nil || event.FX != 0 {
		t.Fatal("hidden victim disclosed its origin or equipment effect", event)
	}
	visible := projection(state, -1).Events[0]
	if visible.X == nil || visible.FX != 1 {
		t.Fatal("independent observer lost its visible hit effect", visible)
	}
}
