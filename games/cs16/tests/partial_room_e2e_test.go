package cs16_test

import (
	"fmt"
	"testing"
)

func TestDefaultTenSeatRoomStartsWithActualParticipants(t *testing.T) {
	for _, count := range []int{2, 3, 6, 10} {
		t.Run(fmt.Sprintf("players-%d", count), func(t *testing.T) {
			harness := newHTTPRoom(t, count)
			host := harness.people[0]
			created := host.request("POST", "/api/rooms", map[string]any{"kind": "cs16", "name": "十席非满员开局", "capacity": 10}, 201)
			id := created["id"].(string)
			for index := 1; index < count; index++ {
				harness.people[index].request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": host.room()["version"], "nonce": fmt.Sprintf("partial-join-%d", index)}, 200)
			}
			harness.people[1].command("prepare", map[string]any{"action": map[string]any{"type": "character", "character": "sas"}}, 200)
			for _, person := range harness.people {
				person.command("ready", nil, 200)
			}
			host.command("start", nil, 200)
			current := host.room()
			data := current["game"].(map[string]any)["data"].(map[string]any)
			players := data["players"].([]any)
			if integer(current["capacity"]) != 10 || len(players) != count || players[1].(map[string]any)["character"] != "sas" {
				t.Fatal("actual roster or selected character changed", current)
			}
			artifact(t, map[string]any{"passed": true, "capacity": 10, "participants": count, "status": current["status"], "players": players})
		})
	}
}

func TestEmptySeatsDoNotBlockTeamSelection(t *testing.T) {
	harness := newHTTPRoom(t, 3)
	host := harness.people[0]
	created := host.request("POST", "/api/rooms", map[string]any{"kind": "cs16", "name": "按实际玩家组队", "capacity": 10}, 201)
	id := created["id"].(string)
	for index := 1; index < 3; index++ {
		harness.people[index].request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": host.room()["version"], "nonce": fmt.Sprintf("team-join-%d", index)}, 200)
	}
	harness.people[2].command("prepare", map[string]any{"action": map[string]any{"type": "team", "team": "ct"}}, 200)
	for _, person := range harness.people {
		person.command("ready", nil, 200)
	}
	host.command("start", nil, 200)
	current := host.room()
	data := current["game"].(map[string]any)["data"].(map[string]any)
	players := data["players"].([]any)
	if players[0].(map[string]any)["team"] != "t" || players[1].(map[string]any)["team"] != "ct" || players[2].(map[string]any)["team"] != "ct" {
		t.Fatal("actual teams changed", players)
	}
	artifact(t, map[string]any{"passed": true, "capacity": 10, "participants": 3, "players": players})
}

func TestPartialRoomRematchKeepsSelectionsAndAcceptsNewPlayers(t *testing.T) {
	harness := newHTTPRoom(t, 3)
	host, peer := harness.people[0], harness.people[1]
	created := host.request("POST", "/api/rooms", map[string]any{"kind": "cs16", "name": "非满员续局", "capacity": 10}, 201)
	id := created["id"].(string)
	peer.request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": host.room()["version"], "nonce": "partial-rematch-peer"}, 200)
	peer.command("prepare", map[string]any{"action": map[string]any{"type": "character", "character": "sas"}}, 200)
	for _, person := range harness.people[:2] {
		person.command("ready", nil, 200)
	}
	host.command("start", nil, 200)
	host.command("close", nil, 200)
	host.command("rematch", nil, 200)
	host.command("prepare", map[string]any{"action": map[string]any{"type": "map", "map": "cs_bloodstrike"}}, 200)
	harness.people[2].request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": host.room()["version"], "nonce": "partial-rematch-third"}, 200)
	for _, person := range harness.people {
		person.command("ready", nil, 200)
	}
	host.command("start", nil, 200)
	current := host.room()
	data := current["game"].(map[string]any)["data"].(map[string]any)
	players := data["players"].([]any)
	if len(players) != 3 || players[1].(map[string]any)["character"] != "sas" || data["mapId"] != "cs_bloodstrike" {
		t.Fatal("partial rematch lost selections or roster", data)
	}
	artifact(t, map[string]any{"passed": true, "capacity": 10, "participants": 3, "map": data["mapId"], "players": players})
}
