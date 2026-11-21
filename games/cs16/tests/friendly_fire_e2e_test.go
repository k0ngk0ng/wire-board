package cs16_test

import (
	"encoding/json"
	"testing"

	game "github.com/k0ngk0ng/wire-board/games/cs16/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestFriendlyFireStrictConfigurationAndPreparation(t *testing.T) {
	module := game.Module()
	raw, err := module.Configure(10, nil)
	if err != nil {
		t.Fatal(err)
	}
	var options struct {
		FriendlyFire bool `json:"friendlyFire"`
	}
	if json.Unmarshal(raw, &options) != nil || options.FriendlyFire {
		t.Fatal("friendly fire default must be disabled")
	}
	for _, value := range []string{`{"friendlyFire":null}`, `{"friendlyFire":1}`, `{"friendlyFire":"true"}`, `{"friendlyFire":[]}`, `{"friendlyFire":true,"friendlyFire":false}`} {
		if _, err = module.Configure(10, json.RawMessage(value)); err == nil {
			t.Fatal("invalid friendly fire accepted", value)
		}
	}
	on, err := module.Configure(10, encode(map[string]any{"friendlyFire": true}))
	if err != nil || json.Unmarshal(on, &options) != nil || !options.FriendlyFire {
		t.Fatal("friendly fire room option rejected", err)
	}
	ctx := gameapi.PreparationContext{Actor: 1, Capacity: 10, Bots: []bool{false, false}}
	action := encode(map[string]any{"type": "settings", "friendlyFire": false})
	if _, err = module.Prepare(ctx, on, action); err == nil {
		t.Fatal("non-host changed friendly fire")
	}
	ctx.Actor, ctx.Host = 0, true
	off, err := module.Prepare(ctx, on, action)
	if err != nil || json.Unmarshal(off, &options) != nil || options.FriendlyFire {
		t.Fatal("host could not disable friendly fire", err)
	}
	artifact(t, map[string]any{"passed": true, "default": json.RawMessage(raw), "enabled": json.RawMessage(on), "disabled": json.RawMessage(off)})
}

func TestFriendlyFireRoomHTTPFreezesAtStart(t *testing.T) {
	harness := newHTTPRoom(t, 2)
	host, peer := harness.people[0], harness.people[1]
	created := host.request("POST", "/api/rooms", map[string]any{"kind": "cs16", "name": "队友误伤房间规则", "capacity": 2, "options": map[string]any{"friendlyFire": true}}, 201)
	id := created["id"].(string)
	peer.request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": host.room()["version"], "nonce": "cs16-friendly-peer-join"}, 200)
	peer.command("prepare", map[string]any{"action": map[string]any{"type": "settings", "friendlyFire": false}}, 400)
	host.command("prepare", map[string]any{"action": map[string]any{"type": "settings", "friendlyFire": false}}, 200)
	host.command("prepare", map[string]any{"action": map[string]any{"type": "settings", "friendlyFire": true}}, 200)
	host.command("ready", nil, 200)
	peer.command("ready", nil, 200)
	host.command("start", nil, 200)
	before := host.room()["game"].(map[string]any)["data"].(map[string]any)
	host.command("prepare", map[string]any{"action": map[string]any{"type": "settings", "friendlyFire": false}}, 400)
	after := host.room()["game"].(map[string]any)["data"].(map[string]any)
	if before["options"].(map[string]any)["friendlyFire"] != true || after["options"].(map[string]any)["friendlyFire"] != true {
		t.Fatal("live friendly fire changed or was lost")
	}
	artifact(t, map[string]any{"passed": true, "scope": "real HTTP host-only friendly fire room option frozen at match start", "before": before, "after": after})
}
