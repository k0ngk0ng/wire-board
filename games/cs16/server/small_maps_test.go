package cs16

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestSmallMapsConfigureAndRulesIdentity(t *testing.T) {
	for _, id := range []string{"fy_iceworld", "cs_bloodstrike"} {
		raw, err := configure(10, json.RawMessage(`{"map":"`+id+`","friendlyFire":true}`))
		if err != nil {
			t.Fatal(err)
		}
		var options engine.Options
		if json.Unmarshal(raw, &options) != nil || options.Map != id || !options.FriendlyFire || len(options.Teams) != 10 {
			t.Fatal("small map configuration lost rules")
		}
		if len(rulesIdentities[id]) != 64 {
			t.Fatal("small map lacks immutable rules identity")
		}
		match, err := start(gameapi.Context{Players: 10, Now: time.Unix(1700000000, 0)}, raw)
		if err != nil {
			t.Fatal(err)
		}
		state, err := decode(match)
		if err != nil || state.RulesSHA256 != rulesIdentities[id] || state.World.MapID != id {
			t.Fatal("small map start or identity restore failed")
		}
	}
	if _, err := configure(10, json.RawMessage(`{"map":"de_bloodstrike"}`)); err == nil {
		t.Fatal("unregistered alias accepted")
	}
}
