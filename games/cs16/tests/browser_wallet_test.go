package cs16_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/cs16/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestCS16BrowserSampleWallet(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		mapID     string
		rounds    int
		automated []bool
		expected  []int
	}{
		{"ice-manual", "fy_iceworld", 30, []bool{false, false, true, true}, []int{16000, 16000, 800, 800}},
		{"blood-manual", "cs_bloodstrike", 30, []bool{false, false, true, true}, []int{16000, 16000, 800, 800}},
		{"natural", "fy_iceworld", 2, []bool{false, false, true, true}, []int{800, 800, 800, 800}},
		{"classic", "de_dust2", 30, []bool{false, false, true, true}, []int{800, 800, 800, 800}},
		{"automated", "cs_bloodstrike", 30, []bool{true, false, false, true}, []int{800, 16000, 800, 800}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := gameapi.Context{Players: 4, Automated: scenario.automated, Now: time.Unix(1700000000, 0)}
			options, _ := json.Marshal(map[string]any{"map": scenario.mapID, "maxRounds": scenario.rounds})
			original, err := game.Module().Start(ctx, options)
			if err != nil {
				t.Fatal(err)
			}
			before := bytes.Clone(original.Data)
			updated, err := cs16BrowserSampleWallet(ctx, original)
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
				var money int
				_ = json.Unmarshal(newPlayers[seat]["money"], &money)
				if money != expected {
					t.Fatalf("seat %d money %d, expected %d", seat, money, expected)
				}
				newPlayers[seat]["money"] = oldPlayers[seat]["money"]
			}
			newWorld["players"], _ = json.Marshal(newPlayers)
			oldWorld["players"], _ = json.Marshal(oldPlayers)
			newFields["world"], _ = json.Marshal(newWorld)
			oldFields["world"], _ = json.Marshal(oldWorld)
			oldNormalized, _ := json.Marshal(oldFields)
			newNormalized, _ := json.Marshal(newFields)
			if !bytes.Equal(oldNormalized, newNormalized) || !bytes.Equal(original.Data, before) {
				t.Fatal("sample wallet changed non-wallet state or mutated the production match")
			}
		})
	}
}

func TestCS16BrowserSampleWalletNeverRefillsAnExistingMatch(t *testing.T) {
	for _, raw := range []string{
		`{"world":{"mapId":"fy_iceworld","phase":"live","round":1,"frame":1,"options":{"maxRounds":30},"players":[{"alive":true,"money":75}]}}`,
		`{"world":{"mapId":"cs_bloodstrike","phase":"freeze","round":2,"frame":0,"options":{"maxRounds":30},"players":[{"alive":true,"money":80}]}}`,
		`{"world":{"mapId":"cs_bloodstrike","phase":"freeze","round":1,"frame":1,"options":{"maxRounds":30},"players":[{"alive":true,"money":80}]}}`,
		`{"world":{"mapId":"cs_bloodstrike","phase":"freeze","round":1,"frame":0,"options":{"maxRounds":30},"players":[{"alive":false,"money":80}]}}`,
	} {
		match := gameapi.Match{Data: json.RawMessage(raw)}
		updated, err := cs16BrowserSampleWallet(gameapi.Context{Automated: []bool{false}}, match)
		if err != nil || !bytes.Equal(updated.Data, match.Data) {
			t.Fatalf("existing or dead match wallet changed: %s, %v", updated.Data, err)
		}
	}
}

func TestCS16BrowserModuleFundsOnlyManualSmallMapStart(t *testing.T) {
	ctx := gameapi.Context{Players: 2, Automated: []bool{false, false}, Now: time.Unix(1700000000, 0)}
	options := json.RawMessage(`{"map":"cs_bloodstrike","maxRounds":30}`)
	for _, fixture := range []bool{false, true} {
		module := game.Module()
		if fixture {
			module = cs16BrowserModule()
		}
		match, err := module.Start(ctx, options)
		if err != nil {
			t.Fatal(err)
		}
		var state struct {
			World struct {
				Players []struct {
					Money int `json:"money"`
				} `json:"players"`
			} `json:"world"`
		}
		if err = json.Unmarshal(match.Data, &state); err != nil {
			t.Fatal(err)
		}
		expected := 800
		if fixture {
			expected = 16000
		}
		if state.World.Players[0].Money != expected {
			t.Fatal("fixture funding escaped the browser wrapper", fixture, state.World.Players[0].Money)
		}
	}
}
