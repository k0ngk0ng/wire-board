package engine

import (
	"errors"
	"slices"
)

func progressionSwapItems(state *State, firstID, secondID string) error {
	if firstID == "" || firstID == secondID {
		return errors.New("请选择两个不同的已拥有道具")
	}
	first := slices.IndexFunc(state.Inventory, func(item OwnedItem) bool { return item.ID == firstID })
	second := slices.IndexFunc(state.Inventory, func(item OwnedItem) bool { return item.ID == secondID })
	if first < 0 || second < 0 {
		return errors.New("只能整理已拥有的道具")
	}
	state.Inventory[first], state.Inventory[second] = state.Inventory[second], state.Inventory[first]
	return nil
}
