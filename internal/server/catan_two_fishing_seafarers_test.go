package server

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanTwoFishingSeafarersSwitchAndWorldMap(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, friend := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("捕鱼海图房主")
	friend.register("捕鱼海图朋友")
	raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "双人捕鱼地图", "capacity": 2, "catanTwoScenario": "new_world", "catanFishing": true}, 201)
	id := raw["id"].(string)
	friend.command(current(h), "join", nil, 200)
	command := func(client *testClient, kind string, fields map[string]any, status int) {
		fields["type"], fields["version"], fields["nonce"] = kind, s.rooms[id].Version, randomID(12)
		client.post("/api/rooms/"+id, fields, status)
	}
	original := s.rooms[id].CatanNewWorldMap
	for _, who := range []*testClient{h, friend} {
		before, _ := json.Marshal(s.rooms[id])
		command(who, "catan_world_map", map[string]any{"catanNewWorldMap": game.CatanNewWorldMap{}}, 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("invalid map changed room")
		}
	}
	command(friend, "catan_fishing", map[string]any{"enabled": false}, 400)
	h.command(current(h), "ready", nil, 200)
	friend.command(current(friend), "ready", nil, 200)
	command(h, "catan_fishing", map[string]any{"enabled": true}, 200)
	if !s.rooms[id].Seats[0].Ready || !s.rooms[id].Seats[1].Ready {
		t.Fatal("unchanged toggle clears readiness")
	}
	command(h, "catan_fishing", map[string]any{"enabled": false}, 200)
	if s.rooms[id].Seats[0].Ready || s.rooms[id].Seats[1].Ready || !reflect.DeepEqual(original, s.rooms[id].CatanNewWorldMap) {
		t.Fatal("toggle must reset readiness but preserve map")
	}
	command(h, "catan_fishing", map[string]any{"enabled": true}, 200)
	if !reflect.DeepEqual(original, s.rooms[id].CatanNewWorldMap) {
		t.Fatal("toggle silently rerolled map")
	}
	command(h, "catan_world_map_shuffle", map[string]any{}, 200)
	approved, _ := json.Marshal(s.rooms[id].CatanNewWorldMap)
	for _, c := range []*testClient{h, friend} {
		c.command(current(c), "ready", nil, 200)
	}
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, friend}, id)
	h.command(current(h), "start", nil, 200)
	actual, _ := json.Marshal(s.rooms[id].Game.Catan.NewWorldMap())
	if string(approved) != string(actual) {
		t.Fatal("start changed approved map")
	}
	seenFish := false
	for steps := 0; steps < 100 && !seenFish; steps++ {
		state := s.rooms[id].Game
		actor := twoHTTPActor(state)
		a, err := state.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		if state.Phase == "catan_world_fish" {
			seenFish = true
			before, _ := json.Marshal(s.rooms[id])
			[]*testClient{h, friend}[1-actor].command(current([]*testClient{h, friend}[1-actor]), "action", a, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("nonactor placed fish ground")
			}
		}
		[]*testClient{h, friend}[actor].command(current([]*testClient{h, friend}[actor]), "action", a, 200)
	}
	if !seenFish {
		t.Fatal("missing fish ground placement")
	}
}
