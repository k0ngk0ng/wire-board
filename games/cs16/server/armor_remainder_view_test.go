package cs16

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestArmorRemainderStaysOutOfPublicProjection(t *testing.T) {
	state := observerFixture()
	state.World.Players[0].ArmorRemainder = 400000
	state.World.Players[1].ArmorRemainder = 300000
	private, err := json.Marshal(state.World)
	if err != nil || !strings.Contains(string(private), "armorRemainder") {
		t.Fatal("saved rule state lost internal armor precision", err)
	}
	for _, actor := range []int{-1, 0, 1} {
		raw, err := json.Marshal(projection(state, actor))
		if err != nil || strings.Contains(string(raw), "armorRemainder") {
			t.Fatal("internal armor precision leaked into public projection", actor, err)
		}
	}
}

func TestArmorRemainderDefinesActualHelmetProjection(t *testing.T) {
	for _, fraction := range []int{0, 400000} {
		state := observerFixture()
		p := &state.World.Players[0]
		p.Armor, p.ArmorRemainder, p.Helmet = 0, fraction, true
		view := projection(state, 0)
		if view.Players[0].Helmet == nil || *view.Players[0].Helmet != (fraction > 0) {
			t.Fatal("helmet projection does not match actual full and fractional armor", fraction)
		}
		if !p.Helmet || p.ArmorRemainder != fraction {
			t.Fatal("view normalization mutated saved player state", fraction)
		}
	}
}
