package cs16

import (
	"encoding/json"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func envelope(state *matchState, computer bool, now int64) (gameapi.Match, error) {
	state.ObservedAt = max(state.ObservedAt, now)
	for index := range state.World.History {
		state.World.History[index].Brushes = nil
	}
	raw, err := state.MarshalJSON()
	if err != nil {
		return gameapi.Match{}, err
	}
	count := len(state.World.Players)
	match := gameapi.Match{Schema: 1, Data: raw, Actors: []int{}, TimeoutActors: []int{}, Attention: []int{}, Log: []string{}, HasComputerPlayers: computer, CheckpointKey: fmt.Sprintf("round:%d:%s", state.World.Round, state.World.Phase)}
	if !state.World.Finished {
		for seat := range count {
			match.Actors = append(match.Actors, seat)
		}
		match.SystemDeadline = state.BaseAt + ((state.World.Frame+3)*1000+engine.FPS-1)/engine.FPS
		match.SystemDeadline = max(match.SystemDeadline, now+1000/engine.FPS)
		return match, nil
	}
	result := &gameapi.Result{Ranks: make([]int, count), Winners: []int{}, Scores: make([]int, count), Team: true}
	winner := "t"
	if state.World.TeamScores[1] > state.World.TeamScores[0] {
		winner = "ct"
	}
	if state.World.TeamScores[0] == state.World.TeamScores[1] {
		result.Draw = true
	}
	for seat, player := range state.World.Players {
		index := 0
		if player.Team == "ct" {
			index = 1
		}
		result.Scores[seat] = state.World.TeamScores[index]
		result.Ranks[seat] = 1
		if !result.Draw {
			if player.Team == winner {
				result.Winners = append(result.Winners, seat)
			} else {
				result.Ranks[seat] = 2
			}
		}
	}
	result.Details, _ = json.Marshal(map[string]any{"map": state.World.MapID, "rounds": state.World.Round, "teamScores": state.World.TeamScores, "players": projection(state, -1).Players})
	match.Result = result
	match.CheckpointKey = "finished"
	return match, nil
}
