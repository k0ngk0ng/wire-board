package campaign

import "fmt"

// Equip confirms a source91DD selection by inventory index. The item cannot
// be supplied independently of ownership. Confirmed changes persist when a
// later menu selection is canceled; cancellation submits no operation here.
func Equip(p Progress, character, index uint8) (Progress, error) {
	if err := p.Validate(); err != nil {
		return p, err
	}
	if character >= 4 || index >= 16 {
		return p, fmt.Errorf("equipment character/index out of range")
	}
	item := p.Inventory[index]
	if item == Empty {
		return p, fmt.Errorf("inventory index %d has no owned equipment", index)
	}
	slot, err := SlotForItem(item)
	if err != nil {
		return p, err
	}
	next := p
	old := p.Equipment[character][slot]
	next.Equipment[character][slot] = item
	if old != Empty {
		next.Inventory[index] = old
		return next, nil
	}
	copy(next.Inventory[index:], next.Inventory[index+1:])
	next.Inventory[len(next.Inventory)-1] = Empty
	return next, nil
}

type UnequipStep struct {
	Progress Progress
	// Empty means no discard. The original clears equipment before looking for
	// a vacancy and discards it when all16 inventory slots are occupied. This
	// explicit observation does not claim that state is normally menu-reachable.
	DiscardedItem Item
}

// Unequip restores931D. Empty equipment is a no-op. In the abnormal full-
// inventory state it retains the original discard behavior, visibly reported
// to the caller; it does not invent a transactional capacity protection rule.
func Unequip(p Progress, character uint8, slot Slot) (UnequipStep, error) {
	initial := UnequipStep{Progress: p, DiscardedItem: Empty}
	if err := p.Validate(); err != nil {
		return initial, err
	}
	if character >= 4 || slot > Shoes {
		return initial, fmt.Errorf("equipment character/slot out of range")
	}
	item := p.Equipment[character][slot]
	if item == Empty {
		return initial, nil
	}
	next := initial
	next.Progress.Equipment[character][slot] = Empty
	for index, v := range next.Progress.Inventory {
		if v == Empty {
			next.Progress.Inventory[index] = item
			return next, nil
		}
	}
	next.DiscardedItem = item
	return next, nil
}
