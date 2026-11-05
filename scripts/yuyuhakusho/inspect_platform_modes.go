//go:build ignore

// Read-only acceptance inspector. Checkpoint bytes remain opaque in JavaScript.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	yuyu "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/server"
)

func main() {
	input := flag.String("input", "", "actual authenticated sync envelope")
	flag.Parse()
	raw, err := os.ReadFile(*input)
	if err != nil {
		panic(err)
	}
	var envelope struct {
		Data struct {
			Checkpoint string `json:"checkpoint"`
		} `json:"data"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		panic(err)
	}
	var checkpoint yuyu.Checkpoint
	if err = json.Unmarshal([]byte(envelope.Data.Checkpoint), &checkpoint); err != nil {
		panic(err)
	}
	players := []map[string]any{}
	for seat, p := range checkpoint.Options.Players {
		object := checkpoint.World.Objects[0x8400+uint16(p.Slot)*0x400]
		players = append(players, map[string]any{"seat": seat, "slot": p.Slot, "characterID": p.CharacterID, "configuredHumanChannel": p.HumanChannel, "effectiveInputChannel": object.InputChannel, "baseTeam": object.Body.BaseTeam, "team": object.Body.Team, "health": object.Body.Health, "energy": int16(object.Energy >> 16)})
	}
	out := map[string]any{"epoch": checkpoint.Epoch, "frame": fmt.Sprint(checkpoint.Frame), "arena": checkpoint.Options.Arena, "teamBattle": checkpoint.Options.TeamBattle, "grouping": checkpoint.World.Match.Grouping, "players": players}
	if err = json.NewEncoder(os.Stdout).Encode(out); err != nil {
		panic(err)
	}
}
