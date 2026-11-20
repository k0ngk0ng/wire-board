package pokepath_test

import "github.com/k0ngk0ng/wire-board/games/pokepath/engine"

func referenceNew(options engine.Options, players int, seed uint64, now int64) (*engine.State, error) {
	state, err := engine.New(options, 1, seed, now)
	if err != nil {
		return nil, err
	}
	state.Players = players
	state.Team = state.Team[:1]
	state.Progression = engine.ProgressionState{}
	if err := engine.InitProgression(state); err != nil {
		return nil, err
	}
	return state, nil
}
