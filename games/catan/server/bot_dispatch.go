package catan

import (
	"errors"
	"sort"
)

func (s *State) BotAction(player int) (Action, error) {
	if s.Finished {
		return Action{}, errors.New("game finished")
	}
	return s.catanBot(player)
}

type botChoice struct {
	action Action
	score  int
}

func (s *State) botLegal(player int, choices []botChoice) (Action, error) {
	sort.SliceStable(choices, func(i, j int) bool { return choices[i].score > choices[j].score })
	for _, c := range choices {
		trial := clone(*s)
		if trial.Apply(player, c.action) == nil {
			return c.action, nil
		}
	}
	return Action{}, errors.New("no legal bot action")
}
