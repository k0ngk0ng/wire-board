package cs16_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/cs16/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestCS16BrowserSampleHealth(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		mapID     string
		rounds    int
		automated []bool
		expected  []int
	}{
		{"ice-manual", "fy_iceworld", 30, []bool{false, false, true, true}, []int{10000, 10000, 100, 100}},
		{"blood-manual", "cs_bloodstrike", 30, []bool{false, false, true, true}, []int{10000, 10000, 100, 100}},
		{"ice-natural", "fy_iceworld", 2, []bool{false, false, true, true}, []int{100, 100, 100, 100}},
		{"blood-natural", "cs_bloodstrike", 2, []bool{false, false, true, true}, []int{100, 100, 100, 100}},
		{"classic", "de_dust2", 30, []bool{false, false, true, true}, []int{100, 100, 100, 100}},
		{"automated", "fy_iceworld", 30, []bool{true, false, true, true}, []int{100, 10000, 100, 100}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := gameapi.Context{Players: 4, Automated: scenario.automated}
			ctx.Now = time.Unix(1700000000, 0)
			options, _ := json.Marshal(struct {
				Map       string `json:"map"`
				MaxRounds int    `json:"maxRounds"`
			}{scenario.mapID, scenario.rounds})
			original, err := game.Module().Start(ctx, options)
			if err != nil {
				t.Fatal(err)
			}
			before := bytes.Clone(original.Data)
			updated, err := cs16BrowserSampleHealth(ctx, original)
			if err != nil {
				t.Fatal(err)
			}
			var oldFields, newFields map[string]json.RawMessage
			_ = json.Unmarshal(before, &oldFields)
			_ = json.Unmarshal(updated.Data, &newFields)
			var oldWorld, newWorld map[string]json.RawMessage
			_ = json.Unmarshal(oldFields["world"], &oldWorld)
			_ = json.Unmarshal(newFields["world"], &newWorld)
			var oldPlayers, newPlayers []map[string]json.RawMessage
			_ = json.Unmarshal(oldWorld["players"], &oldPlayers)
			_ = json.Unmarshal(newWorld["players"], &newPlayers)
			for seat, expected := range scenario.expected {
				var health int
				_ = json.Unmarshal(newPlayers[seat]["health"], &health)
				if health != expected {
					t.Fatalf("seat %d health %d, expected %d", seat, health, expected)
				}
				newPlayers[seat]["health"] = oldPlayers[seat]["health"]
			}
			newWorld["players"], _ = json.Marshal(newPlayers)
			oldWorld["players"], _ = json.Marshal(oldPlayers)
			newFields["world"], _ = json.Marshal(newWorld)
			oldFields["world"], _ = json.Marshal(oldWorld)
			normalizedOld, _ := json.Marshal(oldFields)
			normalizedNew, _ := json.Marshal(newFields)
			if !bytes.Equal(normalizedOld, normalizedNew) || !bytes.Equal(original.Data, before) {
				t.Fatal("sample health changed non-health state or mutated input")
			}
		})
	}
}

func TestCS16BrowserSampleHealthPreservesLiveAndDeadPlayers(t *testing.T) {
	for _, raw := range []string{
		`{"world":{"mapId":"fy_iceworld","phase":"live","options":{"maxRounds":30},"players":[{"alive":true,"health":75},{"alive":true,"health":80}]}}`,
		`{"world":{"mapId":"cs_bloodstrike","phase":"freeze","options":{"maxRounds":30},"players":[{"alive":false,"health":0},{"alive":false,"health":0}]}}`,
	} {
		match := gameapi.Match{Data: json.RawMessage(raw)}
		updated, err := cs16BrowserSampleHealth(gameapi.Context{Automated: []bool{false, false}}, match)
		if err != nil || !bytes.Equal(updated.Data, match.Data) {
			t.Fatalf("live or dead state changed: %s, %v", updated.Data, err)
		}
	}
}
