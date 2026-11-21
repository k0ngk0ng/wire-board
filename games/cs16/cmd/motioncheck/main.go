package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
)

type step struct {
	Input   engine.Input        `json:"input"`
	Frames  int                 `json:"frames"`
	Brushes []engine.BrushState `json:"brushes,omitempty"`
	Players []engine.Player     `json:"players,omitempty"`
}

type request struct {
	MapID  string         `json:"mapId"`
	Player *engine.Player `json:"player,omitempty"`
	Steps  []step         `json:"steps"`
}

type response struct {
	Initial engine.Player   `json:"initial"`
	Players []engine.Player `json:"players"`
}

func run() error {
	var value request
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if len(value.Steps) < 1 || len(value.Steps) > 1024 {
		return fmt.Errorf("step count out of bounds")
	}
	world := engine.MapByID(value.MapID)
	if world.ID == "" || len(world.SpawnsT) == 0 {
		return fmt.Errorf("unknown map")
	}
	spawn := world.SpawnsT[0]
	player := engine.Player{Seat: 0, Team: "t", Character: "terror", X: spawn.X, Y: spawn.Y, Z: spawn.Z, Yaw: spawn.Yaw, Alive: true, Life: 1, Health: 100, Weapon: "knife"}
	if value.Player != nil {
		player = *value.Player
	}
	result := response{Initial: player, Players: make([]engine.Player, 0, len(value.Steps))}
	for _, current := range value.Steps {
		if current.Frames < 1 || current.Frames > 20 {
			return fmt.Errorf("frame count out of bounds")
		}
		player = engine.PredictMotionWithDynamics(world, player, current.Input, current.Frames, current.Brushes, current.Players)
		result.Players = append(result.Players, player)
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
