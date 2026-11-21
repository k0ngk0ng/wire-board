package hotlinemiami

import (
	"encoding/json"

	"github.com/k0ngk0ng/wire-board/games/hotlinemiami/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func envelope(state *matchState, computer bool) (gameapi.Match, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return gameapi.Match{}, err
	}
	count := len(state.World.Players)
	match := gameapi.Match{Schema: 1, Data: raw, Actors: []int{}, TimeoutActors: []int{}, Attention: []int{}, Log: []string{}, HasComputerPlayers: computer, CheckpointKey: "battle"}
	if !state.World.Finished {
		for seat := range count {
			match.Actors = append(match.Actors, seat)
		}
		match.SystemDeadline = state.BaseAt + ((state.World.Frame+2)*1000+engine.FPS-1)/engine.FPS
		return match, nil
	}
	result := &gameapi.Result{Ranks: make([]int, count), Winners: []int{}, Scores: make([]int, count)}
	best := state.World.Players[0].Score
	for _, player := range state.World.Players {
		best = max(best, player.Score)
	}
	for seat, player := range state.World.Players {
		result.Scores[seat] = player.Score
		result.Ranks[seat] = 1
		for _, other := range state.World.Players {
			if other.Score > player.Score {
				result.Ranks[seat]++
			}
		}
		if player.Score == best {
			result.Winners = append(result.Winners, seat)
		}
	}
	if len(result.Winners) > 1 {
		result.Draw = true
		result.Winners = []int{}
		for seat := range result.Ranks {
			result.Ranks[seat] = 1
		}
	}
	result.Details, _ = json.Marshal(map[string]any{"map": state.World.MapID, "roundSeconds": state.World.Options.RoundSeconds, "players": projection(state, -1).Players})
	match.Result = result
	match.CheckpointKey = "finished"
	return match, nil
}
